package test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/AlexsanderHamir/memory_context/pool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPoolParallelism(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(100).
		SetMinShrinkCapacity(100).   // prevent shrinking
		SetRingBufferBlocking(true). // prevent nil returns
		SetHardLimit(1000).
		Build()
	require.NoError(t, err)

	p := createTestPool(t, config)
	defer func() {
		require.NoError(t, p.Close())
	}()

	numGoroutines := 150
	numOpsPerGoroutine := 20000
	reqNum := numGoroutines * numOpsPerGoroutine

	var wg sync.WaitGroup
	errCh := make(chan error, numGoroutines*numOpsPerGoroutine)

	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for range numOpsPerGoroutine {
				obj := p.Get()
				if obj == nil {
					errCh <- fmt.Errorf("goroutine %d: got nil object", id)
					return
				}

				time.Sleep(time.Microsecond * 100)

				if err := p.Put(obj); err != nil {
					errCh <- fmt.Errorf("goroutine %d: put error: %w", id, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	p.PrintPoolStats()

	for err := range errCh {
		require.NoError(t, err)
	}

	stats := p.GetPoolStatsSnapshot()
	require.NoError(t, stats.Validate(reqNum))
}

func TestConcurrentGrowth(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(10).
		SetHardLimit(1000).
		SetGrowthPercent(0.5).
		Build()
	require.NoError(t, err)

	p := createTestPool(t, config)
	defer func() {
		require.NoError(t, p.Close())
	}()

	numGoroutines := 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Create a burst of concurrent requests
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			objects := make([]*TestObject, 20)
			for j := range objects {
				objects[j] = p.Get()
				require.NotNil(t, objects[j])
			}
			for _, obj := range objects {
				err := p.Put(obj)
				require.NoError(t, err)
			}
		}()
	}

	wg.Wait()
	assert.True(t, p.IsGrowth())
}

func TestConcurrentShrink(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(100).
		SetHardLimit(1000).
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

	// First, fill the pool
	objects := make([]*TestObject, 50)
	for i := range objects {
		objects[i] = p.Get()
		require.NotNil(t, objects[i])
	}

	// Return all objects
	for _, obj := range objects {
		err := p.Put(obj)
		require.NoError(t, err)
	}

	// Wait for shrink to occur
	time.Sleep(100 * time.Millisecond)
	assert.True(t, p.IsShrunk())
}

func TestConcurrentClose(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(100).
		SetHardLimit(1000).
		Build()
	require.NoError(t, err)

	p := createTestPool(t, config)

	numGoroutines := 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Start concurrent operations
	for range numGoroutines {
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				obj := p.Get()
				if obj != nil {
					time.Sleep(time.Millisecond)
					_ = p.Put(obj)
				}
			}
		}()
	}

	// Close the pool while operations are ongoing
	time.Sleep(50 * time.Millisecond)
	err = p.Close()
	require.NoError(t, err)

	wg.Wait()
}
