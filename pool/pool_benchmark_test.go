package pool

import (
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
		SetVerbose(true).
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

func Benchmark_Setup(b *testing.B) {
	debug.SetGCPercent(-1)
	b.ReportAllocs()
	InitDefaultFields()

	for i := 0; i < b.N; i++ {
		poolObj := setupPool(b)
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

func Benchmark_Get(b *testing.B) {
	debug.SetGCPercent(-1)
	b.ReportAllocs()
	InitDefaultFields()

	b.SetParallelism(10)
	poolObj := setupPool(b)
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
	InitDefaultFields()

	b.SetParallelism(10)
	poolObj := setupPool(b)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			poolObj.Put(&Example{})
		}
	})
}

func Benchmark_Grow(b *testing.B) {
	debug.SetGCPercent(-1)
	b.ReportAllocs()
	InitDefaultFields()

	poolObj := setupPool(b)

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
	InitDefaultFields()

	poolObj := setupPool(b)
	b.SetParallelism(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			obj := poolObj.slowPath()
			_ = obj
		}
	})
}

func Benchmark_RepeatedShrink(b *testing.B) {
	debug.SetGCPercent(-1)
	b.ReportAllocs()
	InitDefaultFields()
	poolObj := setupPool(b)

	for range 2_000_000 {
		poolObj.Put(poolObj.allocator())
	}

	prevCap := len(poolObj.pool)
	minCap := int(poolObj.config.shrink.minCapacity)

	for {
		inUse := 0
		newCap := prevCap - 10000
		if newCap < minCap {
			break
		}

		poolObj.performShrink(newCap, inUse, uint64(prevCap))

		newLen := len(poolObj.pool)
		if newLen >= prevCap {
			break
		}
		prevCap = newLen
	}
}
