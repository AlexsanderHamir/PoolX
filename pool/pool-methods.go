package pool

import (
	"fmt"
	"log"
	"reflect"
	"sync"
	"time"
)

type aggressivenessLevel int

const (
	aggressivenessDisabled            aggressivenessLevel = 0
	aggressivenessConservative        aggressivenessLevel = 1
	aggressivenessBalanced            aggressivenessLevel = 2
	aggressivenessAggressive          aggressivenessLevel = 3
	aggressivenessVeryAggressive      aggressivenessLevel = 4
	aggressivenessExtreme             aggressivenessLevel = 5
	defaultPoolCapacity                                   = 64
	defaultExponentialThresholdFactor                     = 4.0
	defaultGrowthPercent                                  = 0.5
	defaultFixedGrowthFactor                              = 1.0
	defaultMinCapacity                                    = 8
)

func (p *pool) Get() any {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	var obj any

	log.Printf("[POOL] Get() called | Current pool size: %d | In-use: %d | Capacity: %d", len(p.pool), p.Stats.objectsInUse, p.Stats.currentCapacity)

	if len(p.pool) == 0 {
		p.grow(&now)
		log.Printf("[POOL] pool was empty — triggered grow() | New pool size: %d", len(p.pool))
	} else {
		p.Stats.hitCount++
		log.Println("[POOL] Reused object from pool")
	}

	p.Stats.lastTimeCalledGet = now
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

func (p *pool) Put(obj any) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.cleaner(obj)
	p.pool = append(p.pool, obj)

	p.Stats.totalPuts++
	p.Stats.objectsInUse--
	p.Stats.lastTimeCalledPut = time.Now()

	log.Printf("[POOL] Put() called | Object returned to pool | pool size: %d | In-use: %d", len(p.pool), p.Stats.objectsInUse)
}

func (p *pool) shrink() {
	shrinkParams := p.config.poolShrinkParameters
	ticker := time.NewTicker(shrinkParams.checkInterval)
	defer ticker.Stop()

	var (
		idles                       int
		underutilizationRounds      int
		shrinkPermissionIdleness    bool
		shrinkPermissionUtilization bool
	)

	for range ticker.C {
		p.mu.Lock()

		if p.Stats.consecutiveShrinks == uint64(shrinkParams.maxConsecutiveShrinks) {
			log.Println("[SHRINK] BLOCKED UNTIL A GET CALL IS MADE !!!!!")
			p.isShrinkBlocked = true
			p.cond.Wait()
			p.isShrinkBlocked = false
		}

		if time.Since(p.Stats.lastShrinkTime) < shrinkParams.shrinkCooldown {
			log.Printf("[SHRINK] Cooldown active: %v remaining", shrinkParams.shrinkCooldown-time.Since(p.Stats.lastShrinkTime))
			p.mu.Unlock()
			continue
		}

		p.IndleCheck(&idles, &shrinkPermissionIdleness)
		log.Printf("[SHRINK] IdleCheck => idles: %d / minRequired: %d | permission: %v", idles, shrinkParams.minIdleBeforeShrink, shrinkPermissionIdleness)

		p.UtilizationCheck(&underutilizationRounds, &shrinkPermissionUtilization)
		log.Printf("[SHRINK] UtilizationCheck => rounds: %d / stableRequired: %d | permission: %v", underutilizationRounds, shrinkParams.stableUnderutilizationRounds, shrinkPermissionUtilization)

		if shrinkPermissionIdleness || shrinkPermissionUtilization {
			log.Println("[SHRINK] Conditions met. Executing shrink.")
			p.ShrinkExecution()

			idles, underutilizationRounds = 0, 0
			shrinkPermissionIdleness, shrinkPermissionUtilization = false, false
		}

		p.mu.Unlock()
	}
}

