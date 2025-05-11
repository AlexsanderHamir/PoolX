package pool

import (
	"fmt"
)

// validateBasicConfig performs validation of the core pool configuration parameters:
// - initialCapacity must be positive
// - hardLimit must be positive and greater than initialCapacity
// - hardLimit must be greater than or equal to minCapacity
// Returns an error if any validation fails.
func (b *poolConfigBuilder[T]) validateBasicConfig() error {
	if b.config.initialCapacity <= 0 {
		return fmt.Errorf("initialCapacity must be greater than 0, got %d", b.config.initialCapacity)
	}

	if b.config.hardLimit <= 0 {
		return fmt.Errorf("hardLimit must be greater than 0, got %d", b.config.hardLimit)
	}

	if b.config.hardLimit < b.config.initialCapacity {
		return fmt.Errorf("hardLimit (%d) must be >= initialCapacity (%d)", b.config.hardLimit, b.config.initialCapacity)
	}

	if b.config.hardLimit < b.config.shrink.minCapacity {
		return fmt.Errorf("hardLimit (%d) must be >= minCapacity (%d)", b.config.hardLimit, b.config.shrink.minCapacity)
	}

	return nil
}

// validateShrinkConfig validates all shrink-related configuration parameters:
// - maxConsecutiveShrinks must be non-negative
// - checkInterval and idleThreshold must be positive
// - minIdleBeforeShrink must be positive
// - idleThreshold must be greater than checkInterval
// - minCapacity must be less than or equal to initialCapacity
// - shrinkCooldown must be positive
// - minUtilizationBeforeShrink must be between 0 and 1.0
// - stableUnderutilizationRounds must be positive
// - shrinkPercent must be between 0 and 1.0
// Returns an error if any validation fails.
func (b *poolConfigBuilder[T]) validateShrinkConfig() error {
	sp := b.config.shrink

	if sp.maxConsecutiveShrinks < 0 {
		return fmt.Errorf("maxConsecutiveShrinks must be >= 0, got %d", sp.maxConsecutiveShrinks)
	}

	if sp.checkInterval <= 0 {
		return fmt.Errorf("checkInterval must be greater than 0, got %v", sp.checkInterval)
	}

	if sp.minCapacity > b.config.initialCapacity {
		return fmt.Errorf("minCapacity (%d) must be <= initialCapacity (%d)", sp.minCapacity, b.config.initialCapacity)
	}

	if sp.shrinkCooldown <= 0 {
		return fmt.Errorf("shrinkCooldown must be greater than 0, got %v", sp.shrinkCooldown)
	}

	if sp.minUtilizationBeforeShrink <= 0 {
		return fmt.Errorf("minUtilizationBeforeShrink must be greater than 0, got %d", sp.minUtilizationBeforeShrink)
	}

	if sp.stableUnderutilizationRounds <= 0 {
		return fmt.Errorf("stableUnderutilizationRounds must be greater than 0, got %d", sp.stableUnderutilizationRounds)
	}

	if sp.shrinkPercent <= 0 {
		return fmt.Errorf("shrinkPercent must be greater than 0, got %d", sp.shrinkPercent)
	}

	if sp.minCapacity <= 0 {
		return fmt.Errorf("minCapacity must be greater than 0, got %d", sp.minCapacity)
	}

	return nil
}

// validateGrowthConfig validates all growth-related configuration parameters:
// - exponentialThresholdFactor must be positive
// - growthPercent must be positive
// - fixedGrowthFactor must be positive
// Returns an error if any validation fails.
func (b *poolConfigBuilder[T]) validateGrowthConfig() error {
	gp := b.config.growth

	if gp.thresholdFactor <= 0 {
		return fmt.Errorf("thresholdFactor must be greater than 0, got %f", gp.thresholdFactor)
	}

	if gp.bigGrowthFactor <= 0 {
		return fmt.Errorf("bigGrowthFactor must be greater than 0, got %f", gp.bigGrowthFactor)
	}

	if gp.controlledGrowthFactor <= 0 {
		return fmt.Errorf("controlledGrowthFactor must be greater than 0, got %f", gp.controlledGrowthFactor)
	}

	return nil
}

// validateFastPathConfig validates all fast path (L1 cache) configuration parameters:
// - initialSize must be positive
// - fillAggressiveness must be between 0 and 1.0
// - refillPercent must be between 0 and 0.99
// - growthEventsTrigger must be positive
// - growth parameters (exponentialThresholdFactor, growthPercent, fixedGrowthFactor) must be positive
// - shrinkEventsTrigger must be positive
// - shrink parameters (minCapacity, shrinkPercent) must be positive
// Returns an error if any validation fails.
func (b *poolConfigBuilder[T]) validateFastPathConfig() error {
	fp := b.config.fastPath

	if fp.initialSize <= 0 {
		return fmt.Errorf("fastPath.initialSize must be greater than 0, got %d", fp.initialSize)
	}

	if fp.fillAggressiveness <= 0 {
		return fmt.Errorf("fastPath.fillAggressiveness must be greater than 0, got %d", fp.fillAggressiveness)
	}

	if fp.refillPercent <= 0 {
		return fmt.Errorf("fastPath.refillPercent must be greater than 0, got %d", fp.refillPercent)
	}

	if fp.growthEventsTrigger <= 0 {
		return fmt.Errorf("fastPath.growthEventsTrigger must be greater than 0, got %d", fp.growthEventsTrigger)
	}

	if fp.growth.thresholdFactor <= 0 {
		return fmt.Errorf("fastPath.growth.thresholdFactor must be greater than 0, got %f", fp.growth.thresholdFactor)
	}

	if fp.growth.bigGrowthFactor <= 0 {
		return fmt.Errorf("fastPath.growth.bigGrowthFactor must be greater than 0, got %f", fp.growth.bigGrowthFactor)
	}

	if fp.growth.controlledGrowthFactor <= 0 {
		return fmt.Errorf("fastPath.growth.controlledGrowthFactor must be greater than 0, got %f", fp.growth.controlledGrowthFactor)
	}

	if fp.shrinkEventsTrigger <= 0 {
		return fmt.Errorf("fastPath.shrinkEventsTrigger must be greater than 0, got %d", fp.shrinkEventsTrigger)
	}

	if fp.shrink.minCapacity <= 0 {
		return fmt.Errorf("fastPath.shrink.minCapacity must be greater than 0, got %d", fp.shrink.minCapacity)
	}

	if fp.shrink.shrinkPercent <= 0 {
		return fmt.Errorf("fastPath.shrink.shrinkPercent must be greater than 0, got %d", fp.shrink.shrinkPercent)
	}

	return nil
}

func (b *poolConfigBuilder[T]) validateAllocationStrategy() error {
	as := b.config.allocationStrategy

	if as.AllocPercent <= 0 || as.AllocPercent > 100 {
		return fmt.Errorf("allocationStrategy.AllocPercent must be between 0 and 100, got %d", as.AllocPercent)
	}

	if as.AllocAmount <= 0 {
		return fmt.Errorf("allocationStrategy.AllocAmount must be greater than 0, got %d", as.AllocAmount)
	}

	return nil
}
