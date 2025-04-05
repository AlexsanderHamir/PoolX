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
	AggressivenessDisabled            AggressivenessLevel = 0
	AggressivenessConservative        AggressivenessLevel = 1
	AggressivenessBalanced            AggressivenessLevel = 2
	AggressivenessAggressive          AggressivenessLevel = 3
	AggressivenessVeryAggressive      AggressivenessLevel = 4
	AggressivenessExtreme             AggressivenessLevel = 5
	defaultPoolCapacity                                   = 64
	defaultExponentialThresholdFactor                     = 4.0
	defaultGrowthPercent                                  = 0.5
	defaultFixedGrowthFactor                              = 1.0
	defaultMinCapacity                                    = 8
)

func (p *pool[T]) Get() T {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	log.Printf("[POOL] Get() called | Current pool size: %d | In-use: %d | Capacity: %d", len(p.pool), p.Stats.objectsInUse, p.Stats.currentCapacity)

	if len(p.pool) == 0 {
		p.grow(&now)
		log.Printf("[POOL] pool was empty — triggered grow() | New pool size: %d", len(p.pool))

		if len(p.pool) == 0 {
			var zero T
			log.Println("[POOL] grow() did not add to the pool — returning zero value")
			return zero
		}
	} else {
		p.Stats.hitCount++
		log.Println("[POOL] Reused object from pool")
	}

	p.Stats.lastTimeCalledGet = now
	p.updateUsageStats()
	p.updateDerivedStats()

	last := len(p.pool) - 1
	obj := p.pool[last]
	p.pool = p.pool[:last]

	log.Printf("[POOL] Object checked out | New pool size: %d", len(p.pool))

	if p.isShrinkBlocked {
		log.Println("[POOL] Shrink logic was blocked — unblocking via cond.Broadcast()")
		p.cond.Broadcast()
	}

	return obj
}

func (p *pool[T]) Put(obj T) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.cleaner(obj)
	p.pool = append(p.pool, obj)

	p.Stats.totalPuts++
	p.Stats.objectsInUse--
	p.Stats.lastTimeCalledPut = time.Now()

	log.Printf("[POOL] Put() called | Object returned to pool | pool size: %d | In-use: %d", len(p.pool), p.Stats.objectsInUse)
}

func (p *pool[T]) shrink() {
	params := p.config.poolShrinkParameters
	ticker := time.NewTicker(params.checkInterval)
	defer ticker.Stop()

	var (
		idleCount, underutilCount int
		idleOK, utilOK            bool
	)

	for range ticker.C {
		p.mu.Lock()

		if p.Stats.consecutiveShrinks == uint64(params.maxConsecutiveShrinks) {
			log.Println("[SHRINK] Max consecutive shrinks reached — waiting for Get() call")
			p.isShrinkBlocked = true
			p.cond.Wait()
			p.isShrinkBlocked = false
		}

		if time.Since(p.Stats.lastShrinkTime) < params.shrinkCooldown {
			remaining := params.shrinkCooldown - time.Since(p.Stats.lastShrinkTime)
			log.Printf("[SHRINK] Cooldown active: %v remaining", remaining)
			p.mu.Unlock()
			continue
		}

		p.IndleCheck(&idleCount, &idleOK)
		log.Printf("[SHRINK] IdleCheck — idles: %d / min: %d | allowed: %v", idleCount, params.minIdleBeforeShrink, idleOK)

		p.UtilizationCheck(&underutilCount, &utilOK)
		log.Printf("[SHRINK] UtilCheck — rounds: %d / required: %d | allowed: %v", underutilCount, params.stableUnderutilizationRounds, utilOK)

		if idleOK || utilOK {
			log.Println("[SHRINK] Shrink conditions met — executing shrink.")
			p.ShrinkExecution()

			idleCount, underutilCount = 0, 0
			idleOK, utilOK = false, false
		}

		p.mu.Unlock()
	}
}

