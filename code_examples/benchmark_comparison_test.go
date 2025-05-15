package code_examples

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/AlexsanderHamir/PoolX/v2/code_examples/configs"
	"github.com/AlexsanderHamir/PoolX/v2/pool"
)

func performWorkload(obj *configs.Example) {
	// Simulate CPU-intensive work
	for range 1000 {
		obj.Data = append(obj.Data, byte(rand.Intn(256)))
	}
	// Simulate some I/O or network delay
	time.Sleep(time.Microsecond * 100)
}

// go test -run=^$ -bench=^BenchmarkPoolXHighContention$ -benchmem -cpuprofile=cpu.out -memprofile=mem.out -trace=trace.out -mutexprofile=mutex.out

func BenchmarkPoolXHighContention(b *testing.B) {
	config := configs.CreateHighThroughputConfig()
	pool, err := pool.NewPool(
		config,
		func() *configs.Example {
			return &configs.Example{
				Data: make([]byte, 1024),
			}
		},
		func(obj *configs.Example) {
			obj.ID = 0
			obj.Name = ""
			obj.Data = obj.Data[:0]
		},
		func(obj *configs.Example) *configs.Example {
			dst := *obj
			return &dst
		},
	)
	if err != nil {
		b.Fatal(err)
	}
	defer pool.Close()

	const numGoroutines = 1000
	workpergoroutine := b.N / numGoroutines
	var wg sync.WaitGroup
	start := make(chan struct{})

	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start

			for range workpergoroutine {
				obj, err := pool.Get()
				if err != nil {
					panic(err)
				}
				obj.ID = rand.Intn(1000)
				obj.Name = "test"
				obj.Data = append(obj.Data, byte(rand.Intn(256)))

				performWorkload(obj)

				if err := pool.Put(obj); err != nil {
					panic(err)
				}
			}
		}()
	}

	b.ResetTimer()
	close(start)
	wg.Wait()
}

// go test -run=^$ -bench=^BenchmarkSyncPoolHighContention$ -benchmem -cpuprofile=cpu.out -memprofile=mem.out -trace=trace.out -mutexprofile=mutex.out

func BenchmarkSyncPoolHighContention(b *testing.B) {
	var syncPool = sync.Pool{
		New: func() any {
			return &configs.Example{
				Data: make([]byte, 1024),
			}
		},
	}

	const numGoroutines = 1000
	workpergoroutine := b.N / numGoroutines
	var wg sync.WaitGroup
	start := make(chan struct{})

	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start

			for range workpergoroutine {
				obj := syncPool.Get().(*configs.Example)

				obj.ID = rand.Intn(1000)
				obj.Name = "test"
				obj.Data = append(obj.Data, byte(rand.Intn(256)))

				performWorkload(obj)

				// Clean up before returning
				obj.ID = 0
				obj.Name = ""
				obj.Data = obj.Data[:0]

				syncPool.Put(obj)
			}
		}()
	}

	b.ResetTimer()
	close(start)
	wg.Wait()
}
