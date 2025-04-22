package pool

import (
	"fmt"
	"time"
)

// ============================================================================
// Basic Pool Configuration Methods
// ============================================================================

// SetPoolBasicConfigs sets the basic configuration parameters for the pool.
// Zero or negative values are ignored, the default values will be used.
//
// WARNING: The initial capacity apply to both ring buffer and fast path, which you can override.
func (b *poolConfigBuilder) SetPoolBasicConfigs(initialCapacity int, hardLimit int, verbose, enableFastPath bool) *poolConfigBuilder {
	if initialCapacity > 0 {
		b.config.initialCapacity = initialCapacity
	}

	if hardLimit > 0 {
		b.config.hardLimit = hardLimit
	}

	if verbose {
		b.config.verbose = verbose
	}

	b.config.fastPath.enableChannelGrowth = enableFastPath

	return b
}

// ============================================================================
// Growth Configuration Methods
// ============================================================================

// SetRingBufferGrowthConfigs sets the growth configuration parameters for the ring buffer.
// Zero or negative values are ignored, the default values will be used.
func (b *poolConfigBuilder) SetRingBufferGrowthConfigs(exponentialThresholdFactor float64, growthPercent float64, fixedGrowthFactor float64) *poolConfigBuilder {
	if exponentialThresholdFactor > 0 {
		b.config.growth.exponentialThresholdFactor = exponentialThresholdFactor
	}

	if growthPercent > 0 {
		b.config.growth.growthPercent = growthPercent
	}

	if fixedGrowthFactor > 0 {
		b.config.growth.fixedGrowthFactor = fixedGrowthFactor
	}
	return b
}

// ============================================================================
// Shrink Configuration Methods
// ============================================================================

// SetShrinkAggressiveness sets the auto-shrink level (1-5) with preset defaults for both ring buffer and fast path.
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

// EnforceCustomConfig disables default shrink configuration, requiring manual setting of all shrink parameters.
func (b *poolConfigBuilder) EnforceCustomConfig() *poolConfigBuilder {
	newBuilder := *b
	copiedShrink := *b.config.shrink

	copiedShrink.enforceCustomConfig = true
	copiedShrink.aggressivenessLevel = AggressivenessDisabled
	copiedShrink.ApplyDefaults(getShrinkDefaultsMap())

	newBuilder.config.shrink = &copiedShrink

	return &newBuilder
}

// SetRingBufferShrinkParams sets custom shrink parameters.
// Zero or negative values are ignored, the default values will be used.
func (b *poolConfigBuilder) SetRingBufferShrinkConfigs(checkInterval, idleThreshold, shrinkCooldown time.Duration, minIdleBeforeShrink, stableUnderutilizationRounds, minCapacity, maxConsecutiveShrinks int, minUtilizationBeforeShrink, shrinkPercent float64) *poolConfigBuilder {
	if checkInterval > 0 {
		b.config.shrink.checkInterval = checkInterval
	}

	if idleThreshold > 0 {
		b.config.shrink.idleThreshold = idleThreshold
	}

	if minIdleBeforeShrink > 0 {
		b.config.shrink.minIdleBeforeShrink = minIdleBeforeShrink
	}

	if shrinkCooldown > 0 {
		b.config.shrink.shrinkCooldown = shrinkCooldown
	}

	if minUtilizationBeforeShrink > 0 {
		b.config.shrink.minUtilizationBeforeShrink = minUtilizationBeforeShrink
	}

	if stableUnderutilizationRounds > 0 {
		b.config.shrink.stableUnderutilizationRounds = stableUnderutilizationRounds
	}

	if shrinkPercent > 0 {
		b.config.shrink.shrinkPercent = shrinkPercent
	}

	if minCapacity > 0 {
		b.config.shrink.minCapacity = minCapacity
	}

	if maxConsecutiveShrinks > 0 {
		b.config.shrink.maxConsecutiveShrinks = maxConsecutiveShrinks
	}

	return b
}

// ============================================================================
// Fast Path Configuration Methods
// ============================================================================

// SetFastPathBasicConfigs sets the basic configuration parameters for the fast path.
func (b *poolConfigBuilder) SetFastPathBasicConfigs(initialSize int, growthEventsTrigger int, shrinkEventsTrigger int, fillAggressiveness, refillPercent float64) *poolConfigBuilder {
	if initialSize > 0 {
		b.config.fastPath.initialSize = initialSize
	}

	if growthEventsTrigger > 0 {
		b.config.fastPath.growthEventsTrigger = growthEventsTrigger
	}

	if shrinkEventsTrigger > 0 {
		b.config.fastPath.shrinkEventsTrigger = shrinkEventsTrigger
	}

	if fillAggressiveness > 0 {
		b.config.fastPath.fillAggressiveness = fillAggressiveness
	}

	if refillPercent > 0 {
		b.config.fastPath.refillPercent = refillPercent
	}

	return b
}

// SetFastPathGrowthConfigs sets the growth configuration parameters for the fast path.
func (b *poolConfigBuilder) SetFastPathGrowthConfigs(exponentialThresholdFactor float64, fixedGrowthFactor float64, growthPercent float64) *poolConfigBuilder {
	if exponentialThresholdFactor > 0 {
		b.config.fastPath.growth.exponentialThresholdFactor = exponentialThresholdFactor
	}

	if fixedGrowthFactor > 0 {
		b.config.fastPath.growth.fixedGrowthFactor = fixedGrowthFactor
	}

	if growthPercent > 0 {
		b.config.fastPath.growth.growthPercent = growthPercent
	}

	return b
}

// SetFastPathShrinkConfigs sets the shrink configuration parameters for the fast path.
func (b *poolConfigBuilder) SetFastPathShrinkConfigs(shrinkPercent float64, minCapacity int) *poolConfigBuilder {
	if shrinkPercent > 0 {
		b.config.fastPath.shrink.shrinkPercent = shrinkPercent
	}

	if minCapacity > 0 {
		b.config.fastPath.shrink.minCapacity = minCapacity
	}

	return b
}

// SetFastPathShrinkAggressiveness sets the shrink aggressiveness for the fast path.
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

// ============================================================================
// Ring Buffer Configuration Methods
// ============================================================================

// SetRingBufferBasicConfigs sets the basic configuration parameters for the ring buffer.
func (b *poolConfigBuilder) SetRingBufferBasicConfigs(block bool, rTimeout, wTimeout, bothTimeout time.Duration) *poolConfigBuilder {
	if block {
		b.config.ringBufferConfig.block = block
	}

	if rTimeout > 0 {
		b.config.ringBufferConfig.rTimeout = rTimeout
	}

	if wTimeout > 0 {
		b.config.ringBufferConfig.wTimeout = wTimeout
	}

	if bothTimeout > 0 {
		b.config.ringBufferConfig.rTimeout = bothTimeout
		b.config.ringBufferConfig.wTimeout = bothTimeout
	}

	return b
}
