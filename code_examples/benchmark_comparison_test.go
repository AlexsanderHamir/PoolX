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
	obj.Name = "test"
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

			performWorkload(obj)

			if err := pool.Put(obj); err != nil {
				b.Fatal(err)
			}
		}
	})
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

	b.SetParallelism(1000)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			obj := syncPool.Get().(*configs.Example)

			performWorkload(obj)

			obj.Name = ""
			obj.Data = obj.Data[:0]

			syncPool.Put(obj)
		}
	})
}
