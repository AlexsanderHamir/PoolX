package pool

import (
	"context"
	"fmt"
	"time"
)

type poolConfigBuilder struct {
	config *poolConfig
}

func NewPoolConfigBuilder() *poolConfigBuilder {
	copiedShrink := *defaultShrinkParameters
	copiedGrowth := *defaultGrowthParameters
	copiedFastPath := *defaultFastPath
	copiedRingBufferConfig := *defaultRingBufferConfig

	copiedFastPath.shrink = &shrinkParameters{
		aggressivenessLevel: copiedShrink.aggressivenessLevel,
	}

	pgb := &poolConfigBuilder{
		config: &poolConfig{
			initialCapacity:  defaultPoolCapacity,
			hardLimit:        defaultHardLimit,
			shrink:           &copiedShrink,
			growth:           &copiedGrowth,
			fastPath:         &copiedFastPath,
			ringBufferConfig: &copiedRingBufferConfig,
		},
	}

	pgb.config.shrink.ApplyDefaults(getShrinkDefaultsMap())
	pgb.config.fastPath.shrink.ApplyDefaults(getShrinkDefaultsMap())
	pgb.config.fastPath.shrink.minCapacity = defaultL1MinCapacity

	return pgb
}

// since shrink, growth, fastpath and ringBufferConfig are all pointers, we need to copy the values
// to avoid mutating the default values.

func (b *poolConfigBuilder) SetInitialCapacity(cap int) *poolConfigBuilder {
	b.config.initialCapacity = cap
	return b
}

func (b *poolConfigBuilder) SetGrowthExponentialThresholdFactor(factor float64) *poolConfigBuilder {
	b.config.growth.exponentialThresholdFactor = factor
	return b
}

func (b *poolConfigBuilder) SetGrowthPercent(percent float64) *poolConfigBuilder {
	b.config.growth.growthPercent = percent
	return b
}

func (b *poolConfigBuilder) SetFixedGrowthFactor(factor float64) *poolConfigBuilder {
	b.config.growth.fixedGrowthFactor = factor
	return b
}

// When called, all shrink parameters must be set manually.
// For partial overrides, leave EnforceCustomConfig to its default and set values directly.
func (b *poolConfigBuilder) EnforceCustomConfig() *poolConfigBuilder {
	newBuilder := *b
	copiedShrink := *b.config.shrink

	copiedShrink.enforceCustomConfig = true
	copiedShrink.aggressivenessLevel = AggressivenessDisabled
	copiedShrink.ApplyDefaults(getShrinkDefaultsMap())

	newBuilder.config.shrink = &copiedShrink

	return &newBuilder
}

// This controls how quickly and frequently the pool will shrink when underutilized, or idle.
// Calling this will override individual shrink settings by applying preset defaults.
// Use levels between aggressivenessConservative (1) and AggressivenessExtreme (5).
// (can't call this function if you enable EnforceCustomConfig)
func (b *poolConfigBuilder) SetShrinkAggressiveness(level AggressivenessLevel) (*poolConfigBuilder, error) {
	if b.config.shrink.enforceCustomConfig {
		return nil, fmt.Errorf("cannot set AggressivenessLevel when EnforceCustomConfig is active")
	}

	if level <= AggressivenessDisabled || level > AggressivenessExtreme {
		return nil, fmt.Errorf("aggressiveness level %d is out of bounds, must be between %d and %d",
			level, AggressivenessDisabled+1, AggressivenessExtreme)
	}

	b.config.shrink.aggressivenessLevel = level
	b.config.shrink.ApplyDefaults(getShrinkDefaultsMap())

	b.config.fastPath.shrink.aggressivenessLevel = level
	b.config.fastPath.shrink.ApplyDefaults(getShrinkDefaultsMap())
	b.config.fastPath.shrink.minCapacity = defaultL1MinCapacity

	return b, nil
}

func (b *poolConfigBuilder) SetShrinkCheckInterval(interval time.Duration) *poolConfigBuilder {
	b.config.shrink.checkInterval = interval
	return b
}

func (b *poolConfigBuilder) SetIdleThreshold(duration time.Duration) *poolConfigBuilder {
	b.config.shrink.idleThreshold = duration
	return b
}

func (b *poolConfigBuilder) SetMinIdleBeforeShrink(count int) *poolConfigBuilder {
	b.config.shrink.minIdleBeforeShrink = count
	return b
}

func (b *poolConfigBuilder) SetShrinkCooldown(duration time.Duration) *poolConfigBuilder {
	b.config.shrink.shrinkCooldown = duration
	return b
}

func (b *poolConfigBuilder) SetMinUtilizationBeforeShrink(threshold float64) *poolConfigBuilder {
	b.config.shrink.minUtilizationBeforeShrink = threshold
	return b
}

func (b *poolConfigBuilder) SetStableUnderutilizationRounds(rounds int) *poolConfigBuilder {
	b.config.shrink.stableUnderutilizationRounds = rounds
	return b
}

func (b *poolConfigBuilder) SetShrinkPercent(percent float64) *poolConfigBuilder {
	b.config.shrink.shrinkPercent = percent
	return b
}

func (b *poolConfigBuilder) SetMinShrinkCapacity(minCap int) *poolConfigBuilder {
	b.config.shrink.minCapacity = minCap
	return b
}

