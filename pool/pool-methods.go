package pool

import (
	"fmt"
	"log"
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

	log.Printf("[POOL] Get() called | Current pool size: %d | In-use: %d | Capacity: %d", len(p.pool), p.Stats.ObjectsInUse, p.Stats.CurrentCapacity)

	if len(p.pool) == 0 {
		p.grow(&now)
		log.Printf("[POOL] Pool was empty — triggered grow() | New pool size: %d", len(p.pool))
	} else {
		p.Stats.HitCount++
		log.Println("[POOL] Reused object from pool")
	}

	p.Stats.LastTimeCalledGet = now
	p.updateUsageStats()
	p.updateDerivedStats()

	last := len(p.pool) - 1
	obj = p.pool[last]
	p.pool = p.pool[:last]

	log.Printf("[POOL] Object checked out | New pool size: %d", len(p.pool))

	if p.isShrinkBlocked {
		log.Println("[POOL] Shrink logic was blocked — unblocking via cond.Broadcast()")
		p.cond.Broadcast()
	}

	return obj
}

func (p *Pool) Put(obj any) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.cleaner(obj)
	p.pool = append(p.pool, obj)

	p.Stats.TotalPuts++
	p.Stats.ObjectsInUse--
	p.Stats.LastTimeCalledPut = time.Now()

	log.Printf("[POOL] Put() called | Object returned to pool | Pool size: %d | In-use: %d", len(p.pool), p.Stats.ObjectsInUse)
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

		if p.Stats.ConsecutiveShrinks == uint64(shrinkParams.MaxConsecutiveShrinks) {
			log.Println("[SHRINK] BLOCKED UNTIL A GET CALL IS MADE !!!!!")
			p.isShrinkBlocked = true
			p.cond.Wait()
			p.isShrinkBlocked = false
		}

		if time.Since(p.Stats.LastShrinkTime) < shrinkParams.ShrinkCooldown {
			log.Printf("[SHRINK] Cooldown active: %v remaining", shrinkParams.ShrinkCooldown-time.Since(p.Stats.LastShrinkTime))
			p.mu.Unlock()
			continue
		}

		p.IndleCheck(&idles, &shrinkPermissionIdleness)
		log.Printf("[SHRINK] IdleCheck => idles: %d / minRequired: %d | permission: %v", idles, shrinkParams.MinIdleBeforeShrink, shrinkPermissionIdleness)

		p.UtilizationCheck(&underutilizationRounds, &shrinkPermissionUtilization)
		log.Printf("[SHRINK] UtilizationCheck => rounds: %d / stableRequired: %d | permission: %v", underutilizationRounds, shrinkParams.StableUnderutilizationRounds, shrinkPermissionUtilization)

		if shrinkPermissionIdleness || shrinkPermissionUtilization {
			log.Println("[SHRINK] Conditions met. Executing shrink.")
			p.ShrinkExecution()

			idles = 0
			underutilizationRounds = 0
			shrinkPermissionIdleness = false
			shrinkPermissionUtilization = false
		}

		p.mu.Unlock()
	}
}

func (p *Pool) grow(now *time.Time) {
	cfg := p.config.PoolGrowthParameters
	initialCap := p.config.InitialCapacity

	exponentialThreshold := int(float64(initialCap) * cfg.ExponentialThresholdFactor)
	fixedStep := int(float64(initialCap) * cfg.FixedGrowthFactor)

	var newCapacity int
	if p.Stats.CurrentCapacity < exponentialThreshold {
		growth := max(int(float64(p.Stats.CurrentCapacity)*cfg.GrowthPercent), 1)
		newCapacity = p.Stats.CurrentCapacity + growth
		log.Printf("[GROW] Using exponential growth: +%d → %d", growth, newCapacity)
	} else {
		newCapacity = p.Stats.CurrentCapacity + fixedStep
		log.Printf("[GROW] Using fixed-step growth: +%d → %d", fixedStep, newCapacity)
	}

	newPool := make([]any, len(p.pool), newCapacity)
	copy(newPool, p.pool)

	newObjsQty := newCapacity - p.Stats.CurrentCapacity
	for range newObjsQty {
		newPool = append(newPool, p.allocator())
	}

	p.pool = newPool
	p.Stats.CurrentCapacity = newCapacity
	p.Stats.AvailableObjects += uint64(newObjsQty)
	p.Stats.MissCount++
	p.Stats.TotalGrowthEvents++
	p.Stats.LastGrowTime = *now

	log.Printf("[GROW] Pool grew to capacity %d (added %d new objects)", newCapacity, newObjsQty)
}

func (p *PoolShrinkParameters) ApplyDefaultsFromTable(table map[AggressivenessLevel]*shrinkDefaults) {
	if p.AggressivenessLevel < aggressivenessDisabled {
		p.AggressivenessLevel = aggressivenessDisabled
	}
	if p.AggressivenessLevel > aggressivenessExtreme {
		p.AggressivenessLevel = aggressivenessExtreme
	}
	def, ok := table[p.AggressivenessLevel]
	if !ok {
		return
	}

	p.CheckInterval = def.interval
	p.IdleThreshold = def.idle
	p.MinIdleBeforeShrink = def.minIdle
	p.ShrinkCooldown = def.cooldown
	p.MinUtilizationBeforeShrink = def.utilization
	p.StableUnderutilizationRounds = def.underutilized
	p.ShrinkPercent = def.percent
	p.MaxConsecutiveShrinks = def.maxShrinks
}

