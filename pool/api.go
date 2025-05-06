package pool

import "time"

type PoolObj[T any] interface {
	Get() (T, error)
	Put(T) error
	Close() error
	PrintPoolStats()
}

type PoolConfigBuilder interface {
	// bulk methods
	//
	// SetPoolBasicConfigs sets the basic configuration parameters for the pool.
	// Parameters:
	//   - initialCapacity: Initial size of both ring buffer and fast path (can be overridden)
	//   - hardLimit: Maximum number of objects the pool can grow to
	//   - verbose: Enable detailed logging of pool operations
	//   - enableChannelGrowth: Enable dynamic growth of the fast path channel
	//   - enableStats: Enable collection of non-essential pool statistics
	//
	// Note: Zero or negative values are ignored, default values will be used instead.
	SetPoolBasicConfigs(initialCapacity int, hardLimit int, verbose, enableChannelGrowth, enableStats bool) PoolConfigBuilder

	// SetRingBufferBasicConfigs sets the basic configuration parameters for the ring buffer.
	// Parameters:
	//   - block: Whether operations should block when buffer is full/empty
	//   - rTimeout: Read operation timeout
	//   - wTimeout: Write operation timeout
	//   - bothTimeout: Sets both read and write timeouts to the same value
	//
	// Note: Timeout values must be positive to take effect.
	SetRingBufferBasicConfigs(block bool, rTimeout, wTimeout, bothTimeout time.Duration) PoolConfigBuilder

	// SetRingBufferGrowthConfigs sets the growth configuration parameters for the ring buffer.
	// Parameters:
	//   - exponentialThresholdFactor: Threshold for switching from exponential to fixed growth
	//   - growthPercent: Percentage growth rate when below threshold
	//   - fixedGrowthFactor: Fixed step size for growth when above threshold
	//
	// Note: Zero or negative values are ignored, default values will be used instead.
	SetRingBufferGrowthConfigs(exponentialThresholdFactor float64, growthPercent float64, fixedGrowthFactor float64) PoolConfigBuilder

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
	SetRingBufferShrinkConfigs(checkInterval, idleThreshold, shrinkCooldown time.Duration, minIdleBeforeShrink, stableUnderutilizationRounds, minCapacity, maxConsecutiveShrinks int, minUtilizationBeforeShrink, shrinkPercent float64) PoolConfigBuilder

	// EnforceCustomConfig disables default shrink configuration, requiring manual setting
	// of all shrink parameters. This is useful when you need precise control over
	// the shrinking behavior and don't want to use the preset aggressiveness levels.
	EnforceCustomConfig() PoolConfigBuilder

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
	SetShrinkAggressiveness(level AggressivenessLevel) (PoolConfigBuilder, error)

	// SetFastPathBasicConfigs sets the basic configuration parameters for the fast path (L1 cache).
	// Parameters:
	//   - initialSize: Initial capacity of the fast path buffer
	//   - growthEventsTrigger: Number of growth events before fast path grows
	//   - shrinkEventsTrigger: Number of shrink events before fast path shrinks
	//   - fillAggressiveness: How aggressively to fill the fast path initially
	//   - refillPercent: Threshold for refilling the fast path
	SetFastPathBasicConfigs(initialSize int, growthEventsTrigger int, shrinkEventsTrigger int, fillAggressiveness, refillPercent float64) PoolConfigBuilder

	// SetFastPathGrowthConfigs sets the growth configuration parameters for the fast path.
	// Parameters:
	//   - exponentialThresholdFactor: Threshold for switching growth modes
	//   - fixedGrowthFactor: Fixed step size for growth above threshold
	//   - growthPercent: Percentage growth rate below threshold
	SetFastPathGrowthConfigs(exponentialThresholdFactor float64, fixedGrowthFactor float64, growthPercent float64) PoolConfigBuilder

	// SetFastPathShrinkConfigs sets the shrink configuration parameters for the fast path.
	// Parameters:
	//   - shrinkPercent: Percentage by which to shrink the fast path
	//   - minCapacity: Minimum capacity after shrinking
	SetFastPathShrinkConfigs(shrinkPercent float64, minCapacity int) PoolConfigBuilder

	// SetFastPathShrinkAggressiveness sets the shrink aggressiveness level for the fast path.
	// Uses the same aggressiveness levels as the main pool (1-5).
	// Panics if:
	//   - Custom configuration is enforced
	//   - Level is out of valid range
	SetFastPathShrinkAggressiveness(level AggressivenessLevel) PoolConfigBuilder

	// Single methods
	// Default values will be applied to the remaining fields if not set.

	SetInitialCapacity(cap int) PoolConfigBuilder
	SetHardLimit(count int) PoolConfigBuilder
	SetVerbose(verbose bool) PoolConfigBuilder
	SetGrowthExponentialThresholdFactor(factor float64) PoolConfigBuilder
	SetGrowthPercent(percent float64) PoolConfigBuilder
	SetFixedGrowthFactor(factor float64) PoolConfigBuilder
	SetShrinkCheckInterval(interval time.Duration) PoolConfigBuilder
	SetIdleThreshold(duration time.Duration) PoolConfigBuilder
	SetMinIdleBeforeShrink(count int) PoolConfigBuilder
	SetShrinkCooldown(duration time.Duration) PoolConfigBuilder
	SetMinUtilizationBeforeShrink(threshold float64) PoolConfigBuilder
	SetStableUnderutilizationRounds(rounds int) PoolConfigBuilder
	SetShrinkPercent(percent float64) PoolConfigBuilder
	SetMinShrinkCapacity(minCap int) PoolConfigBuilder
	SetMaxConsecutiveShrinks(count int) PoolConfigBuilder
	SetFastPathInitialSize(count int) PoolConfigBuilder
	SetFastPathFillAggressiveness(percent float64) PoolConfigBuilder
	SetFastPathRefillPercent(percent float64) PoolConfigBuilder
	SetFastPathEnableChannelGrowth(enable bool) PoolConfigBuilder
	SetFastPathGrowthEventsTrigger(count int) PoolConfigBuilder
	SetFastPathGrowthPercent(percent float64) PoolConfigBuilder
	SetFastPathExponentialThresholdFactor(percent float64) PoolConfigBuilder
	SetFastPathFixedGrowthFactor(percent float64) PoolConfigBuilder
	SetFastPathShrinkEventsTrigger(count int) PoolConfigBuilder
	SetFastPathShrinkPercent(percent float64) PoolConfigBuilder
	SetFastPathShrinkMinCapacity(minCap int) PoolConfigBuilder
	SetPreReadBlockHookAttempts(attempts int) PoolConfigBuilder
	SetRingBufferBlocking(block bool) PoolConfigBuilder
	SetRingBufferTimeout(d time.Duration) PoolConfigBuilder
	SetRingBufferReadTimeout(d time.Duration) PoolConfigBuilder
	SetRingBufferWriteTimeout(d time.Duration) PoolConfigBuilder
	SetEnableStats(enable bool) PoolConfigBuilder
	Build() (*PoolConfig, error)
}
