package test

import (
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"memctx/contexts"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryContextConcurrentOperations(t *testing.T) {
	ctx := contexts.NewMemoryContext(contexts.MemoryContextConfig{
		ContextType: "test_context",
	})

	poolConfig := createTestPoolConfig(t, 2, 10, false)
	allocator := func() any {
		return &TestObject{Value: 42}
	}
	cleaner := func(obj any) {
		if to, ok := obj.(*TestObject); ok {
			to.Value = 0
		}
	}

	err := ctx.CreatePool(reflect.TypeOf(&TestObject{}), poolConfig, allocator, cleaner)
	require.NoError(t, err)

	t.Run("ConcurrentAcquireRelease", testConcurrentAcquireRelease)
	t.Run("ConcurrentChildCreation", testConcurrentChildCreation)
	t.Run("ConcurrentPoolOperationsWithChildren", testConcurrentPoolOperationsWithChildren)
	t.Run("ConcurrentClose", testConcurrentClose)
	t.Run("OperationsAfterClose", testOperationsAfterClose)
	t.Run("StressTest", testStressTest)
}

func testConcurrentAcquireRelease(t *testing.T) {
	ctx := contexts.NewMemoryContext(contexts.MemoryContextConfig{
		ContextType: "test_context",
	})

	// blocking mode otherwise it will return nil
	poolConfig := createTestPoolConfig(t, 8, 8, true)
	allocator := func() any {
		return &TestObject{Value: 42}
	}
	cleaner := func(obj any) {
		if to, ok := obj.(*TestObject); ok {
			to.Value = 0
		}
	}

	err := ctx.CreatePool(reflect.TypeOf(&TestObject{}), poolConfig, allocator, cleaner)
	require.NoError(t, err)

	const numGoroutines = 50
	const operationsPerGoroutine = 100
	var wg sync.WaitGroup
	var successCount int32

	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range operationsPerGoroutine {
				obj := ctx.Acquire(reflect.TypeOf(&TestObject{}))
				if obj != nil {
					if success := ctx.Release(reflect.TypeOf(&TestObject{}), obj); success {
						atomic.AddInt32(&successCount, 1)
					}
				}
			}
		}()
	}

	wg.Wait()
	fmt.Println("successCount", successCount)
	assert.Equal(t, int32(numGoroutines*operationsPerGoroutine), successCount, "Expected all operations to succeed")
}

func testConcurrentChildCreation(t *testing.T) {
	ctx := contexts.NewMemoryContext(contexts.MemoryContextConfig{
		ContextType: "test_context",
	})

	const numGoroutines = 20
	var wg sync.WaitGroup
	var successCount int32

	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ctx.CreateChild(); err == nil {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, int32(numGoroutines), successCount, "Expected all child creations to succeed")
}

func testConcurrentPoolOperationsWithChildren(t *testing.T) {
	ctx := contexts.NewMemoryContext(contexts.MemoryContextConfig{
		ContextType: "test_context",
	})

	poolConfig := createTestPoolConfig(t, 2, 10, false)
	allocator := func() any {
		return &TestObject{Value: 42}
	}
	cleaner := func(obj any) {
		if to, ok := obj.(*TestObject); ok {
			to.Value = 0
		}
	}

	err := ctx.CreatePool(reflect.TypeOf(&TestObject{}), poolConfig, allocator, cleaner)
	require.NoError(t, err)

	const numChildren = 5
	const numGoroutines = 10
	const operationsPerGoroutine = 50
	var wg sync.WaitGroup

	// Create child contexts
	children := make([]*contexts.MemoryContext, numChildren)
	for i := range numChildren {
		require.NoError(t, ctx.CreateChild())
		// We can't access children directly, so we'll create a new context for each child
		childCtx := contexts.NewMemoryContext(contexts.MemoryContextConfig{
			Parent:      ctx,
			ContextType: "child_context",
		})
		children[i] = childCtx
	}

	// Test concurrent operations on all contexts
	for _, child := range children {
		for range numGoroutines {
			wg.Add(1)
			go func(c *contexts.MemoryContext) {
				defer wg.Done()
				for range operationsPerGoroutine {
					obj := c.Acquire(reflect.TypeOf(&TestObject{}))
					if obj != nil {
						c.Release(reflect.TypeOf(&TestObject{}), obj)
					}
				}
			}(child)
		}
	}

	wg.Wait()
}

func testConcurrentClose(t *testing.T) {
	ctx := contexts.NewMemoryContext(contexts.MemoryContextConfig{
		ContextType: "test_context",
	})

	const numGoroutines = 10
	var wg sync.WaitGroup
	var closeCount int32

	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ctx.Close(); err == nil {
				atomic.AddInt32(&closeCount, 1)
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, int32(1), closeCount, "Expected only one successful close")
}

func testOperationsAfterClose(t *testing.T) {
	ctx := contexts.NewMemoryContext(contexts.MemoryContextConfig{
		ContextType: "test_context_after_close",
	})

	// Create a test pool
	poolConfig := createTestPoolConfig(t, 2, 10, false)
	allocator := func() any {
		return &TestObject{Value: 42}
	}
	cleaner := func(obj any) {
		if to, ok := obj.(*TestObject); ok {
			to.Value = 0
		}
	}

	err := ctx.CreatePool(reflect.TypeOf(&TestObject{}), poolConfig, allocator, cleaner)
	require.NoError(t, err)

	// Close the context
	require.NoError(t, ctx.Close())

	// Test operations after close
	assert.Nil(t, ctx.Acquire(reflect.TypeOf(&TestObject{})), "Expected nil when acquiring after close")
	assert.False(t, ctx.Release(reflect.TypeOf(&TestObject{}), &TestObject{}), "Expected false when releasing after close")
	assert.Error(t, ctx.CreateChild(), "Expected error when creating child after close")
}

func testStressTest(t *testing.T) {
	ctx := contexts.NewMemoryContext(contexts.MemoryContextConfig{
		ContextType: "stress_test",
	})

	// Create multiple pools
	poolConfig := createTestPoolConfig(t, 2, 20, false)
	for i := range 5 {
		objType := reflect.TypeOf(&TestObject{})
		allocator := func() any {
			return &TestObject{Value: i}
		}
		cleaner := func(obj any) {
			if to, ok := obj.(*TestObject); ok {
				to.Value = 0
			}
		}
		require.NoError(t, ctx.CreatePool(objType, poolConfig, allocator, cleaner))
	}

	const numGoroutines = 100
	const duration = 2 * time.Second
	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					// Randomly choose an operation
					switch id % 4 {
					case 0:
						obj := ctx.Acquire(reflect.TypeOf(&TestObject{}))
						if obj != nil {
							ctx.Release(reflect.TypeOf(&TestObject{}), obj)
						}
					case 1:
						ctx.CreateChild()
					case 2:
						ctx.GetPool(reflect.TypeOf(&TestObject{}))
					case 3:
						// Simulate some work
						time.Sleep(time.Millisecond)
					}
				}
			}
		}(i)
	}

	time.Sleep(duration)
	close(stop)
	wg.Wait()

	// Final cleanup
	assert.NoError(t, ctx.Close())
}
