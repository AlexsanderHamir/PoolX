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
	defaultAggressiveness             AggressivenessLevel = AggressivenessBalanced
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
	defaultfillAggressiveness                             = fillAggressivenessExtreme
	fillAggressivenessExtreme                             = 1.0
)

func NewPool[T any](config *poolConfig, allocator func() T, cleaner func(T)) (*pool[T], error) {
	if config == nil {
		config = &poolConfig{
			initialCapacity: defaultPoolCapacity,
			shrink:          defaultPoolShrinkParameters(),
			growth:          defaultPoolGrowthParameters(),
			fastPath:        defaultFastPathParameters(),
		}
	}

	stats := &poolStats{mu: &sync.RWMutex{}}
	stats.initialCapacity += config.initialCapacity
	stats.currentCapacity.Store(uint64(config.initialCapacity))
	stats.availableObjects.Store(uint64(config.initialCapacity))

	poolObj := pool[T]{
		cacheL1:   make(chan T, config.fastPath.bufferSize),
		allocator: allocator,
		cleaner:   cleaner,
		mu:        &sync.RWMutex{},
		config:    config,
		stats:     stats,
	}
	poolObj.cond = sync.NewCond(poolObj.mu)

	obj := poolObj.allocator()
	if reflect.TypeOf(obj).Kind() != reflect.Ptr {
		return nil, fmt.Errorf("allocator must return a pointer, got %T", obj)
	}

	fillTarget := int(float64(poolObj.config.fastPath.bufferSize) * poolObj.config.fastPath.fillAggressiveness)
	fastPathRemaining := fillTarget

	poolObj.setPoolAndBuffer(obj, &fastPathRemaining)
	for i := 1; i < config.initialCapacity; i++ {
		obj := poolObj.allocator()
		poolObj.setPoolAndBuffer(obj, &fastPathRemaining)
	}

	go poolObj.shrink()

	return &poolObj, nil
}

// func (p *pool[T]) Get() T {

// }

func (p *pool[T]) setPoolAndBuffer(obj T, fastPathRemaining *int) {
	if *fastPathRemaining > 0 {
		select {
		case p.cacheL1 <- obj:
			*fastPathRemaining--
			return
		default:
			// avoids blocking
		}
	}
	p.pool = append(p.pool, obj)
}

func (p *pool[T]) fastPath() T { return <-p.cacheL1 }

func (p *pool[T]) slowPath(isRefil bool) T {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	log.Printf("[POOL] Get() called | Current pool size: %d | In-use: %d | Capacity: %d", len(p.pool), p.stats.objectsInUse.Load(), p.stats.currentCapacity.Load())

	if len(p.pool) == 0 {
		p.grow(now)
		log.Printf("[POOL] pool was empty — triggered grow() | New pool size: %d", len(p.pool))

	} else {
		p.stats.hitCount.Add(1)
		log.Println("[POOL] Reused object from pool")
	}

	p.stats.lastTimeCalledGet = now
	if !isRefil {
		p.updateUsageStats()
		p.updateDerivedStats()
	}

	last := len(p.pool) - 1
	obj := p.pool[last]
	p.pool = p.pool[:last]

	log.Printf("[POOL] Object checked out | New pool size: %d", len(p.pool))

	// TODO -  move to get
	if p.isShrinkBlocked {
		log.Println("[POOL] Shrink logic was blocked — unblocking via cond.Broadcast()")
		p.cond.Broadcast()
	}

	return obj
}

func (p *pool[T]) slowPathBatchInternal(n int) []T {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.pool) == 0 || n > len(p.pool) {
		now := time.Now()
		p.grow(now)
	}

	start := len(p.pool) - n
	batch := p.pool[start:]
	p.pool = p.pool[:start]

	return batch
}

func (p *pool[T]) Put(obj T) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.cleaner(obj)
	p.pool = append(p.pool, obj)

	p.stats.totalPuts.Add(1)
	p.stats.objectsInUse.Add(^uint64(0))
	p.stats.lastTimeCalledPut = time.Now()

	log.Printf("[POOL] Put() called | Object returned to pool | pool size: %d | In-use: %d", len(p.pool), p.stats.objectsInUse.Load())
}

