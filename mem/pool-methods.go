package mem

import (
	"fmt"
	"reflect"
	"sync"
	"time"
)

type AggressivenessLevel int

const (
	aggressivenessDisabled            AggressivenessLevel = 0
	aggressivenessConservative        AggressivenessLevel = 1
	aggressivenessBalanced            AggressivenessLevel = 2
	aggressivenessAggressive          AggressivenessLevel = 3
	aggressivenessVeryAggressive      AggressivenessLevel = 4
	aggressivenessExtreme             AggressivenessLevel = 5
	defaultPoolCapacity                                   = 64
	defaultExponentialThresholdFactor                     = 4.0
	defaultGrowthPercent                                  = 0.5
	defaultFixedGrowthFactor                              = 1.0
	defaultMinCapacity                                    = 8
)

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

// x
func (pObj *Pool) put(obj any) {
	pObj.mu.Lock()
	defer pObj.mu.Unlock()

	pObj.cleaner(obj)
	pObj.pool = append(pObj.pool, obj)
}

func (pObj *Pool) shrink() {
	if pObj.stats.CurrentCapacity <= 1 {
		return
	}

	newCapacity := pObj.stats.CurrentCapacity / 2

	newPool := make([]any, newCapacity)
	newPool = append(newPool, pObj.pool[:newCapacity]...)

	pObj.stats.CurrentCapacity = newCapacity
	pObj.pool = newPool
}

func (pObj *Pool) grow() {
	exponentialThresholdFactor := pObj.config.PoolGrowthParameters.ExponentialThresholdFactor
	fixedStepFactor := pObj.config.PoolGrowthParameters.FixedGrowthFactor
	growthPercent := pObj.config.PoolGrowthParameters.GrowthPercent

	initialCap := pObj.config.InitialCapacity
	exponentialThreshold := int(float64(initialCap) * exponentialThresholdFactor)
	fixedStep := int(float64(initialCap) * fixedStepFactor)

	var newCapacity int
	if pObj.stats.CurrentCapacity < exponentialThreshold {
		growth := max(int(float64(pObj.stats.CurrentCapacity)*growthPercent), 1)
		newCapacity = pObj.stats.CurrentCapacity + growth
	} else {
		newCapacity = pObj.stats.CurrentCapacity + fixedStep
	}

	newPool := make([]any, len(pObj.pool), newCapacity)
	copy(newPool, pObj.pool)

	for range newCapacity - pObj.stats.CurrentCapacity {
		newPool = append(newPool, pObj.allocator())
	}

	pObj.pool = newPool
	pObj.stats.CurrentCapacity = newCapacity
}

func (p *PoolShrinkParameters) ApplyDefaults() {
	if p.AggressivenessLevel < 0 {
		p.AggressivenessLevel = 0
	}
	if p.AggressivenessLevel > 5 {
		p.AggressivenessLevel = 5
	}

	switch p.AggressivenessLevel {
	case aggressivenessDisabled:
		p.EnableAutoShrink = false
		p.CheckInterval = 0
		p.IdleThreshold = 0
		p.MinIdleBeforeShrink = 0
		p.ShrinkCooldown = 0
		p.MinUtilizationBeforeShrink = 0
		p.StableUnderutilizationRounds = 0
		p.ShrinkPercent = 0

	case aggressivenessConservative:
		p.CheckInterval = 5 * time.Minute
		p.IdleThreshold = 10 * time.Minute
		p.MinIdleBeforeShrink = 3
		p.ShrinkCooldown = 10 * time.Minute
		p.MinUtilizationBeforeShrink = 0.20
		p.StableUnderutilizationRounds = 3
		p.ShrinkPercent = 0.10

	case aggressivenessBalanced:
		p.CheckInterval = 2 * time.Minute
		p.IdleThreshold = 5 * time.Minute
		p.MinIdleBeforeShrink = 2
		p.ShrinkCooldown = 5 * time.Minute
		p.MinUtilizationBeforeShrink = 0.30
		p.StableUnderutilizationRounds = 3
		p.ShrinkPercent = 0.25

	case aggressivenessAggressive:
		p.CheckInterval = 1 * time.Minute
		p.IdleThreshold = 2 * time.Minute
		p.MinIdleBeforeShrink = 2
		p.ShrinkCooldown = 2 * time.Minute
		p.MinUtilizationBeforeShrink = 0.40
		p.StableUnderutilizationRounds = 2
		p.ShrinkPercent = 0.50

	case aggressivenessVeryAggressive:
		p.CheckInterval = 30 * time.Second
		p.IdleThreshold = 1 * time.Minute
		p.MinIdleBeforeShrink = 1
		p.ShrinkCooldown = 1 * time.Minute
		p.MinUtilizationBeforeShrink = 0.50
		p.StableUnderutilizationRounds = 1
		p.ShrinkPercent = 0.65

	case aggressivenessExtreme:
		p.CheckInterval = 10 * time.Second
		p.IdleThreshold = 20 * time.Second
		p.MinIdleBeforeShrink = 1
		p.ShrinkCooldown = 30 * time.Second
		p.MinUtilizationBeforeShrink = 0.60
		p.StableUnderutilizationRounds = 1
		p.ShrinkPercent = 0.80
	}

	if p.MinCapacity == 0 {
		p.MinCapacity = defaultMinCapacity
	}
}

func DefaultPoolShrinkParameters() *PoolShrinkParameters {
	psp := &PoolShrinkParameters{
		EnableAutoShrink:    true,
		AggressivenessLevel: aggressivenessBalanced,
	}

	psp.ApplyDefaults()

	return psp
}

func DefaultPoolGrowthParameters() *PoolGrowthParameters {
	return &PoolGrowthParameters{
		ExponentialThresholdFactor: defaultExponentialThresholdFactor,
		GrowthPercent:              defaultGrowthPercent,
		FixedGrowthFactor:          defaultFixedGrowthFactor,
	}
}

func NewPool(config *PoolConfig, allocator func() any, cleaner func(any)) (*Pool, error) {
	if config == nil {
		config = &PoolConfig{
			InitialCapacity:      defaultPoolCapacity,
			PoolShrinkParameters: DefaultPoolShrinkParameters(),
			PoolGrowthParameters: DefaultPoolGrowthParameters(),
		}
	}

	poolObj := Pool{
		allocator: allocator,
		cleaner:   cleaner,
		mu:        &sync.RWMutex{},
		config:    config,
		stats:     &PoolStats{},
	}

	obj := allocator()
	if reflect.TypeOf(obj).Kind() != reflect.Ptr {
		return nil, fmt.Errorf("allocator must return a pointer, got %T", obj)
	}

	poolObj.pool = append(poolObj.pool, obj)

	for i := 1; i < config.InitialCapacity; i++ {
		poolObj.pool = append(poolObj.pool, allocator())
	}

	return &poolObj, nil
}
