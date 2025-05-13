package pool

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AlexsanderHamir/ringbuffer"
	config "github.com/AlexsanderHamir/ringbuffer/config"
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
// Type parameter T must be a pointer type.
type Pool[T any] struct {
	// This provides fast access to frequently used objects without main pool contention.
	cacheL1 *chan T

	// pool is the main storage using a ring buffer.
	// It provides efficient operations and handles the bulk of object storage.
	pool *ringbuffer.RingBuffer[T]

	mu sync.RWMutex

	refillSemaphore chan struct{}

	// shrinkCond is used for blocking shrink when it reaches maxConsecutiveShrinks
	shrinkCond *sync.Cond

	// refillCond is used for blocking multiple goroutines while one goroutine is refilling the pool
	refillCond *sync.Cond

	// stats tracks essential pool statistics for the functionallity of the pool
	stats *poolStats

	// isGrowthBlocked prevents growth operations when true
	isGrowthBlocked atomic.Bool

	// config holds all pool configuration parameters
	config *PoolConfig[T]

	// Clean up objects when they're returned to the pool
	cleaner func(T)

	// Create new objects when the pool needs to grow
	allocator func() T

	// cloneTemplate creates a shallow copy of the object provided by the allocator, any reference types will be shared,
	// delaying the initialization of the object state. (which you will be responsible for in case of reference types)
	cloneTemplate func(T) T

	// template is a template object that is used to create new objects
	template T

	// ctx and cancel manage the pool's lifecycle
	ctx    context.Context
	cancel context.CancelFunc
}

// PoolConfig defines the configuration parameters for the pool.
// It controls various aspects of pool behavior including growth, shrinking,
// and performance characteristics.
type PoolConfig[T any] struct {
	// initialCapacity sets the starting size of the pool.
	// This helps avoid initial resizing operations and provides
	// immediate capacity for expected load.
	initialCapacity int

	// hardLimit is the maximum number of objects the pool can hold.
	// When reached, Get() calls will block or return nil if the pool
	// is configured to not block. This helps prevent unbounded memory growth.
	hardLimit int

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
	ringBufferConfig *config.RingBufferConfig[T]

	// allocationStrategy configures how the pool allocates objects.
	allocationStrategy *AllocationStrategy
}

// Getter methods for PoolConfig
func (c *PoolConfig[T]) GetInitialCapacity() int {
	return c.initialCapacity
}

func (c *PoolConfig[T]) GetHardLimit() int {
	return c.hardLimit
}

func (c *PoolConfig[T]) GetGrowth() *growthParameters {
	return c.growth
}

func (c *PoolConfig[T]) GetShrink() *shrinkParameters {
	return c.shrink
}

func (c *PoolConfig[T]) GetFastPath() *fastPathParameters {
	return c.fastPath
}

func (c *PoolConfig[T]) GetRingBufferConfig() *config.RingBufferConfig[T] {
	return c.ringBufferConfig
}

// growthParameters controls how the pool expands to meet demand.
// It supports both exponential and fixed growth strategies to balance
// between rapid growth for high demand and controlled growth for stability.
type growthParameters struct {
	// thresholdFactor determines when to switch from big growth to controlled growth.
	// Once capacity reaches (InitialCapacity * ExponentialThresholdFactor),
	// growth switches to controlled mode to prevent excessive expansion.
	thresholdFactor float64

	// bigGrowthFactor defines the growth rate during exponential phase.
	// For example, 50 means increase capacity by initialCapacity * 50 each time.
	bigGrowthFactor float64

	// controlledGrowthFactor determines the fixed growth amount after big growth phase.
	// The pool grows by (InitialCapacity * FixedGrowthFactor) each time.
	controlledGrowthFactor float64
}

func (g *growthParameters) GetThresholdFactor() float64 {
	return g.thresholdFactor
}

func (g *growthParameters) GetBigGrowthFactor() float64 {
	return g.bigGrowthFactor
}

func (g *growthParameters) GetControlledGrowthFactor() float64 {
	return g.controlledGrowthFactor
}

// shrinkParameters controls how the pool contracts when demand decreases.
// It provides fine-grained control over when and how the pool shrinks,
// allowing for different balance points between memory efficiency and performance.
type shrinkParameters struct {
	// enforceCustomConfig reqTestFastPathConfigurationsres explicit configuration of all parameters.
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

	// shrinkCooldown prevents too frequent shrink operations.
	// Enforces a minimum time between consecutive shrink operations.
	shrinkCooldown time.Duration

	// minUtilizationBeforeShrink defines the utilization threshold for shrinking.
	// The pool will only shrink if its utilization falls below this value.
	// Must be between 0 and 100.
	minUtilizationBeforeShrink int

	// stableUnderutilizationRounds requires consistent underutilization before shrinking.
	// Ensures the underutilization is not just a temporary condition.
	stableUnderutilizationRounds int

	// shrinkPercent determines how much to reduce the pool size when shrinking.
	// For example, 20 means reduce capacity by 20% each time.
	// Must be between 0 and 100.
	shrinkPercent int

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

func (s *shrinkParameters) GetShrinkCooldown() time.Duration {
	return s.shrinkCooldown
}

func (s *shrinkParameters) GetMinUtilizationBeforeShrink() int {
	return s.minUtilizationBeforeShrink
}

func (s *shrinkParameters) GetStableUnderutilizationRounds() int {
	return s.stableUnderutilizationRounds
}

func (s *shrinkParameters) GetShrinkPercent() int {
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

	// fillAggressiveness controls how aggressively to fill the fast path based on percentage of capacity.
	// For example, 50 means fill the fast path to 50% of its capacity.
	// How aggressively should the fast path be filled?
	fillAggressiveness int

	// Value will be interpreted as a percentage of the fast path capacity.
	// For example, 50 means refill will be triggered when occupancy drops below 50% of the fast path capacity.
	// Below what percentage of the fast path capacity should the fast path be refilled?
	refillPercent int

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

func (f *fastPathParameters) GetFillAggressiveness() int {
	return f.fillAggressiveness
}

func (f *fastPathParameters) GetRefillPercent() int {
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

	// cooldown is the default time between shrink operations
	cooldown time.Duration

	// utilization is the default utilization threshold for shrinking
	utilization int

	// underutilized is the default number of underutilization rounds required
	underutilized int

	// percent is the default shrink percentage
	percent int

	// maxShrinks is the default maximum number of consecutive shrinks
	maxShrinks int
}

type AllocationStrategy struct {
	// The percentage of objects to preallocate at initialization
	// The percentage of objects to fill the pool up to when growing
	AllocPercent int

	// The amount of objects to create per request
	// If it exceeds the ring buffer capacity it will be adjusted to the ring buffer capacity.
	AllocAmount int
}
