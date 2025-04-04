package pool

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

func (p *Pool) Get() any {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	var obj any

	if len(p.pool) == 0 {
		p.grow()
		p.Stats.MissCount++
		p.Stats.TotalGrowthEvents++
		p.Stats.LastGrowTime = now
	} else {
		p.Stats.HitCount++
	}

	p.Stats.LastTimeCalledGet = now
	p.updateUsageStats()
	p.updateDerivedStats()

	last := len(p.pool) - 1
	obj = p.pool[last]
	p.pool = p.pool[:last]

	return obj
}

func (p *Pool) Put(obj any) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.cleaner(obj)
	p.pool = append(p.pool, obj)

	p.Stats.TotalPuts++
	p.Stats.LastTimeCalledPut = time.Now()
}
func (p *Pool) shrink() {
	shrinkParams := p.config.PoolShrinkParameters
	ticker := time.NewTicker(shrinkParams.CheckInterval)
	defer ticker.Stop()

	var (
		idles                       int
		underutilizationRounds      int
		shrinkPermissionIdleness    bool
		shrinkPermissionUtilization bool
	)

	for range ticker.C {
		p.mu.Lock()

		if time.Since(p.Stats.LastShrinkTime) < shrinkParams.ShrinkCooldown {
			p.mu.Unlock()
			continue
		}

		p.IndleCheck(&idles, &shrinkPermissionIdleness)
		p.UtilizationCheck(&underutilizationRounds, &shrinkPermissionUtilization)

		if shrinkPermissionIdleness && shrinkPermissionUtilization {
			p.ShrinkExecution()
		}

		idles = 0
		underutilizationRounds = 0
		shrinkPermissionIdleness = false
		shrinkPermissionUtilization = false

		p.mu.Unlock()
	}
}

func (pObj *Pool) grow() {
	exponentialThresholdFactor := pObj.config.PoolGrowthParameters.ExponentialThresholdFactor
	fixedStepFactor := pObj.config.PoolGrowthParameters.FixedGrowthFactor
	growthPercent := pObj.config.PoolGrowthParameters.GrowthPercent

	initialCap := pObj.config.InitialCapacity
	exponentialThreshold := int(float64(initialCap) * exponentialThresholdFactor)
	fixedStep := int(float64(initialCap) * fixedStepFactor)

	var newCapacity int
	if pObj.Stats.CurrentCapacity < exponentialThreshold {
		growth := max(int(float64(pObj.Stats.CurrentCapacity)*growthPercent), 1)
		newCapacity = pObj.Stats.CurrentCapacity + growth
	} else {
		newCapacity = pObj.Stats.CurrentCapacity + fixedStep
	}

	newPool := make([]any, len(pObj.pool), newCapacity)
	copy(newPool, pObj.pool)

	newObjsQty := newCapacity - pObj.Stats.CurrentCapacity
	for range newObjsQty {
		newPool = append(newPool, pObj.allocator())
	}

	pObj.pool = newPool
	pObj.Stats.CurrentCapacity = newCapacity
	pObj.Stats.AvailableObjects += uint64(newObjsQty)
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

func NewPool(config *poolConfig, allocator func() any, cleaner func(any)) (*Pool, error) {
	if config == nil {
		config = &poolConfig{
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
		Stats:     &PoolStats{CurrentCapacity: config.InitialCapacity, AvailableObjects: uint64(config.InitialCapacity), InitialCapacity: config.InitialCapacity},
	}

	obj := allocator()
	if reflect.TypeOf(obj).Kind() != reflect.Ptr {
		return nil, fmt.Errorf("allocator must return a pointer, got %T", obj)
	}

	poolObj.pool = append(poolObj.pool, obj)

	for i := 1; i < config.InitialCapacity; i++ {
		poolObj.pool = append(poolObj.pool, allocator())
	}

	go poolObj.shrink()

	return &poolObj, nil
}

func (p *Pool) updateUsageStats() {
	p.Stats.ObjectsInUse++
	p.Stats.AvailableObjects--
	p.Stats.TotalGets++

	if p.Stats.ObjectsInUse > p.Stats.PeakInUse {
		p.Stats.PeakInUse = p.Stats.ObjectsInUse
	}
}

func (p *Pool) updateDerivedStats() {
	totalAccesses := p.Stats.HitCount + p.Stats.MissCount
	if totalAccesses > 0 {
		p.Stats.HitRate = float64(p.Stats.HitCount) / float64(totalAccesses)
		p.Stats.MissRate = float64(p.Stats.MissCount) / float64(totalAccesses)
	}

	if p.Stats.TotalGets > 0 {
		p.Stats.ReuseRatio = float64(p.Stats.HitCount) / float64(p.Stats.TotalGets)
	}

	totalObjects := p.Stats.ObjectsInUse + p.Stats.AvailableObjects
	if totalObjects > 0 {
		p.Stats.UtilizationPercentage = (float64(p.Stats.ObjectsInUse) / float64(totalObjects)) * 100
	}
}

func (p *Pool) performShrink(newCapacity int) {
	copyRoom := newCapacity - int(p.Stats.ObjectsInUse)
	copyCount := min(copyRoom, len(p.pool))

	newPool := make([]any, copyCount, newCapacity)
	copy(newPool, p.pool[:copyCount])
	p.pool = newPool

	p.Stats.AvailableObjects = uint64(copyCount)
	p.Stats.CurrentCapacity = newCapacity
	p.Stats.TotalShrinkEvents++
	p.Stats.LastShrinkTime = time.Now()
}

func (p *Pool) IndleCheck(idles *int, shrinkPermissionIdleness *bool) {
	if time.Since(p.Stats.LastTimeCalledGet) >= p.config.PoolShrinkParameters.IdleThreshold {
		*idles++
		if *idles >= p.config.PoolShrinkParameters.MinIdleBeforeShrink {
			*shrinkPermissionIdleness = true
		}
	} else {
		if *idles > 0 {
			*idles--
		}
	}
}

func (p *Pool) UtilizationCheck(underutilizationRounds *int, shrinkPermissionUtilization *bool) {
	total := p.Stats.ObjectsInUse + p.Stats.AvailableObjects
	var utilization float64
	if total > 0 {
		utilization = (float64(p.Stats.ObjectsInUse) / float64(total)) * 100
	}

	if utilization <= p.config.PoolShrinkParameters.MinUtilizationBeforeShrink {
		*underutilizationRounds++
		if *underutilizationRounds >= p.config.PoolShrinkParameters.StableUnderutilizationRounds {
			*shrinkPermissionUtilization = true
		}
	} else {
		// Combat random drops in usage
		if *underutilizationRounds > 0 {
			*underutilizationRounds--
		}
	}
}

func (p *Pool) ShrinkExecution() {
	newCapacity := int(float64(p.Stats.CurrentCapacity) * (1.0 - p.config.PoolShrinkParameters.ShrinkPercent))
	if newCapacity < p.config.PoolShrinkParameters.MinCapacity {
		newCapacity = p.config.PoolShrinkParameters.MinCapacity
	}

	if newCapacity < int(p.Stats.ObjectsInUse) {
		newCapacity = int(p.Stats.ObjectsInUse)
	}

	p.performShrink(newCapacity)
}
