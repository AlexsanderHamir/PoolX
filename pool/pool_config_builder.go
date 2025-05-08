package pool

import (
	"fmt"
	"time"
)

// poolConfigBuilder is responsible for building pool configurations with various settings.
// It provides a fluent interface for configuring all aspects of the pool's behavior,
// including growth, shrinking, fast path, and ring buffer settings.
type poolConfigBuilder struct {
	config *PoolConfig
}

// NewPoolConfigBuilder creates a new pool configuration builder with default settings.
// It initializes all configuration parameters with safe default values and sets up
// the necessary parameter relationships. Returns a builder ready for customization.
func NewPoolConfigBuilder() PoolConfigBuilder {
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

// SetInitialCapacity sets the initial capacity of the pool. This determines how many
// objects are created when the pool is first initialized.
func (b *poolConfigBuilder) SetInitialCapacity(cap int) PoolConfigBuilder {
	b.config.initialCapacity = cap
	return b
}

// SetHardLimit sets the maximum number of objects the pool can grow to. This is an
// absolute upper bound that the pool will never exceed, even during growth operations.
func (b *poolConfigBuilder) SetHardLimit(count int) PoolConfigBuilder {
	b.config.hardLimit = count
	return b
}

// SetGrowthExponentialThresholdFactor sets the threshold factor for switching from
// exponential to fixed growth. When the pool's capacity exceeds this threshold,
// growth switches from percentage-based to fixed-step growth.
func (b *poolConfigBuilder) SetGrowthExponentialThresholdFactor(factor float64) PoolConfigBuilder {
	b.config.growth.thresholdFactor = factor
	return b
}

// SetGrowthFactor sets the growth factor used in exponential growth mode.
// This determines how much the pool grows by factor when below the exponential threshold.
func (b *poolConfigBuilder) SetGrowthFactor(factor float64) PoolConfigBuilder {
	b.config.growth.bigGrowthFactor = factor
	return b
}

// SetFixedGrowthFactor sets the fixed growth factor used after exponential growth ends.
// This determines the fixed step size for growth when above the exponential threshold.
func (b *poolConfigBuilder) SetFixedGrowthFactor(factor float64) PoolConfigBuilder {
	b.config.growth.controlledGrowthFactor = factor
	return b
}

// SetShrinkCheckInterval sets how often the pool checks if it should shrink.
// This is the time between consecutive shrink eligibility checks.
func (b *poolConfigBuilder) SetShrinkCheckInterval(interval time.Duration) PoolConfigBuilder {
	b.config.shrink.checkInterval = interval
	return b
}

// SetShrinkCooldown sets the minimum time between shrink operations.
// This prevents too frequent shrinking by enforcing a cooldown period.
func (b *poolConfigBuilder) SetShrinkCooldown(duration time.Duration) PoolConfigBuilder {
	b.config.shrink.shrinkCooldown = duration
	return b
}

// SetMinUtilizationBeforeShrink sets the utilization threshold for shrinking.
// The pool will only shrink if its utilization falls below this threshold.
func (b *poolConfigBuilder) SetMinUtilizationBeforeShrink(threshold int) PoolConfigBuilder {
	b.config.shrink.minUtilizationBeforeShrink = threshold
	return b
}

// SetStableUnderutilizationRounds sets the number of rounds underutilization must be stable.
// This ensures the underutilization is consistent before triggering a shrink.
func (b *poolConfigBuilder) SetStableUnderutilizationRounds(rounds int) PoolConfigBuilder {
	b.config.shrink.stableUnderutilizationRounds = rounds
	return b
}

// SetShrinkPercent sets the percentage by which the pool shrinks.
// This determines how much capacity is reduced during shrink operations.
func (b *poolConfigBuilder) SetShrinkPercent(percent int) PoolConfigBuilder {
	b.config.shrink.shrinkPercent = percent
	return b
}

// SetMinShrinkCapacity sets the minimum capacity after shrinking.
// This ensures the pool never shrinks below this capacity.
func (b *poolConfigBuilder) SetMinShrinkCapacity(minCap int) PoolConfigBuilder {
	b.config.shrink.minCapacity = minCap
	return b
}

// SetMaxConsecutiveShrinks sets the maximum number of consecutive shrink operations.
// This prevents excessive shrinking by limiting consecutive shrink operations.
func (b *poolConfigBuilder) SetMaxConsecutiveShrinks(count int) PoolConfigBuilder {
	b.config.shrink.maxConsecutiveShrinks = count
	return b
}

// SetFastPathInitialSize sets the initial size of the fast path buffer.
// This determines the initial capacity of the L1 cache.
func (b *poolConfigBuilder) SetFastPathInitialSize(count int) PoolConfigBuilder {
	b.config.fastPath.initialSize = count
	return b
}

// SetFastPathFillAggressiveness sets how aggressively the fast path is filled.
// This determines what percentage of the fast path capacity should be filled initially.
func (b *poolConfigBuilder) SetFastPathFillAggressiveness(percent int) PoolConfigBuilder {
	b.config.fastPath.fillAggressiveness = percent
	return b
}

// SetFastPathRefillPercent sets the refill threshold for the fast path.
// When the fast path usage falls below this percentage, it will be refilled.
func (b *poolConfigBuilder) SetFastPathRefillPercent(percent int) PoolConfigBuilder {
	b.config.fastPath.refillPercent = percent
	return b
}

// SetFastPathEnableChannelGrowth enables or disables dynamic growth of the fast path channel.
// When enabled, the L1 cache can grow based on usage patterns.
func (b *poolConfigBuilder) SetFastPathEnableChannelGrowth(enable bool) PoolConfigBuilder {
	b.config.fastPath.enableChannelGrowth = enable
	return b
}

// SetFastPathGrowthEventsTrigger sets the number of growth events before fast path grows.
// This determines how many growth events must occur before the L1 cache grows.
func (b *poolConfigBuilder) SetFastPathGrowthEventsTrigger(count int) PoolConfigBuilder {
	b.config.fastPath.growthEventsTrigger = count
	return b
}

// SetFastPathGrowthPercent sets the growth percentage for the fast path.
// This determines how much the L1 cache grows by percentage.
func (b *poolConfigBuilder) SetFastPathGrowthFactor(factor float64) PoolConfigBuilder {
	b.config.fastPath.growth.bigGrowthFactor = factor
	return b
}

// SetFastPathExponentialThresholdFactor sets the exponential threshold factor for fast path growth.
// This determines when the L1 cache switches from exponential to fixed growth.
func (b *poolConfigBuilder) SetFastPathExponentialThresholdFactor(percent float64) PoolConfigBuilder {
	b.config.fastPath.growth.thresholdFactor = percent
	return b
}

// SetFastPathFixedGrowthFactor sets the fixed growth factor for fast path growth.
// This determines the fixed step size for L1 cache growth above the threshold.
func (b *poolConfigBuilder) SetFastPathFixedGrowthFactor(factor float64) PoolConfigBuilder {
	b.config.fastPath.growth.controlledGrowthFactor = factor
	return b
}

// SetFastPathShrinkEventsTrigger sets the number of shrink events before fast path shrinks.
// This determines how many shrink events must occur before the L1 cache shrinks.
func (b *poolConfigBuilder) SetFastPathShrinkEventsTrigger(count int) PoolConfigBuilder {
	b.config.fastPath.shrinkEventsTrigger = count
	return b
}

// SetFastPathShrinkPercent sets the shrink percentage for the fast path.
// This determines how much the L1 cache shrinks by percentage.
func (b *poolConfigBuilder) SetFastPathShrinkPercent(percent int) PoolConfigBuilder {
	b.config.fastPath.shrink.shrinkPercent = percent
	return b
}

// SetFastPathShrinkMinCapacity sets the minimum capacity for the fast path after shrinking.
// This ensures the L1 cache never shrinks below this capacity.
func (b *poolConfigBuilder) SetFastPathShrinkMinCapacity(minCap int) PoolConfigBuilder {
	b.config.fastPath.shrink.minCapacity = minCap
	return b
}

// SetPreReadBlockHookAttempts sets the number of attempts to get an object from L1
// in preReadBlockHook. This determines how many times to try the fast path before
// falling back to the main pool.
func (b *poolConfigBuilder) SetPreReadBlockHookAttempts(attempts int) PoolConfigBuilder {
	if attempts > 0 {
		b.config.fastPath.preReadBlockHookAttempts = attempts
	}
	return b
}

// SetRingBufferBlocking sets whether the ring buffer operates in blocking mode.
// When enabled, operations will block when the buffer is full/empty.
func (b *poolConfigBuilder) SetRingBufferBlocking(block bool) PoolConfigBuilder {
	b.config.ringBufferConfig.Block = block
	return b
}

// SetRingBufferTimeout sets both read and write timeouts for the ring buffer.
// This determines how long operations will wait before timing out.
func (b *poolConfigBuilder) SetRingBufferTimeout(d time.Duration) PoolConfigBuilder {
	if d > 0 {
		b.config.ringBufferConfig.RTimeout = d
		b.config.ringBufferConfig.WTimeout = d
	}
	return b
}

// SetRingBufferReadTimeout sets the read timeout for the ring buffer.
// This determines how long read operations will wait before timing out.
func (b *poolConfigBuilder) SetRingBufferReadTimeout(d time.Duration) PoolConfigBuilder {
	if d > 0 {
		b.config.ringBufferConfig.RTimeout = d
	}
	return b
}

// SetRingBufferWriteTimeout sets the write timeout for the ring buffer.
// This determines how long write operations will wait before timing out.
func (b *poolConfigBuilder) SetRingBufferWriteTimeout(d time.Duration) PoolConfigBuilder {
	if d > 0 {
		b.config.ringBufferConfig.WTimeout = d
	}
	return b
}

// Build creates a new pool configuration with the configured settings.
// It validates all configuration parameters and returns an error if any validation fails.
// Returns a fully configured and validated PoolConfig instance.
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
