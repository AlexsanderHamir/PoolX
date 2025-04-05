package pool

import (
	"fmt"
	"time"
)

type poolConfigBuilder struct {
	config *poolConfig
}

func NewPoolConfigBuilder() *poolConfigBuilder {
	return &poolConfigBuilder{
		config: &poolConfig{
			initialCapacity:      defaultPoolCapacity,
			poolShrinkParameters: defaultPoolShrinkParameters(),
			poolGrowthParameters: defaultPoolGrowthParameters(),
		},
	}
}

func (b *poolConfigBuilder) SetInitialCapacity(cap int) *poolConfigBuilder {
	b.config.initialCapacity = cap
	return b
}

func (b *poolConfigBuilder) SetGrowthExponentialThresholdFactor(factor float64) *poolConfigBuilder {
	if factor > 0 {
		b.config.poolGrowthParameters.exponentialThresholdFactor = factor
	}
	return b
}

func (b *poolConfigBuilder) SetGrowthPercent(percent float64) *poolConfigBuilder {
	if percent > 0 {
		b.config.poolGrowthParameters.growthPercent = percent
	}
	return b
}

func (b *poolConfigBuilder) SetFixedGrowthFactor(factor float64) *poolConfigBuilder {
	if factor > 0 {
		b.config.poolGrowthParameters.fixedGrowthFactor = factor
	}
	return b
}

// When called, all shrink parameters must be set manually.
// For partial overrides, leave EnforceCustomConfig to its default and set values directly.
func (b *poolConfigBuilder) EnforceCustomConfig() *poolConfigBuilder {
	b.config.poolShrinkParameters.enforceCustomConfig = true
	b.config.poolShrinkParameters.aggressivenessLevel = aggressivenessDisabled
	b.config.poolShrinkParameters.ApplyDefaults(getShrinkDefaults())
	return b
}

// This controls how quickly and frequently the pool will shrink when underutilized, or idle.
// Calling this will override individual shrink settings by applying preset defaults.
// Use levels between aggressivenessConservative (1) and AggressivenessExtreme (5).
// (can't call this function if you enable EnforceCustomConfig)
func (b *poolConfigBuilder) SetShrinkAggressiveness(level aggressivenessLevel) *poolConfigBuilder {
	if b.config.poolShrinkParameters.enforceCustomConfig {
		panic("can't set AggressivenessLevel if EnforceCustomConfig is active")
	}

	if level <= aggressivenessDisabled || level > aggressivenessExtreme {
		panic("aggressive level is out of bounds")
	}

	b.config.poolShrinkParameters.aggressivenessLevel = level
	b.config.poolShrinkParameters.ApplyDefaults(getShrinkDefaults())
	return b
}

func (b *poolConfigBuilder) SetShrinkCheckInterval(interval time.Duration) *poolConfigBuilder {
	b.config.poolShrinkParameters.checkInterval = interval
	return b
}

func (b *poolConfigBuilder) SetIdleThreshold(duration time.Duration) *poolConfigBuilder {
	b.config.poolShrinkParameters.idleThreshold = duration
	return b
}

func (b *poolConfigBuilder) SetMinIdleBeforeShrink(count int) *poolConfigBuilder {
	b.config.poolShrinkParameters.minIdleBeforeShrink = count
	return b
}

func (b *poolConfigBuilder) SetShrinkCooldown(duration time.Duration) *poolConfigBuilder {
	b.config.poolShrinkParameters.shrinkCooldown = duration
	return b
}

func (b *poolConfigBuilder) SetMinUtilizationBeforeShrink(threshold float64) *poolConfigBuilder {
	b.config.poolShrinkParameters.minUtilizationBeforeShrink = threshold
	return b
}

func (b *poolConfigBuilder) SetStableUnderutilizationRounds(rounds int) *poolConfigBuilder {
	b.config.poolShrinkParameters.stableUnderutilizationRounds = rounds
	return b
}

func (b *poolConfigBuilder) SetShrinkPercent(percent float64) *poolConfigBuilder {
	b.config.poolShrinkParameters.shrinkPercent = percent
	return b
}

func (b *poolConfigBuilder) SetMinShrinkCapacity(minCap int) *poolConfigBuilder {
	b.config.poolShrinkParameters.minCapacity = minCap
	return b
}

func (b *poolConfigBuilder) SetMaxConsecutiveShrinks(count int) *poolConfigBuilder {
	b.config.poolShrinkParameters.maxConsecutiveShrinks = count
	return b
}

func (b *poolConfigBuilder) Build() (*poolConfig, error) {
	if b.config.initialCapacity <= 0 {
		return nil, fmt.Errorf("InitialCapacity must be greater than 0")
	}

	psp := b.config.poolShrinkParameters

	if psp.enforceCustomConfig {
		switch {
		case psp.maxConsecutiveShrinks <= 0:
			return nil, fmt.Errorf("MaxConsecutiveShrinks must be greater than 0")
		case psp.checkInterval <= 0:
			return nil, fmt.Errorf("CheckInterval must be greater than 0")
		case psp.idleThreshold <= 0:
			return nil, fmt.Errorf("IdleThreshold must be greater than 0")
		case psp.minIdleBeforeShrink <= 0:
			return nil, fmt.Errorf("MinIdleBeforeShrink must be greater than 0")
		case psp.idleThreshold < psp.checkInterval:
			return nil, fmt.Errorf("IdleThreshold must be >= CheckInterval")
		case psp.minCapacity > b.config.initialCapacity:
			return nil, fmt.Errorf("MinCapacity must be <= InitialCapacity")
		case psp.shrinkCooldown <= 0:
			return nil, fmt.Errorf("ShrinkCooldown must be greater than 0")
		case psp.minUtilizationBeforeShrink <= 0 || psp.minUtilizationBeforeShrink > 1.0:
			return nil, fmt.Errorf("MinUtilizationBeforeShrink must be between 0 and 1.0")
		case psp.stableUnderutilizationRounds <= 0:
			return nil, fmt.Errorf("StableUnderutilizationRounds must be greater than 0")
		case psp.shrinkPercent <= 0 || psp.shrinkPercent > 1.0:
			return nil, fmt.Errorf("ShrinkPercent must be between 0 and 1.0")
		case psp.minCapacity <= 0:
			return nil, fmt.Errorf("MinCapacity must be greater than 0")
		}
	}

	pgp := b.config.poolGrowthParameters
	if pgp.exponentialThresholdFactor <= 0 {
		return nil, fmt.Errorf("ExponentialThresholdFactor must be greater than 0")
	}

	if pgp.growthPercent <= 0 {
		return nil, fmt.Errorf("GrowthPercent must be > 0 (you gave %.2f)", pgp.growthPercent)
	}

	if pgp.fixedGrowthFactor <= 0 {
		return nil, fmt.Errorf("FixedGrowthFactor must be > 0 (you gave %.2f)", pgp.fixedGrowthFactor)
	}

	return b.config, nil
}
