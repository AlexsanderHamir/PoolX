package main

import (
	"fmt"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"reflect"
	"sync"
	"time"

	"github.com/AlexsanderHamir/memory_context/pool"
)

type Example struct {
	ID   int
	Name string
}

var allocator = func() *Example {
	return &Example{
		ID:   0,
		Name: "",
	}
}

var cleaner = func(obj *Example) {
	obj.ID = 0
	obj.Name = ""
}

func main() {
	numWorkers := 50
	objectsPerWorker := 5

	// Start pprof HTTP server
	go func() {
		fmt.Println("Starting pprof server on :6060")
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			fmt.Printf("pprof server error: %v\n", err)
		}
	}()

	poolConfig, err := pool.NewPoolConfigBuilder().
		SetPoolBasicConfigs(64, 10000, true, true).
		SetRingBufferBasicConfigs(true, 0, 0, time.Second*10).
		SetRingBufferGrowthConfigs(1000.0, 0.75, 1.0).
		SetRingBufferShrinkConfigs(time.Second*2, time.Second*10, time.Second*10, 3, 3, 8, 10, 0.5, 0.5).
		SetFastPathBasicConfigs(64, 2, 2, 1.0, 0.10).
		SetFastPathGrowthConfigs(1000.0, 1.0, 0.75).
		SetFastPathShrinkConfigs(0.5, 8).
		Build()
	if err != nil {
		panic(err)
	}

	pool, err := pool.NewPool(poolConfig, allocator, cleaner, reflect.TypeOf(&Example{}))
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	wg.Add(numWorkers)

	activeObjects := make(chan struct{}, numWorkers*objectsPerWorker)

	for i := range numWorkers {
		go func(workerID int) {
			defer wg.Done()
			counter := 0
			objects := make([]*Example, objectsPerWorker)

			for {
				for j := range objectsPerWorker {
					objects[j] = pool.Get()
					objects[j].ID = counter + j
					objects[j].Name = fmt.Sprintf("Worker-%d-Test-%d", workerID, counter+j)
					activeObjects <- struct{}{}
				}

				time.Sleep(time.Duration(rand.Float64() * float64(time.Second*2)))

				for j := range objectsPerWorker {
					pool.Put(objects[j])
					<-activeObjects
				}

				counter += objectsPerWorker
			}
		}(i)
	}

	go func() {
		for {
			time.Sleep(time.Second)
			fmt.Printf("Active objects: %d/%d\n", len(activeObjects), cap(activeObjects))
		}
	}()

	select {}
}
