package main

import (
	"log"
	"memctx/pool"
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

	var objs []*Example
	for range 120 {
		obj := poolObj.Get()
		objs = append(objs, obj)
	}

	time.Sleep(10 * time.Second)

	log.Println("[SHRINK] putting objs back")
	for _, obj := range objs {
		poolObj.Put(obj)
	}

	select {}
}