func DefaultPoolShrinkParameters() *PoolShrinkParameters {
	psp := &PoolShrinkParameters{
		AggressivenessLevel: aggressivenessBalanced,
	}

	psp.ApplyDefaults(getShrinkDefaults())

	return psp
}

func (p *PoolShrinkParameters) ApplyDefaults(table map[AggressivenessLevel]*shrinkDefaults) {
	if p.AggressivenessLevel < aggressivenessDisabled {
		p.AggressivenessLevel = aggressivenessDisabled
	}

	if p.AggressivenessLevel > aggressivenessExtreme {
		p.AggressivenessLevel = aggressivenessExtreme
	}

	def, ok := table[p.AggressivenessLevel]
	if !ok {
		return
	}

	p.CheckInterval = def.interval
	p.IdleThreshold = def.idle
	p.MinIdleBeforeShrink = def.minIdle
	p.ShrinkCooldown = def.cooldown
	p.MinUtilizationBeforeShrink = def.utilization
	p.StableUnderutilizationRounds = def.underutilized
	p.ShrinkPercent = def.percent
	p.MaxConsecutiveShrinks = def.maxShrinks
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

	poolObj.cond = sync.NewCond(poolObj.mu)

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

	if p.Stats.ConsecutiveShrinks > 0 {
		p.Stats.ConsecutiveShrinks--
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
	if len(p.pool) == 0 {
		log.Println("[SHRINK] All objects in use, can't resize")
		return
	}

	if p.Stats.CurrentCapacity <= p.config.PoolShrinkParameters.MinCapacity {
		log.Println("[SHRINK] Pool is at or below MinCapacity, can't resize")
		return
	}

	if newCapacity >= p.Stats.CurrentCapacity {
		log.Println("[SHRINK] New capacity is not smaller, skipping shrink")
		return
	}

	if newCapacity == 0 {
		log.Println("[SHRINK] New capacity is zero — invalid shrink")
		return
	}

	copyRoom := newCapacity - int(p.Stats.ObjectsInUse)
	copyCount := min(copyRoom, len(p.pool))

	newPool := make([]any, copyCount, newCapacity)
	copy(newPool, p.pool[:copyCount])
	p.pool = newPool

	p.Stats.AvailableObjects = uint64(copyCount)
	p.Stats.CurrentCapacity = newCapacity
	p.Stats.TotalShrinkEvents++
	p.Stats.LastShrinkTime = time.Now()
	p.Stats.ConsecutiveShrinks++
}

func (p *Pool) IndleCheck(idles *int, shrinkPermissionIdleness *bool) {
	if time.Since(p.Stats.LastTimeCalledGet) >= p.config.PoolShrinkParameters.IdleThreshold {
		*idles++
		if *idles >= p.config.PoolShrinkParameters.MinIdleBeforeShrink {
			*shrinkPermissionIdleness = true
		}
	} else {
		// Combat random drops in get calls
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
		// Combat random drops in pool usage
		if *underutilizationRounds > 0 {
			*underutilizationRounds--
		}
	}
}

func (p *Pool) ShrinkExecution() {
	newCapacity := int(float64(p.Stats.CurrentCapacity) * (1.0 - p.config.PoolShrinkParameters.ShrinkPercent))
	log.Printf("[SHRINK] current capacity: %d", p.Stats.CurrentCapacity)

	log.Printf("[SHRINK] new capacity: %d", newCapacity)
	log.Printf("[SHRINK] min capacity: %d", p.config.PoolShrinkParameters.MinCapacity)

	if newCapacity < p.config.PoolShrinkParameters.MinCapacity {
		log.Printf("[SHRINK] min capacity hit: %d", newCapacity)
		newCapacity = p.config.PoolShrinkParameters.MinCapacity
	}

	if newCapacity < int(p.Stats.ObjectsInUse) {
		log.Printf("[SHRINK] more objects in use than capacity: %d", p.Stats.ObjectsInUse)
		newCapacity = int(p.Stats.ObjectsInUse)
	}

	log.Println("[SHRINK] pool length: ", len(p.pool))

	p.performShrink(newCapacity)

}

func getShrinkDefaults() map[AggressivenessLevel]*shrinkDefaults {
	return map[AggressivenessLevel]*shrinkDefaults{
		aggressivenessDisabled: {
			0, 0, 0, 0, 0, 0, 0, 0,
		},
		aggressivenessConservative: {
			5 * time.Minute, 10 * time.Minute, 3, 10 * time.Minute, 0.20, 3, 0.10, 1,
		},
		aggressivenessBalanced: {
			2 * time.Minute, 5 * time.Minute, 2, 5 * time.Minute, 0.30, 3, 0.25, 2,
		},
		aggressivenessAggressive: {
			1 * time.Minute, 2 * time.Minute, 2, 2 * time.Minute, 0.40, 2, 0.50, 3,
		},
		aggressivenessVeryAggressive: {
			30 * time.Second, 1 * time.Minute, 1, 1 * time.Minute, 0.50, 1, 0.65, 4,
		},
		aggressivenessExtreme: {
			10 * time.Second, 20 * time.Second, 1, 30 * time.Second, 0.60, 1, 0.80, 5,
		},
	}
}
