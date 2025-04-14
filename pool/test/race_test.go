package test

import (
	"sync"
	"testing"
	"time"

	"memctx/pool"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRaceConditions(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(10).
		SetVerbose(true).
		Build()
	require.NoError(t, err)

	allocator := func() *testObject {
		return &testObject{value: 42}
	}

	cleaner := func(obj *testObject) {
		obj.value = 0
	}

	p, err := pool.NewPool(config, allocator, cleaner)
	require.NoError(t, err)

	var wg sync.WaitGroup
	iterations := 1000
	workers := 20

	// Test concurrent Get and Put operations
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iterations {
				obj := p.Get()
				assert.NotNil(t, obj)
				time.Sleep(time.Microsecond)
				p.Put(obj)
			}
		}()
	}

	wg.Wait()
}

func TestConcurrentGrowthAndShrink(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(2).
		SetGrowthPercent(0.5).
		SetShrinkAggressiveness(pool.AggressivenessExtreme).
		SetVerbose(true).
		Build()
	require.NoError(t, err)

	allocator := func() *testObject {
		return &testObject{value: 42}
	}

	cleaner := func(obj *testObject) {
		obj.value = 0
	}

	p, err := pool.NewPool(config, allocator, cleaner)
	require.NoError(t, err)

	var wg sync.WaitGroup
	workers := 10
	iterations := 10000

	// Mix of operations that could trigger both growth and shrink
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iterations {
				// Get multiple objects to potentially trigger growth
				objects := make([]*testObject, 5)
				for k := range 5 {
					objects[k] = p.Get()
					assert.NotNil(t, objects[k])
				}

				// Wait a bit to allow shrink to potentially occur
				time.Sleep(time.Second)

				// Return objects
				for _, obj := range objects {
					p.Put(obj)
				}
			}
		}()
	}

	wg.Wait()
}

func TestEdgeCases(t *testing.T) {
	// Test with minimal configuration
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(1).
		SetVerbose(true).
		Build()
	require.NoError(t, err)

	allocator := func() *testObject {
		return &testObject{value: 42}
	}

	cleaner := func(obj *testObject) {
		obj.value = 0
	}

	p, err := pool.NewPool(config, allocator, cleaner)
	require.NoError(t, err)

	// Test rapid Get/Put cycles
	for range 1000 {
		obj := p.Get()
		assert.NotNil(t, obj)
		p.Put(obj)
	}

	// Test getting more objects than initial capacity
	objects := make([]*testObject, 10)
	for i := range 10 {
		objects[i] = p.Get()
		assert.NotNil(t, objects[i])
	}
}

func TestStressTest(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(100).
		SetHardLimit(1000).
		SetVerbose(true).
		Build()
	require.NoError(t, err)

	allocator := func() *testObject {
		return &testObject{value: 42}
	}

	cleaner := func(obj *testObject) {
		obj.value = 0
	}

	p, err := pool.NewPool(config, allocator, cleaner)
	require.NoError(t, err)

	var wg sync.WaitGroup
	workers := 50
	iterations := 1000

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iterations {
				// Get multiple objects
				objects := make([]*testObject, 10)
				for k := range 10 {
					objects[k] = p.Get()
					assert.NotNil(t, objects[k])
				}

				// Return objects
				for _, obj := range objects {
					p.Put(obj)
				}
			}
		}()
	}

	wg.Wait()
}
