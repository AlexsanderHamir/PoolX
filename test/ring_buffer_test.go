package test

import (
	"context"
	"testing"
	"time"

	"github.com/AlexsanderHamir/memory_context/v1.0.2/pool"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRingBufferBasicOperations(t *testing.T) {
	rb := pool.New[int](10)
	require.NotNil(t, rb)

	err := rb.Write(42)
	assert.NoError(t, err)

	val, err := rb.GetOne()
	assert.NoError(t, err)
	assert.Equal(t, 42, val)

	assert.Equal(t, 0, rb.Length())
	assert.Equal(t, 10, rb.Capacity())

	assert.True(t, rb.IsEmpty())
	assert.False(t, rb.IsFull())
}

func TestRingBufferBlocking(t *testing.T) {
	rb := pool.New[int](2).WithBlocking(true)
	require.NotNil(t, rb)

	n, err := rb.WriteMany([]int{1, 2})
	assert.NoError(t, err)
	assert.Equal(t, 2, n)

	done := make(chan bool)
	go func() {
		err := rb.Write(3)
		assert.NoError(t, err)
		done <- true
	}()

	time.Sleep(100 * time.Millisecond)
	val, err := rb.GetOne()
	assert.NoError(t, err)
	assert.Equal(t, 1, val)

	<-done
}

func TestRingBufferTimeout(t *testing.T) {
	rb := pool.New[int](2).
		WithBlocking(true).
		WithTimeout(100 * time.Millisecond)
	require.NotNil(t, rb)

	n, err := rb.WriteMany([]int{1, 2})
	assert.NoError(t, err)
	assert.Equal(t, 2, n)

	err = rb.Write(3)
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)

	_, err = rb.GetOne()
	assert.NoError(t, err)

	_, err = rb.GetOne()
	assert.NoError(t, err)

	_, err = rb.GetOne()
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestRingBufferWriteMany(t *testing.T) {
	rb := pool.New[int](10)
	require.NotNil(t, rb)

	items := []int{1, 2, 3, 4, 5}
	n, err := rb.WriteMany(items)
	assert.NoError(t, err)
	assert.Equal(t, len(items), n)

	vals, err := rb.GetN(3)
	assert.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, vals)

	assert.Equal(t, 2, rb.Length())
}

func TestRingBufferReset(t *testing.T) {
	rb := pool.New[int](10)
	require.NotNil(t, rb)

	err := rb.Write(1)
	assert.NoError(t, err)
	err = rb.Write(2)
	assert.NoError(t, err)

	rb.Reset()
	assert.Equal(t, 0, rb.Length())
	assert.True(t, rb.IsEmpty())
}

func TestRingBufferEdgeCases(t *testing.T) {
	rb := pool.New[int](0)
	assert.Nil(t, rb)

	rb = pool.New[int](-1)
	assert.Nil(t, rb)
}
