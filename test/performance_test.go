package test

import (
	"testing"
	"time"

	"github.com/AlexsanderHamir/memory_context/pool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPerformanceGetPut(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(1000).
		SetHardLimit(10000).
		Build()
	require.NoError(t, err)

	p := createTestPool(t, config)
	defer func() {
		require.NoError(t, p.Close())
	}()

	// Measure Get performance
	start := time.Now()
	objects := make([]*TestObject, 1000)
	for i := range objects {
		objects[i], err = p.Get()
		require.NoError(t, err)
		require.NotNil(t, objects[i])
	}
	getDuration := time.Since(start)

	// Measure Put performance
	start = time.Now()
	for _, obj := range objects {
		err := p.Put(obj)
		require.NoError(t, err)
	}
	putDuration := time.Since(start)

	// Verify performance is within acceptable limits
	assert.Less(t, getDuration, 100*time.Millisecond)
	assert.Less(t, putDuration, 100*time.Millisecond)
}

func TestPerformanceGrowth(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(100).
		SetHardLimit(10000).
		SetGrowthPercent(0.5).
		Build()
	require.NoError(t, err)

	p := createTestPool(t, config)
	defer func() {
		require.NoError(t, p.Close())
	}()

	// Measure growth performance
	start := time.Now()
	objects := make([]*TestObject, 1000)
	for i := range objects {
		objects[i], err = p.Get()
		require.NoError(t, err)
		require.NotNil(t, objects[i])
	}
	growthDuration := time.Since(start)

	// Verify growth performance is within acceptable limits
	assert.Less(t, growthDuration, 500*time.Millisecond)

	// Cleanup
	for _, obj := range objects {
		err := p.Put(obj)
		require.NoError(t, err)
	}
}

func TestPerformanceShrink(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(1000).
		SetHardLimit(10000).
		SetShrinkCheckInterval(10 * time.Millisecond).
		SetIdleThreshold(20 * time.Millisecond).
		SetMinIdleBeforeShrink(1).
		SetShrinkCooldown(10 * time.Millisecond).
		SetMinUtilizationBeforeShrink(0.1).
		SetStableUnderutilizationRounds(1).
		SetShrinkPercent(0.5).
		Build()
	require.NoError(t, err)

	p := createTestPool(t, config)
	defer func() {
		require.NoError(t, p.Close())
	}()

	// Fill pool
	objects := make([]*TestObject, 1000)
	for i := range objects {
		objects[i], err = p.Get()
		require.NoError(t, err)
		require.NotNil(t, objects[i])
	}

	// Return all objects
	for _, obj := range objects {
		err := p.Put(obj)
		require.NoError(t, err)
	}

	// Measure shrink performance
	start := time.Now()
	time.Sleep(100 * time.Millisecond) // Wait for shrink to occur
	shrinkDuration := time.Since(start)

	// Verify shrink performance is within acceptable limits
	assert.Less(t, shrinkDuration, 200*time.Millisecond)
	assert.True(t, p.IsShrunk())
}

func TestStressTest(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(100).
		SetHardLimit(10000).
		SetGrowthPercent(0.5).
		SetShrinkCheckInterval(10 * time.Millisecond).
		SetIdleThreshold(20 * time.Millisecond).
		SetMinIdleBeforeShrink(1).
		SetShrinkCooldown(10 * time.Millisecond).
		SetMinUtilizationBeforeShrink(0.1).
		SetStableUnderutilizationRounds(1).
		SetShrinkPercent(0.5).
		Build()
	require.NoError(t, err)

	p := createTestPool(t, config)
	defer func() {
		require.NoError(t, p.Close())
	}()

	// Run stress test for 5 seconds
	start := time.Now()
	objects := make([]*TestObject, 1000)
	for time.Since(start) < 5*time.Second {
		// Get objects
		for i := range objects {
			objects[i], err = p.Get()
			require.NoError(t, err)
			require.NotNil(t, objects[i])
		}

		// Simulate work
		time.Sleep(10 * time.Millisecond)

		// Return objects
		for _, obj := range objects {
			err := p.Put(obj)
			require.NoError(t, err)
		}

		// Simulate work
		time.Sleep(10 * time.Millisecond)
	}

	// Verify pool is still functioning
	var obj *TestObject
	obj, err = p.Get()
	require.NoError(t, err)
	require.NotNil(t, obj)
	err = p.Put(obj)
	require.NoError(t, err)
}

func TestPerformanceFastPath(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(1000).
		SetHardLimit(10000).
		SetFastPathInitialSize(100).
		SetFastPathFillAggressiveness(1.0).
		Build()
	require.NoError(t, err)

	p := createTestPool(t, config)
	defer func() {
		require.NoError(t, p.Close())
	}()

	// Measure fast path performance
	start := time.Now()
	objects := make([]*TestObject, 100)
	for i := range objects {
		objects[i], err = p.Get()
		require.NoError(t, err)
		require.NotNil(t, objects[i])
	}
	fastPathDuration := time.Since(start)

	// Verify fast path performance is within acceptable limits
	assert.Less(t, fastPathDuration, 10*time.Millisecond)

	// Cleanup
	for _, obj := range objects {
		err := p.Put(obj)
		require.NoError(t, err)
	}
}
