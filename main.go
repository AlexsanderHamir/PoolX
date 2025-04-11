package main

import (
	"fmt"
	"log"
	"memctx/pool"
	"net/http"
	_ "net/http/pprof" // Import pprof
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

func main() {
	enableProfiling()

	debug.SetGCPercent(-1)

	fmt.Println("[PPROF] Ready to profile at http://localhost:6060/debug/pprof/")
	time.Sleep(5 * time.Second)

	runWorkload()

	fmt.Println("[DONE] Workload finished")
	time.Sleep(30 * time.Second)
}

func enableProfiling() {
	runtime.SetMutexProfileFraction(1)
	runtime.SetBlockProfileRate(1)

	go func() {
		log.Println("[PPROF] Server running on :6060")
		http.ListenAndServe("localhost:6060", nil)
	}()
}

func runWorkload() {

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
		log.Fatalf("Failed to create pool: %v", err)
	}

	numWorkers := 5
	objectsPerWorker := 10000
	delayBetweenTasks := 100 * time.Millisecond

	log.Println("[WORKLOAD] Starting")

	var innerWg sync.WaitGroup
	for i := range numWorkers {
		innerWg.Add(1)
		go func(id int) {
			defer innerWg.Done()
			for range objectsPerWorker {
				obj := poolObj.Get()
				obj.Name = "user1"
				obj.Age = 120
				time.Sleep(50 * time.Millisecond)
				poolObj.Put(obj)
				time.Sleep(delayBetweenTasks)
			}
		}(i)
	}
	innerWg.Wait()
	log.Println("[WORKLOAD] All done")
}
