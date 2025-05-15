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

func BenchmarkPoolX(b *testing.B) {
	config := configs.CreateHighThroughputConfig()
	pool, err := pool.NewPool(
		config,
		func() *configs.Example {
			return &configs.Example{
				ID:   0,
				Name: "",
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

	b.SetParallelism(1000)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			obj, err := pool.Get()
			if err != nil {
				b.Fatal(err)
			}

			// Simulate some work
			obj.ID = rand.Intn(1000)
			obj.Name = "test"
			obj.Data = append(obj.Data, byte(rand.Intn(256)))

			// create a function that performs a real workload
			performWorkload(obj)

			if err := pool.Put(obj); err != nil {
				b.Fatal(err)
			}
		}
	})

}

func BenchmarkSyncPool(b *testing.B) {
	var syncPool = sync.Pool{
		New: func() any {
			return &configs.Example{
				ID:   0,
				Name: "",
				Data: make([]byte, 1024),
			}
		},
	}

	b.SetParallelism(1000)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			obj := syncPool.Get().(*configs.Example)

			obj.ID = rand.Intn(1000)
			obj.Name = "test"
			obj.Data = append(obj.Data, byte(rand.Intn(256)))

			performWorkload(obj)

			obj.ID = 0
			obj.Name = ""
			obj.Data = obj.Data[:0]

			syncPool.Put(obj)
		}
	})
}
