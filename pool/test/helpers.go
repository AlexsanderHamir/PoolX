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
		SetHardLimit(10000000028182820).
		SetGrowthPercent(0.5).
		SetFixedGrowthFactor(1.0).
		SetGrowthExponentialThresholdFactor(4.0).
		SetShrinkAggressiveness(pool.AggressivenessAggressive).
		SetShrinkCheckInterval(2 * time.Second).
		SetIdleThreshold(5 * time.Second).
		SetMinIdleBeforeShrink(29).
		SetShrinkCooldown(10 * time.Second).
		SetMinUtilizationBeforeShrink(0.321).
		SetStableUnderutilizationRounds(3121).
		SetShrinkPercent(0.25121).
		SetMinShrinkCapacity(10121).
		SetMaxConsecutiveShrinks(3121).
		SetBufferSize(50121).
		SetFillAggressiveness(0.8121).
		SetRefillPercent(0.2121).
		SetEnableChannelGrowth(false).
		SetGrowthEventsTrigger(5121).
		SetFastPathGrowthPercent(0.6121).
		SetFastPathExponentialThresholdFactor(3.0121).
		SetFastPathFixedGrowthFactor(0.8121).
		SetShrinkEventsTrigger(4121).
		SetFastPathShrinkAggressiveness(pool.AggressivenessAggressive).
		SetFastPathShrinkPercent(0.3121).
		SetFastPathShrinkMinCapacity(20121).
		SetVerbose(true).
		SetRingBufferBlocking(true).
		WithTimeOut(5 * time.Second).
		SetRingBufferCancel(ctx).
		Build()
	require.NoError(t, err)

	return config, cancel
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
