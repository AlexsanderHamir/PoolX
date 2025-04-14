package pool

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type poolConfigBuilder struct {
	config *poolConfig
}

func NewPoolConfigBuilder() *poolConfigBuilder {
	pgb := &poolConfigBuilder{
		config: &poolConfig{
			initialCapacity:  defaultPoolCapacity,
			hardLimit:        defaultHardLimit,
			shrink:           defaultShrinkParameters,
			growth:           defaultGrowthParameters,
			fastPath:         defaultFastPath,
			ringBufferConfig: defaultRingBufferConfig,
		},
	}

	pgb.config.shrink.ApplyDefaults(getShrinkDefaultsMap())
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

	if factor > 0 {
		b.config.growth.exponentialThresholdFactor = factor
	}
	return b
}

func (b *poolConfigBuilder) SetGrowthPercent(percent float64) *poolConfigBuilder {
	if percent > 0 {
		b.config.growth.growthPercent = percent
	}
	return b
}

func (b *poolConfigBuilder) SetFixedGrowthFactor(factor float64) *poolConfigBuilder {
	if factor > 0 {
		b.config.growth.fixedGrowthFactor = factor
	}
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
func (b *poolConfigBuilder) SetShrinkAggressiveness(level AggressivenessLevel) *poolConfigBuilder {
	if b.config.shrink.enforceCustomConfig {
		panic("can't set AggressivenessLevel if EnforceCustomConfig is active")
	}

	if level <= AggressivenessDisabled || level > AggressivenessExtreme {
		panic("aggressive level is out of bounds")
	}

	newBuilder := *b

	copiedShrink := *b.config.shrink
	copiedShrink.aggressivenessLevel = level
	copiedShrink.ApplyDefaults(getShrinkDefaultsMap())

	copiedFastPathShrink := *b.config.fastPath.shrink
	copiedFastPathShrink.aggressivenessLevel = level
	copiedFastPathShrink.ApplyDefaults(getShrinkDefaultsMap())
	copiedFastPathShrink.minCapacity = defaultL1MinCapacity

	newBuilder.config.shrink = &copiedShrink
	newBuilder.config.fastPath.shrink = &copiedFastPathShrink

	return &newBuilder
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

func (b *poolConfigBuilder) SetBufferSize(count int) *poolConfigBuilder {
	b.config.fastPath.bufferSize = count
	return b
}

func (b *poolConfigBuilder) SetFillAggressiveness(percent float64) *poolConfigBuilder {
	b.config.fastPath.fillAggressiveness = percent
	return b
}

func (b *poolConfigBuilder) SetRefillPercent(percent float64) *poolConfigBuilder {
	b.config.fastPath.refillPercent = percent
	return b
}

func (b *poolConfigBuilder) SetHardLimit(count int) *poolConfigBuilder {
	b.config.hardLimit = count
	return b
}

func (b *poolConfigBuilder) SetEnableChannelGrowth(enable bool) *poolConfigBuilder {
	b.config.fastPath.enableChannelGrowth = enable
	return b
}

func (b *poolConfigBuilder) SetGrowthEventsTrigger(count int) *poolConfigBuilder {
	b.config.fastPath.growthEventsTrigger = count
	return b
}
func (b *poolConfigBuilder) SetFastPathGrowthPercent(percent float64) *poolConfigBuilder {
	copiedGrowth := *b.config.fastPath.growth
	copiedGrowth.growthPercent = percent
	b.config.fastPath.growth = &copiedGrowth
	return b
}

func (b *poolConfigBuilder) SetFastPathExponentialThresholdFactor(percent float64) *poolConfigBuilder {
	b.config.fastPath.growth.exponentialThresholdFactor = percent
	return b
}

func (b *poolConfigBuilder) SetFastPathFixedGrowthFactor(percent float64) *poolConfigBuilder {
	b.config.fastPath.growth.fixedGrowthFactor = percent
	return b
}

func (b *poolConfigBuilder) SetShrinkEventsTrigger(count int) *poolConfigBuilder {
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

	newBuilder := *b

	copiedFastPathShrink := *b.config.fastPath.shrink
	copiedFastPathShrink.aggressivenessLevel = level
	copiedFastPathShrink.ApplyDefaults(getShrinkDefaultsMap())

	newBuilder.config.fastPath.shrink = &copiedFastPathShrink

	return &newBuilder
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

func (b *poolConfigBuilder) Build() (*poolConfig, error) {
	if b.config.initialCapacity <= 0 {
		return nil, fmt.Errorf("InitialCapacity must be greater than 0")
	}

	if b.config.hardLimit <= 0 {
		return nil, fmt.Errorf("HardLimit must be greater than 0")
	}

	if b.config.hardLimit < b.config.initialCapacity {
		return nil, fmt.Errorf("HardLimit must be >= InitialCapacity")
	}
	if b.config.hardLimit < b.config.shrink.minCapacity {
		return nil, fmt.Errorf("HardLimit must be >= MinCapacity")
	}

	sp := b.config.shrink
	if sp.enforceCustomConfig {
		switch {
		case sp.maxConsecutiveShrinks <= 0:
			return nil, errors.New("MaxConsecutiveShrinks must be greater than 0")
		case sp.checkInterval <= 0:
			return nil, errors.New("CheckInterval must be greater than 0")
		case sp.idleThreshold <= 0:
			return nil, errors.New("IdleThreshold must be greater than 0")
		case sp.minIdleBeforeShrink <= 0:
			return nil, errors.New("MinIdleBeforeShrink must be greater than 0")
		case sp.idleThreshold < sp.checkInterval:
			return nil, errors.New("IdleThreshold must be >= CheckInterval")
		case sp.minCapacity > b.config.initialCapacity:
			return nil, errors.New("MinCapacity must be <= InitialCapacity")
		case sp.shrinkCooldown <= 0:
			return nil, errors.New("ShrinkCooldown must be greater than 0")
		case sp.minUtilizationBeforeShrink <= 0 || sp.minUtilizationBeforeShrink > 1.0:
			return nil, errors.New("MinUtilizationBeforeShrink must be between 0 and 1.0")
		case sp.stableUnderutilizationRounds <= 0:
			return nil, errors.New("StableUnderutilizationRounds must be greater than 0")
		case sp.shrinkPercent <= 0 || sp.shrinkPercent > 1.0:
			return nil, errors.New("ShrinkPercent must be between 0 and 1.0")
		case sp.minCapacity <= 0:
			return nil, errors.New("MinCapacity must be greater than 0")
		}
	}

	gp := b.config.growth
	if gp.exponentialThresholdFactor <= 0 {
		return nil, errors.New("ExponentialThresholdFactor must be greater than 0")
	}

	if gp.growthPercent <= 0 {
		return nil, fmt.Errorf("GrowthPercent must be > 0 (you gave %.2f)", gp.growthPercent)
	}

	if gp.fixedGrowthFactor <= 0 {
		return nil, fmt.Errorf("FixedGrowthFactor must be > 0 (you gave %.2f)", gp.fixedGrowthFactor)
	}

	fp := b.config.fastPath
	if fp.bufferSize <= 0 {
		return nil, fmt.Errorf("buffer must be > 0")
	}

	if fp.fillAggressiveness <= 0 || fp.fillAggressiveness > 1.0 {
		return nil, fmt.Errorf("fillAggressiveness  must be between 0 and 1.0")
	}

	if fp.refillPercent <= 0 || fp.refillPercent >= 1.0 {
		return nil, fmt.Errorf("refillPercent must be between 0 and 0.99")
	}

	if fp.growthEventsTrigger <= 0 {
		return nil, fmt.Errorf("growthEventsTrigger must be greater than 0")
	}

	if fp.growth.exponentialThresholdFactor <= 0 {
		return nil, fmt.Errorf("exponentialThresholdFactor must be greater than 0")
	}

	if fp.growth.growthPercent <= 0 {
		return nil, fmt.Errorf("growthPercent must be greater than 0")
	}

	if fp.growth.fixedGrowthFactor <= 0 {
		return nil, fmt.Errorf("fixedGrowthFactor must be greater than 0")
	}

	if fp.shrinkEventsTrigger <= 0 {
		return nil, fmt.Errorf("shrinkEventsTrigger must be greater than 0")
	}

	if fp.shrink.minCapacity <= 0 {
		return nil, fmt.Errorf("minCapacity must be greater than 0")
	}

	if fp.shrink.shrinkPercent <= 0 {
		return nil, fmt.Errorf("shrinkPercent must be greater than 0")
	}

	return b.config, nil
}
