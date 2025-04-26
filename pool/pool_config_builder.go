package pool

import (
	"fmt"
	"time"
)

// poolConfigBuilder is responsible for building pool configurations with various settings
type poolConfigBuilder struct {
	config *PoolConfig
}

// NewPoolConfigBuilder creates a new pool configuration builder with default settings.
func NewPoolConfigBuilder() *poolConfigBuilder {
	copiedShrink := *defaultShrinkParameters
	copiedGrowth := *defaultGrowthParameters
	copiedFastPath := *defaultFastPath
	copiedRingBufferConfig := *defaultRingBufferConfig

	copiedFastPath.shrink = &shrinkParameters{
		aggressivenessLevel: copiedShrink.aggressivenessLevel,
	}

	pgb := &poolConfigBuilder{
		config: &PoolConfig{
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

// SetInitialCapacity sets the initial capacity of the pool.
func (b *poolConfigBuilder) SetInitialCapacity(cap int) *poolConfigBuilder {
	b.config.initialCapacity = cap
	return b
}

// SetHardLimit sets the maximum number of objects the pool can grow to.
func (b *poolConfigBuilder) SetHardLimit(count int) *poolConfigBuilder {
	b.config.hardLimit = count
	return b
}

// SetVerbose enables or disables verbose logging.
func (b *poolConfigBuilder) SetVerbose(verbose bool) *poolConfigBuilder {
	b.config.verbose = verbose
	return b
}

// SetGrowthExponentialThresholdFactor sets the threshold factor for switching from exponential to fixed growth.
func (b *poolConfigBuilder) SetGrowthExponentialThresholdFactor(factor float64) *poolConfigBuilder {
	b.config.growth.exponentialThresholdFactor = factor
	return b
}

// SetGrowthPercent sets the growth percentage used in exponential growth mode.
func (b *poolConfigBuilder) SetGrowthPercent(percent float64) *poolConfigBuilder {
	b.config.growth.growthPercent = percent
	return b
}

// SetFixedGrowthFactor sets the fixed growth factor used after exponential growth ends.
func (b *poolConfigBuilder) SetFixedGrowthFactor(factor float64) *poolConfigBuilder {
	b.config.growth.fixedGrowthFactor = factor
	return b
}

// SetShrinkCheckInterval sets how often the pool checks if it should shrink.
func (b *poolConfigBuilder) SetShrinkCheckInterval(interval time.Duration) *poolConfigBuilder {
	b.config.shrink.checkInterval = interval
	return b
}

// SetIdleThreshold sets the minimum idle duration before shrinking is considered.
func (b *poolConfigBuilder) SetIdleThreshold(duration time.Duration) *poolConfigBuilder {
	b.config.shrink.idleThreshold = duration
	return b
}

// SetMinIdleBeforeShrink sets the number of consecutive idle checks before shrinking.
func (b *poolConfigBuilder) SetMinIdleBeforeShrink(count int) *poolConfigBuilder {
	b.config.shrink.minIdleBeforeShrink = count
	return b
}

// SetShrinkCooldown sets the minimum time between shrink operations.
func (b *poolConfigBuilder) SetShrinkCooldown(duration time.Duration) *poolConfigBuilder {
	b.config.shrink.shrinkCooldown = duration
	return b
}

// SetMinUtilizationBeforeShrink sets the utilization threshold for shrinking.
func (b *poolConfigBuilder) SetMinUtilizationBeforeShrink(threshold float64) *poolConfigBuilder {
	b.config.shrink.minUtilizationBeforeShrink = threshold
	return b
}

// SetStableUnderutilizationRounds sets the number of rounds underutilization must be stable.
func (b *poolConfigBuilder) SetStableUnderutilizationRounds(rounds int) *poolConfigBuilder {
	b.config.shrink.stableUnderutilizationRounds = rounds
	return b
}

// SetShrinkPercent sets the percentage by which the pool shrinks.
func (b *poolConfigBuilder) SetShrinkPercent(percent float64) *poolConfigBuilder {
	b.config.shrink.shrinkPercent = percent
	return b
}

// SetMinShrinkCapacity sets the minimum capacity after shrinking.
func (b *poolConfigBuilder) SetMinShrinkCapacity(minCap int) *poolConfigBuilder {
	b.config.shrink.minCapacity = minCap
	return b
}

// SetMaxConsecutiveShrinks sets the maximum number of consecutive shrink operations.
func (b *poolConfigBuilder) SetMaxConsecutiveShrinks(count int) *poolConfigBuilder {
	b.config.shrink.maxConsecutiveShrinks = count
	return b
}

// SetFastPathInitialSize sets the initial size of the fast path buffer.
func (b *poolConfigBuilder) SetFastPathInitialSize(count int) *poolConfigBuilder {
	b.config.fastPath.initialSize = count
	return b
}

// SetFastPathFillAggressiveness sets how aggressively the fast path is filled.
func (b *poolConfigBuilder) SetFastPathFillAggressiveness(percent float64) *poolConfigBuilder {
	b.config.fastPath.fillAggressiveness = percent
	return b
}

// SetFastPathRefillPercent sets the refill threshold for the fast path.
func (b *poolConfigBuilder) SetFastPathRefillPercent(percent float64) *poolConfigBuilder {
	b.config.fastPath.refillPercent = percent
	return b
}

// SetFastPathEnableChannelGrowth enables or disables dynamic growth of the fast path channel.
func (b *poolConfigBuilder) SetFastPathEnableChannelGrowth(enable bool) *poolConfigBuilder {
	b.config.fastPath.enableChannelGrowth = enable
	return b
}

// SetFastPathGrowthEventsTrigger sets the number of growth events before fast path grows.
func (b *poolConfigBuilder) SetFastPathGrowthEventsTrigger(count int) *poolConfigBuilder {
	b.config.fastPath.growthEventsTrigger = count
	return b
}

// SetFastPathGrowthPercent sets the growth percentage for the fast path.
func (b *poolConfigBuilder) SetFastPathGrowthPercent(percent float64) *poolConfigBuilder {
	b.config.fastPath.growth.growthPercent = percent
	return b
}

// SetFastPathExponentialThresholdFactor sets the exponential threshold factor for fast path growth.
func (b *poolConfigBuilder) SetFastPathExponentialThresholdFactor(percent float64) *poolConfigBuilder {
	b.config.fastPath.growth.exponentialThresholdFactor = percent
	return b
}

// SetFastPathFixedGrowthFactor sets the fixed growth factor for fast path growth.
func (b *poolConfigBuilder) SetFastPathFixedGrowthFactor(percent float64) *poolConfigBuilder {
	b.config.fastPath.growth.fixedGrowthFactor = percent
	return b
}

// SetFastPathShrinkEventsTrigger sets the number of shrink events before fast path shrinks.
func (b *poolConfigBuilder) SetFastPathShrinkEventsTrigger(count int) *poolConfigBuilder {
	b.config.fastPath.shrinkEventsTrigger = count
	return b
}

// SetFastPathShrinkPercent sets the shrink percentage for the fast path.
func (b *poolConfigBuilder) SetFastPathShrinkPercent(percent float64) *poolConfigBuilder {
	b.config.fastPath.shrink.shrinkPercent = percent
	return b
}

// SetFastPathShrinkMinCapacity sets the minimum capacity for the fast path after shrinking.
func (b *poolConfigBuilder) SetFastPathShrinkMinCapacity(minCap int) *poolConfigBuilder {
	b.config.fastPath.shrink.minCapacity = minCap
	return b
}

// SetPreReadBlockHookAttempts sets the number of attempts to get an object from L1 in preReadBlockHook.
func (b *poolConfigBuilder) SetPreReadBlockHookAttempts(attempts int) *poolConfigBuilder {
	if attempts > 0 {
		b.config.fastPath.preReadBlockHookAttempts = attempts
	}
	return b
}

// SetRingBufferBlocking sets whether the ring buffer operates in blocking mode.
func (b *poolConfigBuilder) SetRingBufferBlocking(block bool) *poolConfigBuilder {
	b.config.ringBufferConfig.block = block
	return b
}

// SetRingBufferTimeout sets both read and write timeouts for the ring buffer.
func (b *poolConfigBuilder) SetRingBufferTimeout(d time.Duration) *poolConfigBuilder {
	if d > 0 {
		b.config.ringBufferConfig.rTimeout = d
		b.config.ringBufferConfig.wTimeout = d
	}
	return b
}

// SetRingBufferReadTimeout sets the read timeout for the ring buffer.
func (b *poolConfigBuilder) SetRingBufferReadTimeout(d time.Duration) *poolConfigBuilder {
	if d > 0 {
		b.config.ringBufferConfig.rTimeout = d
	}
	return b
}

// SetRingBufferWriteTimeout sets the write timeout for the ring buffer.
func (b *poolConfigBuilder) SetRingBufferWriteTimeout(d time.Duration) *poolConfigBuilder {
	if d > 0 {
		b.config.ringBufferConfig.wTimeout = d
	}
	return b
}

// SetEnableStats enables or disables the collection of non-essential pool statistics.
func (b *poolConfigBuilder) SetEnableStats(enable bool) *poolConfigBuilder {
	b.config.enableStats = enable
	return b
}

// Build creates a new pool configuration with the configured settings.
// It validates all configuration parameters and returns an error if any validation fails.
func (b *poolConfigBuilder) Build() (*PoolConfig, error) {
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
