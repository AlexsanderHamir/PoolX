package pool

import (
	"sync"
	"sync/atomic"
	"time"
)

// Only pointers can be stored in the pool, anything else will cause an error.
// (no panic will be thrown)
type pool[T any] struct {
	cacheL1 chan T

	// hardLimitResume is used to signal goroutines waiting
	// that new objects have been returned to the pool.
	//
	// When the pool reaches a "hard limit" and no objects are available,
	// goroutines calling Get() will block on this channel until a Put()
	// call signals that an object has been returned.
	hardLimitResume chan struct{}
	pool            []T
	allocator       func() T
	cleaner         func(T)

	config          *poolConfig
	stats           *poolStats
	mu              sync.RWMutex
	cond            *sync.Cond
	isShrinkBlocked bool
	isGrowthBlocked bool
	verbose         bool
}

type shrinkDefaults struct {
	interval      time.Duration
	idle          time.Duration
	minIdle       int
	cooldown      time.Duration
	utilization   float64
	underutilized int
	percent       float64
	maxShrinks    int
}

type poolStats struct {
	// No lock needed

	objectsInUse atomic.Uint64

	// total objects in the pool and on the buffer.
	availableObjects   atomic.Uint64
	peakInUse          atomic.Uint64
	totalGets          atomic.Uint64
	totalPuts          atomic.Uint64 // not using it, just calculating value.
	totalGrowthEvents  atomic.Uint64 // not using it, just calculating value.
	totalShrinkEvents  atomic.Uint64 // not using it, just calculating value.
	consecutiveShrinks atomic.Uint64
	currentCapacity    atomic.Uint64
	blockedGets        atomic.Uint64

	currentL1Capacity     atomic.Uint64
	lastResizeAtGrowthNum atomic.Uint64
	lastResizeAtShrinkNum atomic.Uint64

	FastReturnMiss atomic.Uint64
	FastReturnHit  atomic.Uint64
	L2SplillRate   float64

	// successful Get() calls served from the fast path.
	l1HitCount atomic.Uint64

	// successful Get() calls served from the large pool.
	l2HitCount atomic.Uint64

	// number of times the allocator was invoked to create a new object
	l3MissCount atomic.Uint64

	// Config set value, never changes
	initialCapacity int

	// Lock needed
	mu          sync.RWMutex
	reqPerObj   float64
	utilization float64 // not using it, just calculating value.

	lastTimeCalledGet time.Time
	lastTimeCalledPut time.Time // not using it, just calculating value. // (useful for possibly detecting leaks) \\
	lastShrinkTime    time.Time
	lastGrowTime      time.Time // not using it, just calculating value.
}

type poolConfig struct {
	// Pool initial capacity which avoids resizing the slice,
	// until it reaches the defined capacity.
	initialCapacity int

	// hardLimit sets the maximum number of objects the pool will grow to.
	// Once reached, the pool stops growing and Get() calls block until an object is returned.
	//
	// If the pool shrinks below the hardLimit, growth is allowed again.
	//
	// ⚠️ WARNING:
	// 1. A hardLimit that's too low for your workload can cause goroutine starvation.
	// 2. A lower hardLimit relative to the number of incoming requests increases latency,
	// trading off performance for tighter memory control.
	hardLimit int

	// When the pool reaches the hardLimit, no new objects will be created.
	// Goroutines calling Get() will block and wait for existing objects to be recycled via Put().
	// To prevent Put() from blocking when sending notifications, the channel should be buffered.
	// A buffer size of at least 2 is recommended, but if you expect more than 1,000 concurrent goroutines,
	// consider increasing the buffer size accordingly. An undersized buffer may lead to dropped wake-ups
	// and prolonged blocking for some goroutines.
	hardLimitBufferSize int

	// Determines how the pool grows.
	growth *growthParameters

	// Determines how the pool shrinks.
	shrink *shrinkParameters

	// Determines how fast path is utilized.
	fastPath *fastPathParameters
}

