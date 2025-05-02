package pool

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Pool is a generic object pool implementation that provides efficient object reuse.
// It uses a two-level caching strategy with a fast path (L1 cache) and a main pool.
// The pool is designed to be thread-safe and supports dynamic resizing based on usage patterns.
//
// Key features:
// - Two-level caching (L1 cache + main pool) for optimal performance
// - Dynamic resizing based on usage patterns
// - Thread-safe operations
// - Configurable growth and shrink behavior
// - Optional object cleanup
// - Detailed statistics tracking
//
// Type parameter T must be a pointer type. Non-pointer types will cause an error.
type Pool[T any] struct {
	// cacheL1 is an atomic pointer to a channel serving as the L1 cache.
	// This provides fast access to frequently used objects without main pool contention.
	cacheL1 atomic.Pointer[chan T]

	// pool is the main storage using a ring buffer.
	// It provides efficient operations and handles the bulk of object storage.
	pool *RingBuffer[T]

	// mu protects pool state modifications and ensures thread safety
	mu sync.RWMutex

	// shrinkCond is used for blocking shrink when it reaches maxConsecutiveShrinks
	shrinkCond *sync.Cond

	// stats tracks various pool usage statistics when enabled
	stats *poolStats

	// isShrinkBlocked prevents shrinking operations when true
	isShrinkBlocked atomic.Bool

	// isGrowthBlocked prevents growth operations when true
	isGrowthBlocked atomic.Bool

	// config holds all pool configuration parameters
	config *PoolConfig

	// Clean up objects when they're returned to the pool
	cleaner func(T)

	// Create new objects when the pool needs to grow
	allocator func() T

	// ctx and cancel manage the pool's lifecycle
	ctx    context.Context
	cancel context.CancelFunc

	// closed prevents further operations after Close() is called
	closed atomic.Bool
}

// PoolConfig defines the configuration parameters for the pool.
// It controls various aspects of pool behavior including growth, shrinking,
// and performance characteristics.
type PoolConfig struct {
	// initialCapacity sets the starting size of the pool.
	// This helps avoid initial resizing operations and provides
	// immediate capacity for expected load.
	initialCapacity int

	// hardLimit is the maximum number of objects the pool can hold.
	// When reached, Get() calls will block or return nil if the pool
	// is configured to not block. This helps prevent unbounded memory growth.
	hardLimit int

	// verbose enables detailed logging of pool operations.
	// Useful for debugging and monitoring pool behavior.
	verbose bool

	// enableStats enables the collection of non-essential pool statistics.
	// These statistics help monitor pool performance and behavior.
	enableStats bool

	// growth defines how the pool expands when demand increases.
	// Controls both the growth strategy and rate.
	growth *growthParameters

	// shrink defines how the pool contracts when demand decreases.
	// Controls when and how aggressively the pool shrinks.
	shrink *shrinkParameters

	// fastPath controls the L1 cache behavior for high-performance access.
	// Manages the fast path channel size and refill behavior.
	fastPath *fastPathParameters

	// ringBufferConfig configures the main pool's ring buffer.
	// Controls blocking behavior and timeouts for the main storage.
	ringBufferConfig *ringBufferConfig
}

// Getter methods for PoolConfig
func (c *PoolConfig) GetInitialCapacity() int {
	return c.initialCapacity
}

func (c *PoolConfig) GetHardLimit() int {
	return c.hardLimit
}

func (c *PoolConfig) GetGrowth() *growthParameters {
	return c.growth
}

func (c *PoolConfig) GetShrink() *shrinkParameters {
	return c.shrink
}

func (c *PoolConfig) GetFastPath() *fastPathParameters {
	return c.fastPath
}

func (c *PoolConfig) GetRingBufferConfig() *ringBufferConfig {
	return c.ringBufferConfig
}

func (c *PoolConfig) IsVerbose() bool {
	return c.verbose
}

