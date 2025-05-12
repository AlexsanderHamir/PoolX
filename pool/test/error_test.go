package test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/AlexsanderHamir/PoolX/pool"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapacityConfigurations(t *testing.T) {
	tests := []struct {
		name   string
		config func() (*pool.PoolConfig[*TestObject], error)
	}{
		{
			name: "zero initial capacity",
			config: func() (*pool.PoolConfig[*TestObject], error) {
				return pool.NewPoolConfigBuilder[*TestObject]().SetInitialCapacity(0).Build()
			},
		},
		{
			name: "negative initial capacity",
			config: func() (*pool.PoolConfig[*TestObject], error) {
				return pool.NewPoolConfigBuilder[*TestObject]().SetInitialCapacity(-1).Build()
			},
		},
		{
			name: "hard limit less than initial capacity",
			config: func() (*pool.PoolConfig[*TestObject], error) {
				return pool.NewPoolConfigBuilder[*TestObject]().
					SetInitialCapacity(10).
					SetHardLimit(5).
					Build()
			},
		},
		{
			name: "zero hard limit",
			config: func() (*pool.PoolConfig[*TestObject], error) {
				return pool.NewPoolConfigBuilder[*TestObject]().
					SetInitialCapacity(10).
					SetHardLimit(0).
					Build()
			},
		},
		{
			name: "negative hard limit",
			config: func() (*pool.PoolConfig[*TestObject], error) {
				return pool.NewPoolConfigBuilder[*TestObject]().
					SetInitialCapacity(10).
					SetHardLimit(-1).
					Build()
			},
		},
	}

	for _, tt := range tests {
		testInvalidConfig(t, tt.name, tt.config)
	}
}

func TestGrowthConfigurations(t *testing.T) {
	tests := []struct {
		name   string
		config func() (*pool.PoolConfig[*TestObject], error)
	}{
		{
			name: "zero growth percent",
			config: func() (*pool.PoolConfig[*TestObject], error) {
				return pool.NewPoolConfigBuilder[*TestObject]().SetGrowthFactor(0).Build()
			},
		},
		{
			name: "negative growth percent",
			config: func() (*pool.PoolConfig[*TestObject], error) {
				return pool.NewPoolConfigBuilder[*TestObject]().SetGrowthFactor(-1).Build()
			},
		},
		{
			name: "negative fixed growth factor",
			config: func() (*pool.PoolConfig[*TestObject], error) {
				return pool.NewPoolConfigBuilder[*TestObject]().SetFixedGrowthFactor(-1.0).Build()
			},
		},
		{
			name: "zero fixed growth factor",
			config: func() (*pool.PoolConfig[*TestObject], error) {
				return pool.NewPoolConfigBuilder[*TestObject]().SetFixedGrowthFactor(0).Build()
			},
		},
		{
			name: "negative exponential threshold factor",
			config: func() (*pool.PoolConfig[*TestObject], error) {
				return pool.NewPoolConfigBuilder[*TestObject]().SetGrowthExponentialThresholdFactor(-1.0).Build()
			},
		},
		{
			name: "zero exponential threshold factor",
			config: func() (*pool.PoolConfig[*TestObject], error) {
				return pool.NewPoolConfigBuilder[*TestObject]().SetGrowthExponentialThresholdFactor(0).Build()
			},
		},
	}

	for _, tt := range tests {
		testInvalidConfig(t, tt.name, tt.config)
	}
}

func TestShrinkConfigurations(t *testing.T) {
	tests := []struct {
		name   string
		config func() (*pool.PoolConfig[*TestObject], error)
	}{
		{
			name: "zero shrink percent",
			config: func() (*pool.PoolConfig[*TestObject], error) {
				return pool.NewPoolConfigBuilder[*TestObject]().SetShrinkPercent(0).Build()
			},
		},
		{
			name: "negative shrink percent",
			config: func() (*pool.PoolConfig[*TestObject], error) {
				return pool.NewPoolConfigBuilder[*TestObject]().SetShrinkPercent(-1).Build()
			},
		},
		{
			name: "negative shrink aggressiveness",
			config: func() (*pool.PoolConfig[*TestObject], error) {
				builder, err := pool.NewPoolConfigBuilder[*TestObject]().SetShrinkAggressiveness(-1)
				if err != nil {
					return nil, err
				}
				return builder.Build()
			},
		},
		{
			name: "shrink aggressiveness above extreme",
			config: func() (*pool.PoolConfig[*TestObject], error) {
				builder, err := pool.NewPoolConfigBuilder[*TestObject]().SetShrinkAggressiveness(pool.AggressivenessExtreme + 1)
				if err != nil {
					return nil, err
				}
				return builder.Build()
			},
		},
		{
			name: "negative shrink check interval",
			config: func() (*pool.PoolConfig[*TestObject], error) {
				return pool.NewPoolConfigBuilder[*TestObject]().SetShrinkCheckInterval(-1 * time.Second).Build()
			},
		},
		{
			name: "negative shrink cooldown",
			config: func() (*pool.PoolConfig[*TestObject], error) {
				return pool.NewPoolConfigBuilder[*TestObject]().SetShrinkCooldown(-1 * time.Second).Build()
			},
		},
		{
			name: "negative min utilization before shrink",
			config: func() (*pool.PoolConfig[*TestObject], error) {
				return pool.NewPoolConfigBuilder[*TestObject]().SetMinUtilizationBeforeShrink(-1).Build()
			},
		},
		{
			name: "negative stable underutilization rounds",
			config: func() (*pool.PoolConfig[*TestObject], error) {
				return pool.NewPoolConfigBuilder[*TestObject]().SetStableUnderutilizationRounds(-1).Build()
			},
		},
		{
			name: "negative min shrink capacity",
			config: func() (*pool.PoolConfig[*TestObject], error) {
				return pool.NewPoolConfigBuilder[*TestObject]().SetMinShrinkCapacity(-1).Build()
			},
		},
		{
			name: "negative max consecutive shrinks",
			config: func() (*pool.PoolConfig[*TestObject], error) {
				return pool.NewPoolConfigBuilder[*TestObject]().SetMaxConsecutiveShrinks(-1).Build()
			},
		},
	}

	for _, tt := range tests {
		testInvalidConfig(t, tt.name, tt.config)
	}
}

