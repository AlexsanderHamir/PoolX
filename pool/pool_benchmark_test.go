package pool

import (
	"fmt"
	"runtime/debug"
	"testing"
	"time"
)

var allocator = func() *Example {
	return &Example{}
}

var cleaner = func(e *Example) {
	e.Name = ""
	e.Age = 0
}

func setupPool(b *testing.B) *pool[*Example] {
	config, err := NewPoolConfigBuilder().
		SetShrinkAggressiveness(AggressivenessExtreme).
		Build()
	if err != nil {
		b.Fatalf("Failed to build pool config: %v", err)
	}

	p, err := NewPool(config, allocator, cleaner)
	if err != nil {
		b.Fatalf("Failed to create pool: %v", err)
	}
	return p
}

func BenchmarkPool_Setup(b *testing.B) {
	debug.SetGCPercent(-1)
	b.ReportAllocs()
	InitDefaultFields()

	for i := 0; i < b.N; i++ {
		poolObj := setupPool(b)
		_ = poolObj
	}
}

// POOL CONFIG
//  defaultMinCapacity                                    = 8
//	defaultPoolCapacity                                   = 8
//	defaultHardLimit                                      = 1024
//	defaultHardLimitBufferSize                            = 1024
//	defaultGrowthEventsTrigger                            = 1
//	defaultL1MinCapacity                                  = defaultPoolCapacity

//EXPERIMENT CONFIG
// holdTimes := []time.Duration{1 * time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond}
// The pool is being forced to grow as much as possible, small hold times yeild better results.

func BenchmarkPool_Growth(b *testing.B) {
	debug.SetGCPercent(-1)
	b.ReportAllocs()
	InitDefaultFields()

	holdTimes := []time.Duration{1 * time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond}
	for _, holdTime := range holdTimes {
		b.Run(fmt.Sprintf("hold_%v", holdTime), func(b *testing.B) {
			b.SetParallelism(10)
			poolObj := setupPool(b)
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					obj := poolObj.Get()
					time.Sleep(holdTime)
					poolObj.Put(obj)
				}
			})
		})
	}
}

// POOL CONFIG
// defaultExponentialThresholdFactor                     = 100.0
// defaultGrowthPercent                                  = 0.95
// defaultFixedGrowthFactor                              = 2.0
// fillAggressivenessExtreme                             = 1.0 // 100%
// defaultRefillPercent                                  = 0.95 // 95%
//  defaultMinCapacity                                    = 64
//	defaultPoolCapacity                                   = 64
//	defaultHardLimit                                      = 20480
//	defaultHardLimitBufferSize                            = 20480
//	defaultGrowthEventsTrigger                            = 1
//	defaultL1MinCapacity                                  = defaultPoolCapacity
// defaultEnableChannelGrowth                            = false

//EXPERIMENT
// Making 100 get requests without any return puts a lot of pressure on get and its methods,
//  specially on the refill mode since the channel can't grow

func BenchmarkPool_tryRefillIfNeeded(b *testing.B) {
	debug.SetGCPercent(-1)
	b.ReportAllocs()
	InitDefaultFields()

	holdTimes := []time.Duration{5 * time.Millisecond, 10 * time.Millisecond, 15 * time.Millisecond}
	for _, holdTime := range holdTimes {
		b.Run(fmt.Sprintf("hold_%v", holdTime), func(b *testing.B) {
			poolObj := setupPool(b)
			b.SetParallelism(10)
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					var objs []*Example
					for range 100 {
						objs = append(objs, poolObj.Get())
					}

					time.Sleep(holdTime)

					for _, obj := range objs {
						poolObj.Put(obj)
					}
				}
			})
		})
	}
}
