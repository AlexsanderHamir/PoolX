package test

import (
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/AlexsanderHamir/memory_context/src/pool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryLeak(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(100).
		SetHardLimit(1000).
		Build()
	require.NoError(t, err)

	// Get initial memory stats
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Create and use pool
	p := createTestPool(t, config)
	objects := make([]*TestObject, 1000)
	for i := range objects {
		objects[i], err = p.Get()
		require.NoError(t, err)
		require.NotNil(t, objects[i])
	}

	// Return all objects
	for _, obj := range objects {
		err := p.Put(obj)
		require.NoError(t, err)
	}

	// Close pool
	err = p.Close()
	require.NoError(t, err)

	// Force GC
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	// Get final memory stats
	runtime.ReadMemStats(&m2)

	// Check for significant memory growth
	// Allow some overhead for the pool structure itself
	assert.Less(t, m2.Alloc-m1.Alloc, uint64(1024*1024)) // Less than 1MB growth
}

func TestResourceCleanup(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(100).
		SetHardLimit(1000).
		Build()
	require.NoError(t, err)

	// Track number of objects created and cleaned
	var created, cleaned int
	allocator := func() *TestObject {
		created++
		return &TestObject{Value: created}
	}
	cleaner := func(obj *TestObject) {
		cleaned++
		obj.Value = 0
	}

	p, err := pool.NewPool(config, allocator, cleaner, reflect.TypeOf(&TestObject{}))
	require.NoError(t, err)

	// Get and return objects
	objects := make([]*TestObject, 100)
	for i := range objects {
		objects[i], err = p.Get()
		require.NoError(t, err)
		require.NotNil(t, objects[i])
	}

	for _, obj := range objects {
		err := p.Put(obj)
		require.NoError(t, err)
	}

	// Close pool
	err = p.Close()
	require.NoError(t, err)

	// Verify all objects were cleaned
	assert.Equal(t, created, cleaned)
}

func TestConcurrentResourceCleanup(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(100).
		SetHardLimit(1000).
		Build()
	require.NoError(t, err)

	var created, cleaned int
	var mu sync.Mutex

	allocator := func() *TestObject {
		mu.Lock()
		created++
		mu.Unlock()
		return &TestObject{Value: created}
	}
	cleaner := func(obj *TestObject) {
		mu.Lock()
		cleaned++
		mu.Unlock()
		obj.Value = 0
	}

	p, err := pool.NewPool(config, allocator, cleaner, reflect.TypeOf(&TestObject{}))
	require.NoError(t, err)

	numGoroutines := 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			objects := make([]*TestObject, 20)
			for j := range objects {
				objects[j], err = p.Get()
				require.NoError(t, err)
				if objects[j] != nil {
					time.Sleep(time.Millisecond)
					_ = p.Put(objects[j])
				}
			}
		}()
	}

	wg.Wait()
	err = p.Close()
	require.NoError(t, err)

	// Verify all objects were cleaned
	assert.Equal(t, created, cleaned)
}

func TestPoolReuse(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(100).
		SetHardLimit(1000).
		Build()
	require.NoError(t, err)

	// Create and use first pool
	p1 := createTestPool(t, config)
	objects1 := make([]*TestObject, 100)
	for i := range objects1 {
		objects1[i], err = p1.Get()
		require.NoError(t, err)
		require.NotNil(t, objects1[i])
	}
	for _, obj := range objects1 {
		err := p1.Put(obj)
		require.NoError(t, err)
	}
	err = p1.Close()
	require.NoError(t, err)

	// Create and use second pool
	p2 := createTestPool(t, config)
	objects2 := make([]*TestObject, 100)
	for i := range objects2 {
		objects2[i], err = p2.Get()
		require.NoError(t, err)
		require.NotNil(t, objects2[i])
	}
	for _, obj := range objects2 {
		err := p2.Put(obj)
		require.NoError(t, err)
	}
	err = p2.Close()
	require.NoError(t, err)

	// Verify memory usage is stable
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)
	time.Sleep(100 * time.Millisecond)
	runtime.ReadMemStats(&m2)
	assert.Less(t, m2.Alloc-m1.Alloc, uint64(1024*1024)) // Less than 1MB growth
}