func TestFastPathConfigurations(t *testing.T) {
	tests := []struct {
		name   string
		config func() (*pool.PoolConfig[*TestObject], error)
	}{
		{
			name: "zero fast path initial size",
			config: func() (*pool.PoolConfig[*TestObject], error) {
				return pool.NewPoolConfigBuilder[*TestObject]().SetFastPathInitialSize(0).Build()
			},
		},
		{
			name: "negative fast path initial size",
			config: func() (*pool.PoolConfig[*TestObject], error) {
				return pool.NewPoolConfigBuilder[*TestObject]().SetFastPathInitialSize(-1).Build()
			},
		},
		{
			name: "negative fast path fill aggressiveness",
			config: func() (*pool.PoolConfig[*TestObject], error) {
				return pool.NewPoolConfigBuilder[*TestObject]().SetFastPathFillAggressiveness(-1).Build()
			},
		},
		{
			name: "negative fast path refill percent",
			config: func() (*pool.PoolConfig[*TestObject], error) {
				return pool.NewPoolConfigBuilder[*TestObject]().SetFastPathRefillPercent(-1).Build()
			},
		},
	}

	for _, tt := range tests {
		testInvalidConfig(t, tt.name, tt.config)
	}
}

func TestValidConfiguration(t *testing.T) {
	configFunc := func() (*pool.PoolConfig[*TestObject], error) {
		builder, err := pool.NewPoolConfigBuilder[*TestObject]().
			SetInitialCapacity(10).
			SetHardLimit(20).
			SetGrowthFactor(50).
			SetShrinkPercent(30).
			SetFastPathInitialSize(5).
			SetFastPathFillAggressiveness(80).
			SetFastPathRefillPercent(20).
			SetFixedGrowthFactor(100).
			SetGrowthExponentialThresholdFactor(200).
			SetShrinkAggressiveness(pool.AggressivenessBalanced)
		if err != nil {
			return nil, err
		}
		return builder.
			SetShrinkCheckInterval(1 * time.Second).
			SetShrinkCooldown(2 * time.Second).
			SetMinUtilizationBeforeShrink(20).
			SetStableUnderutilizationRounds(3).
			SetMinShrinkCapacity(5).
			SetMaxConsecutiveShrinks(3).
			Build()
	}

	testValidConfig(t, "valid configuration", configFunc)
}

func TestNilConfig(t *testing.T) {
	allocator := func() *TestObject {
		return &TestObject{Value: 42}
	}

	cleaner := func(obj *TestObject) {
		obj.Value = 0
	}

	cloneTemplate := func(obj *TestObject) *TestObject {
		dst := *obj
		return &dst
	}

	p, err := pool.NewPool(nil, allocator, cleaner, cloneTemplate)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestResourceCleanup(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder[*TestObject]().
		SetInitialCapacity(100).
		SetMinShrinkCapacity(100).
		SetHardLimit(100).
		SetFastPathInitialSize(64).
		Build()
	require.NoError(t, err)

	var created, cleaned atomic.Int64
	allocator := func() *TestObject {
		created.Add(1)
		return &TestObject{Value: 42}
	}
	cleaner := func(obj *TestObject) {
		cleaned.Add(1)
		obj.Value = 0
	}

	cloneTemplate := func(obj *TestObject) *TestObject {
		created.Add(1)
		dst := *obj
		return &dst
	}

	p, err := pool.NewPool(config, allocator, cleaner, cloneTemplate)
	require.NoError(t, err)

	objNum := 100
	objects := make([]*TestObject, objNum)
	for i := range objects {
		objects[i], err = p.Get()
		require.NoError(t, err)
		require.NotNil(t, objects[i])
	}

	for _, obj := range objects {
		err := p.Put(obj)
		require.NoError(t, err)
	}

	err = p.Close()
	require.NoError(t, err)

	validation := 2
	movedToL1 := 64
	assert.Equal(t, int64(objNum+validation), created.Load())
	assert.Equal(t, int64(objNum+movedToL1), cleaned.Load())
}
