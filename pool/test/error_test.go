package test

import (
	"testing"

	"memctx/pool"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvalidConfigurations(t *testing.T) {
	// Test zero initial capacity
	_, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(0).
		Build()
	assert.Error(t, err)

	// Test hard limit less than initial capacity
	_, err = pool.NewPoolConfigBuilder().
		SetInitialCapacity(10).
		SetHardLimit(5).
		Build()
	assert.Error(t, err)

	// Test zero growth percent
	_, err = pool.NewPoolConfigBuilder().
		SetGrowthPercent(0).
		Build()
	assert.Error(t, err)

	// Test zero shrink percent
	_, err = pool.NewPoolConfigBuilder().
		SetShrinkPercent(0).
		Build()
	assert.Error(t, err)
}

func TestInvalidAllocator(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().Build()
	require.NoError(t, err)

	// Test non-pointer allocator
	allocator := func() testObject {
		return testObject{value: 42}
	}

	cleaner := func(obj testObject) {
		obj.value = 0
	}

	_, err = pool.NewPool(config, allocator, cleaner)
	assert.Error(t, err)
}

func TestNilConfig(t *testing.T) {
	allocator := func() *testObject {
		return &testObject{value: 42}
	}

	cleaner := func(obj *testObject) {
		obj.value = 0
	}

	// Test nil config
	p, err := pool.NewPool(nil, allocator, cleaner)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestInvalidFastPathConfig(t *testing.T) {
	// Test zero buffer size
	_, err := pool.NewPoolConfigBuilder().
		SetBufferSize(0).
		Build()
	assert.Error(t, err)

	// Test negative fill aggressiveness
	_, err = pool.NewPoolConfigBuilder().
		SetFillAggressiveness(-1).
		Build()
	assert.Error(t, err)
}

func TestInvalidShrinkConfig(t *testing.T) {
	// Test negative shrink aggressiveness
	_, err := pool.NewPoolConfigBuilder().
		SetShrinkAggressiveness(-1).
		Build()
	assert.Error(t, err)

	// Test shrink percent greater than 100
	_, err = pool.NewPoolConfigBuilder().
		SetShrinkPercent(1.5).
		Build()
	assert.Error(t, err)
}

func TestInvalidGrowthConfig(t *testing.T) {
	// Test negative growth factor
	_, err := pool.NewPoolConfigBuilder().
		SetFixedGrowthFactor(-1).
		Build()
	assert.Error(t, err)

	// Test growth percent greater than 100
	_, err = pool.NewPoolConfigBuilder().
		SetGrowthPercent(1.5).
		Build()
	assert.Error(t, err)
}