type fastPathParameters struct {
	// bufferSize defines the capacity of the fast path channel.
	// This determines how many objects can be held in the fast path before falling back
	// to the slower, lock-protected main pool. (length <= currentCapacity)
	bufferSize int

	// growthEventsTrigger defines how many pool growth events must occur
	// before triggering a capacity increase for the L1 channel.
	// This helps align L1 growth with real demand, reducing premature allocations
	// while still adapting to sustained load.
	//
	// ⚠️ WARNING:
	// growing or shrinking the l1 buffer too often will cause goroutines to drop.
	growthEventsTrigger int

	// shrinkEventsTrigger defines how many pool shrink events must occur
	// before triggering a capacity decrease for the L1 channel.
	// This helps align L1 shrink with real demand, reducing premature allocations
	// while still adapting to sustained load.
	shrinkEventsTrigger int

	// fillAggressiveness controls how aggressively the pool refills the fast path buffer
	// from the main pool. It is a float between 0.0 and 1.0, representing the fraction of the
	// fast path buffer that should be proactively filled during initialization, and refills.
	// Example: 0.5 means fill the fast path up to 50% of its capacity.
	fillAggressiveness float64

	// refillPercent defines the minimum occupancy threshold (as a fraction of bufferSize)
	// at which the fast path buffer should be refilled from the main pool.
	// When the number of objects in the fast path drops below this percentage of its capacity,
	// a refill is triggered.
	//
	// Must be a float > 0.0 and < 1.0 (e.g., 0.99). A value of 1.0 is invalid,
	// because it would cause a refill attempt even when the buffer is completely full,
	// which is unnecessary and wasteful.
	refillPercent float64

	growth *growthParameters
	shrink *shrinkParameters

	// enableChannelGrowth allows the L1 channel (cache layer) to dynamically grow
	// by reallocating it with a larger buffer size when needed.
	// This can improve performance under high concurrency by reducing contention
	// and increasing fast-path hit rates.
	enableChannelGrowth bool
}

type shrinkParameters struct {
	// EnforceCustomConfig controls whether the pool requires explicit configuration.
	// When set to true, the user must manually provide all configuration values (e.g., shrink/growth parameters).
	// If set to false (default), the pool will fall back to built-in default configurations when values are missing.
	// This flag does not disable auto-shrink behavior—it only governs configuration strictness.
	enforceCustomConfig bool

	// AggressivenessLevel is an optional high-level control that adjusts
	// shrink sensitivity and timing behavior. Valid values range from 0 (disabled)
	// to higher levels (1–5), where higher levels cause faster and more frequent shrinking.
	// This can override individual parameter values.
	aggressivenessLevel AggressivenessLevel

	// CheckInterval controls how frequently the background shrink goroutine runs.
	// This determines how often the pool is evaluated for possible shrink conditions.
	checkInterval time.Duration

	// IdleThreshold is the minimum duration the pool must remain idle
	// (no calls to Get) before it can be considered for shrinking.
	idleThreshold time.Duration

	// MinIdleBeforeShrink defines how many consecutive idle checks
	// (based on IdleThreshold and CheckInterval) must occur before a shrink is allowed.
	// This prevents shrinking during short idle spikes.
	minIdleBeforeShrink int

	// ShrinkCooldown is the minimum amount of time that must pass between
	// two consecutive shrink operations. This prevents excessive or aggressive shrinking.
	shrinkCooldown time.Duration

	// MinUtilizationBeforeShrink defines the threshold for utilization ratio
	// (ObjectsInUse / CurrentCapacity) under which the pool is considered underutilized.
	// If the utilization stays below this value for StableUnderutilizationRounds,
	// the pool becomes a shrink candidate.
	minUtilizationBeforeShrink float64

	// StableUnderutilizationRounds defines how many consecutive background checks
	// must detect underutilization before a shrink is triggered.
	// This avoids false positives caused by temporary usage dips.
	stableUnderutilizationRounds int

	// ShrinkStepPercent determines how much of the pool should be reduced
	// when a shrink operation is triggered (e.g. 0.25 = shrink by 25%).
	shrinkPercent float64

	// MaxConsecutiveShrinks defines how many shrink operations can happen back-to-back
	// before the shrink logic pauses until a get request happens.
	// The default is 2, setting for less than two won't be allowed.
	maxConsecutiveShrinks int

	// MinCapacity defines the lowest allowed capacity after shrinking.
	// The pool will never shrink below this value, even under aggressive conditions.
	minCapacity int
}

type growthParameters struct {
	// Threshold multiplier that determines when to switch from exponential to fixed growth.
	// Once the capacity reaches (InitialCapacity * ExponentialThresholdFactor), the growth
	// strategy switches to fixed mode.
	//
	// Example:
	//   InitialCapacity = 12
	//   ExponentialThresholdFactor = 4.0
	//   Threshold = 12 * 4.0 = 48
	//
	//   → Pool grows exponentially until it reaches capacity 48,
	//     then it grows at a fixed pace.
	exponentialThresholdFactor float64

	// Growth percentage used while in exponential mode.
	// Determines how much the capacity increases as a percentage of the current capacity.
	//
	// Example:
	//   CurrentCapacity = 20
	//   GrowthPercent = 0.5 (50%)
	//   Growth = 20 * 0.5 = 10 → NewCapacity = 30
	//
	//   → Pool grows: 12 → 18 → 27 → 40 → 60 → ...
	growthPercent float64

	// Once in fixed growth mode, this fixed value is added to the current capacity
	// each time the pool grows.
	//
	// Example:
	//   InitialCapacity = 12
	//   FixedGrowthFactor = 1.0
	//   fixed step = 12 * 1.0 = 12
	//
	//   → Pool grows: 48 → 60 → 72 → ...
	fixedGrowthFactor float64
}

type Example struct {
	Name string
	Age  int
}