func (p *pool[T]) shrink() {
	params := p.config.shrink
	ticker := time.NewTicker(params.checkInterval)
	defer ticker.Stop()

	var (
		idleCount, underutilCount int
		idleOK, utilOK            bool
	)

	for range ticker.C {
		p.mu.Lock()

		if p.stats.consecutiveShrinks.Load() == uint64(params.maxConsecutiveShrinks) {
			log.Println("[SHRINK] Max consecutive shrinks reached — waiting for Get() call")
			p.isShrinkBlocked = true
			p.cond.Wait()
			p.isShrinkBlocked = false
		}

		if time.Since(p.stats.lastShrinkTime) < params.shrinkCooldown {
			remaining := params.shrinkCooldown - time.Since(p.stats.lastShrinkTime)
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

func (p *pool[T]) grow(now time.Time) {
	cfg := p.config.growth
	initialCap := uint64(p.config.initialCapacity)

	currentCap := p.stats.currentCapacity.Load()
	exponentialThreshold := uint64(float64(initialCap) * cfg.exponentialThresholdFactor)
	fixedStep := uint64(float64(initialCap) * cfg.fixedGrowthFactor)

	var newCapacity uint64
	if currentCap < exponentialThreshold {
		growth := maxUint32(uint64(float64(currentCap)*cfg.growthPercent), 1)
		newCapacity = currentCap + growth
		log.Printf("[GROW] Strategy: exponential | Threshold: %d | Current: %d | Growth: %d | New capacity: %d",
			exponentialThreshold, currentCap, growth, newCapacity)
	} else {
		newCapacity = currentCap + fixedStep
		log.Printf("[GROW] Strategy: fixed-step | Threshold: %d | Current: %d | Step: %d | New capacity: %d",
			exponentialThreshold, currentCap, fixedStep, newCapacity)
	}

	var newPool []T
	if len(p.pool) > 0 {
		newPool = make([]T, len(p.pool), int(newCapacity))
		copy(newPool, p.pool)
	} else {
		newPool = make([]T, 0, int(newCapacity))
	}

	newObjsQty := newCapacity - currentCap
	for range newObjsQty {
		newPool = append(newPool, p.allocator())
	}

	p.pool = newPool
	p.stats.currentCapacity.Store(newCapacity)
	p.stats.lastGrowTime = now

	p.stats.availableObjects.Add(uint64(newObjsQty))
	p.stats.missCount.Add(1)
	p.stats.totalGrowthEvents.Add(1)

	log.Printf("[GROW] Final state | New capacity: %d | Objects added: %d | Pool size after grow: %d",
		newCapacity, newObjsQty, len(p.pool))
}

func (p *shrinkParameters) ApplyDefaultsFromTable(table map[AggressivenessLevel]*shrinkDefaults) {
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

func defaultPoolShrinkParameters() *shrinkParameters {
	psp := &shrinkParameters{
		aggressivenessLevel: defaultAggressiveness,
	}

	psp.ApplyDefaults(getShrinkDefaults())

	return psp
}

func (p *shrinkParameters) ApplyDefaults(table map[AggressivenessLevel]*shrinkDefaults) {
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

func defaultPoolGrowthParameters() *growthParameters {
	return &growthParameters{
		exponentialThresholdFactor: defaultExponentialThresholdFactor,
		growthPercent:              defaultGrowthPercent,
		fixedGrowthFactor:          defaultFixedGrowthFactor,
	}
}

func (p *pool[T]) updateUsageStats() {
	currentInUse := p.stats.objectsInUse.Add(1)
	p.stats.availableObjects.Add(^uint64(0))
	p.stats.totalGets.Add(1)

	for {
		peak := p.stats.peakInUse.Load()
		if currentInUse <= peak {
			break
		}
		if p.stats.peakInUse.CompareAndSwap(peak, currentInUse) {
			break
		}
	}

	for {
		current := p.stats.consecutiveShrinks.Load()
		if current == 0 {
			break
		}
		if p.stats.consecutiveShrinks.CompareAndSwap(current, current-1) {
			break
		}
	}
}

func (p *pool[T]) updateDerivedStats() {
	hitCount := p.stats.hitCount.Load()
	missCount := p.stats.missCount.Load()
	totalGets := p.stats.totalGets.Load()
	objectsInUse := p.stats.objectsInUse.Load()
	availableObjects := p.stats.availableObjects.Load()

	totalAccesses := hitCount + missCount
	if totalAccesses > 0 {
		p.stats.hitRate = float64(hitCount) / float64(totalAccesses)
		p.stats.missRate = float64(missCount) / float64(totalAccesses)
	}

	if totalGets > 0 {
		p.stats.reuseRatio = float64(hitCount) / float64(totalGets)
	}

	totalObjects := objectsInUse + availableObjects
	if totalObjects > 0 {
		p.stats.utilizationPercentage = (float64(objectsInUse) / float64(totalObjects)) * 100
	}
}

func (p *pool[T]) performShrink(newCapacity int) {
	currentCap := int(p.stats.currentCapacity.Load())
	inUse := int(p.stats.objectsInUse.Load())
	minCap := p.config.shrink.minCapacity

	// CAUTION -  fast path containing all objects would trigger this.
	// (can use the flag indicating that the fast path is active, and get the length of the channel)
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

	// DOUBT - The fast path updates the state accurately, I fail to see how the fast path would break the shrink.
	copyRoom := newCapacity - inUse
	if copyRoom <= 0 {
		log.Printf("[SHRINK] Skipped — no room for available objects after shrink (in-use: %d, requested: %d)", inUse, newCapacity)
		return
	}

	copyCount := min(copyRoom, len(p.pool))
	newPool := make([]T, copyCount, newCapacity)
	copy(newPool, p.pool[:copyCount])
	p.pool = newPool

	log.Printf("[SHRINK] Shrinking pool → From: %d → To: %d | Preserved: %d | In-use: %d", currentCap, newCapacity, copyCount, inUse)

	// ## TODO - why the copy count ?
	p.stats.availableObjects.Store(uint64(copyCount))
	p.stats.currentCapacity.Store(uint64(newCapacity))
	p.stats.totalShrinkEvents.Add(1)
	p.stats.lastShrinkTime = time.Now()
	p.stats.consecutiveShrinks.Add(1)
}

func (p *pool[T]) IndleCheck(idles *int, shrinkPermissionIdleness *bool) {
	idleDuration := time.Since(p.stats.lastTimeCalledGet)
	threshold := p.config.shrink.idleThreshold
	required := p.config.shrink.minIdleBeforeShrink

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
	inUse := p.stats.objectsInUse.Load()
	available := p.stats.availableObjects.Load()
	total := inUse + available

	var utilization float64
	if total > 0 {
		utilization = (float64(inUse) / float64(total)) * 100
	}

	minUtil := p.config.shrink.minUtilizationBeforeShrink
	requiredRounds := p.config.shrink.stableUnderutilizationRounds

	if utilization <= minUtil {
		*underutilizationRounds += 1
		log.Printf("[SHRINK] UtilizationCheck — utilization: %.2f%% (threshold: %.2f%%) | round: %d/%d",
			utilization, minUtil, *underutilizationRounds, requiredRounds)

		if *underutilizationRounds >= requiredRounds {
			*shrinkPermissionUtilization = true
			log.Println("[SHRINK] UtilizationCheck — underutilization stable, shrink allowed")
		}
	} else {
		if *underutilizationRounds > 0 {
			*underutilizationRounds -= 1
			log.Printf("[SHRINK] UtilizationCheck — usage recovered, reducing rounds: %d/%d",
				*underutilizationRounds, requiredRounds)
		} else {
			log.Printf("[SHRINK] UtilizationCheck — usage healthy: %.2f%% > %.2f%%", utilization, minUtil)
		}
	}
}

func (p *pool[T]) ShrinkExecution() {
	currentCap := p.stats.currentCapacity.Load()
	minCap := p.config.shrink.minCapacity
	inUse := int(p.stats.objectsInUse.Load())

	newCapacity := int(float64(currentCap) * (1.0 - p.config.shrink.shrinkPercent))

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

func maxUint32(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

func defaultFastPathParameters() *fastPathParameters {
	return &fastPathParameters{
		bufferSize:         defaultPoolCapacity,
		fillAggressiveness: defaultfillAggressiveness,
	}
}
