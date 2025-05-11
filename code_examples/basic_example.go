package code_examples

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/AlexsanderHamir/PoolX/code_examples/configs"
	"github.com/AlexsanderHamir/PoolX/pool"
)


func RunBasicExample() error {
	config := configs.CreateHighThroughputConfig()
	pool, err := pool.NewPool(
		config,
		func() *configs.Example { return &configs.Example{ID: 23, Name: "test"} },
		func(obj *configs.Example) {
			obj.ID = 0
			obj.Name = ""
		},
	)
	if err != nil {
		return err
	}
	defer pool.Close()


	const numWorkers = 2000
	const objectsPerWorker = 100

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, numWorkers)
	var wg sync.WaitGroup

	for i := range numWorkers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := range objectsPerWorker {
				select {
				case <-ctx.Done():
					return
				default:
					obj, err := pool.Get()
					if err != nil {
						errChan <- fmt.Errorf("worker %d failed to get object: %w", workerID, err)
						cancel()
						return
					}
					obj.ID = workerID*objectsPerWorker + j
					obj.Name = fmt.Sprintf("Worker-%d-Object-%d", workerID, j)

					time.Sleep(time.Duration(rand.Float64() * float64(time.Millisecond*400)))
					if err := pool.Put(obj); err != nil {
						errChan <- fmt.Errorf("worker %d failed to put object: %w", workerID, err)
						cancel()
						return
					}
				}
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	if err := <-errChan; err != nil {
		return err
	}

	pool.PrintPoolStats()
	return nil
}
