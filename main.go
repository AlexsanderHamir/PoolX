package main

import (
	"log"
	"math/rand"
	"memctx/pool"
	"sync"
	"time"
)

type Example struct {
	name    string
	age     int
	friends []string
}

func main() {
	allocator := func() *Example {
		return &Example{}
	}

	cleaner := func(e *Example) {
		e.name = ""
		e.age = 0
		e.friends = nil
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

	var wg sync.WaitGroup

	numBursts := 5
	burstSize := 20                  // Number of workers per burst
	objectsPerWorker := 10           // Each worker does 10 tasks
	burstInterval := 2 * time.Second // Idle time between bursts

	log.Println("[WORKLOAD] Starting bursty load test")

	for burst := 0; burst < numBursts; burst++ {
		log.Printf("[BURST %d] Launching %d workers", burst+1, burstSize)

		for i := 0; i < burstSize; i++ {
			wg.Add(1)

			go func(workerID int) {
				defer wg.Done()

				for j := 0; j < objectsPerWorker; j++ {
					obj := poolObj.Get()
					obj.name = "user_" + randomString(5)
					obj.age = rand.Intn(100)
					obj.friends = make([]string, rand.Intn(5))
					for k := range obj.friends {
						obj.friends[k] = randomString(4)
					}

					time.Sleep(50 * time.Millisecond)
					poolObj.Put(obj)
				}
			}(i + burst*burstSize)
		}

		time.Sleep(burstInterval)
	}

	wg.Wait()
	log.Println("[DONE] Bursty load goroutines completed")

	select {}
}

func randomString(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyz")
	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}