func (p *pool[T]) grow(now *time.Time) {
	cfg := p.config.poolGrowthParameters
	initialCap := p.config.initialCapacity

	exponentialThreshold := int(float64(initialCap) * cfg.exponentialThresholdFactor)
	fixedStep := int(float64(initialCap) * cfg.fixedGrowthFactor)

	var newCapacity int
	if p.Stats.currentCapacity < exponentialThreshold {
		growth := max(int(float64(p.Stats.currentCapacity)*cfg.growthPercent), 1)
		newCapacity = p.Stats.currentCapacity + growth
		log.Printf("[GROW] Strategy: exponential | Threshold: %d | Current: %d | Growth: %d | New capacity: %d",
			exponentialThreshold, p.Stats.currentCapacity, growth, newCapacity)
	} else {
		newCapacity = p.Stats.currentCapacity + fixedStep
		log.Printf("[GROW] Strategy: fixed-step | Threshold: %d | Current: %d | Step: %d | New capacity: %d",
			exponentialThreshold, p.Stats.currentCapacity, fixedStep, newCapacity)
	}

	newPool := make([]T, len(p.pool), newCapacity)
	copy(newPool, p.pool)

	newObjsQty := newCapacity - p.Stats.currentCapacity
	for i := 0; i < newObjsQty; i++ {
		newPool = append(newPool, p.allocator())
	}

	p.pool = newPool
	p.Stats.currentCapacity = newCapacity
	p.Stats.availableObjects += uint64(newObjsQty)
	p.Stats.missCount++
	p.Stats.totalGrowthEvents++
	p.Stats.lastGrowTime = *now

	log.Printf("[GROW] Final state | New capacity: %d | Objects added: %d | Pool size after grow: %d", newCapacity, newObjsQty, len(p.pool))
}

func (p *poolShrinkParameters) ApplyDefaultsFromTable(table map[AggressivenessLevel]*shrinkDefaults) {
	if p.aggressivenessLevel < AggressivenessDisabled {
		p.aggressivenessLevel = AggressivenessDisabled
	}
	if p.aggressivenessLevel > AggressivenessExtreme {
		p.aggressivenessLevel = AggressivenessExtreme
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
		aggressivenessLevel: AggressivenessBalanced,
	}

	psp.ApplyDefaults(getShrinkDefaults())

	return psp
}

