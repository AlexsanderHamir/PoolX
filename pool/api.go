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
type PoolConfigBuilder interface {
	// bulk methods

	// SetPoolBasicConfigs configures the fundamental pool parameters.
	// Parameters:
	//   - initialCapacity: Initial size of both ring buffer and fast path (can be overridden)
	//   - hardLimit: Maximum number of objects the pool can grow to
	//   - verbose: Enable detailed logging of pool operations
	//   - enableChannelGrowth: Enable dynamic growth of the fast path channel
	//   - enableStats: Enable collection of non-essential pool statistics
	//
	// Note: Zero or negative values are ignored, default values will be used instead.
	SetPoolBasicConfigs(initialCapacity int, hardLimit int, verbose, enableChannelGrowth, enableStats bool) PoolConfigBuilder

	// SetRingBufferBasicConfigs configures the core behavior of the ring buffer component.
	// Parameters:
	//   - block: Whether operations should block when buffer is full/empty
	//   - rTimeout: Read operation timeout
	//   - wTimeout: Write operation timeout
	//   - bothTimeout: Sets both read and write timeouts to the same value
	//
	// Note: Timeout values must be positive to take effect.
	SetRingBufferBasicConfigs(block bool, rTimeout, wTimeout, bothTimeout time.Duration) PoolConfigBuilder

	// SetRingBufferGrowthConfigs defines how the ring buffer expands when under pressure.
	// Parameters:
	//   - exponentialThresholdFactor: Threshold for switching from exponential to fixed growth
	//   - growthPercent: Percentage growth rate when below threshold
	//   - fixedGrowthFactor: Fixed step size for growth when above threshold
	//
	// Note: Zero or negative values are ignored, default values will be used instead.
	SetRingBufferGrowthConfigs(exponentialThresholdFactor float64, growthPercent float64, fixedGrowthFactor float64) PoolConfigBuilder

	// SetRingBufferShrinkConfigs controls the automatic shrinking behavior of the ring buffer.
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
	SetShrinkAggressiveness(level AggressivenessLevel) (PoolConfigBuilder, error)

	// SetFastPathBasicConfigs configures the L1 cache (fast path) parameters.
	// Parameters:
	//   - initialSize: Initial capacity of the fast path buffer
	//   - growthEventsTrigger: Number of growth events before fast path grows
	//   - shrinkEventsTrigger: Number of shrink events before fast path shrinks
	//   - fillAggressiveness: How aggressively to fill the fast path initially
	//   - refillPercent: Threshold for refilling the fast path
	SetFastPathBasicConfigs(initialSize int, growthEventsTrigger int, shrinkEventsTrigger int, fillAggressiveness, refillPercent float64) PoolConfigBuilder

	// SetFastPathGrowthConfigs defines how the fast path expands when under pressure.
	// Parameters:
	//   - exponentialThresholdFactor: Threshold for switching growth modes
	//   - fixedGrowthFactor: Fixed step size for growth above threshold
	//   - growthPercent: Percentage growth rate below threshold
	SetFastPathGrowthConfigs(exponentialThresholdFactor float64, fixedGrowthFactor float64, growthPercent float64) PoolConfigBuilder

	// SetFastPathShrinkConfigs controls the automatic shrinking behavior of the fast path.
	// Parameters:
	//   - shrinkPercent: Percentage by which to shrink the fast path
	//   - minCapacity: Minimum capacity after shrinking
	SetFastPathShrinkConfigs(shrinkPercent float64, minCapacity int) PoolConfigBuilder

	// SetFastPathShrinkAggressiveness configures the fast path shrink behavior using predefined levels.
	// Uses the same aggressiveness levels as the main pool (1-5).
	// Panics if:
	//   - Custom configuration is enforced
	//   - Level is out of valid range (1-5)
	SetFastPathShrinkAggressiveness(level AggressivenessLevel) PoolConfigBuilder

	// Single configuration methods
	// These methods allow fine-grained control over individual parameters.
	// Default values will be applied to unset parameters.

	// SetInitialCapacity sets the initial size of the pool
	SetInitialCapacity(cap int) PoolConfigBuilder
	// SetHardLimit sets the maximum number of objects the pool can contain
	SetHardLimit(count int) PoolConfigBuilder
	// SetVerbose enables or disables detailed logging
	SetVerbose(verbose bool) PoolConfigBuilder
	// SetGrowthExponentialThresholdFactor sets the threshold for switching growth modes
	SetGrowthExponentialThresholdFactor(factor float64) PoolConfigBuilder
	// SetGrowthPercent sets the percentage growth rate for exponential growth
	SetGrowthPercent(percent float64) PoolConfigBuilder
	// SetFixedGrowthFactor sets the fixed step size for linear growth
	SetFixedGrowthFactor(factor float64) PoolConfigBuilder
	// SetShrinkCheckInterval sets the time between shrink eligibility checks
	SetShrinkCheckInterval(interval time.Duration) PoolConfigBuilder
	// SetIdleThreshold sets the minimum idle duration before shrinking
	SetIdleThreshold(duration time.Duration) PoolConfigBuilder
	// SetMinIdleBeforeShrink sets the required consecutive idle checks
	SetMinIdleBeforeShrink(count int) PoolConfigBuilder
	// SetShrinkCooldown sets the minimum time between shrink operations
	SetShrinkCooldown(duration time.Duration) PoolConfigBuilder
	// SetMinUtilizationBeforeShrink sets the utilization threshold for shrinking
	SetMinUtilizationBeforeShrink(threshold float64) PoolConfigBuilder
	// SetStableUnderutilizationRounds sets the required stable underutilization rounds
	SetStableUnderutilizationRounds(rounds int) PoolConfigBuilder
	// SetShrinkPercent sets the percentage by which to shrink
	SetShrinkPercent(percent float64) PoolConfigBuilder
	// SetMinShrinkCapacity sets the minimum capacity after shrinking
	SetMinShrinkCapacity(minCap int) PoolConfigBuilder
	// SetMaxConsecutiveShrinks sets the maximum consecutive shrink operations
	SetMaxConsecutiveShrinks(count int) PoolConfigBuilder
	// SetFastPathInitialSize sets the initial capacity of the fast path
	SetFastPathInitialSize(count int) PoolConfigBuilder
	// SetFastPathFillAggressiveness sets how aggressively to fill the fast path
	SetFastPathFillAggressiveness(percent float64) PoolConfigBuilder
	// SetFastPathRefillPercent sets the threshold for refilling the fast path
	SetFastPathRefillPercent(percent float64) PoolConfigBuilder
	// SetFastPathEnableChannelGrowth enables or disables dynamic growth of the fast path
	SetFastPathEnableChannelGrowth(enable bool) PoolConfigBuilder
	// SetFastPathGrowthEventsTrigger sets the number of growth events before fast path grows
	SetFastPathGrowthEventsTrigger(count int) PoolConfigBuilder
	// SetFastPathGrowthPercent sets the percentage growth rate for the fast path
	SetFastPathGrowthPercent(percent float64) PoolConfigBuilder
	// SetFastPathExponentialThresholdFactor sets the threshold for switching fast path growth modes
	SetFastPathExponentialThresholdFactor(percent float64) PoolConfigBuilder
	// SetFastPathFixedGrowthFactor sets the fixed step size for fast path growth
	SetFastPathFixedGrowthFactor(percent float64) PoolConfigBuilder
	// SetFastPathShrinkEventsTrigger sets the number of shrink events before fast path shrinks
	SetFastPathShrinkEventsTrigger(count int) PoolConfigBuilder
	// SetFastPathShrinkPercent sets the percentage by which to shrink the fast path
	SetFastPathShrinkPercent(percent float64) PoolConfigBuilder
	// SetFastPathShrinkMinCapacity sets the minimum capacity of the fast path after shrinking
	SetFastPathShrinkMinCapacity(minCap int) PoolConfigBuilder
	// SetPreReadBlockHookAttempts sets the number of attempts for pre-read block hooks
	SetPreReadBlockHookAttempts(attempts int) PoolConfigBuilder
	// SetRingBufferBlocking sets whether ring buffer operations should block
	SetRingBufferBlocking(block bool) PoolConfigBuilder
	// SetRingBufferTimeout sets both read and write timeouts for the ring buffer
	SetRingBufferTimeout(d time.Duration) PoolConfigBuilder
	// SetRingBufferReadTimeout sets the read timeout for the ring buffer
	SetRingBufferReadTimeout(d time.Duration) PoolConfigBuilder
	// SetRingBufferWriteTimeout sets the write timeout for the ring buffer
	SetRingBufferWriteTimeout(d time.Duration) PoolConfigBuilder
	// SetEnableStats enables or disables collection of pool statistics
	SetEnableStats(enable bool) PoolConfigBuilder
	// Build creates and returns a new PoolConfig with the specified settings
	Build() (*PoolConfig, error)
}