// growthParameters controls how the pool expands to meet demand.
// It supports both exponential and fixed growth strategies to balance
// between rapid growth for high demand and controlled growth for stability.
type growthParameters struct {
	// exponentialThresholdFactor determines when to switch from exponential to fixed growth.
	// Once capacity reaches (InitialCapacity * ExponentialThresholdFactor),
	// growth switches to fixed mode to prevent excessive expansion.
	// Must be greater than 0.
	exponentialThresholdFactor float64

	// growthPercent defines the growth rate during exponential phase.
	// For example, 0.5 means increase capacity by 50% each time.
	// Must be greater than 0.
	growthPercent float64

	// fixedGrowthFactor determines the fixed growth amount after exponential phase.
	// The pool grows by (InitialCapacity * FixedGrowthFactor) each time.
	// Must be greater than 0.
	fixedGrowthFactor float64
}

func (g *growthParameters) GetExponentialThresholdFactor() float64 {
	return g.exponentialThresholdFactor
}

func (g *growthParameters) GetGrowthPercent() float64 {
	return g.growthPercent
}

func (g *growthParameters) GetFixedGrowthFactor() float64 {
	return g.fixedGrowthFactor
}

// shrinkParameters controls how the pool contracts when demand decreases.
// It provides fine-grained control over when and how the pool shrinks,
// allowing for different balance points between memory efficiency and performance.
type shrinkParameters struct {
	// enforceCustomConfig requires explicit configuration of all parameters.
	// When false, uses default values for unspecified parameters.
	// When true, all parameters must be manually configured.
	enforceCustomConfig bool

	// aggressivenessLevel provides a high-level control for shrink behavior.
	// Levels range from 1 (Conservative) to 5 (Extreme), with higher levels
	// causing more aggressive shrinking.
	aggressivenessLevel AggressivenessLevel

	// checkInterval determines how often the pool checks for shrink conditions.
	// Shorter intervals allow faster response to decreased demand but increase overhead.
	checkInterval time.Duration

	// idleThreshold is the minimum time the pool must be idle before shrinking.
	// Helps prevent premature shrinking during temporary lulls in activity.
	idleThreshold time.Duration

	// minIdleBeforeShrink requires multiple consecutive idle checks before shrinking.
	// Provides stability by ensuring the idle state is consistent.
	minIdleBeforeShrink int

	// shrinkCooldown prevents too frequent shrink operations.
	// Enforces a minimum time between consecutive shrink operations.
	shrinkCooldown time.Duration

	// minUtilizationBeforeShrink defines the utilization threshold for shrinking.
	// The pool will only shrink if its utilization falls below this value.
	// Must be between 0 and 1.0.
	minUtilizationBeforeShrink float64

	// stableUnderutilizationRounds requires consistent underutilization before shrinking.
	// Ensures the underutilization is not just a temporary condition.
	stableUnderutilizationRounds int

	// shrinkPercent determines how much to reduce the pool size when shrinking.
	// For example, 0.2 means reduce capacity by 20% each time.
	// Must be between 0 and 1.0.
	shrinkPercent float64

	// maxConsecutiveShrinks limits back-to-back shrink operations.
	// Prevents excessive shrinking in response to temporary demand changes.
	maxConsecutiveShrinks int

	// minCapacity sets the minimum pool size, preventing excessive shrinking.
	// The pool will never shrink below this capacity.
	minCapacity int
}

func (s *shrinkParameters) GetEnforceCustomConfig() bool {
	return s.enforceCustomConfig
}

func (s *shrinkParameters) GetAggressivenessLevel() AggressivenessLevel {
	return s.aggressivenessLevel
}

func (s *shrinkParameters) GetCheckInterval() time.Duration {
	return s.checkInterval
}

func (s *shrinkParameters) GetIdleThreshold() time.Duration {
	return s.idleThreshold
}

func (s *shrinkParameters) GetMinIdleBeforeShrink() int {
	return s.minIdleBeforeShrink
}

