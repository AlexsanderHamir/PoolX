package mem

import (
	"sync"
	"time"
)

// Only pointers can be stored in the pool, anything else will cause an error.
// (no panic will be thrown)
type Pool struct {
	allocator func() any
	cleaner   func(any)
	pool      []any

	// Pass nil if you would like default config.
	config *PoolConfig
	stats  *PoolStats
	mu     *sync.RWMutex
}

type PoolStats struct {
	ObjectsInUse          uint64
	UtilizationPercentage float64
	AvailableObjects      uint64
	PeakInUse             uint64

	TotalGets  uint64
	TotalPuts  uint64
	HitCount   uint64
	MissCount  uint64
	HitRate    float64
	MissRate   float64
	ReuseRatio float64

	TotalGrowthEvents uint64
	TotalShrinkEvents uint64

	CurrentCapacity int
	InitialCapacity int

	LastTimeCalledGet time.Time
	LastTimeCalledPut time.Time
	LastShrinkTime    time.Time
	LastGrowTime      time.Time

	AverageObjTimeInUse time.Duration
	MaxObjTimeInUse     time.Duration
	TimeSinceLastGet    time.Duration
	TimeSinceLastPut    time.Duration

	LeakSuspected bool
	OverAllocated bool
	HealthStatus  string
}

type PoolConfig struct {
	// Pool initial capacity which avoids resizing the slice,
	// until it reaches the defined capacity.
	InitialCapacity int

	// Determines how the pool grows.
	PoolGrowthParameters *PoolGrowthParameters

	// Determines how the pool shrinks.
	PoolShrinkParameters *PoolShrinkParameters
}

type PoolShrinkParameters struct {
	// EnableAutoShrink controls whether the pool automatically manages shrinking behavior.
	// When enabled (default), the pool uses the configured aggressiveness level
	// to determine when and how to shrink. To configure shrink behavior manually, disable this flag.
	EnableAutoShrink bool

	// AggressivenessLevel is an optional high-level control that adjusts
	// shrink sensitivity and timing behavior. Valid values range from 0 (disabled)
	// to higher levels (1–5), where higher levels cause faster and more frequent shrinking.
	// This can override individual parameter values.
	AggressivenessLevel AggressivenessLevel

	// CheckInterval controls how frequently the background shrink goroutine runs.
	// This determines how often the pool is evaluated for possible shrink conditions.
	CheckInterval time.Duration

	// IdleThreshold is the minimum duration the pool must remain idle
	// (no successful calls to Get) before it can be considered for shrinking.
	IdleThreshold time.Duration

	// MinIdleBeforeShrink defines how many consecutive idle checks
	// (based on IdleThreshold and CheckInterval) must occur before a shrink is allowed.
	// This prevents shrinking during short idle spikes.
	MinIdleBeforeShrink int

	// ShrinkCooldown is the minimum amount of time that must pass between
	// two consecutive shrink operations. This prevents excessive or aggressive shrinking.
	ShrinkCooldown time.Duration

	// MinUtilizationBeforeShrink defines the threshold for utilization ratio
	// (ObjectsInUse / CurrentCapacity) under which the pool is considered underutilized.
	// If the utilization stays below this value for StableUnderutilizationRounds,
	// the pool becomes a shrink candidate.
	MinUtilizationBeforeShrink float64

	// StableUnderutilizationRounds defines how many consecutive background checks
	// must detect underutilization before a shrink is triggered.
	// This avoids false positives caused by temporary usage dips.
	StableUnderutilizationRounds int

	// ShrinkStepPercent determines how much of the pool should be reduced
	// when a shrink operation is triggered (e.g. 0.25 = shrink by 25%).
	ShrinkPercent float64

	// MinCapacity defines the lowest allowed capacity after shrinking.
	// The pool will never shrink below this value, even under aggressive conditions.
	MinCapacity int
}

type PoolGrowthParameters struct {
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
	ExponentialThresholdFactor float64

	// Growth percentage used while in exponential mode.
	// Determines how much the capacity increases as a percentage of the current capacity.
	//
	// Example:
	//   CurrentCapacity = 20
	//   GrowthPercent = 0.5 (50%)
	//   Growth = 20 * 0.5 = 10 → NewCapacity = 30
	//
	//   → Pool grows: 12 → 18 → 27 → 40 → 60 → ...
	GrowthPercent float64

	// Once in fixed growth mode, this fixed value is added to the current capacity
	// each time the pool grows.
	//
	// Example:
	//   InitialCapacity = 12
	//   FixedGrowthFactor = 1.0
	//   fixed step = 12 * 1.0 = 12
	//
	//   → Pool grows: 48 → 60 → 72 → ...
	FixedGrowthFactor float64
}
