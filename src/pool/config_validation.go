package pool

import (
	"fmt"
)

func (b *poolConfigBuilder) validateBasicConfig() error {
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

func (b *poolConfigBuilder) validateShrinkConfig() error {
	sp := b.config.shrink

	if sp.maxConsecutiveShrinks <= 0 {
		return fmt.Errorf("maxConsecutiveShrinks must be greater than 0, got %d", sp.maxConsecutiveShrinks)
	}

	if sp.checkInterval <= 0 {
		return fmt.Errorf("checkInterval must be greater than 0, got %v", sp.checkInterval)
	}

	if sp.idleThreshold <= 0 {
		return fmt.Errorf("idleThreshold must be greater than 0, got %v", sp.idleThreshold)
	}

	if sp.minIdleBeforeShrink <= 0 {
		return fmt.Errorf("minIdleBeforeShrink must be greater than 0, got %d", sp.minIdleBeforeShrink)
	}

	if sp.idleThreshold < sp.checkInterval {
		return fmt.Errorf("idleThreshold (%v) must be >= checkInterval (%v)", sp.idleThreshold, sp.checkInterval)
	}

	if sp.minCapacity > b.config.initialCapacity {
		return fmt.Errorf("minCapacity (%d) must be <= initialCapacity (%d)", sp.minCapacity, b.config.initialCapacity)
	}

	if sp.shrinkCooldown <= 0 {
		return fmt.Errorf("shrinkCooldown must be greater than 0, got %v", sp.shrinkCooldown)
	}

	if sp.minUtilizationBeforeShrink <= 0 || sp.minUtilizationBeforeShrink > 1.0 {
		return fmt.Errorf("minUtilizationBeforeShrink must be between 0 and 1.0, got %.2f", sp.minUtilizationBeforeShrink)
	}

	if sp.stableUnderutilizationRounds <= 0 {
		return fmt.Errorf("stableUnderutilizationRounds must be greater than 0, got %d", sp.stableUnderutilizationRounds)
	}

	if sp.shrinkPercent <= 0 || sp.shrinkPercent > 1.0 {
		return fmt.Errorf("shrinkPercent must be between 0 and 1.0, got %.2f", sp.shrinkPercent)
	}

	if sp.minCapacity <= 0 {
		return fmt.Errorf("minCapacity must be greater than 0, got %d", sp.minCapacity)
	}

	return nil
}

func (b *poolConfigBuilder) validateGrowthConfig() error {
	gp := b.config.growth

	if gp.exponentialThresholdFactor <= 0 {
		return fmt.Errorf("exponentialThresholdFactor must be greater than 0, got %.2f", gp.exponentialThresholdFactor)
	}

	if gp.growthPercent <= 0 {
		return fmt.Errorf("growthPercent must be greater than 0, got %.2f", gp.growthPercent)
	}

	if gp.fixedGrowthFactor <= 0 {
		return fmt.Errorf("fixedGrowthFactor must be greater than 0, got %.2f", gp.fixedGrowthFactor)
	}

	return nil
}

func (b *poolConfigBuilder) validateFastPathConfig() error {
	fp := b.config.fastPath

	if fp.initialSize <= 0 {
		return fmt.Errorf("fastPath.initialSize must be greater than 0, got %d", fp.initialSize)
	}

	if fp.fillAggressiveness <= 0 || fp.fillAggressiveness > 1.0 {
		return fmt.Errorf("fastPath.fillAggressiveness must be between 0 and 1.0, got %.2f", fp.fillAggressiveness)
	}

	if fp.refillPercent <= 0 || fp.refillPercent >= 1.0 {
		return fmt.Errorf("fastPath.refillPercent must be between 0 and 0.99, got %.2f", fp.refillPercent)
	}

	if fp.growthEventsTrigger <= 0 {
		return fmt.Errorf("fastPath.growthEventsTrigger must be greater than 0, got %d", fp.growthEventsTrigger)
	}

	if fp.growth.exponentialThresholdFactor <= 0 {
		return fmt.Errorf("fastPath.growth.exponentialThresholdFactor must be greater than 0, got %.2f", fp.growth.exponentialThresholdFactor)
	}

	if fp.growth.growthPercent <= 0 {
		return fmt.Errorf("fastPath.growth.growthPercent must be greater than 0, got %.2f", fp.growth.growthPercent)
	}

	if fp.growth.fixedGrowthFactor <= 0 {
		return fmt.Errorf("fastPath.growth.fixedGrowthFactor must be greater than 0, got %.2f", fp.growth.fixedGrowthFactor)
	}

	if fp.shrinkEventsTrigger <= 0 {
		return fmt.Errorf("fastPath.shrinkEventsTrigger must be greater than 0, got %d", fp.shrinkEventsTrigger)
	}

	if fp.shrink.minCapacity <= 0 {
		return fmt.Errorf("fastPath.shrink.minCapacity must be greater than 0, got %d", fp.shrink.minCapacity)
	}

	if fp.shrink.shrinkPercent <= 0 {
		return fmt.Errorf("fastPath.shrink.shrinkPercent must be greater than 0, got %.2f", fp.shrink.shrinkPercent)
	}

	return nil
}
