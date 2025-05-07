package pool

import (
	"fmt"
	"time"
)

// ============================================================================
// Basic Pool Configuration Methods
// ============================================================================

// SetPoolBasicConfigs sets the basic configuration parameters for the pool.
// Parameters:
//   - initialCapacity: Initial size of both ring buffer and fast path (can be overridden)
//   - hardLimit: Maximum number of objects the pool can grow to
//   - verbose: Enable detailed logging of pool operations
//   - enableChannelGrowth: Enable dynamic growth of the fast path channel
//   - enableStats: Enable collection of non-essential pool statistics
//
// Note: Zero or negative values are ignored, default values will be used instead.
func (b *poolConfigBuilder) SetPoolBasicConfigs(initialCapacity int, hardLimit int, enableChannelGrowth bool) PoolConfigBuilder {
	if initialCapacity > 0 {
		b.config.initialCapacity = initialCapacity
	}

	if hardLimit > 0 {
		b.config.hardLimit = hardLimit
	}

	b.config.fastPath.enableChannelGrowth = enableChannelGrowth

	return b
}

// ============================================================================
// Growth Configuration Methods
// ============================================================================

// SetRingBufferGrowthConfigs sets the growth configuration parameters for the ring buffer.
// Parameters:
//   - exponentialThresholdFactor: Threshold for switching from exponential to fixed growth
//   - growthPercent: Percentage growth rate when below threshold
//   - fixedGrowthFactor: Fixed step size for growth when above threshold
//
// Note: Zero or negative values are ignored, default values will be used instead.
func (b *poolConfigBuilder) SetRingBufferGrowthConfigs(exponentialThresholdFactor, growthPercent, fixedGrowthFactor int) PoolConfigBuilder {
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

// SetShrinkAggressiveness sets the auto-shrink level (1-5) with preset defaults for both
// ring buffer and fast path. Each level represents a different balance between
// memory efficiency and performance.
//
// Levels:
//
//	1: Conservative - Minimal shrinking, prioritizes performance
//	2: Moderate - Balanced approach
//	3: Aggressive - More aggressive shrinking
//	4: Very Aggressive - Heavy shrinking
//	5: Extreme - Maximum shrinking
//
// Returns an error if:
//   - Custom configuration is enforced
//   - Level is out of valid range
func (b *poolConfigBuilder) SetShrinkAggressiveness(level AggressivenessLevel) (PoolConfigBuilder, error) {
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

// EnforceCustomConfig disables default shrink configuration, requiring manual setting
// of all shrink parameters. This is useful when you need precise control over
// the shrinking behavior and don't want to use the preset aggressiveness levels.
func (b *poolConfigBuilder) EnforceCustomConfig() PoolConfigBuilder {
	newBuilder := *b
	copiedShrink := *b.config.shrink

	copiedShrink.enforceCustomConfig = true
	copiedShrink.aggressivenessLevel = AggressivenessDisabled
	copiedShrink.ApplyDefaults(getShrinkDefaultsMap())

	newBuilder.config.shrink = &copiedShrink

	return &newBuilder
}

// SetRingBufferShrinkConfigs sets custom shrink parameters for the ring buffer.
// Parameters:
//   - checkInterval: Time between shrink eligibility checks
//   - idleThreshold: Minimum idle duration before shrinking
//   - shrinkCooldown: Minimum time between shrink operations
//   - minIdleBeforeShrink: Required consecutive idle checks
//   - stableUnderutilizationRounds: Required stable underutilization rounds
//   - minCapacity: Minimum capacity after shrinking
//   - maxConsecutiveShrinks: Maximum consecutive shrink operations
//   - minUtilizationBeforeShrink: Utilization threshold for shrinking
//   - shrinkPercent: Percentage by which to shrink
//
// Note: Zero or negative values are ignored, default values will be used instead.
func (b *poolConfigBuilder) SetRingBufferShrinkConfigs(checkInterval, idleThreshold, shrinkCooldown time.Duration, minIdleBeforeShrink, stableUnderutilizationRounds, minCapacity, maxConsecutiveShrinks int, minUtilizationBeforeShrink, shrinkPercent int) PoolConfigBuilder {
	if checkInterval > 0 {
		b.config.shrink.checkInterval = checkInterval
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

// SetFastPathBasicConfigs sets the basic configuration parameters for the fast path (L1 cache).
// Parameters:
//   - initialSize: Initial capacity of the fast path buffer
//   - growthEventsTrigger: Number of growth events before fast path grows
//   - shrinkEventsTrigger: Number of shrink events before fast path shrinks
//   - fillAggressiveness: How aggressively to fill the fast path initially
//   - refillPercent: Threshold for refilling the fast path
func (b *poolConfigBuilder) SetFastPathBasicConfigs(initialSize, growthEventsTrigger, shrinkEventsTrigger int, fillAggressiveness, refillPercent int) PoolConfigBuilder {
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
// Parameters:
//   - exponentialThresholdFactor: Threshold for switching growth modes
//   - fixedGrowthFactor: Fixed step size for growth above threshold
//   - growthPercent: Percentage growth rate below threshold
func (b *poolConfigBuilder) SetFastPathGrowthConfigs(exponentialThresholdFactor, fixedGrowthFactor, growthPercent int) PoolConfigBuilder {
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
// Parameters:
//   - shrinkPercent: Percentage by which to shrink the fast path
//   - minCapacity: Minimum capacity after shrinking
func (b *poolConfigBuilder) SetFastPathShrinkConfigs(shrinkPercent, minCapacity int) PoolConfigBuilder {
	if shrinkPercent > 0 {
		b.config.fastPath.shrink.shrinkPercent = shrinkPercent
	}

	if minCapacity > 0 {
		b.config.fastPath.shrink.minCapacity = minCapacity
	}

	return b
}

// SetFastPathShrinkAggressiveness sets the shrink aggressiveness level for the fast path.
// Uses the same aggressiveness levels as the main pool (1-5).
// Panics if:
//   - Custom configuration is enforced
//   - Level is out of valid range
func (b *poolConfigBuilder) SetFastPathShrinkAggressiveness(level AggressivenessLevel) PoolConfigBuilder {
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
// Parameters:
//   - block: Whether operations should block when buffer is full/empty
//   - rTimeout: Read operation timeout
//   - wTimeout: Write operation timeout
//   - bothTimeout: Sets both read and write timeouts to the same value
//
// Note: Timeout values must be positive to take effect.
func (b *poolConfigBuilder) SetRingBufferBasicConfigs(block bool, rTimeout, wTimeout, bothTimeout time.Duration) PoolConfigBuilder {
	b.config.ringBufferConfig.Block = block

	if rTimeout > 0 {
		b.config.ringBufferConfig.RTimeout = rTimeout
	}

	if wTimeout > 0 {
		b.config.ringBufferConfig.WTimeout = wTimeout
	}

	if bothTimeout > 0 {
		b.config.ringBufferConfig.RTimeout = bothTimeout
		b.config.ringBufferConfig.WTimeout = bothTimeout
	}

	return b
}
