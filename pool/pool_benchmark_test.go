package pool

import (
	"runtime/debug"
	"testing"
	"time"
)

type example struct {
	Name string
	Age  int
}

var allocator = func() *example {
	return &example{}
}

var cleaner = func(e *example) {
	e.Name = ""
	e.Age = 0
}

func setupPool(b *testing.B, config *PoolConfig[*example]) *Pool[*example] {
	if config == nil {
		builder, err := NewPoolConfigBuilder[*example]().
			SetShrinkAggressiveness(AggressivenessExtreme)
		if err != nil {
			b.Fatalf("Failed to set shrink aggressiveness: %v", err)
		}
		config, err = builder.
			Build()
		if err != nil {
			b.Fatalf("Failed to build pool config: %v", err)
		}
	}

	cloneTemplate := func(obj *example) *example {
		dst := *obj
		return &dst
	}

	p, err := NewPool(config, allocator, cleaner, cloneTemplate)
	if err != nil {
		b.Fatalf("Failed to create pool: %v", err)
	}

	return p.(*Pool[*example])
}

func Benchmark_Get(b *testing.B) {
	debug.SetGCPercent(-1)
	b.ReportAllocs()

	config, err := NewPoolConfigBuilder[*example]().
		SetInitialCapacity(128).   // defaultPoolCapacity
		SetHardLimit(10_000_000).  // defaultHardLimit
		SetMinShrinkCapacity(128). // defaultMinCapacity
		SetFastPathEnableChannelGrowth(true).
		SetFastPathFillAggressiveness(100). // fillAggressivenessExtreme
		SetFastPathRefillPercent(10).       // defaultRefillPercent
		SetFastPathGrowthEventsTrigger(3).
		SetFastPathGrowthFactor(75).
		SetFastPathExponentialThresholdFactor(1000).
		SetFastPathFixedGrowthFactor(100).
		Build()
	if err != nil {
		b.Fatalf("Failed to create custom config: %v", err)
	}

	poolObj := setupPool(b, config)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			obj, err := poolObj.Get()
			if err != nil {
				b.Fatalf("Failed to get object from pool: %v", err)
			}
			_ = obj
		}
	})

}

func Benchmark_Grow(b *testing.B) {
	debug.SetGCPercent(-1)
	b.ReportAllocs()

	poolObj := setupPool(b, nil)

	for i := 0; i < b.N; i++ {
		poolObj.grow()

		time.Sleep(3 * time.Millisecond)
	}
}

func Benchmark_SlowPath(b *testing.B) {
	debug.SetGCPercent(-1)
	b.ReportAllocs()

	config, err := NewPoolConfigBuilder[*example]().
		SetInitialCapacity(4096 * 1000).
		SetHardLimit(4096 * 1000).
		Build()
	if err != nil {
		b.Fatalf("Failed to create custom config: %v", err)
	}

	poolObj := setupPool(b, config)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			obj, _ := poolObj.SlowPathGet()
			_ = obj
		}
	})
}

func Benchmark_Shrink(b *testing.B) {
	debug.SetGCPercent(-1)
	b.ReportAllocs()

	config, err := NewPoolConfigBuilder[*example]().
		SetInitialCapacity(4096 * 100).
		SetHardLimit(4096 * 100).
		SetMinShrinkCapacity(1).
		Build()
	if err != nil {
		b.Fatalf("Failed to create custom config: %v", err)
	}

	poolObj := setupPool(b, config)

	prevCap := poolObj.pool.Capacity()
	minCap := int(poolObj.config.shrink.minCapacity)

	for {
		inUse := 0
		newCap := prevCap - 2000
		if newCap < minCap {
			break
		}

		poolObj.performShrink(newCap, inUse)

		newLen := poolObj.pool.Capacity()
		if newLen >= prevCap {
			break
		}
		prevCap = newLen
	}
}
