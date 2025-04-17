package pool

import (
	"fmt"
	"reflect"
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

func setupPool(b *testing.B, config PoolConfig) *Pool[*Example] {
	if config == nil {
		builder, err := NewPoolConfigBuilder().
			SetShrinkAggressiveness(AggressivenessExtreme)
		if err != nil {
			b.Fatalf("Failed to set shrink aggressiveness: %v", err)
		}
		config, err = builder.
			SetVerbose(true).
			Build()
		if err != nil {
			b.Fatalf("Failed to build pool config: %v", err)
		}
	}

	p, err := NewPool(ToInternalConfig(config), allocator, cleaner, reflect.TypeOf(&Example{}))
	if err != nil {
		b.Fatalf("Failed to create pool: %v", err)
	}
	return p
}

func Benchmark_Setup(b *testing.B) {
	debug.SetGCPercent(-1)
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		poolObj := setupPool(b, nil)
		_ = poolObj
	}
}

// POOL CONFIG
// fillAggressivenessExtreme                             = 1.0
// defaultRefillPercent                                  = 0.10
// defaultMinCapacity                                    = 128
// defaultPoolCapacity                                   = 128

// defaultHardLimit                                      = 10_000_000
// defaultHardLimitBufferSize                            = 10_000_000
// defaultGrowthEventsTrigger                            = 3
// defaultEnableChannelGrowth                            = true
// defaultExponentialThresholdFactor                     = 100.0
// defaultGrowthPercent                                  = 0.25
// defaultFixedGrowthFactor                              = 2.0

func Benchmark_Get(b *testing.B) {
	debug.SetGCPercent(-1)
	b.ReportAllocs()

	b.SetParallelism(10)

	config, err := NewPoolConfigBuilder().
		SetInitialCapacity(128).   // defaultPoolCapacity
		SetHardLimit(10_000_000).  // defaultHardLimit
		SetMinShrinkCapacity(128). // defaultMinCapacity
		SetFastPathEnableChannelGrowth(true).
		SetFastPathFillAggressiveness(1.0). // fillAggressivenessExtreme
		SetFastPathRefillPercent(0.10).     // defaultRefillPercent
		SetFastPathGrowthEventsTrigger(3).
		SetFastPathGrowthPercent(0.75).
		SetFastPathExponentialThresholdFactor(1000.0).
		SetFastPathFixedGrowthFactor(1.0).
		Build()
	if err != nil {
		b.Fatalf("Failed to create custom config: %v", err)
	}

	poolObj := setupPool(b, config)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			obj := poolObj.Get()
			_ = obj
		}
	})
}

// POOL CONFIG
// fillAggressivenessExtreme                             = 1.0
// defaultRefillPercent                                  = 0.10
// defaultMinCapacity                                    = 128
// defaultPoolCapacity                                   = 128

// defaultHardLimit                                      = 10_000_000
// defaultHardLimitBufferSize                            = 10_000_000
func Benchmark_Put(b *testing.B) {
	debug.SetGCPercent(-1)
	b.ReportAllocs()

	b.SetParallelism(10)
	poolObj := setupPool(b, nil)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			poolObj.Put(&Example{})
		}
	})
}

func Benchmark_Grow(b *testing.B) {
	debug.SetGCPercent(-1)
	b.ReportAllocs()

	poolObj := setupPool(b, nil)

	for i := 0; i < b.N; i++ {
		poolObj.grow(time.Now())
	}
}

// POOL CONFIG
// defaultPoolCapacity                                   = 10_000_000
// defaultHardLimit                                      = 10_000_000

func Benchmark_SlowPath(b *testing.B) {
	debug.SetGCPercent(-1)
	b.ReportAllocs()

	poolObj := setupPool(b, nil)
	b.SetParallelism(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			obj := poolObj.slowPath()
			_ = obj
		}
	})
}

// func Benchmark_RepeatedShrink(b *testing.B) {
// 	debug.SetGCPercent(-1)
// 	b.ReportAllocs()
//
// 	poolObj := setupPool(b)

// 	for range 2_000_000 {
// 		poolObj.Put(poolObj.allocator())
// 	}

// 	prevCap := len(poolObj.pool)
// 	minCap := int(poolObj.config.shrink.minCapacity)

// 	for {
// 		inUse := 0
// 		newCap := prevCap - 10000
// 		if newCap < minCap {
// 			break
// 		}

// 		poolObj.performShrink(newCap, inUse, uint64(prevCap))

// 		newLen := len(poolObj.pool)
// 		if newLen >= prevCap {
// 			break
// 		}
// 		prevCap = newLen
// 	}
// }

func createCustomBenchmarkConfig() PoolConfig {
	config, err := NewPoolConfigBuilder().
		SetInitialCapacity(defaultPoolCapacity).
		SetHardLimit(defaultHardLimit).
		SetMinShrinkCapacity(defaultMinCapacity).
		SetFastPathFillAggressiveness(fillAggressivenessExtreme).
		SetFastPathRefillPercent(defaultRefillPercent).
		SetFastPathEnableChannelGrowth(defaultEnableChannelGrowth).
		SetFastPathGrowthEventsTrigger(defaultGrowthEventsTrigger).
		SetVerbose(true).
		Build()
	if err != nil {
		panic(fmt.Sprintf("Failed to create custom benchmark config: %v", err))
	}
	return config
}