func (p *pool) grow(now *time.Time) {
	cfg := p.config.poolGrowthParameters
	initialCap := p.config.initialCapacity

	exponentialThreshold := int(float64(initialCap) * cfg.exponentialThresholdFactor)
	fixedStep := int(float64(initialCap) * cfg.fixedGrowthFactor)

	var newCapacity int
	if p.Stats.currentCapacity < exponentialThreshold {
		growth := max(int(float64(p.Stats.currentCapacity)*cfg.growthPercent), 1)
		newCapacity = p.Stats.currentCapacity + growth
		log.Printf("[GROW] Using exponential growth: +%d → %d", growth, newCapacity)
	} else {
		newCapacity = p.Stats.currentCapacity + fixedStep
		log.Printf("[GROW] Using fixed-step growth: +%d → %d", fixedStep, newCapacity)
	}

	newPool := make([]any, len(p.pool), newCapacity)
	copy(newPool, p.pool)

	newObjsQty := newCapacity - p.Stats.currentCapacity
	for range newObjsQty {
		newPool = append(newPool, p.allocator())
	}

	p.pool = newPool
	p.Stats.currentCapacity = newCapacity
	p.Stats.availableObjects += uint64(newObjsQty)
	p.Stats.missCount++
	p.Stats.totalGrowthEvents++
	p.Stats.lastGrowTime = *now

	log.Printf("[GROW] pool grew to capacity %d (added %d new objects)", newCapacity, newObjsQty)
}

func (p *poolShrinkParameters) ApplyDefaultsFromTable(table map[aggressivenessLevel]*shrinkDefaults) {
	if p.aggressivenessLevel < aggressivenessDisabled {
		p.aggressivenessLevel = aggressivenessDisabled
	}
	if p.aggressivenessLevel > aggressivenessExtreme {
		p.aggressivenessLevel = aggressivenessExtreme
	}
	def, ok := table[p.aggressivenessLevel]
	if !ok {
		return
	}

	p.checkInterval = def.interval
	p.idleThreshold = def.idle
	p.minIdleBeforeShrink = def.minIdle
	p.shrinkCooldown = def.cooldown
	p.minUtilizationBeforeShrink = def.utilization
	p.stableUnderutilizationRounds = def.underutilized
	p.shrinkPercent = def.percent
	p.maxConsecutiveShrinks = def.maxShrinks
}

func defaultPoolShrinkParameters() *poolShrinkParameters {
	psp := &poolShrinkParameters{
		aggressivenessLevel: aggressivenessBalanced,
	}

	psp.ApplyDefaults(getShrinkDefaults())

	return psp
}

func (p *poolShrinkParameters) ApplyDefaults(table map[aggressivenessLevel]*shrinkDefaults) {
	if p.aggressivenessLevel < aggressivenessDisabled {
		p.aggressivenessLevel = aggressivenessDisabled
	}

	if p.aggressivenessLevel > aggressivenessExtreme {
		p.aggressivenessLevel = aggressivenessExtreme
	}

	def, ok := table[p.aggressivenessLevel]
	if !ok {
		return
	}

	p.checkInterval = def.interval
	p.idleThreshold = def.idle
	p.minIdleBeforeShrink = def.minIdle
	p.shrinkCooldown = def.cooldown
	p.minUtilizationBeforeShrink = def.utilization
	p.stableUnderutilizationRounds = def.underutilized
	p.shrinkPercent = def.percent
	p.maxConsecutiveShrinks = def.maxShrinks
}

func defaultPoolGrowthParameters() *poolGrowthParameters {
	return &poolGrowthParameters{
		exponentialThresholdFactor: defaultExponentialThresholdFactor,
		growthPercent:              defaultGrowthPercent,
		fixedGrowthFactor:          defaultFixedGrowthFactor,
	}
}

func NewPool(config *poolConfig, allocator func() any, cleaner func(any)) (*pool, error) {
	if config == nil {
		config = &poolConfig{
			initialCapacity:      defaultPoolCapacity,
			poolShrinkParameters: defaultPoolShrinkParameters(),
			poolGrowthParameters: defaultPoolGrowthParameters(),
		}
	}

	poolObj := pool{
		allocator: allocator,
		cleaner:   cleaner,
		mu:        &sync.RWMutex{},
		config:    config,
		Stats:     &poolStats{currentCapacity: config.initialCapacity, availableObjects: uint64(config.initialCapacity), initialCapacity: config.initialCapacity},
	}

	poolObj.cond = sync.NewCond(poolObj.mu)

	obj := allocator()
	if reflect.TypeOf(obj).Kind() != reflect.Ptr {
		return nil, fmt.Errorf("allocator must return a pointer, got %T", obj)
	}

	poolObj.pool = append(poolObj.pool, obj)

	for i := 1; i < config.initialCapacity; i++ {
		poolObj.pool = append(poolObj.pool, allocator())
	}

	go poolObj.shrink()

	return &poolObj, nil
}

func (p *pool) updateUsageStats() {
	p.Stats.objectsInUse++
	p.Stats.availableObjects--
	p.Stats.totalGets++

	if p.Stats.objectsInUse > p.Stats.peakInUse {
		p.Stats.peakInUse = p.Stats.objectsInUse
	}

	if p.Stats.consecutiveShrinks > 0 {
		p.Stats.consecutiveShrinks--
	}
}

