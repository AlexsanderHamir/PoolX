package test

import (
	"sync"
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

	obj := p.Get()
	assert.NotNil(t, obj)
	assert.Equal(t, 42, obj.value)

	p.Put(obj)
	obj = p.Get()
	assert.NotNil(t, obj)
	assert.Equal(t, 42, obj.value)

	p.PrintPoolStats()
}

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

	// Get more objects than initial capacity to trigger growth
	objects := make([]*testObject, 10)
	for i := range 10 {
		objects[i] = p.Get()
		assert.NotNil(t, objects[i])
	}

	assert.True(t, p.IsGrowth())
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

	// Get and return objects to trigger shrink
	objects := make([]*testObject, 10)
	for i := range 10 {
		objects[i] = p.Get()
	}

	// Wait for shrink to potentially occur
	time.Sleep(20 * time.Millisecond)

	assert.True(t, p.IsShrunk())
}

func TestConcurrentAccess(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetRingBufferBlocking(true). // Blocking ring buffer so it doesn't return nil.
		SetInitialCapacity(64).
		SetHardLimit(90).                         // Reasonable hard limit
		SetGrowthPercent(0.5).                    // Grow by 50% when below threshold
		SetFixedGrowthFactor(1.0).                // Grow by initial capacity when above threshold
		SetGrowthExponentialThresholdFactor(4.0). // Switch to fixed growth at 4x initial capacity
		SetMinShrinkCapacity(32).                 // Allow shrinking down to half of initial capacity
		// L1 Cache settings
		SetFastPathInitialSize(32).           // L1 cache size
		SetFastPathFillAggressiveness(0.8).   // Fill L1 aggressively
		SetFastPathRefillPercent(0.2).        // Refill L1 when below 20%
		SetFastPathEnableChannelGrowth(true). // Allow L1 to grow
		SetFastPathGrowthEventsTrigger(3).    // Grow L1 after 3 growth events
		SetFastPathGrowthPercent(0.5).        // L1 growth rate
		SetFastPathExponentialThresholdFactor(4.0).
		SetFastPathFixedGrowthFactor(1.0).
		SetFastPathShrinkEventsTrigger(3). // Shrink L1 after 3 shrink events
		SetFastPathShrinkAggressiveness(pool.AggressivenessBalanced).
		SetFastPathShrinkPercent(0.25).
		SetFastPathShrinkMinCapacity(16). // L1 minimum size
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

	var wg sync.WaitGroup
	iterations := 1000
	workers := 100

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iterations {
				obj := p.Get()
				time.Sleep(3 * time.Millisecond)
				assert.NotNil(t, obj)
				p.Put(obj)
			}
		}()
	}

	wg.Wait()
	p.PrintPoolStats()
}

func TestHardLimit(t *testing.T) {
	t.Run("nil return", func(t *testing.T) {
		config := createHardLimitTestConfig(t, false)
		p := createTestPool(t, config)
		runNilReturnTest(t, p)
	})

	t.Run("blocking", func(t *testing.T) {
		config := createHardLimitTestConfig(t, true)
		p := createTestPool(t, config)
		runBlockingTest(t, p)
	})

	t.Run("blocking (concurrent)", func(t *testing.T) {
		config := createHardLimitTestConfig(t, true)
		p := createTestPool(t, config)
		runConcurrentBlockingTest(t, p, 10)
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
