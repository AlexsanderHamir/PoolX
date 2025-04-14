package test

import (
	"context"
	"memctx/pool"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRingBufferBasicOperations(t *testing.T) {
	rb := pool.New[int](10)
	require.NotNil(t, rb)

	// Test Write and GetOne
	err := rb.Write(42)
	assert.NoError(t, err)

	val, err := rb.GetOne()
	assert.NoError(t, err)
	assert.Equal(t, 42, val)

	// Test Length and Capacity
	assert.Equal(t, 0, rb.Length())
	assert.Equal(t, 10, rb.Capacity())

	// Test IsEmpty and IsFull
	assert.True(t, rb.IsEmpty())
	assert.False(t, rb.IsFull())
}

func TestRingBufferBlocking(t *testing.T) {
	rb := pool.New[int](2).WithBlocking(true)
	require.NotNil(t, rb)

	// Fill the buffer
	err := rb.Write(1)
	assert.NoError(t, err)
	err = rb.Write(2)
	assert.NoError(t, err)

	// Test blocking write
	done := make(chan bool)
	go func() {
		err := rb.Write(3)
		assert.NoError(t, err)
		done <- true
	}()

	// Wait a bit and then read to unblock the write
	time.Sleep(100 * time.Millisecond)
	val, err := rb.GetOne()
	assert.NoError(t, err)
	assert.Equal(t, 1, val)

	// Wait for the write to complete
	<-done
}

func TestRingBufferTimeout(t *testing.T) {
	rb := pool.New[int](2).
		WithBlocking(true).
		WithTimeout(100 * time.Millisecond)
	require.NotNil(t, rb)

	// Fill the buffer
	err := rb.Write(1)
	assert.NoError(t, err)
	err = rb.Write(2)
	assert.NoError(t, err)

	// Test timeout on write
	err = rb.Write(3)
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)

	// Empty the buffer
	_, err = rb.GetOne()
	assert.NoError(t, err)
	_, err = rb.GetOne()
	assert.NoError(t, err)

	// Test timeout on read
	_, err = rb.GetOne()
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestRingBufferConcurrentAccess(t *testing.T) {
	rb := pool.New[int](100).WithBlocking(true)
	require.NotNil(t, rb)

	var wg sync.WaitGroup
	iterations := 1000
	workers := 10

	// Test concurrent writes
	for i := range workers {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range iterations {
				err := rb.Write(id*iterations + j)
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()
	assert.Equal(t, iterations*workers, rb.Length())

	// Test concurrent reads
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iterations {
				_, err := rb.GetOne()
				assert.NoError(t, err)
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, 0, rb.Length())
}

func TestRingBufferWriteMany(t *testing.T) {
	rb := pool.New[int](10)
	require.NotNil(t, rb)

	// Test WriteMany
	items := []int{1, 2, 3, 4, 5}
	n, err := rb.WriteMany(items)
	assert.NoError(t, err)
	assert.Equal(t, len(items), n)

	// Test GetN
	vals, err := rb.GetN(3)
	assert.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, vals)

	// Test remaining items
	assert.Equal(t, 2, rb.Length())
}

func TestRingBufferReset(t *testing.T) {
	rb := pool.New[int](10)
	require.NotNil(t, rb)

	// Fill the buffer
	err := rb.Write(1)
	assert.NoError(t, err)
	err = rb.Write(2)
	assert.NoError(t, err)

	// Reset and verify
	rb.Reset()
	assert.Equal(t, 0, rb.Length())
	assert.True(t, rb.IsEmpty())
}

func TestRingBufferWithCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	rb := pool.New[int](10).WithCancel(ctx)
	require.NotNil(t, rb)

	// Fill the buffer
	err := rb.Write(1)
	assert.NoError(t, err)

	// Cancel the context
	cancel()

	// Verify operations fail after cancellation
	err = rb.Write(2)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)

	_, err = rb.GetOne()
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestRingBufferEdgeCases(t *testing.T) {
	// Test zero capacity
	rb := pool.New[int](0)
	assert.Nil(t, rb)

	// Test negative capacity
	rb = pool.New[int](-1)
	assert.Nil(t, rb)

	// Test with nil context
	rb = pool.New[int](10).WithCancel(nil)
	assert.NotNil(t, rb)
}
