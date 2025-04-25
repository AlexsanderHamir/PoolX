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
		SetInitialCapacity(1).
		SetMinShrinkCapacity(1).     // prevent shrinking
		SetRingBufferBlocking(true). // prevent nil returns
		SetHardLimit(100).
		SetRingBufferTimeout(3 * time.Second).
		Build()
	require.NoError(t, err)

	p := createTestPool(t, config)
	defer func() {
		require.NoError(t, p.Close())
	}()

	numGoroutines := 1000
	numOpsPerGoroutine := 1000
	reqNum := numGoroutines * numOpsPerGoroutine

	var wg sync.WaitGroup
	errCh := make(chan error, numGoroutines*numOpsPerGoroutine)

	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for range numOpsPerGoroutine {
				obj, err := p.Get()
				if err != nil {
					errCh <- fmt.Errorf("goroutine %d: got nil object, error: %w", id, err)
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

	for err := range errCh {
		require.NoError(t, err)
	}

	p.PrintPoolStats()
	stats := p.GetPoolStatsSnapshot()
	require.NoError(t, stats.Validate(reqNum))
}

// high number of goroutines and low objects in pool causes blocking and sometimes timeout

// func TestPoolConcurrency(t *testing.T) {
// 	config, err := pool.NewPoolConfigBuilder().
// 		SetInitialCapacity(3).
// 		SetMinShrinkCapacity(3).     // prevent shrinking
// 		SetRingBufferBlocking(true). // prevent nil returns
// 		SetHardLimit(150).
// 		Build()
// 	require.NoError(t, err)

// 	p := createTestPool(t, config)
// 	defer func() {
// 		require.NoError(t, p.Close())
// 	}()

// 	numGoroutines := 100
// 	numOpsPerGoroutine := 20
// 	reqNum := numGoroutines * numOpsPerGoroutine

// 	var wg sync.WaitGroup
// 	errCh := make(chan error, numGoroutines*numOpsPerGoroutine)

// 	for i := range numGoroutines {
// 		wg.Add(1)
// 		go func(id int) {
// 			defer wg.Done()
// 			for range numOpsPerGoroutine {
// 				obj, err := p.Get()
// 				if err != nil {
// 					errCh <- fmt.Errorf("goroutine %d: got nil object, error: %w", id, err)
// 					return
// 				}

// 				time.Sleep(time.Microsecond * 100)

// 				if err := p.Put(obj); err != nil {
// 					errCh <- fmt.Errorf("goroutine %d: put error: %w", id, err)
// 					return
// 				}
// 			}
// 		}(i)
// 	}

// 	wg.Wait()
// 	close(errCh)

// 	for err := range errCh {
// 		require.NoError(t, err)
// 	}

// 	stats := p.GetPoolStatsSnapshot()
// 	require.NoError(t, stats.Validate(reqNum))

// }

// func TestConcurrentClose(t *testing.T) {
// 	config, err := pool.NewPoolConfigBuilder().
// 		SetInitialCapacity(100).
// 		SetHardLimit(1000).
// 		SetVerbose(false).
// 		Build()
// 	require.NoError(t, err)

// 	p := createTestPool(t, config)

// 	numGoroutines := 50
// 	var wg sync.WaitGroup
// 	wg.Add(numGoroutines)

// 	for range numGoroutines {
// 		go func() {
// 			defer wg.Done()
// 			for range 10 {
// 				obj := p.Get()
// 				if obj != nil {
// 					time.Sleep(time.Millisecond)
// 					_ = p.Put(obj)
// 				}
// 			}
// 		}()
// 	}

// 	time.Sleep(50 * time.Millisecond)
// 	err = p.Close()
// 	require.NoError(t, err)

// 	wg.Wait()
// }

// // NOT READY
// func TestConcurrentGetPutWithGrowth(t *testing.T) {
// 	config, err := pool.NewPoolConfigBuilder().
// 		SetInitialCapacity(10).
// 		SetMinShrinkCapacity(10).
// 		SetRingBufferBlocking(true).
// 		SetHardLimit(1000).
// 		SetGrowthPercent(0.5).
// 		Build()
// 	require.NoError(t, err)

// 	p := createTestPool(t, config)
// 	defer func() {
// 		require.NoError(t, p.Close())
// 	}()

// 	numGoroutines := 100
// 	numOpsPerGoroutine := 50
// 	reqNum := numGoroutines * numOpsPerGoroutine

// 	var wg sync.WaitGroup
// 	errCh := make(chan error, numGoroutines*numOpsPerGoroutine)

// 	for i := range numGoroutines {
// 		wg.Add(1)
// 		go func(id int) {
// 			defer wg.Done()
// 			for range numOpsPerGoroutine {
// 				obj := p.Get()
// 				if obj == nil {
// 					errCh <- fmt.Errorf("goroutine %d: got nil object", id)
// 					return
// 				}

// 				time.Sleep(time.Microsecond * 50)

// 				if err := p.Put(obj); err != nil {
// 					errCh <- fmt.Errorf("goroutine %d: put error: %w", id, err)
// 					return
// 				}
// 			}
// 		}(i)
// 	}

// 	wg.Wait()
// 	close(errCh)

// 	p.PrintPoolStats()
// 	for err := range errCh {
// 		require.NoError(t, err)
// 	}

// 	stats := p.GetPoolStatsSnapshot()
// 	require.NoError(t, stats.Validate(reqNum))
// }

// func TestConcurrentGetPutWithShrink(t *testing.T) {
// 	config, err := pool.NewPoolConfigBuilder().
// 		SetInitialCapacity(100).
// 		SetMinShrinkCapacity(50).
// 		SetRingBufferBlocking(true).
// 		SetHardLimit(1000).
// 		SetShrinkPercent(0.5).
// 		SetMinUtilizationBeforeShrink(0.3).
// 		SetStableUnderutilizationRounds(2).
// 		Build()
// 	require.NoError(t, err)

// 	p := createTestPool(t, config)
// 	defer func() {
// 		require.NoError(t, p.Close())
// 	}()

// 	numGoroutines := 20
// 	numOpsPerGoroutine := 100
// 	reqNum := numGoroutines * numOpsPerGoroutine

// 	var wg sync.WaitGroup
// 	errCh := make(chan error, numGoroutines*numOpsPerGoroutine)

// 	for i := range numGoroutines {
// 		wg.Add(1)
// 		go func(id int) {
// 			defer wg.Done()
// 			for range numOpsPerGoroutine {
// 				obj := p.Get()
// 				if obj == nil {
// 					errCh <- fmt.Errorf("goroutine %d: got nil object", id)
// 					return
// 				}

// 				// Simulate some work
// 				time.Sleep(time.Millisecond)

// 				if err := p.Put(obj); err != nil {
// 					errCh <- fmt.Errorf("goroutine %d: put error: %w", id, err)
// 					return
// 				}
// 			}
// 		}(i)
// 	}

// 	wg.Wait()
// 	close(errCh)

// 	for err := range errCh {
// 		require.NoError(t, err)
// 	}

// 	stats := p.GetPoolStatsSnapshot()
// 	require.NoError(t, stats.Validate(reqNum))
// }

// func TestConcurrentGetPutWithL1Cache(t *testing.T) {
// 	config, err := pool.NewPoolConfigBuilder().
// 		SetInitialCapacity(50).
// 		SetMinShrinkCapacity(50).
// 		SetRingBufferBlocking(true).
// 		SetHardLimit(1000).
// 		SetFastPathRefillPercent(0.8).
// 		Build()
// 	require.NoError(t, err)

// 	p := createTestPool(t, config)
// 	defer func() {
// 		require.NoError(t, p.Close())
// 	}()

// 	numGoroutines := 30
// 	numOpsPerGoroutine := 200
// 	reqNum := numGoroutines * numOpsPerGoroutine

// 	var wg sync.WaitGroup
// 	errCh := make(chan error, numGoroutines*numOpsPerGoroutine)

// 	for i := range numGoroutines {
// 		wg.Add(1)
// 		go func(id int) {
// 			defer wg.Done()
// 			for range numOpsPerGoroutine {
// 				obj := p.Get()
// 				if obj == nil {
// 					errCh <- fmt.Errorf("goroutine %d: got nil object", id)
// 					return
// 				}

// 				// Simulate some work
// 				time.Sleep(time.Microsecond * 100)

// 				if err := p.Put(obj); err != nil {
// 					errCh <- fmt.Errorf("goroutine %d: put error: %w", id, err)
// 					return
// 				}
// 			}
// 		}(i)
// 	}

// 	wg.Wait()
// 	close(errCh)

// 	for err := range errCh {
// 		require.NoError(t, err)
// 	}

// 	stats := p.GetPoolStatsSnapshot()
// 	require.NoError(t, stats.Validate(reqNum))
// }

// func TestConcurrentGetPutWithBlocking(t *testing.T) {
// 	config, err := pool.NewPoolConfigBuilder().
// 		SetInitialCapacity(10).
// 		SetMinShrinkCapacity(10).
// 		SetRingBufferBlocking(true).
// 		SetHardLimit(20).
// 		Build()
// 	require.NoError(t, err)

// 	p := createTestPool(t, config)
// 	defer func() {
// 		require.NoError(t, p.Close())
// 	}()

// 	numGoroutines := 50
// 	numOpsPerGoroutine := 100
// 	reqNum := numGoroutines * numOpsPerGoroutine

// 	var wg sync.WaitGroup
// 	errCh := make(chan error, numGoroutines*numOpsPerGoroutine)

// 	for i := range numGoroutines {
// 		wg.Add(1)
// 		go func(id int) {
// 			defer wg.Done()
// 			for range numOpsPerGoroutine {
// 				obj := p.Get()
// 				if obj == nil {
// 					errCh <- fmt.Errorf("goroutine %d: got nil object", id)
// 					return
// 				}

// 				// Simulate some work
// 				time.Sleep(time.Millisecond)

// 				if err := p.Put(obj); err != nil {
// 					errCh <- fmt.Errorf("goroutine %d: put error: %w", id, err)
// 					return
// 				}
// 			}
// 		}(i)
// 	}

// 	wg.Wait()
// 	close(errCh)

// 	for err := range errCh {
// 		require.NoError(t, err)
// 	}

// 	stats := p.GetPoolStatsSnapshot()
// 	require.NoError(t, stats.Validate(reqNum))
// }

// func TestConcurrentGetPutWithCleaner(t *testing.T) {
// 	config, err := pool.NewPoolConfigBuilder().
// 		SetInitialCapacity(50).
// 		SetMinShrinkCapacity(50).
// 		SetRingBufferBlocking(true).
// 		SetHardLimit(1000).
// 		Build()
// 	require.NoError(t, err)

// 	cleanerCalled := atomic.Int32{}
// 	cleaner := func(obj interface{}) {
// 		cleanerCalled.Add(1)
// 	}

// 	p := createTestPoolWithCleaner(t, config, cleaner)
// 	defer func() {
// 		require.NoError(t, p.Close())
// 	}()

// 	numGoroutines := 30
// 	numOpsPerGoroutine := 200
// 	reqNum := numGoroutines * numOpsPerGoroutine

// 	var wg sync.WaitGroup
// 	errCh := make(chan error, numGoroutines*numOpsPerGoroutine)

// 	for i := range numGoroutines {
// 		wg.Add(1)
// 		go func(id int) {
// 			defer wg.Done()
// 			for range numOpsPerGoroutine {
// 				obj := p.Get()
// 				if obj == nil {
// 					errCh <- fmt.Errorf("goroutine %d: got nil object", id)
// 					return
// 				}

// 				// Simulate some work
// 				time.Sleep(time.Microsecond * 100)

// 				if err := p.Put(obj); err != nil {
// 					errCh <- fmt.Errorf("goroutine %d: put error: %w", id, err)
// 					return
// 				}
// 			}
// 		}(i)
// 	}

// 	wg.Wait()
// 	close(errCh)

// 	for err := range errCh {
// 		require.NoError(t, err)
// 	}

// 	stats := p.GetPoolStatsSnapshot()
// 	require.NoError(t, stats.Validate(reqNum))
// 	require.Equal(t, int32(reqNum), cleanerCalled.Load())
// }

// func TestConcurrentGetPutWithStats(t *testing.T) {
// 	config, err := pool.NewPoolConfigBuilder().
// 		SetInitialCapacity(50).
// 		SetMinShrinkCapacity(50).
// 		SetRingBufferBlocking(true).
// 		SetHardLimit(1000).
// 		SetEnableStats(true).
// 		Build()
// 	require.NoError(t, err)

// 	p := createTestPool(t, config)
// 	defer func() {
// 		require.NoError(t, p.Close())
// 	}()

// 	numGoroutines := 30
// 	numOpsPerGoroutine := 200
// 	reqNum := numGoroutines * numOpsPerGoroutine

// 	var wg sync.WaitGroup
// 	errCh := make(chan error, numGoroutines*numOpsPerGoroutine)

// 	for i := range numGoroutines {
// 		wg.Add(1)
// 		go func(id int) {
// 			defer wg.Done()
// 			for range numOpsPerGoroutine {
// 				obj := p.Get()
// 				if obj == nil {
// 					errCh <- fmt.Errorf("goroutine %d: got nil object", id)
// 					return
// 				}

// 				// Simulate some work
// 				time.Sleep(time.Microsecond * 100)

// 				if err := p.Put(obj); err != nil {
// 					errCh <- fmt.Errorf("goroutine %d: put error: %w", id, err)
// 					return
// 				}
// 			}
// 		}(i)
// 	}

// 	wg.Wait()
// 	close(errCh)

// 	for err := range errCh {
// 		require.NoError(t, err)
// 	}

// 	stats := p.GetPoolStatsSnapshot()
// 	require.NoError(t, stats.Validate(reqNum))
// 	require.Equal(t, uint64(reqNum), stats.TotalGets)
// 	require.Equal(t, uint64(reqNum), stats.FastReturnHit+stats.FastReturnMiss)
// }

// func createTestPoolWithCleaner(t *testing.T, config *pool.PoolConfig, cleaner func(interface{})) *pool.Pool[interface{}] {
// 	p, err := pool.NewPool(config, func() interface{} { return new(interface{}) }, cleaner, reflect.TypeOf((*interface{})(nil)))
// 	require.NoError(t, err)
// 	return p
// }
