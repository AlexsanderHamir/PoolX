package test

import (
	"sync"
	"testing"
	"time"

	"memctx/pool"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testObject struct {
	value int
}

func TestBasicPoolOperations(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(10).
		SetMinShrinkCapacity(1).
		SetShrinkCheckInterval(1 * time.Second).
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
	require.NotNil(t, p)

	obj := p.Get()
	assert.NotNil(t, obj)
	assert.Equal(t, 42, obj.value)

	p.Put(obj)
	obj = p.Get()
	assert.NotNil(t, obj)
	assert.Equal(t, 42, obj.value)

	p.PrintPoolStats()
}

func TestPoolGrowth(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(2).
		SetGrowthPercent(0.5).
		SetFixedGrowthFactor(1.0).
		SetMinShrinkCapacity(1).
		SetShrinkCheckInterval(1 * time.Second).
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

	// Get more objects than initial capacity to trigger growth
	objects := make([]*testObject, 10)
	for i := range 10 {
		objects[i] = p.Get()
		assert.NotNil(t, objects[i])
	}

	// Return objects
	for _, obj := range objects {
		p.Put(obj)
	}

	p.PrintPoolStats()
}

func TestPoolShrink(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(10).
		SetShrinkAggressiveness(pool.AggressivenessExtreme).
		SetMinShrinkCapacity(1).
		SetShrinkCheckInterval(1 * time.Second).
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

	// Get and return objects to trigger shrink
	objects := make([]*testObject, 5)
	for i := range 5 {
		objects[i] = p.Get()
	}

	// Wait for shrink to potentially occur
	time.Sleep(20 * time.Second)

	assert.True(t, p.IsShrunk())
}

func TestConcurrentAccess(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(64).   // blocks shrink
		SetMinShrinkCapacity(64). // blocks shrink
		SetHardLimit(1000).       // blocks growth
		SetShrinkCheckInterval(1 * time.Second).
		SetFastPathGrowthPercent(0.8).
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
	iterations := 10
	workers := 10

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iterations {
				obj := p.Get()
				assert.NotNil(t, obj)
				assert.Equal(t, 42, obj.value)
				time.Sleep(time.Millisecond)
				p.Put(obj)
			}
		}()
	}

	wg.Wait()
}

// The default configuration is non blocking, so if we can't grow and there are no more available elements
// the ring buffer will return nil.
func TestHardLimit(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(2).
		SetHardLimit(5).
		SetMinShrinkCapacity(1).
		SetShrinkCheckInterval(1 * time.Second).
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

	objects := make([]*testObject, 6)
	var nilCount int
	for i := range 6 {
		objects[i] = p.Get()
		if objects[i] == nil {
			nilCount++
		}
	}

	assert.Equal(t, 1, nilCount)
}

func TestDefaultConfigNotModified(t *testing.T) {
	// Get default configuration values
	defaultConfig, err := pool.NewPoolConfigBuilder().Build()
	require.NoError(t, err)

	// Store original values
	originalValues := storeDefaultConfigValues(defaultConfig)

	// Create a custom configuration with different values
	customConfig, cancel := createCustomConfig(t)
	defer cancel()

	// Get new default configuration
	newDefaultConfig, err := pool.NewPoolConfigBuilder().Build()
	require.NoError(t, err)
	newDefaultValues := storeDefaultConfigValues(newDefaultConfig)

	// Verify custom values are different from default values
	verifyCustomValuesDifferent(t, originalValues, customConfig)

	// Verify default values remain unchanged
	verifyDefaultValuesUnchanged(t, originalValues, newDefaultValues)

}
