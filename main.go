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

	config, err := pool.NewPoolConfigBuilder().
		EnforceCustomConfig().
		SetInitialCapacity(64).
		SetShrinkCheckInterval(2 * time.Second).
		SetIdleThreshold(3 * time.Second).
		SetMinIdleBeforeShrink(2).
		SetShrinkCooldown(5 * time.Second).
		SetMinUtilizationBeforeShrink(0.3).
		SetStableUnderutilizationRounds(2).
		SetShrinkPercent(0.25).
		SetMinShrinkCapacity(16).
		SetMaxConsecutiveShrinks(3).
		Build()

	if err != nil {
		log.Fatalf("Failed to build pool config: %v", err)
	}

	poolObj, err := pool.NewPool(config, allocator, cleaner)
	if err != nil {
		panic(err)
	}

	var objs []any
	for range 30 {
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
