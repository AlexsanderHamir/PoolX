package test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/AlexsanderHamir/memory_context/pool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorRecovery(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(100).
		SetHardLimit(1000).
		Build()
	require.NoError(t, err)

	p := createTestPool(t, config)
	defer func() {
		require.NoError(t, p.Close())
	}()

	// Simulate error by closing pool
	err = p.Close()
	require.NoError(t, err)

	// Try to use closed pool
	obj := p.Get()
	assert.Nil(t, obj)

	// Create new pool
	p = createTestPool(t, config)
	defer func() {
		require.NoError(t, p.Close())
	}()

	// Verify new pool works
	obj = p.Get()
	require.NotNil(t, obj)
	err = p.Put(obj)
	require.NoError(t, err)
}

func TestContextCancellation(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(100).
		SetHardLimit(1000).
		Build()
	require.NoError(t, err)

	p := createTestPool(t, config)
	defer func() {
		require.NoError(t, p.Close())
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Get objects until context is cancelled
	objects := make([]*TestObject, 0)
	for {
		select {
		case <-ctx.Done():
			goto done
		default:
			obj := p.Get()
			if obj != nil {
				objects = append(objects, obj)
			}
		}
	}
done:

	// Verify we got some objects
	assert.Greater(t, len(objects), 0)

	// Return objects
	for _, obj := range objects {
		err := p.Put(obj)
		require.NoError(t, err)
	}
}

func TestConcurrentErrorHandling(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(100).
		SetHardLimit(1000).
		Build()
	require.NoError(t, err)

	p := createTestPool(t, config)
	defer func() {
		require.NoError(t, p.Close())
	}()

	numGoroutines := 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Simulate concurrent errors
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				obj := p.Get()
				if obj != nil {
					time.Sleep(time.Millisecond)
					_ = p.Put(obj)
				}
			}
		}()
	}

	wg.Wait()

	// Verify pool is still functioning
	obj := p.Get()
	require.NotNil(t, obj)
	err = p.Put(obj)
	require.NoError(t, err)
}

func TestResourceExhaustion(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(10).
		SetHardLimit(20).
		Build()
	require.NoError(t, err)

	p := createTestPool(t, config)
	defer func() {
		require.NoError(t, p.Close())
	}()

	// Exhaust pool
	objects := make([]*TestObject, 20)
	for i := range objects {
		objects[i] = p.Get()
		require.NotNil(t, objects[i])
	}

	// Try to get more objects
	obj := p.Get()
	assert.Nil(t, obj)

	// Return some objects
	for i := 0; i < 10; i++ {
		err := p.Put(objects[i])
		require.NoError(t, err)
	}

	// Verify pool is still functioning
	obj = p.Get()
	require.NotNil(t, obj)
	err = p.Put(obj)
	require.NoError(t, err)

	// Return remaining objects
	for i := 10; i < 20; i++ {
		err := p.Put(objects[i])
		require.NoError(t, err)
	}
}

func TestInvalidObjectHandling(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(100).
		SetHardLimit(1000).
		Build()
	require.NoError(t, err)

	p := createTestPool(t, config)
	defer func() {
		require.NoError(t, p.Close())
	}()

	// Try to put nil object
	var nilObj *TestObject
	err = p.Put(nilObj)
	assert.Error(t, err)

	// Try to put object of wrong type
	wrongObj := &struct{}{}
	err = p.Put(any(wrongObj).(*TestObject))
	assert.Error(t, err)

	// Verify pool is still functioning
	obj := p.Get()
	require.NotNil(t, obj)
	err = p.Put(obj)
	require.NoError(t, err)
}
