package test

import (
	"github.com/AlexsanderHamir/memory_context/v1.0.2/contexts"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextManagerBasicOperations(t *testing.T) {
	cm := contexts.NewContextManager(nil)
	defer cm.Close()

	ctxType := contexts.MemoryContextType("test_context")
	config := contexts.MemoryContextConfig{
		ContextType: ctxType,
	}

	// Test creating a new context
	ctx, exists, err := cm.GetOrCreateContext(ctxType, config)
	require.NoError(t, err)
	require.False(t, exists, "Expected context to be newly created")
	require.NotNil(t, ctx)

	// Test getting existing context
	ctx2, exists, err := cm.GetOrCreateContext(ctxType, config)
	require.NoError(t, err)
	require.True(t, exists, "Expected context to exist")
	require.Equal(t, ctx, ctx2)

	// Test returning context
	// return the same context twice because we got a reference to it twice
	err = cm.ReturnContext(ctx)
	require.NoError(t, err)

	err = cm.ReturnContext(ctx2)
	require.NoError(t, err)

	// Test closing context
	err = cm.CloseContext(ctxType, false)
	require.NoError(t, err)

	// Test getting non-existent context
	_, exists, err = cm.GetOrCreateContext(ctxType, config)
	require.NoError(t, err)
	require.False(t, exists, "Expected context to be newly created after close")
}

func TestContextManagerConcurrentAccess(t *testing.T) {
	cm := contexts.NewContextManager(nil)
	defer cm.Close()

	const numGoroutines = 50
	const operationsPerGoroutine = 100
	var wg sync.WaitGroup
	var successCount int32

	ctxType := contexts.MemoryContextType("concurrent_context")
	config := contexts.MemoryContextConfig{
		ContextType: ctxType,
	}

	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range operationsPerGoroutine {
				ctx, _, err := cm.GetOrCreateContext(ctxType, config)
				require.NoError(t, err)

				err = cm.ReturnContext(ctx)
				require.NoError(t, err)
				atomic.AddInt32(&successCount, 1)

			}
		}()
	}

	wg.Wait()
	assert.Equal(t, int32(numGoroutines*operationsPerGoroutine), successCount, "Expected all operations to succeed")
}

func TestContextManagerErrorConditions(t *testing.T) {
	cm := contexts.NewContextManager(nil)
	defer cm.Close()

	// Test invalid context type
	_, _, err := cm.GetOrCreateContext("", contexts.MemoryContextConfig{})
	require.Error(t, err)
	require.Equal(t, contexts.ErrInvalidContextType, err)

	// Test returning nil context
	err = cm.ReturnContext(nil)
	require.Error(t, err)

	// Test closing non-existent context
	err = cm.CloseContext("non_existent", false)
	require.Error(t, err)
	require.Equal(t, contexts.ErrContextNotFound, err)

	// Test closing context in use
	ctxType := contexts.MemoryContextType("in_use_context")
	config := contexts.MemoryContextConfig{
		ContextType: ctxType,
	}

	ctx, _, err := cm.GetOrCreateContext(ctxType, config)
	require.NoError(t, err)
	err = cm.CloseContext(ctxType, false)
	require.Error(t, err)
	require.Equal(t, contexts.ErrContextInUse, err)

	// Clean up
	cm.ReturnContext(ctx)
	err = cm.CloseContext(ctxType, false)
	require.NoError(t, err)
}

func TestContextManagerAutoCleanup(t *testing.T) {
	config := &contexts.ContextConfig{
		MaxIdleTime:     100 * time.Millisecond,
		CleanupInterval: 50 * time.Millisecond,
	}

	cm := contexts.NewContextManager(config)
	defer cm.Close()

	ctxType := contexts.MemoryContextType("cleanup_test")
	config2 := contexts.MemoryContextConfig{
		ContextType: ctxType,
	}

	// Create and return context
	ctx, exists, err := cm.GetOrCreateContext(ctxType, config2)
	require.NoError(t, err)
	require.False(t, exists)
	require.NotNil(t, ctx)

	err = cm.ReturnContext(ctx)
	require.NoError(t, err)

	// Wait for cleanup
	time.Sleep(200 * time.Millisecond)

	// Verify context was cleaned up
	ctx, exists, err = cm.GetOrCreateContext(ctxType, config2)
	require.NoError(t, err)
	require.False(t, exists)
	require.NotNil(t, ctx)
}

func TestContextManagerConcurrentClose(t *testing.T) {
	cm := contexts.NewContextManager(nil)
	defer cm.Close()

	const numGoroutines = 10
	var wg sync.WaitGroup
	var closeCount int32

	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := cm.Close(); err == nil {
				atomic.AddInt32(&closeCount, 1)
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, int32(1), closeCount, "Expected only one successful close")
}

