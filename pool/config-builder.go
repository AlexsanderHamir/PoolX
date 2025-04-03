package pool

import (
	"fmt"
	"time"
)

type PoolConfigBuilder struct {
	config *PoolConfig
}

func NewPoolConfigBuilder() *PoolConfigBuilder {
	return &PoolConfigBuilder{
		config: &PoolConfig{
			InitialCapacity:      defaultPoolCapacity,
			PoolShrinkParameters: DefaultPoolShrinkParameters(),
			PoolGrowthParameters: DefaultPoolGrowthParameters(),
		},
	}
}

func (b *PoolConfigBuilder) SetInitialCapacity(cap int) *PoolConfigBuilder {
	b.config.InitialCapacity = cap
	return b
}

func (b *PoolConfigBuilder) SetGrowthExponentialThresholdFactor(factor float64) *PoolConfigBuilder {
	if factor > 0 {
		b.config.PoolGrowthParameters.ExponentialThresholdFactor = factor
	}
	return b
}

func (b *PoolConfigBuilder) SetGrowthPercent(percent float64) *PoolConfigBuilder {
	if percent > 0 {
		b.config.PoolGrowthParameters.GrowthPercent = percent
	}
	return b
}

func (b *PoolConfigBuilder) SetFixedGrowthFactor(factor float64) *PoolConfigBuilder {
	if factor > 0 {
		b.config.PoolGrowthParameters.FixedGrowthFactor = factor
	}
	return b
}

// Disables auto shrink. When called, all shrink parameters must be set manually.
// For partial overrides, leave auto shrink enabled and set values directly.
func (b *PoolConfigBuilder) DisableAutoShrink() *PoolConfigBuilder {
	b.config.PoolShrinkParameters.AggressivenessLevel = aggressivenessDisabled
	b.config.PoolShrinkParameters.ApplyDefaults()
	return b
}

// This controls how quickly and frequently the pool will shrink when underutilized, or idle.
// Calling this will override individual shrink settings by applying preset defaults.
// Use levels between aggressivenessConservative (1) and AggressivenessExtreme (5).
// (can't call this function if you disable auto shrink)
func (b *PoolConfigBuilder) SetShrinkAggressiveness(level AggressivenessLevel) *PoolConfigBuilder {
	if !b.config.PoolShrinkParameters.EnableAutoShrink {
		panic("can't set AggressivenessLevel if previously called DisableAutoShrink")
	}

	if level <= aggressivenessDisabled || level > aggressivenessExtreme {
		panic("aggressive level is out of bounds")
	}

	b.config.PoolShrinkParameters.AggressivenessLevel = level
	b.config.PoolShrinkParameters.ApplyDefaults()
	return b
}

func (b *PoolConfigBuilder) SetShrinkCheckInterval(interval time.Duration) *PoolConfigBuilder {
	b.config.PoolShrinkParameters.CheckInterval = interval
	return b
}

func (b *PoolConfigBuilder) SetIdleThreshold(duration time.Duration) *PoolConfigBuilder {
	b.config.PoolShrinkParameters.IdleThreshold = duration
	return b
}

func (b *PoolConfigBuilder) SetMinIdleBeforeShrink(count int) *PoolConfigBuilder {
	b.config.PoolShrinkParameters.MinIdleBeforeShrink = count
	return b
}

func (b *PoolConfigBuilder) SetShrinkCooldown(duration time.Duration) *PoolConfigBuilder {
	b.config.PoolShrinkParameters.ShrinkCooldown = duration
	return b
}

func (b *PoolConfigBuilder) SetMinUtilizationBeforeShrink(threshold float64) *PoolConfigBuilder {
	b.config.PoolShrinkParameters.MinUtilizationBeforeShrink = threshold
	return b
}

func (b *PoolConfigBuilder) SetStableUnderutilizationRounds(rounds int) *PoolConfigBuilder {
	b.config.PoolShrinkParameters.StableUnderutilizationRounds = rounds
	return b
}

func (b *PoolConfigBuilder) SetShrinkPercent(percent float64) *PoolConfigBuilder {
	b.config.PoolShrinkParameters.ShrinkPercent = percent
	return b
}

func (b *PoolConfigBuilder) SetMinShrinkCapacity(minCap int) *PoolConfigBuilder {
	b.config.PoolShrinkParameters.MinCapacity = minCap
	return b
}

func (b *PoolConfigBuilder) Build() (*PoolConfig, error) {
	if b.config.InitialCapacity <= 0 {
		return nil, fmt.Errorf("InitialCapacity must be greater than 0")
	}

	psp := b.config.PoolShrinkParameters

	if !psp.EnableAutoShrink {
		switch {
		case psp.CheckInterval <= 0:
			return nil, fmt.Errorf("CheckInterval must be greater than 0")
		case psp.IdleThreshold <= 0:
			return nil, fmt.Errorf("IdleThreshold must be greater than 0")
		case psp.MinIdleBeforeShrink <= 0:
			return nil, fmt.Errorf("MinIdleBeforeShrink must be greater than 0")
		case psp.IdleThreshold < psp.CheckInterval:
			return nil, fmt.Errorf("IdleThreshold must be >= CheckInterval")
		case psp.MinCapacity > b.config.InitialCapacity:
			return nil, fmt.Errorf("MinCapacity must be <= InitialCapacity")
		case psp.ShrinkCooldown <= 0:
			return nil, fmt.Errorf("ShrinkCooldown must be greater than 0")
		case psp.MinUtilizationBeforeShrink <= 0 || psp.MinUtilizationBeforeShrink > 1.0:
			return nil, fmt.Errorf("MinUtilizationBeforeShrink must be between 0 and 1.0")
		case psp.StableUnderutilizationRounds <= 0:
			return nil, fmt.Errorf("StableUnderutilizationRounds must be greater than 0")
		case psp.ShrinkPercent <= 0 || psp.ShrinkPercent > 1.0:
			return nil, fmt.Errorf("ShrinkPercent must be between 0 and 1.0")
		case psp.MinCapacity <= 0:
			return nil, fmt.Errorf("MinCapacity must be greater than 0")
		}
	}

	pgp := b.config.PoolGrowthParameters
	if pgp.ExponentialThresholdFactor <= 0 {
		return nil, fmt.Errorf("ExponentialThresholdFactor must be greater than 0")
	}

	if pgp.GrowthPercent <= 0 {
		return nil, fmt.Errorf("GrowthPercent must be > 0 (you gave %.2f)", pgp.GrowthPercent)
	}

	if pgp.FixedGrowthFactor <= 0 {
		return nil, fmt.Errorf("FixedGrowthFactor must be > 0 (you gave %.2f)", pgp.FixedGrowthFactor)
	}

	return b.config, nil
}
