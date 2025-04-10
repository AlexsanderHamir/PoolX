package pool

import (
	"math/rand"
	"sync"
	"testing"
	"time"
)


func setupPool(b *testing.B) *pool[*Example] {

	allocator := func() *Example {
		return &Example{}
	}

	cleaner := func(e *Example) {
		e.Name = ""
		e.Age = 0
	}

	config, err := NewPoolConfigBuilder().
		SetShrinkAggressiveness(AggressivenessExtreme).
		Build()
	if err != nil {
		b.Fatalf("Failed to build pool config: %v", err)
	}

	p, err := NewPool(config, allocator, cleaner)
	if err != nil {
		b.Fatalf("Failed to create pool: %v", err)
	}
	return p
}

// Workload:  High concurrency
// Heavy I/O work
// Heavy CPU work
// Random arrival of goroutines
// Random return of objects (based on heavy I/O, no extra latency)
func BenchmarkPool_PUT_GET(b *testing.B) {
	b.StopTimer()
	poolObj := setupPool(b)
	b.StartTimer()

	numWorkers := 1000
	objectsPerWorker := 5

	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup

		for w := range numWorkers {
			wg.Add(1)

			go func(workerID int) {
				defer wg.Done()

				// Simulate random arrival
				time.Sleep(time.Duration(rand.Intn(20)) * time.Millisecond)

				for range objectsPerWorker {
					obj := poolObj.Get()
					obj.Name = "user_1"
					obj.Age = 140

					// Simulate I/O-bound latency
					time.Sleep(time.Duration(rand.Intn(30)+10) * time.Millisecond)

					poolObj.Put(obj)
				}
			}(w)
		}

		wg.Wait()
	}
}
