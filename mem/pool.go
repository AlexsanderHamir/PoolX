package mem

import "sync"

type Pool struct {
	allocator func() any
	cleaner   func(any)
	pool      []any
	mu        *sync.Mutex
	config    *PoolConfig
}

type PoolConfig struct {
	// Initial pool capacity.
	// A number too large can waste memory,
	// while a number too small may cause frequent growth and performance hits.
	InitialCapacity int
	CurrentCapacity int

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
}

func (pObj *Pool) get() any {
	pObj.mu.Lock()
	defer pObj.mu.Unlock()

	if len(pObj.pool) == 0 {
		pObj.grow()
	}

	obj := pObj.pool[len(pObj.pool)-1]
	pObj.pool = pObj.pool[:len(pObj.pool)-1]

	return obj
}

func (pObj *Pool) put(obj any) {
	pObj.mu.Lock()
	defer pObj.mu.Unlock()

	pObj.cleaner(obj)
	pObj.pool = append(pObj.pool, obj)

}

func (pObj *Pool) shrink() {
	if pObj.config.CurrentCapacity <= 1 {
		return
	}

	newCapacity := pObj.config.CurrentCapacity / 2

	newPool := make([]any, newCapacity)
	newPool = append(newPool, pObj.pool[:newCapacity]...)

	pObj.config.CurrentCapacity = newCapacity
	pObj.pool = newPool
}

func (pObj *Pool) grow() {
	exponentialThresholdFactor := pObj.config.ExponentialThresholdFactor
	linearStepFactor := pObj.config.FixedGrowthFactor
	growthPercent := pObj.config.GrowthPercent

	initialCap := pObj.config.InitialCapacity
	exponentialThreshold := int(float64(initialCap) * exponentialThresholdFactor)
	linearStep := int(float64(initialCap) * linearStepFactor)

	var newCapacity int
	if pObj.config.CurrentCapacity < exponentialThreshold {
		growth := max(int(float64(pObj.config.CurrentCapacity)*growthPercent), 1)
		newCapacity = pObj.config.CurrentCapacity + growth
	} else {
		newCapacity = pObj.config.CurrentCapacity + linearStep
	}

	newPool := make([]any, len(pObj.pool), newCapacity)
	copy(newPool, pObj.pool)

	for range newCapacity - pObj.config.CurrentCapacity {
		newPool = append(newPool, pObj.allocator())
	}

	pObj.pool = newPool
	pObj.config.CurrentCapacity = newCapacity
}
