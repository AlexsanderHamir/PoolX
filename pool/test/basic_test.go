package test

import (
	"sync"
	"testing"
	"time"

	"memctx/pool"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testObject struct {
	value int
}

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

	// Return objects
	for _, obj := range objects {
		p.Put(obj)
	}

	p.PrintPoolStats()
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
		SetInitialCapacity(64).
		SetMinShrinkCapacity(32).                             // Allow shrinking down to half of initial capacity
		SetHardLimit(1000).                                   // Reasonable hard limit
		SetGrowthPercent(0.5).                                // Grow by 50% when below threshold
		SetFixedGrowthFactor(1.0).                            // Grow by initial capacity when above threshold
		SetGrowthExponentialThresholdFactor(4.0).             // Switch to fixed growth at 4x initial capacity
		SetShrinkAggressiveness(pool.AggressivenessBalanced). // Balanced shrink behavior
		SetShrinkCheckInterval(2 * time.Second).
		SetIdleThreshold(5 * time.Second).
		SetMinIdleBeforeShrink(2).
		SetShrinkCooldown(5 * time.Second).
		SetMinUtilizationBeforeShrink(0.3). // Shrink if utilization below 30%
		SetStableUnderutilizationRounds(3).
		SetShrinkPercent(0.25). // Shrink by 25% when conditions met
		// L1 Cache settings
		SetBufferSize(32).             // L1 cache size
		SetFillAggressiveness(0.8).    // Fill L1 aggressively
		SetRefillPercent(0.2).         // Refill L1 when below 20%
		SetEnableChannelGrowth(true).  // Allow L1 to grow
		SetGrowthEventsTrigger(3).     // Grow L1 after 3 growth events
		SetFastPathGrowthPercent(0.5). // L1 growth rate
		SetFastPathExponentialThresholdFactor(4.0).
		SetFastPathFixedGrowthFactor(1.0).
		SetShrinkEventsTrigger(3). // Shrink L1 after 3 shrink events
		SetFastPathShrinkAggressiveness(pool.AggressivenessBalanced).
		SetFastPathShrinkPercent(0.25).
		SetFastPathShrinkMinCapacity(16). // L1 minimum size
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

	var wg sync.WaitGroup
	iterations := 10
	workers := 10

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iterations {
				obj := p.Get()
				assert.NotNil(t, obj)
				time.Sleep(time.Millisecond)
				p.Put(obj)
			}
		}()
	}

	wg.Wait()
}

// The default configuration is non blocking, so if we can't grow and there are no more available elements
// the ring buffer will return nil.
func TestHardLimit(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(2).
		SetHardLimit(5).
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

	objects := make([]*testObject, 6)
	var nilCount int
	for i := range 6 {
		objects[i] = p.Get()
		if objects[i] == nil {
			nilCount++
		}
	}

	assert.Equal(t, 1, nilCount)
}

func TestConfigValues(t *testing.T) {
	defaultConfig, err := pool.NewPoolConfigBuilder().Build()
	require.NoError(t, err)

	originalValues := storeDefaultConfigValues(defaultConfig)

	customConfig, cancel := createCustomConfig(t)
	defer cancel()

	verifyCustomValuesDifferent(t, originalValues, customConfig)
}
