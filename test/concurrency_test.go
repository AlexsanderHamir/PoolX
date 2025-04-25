package test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/AlexsanderHamir/memory_context/pool"
	"github.com/stretchr/testify/require"
)

func TestPoolConcurrency(t *testing.T) {
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

	numGoroutines := 50
	numOpsPerGoroutine := 200
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

func TestConcurrentClose(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(100).
		SetHardLimit(1000).
		SetVerbose(false).
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
			for range 10 {
				obj := p.Get()
				if obj != nil {
					time.Sleep(time.Millisecond)
					_ = p.Put(obj)
				}
			}
		}()
	}

	time.Sleep(50 * time.Millisecond)
	err = p.Close()
	require.NoError(t, err)

	wg.Wait()
}
