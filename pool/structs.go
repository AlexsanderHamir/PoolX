package pool

import (
	"context"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

// Pool is a generic object pool implementation that provides efficient object reuse.
// It uses a two-level caching strategy with a fast path (L1 cache) and a main pool.
// The pool is designed to be thread-safe and supports dynamic resizing based on usage patterns.
//
// Type parameter T must be a pointer type. Non-pointer types will cause an error.
type Pool[T any] struct {
	cacheL1 chan T         // Fast path channel for quick object access
	pool    *RingBuffer[T] // Main pool storage using a ring buffer

	mu    sync.RWMutex // Protects pool state modifications
	shrinkCond  *sync.Cond   // Condition variable for blocking operations
	stats *poolStats   // Tracks pool usage statistics

	isShrinkBlocked bool         // Prevents shrinking when true
	isGrowthBlocked bool         // Prevents growth when true
	poolType        reflect.Type // Type information for validation

	config    *PoolConfig // Pool configuration parameters
	cleaner   func(T)     // Optional cleanup function for objects
	allocator func() T    // Function to create new objects

	ctx    context.Context    // Context for managing pool lifecycle
	cancel context.CancelFunc // Function to cancel pool operations
	closed atomic.Bool        // Prevents further operations after Close() is called
}

// PoolConfig defines the configuration parameters for the pool.
// It controls various aspects of pool behavior including growth, shrinking,
// and performance characteristics.
type PoolConfig struct {
	// initialCapacity sets the starting size of the pool.
	// This helps avoid initial resizing operations.
	initialCapacity int

	// hardLimit is the maximum number of objects the pool can hold.
	// When reached, Get() calls will block or return nil if the pool is configured to not block.
	// This helps prevent unbounded memory growth.
	hardLimit int

	// verbose enables detailed logging of pool operations
	verbose bool

	// enableStats enables the collection of non-essential pool statistics
	enableStats bool

	// growth defines how the pool expands when demand increases
	growth *growthParameters

	// shrink defines how the pool contracts when demand decreases
	shrink *shrinkParameters

	// fastPath controls the L1 cache behavior for high-performance access
	fastPath *fastPathParameters

	// ringBufferConfig configures the main pool's ring buffer
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
// It supports both exponential and fixed growth strategies.
type growthParameters struct {
	// exponentialThresholdFactor determines when to switch from exponential to fixed growth.
	// Once capacity reaches (InitialCapacity * ExponentialThresholdFactor),
	// growth switches to fixed mode to prevent excessive expansion.
	exponentialThresholdFactor float64

	// growthPercent defines the growth rate during exponential phase.
	// For example, 0.5 means increase capacity by 50% each time.
	growthPercent float64

	// fixedGrowthFactor determines the fixed growth amount after exponential phase.
	// The pool grows by (InitialCapacity * FixedGrowthFactor) each time.
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
// It provides fine-grained control over when and how the pool shrinks.
type shrinkParameters struct {
	// enforceCustomConfig requires explicit configuration of all parameters.
	// When false, uses default values for unspecified parameters.
	enforceCustomConfig bool

	// aggressivenessLevel provides a high-level control for shrink behavior.
	// Higher levels (1-5) cause more aggressive shrinking.
	aggressivenessLevel AggressivenessLevel

	// checkInterval determines how often the pool checks for shrink conditions
	checkInterval time.Duration

	// idleThreshold is the minimum time the pool must be idle before shrinking
	idleThreshold time.Duration

	// minIdleBeforeShrink requires multiple consecutive idle checks before shrinking
	minIdleBeforeShrink int

	// shrinkCooldown prevents too frequent shrink operations
	shrinkCooldown time.Duration

	// minUtilizationBeforeShrink defines the utilization threshold for shrinking
	minUtilizationBeforeShrink float64

	// stableUnderutilizationRounds requires consistent underutilization before shrinking
	stableUnderutilizationRounds int

	// shrinkPercent determines how much to reduce the pool size when shrinking
	shrinkPercent float64

	// maxConsecutiveShrinks limits back-to-back shrink operations
	maxConsecutiveShrinks int

	// minCapacity sets the minimum pool size, preventing excessive shrinking
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
// The fast path provides quick access to objects without main pool contention.
type fastPathParameters struct {
	// initialSize sets the starting capacity of the fast path channel
	initialSize int

	// growthEventsTrigger determines when to increase fast path capacity
	growthEventsTrigger int

	// shrinkEventsTrigger determines when to decrease fast path capacity
	shrinkEventsTrigger int

	// fillAggressiveness controls how aggressively to fill the fast path
	// (0.0 to 1.0, representing fill percentage)
	fillAggressiveness float64

	// refillPercent triggers refill when occupancy drops below this threshold
	refillPercent float64

	// growth controls how the fast path expands
	growth *growthParameters

	// shrink controls how the fast path contracts
	shrink *shrinkParameters

	// enableChannelGrowth allows dynamic resizing of the fast path channel
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

// refillResult reports the outcome of a fast path refill operation
type refillResult struct {
	Success       bool   // Whether the refill was successful
	Reason        string // Explanation if refill failed
	ItemsMoved    int    // Number of items moved to fast path
	ItemsFailed   int    // Number of items that failed to move
	GrowthNeeded  bool   // Whether fast path needs to grow
	GrowthBlocked bool   // Whether growth is currently blocked
}

// shrinkDefaults provides default values for shrink parameters
type shrinkDefaults struct {
	interval      time.Duration // Default check interval
	idle          time.Duration // Default idle threshold
	minIdle       int           // Default minimum idle checks
	cooldown      time.Duration // Default shrink cooldown
	utilization   float64       // Default utilization threshold
	underutilized int           // Default underutilization rounds
	percent       float64       // Default shrink percentage
	maxShrinks    int           // Default maximum consecutive shrinks
}
