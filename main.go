package main

import (
	"fmt"
	"log"
	"memctx/pool"
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

	fmt.Println(poolObj)
}
