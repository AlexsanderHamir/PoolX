package pool

import "time"

// PoolObj represents a generic object pool interface that manages a collection of reusable objects.
// Type parameter T represents the type of objects stored in the pool.
type PoolObj[T any] interface {
	// Get retrieves an object from the pool. Returns an error if the operation fails.
	Get() (T, error)
	// Put returns an object to the pool. Returns an error if the operation fails.
	Put(T) error
	// Close releases all resources associated with the pool. Returns an error if cleanup fails.
	Close() error
	// PrintPoolStats outputs current pool statistics to stdout.
	PrintPoolStats()
}

// PoolConfigBuilder provides a fluent interface for configuring object pools.
// It supports both bulk configuration methods and individual parameter settings.
type PoolConfigBuilder[T any] interface {
	// bulk methods

	// SetPoolBasicConfigs configures the fundamental pool parameters.
	// Parameters:
	//   - initialCapacity: Initial size of both ring buffer and fast path (fastpath initial capacity can be overridden)
	//   - hardLimit: Maximum number of objects the pool can grow to
	//   - enableChannelGrowth: Enable dynamic growth of the fast path channel
	//
	// Note: Zero or negative values are ignored, default values will be used instead.
	SetPoolBasicConfigs(initialCapacity int, hardLimit int, enableChannelGrowth bool) PoolConfigBuilder[T]

	// SetRingBufferBasicConfigs configures the core behavior of the ring buffer component.
	// Parameters:
	//   - block: Whether operations should block when buffer is full/empty
	//   - rTimeout: Read operation timeout
	//   - wTimeout: Write operation timeout
	//   - bothTimeout: Sets both read and write timeouts to the same value
	//
	// Note: Timeout values must be positive to take effect.
	SetRingBufferBasicConfigs(block bool, rTimeout, wTimeout, bothTimeout time.Duration) PoolConfigBuilder[T]

	// SetRingBufferGrowthConfigs defines how the ring buffer expands when under pressure.
	// Parameters:
	//   - thresholdFactor: Threshold for switching from exponential to controlled growth
	//   example: initialCapacity(8) * 3000 == 24000, the mark at which we switch to controlled growth
	//
	//   - bigGrowthFactor: Growth rate when below threshold (example: 0.75 for 75% growth)
	//   example: currentCapacity(1000) * bigGrowthFactor(0.75) = 1000 + 750 = 1750
	//
	//   - controlledGrowthFactor: smaller step size for growth when above threshold (example: 0.5 for 50% growth)
	//   example: currentCapacity(1750) * controlledGrowthFactor(0.5) = 1750 + 875 = 2625
	SetRingBufferGrowthConfigs(thresholdFactor, bigGrowthFactor, controlledGrowthFactor float64) PoolConfigBuilder[T]

	// SetRingBufferShrinkConfigs controls the automatic shrinking behavior of the ring buffer.
	// Parameters:
	//   - checkInterval: Time between shrink eligibility checks
	//   - shrinkCooldown: Minimum time between shrink operations
	//   - stableUnderutilizationRounds: Required stable underutilization rounds
	//   - minCapacity: Minimum capacity after shrinking
	//   - maxConsecutiveShrinks: Maximum consecutive shrink operations
	//   - minUtilizationBeforeShrink: Utilization threshold for shrinking
	//   - shrinkPercent: Percentage by which to shrink
	//
	// Note: Zero or negative values are ignored, default values will be used instead.
	SetRingBufferShrinkConfigs(checkInterval, shrinkCooldown time.Duration, stableUnderutilizationRounds, minCapacity, maxConsecutiveShrinks int, minUtilizationBeforeShrink, shrinkPercent int) PoolConfigBuilder[T]

	// EnforceCustomConfig disables default shrink configuration, requiring manual setting
	// of all shrink parameters. This is useful when you need precise control over
	// the shrinking behavior and don't want to use the preset aggressiveness levels.
	EnforceCustomConfig() PoolConfigBuilder[T]

	// SetShrinkAggressiveness configures the auto-shrink behavior using predefined levels.
	// Each level represents a different balance between memory efficiency and performance.
	//
	// Levels:
	//   1: Conservative - Minimal shrinking, prioritizes performance
	//   2: Moderate - Balanced approach
	//   3: Aggressive - More aggressive shrinking
	//   4: Very Aggressive - Heavy shrinking
	//   5: Extreme - Maximum shrinking
	//
	// Returns an error if:
	//   - Custom configuration is enforced
	//   - Level is out of valid range (1-5)
	SetShrinkAggressiveness(level AggressivenessLevel) (PoolConfigBuilder[T], error)

	// SetFastPathBasicConfigs configures the L1 cache (fast path) parameters.
	// Parameters:
	//   - initialSize: Initial capacity of the fast path buffer
	//   - growthEventsTrigger: Number of growth events before fast path grows
	//   - shrinkEventsTrigger: Number of shrink events before fast path shrinks
	//   - fillAggressiveness: How aggressively to fill the fast path initially (percentage of capacity)
	//   - refillPercent: Threshold for refilling the fast path (percentage of capacity)
	SetFastPathBasicConfigs(initialSize, growthEventsTrigger, shrinkEventsTrigger int, fillAggressiveness, refillPercent int) PoolConfigBuilder[T]

	// SetFastPathGrowthConfigs defines how the fast path expands when under pressure.
	// Parameters:
	//   - thresholdFactor: Threshold for switching from exponential to controlled growth
	//   example: initialCapacity(8) * 3000 == 24000, the mark at which we switch to controlled growth
	//
	//   - bigGrowthFactor: Growth rate when below threshold (example: 0.75 for 75% growth)
	//   example: currentCapacity(1000) * bigGrowthFactor(0.75) = 1000 + 750 = 1750
	//
	//   - controlledGrowthFactor: smaller step size for growth when above threshold (example: 0.5 for 50% growth)
	//   example: currentCapacity(1750) * controlledGrowthFactor(0.5) = 1750 + 875 = 2625
	SetFastPathGrowthConfigs(thresholdFactor, bigGrowthFactor, controlledGrowthFactor float64) PoolConfigBuilder[T]

	// SetFastPathShrinkConfigs controls the automatic shrinking behavior of the fast path.
	// Parameters:
	//   - shrinkPercent: Percentage by which to shrink the fast path
	//   - minCapacity: Minimum capacity after shrinking
	SetFastPathShrinkConfigs(shrinkPercent, minCapacity int) PoolConfigBuilder[T]

	// SetFastPathShrinkAggressiveness configures the fast path shrink behavior using predefined levels.
	// Uses the same aggressiveness levels as the main pool (1-5).
	// Panics if:
	//   - Custom configuration is enforced
	//   - Level is out of valid range (1-5)
	SetFastPathShrinkAggressiveness(level AggressivenessLevel) PoolConfigBuilder[T]

	// SetAllocationStrategy configures how the pool allocates objects.
	// Parameters:
	//   - allocPercent: Percentage of objects to preallocate at initialization and when growing
	//   - allocAmount: Amount of objects to create per request when L1 is empty
	SetAllocationStrategy(allocPercent int, allocAmount int) PoolConfigBuilder[T]

	// Single configuration methods
	// These methods allow fine-grained control over individual parameters.
	// Default values will be applied to unset parameters.

	// SetInitialCapacity sets the initial size of the pool
	SetInitialCapacity(cap int) PoolConfigBuilder[T]
	// SetHardLimit sets the maximum number of objects the pool can contain
	SetHardLimit(count int) PoolConfigBuilder[T]
	// SetGrowthExponentialThresholdFactor sets the threshold for switching growth modes
	SetGrowthExponentialThresholdFactor(factor float64) PoolConfigBuilder[T]
	// SetGrowthFactor sets the percentage growth rate for exponential growth
	SetGrowthFactor(factor float64) PoolConfigBuilder[T]
	// SetFixedGrowthFactor sets the fixed step size after exponential growth ends
	SetFixedGrowthFactor(factor float64) PoolConfigBuilder[T]
	// SetShrinkCheckInterval sets the time between shrink eligibility checks
	SetShrinkCheckInterval(interval time.Duration) PoolConfigBuilder[T]
	// SetShrinkCooldown sets the minimum time between shrink operations
	SetShrinkCooldown(duration time.Duration) PoolConfigBuilder[T]
	// SetMinUtilizationBeforeShrink sets the utilization threshold for shrinking
	SetMinUtilizationBeforeShrink(threshold int) PoolConfigBuilder[T]
	// SetStableUnderutilizationRounds sets the required stable underutilization rounds
	SetStableUnderutilizationRounds(rounds int) PoolConfigBuilder[T]
	// SetShrinkPercent sets the percentage by which to shrink
	SetShrinkPercent(percent int) PoolConfigBuilder[T]
	// SetMinShrinkCapacity sets the minimum capacity after shrinking
	SetMinShrinkCapacity(minCap int) PoolConfigBuilder[T]
	// SetMaxConsecutiveShrinks sets the maximum consecutive shrink operations
	SetMaxConsecutiveShrinks(count int) PoolConfigBuilder[T]
	// SetFastPathInitialSize sets the initial capacity of the fast path
	SetFastPathInitialSize(count int) PoolConfigBuilder[T]
	// SetFastPathFillAggressiveness sets how aggressively to fill the fast path
	SetFastPathFillAggressiveness(percent int) PoolConfigBuilder[T]
	// SetFastPathRefillPercent sets the threshold for refilling the fast path
	SetFastPathRefillPercent(percent int) PoolConfigBuilder[T]
	// SetFastPathEnableChannelGrowth enables or disables dynamic growth of the fast path
	SetFastPathEnableChannelGrowth(enable bool) PoolConfigBuilder[T]
	// SetFastPathGrowthEventsTrigger sets the number of growth events before fast path grows
	SetFastPathGrowthEventsTrigger(count int) PoolConfigBuilder[T]
	// SetFastPathGrowthFactor sets the growth factor for the fast path
	SetFastPathGrowthFactor(factor float64) PoolConfigBuilder[T]
	// SetFastPathExponentialThresholdFactor sets the threshold for switching fast path growth modes
	SetFastPathExponentialThresholdFactor(factor float64) PoolConfigBuilder[T]
	// SetFastPathFixedGrowthFactor sets the fixed step size for fast path growth
	SetFastPathFixedGrowthFactor(factor float64) PoolConfigBuilder[T]
	// SetFastPathShrinkEventsTrigger sets the number of shrink events before fast path shrinks
	SetFastPathShrinkEventsTrigger(count int) PoolConfigBuilder[T]
	// SetFastPathShrinkPercent sets the percentage by which to shrink the fast path
	SetFastPathShrinkPercent(percent int) PoolConfigBuilder[T]
	// SetFastPathShrinkMinCapacity sets the minimum capacity of the fast path after shrinking
	SetFastPathShrinkMinCapacity(minCap int) PoolConfigBuilder[T]
	// SetPreReadBlockHookAttempts sets the number of attempts for pre-read block hooks
	SetPreReadBlockHookAttempts(attempts int) PoolConfigBuilder[T]
	// SetRingBufferBlocking sets whether ring buffer operations should block
	SetRingBufferBlocking(block bool) PoolConfigBuilder[T]
	// SetRingBufferTimeout sets both read and write timeouts for the ring buffer
	SetRingBufferTimeout(d time.Duration) PoolConfigBuilder[T]
	// SetRingBufferReadTimeout sets the read timeout for the ring buffer
	SetRingBufferReadTimeout(d time.Duration) PoolConfigBuilder[T]
	// SetRingBufferWriteTimeout sets the write timeout for the ring buffer
	SetRingBufferWriteTimeout(d time.Duration) PoolConfigBuilder[T]
	// Build creates and returns a new PoolConfig with the specified settings
	Build() (*PoolConfig[T], error)
}
