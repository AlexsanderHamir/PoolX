package code_examples

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/AlexsanderHamir/PoolX/code_examples/configs"
	"github.com/AlexsanderHamir/PoolX/src/pool"
)

type TestObject struct {
	ID   int
	Name string
	Data []byte
}

func performWorkload(obj *TestObject) {
	// Simulate CPU-intensive work
	for range 1000 {
		obj.Data = append(obj.Data, byte(rand.Intn(256)))
	}
	// Simulate some I/O or network delay
	time.Sleep(time.Microsecond * 50)
}

func BenchmarkPoolX(b *testing.B) {
	config := configs.CreateHighThroughputConfig()
	pool, err := pool.NewPool(
		config,
		func() *TestObject {
			return &TestObject{
				ID:   0,
				Name: "",
				Data: make([]byte, 1024),
			}
		},
		func(obj *TestObject) {
			obj.ID = 0
			obj.Name = ""
			obj.Data = obj.Data[:0]
		},
	)
	if err != nil {
		b.Fatal(err)
	}
	defer pool.Close()

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
			return &TestObject{
				ID:   0,
				Name: "",
				Data: make([]byte, 1024),
			}
		},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			obj := syncPool.Get().(*TestObject)

			obj.ID = rand.Intn(1000)
			obj.Name = "test"
			obj.Data = append(obj.Data[:0], byte(rand.Intn(256)))

			performWorkload(obj)

			syncPool.Put(obj)
		}
	})
}

// Benchmark with higher contention
func BenchmarkPoolXHighContention(b *testing.B) {
	config := configs.CreateHighThroughputConfig()
	pool, err := pool.NewPool(
		config,
		func() *TestObject {
			return &TestObject{
				ID:   0,
				Name: "",
				Data: make([]byte, 1024),
			}
		},
		func(obj *TestObject) {
			obj.ID = 0
			obj.Name = ""
			obj.Data = obj.Data[:0]
		},
	)
	if err != nil {
		b.Fatal(err)
	}
	defer pool.Close()

	const numGoroutines = 1000
	var wg sync.WaitGroup

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(numGoroutines)
		for range numGoroutines {
			go func() {
				defer wg.Done()
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
			}()
		}
		wg.Wait()
	}
}

func BenchmarkSyncPoolHighContention(b *testing.B) {
	var syncPool = sync.Pool{
		New: func() any {
			return &TestObject{
				ID:   0,
				Name: "",
				Data: make([]byte, 1024),
			}
		},
	}

	const numGoroutines = 1000
	var wg sync.WaitGroup

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(numGoroutines)
		for range numGoroutines {
			go func() {
				defer wg.Done()
				obj := syncPool.Get().(*TestObject)

				obj.ID = rand.Intn(1000)
				obj.Name = "test"
				obj.Data = append(obj.Data[:0], byte(rand.Intn(256)))

				performWorkload(obj)

				syncPool.Put(obj)
			}()
		}
		wg.Wait()
	}
}