func (s *shrinkParameters) GetShrinkCooldown() time.Duration {
	return s.shrinkCooldown
}

func (s *shrinkParameters) GetMinUtilizationBeforeShrink() float64 {
	return s.minUtilizationBeforeShrink
}

func (s *shrinkParameters) GetStableUnderutilizationRounds() int {
	return s.stableUnderutilizationRounds
}

func (s *shrinkParameters) GetShrinkPercent() float64 {
	return s.shrinkPercent
}

func (s *shrinkParameters) GetMaxConsecutiveShrinks() int {
	return s.maxConsecutiveShrinks
}

func (s *shrinkParameters) GetMinCapacity() int {
	return s.minCapacity
}

// fastPathParameters controls the L1 cache behavior for high-performance access.
// The fast path provides quick access to objects without main pool contention,
// significantly improving performance for high-frequency operations.
type fastPathParameters struct {
	// initialSize sets the starting capacity of the fast path channel.
	// This determines how many objects are immediately available in the L1 cache.
	initialSize int

	// growthEventsTrigger determines when to increase fast path capacity.
	// After this many growth events, the fast path will grow.
	growthEventsTrigger int

	// shrinkEventsTrigger determines when to decrease fast path capacity.
	// After this many shrink events, the fast path will shrink.
	shrinkEventsTrigger int

	// fillAggressiveness controls how aggressively to fill the fast path.
	// Value between 0.0 and 1.0, representing the target fill percentage.
	// Higher values mean more objects are kept in the fast path.
	fillAggressiveness float64

	// refillPercent triggers refill when occupancy drops below this threshold.
	// Value between 0.0 and 0.99, representing the minimum acceptable fill level.
	refillPercent float64

	// preReadBlockHookAttempts controls how many times to attempt getting an object
	// from L1 in preReadBlockHook before falling back to the main pool.
	preReadBlockHookAttempts int

	// growth controls how the fast path expands.
	// Uses the same growth parameters as the main pool.
	growth *growthParameters

	// shrink controls how the fast path contracts.
	// Uses the same shrink parameters as the main pool.
	shrink *shrinkParameters

	// enableChannelGrowth allows dynamic resizing of the fast path channel.
	// When enabled, the L1 cache can grow and shrink based on usage patterns.
	enableChannelGrowth bool
}

// Getter methods for fastPathParameters
func (f *fastPathParameters) GetInitialSize() int {
	return f.initialSize
}

func (f *fastPathParameters) GetFillAggressiveness() float64 {
	return f.fillAggressiveness
}

func (f *fastPathParameters) GetRefillPercent() float64 {
	return f.refillPercent
}

func (f *fastPathParameters) IsEnableChannelGrowth() bool {
	return f.enableChannelGrowth
}

func (f *fastPathParameters) GetGrowthEventsTrigger() int {
	return f.growthEventsTrigger
}

func (f *fastPathParameters) GetShrinkEventsTrigger() int {
	return f.shrinkEventsTrigger
}

func (f *fastPathParameters) GetGrowth() *growthParameters {
	return f.growth
}

func (f *fastPathParameters) GetShrink() *shrinkParameters {
	return f.shrink
}

func (f *fastPathParameters) GetPreReadBlockHookAttempts() int {
	return f.preReadBlockHookAttempts
}

// shrinkDefaults provides default values for shrink parameters.
// These defaults are used when specific parameters are not configured.
type shrinkDefaults struct {
	// interval is the default check interval for shrink operations
	interval time.Duration

	// idle is the default idle threshold before shrinking
	idle time.Duration

	// minIdle is the default minimum number of idle checks required
	minIdle int

	// cooldown is the default time between shrink operations
	cooldown time.Duration

	// utilization is the default utilization threshold for shrinking
	utilization float64

	// underutilized is the default number of underutilization rounds required
	underutilized int

	// percent is the default shrink percentage
	percent float64

	// maxShrinks is the default maximum number of consecutive shrinks
	maxShrinks int
}
