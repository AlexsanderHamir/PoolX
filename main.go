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
	numWorkers := 100
	objectsPerWorker := 10 // each goroutine holds 10 objects

	log.Println("[WORKLOAD] Starting goroutines")

	for i := range numWorkers {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			var objs []*Example
			for range objectsPerWorker {
				obj := poolObj.Get()
				obj.name = "user_" + randomString(5)
				obj.age = rand.Intn(100)
				obj.friends = make([]string, rand.Intn(5))
				for k := range obj.friends {
					obj.friends[k] = randomString(4)
				}
				objs = append(objs, obj)
			}

			time.Sleep(time.Duration(500+rand.Intn(500)) * time.Millisecond)

			for _, obj := range objs {
				poolObj.Put(obj)
			}
		}(i)
	}

	wg.Wait()
	log.Println("[DONE] All goroutines completed")

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
