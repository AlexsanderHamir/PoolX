package main

import (
	"log"
	"math/rand"
	"memctx/pool"
	"net/http"
	_ "net/http/pprof" // Import pprof
	"runtime"
	"sync"
	"time"
)

func main() {
	enableProfiling()
	runWorkloadForever()

	select {}
}

func enableProfiling() {
	runtime.SetMutexProfileFraction(1)
	runtime.SetBlockProfileRate(1)

	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()
}

func runWorkloadForever() {
	allocator := func() *pool.Example {
		return &pool.Example{}
	}

	cleaner := func(e *pool.Example) {
		e.Name = ""
		e.Age = 0
	}

	config, err := pool.NewPoolConfigBuilder().
		SetShrinkAggressiveness(pool.AggressivenessExtreme).
		Build()

	if err != nil {
		log.Fatalf("Failed to build pool config: %v", err)
	}

	poolObj, err := pool.NewPool(config, allocator, cleaner)
	if err != nil {
		panic(err)
	}

	numWorkers := 1000
	objectsPerWorker := 5

	log.Println("[WORKLOAD] Starting mixed workload test (infinite mode)")

	for {
		var wg sync.WaitGroup

		for i := range numWorkers {
			wg.Add(1)

			go func(workerID int) {
				defer wg.Done()

				time.Sleep(time.Duration(rand.Intn(20)) * time.Millisecond)

				for j := 0; j < objectsPerWorker; j++ {
					obj := poolObj.Get()

					obj.Name = "user1"
					obj.Age = 140

					time.Sleep(time.Duration(rand.Intn(30)+10) * time.Millisecond)

					poolObj.Put(obj)
				}
			}(i)
		}

		wg.Wait()
	}
}