func (p *poolShrinkParameters) ApplyDefaults(table map[AggressivenessLevel]*shrinkDefaults) {
	if p.aggressivenessLevel < AggressivenessDisabled {
		p.aggressivenessLevel = AggressivenessDisabled
	}

	if p.aggressivenessLevel > AggressivenessExtreme {
		p.aggressivenessLevel = AggressivenessExtreme
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

func NewPool[T any](config *poolConfig, allocator func() T, cleaner func(T)) (*pool[T], error) {
	if config == nil {
		config = &poolConfig{
			initialCapacity:      defaultPoolCapacity,
			poolShrinkParameters: defaultPoolShrinkParameters(),
			poolGrowthParameters: defaultPoolGrowthParameters(),
		}
	}

	poolObj := pool[T]{
		allocator: allocator,
		cleaner:   cleaner,
		mu:        &sync.RWMutex{},
		config:    config,
		Stats: &poolStats{
			currentCapacity:  config.initialCapacity,
			availableObjects: uint64(config.initialCapacity),
			initialCapacity:  config.initialCapacity,
		},
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

func (p *pool[T]) updateUsageStats() {
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

func (p *pool[T]) updateDerivedStats() {
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

func (p *pool[T]) performShrink(newCapacity int) {
	currentCap := p.Stats.currentCapacity
	inUse := p.Stats.objectsInUse
	minCap := p.config.poolShrinkParameters.minCapacity

	if len(p.pool) == 0 {
		log.Printf("[SHRINK] Skipped — all %d objects are currently in use, no shrink possible", inUse)
		return
	}

	if currentCap <= minCap {
		log.Printf("[SHRINK] Skipped — current capacity (%d) is at or below MinCapacity (%d)", currentCap, minCap)
		return
	}

	if newCapacity >= currentCap {
		log.Printf("[SHRINK] Skipped — new capacity (%d) is not smaller than current (%d)", newCapacity, currentCap)
		return
	}

	if newCapacity == 0 {
		log.Println("[SHRINK] Skipped — new capacity is zero (invalid)")
		return
	}

	copyRoom := newCapacity - int(inUse)
	if copyRoom <= 0 {
		log.Printf("[SHRINK] Skipped — no room for available objects after shrink (in-use: %d, requested: %d)", inUse, newCapacity)
		return
	}

	copyCount := min(copyRoom, len(p.pool))
	newPool := make([]T, copyCount, newCapacity)
	copy(newPool, p.pool[:copyCount])
	p.pool = newPool

	log.Printf("[SHRINK] Shrinking pool → From: %d → To: %d | Preserved: %d | In-use: %d", currentCap, newCapacity, copyCount, inUse)

	p.Stats.availableObjects = uint64(copyCount)
	p.Stats.currentCapacity = newCapacity
	p.Stats.totalShrinkEvents++
	p.Stats.lastShrinkTime = time.Now()
	p.Stats.consecutiveShrinks++
}

func (p *pool[T]) IndleCheck(idles *int, shrinkPermissionIdleness *bool) {
	idleDuration := time.Since(p.Stats.lastTimeCalledGet)
	threshold := p.config.poolShrinkParameters.idleThreshold
	required := p.config.poolShrinkParameters.minIdleBeforeShrink

	if idleDuration >= threshold {
		*idles += 1
		if *idles >= required {
			*shrinkPermissionIdleness = true
		}
		log.Printf("[SHRINK] IdleCheck passed — idleDuration: %v | idles: %d/%d", idleDuration, *idles, required)
	} else {
		if *idles > 0 {
			*idles -= 1
		}
		log.Printf("[SHRINK] IdleCheck reset — recent activity detected | idles: %d/%d", *idles, required)
	}
}

func (p *pool[T]) UtilizationCheck(underutilizationRounds *int, shrinkPermissionUtilization *bool) {
	inUse := p.Stats.objectsInUse
	available := p.Stats.availableObjects
	total := inUse + available

	var utilization float64
	if total > 0 {
		utilization = (float64(inUse) / float64(total)) * 100
	}

	minUtil := p.config.poolShrinkParameters.minUtilizationBeforeShrink
	requiredRounds := p.config.poolShrinkParameters.stableUnderutilizationRounds

	if utilization <= minUtil {
		*underutilizationRounds += 1
		log.Printf("[SHRINK] UtilizationCheck — utilization: %.2f%% (threshold: %.2f%%) | round: %d/%d", utilization, minUtil, *underutilizationRounds, requiredRounds)

		if *underutilizationRounds >= requiredRounds {
			*shrinkPermissionUtilization = true
			log.Println("[SHRINK] UtilizationCheck — underutilization stable, shrink allowed")
		}
	} else {
		if *underutilizationRounds > 0 {
			*underutilizationRounds -= 1
			log.Printf("[SHRINK] UtilizationCheck — usage recovered, reducing rounds: %d/%d", *underutilizationRounds, requiredRounds)
		} else {
			log.Printf("[SHRINK] UtilizationCheck — usage healthy: %.2f%% > %.2f%%", utilization, minUtil)
		}
	}
}

func (p *pool[T]) ShrinkExecution() {
	currentCap := p.Stats.currentCapacity
	minCap := p.config.poolShrinkParameters.minCapacity
	inUse := int(p.Stats.objectsInUse)

	newCapacity := int(float64(currentCap) * (1.0 - p.config.poolShrinkParameters.shrinkPercent))

	log.Println("[SHRINK] ----------------------------------------")
	log.Printf("[SHRINK] Starting shrink execution")
	log.Printf("[SHRINK] Current capacity       : %d", currentCap)
	log.Printf("[SHRINK] Requested new capacity : %d", newCapacity)
	log.Printf("[SHRINK] Minimum allowed        : %d", minCap)
	log.Printf("[SHRINK] Currently in use       : %d", inUse)
	log.Printf("[SHRINK] Pool length            : %d", len(p.pool))

	if newCapacity < minCap {
		log.Printf("[SHRINK] Adjusting to min capacity: %d", minCap)
		newCapacity = minCap
	}

	if newCapacity < inUse {
		log.Printf("[SHRINK] Adjusting to match in-use objects: %d", inUse)
		newCapacity = inUse
	}

	p.performShrink(newCapacity)

	log.Printf("[SHRINK] Shrink complete — Final capacity: %d", newCapacity)
	log.Println("[SHRINK] ----------------------------------------")
}

func getShrinkDefaults() map[AggressivenessLevel]*shrinkDefaults {
	return map[AggressivenessLevel]*shrinkDefaults{
		AggressivenessDisabled: {
			0, 0, 0, 0, 0, 0, 0, 0,
		},
		AggressivenessConservative: {
			5 * time.Minute, 10 * time.Minute, 3, 10 * time.Minute, 0.20, 3, 0.10, 1,
		},
		AggressivenessBalanced: {
			2 * time.Minute, 5 * time.Minute, 2, 5 * time.Minute, 0.30, 3, 0.25, 2,
		},
		AggressivenessAggressive: {
			1 * time.Minute, 2 * time.Minute, 2, 2 * time.Minute, 0.40, 2, 0.50, 3,
		},
		AggressivenessVeryAggressive: {
			30 * time.Second, 1 * time.Minute, 1, 1 * time.Minute, 0.50, 1, 0.65, 4,
		},
		AggressivenessExtreme: {
			10 * time.Second, 20 * time.Second, 1, 30 * time.Second, 0.60, 1, 0.80, 5,
		},
	}
}
