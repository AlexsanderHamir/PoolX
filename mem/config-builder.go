package mem

import "fmt"

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

func (b *PoolConfigBuilder) DisableAutoShrink() *PoolConfigBuilder {
	b.config.PoolShrinkParameters.AggressivenessLevel = aggressivenessDisabled
	b.config.PoolShrinkParameters.ApplyDefaults()
	return b
}

func (b *PoolConfigBuilder) SetExponentialThresholdFactor(factor float64) *PoolConfigBuilder {
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

// set PoolShrinkParameters

func (b *PoolConfigBuilder) Build() (*PoolConfig, error) {
	psp := b.config.PoolShrinkParameters

	if !psp.EnableAutoShrink {
		if psp.ShrinkStrategy == "" ||
			psp.CheckInterval == 0 ||
			psp.IdleThreshold == 0 ||
			psp.MinIdleBeforeShrink == 0 ||
			psp.MinUtilizationBeforeShrink == 0 ||
			psp.StableUnderutilizationRounds == 0 ||
			psp.ShrinkStepPercent == 0 {
			return nil, fmt.Errorf("AutoShrink is disabled — you must manually configure all shrink parameters")
		}
	}

	if b.config.InitialCapacity <= 0 {
		return nil, fmt.Errorf("InitialCapacity must be greater than 0")
	}

	return b.config, nil
}
