package test

import (
	"reflect"
	"sync"
	"testing"

	"github.com/AlexsanderHamir/memory_context/v1.0.2/contexts"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPoolOperations(t *testing.T) {
	setup := setupTest(t)
	poolConfig := createTestPoolConfig(t, 2, 10, false)

	allocator := func() any {
		return &TestObject{Value: 42}
	}

	cleaner := func(obj any) {
		if to, ok := obj.(*TestObject); ok {
			to.Value = 0
		}
	}

	err := setup.ctx.CreatePool(reflect.TypeOf(&TestObject{}), poolConfig, allocator, cleaner)
	require.NoError(t, err)

	t.Run("Acquire", func(t *testing.T) {
		obj := setup.ctx.Acquire(reflect.TypeOf(&TestObject{}))
		assert.NotNil(t, obj, "Expected to acquire object from pool")
		if to, ok := obj.(*TestObject); ok {
			assert.Equal(t, 42, to.Value, "Acquired object has incorrect value")
		}
	})

	t.Run("Release", func(t *testing.T) {
		obj := setup.ctx.Acquire(reflect.TypeOf(&TestObject{}))
		success := setup.ctx.Release(reflect.TypeOf(&TestObject{}), obj)
		assert.True(t, success, "Expected successful release of object")
	})

	t.Run("AcquireAfterRelease", func(t *testing.T) {
		obj := setup.ctx.Acquire(reflect.TypeOf(&TestObject{}))
		setup.ctx.Release(reflect.TypeOf(&TestObject{}), obj)

		obj2 := setup.ctx.Acquire(reflect.TypeOf(&TestObject{}))
		assert.NotNil(t, obj2, "Expected to acquire object after release")
	})

	t.Run("InvalidRelease", func(t *testing.T) {
		success := setup.ctx.Release(reflect.TypeOf(&TestObject{}), "wrong type")
		assert.False(t, success, "Expected release to fail with wrong type")
	})

	t.Run("NonExistentPool", func(t *testing.T) {
		obj := setup.ctx.Acquire(reflect.TypeOf(&struct{}{}))
		assert.Nil(t, obj, "Expected nil when acquiring from non-existent pool")
	})
}

func TestMemoryContextWithCorrelatedPools(t *testing.T) {
	// Setup context manager
	cm := contexts.NewContextManager(nil)
	t.Cleanup(func() { cm.Close() })

	// Create a memory context type for our job processing
	ctxType := contexts.MemoryContextType("job_processing")
	config := contexts.MemoryContextConfig{
		ContextType: ctxType,
	}

	// Get or create the context
	ctx, exists, err := cm.GetOrCreateContext(ctxType, config)
	require.NoError(t, err)
	require.False(t, exists, "Expected context to be newly created")

	// Create all test pools
	createTestPools(t, ctx)

	t.Run("ConcurrentJobProcessing", func(t *testing.T) {
		const numJobs = 100
		const numWorkers = 10
		jobs := make(chan int, numJobs)
		results := make(chan *Job, numJobs)
		var wg sync.WaitGroup

		// Create job queue
		for i := range numJobs {
			jobs <- i
		}
		close(jobs)

		// Start workers
		for range numWorkers {
			wg.Add(1)
			go worker(t, ctx, jobs, results, &wg)
		}

		// Wait for all workers to finish
		wg.Wait()
		close(results)

		// Verify all jobs completed
		completedJobs := len(results)
		assert.Equal(t, numJobs, completedJobs, "Expected all jobs to complete")
	})
}

func TestMemoryContextConcurrentOperations(t *testing.T) {
	t.Run("ConcurrentAcquireRelease", testConcurrentAcquireRelease)
	t.Run("ConcurrentChildCreation", testConcurrentChildCreation)
	t.Run("ConcurrentPoolOperationsWithChildren", testConcurrentPoolOperationsWithChildren)
	t.Run("ConcurrentClose", testConcurrentClose)
	t.Run("OperationsAfterClose", testOperationsAfterClose)
	t.Run("StressTest", testStressTest)
}