func (p *pool) updateDerivedStats() {
	totalAccesses := p.Stats.hitCount + p.Stats.missCount
	if totalAccesses > 0 {
		p.Stats.hitRate = float64(p.Stats.hitCount) / float64(totalAccesses)
		p.Stats.missRate = float64(p.Stats.missCount) / float64(totalAccesses)
	}

	if p.Stats.totalGets > 0 {
		p.Stats.reuseRatio = float64(p.Stats.hitCount) / float64(p.Stats.totalGets)
	}

	totalObjects := p.Stats.objectsInUse + p.Stats.availableObjects
	if totalObjects > 0 {
		p.Stats.utilizationPercentage = (float64(p.Stats.objectsInUse) / float64(totalObjects)) * 100
	}
}

func (p *pool) performShrink(newCapacity int) {
	if len(p.pool) == 0 {
		log.Println("[SHRINK] All objects in use, can't resize")
		return
	}

	if p.Stats.currentCapacity <= p.config.poolShrinkParameters.minCapacity {
		log.Println("[SHRINK] pool is at or below MinCapacity, can't resize")
		return
	}

	if newCapacity >= p.Stats.currentCapacity {
		log.Println("[SHRINK] New capacity is not smaller, skipping shrink")
		return
	}

	if newCapacity == 0 {
		log.Println("[SHRINK] New capacity is zero — invalid shrink")
		return
	}

	copyRoom := newCapacity - int(p.Stats.objectsInUse)
	copyCount := min(copyRoom, len(p.pool))

	newPool := make([]any, copyCount, newCapacity)
	copy(newPool, p.pool[:copyCount])
	p.pool = newPool

	p.Stats.availableObjects = uint64(copyCount)
	p.Stats.currentCapacity = newCapacity
	p.Stats.totalShrinkEvents++
	p.Stats.lastShrinkTime = time.Now()
	p.Stats.consecutiveShrinks++
}

func (p *pool) IndleCheck(idles *int, shrinkPermissionIdleness *bool) {
	if time.Since(p.Stats.lastTimeCalledGet) >= p.config.poolShrinkParameters.idleThreshold {
		*idles++
		if *idles >= p.config.poolShrinkParameters.minIdleBeforeShrink {
			*shrinkPermissionIdleness = true
		}
	} else {
		// Combat random drops in get calls
		if *idles > 0 {
			*idles--
		}
	}
}

func (p *pool) UtilizationCheck(underutilizationRounds *int, shrinkPermissionUtilization *bool) {
	total := p.Stats.objectsInUse + p.Stats.availableObjects
	var utilization float64
	if total > 0 {
		utilization = (float64(p.Stats.objectsInUse) / float64(total)) * 100
	}

	if utilization <= p.config.poolShrinkParameters.minUtilizationBeforeShrink {
		*underutilizationRounds++
		if *underutilizationRounds >= p.config.poolShrinkParameters.stableUnderutilizationRounds {
			*shrinkPermissionUtilization = true
		}
	} else {
		// Combat random drops in pool usage
		if *underutilizationRounds > 0 {
			*underutilizationRounds--
		}
	}
}

func (p *pool) ShrinkExecution() {
	newCapacity := int(float64(p.Stats.currentCapacity) * (1.0 - p.config.poolShrinkParameters.shrinkPercent))
	log.Printf("[SHRINK] current capacity: %d", p.Stats.currentCapacity)

	log.Printf("[SHRINK] new capacity: %d", newCapacity)
	log.Printf("[SHRINK] min capacity: %d", p.config.poolShrinkParameters.minCapacity)

	if newCapacity < p.config.poolShrinkParameters.minCapacity {
		log.Printf("[SHRINK] min capacity hit: %d", newCapacity)
		newCapacity = p.config.poolShrinkParameters.minCapacity
	}

	if newCapacity < int(p.Stats.objectsInUse) {
		log.Printf("[SHRINK] more objects in use than capacity: %d", p.Stats.objectsInUse)
		newCapacity = int(p.Stats.objectsInUse)
	}

	log.Println("[SHRINK] pool length: ", len(p.pool))

	p.performShrink(newCapacity)

}

func getShrinkDefaults() map[aggressivenessLevel]*shrinkDefaults {
	return map[aggressivenessLevel]*shrinkDefaults{
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
