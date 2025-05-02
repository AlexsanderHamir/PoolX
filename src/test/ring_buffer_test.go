package test

import (
	"context"
	"testing"
	"time"

	"github.com/AlexsanderHamir/PoolX/src/pool"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestValue struct {
	value int
}

func TestRingBufferBasicOperations(t *testing.T) {
	rb := pool.NewRingBuffer[*TestValue](10)
	require.NotNil(t, rb)

	err := rb.Write(&TestValue{value: 42})
	assert.NoError(t, err)

	val, err := rb.GetOne()
	assert.NoError(t, err)
	assert.Equal(t, 42, val.value)

	assert.Equal(t, 10, rb.Capacity())

	assert.True(t, rb.IsEmpty())
	assert.False(t, rb.IsFull())
}

func TestRingBufferBlocking(t *testing.T) {
	rb := pool.NewRingBuffer[*TestValue](2).WithBlocking(true)
	require.NotNil(t, rb)

	n, err := rb.WriteMany([]*TestValue{{value: 1}, {value: 2}})
	assert.NoError(t, err)
	assert.Equal(t, 2, n)

	done := make(chan bool)
	go func() {
		err := rb.Write(&TestValue{value: 3})
		assert.NoError(t, err)
		done <- true
	}()

	time.Sleep(100 * time.Millisecond)
	val, err := rb.GetOne()
	assert.NoError(t, err)
	assert.Equal(t, 1, val.value)

	<-done
}

func TestRingBufferTimeout(t *testing.T) {
	rb := pool.NewRingBuffer[*TestValue](2).
		WithBlocking(true).
		WithTimeout(100 * time.Millisecond)
	require.NotNil(t, rb)

	n, err := rb.WriteMany([]*TestValue{{value: 1}, {value: 2}})
	assert.NoError(t, err)
	assert.Equal(t, 2, n)

	err = rb.Write(&TestValue{value: 3})
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
	rb := pool.NewRingBuffer[*TestValue](10)
	require.NotNil(t, rb)

	items := []*TestValue{
		{value: 1},
		{value: 2},
		{value: 3},
		{value: 4},
		{value: 5},
	}
	n, err := rb.WriteMany(items)
	assert.NoError(t, err)
	assert.Equal(t, len(items), n)

	vals, err := rb.GetN(3)
	assert.NoError(t, err)
	assert.Equal(t, 1, vals[0].value)
	assert.Equal(t, 2, vals[1].value)
	assert.Equal(t, 3, vals[2].value)

	assert.Equal(t, 2, rb.Length())
}

func TestRingBufferReset(t *testing.T) {
	rb := pool.NewRingBuffer[*TestValue](10)
	require.NotNil(t, rb)

	err := rb.Write(&TestValue{value: 1})
	assert.NoError(t, err)
	err = rb.Write(&TestValue{value: 2})
	assert.NoError(t, err)

	rb.ClearBuffer()
	assert.Equal(t, 0, rb.Length())
	assert.True(t, rb.IsEmpty())
}

func TestRingBufferEdgeCases(t *testing.T) {
	rb := pool.NewRingBuffer[*TestValue](0)
	assert.Nil(t, rb)

	rb = pool.NewRingBuffer[*TestValue](-1)
	assert.Nil(t, rb)

	rb = pool.NewRingBuffer[*TestValue](10)
	require.NotNil(t, rb)
}

func TestRingBufferViewModification(t *testing.T) {
	rb := pool.NewRingBuffer[*TestValue](10)

	values := []*TestValue{
		{value: 1},
		{value: 2},
		{value: 3},
		{value: 4},
		{value: 5},
		{value: 6},
		{value: 7},
		{value: 8},
		{value: 9},
		{value: 10},
	}
	for _, v := range values {
		if err := rb.Write(v); err != nil {
			t.Fatalf("Failed to write to buffer: %v", err)
		}
	}

	part1, _, err := rb.GetNView(5)
	if err != nil {
		t.Fatalf("Failed to get views: %v", err)
	}

	if len(part1) > 0 {
		part1[0].value = 999
		part1 = append(part1, &TestValue{value: 1000})
	}

	readValues, err := rb.GetN(5)
	if err != nil {
		t.Fatalf("Failed to read from buffer: %v", err)
	}

	readValuesMap := make(map[int]bool)
	for _, v := range readValues {
		readValuesMap[v.value] = true
	}

	for i, v := range part1 {
		if readValuesMap[v.value] {
			t.Logf("Value at index %d was modified: got %d, want %d", i, v.value, values[i].value)
			t.Log("WARNING: Do not modify the buffer view")
		}
	}
}
