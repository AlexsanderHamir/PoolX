package main

import (
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"time"

	"github.com/AlexsanderHamir/memory_context/codeConfigExamples/configs"
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
	config := configs.CreateHighThroughputConfig()

	poolType := reflect.TypeOf(&Example{})

	pool, err := pool.NewPool(config, allocator, cleaner, poolType)
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

	select {}
}