func (b *poolConfigBuilder) SetMaxConsecutiveShrinks(count int) *poolConfigBuilder {
	b.config.shrink.maxConsecutiveShrinks = count
	return b
}

func (b *poolConfigBuilder) SetFastPathInitialSize(count int) *poolConfigBuilder {
	b.config.fastPath.initialSize = count
	return b
}

func (b *poolConfigBuilder) SetFastPathFillAggressiveness(percent float64) *poolConfigBuilder {
	b.config.fastPath.fillAggressiveness = percent
	return b
}

func (b *poolConfigBuilder) SetFastPathRefillPercent(percent float64) *poolConfigBuilder {
	b.config.fastPath.refillPercent = percent
	return b
}

func (b *poolConfigBuilder) SetHardLimit(count int) *poolConfigBuilder {
	b.config.hardLimit = count
	return b
}

func (b *poolConfigBuilder) SetFastPathEnableChannelGrowth(enable bool) *poolConfigBuilder {
	b.config.fastPath.enableChannelGrowth = enable
	return b
}

func (b *poolConfigBuilder) SetFastPathGrowthEventsTrigger(count int) *poolConfigBuilder {
	b.config.fastPath.growthEventsTrigger = count
	return b
}
func (b *poolConfigBuilder) SetFastPathGrowthPercent(percent float64) *poolConfigBuilder {
	newBuilder := *b
	copiedFastPath := *b.config.fastPath
	copiedGrowth := *b.config.fastPath.growth

	copiedGrowth.growthPercent = percent
	copiedFastPath.growth = &copiedGrowth
	newBuilder.config.fastPath = &copiedFastPath

	return &newBuilder
}

func (b *poolConfigBuilder) SetFastPathExponentialThresholdFactor(percent float64) *poolConfigBuilder {
	b.config.fastPath.growth.exponentialThresholdFactor = percent
	return b
}

func (b *poolConfigBuilder) SetFastPathFixedGrowthFactor(percent float64) *poolConfigBuilder {
	b.config.fastPath.growth.fixedGrowthFactor = percent
	return b
}

func (b *poolConfigBuilder) SetFastPathShrinkEventsTrigger(count int) *poolConfigBuilder {
	b.config.fastPath.shrinkEventsTrigger = count
	return b
}

func (b *poolConfigBuilder) SetFastPathShrinkAggressiveness(level AggressivenessLevel) *poolConfigBuilder {
	if b.config.fastPath.shrink.enforceCustomConfig {
		panic("cannot set AggressivenessLevel if EnforceCustomConfig is active")
	}
	if level <= AggressivenessDisabled || level > AggressivenessExtreme {
		panic("aggressiveness level is out of bounds")
	}

	b.config.fastPath.shrink.aggressivenessLevel = level
	b.config.fastPath.shrink.ApplyDefaults(getShrinkDefaultsMap())

	return b
}

func (b *poolConfigBuilder) SetFastPathShrinkPercent(percent float64) *poolConfigBuilder {
	b.config.fastPath.shrink.shrinkPercent = percent
	return b
}

func (b *poolConfigBuilder) SetFastPathShrinkMinCapacity(minCap int) *poolConfigBuilder {
	b.config.fastPath.shrink.minCapacity = minCap
	return b
}

func (b *poolConfigBuilder) SetVerbose(verbose bool) *poolConfigBuilder {
	b.config.verbose = verbose
	return b
}

func (b *poolConfigBuilder) SetRingBufferBlocking(block bool) *poolConfigBuilder {
	b.config.ringBufferConfig.block = block
	return b
}

func (b *poolConfigBuilder) WithTimeOut(d time.Duration) *poolConfigBuilder {
	if d > 0 {
		b.config.ringBufferConfig.rTimeout = d
		b.config.ringBufferConfig.wTimeout = d
	}
	return b
}

func (b *poolConfigBuilder) SetRingBufferReadTimeout(d time.Duration) *poolConfigBuilder {
	if d > 0 {
		b.config.ringBufferConfig.rTimeout = d
	}
	return b
}

func (b *poolConfigBuilder) SetRingBufferWriteTimeout(d time.Duration) *poolConfigBuilder {
	if d > 0 {
		b.config.ringBufferConfig.wTimeout = d
	}
	return b
}

func (b *poolConfigBuilder) SetRingBufferCancel(ctx context.Context) *poolConfigBuilder {
	if ctx != nil {
		b.config.ringBufferConfig.cancel = ctx
	}
	return b
}

// Build creates a new pool configuration with the configured settings.
// It validates all configuration parameters and returns an error if any validation fails.
func (b *poolConfigBuilder) Build() (*poolConfig, error) {
	if err := b.validateBasicConfig(); err != nil {
		return nil, fmt.Errorf("basic configuration validation failed: %w", err)
	}

	if err := b.validateShrinkConfig(); err != nil {
		return nil, fmt.Errorf("shrink configuration validation failed: %w", err)
	}

	if err := b.validateGrowthConfig(); err != nil {
		return nil, fmt.Errorf("growth configuration validation failed: %w", err)
	}

	if err := b.validateFastPathConfig(); err != nil {
		return nil, fmt.Errorf("fast path configuration validation failed: %w", err)
	}

	return b.config, nil
}
