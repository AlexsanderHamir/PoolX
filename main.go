package main

import (
	"fmt"
	"memctx/pool"
)

type Example struct {
	name    string
	age     int
	friends []string
}

func main() {
	allocator := func() any {
		return &Example{}
	}

	cleaner := func(obj any) {
		if e, ok := obj.(*Example); ok {
			e.name = ""
			e.age = 0
			e.friends = nil
		}
	}

	poolObj, err := pool.NewPool(nil, allocator, cleaner)
	if err != nil {
		panic(err)
	}

	for range 1 {
		obj := poolObj.Get()
		fmt.Println(obj)
		fmt.Println(poolObj.Stats.CurrentCapacity)
	}
}
