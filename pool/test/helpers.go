package test

import (
	"context"
	"memctx/pool"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DefaultConfigValues stores all default configuration values
type DefaultConfigValues struct {
	InitialCapacity                    int
	HardLimit                          int
	GrowthPercent                      float64
	FixedGrowthFactor                  float64
	ExponentialThresholdFactor         float64
	ShrinkAggressiveness               pool.AggressivenessLevel
	ShrinkCheckInterval                time.Duration
	IdleThreshold                      time.Duration
	MinIdleBeforeShrink                int
	ShrinkCooldown                     time.Duration
	MinUtilizationBeforeShrink         float64
	StableUnderutilizationRounds       int
	ShrinkPercent                      float64
	MinShrinkCapacity                  int
	MaxConsecutiveShrinks              int
	BufferSize                         int
	FillAggressiveness                 float64
	RefillPercent                      float64
	EnableChannelGrowth                bool
	GrowthEventsTrigger                int
	FastPathGrowthPercent              float64
	FastPathExponentialThresholdFactor float64
	FastPathFixedGrowthFactor          float64
	ShrinkEventsTrigger                int
	FastPathShrinkAggressiveness       pool.AggressivenessLevel
	FastPathShrinkPercent              float64
	FastPathShrinkMinCapacity          int
	Verbose                            bool
	RingBufferBlocking                 bool
	ReadTimeout                        time.Duration
	WriteTimeout                       time.Duration
}

// storeDefaultConfigValues stores all default configuration values
func storeDefaultConfigValues(config pool.PoolConfig) DefaultConfigValues {
	return DefaultConfigValues{
		InitialCapacity:                    config.GetInitialCapacity(),
		HardLimit:                          config.GetHardLimit(),
		GrowthPercent:                      config.GetGrowth().GetGrowthPercent(),
		FixedGrowthFactor:                  config.GetGrowth().GetFixedGrowthFactor(),
		ExponentialThresholdFactor:         config.GetGrowth().GetExponentialThresholdFactor(),
		ShrinkAggressiveness:               config.GetShrink().GetAggressivenessLevel(),
		ShrinkCheckInterval:                config.GetShrink().GetCheckInterval(),
		IdleThreshold:                      config.GetShrink().GetIdleThreshold(),
		MinIdleBeforeShrink:                config.GetShrink().GetMinIdleBeforeShrink(),
		ShrinkCooldown:                     config.GetShrink().GetShrinkCooldown(),
		MinUtilizationBeforeShrink:         config.GetShrink().GetMinUtilizationBeforeShrink(),
		StableUnderutilizationRounds:       config.GetShrink().GetStableUnderutilizationRounds(),
		ShrinkPercent:                      config.GetShrink().GetShrinkPercent(),
		MinShrinkCapacity:                  config.GetShrink().GetMinCapacity(),
		MaxConsecutiveShrinks:              config.GetShrink().GetMaxConsecutiveShrinks(),
		BufferSize:                         config.GetFastPath().GetBufferSize(),
		FillAggressiveness:                 config.GetFastPath().GetFillAggressiveness(),
		RefillPercent:                      config.GetFastPath().GetRefillPercent(),
		EnableChannelGrowth:                config.GetFastPath().IsEnableChannelGrowth(),
		GrowthEventsTrigger:                config.GetFastPath().GetGrowthEventsTrigger(),
		FastPathGrowthPercent:              config.GetFastPath().GetGrowth().GetGrowthPercent(),
		FastPathExponentialThresholdFactor: config.GetFastPath().GetGrowth().GetExponentialThresholdFactor(),
		FastPathFixedGrowthFactor:          config.GetFastPath().GetGrowth().GetFixedGrowthFactor(),
		ShrinkEventsTrigger:                config.GetFastPath().GetShrinkEventsTrigger(),
		FastPathShrinkAggressiveness:       config.GetFastPath().GetShrink().GetAggressivenessLevel(),
		FastPathShrinkPercent:              config.GetFastPath().GetShrink().GetShrinkPercent(),
		FastPathShrinkMinCapacity:          config.GetFastPath().GetShrink().GetMinCapacity(),
		Verbose:                            config.IsVerbose(),
		RingBufferBlocking:                 config.GetRingBufferConfig().IsBlocking(),
		ReadTimeout:                        config.GetRingBufferConfig().GetReadTimeout(),
		WriteTimeout:                       config.GetRingBufferConfig().GetWriteTimeout(),
	}
}

// createCustomConfig creates a custom configuration with different values
func createCustomConfig(t *testing.T) (pool.PoolConfig, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(100).
		SetHardLimit(1000).
		SetGrowthPercent(0.5).
		SetFixedGrowthFactor(1.0).
		SetGrowthExponentialThresholdFactor(4.0).
		SetShrinkAggressiveness(pool.AggressivenessAggressive).
		SetShrinkCheckInterval(2 * time.Second).
		SetIdleThreshold(5 * time.Second).
		SetMinIdleBeforeShrink(2).
		SetShrinkCooldown(10 * time.Second).
		SetMinUtilizationBeforeShrink(0.3).
		SetStableUnderutilizationRounds(3).
		SetShrinkPercent(0.25).
		SetMinShrinkCapacity(10).
		SetMaxConsecutiveShrinks(3).
		SetBufferSize(50).
		SetFillAggressiveness(0.8).
		SetRefillPercent(0.2).
		SetEnableChannelGrowth(true).
		SetGrowthEventsTrigger(5).
		SetFastPathGrowthPercent(0.6).
		SetFastPathExponentialThresholdFactor(3.0).
		SetFastPathFixedGrowthFactor(0.8).
		SetShrinkEventsTrigger(4).
		SetFastPathShrinkAggressiveness(pool.AggressivenessBalanced).
		SetFastPathShrinkPercent(0.3).
		SetFastPathShrinkMinCapacity(20).
		SetVerbose(true).
		SetRingBufferBlocking(true).
		WithTimeOut(5 * time.Second).
		SetRingBufferCancel(ctx).
		Build()
	require.NoError(t, err)

	return config, cancel
}

// verifyDefaultValuesUnchanged verifies that default values remain unchanged
func verifyDefaultValuesUnchanged(t *testing.T, original, newDefault DefaultConfigValues) {
	assert.Equal(t, original.InitialCapacity, newDefault.InitialCapacity)
	assert.Equal(t, original.HardLimit, newDefault.HardLimit)
	assert.Equal(t, original.GrowthPercent, newDefault.GrowthPercent)
	assert.Equal(t, original.FixedGrowthFactor, newDefault.FixedGrowthFactor)
	assert.Equal(t, original.ExponentialThresholdFactor, newDefault.ExponentialThresholdFactor)
	assert.Equal(t, original.ShrinkAggressiveness, newDefault.ShrinkAggressiveness)
	assert.Equal(t, original.ShrinkCheckInterval, newDefault.ShrinkCheckInterval)
	assert.Equal(t, original.IdleThreshold, newDefault.IdleThreshold)
	assert.Equal(t, original.MinIdleBeforeShrink, newDefault.MinIdleBeforeShrink)
	assert.Equal(t, original.ShrinkCooldown, newDefault.ShrinkCooldown)
	assert.Equal(t, original.MinUtilizationBeforeShrink, newDefault.MinUtilizationBeforeShrink)
	assert.Equal(t, original.StableUnderutilizationRounds, newDefault.StableUnderutilizationRounds)
	assert.Equal(t, original.ShrinkPercent, newDefault.ShrinkPercent)
	assert.Equal(t, original.MinShrinkCapacity, newDefault.MinShrinkCapacity)
	assert.Equal(t, original.MaxConsecutiveShrinks, newDefault.MaxConsecutiveShrinks)
	assert.Equal(t, original.BufferSize, newDefault.BufferSize)
	assert.Equal(t, original.FillAggressiveness, newDefault.FillAggressiveness)
	assert.Equal(t, original.RefillPercent, newDefault.RefillPercent)
	assert.Equal(t, original.EnableChannelGrowth, newDefault.EnableChannelGrowth)
	assert.Equal(t, original.GrowthEventsTrigger, newDefault.GrowthEventsTrigger)
	assert.Equal(t, original.FastPathGrowthPercent, newDefault.FastPathGrowthPercent)
	assert.Equal(t, original.FastPathExponentialThresholdFactor, newDefault.FastPathExponentialThresholdFactor)
	assert.Equal(t, original.FastPathFixedGrowthFactor, newDefault.FastPathFixedGrowthFactor)
	assert.Equal(t, original.ShrinkEventsTrigger, newDefault.ShrinkEventsTrigger)
	assert.Equal(t, original.FastPathShrinkAggressiveness, newDefault.FastPathShrinkAggressiveness)
	assert.Equal(t, original.FastPathShrinkPercent, newDefault.FastPathShrinkPercent)
	assert.Equal(t, original.FastPathShrinkMinCapacity, newDefault.FastPathShrinkMinCapacity)
	assert.Equal(t, original.Verbose, newDefault.Verbose)
	assert.Equal(t, original.RingBufferBlocking, newDefault.RingBufferBlocking)
	assert.Equal(t, original.ReadTimeout, newDefault.ReadTimeout)
	assert.Equal(t, original.WriteTimeout, newDefault.WriteTimeout)
}

// verifyCustomValuesDifferent verifies that custom values are different from default values
func verifyCustomValuesDifferent(t *testing.T, original DefaultConfigValues, custom pool.PoolConfig) {
	assert.NotEqual(t, original.InitialCapacity, custom.GetInitialCapacity())
	assert.NotEqual(t, original.HardLimit, custom.GetHardLimit())
	assert.NotEqual(t, original.GrowthPercent, custom.GetGrowth().GetGrowthPercent())
	assert.NotEqual(t, original.FixedGrowthFactor, custom.GetGrowth().GetFixedGrowthFactor())
	assert.NotEqual(t, original.ExponentialThresholdFactor, custom.GetGrowth().GetExponentialThresholdFactor())
	assert.NotEqual(t, original.ShrinkAggressiveness, custom.GetShrink().GetAggressivenessLevel())
	assert.NotEqual(t, original.ShrinkCheckInterval, custom.GetShrink().GetCheckInterval())
	assert.NotEqual(t, original.IdleThreshold, custom.GetShrink().GetIdleThreshold())
	assert.NotEqual(t, original.MinIdleBeforeShrink, custom.GetShrink().GetMinIdleBeforeShrink())
	assert.NotEqual(t, original.ShrinkCooldown, custom.GetShrink().GetShrinkCooldown())
	assert.NotEqual(t, original.MinUtilizationBeforeShrink, custom.GetShrink().GetMinUtilizationBeforeShrink())
	assert.NotEqual(t, original.StableUnderutilizationRounds, custom.GetShrink().GetStableUnderutilizationRounds())
	assert.NotEqual(t, original.ShrinkPercent, custom.GetShrink().GetShrinkPercent())
	assert.NotEqual(t, original.MinShrinkCapacity, custom.GetShrink().GetMinCapacity())
	assert.NotEqual(t, original.MaxConsecutiveShrinks, custom.GetShrink().GetMaxConsecutiveShrinks())
	assert.NotEqual(t, original.BufferSize, custom.GetFastPath().GetBufferSize())
	assert.NotEqual(t, original.FillAggressiveness, custom.GetFastPath().GetFillAggressiveness())
	assert.NotEqual(t, original.RefillPercent, custom.GetFastPath().GetRefillPercent())
	assert.NotEqual(t, original.EnableChannelGrowth, custom.GetFastPath().IsEnableChannelGrowth())
	assert.NotEqual(t, original.GrowthEventsTrigger, custom.GetFastPath().GetGrowthEventsTrigger())
	assert.NotEqual(t, original.FastPathGrowthPercent, custom.GetFastPath().GetGrowth().GetGrowthPercent())
	assert.NotEqual(t, original.FastPathExponentialThresholdFactor, custom.GetFastPath().GetGrowth().GetExponentialThresholdFactor())
	assert.NotEqual(t, original.FastPathFixedGrowthFactor, custom.GetFastPath().GetGrowth().GetFixedGrowthFactor())
	assert.NotEqual(t, original.ShrinkEventsTrigger, custom.GetFastPath().GetShrinkEventsTrigger())
	assert.NotEqual(t, original.FastPathShrinkAggressiveness, custom.GetFastPath().GetShrink().GetAggressivenessLevel())
	assert.NotEqual(t, original.FastPathShrinkPercent, custom.GetFastPath().GetShrink().GetShrinkPercent())
	assert.NotEqual(t, original.FastPathShrinkMinCapacity, custom.GetFastPath().GetShrink().GetMinCapacity())
	assert.NotEqual(t, original.Verbose, custom.IsVerbose())
	assert.NotEqual(t, original.RingBufferBlocking, custom.GetRingBufferConfig().IsBlocking())
	assert.NotEqual(t, original.ReadTimeout, custom.GetRingBufferConfig().GetReadTimeout())
	assert.NotEqual(t, original.WriteTimeout, custom.GetRingBufferConfig().GetWriteTimeout())
}