func TestContextManagerReferenceCounting(t *testing.T) {
	cm := contexts.NewContextManager(nil)
	defer cm.Close()

	ctxType := contexts.MemoryContextType("ref_count_test")
	config := contexts.MemoryContextConfig{
		ContextType: ctxType,
	}

	// Create context and get multiple references
	ctx1, _, err := cm.GetOrCreateContext(ctxType, config)
	require.NoError(t, err)
	ctx2, _, err := cm.GetOrCreateContext(ctxType, config)
	require.NoError(t, err)

	// Return one reference
	err = cm.ReturnContext(ctx1)
	require.NoError(t, err)

	// Try to close context (should fail as still in use)
	err = cm.CloseContext(ctxType, false)
	require.Error(t, err)
	require.Equal(t, contexts.ErrContextInUse, err)

	// Return second reference
	err = cm.ReturnContext(ctx2)
	require.NoError(t, err)

	// Now should be able to close
	err = cm.CloseContext(ctxType, false)
	require.NoError(t, err)
}

func TestContextManagerConfigValidation(t *testing.T) {
	config := &contexts.ContextConfig{}

	cm := contexts.NewContextManager(config)
	defer cm.Close()

	ctxType := contexts.MemoryContextType("config_test")
	config2 := contexts.MemoryContextConfig{
		ContextType: ctxType,
	}

	// Should still work with invalid config (uses defaults)
	ctx, _, err := cm.GetOrCreateContext(ctxType, config2)
	require.NoError(t, err)
	require.NotNil(t, ctx)
}

func TestContextManagerMultipleContextTypes(t *testing.T) {
	cm := contexts.NewContextManager(nil)
	defer cm.Close()

	ctxTypes := []contexts.MemoryContextType{
		"type1",
		"type2",
		"type3",
	}

	for _, ctxType := range ctxTypes {
		config := contexts.MemoryContextConfig{
			ContextType: ctxType,
		}

		ctx, _, err := cm.GetOrCreateContext(ctxType, config)
		require.NoError(t, err)
		require.NotNil(t, ctx)

		// Verify each context is independent
		valid := cm.ValidateContext(ctx)
		require.True(t, valid)
	}

	// Verify all contexts exist
	for _, ctxType := range ctxTypes {
		config := contexts.MemoryContextConfig{
			ContextType: ctxType,
		}
		ctx, exists, err := cm.GetOrCreateContext(ctxType, config)
		require.NoError(t, err)
		require.True(t, exists)
		require.NotNil(t, ctx)
	}
}

func TestContextManagerMaxReferences(t *testing.T) {
	config := &contexts.ContextConfig{
		MaxReferences: 2,
	}
	cm := contexts.NewContextManager(config)
	defer cm.Close()

	ctxType := contexts.MemoryContextType("max_ref_test")
	config2 := contexts.MemoryContextConfig{
		ContextType: ctxType,
	}

	// Get first reference
	ctx1, _, err := cm.GetOrCreateContext(ctxType, config2)
	require.NoError(t, err)

	// Get second reference
	ctx2, _, err := cm.GetOrCreateContext(ctxType, config2)
	require.NoError(t, err)

	// Try to get third reference (should fail)
	_, _, err = cm.GetOrCreateContext(ctxType, config2)
	require.Error(t, err)

	// Return references
	cm.ReturnContext(ctx1)
	cm.ReturnContext(ctx2)

	// Now should be able to get new references
	ctx3, _, err := cm.GetOrCreateContext(ctxType, config2)
	require.NoError(t, err)
	require.NotNil(t, ctx3)
}

func TestContextManagerZeroOrNegativeConfig(t *testing.T) {
	// Test with all zero values
	zeroConfig := &contexts.ContextConfig{
		MaxReferences:   0,
		MaxIdleTime:     0,
		CleanupInterval: 0,
	}
	cm := contexts.NewContextManager(zeroConfig)
	defer cm.Close()

	ctxType := contexts.MemoryContextType("zero_config_test")
	config := contexts.MemoryContextConfig{
		ContextType: ctxType,
	}

	// Should be able to get unlimited references
	for range 20 {
		ctx, _, err := cm.GetOrCreateContext(ctxType, config)
		require.NoError(t, err)
		require.NotNil(t, ctx)
	}

	// Test with negative values
	negativeConfig := &contexts.ContextConfig{
		MaxReferences:   -1,
		MaxIdleTime:     -1 * time.Second,
		CleanupInterval: -1 * time.Second,
	}
	cm2 := contexts.NewContextManager(negativeConfig)
	defer cm2.Close()

	ctxType2 := contexts.MemoryContextType("negative_config_test")
	config2 := contexts.MemoryContextConfig{
		ContextType: ctxType2,
	}

	// Should be able to get unlimited references
	for range 20 {
		ctx, _, err := cm2.GetOrCreateContext(ctxType2, config2)
		require.NoError(t, err)
		require.NotNil(t, ctx)
	}

	// Verify cleanup routine is not started by checking that contexts are not cleaned up
	// even after waiting longer than the negative interval
	time.Sleep(100 * time.Millisecond)

	// Try to get the context again - it should still exist
	ctx, exists, err := cm2.GetOrCreateContext(ctxType2, config2)
	require.NoError(t, err)
	require.True(t, exists, "Context should still exist as cleanup is disabled")
	require.NotNil(t, ctx)
}
