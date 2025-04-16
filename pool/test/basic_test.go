package test

import (
	"testing"
	"time"

	"memctx/pool"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasicPoolOperations(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(10).
		SetMinShrinkCapacity(1).
		SetShrinkCheckInterval(1 * time.Second).
		SetVerbose(true).
		Build()
	require.NoError(t, err)

	allocator := func() *testObject {
		return &testObject{value: 42}
	}

	cleaner := func(obj *testObject) {
		obj.value = 0
	}

	p, err := pool.NewPool(config, allocator, cleaner)
	require.NoError(t, err)
	require.NotNil(t, p)
	defer func() {
		require.NoError(t, p.Close())
	}()

	obj := p.Get()
	assert.NotNil(t, obj)
	assert.Equal(t, 42, obj.value)

	p.Put(obj)
	obj = p.Get()
	assert.NotNil(t, obj)
	assert.Equal(t, 42, obj.value)
}

// blocking mode is disabled by default, we always attempt to refill / grow,
// before hitting the retrive functions from the ring buffer, which will return nil.
func TestPoolGrowth(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(2).
		SetGrowthPercent(0.5).
		SetFixedGrowthFactor(1.0).
		SetMinShrinkCapacity(1).
		SetShrinkCheckInterval(1 * time.Second).
		SetVerbose(true).
		Build()
	require.NoError(t, err)

	allocator := func() *testObject {
		return &testObject{value: 42}
	}

	cleaner := func(obj *testObject) {
		obj.value = 0
	}

	p, err := pool.NewPool(config, allocator, cleaner)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Close())
	}()

	objects := make([]*testObject, 10)
	for i := range 10 {
		objects[i] = p.Get()
		assert.NotNil(t, objects[i])
	}

	assert.True(t, p.IsGrowth())

	// Clean up objects before closing
	for _, obj := range objects {
		p.Put(obj)
	}
}

func TestPoolShrink(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(64).
		EnforceCustomConfig().
		SetShrinkCheckInterval(10 * time.Millisecond). // Very frequent checks
		SetIdleThreshold(20 * time.Millisecond).       // Short idle time
		SetMinIdleBeforeShrink(1).                     // Shrink after just 1 idle check
		SetShrinkCooldown(10 * time.Millisecond).      // Short cooldown
		SetMinUtilizationBeforeShrink(0.1).            // Shrink if utilization below 10%
		SetStableUnderutilizationRounds(1).            // Only need 1 round of underutilization
		SetShrinkPercent(0.5).                         // Shrink by 50%
		SetMinShrinkCapacity(1).                       // Can shrink down to 1
		SetMaxConsecutiveShrinks(5).                   // Allow multiple consecutive shrinks
		SetGrowthPercent(0.5).                         // Grow by 50% when needed
		SetFixedGrowthFactor(1.0).                     // Fixed growth factor
		Build()
	require.NoError(t, err)

	allocator := func() *testObject {
		return &testObject{value: 42}
	}

	cleaner := func(obj *testObject) {
		obj.value = 0
	}

	p, err := pool.NewPool(config, allocator, cleaner)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Close())
	}()

	// Get and return objects to trigger shrink
	objects := make([]*testObject, 10)
	for i := range 10 {
		objects[i] = p.Get()
	}

	// Wait for shrink to potentially occur
	time.Sleep(100 * time.Millisecond)
	assert.True(t, p.IsShrunk())

	// Release the first batch of objects
	for _, obj := range objects {
		p.Put(obj)
	}
}

func TestHardLimit(t *testing.T) {
	t.Run("non-blocking", func(t *testing.T) {
		config := createHardLimitTestConfig(t, false)
		p := createTestPool(t, config)
		defer func() {
			require.NoError(t, p.Close())
		}()
		runNilReturnTest(t, p)
	})

	t.Run("blocking", func(t *testing.T) {
		config := createHardLimitTestConfig(t, true)
		p := createTestPool(t, config)
		defer func() {
			require.NoError(t, p.Close())
		}()
		runBlockingTest(t, p)
	})

	t.Run("blocking (concurrent)", func(t *testing.T) {
		config := createHardLimitTestConfig(t, true)
		p := createTestPool(t, config)
		defer func() {
			require.NoError(t, p.Close())
		}()
		runConcurrentBlockingTest(t, p, 20)
	})
}

func TestConfigValues(t *testing.T) {
	defaultConfig, err := pool.NewPoolConfigBuilder().Build()
	require.NoError(t, err)

	originalValues := storeDefaultConfigValues(defaultConfig)

	customConfig, cancel := createCustomConfig(t)
	defer cancel()

	verifyCustomValuesDifferent(t, originalValues, customConfig)
}

func TestPoolComprehensive(t *testing.T) {
	config := createComprehensiveTestConfig(t)
	internalConfig := pool.ToInternalConfig(config)

	allocator := func() *testObject {
		return &testObject{value: 42}
	}

	cleaner := func(obj *testObject) {
		obj.value = 0
	}

	p, err := pool.NewPool(internalConfig, allocator, cleaner)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Close())
	}()

	t.Run("InitialGrowth", func(t *testing.T) {
		objects := testInitialGrowth(t, p, 5)
		cleanupPoolObjects(p, objects)
	})

	t.Run("ShrinkBehavior", func(t *testing.T) {
		objects := testInitialGrowth(t, p, 5)
		testShrinkBehavior(t, p, objects)
		cleanupPoolObjects(p, objects)
	})

	t.Run("HardLimitBlocking", func(t *testing.T) {
		objects := testHardLimitBlocking(t, p, 24)
		cleanupPoolObjects(p, objects[1:])
	})
}
