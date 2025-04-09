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
	defaultHardLimit                                      = 1024
	defaultExponentialThresholdFactor                     = 4.0
	defaultGrowthPercent                                  = 0.5
	defaultFixedGrowthFactor                              = 1.0
	defaultMinCapacity                                    = 8
	defaultfillAggressiveness                             = fillAggressivenessExtreme
	fillAggressivenessExtreme                             = 1.0
	defaultRefillPercent                                  = 0.5
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
		cacheL1:         make(chan T, config.fastPath.bufferSize),
		hardLimitResume: make(chan struct{}, config.hardLimit),
		allocator:       allocator,
		cleaner:         cleaner,
		mu:              &sync.RWMutex{},
		config:          config,
		stats:           stats,
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

func (p *pool[T]) Get() T {
	if p.isShrinkBlocked {
		log.Println("[POOL] Shrink logic was blocked — unblocking via cond.Broadcast()")
		p.cond.Broadcast()
	}

	// If the hard limit is reached, we block until an object is returned.
	ableToRefill := p.tryRefillIfNeeded()
	if !ableToRefill {
		log.Printf("[POOL] Hard limit reached — blocking Get()")
		p.stats.blockedGets.Add(1)
		<-p.hardLimitResume
		p.reduceBlockedGets()
	}

	select {
	case obj := <-p.cacheL1:
		p.stats.l1HitCount.Add(1)
		p.updateUsageStats()
		p.updateDerivedStats()
		return obj
	default:
		p.stats.l2HitCount.Add(1)
		log.Printf("[POOL] Slow path accessed — total L2 hits: %d", p.stats.l2HitCount.Load())
		obj := p.slowPath()
		p.updateUsageStats()
		return obj
	}
}

func (p *pool[T]) Put(obj T) {
	p.cleaner(obj)

	select {
	case p.cacheL1 <- obj:
		p.stats.FastReturnHit.Add(1)
	default:
		p.mu.Lock()
		p.stats.lastTimeCalledPut = time.Now()
		p.pool = append(p.pool, obj)
		p.mu.Unlock()
		p.stats.FastReturnMiss.Add(1)
	}

	p.stats.totalPuts.Add(1)
	p.reduceObjsInUse()
	p.updateAvailableObjs()

	if p.stats.blockedGets.Load() > 0 {
		log.Printf("[POOL] Unblocking Get()")
		p.hardLimitResume <- struct{}{}
	}

	log.Printf("[POOL] Put() called | Object returned to pool | pool size: %d | In-use: %d", len(p.pool), p.stats.objectsInUse.Load())
}

func (p *pool[T]) tryRefillIfNeeded() bool {
	bufferSize := p.config.fastPath.bufferSize
	currentLength := len(p.cacheL1)

	currentPercent := float64(currentLength) / float64(bufferSize)

	// Even though multiple goroutines block right before calling tryRefillIfNeeded
	// this condition will not be true right after a refill, so only one of them will refill.
	if currentPercent <= p.config.fastPath.refillPercent {
		fillTarget := int(float64(bufferSize)*p.config.fastPath.fillAggressiveness) - len(p.cacheL1)
		ableToRefill := p.refill(fillTarget)
		if !ableToRefill {
			return false
		}
	}

	return true
}

func (p *pool[T]) slowPath() T {
	var locked bool = true
	var now time.Time

	p.mu.Lock()
	lenPool := len(p.pool)

	// maybe we should do this in a loop.
	if lenPool == 0 {
		log.Printf("[POOL] Hard limit reached — blocking Get()")
		p.stats.blockedGets.Add(1)
		p.mu.Unlock()
		<-p.hardLimitResume
		p.reduceBlockedGets()
		locked = false
		now = time.Now()
	}

	if !locked {
		p.mu.Lock()
	}

	p.stats.lastTimeCalledGet = now
	obj := p.pool[0]
	p.pool = p.pool[1:]

	p.mu.Unlock()

	p.updateDerivedStats()
	log.Printf("[POOL] Object checked out | New pool size: %d", len(p.pool))

	return obj
}

func (p *pool[T]) refill(n int) bool {
	batch := p.slowPathGetObjBatch(n)
	if batch == nil {
		return false
	}

	p.updateAvailableObjs()

	p.fastRefill(batch)
	return true
}

func (p *pool[T]) slowPathGetObjBatch(requiredPrefill int) []T {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isGrowthBlocked {
		log.Printf("[POOL] Growth is blocked — returning nil")
		return nil
	}

	noObjsAvailable := len(p.pool) == 0
	if noObjsAvailable {
		now := time.Now()
		ableToGrow := p.grow(now)
		if !ableToGrow {
			return nil
		}

		// WARNING - refill is being called together with L1 case.
		p.reduceL1Hit()
	}

	// WARNING - if the min capacity of the pool is lower than L1 size it will cause an error.
	currentObjsAvailable := len(p.pool)
	if requiredPrefill > currentObjsAvailable {
		log.Printf("[WARN] Required prefill (%d) exceeds available objects (%d). Capping to max available.", requiredPrefill, currentObjsAvailable)
		requiredPrefill = currentObjsAvailable
	}

	start := currentObjsAvailable - requiredPrefill
	batch := p.pool[start:]
	p.pool = p.pool[:start]

	return batch
}

func (p *pool[T]) fastRefill(batch []T) {
	var fallbackCount int

	for _, obj := range batch {
		select {
		case p.cacheL1 <- obj:
		default:
			// Fast path was unexpectedly full — fall back to the slower path
			p.mu.Lock()
			p.pool = append(p.pool, obj)
			p.stats.lastTimeCalledPut = time.Now()
			p.mu.Unlock()

			fallbackCount++
		}
	}

	if fallbackCount > 0 {
		log.Printf("[REFILL] Fast path overflowed during refill — %d object(s) added to fallback pool", fallbackCount)
	}
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
			log.Println("[SHRINK] STATS (before shrink)")
			p.PrintPoolStats()
			log.Println("[SHRINK] Shrink conditions met — executing shrink.")
			p.ShrinkExecution()

			idleCount, underutilCount = 0, 0
			idleOK, utilOK = false, false
		}

		p.mu.Unlock()
		p.updateAvailableObjs() // safe
	}
}

// 1. grow is called when the pool is empty.
// 2. grow is called in one place, if you call in another one, you need to check if p.isGrowthBlocked is true before calling.
func (p *pool[T]) grow(now time.Time) bool {
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

	hardLimitReached := newCapacity > uint64(p.config.hardLimit)
	poolSizeReached := p.stats.currentCapacity.Load() >= uint64(p.config.hardLimit)

	if hardLimitReached && poolSizeReached {
		log.Printf("[GROW] New capacity (%d) exceeds hard limit (%d)", newCapacity, p.config.hardLimit)

		p.isGrowthBlocked = true
		return false
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

	p.stats.l3MissCount.Add(1)
	p.stats.totalGrowthEvents.Add(1)

	log.Printf("[GROW] Final state | New capacity: %d | Objects added: %d | Pool size after grow: %d",
		newCapacity, newObjsQty, len(p.pool))

	return true
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
	p.minCapacity = defaultMinCapacity
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
	p.stats.totalGets.Add(1)
	p.updateAvailableObjs() // safe

	for {
		peak := p.stats.peakInUse.Load()
		if currentInUse <= peak {
			break
		}

		if p.stats.peakInUse.CompareAndSwap(peak, currentInUse) {
			break
		}
	}

	// For it to count as a consecutive shrink the get method
	// needs to not be called in between shrinks.
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
	totalGets := p.stats.totalGets.Load()
	objectsInUse := p.stats.objectsInUse.Load()
	availableObjects := p.stats.availableObjects.Load()

	currentCapacity := p.stats.currentCapacity.Load()

	initialCapacity := p.stats.initialCapacity
	totalCreated := currentCapacity - uint64(initialCapacity)

	p.stats.mu.Lock()
	if totalCreated > 0 {
		p.stats.reqPerObj = float64(totalGets) / float64(totalCreated)
	}

	// WARNING -  at the time calculated, it may be low, be selective.
	totalObjects := objectsInUse + availableObjects
	if totalObjects > 0 {
		p.stats.utilization = (float64(objectsInUse) / float64(totalObjects)) * 100
	}
	p.stats.mu.Unlock()
}

func (p *pool[T]) performShrink(newCapacity int) {
	currentCap := int(p.stats.currentCapacity.Load())
	inUse := int(p.stats.objectsInUse.Load())
	minCap := p.config.shrink.minCapacity

	// NOTE: If all available objects are currently in the fast path channel (cacheL1),
	// then p.pool may be empty — but this does NOT mean all objects are in use.
	// We only skip shrinking if there are truly no available objects to preserve.
	availableInPool := len(p.pool)
	availableInCacheL1 := len(p.cacheL1)
	totalAvailable := availableInPool + availableInCacheL1

	if totalAvailable == 0 {
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

	availableObjsToCopy := newCapacity - inUse
	if availableObjsToCopy <= 0 {
		log.Printf("[SHRINK] Skipped — no room for available objects after shrink (in-use: %d, requested: %d)", inUse, newCapacity)
		return
	}

	copyCount := min(availableObjsToCopy, len(p.pool))
	newPool := make([]T, copyCount, newCapacity)
	copy(newPool, p.pool[:copyCount])
	p.pool = newPool

	log.Printf("[SHRINK] Shrinking pool → From: %d → To: %d | Preserved: %d | In-use: %d", currentCap, newCapacity, copyCount, inUse)

	// copy count is the objects being copied from the existing pool,
	// so they become the available objects
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
		refillPercent:      defaultRefillPercent,
	}
}

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

func (p *pool[T]) PrintPoolStats() {
	fmt.Println("========== Pool Stats ==========")
	fmt.Printf("Objects In Use       : %d\n", p.stats.objectsInUse.Load())
	fmt.Printf("Available Objects    : %d\n", p.stats.availableObjects.Load())
	fmt.Printf("Current Capacity     : %d\n", p.stats.currentCapacity.Load())
	fmt.Printf("Peak In Use          : %d\n", p.stats.peakInUse.Load())
	fmt.Printf("Total Gets           : %d\n", p.stats.totalGets.Load())
	fmt.Printf("Total Puts           : %d\n", p.stats.totalPuts.Load())
	fmt.Printf("Total Growth Events  : %d\n", p.stats.totalGrowthEvents.Load())
	fmt.Printf("Total Shrink Events  : %d\n", p.stats.totalShrinkEvents.Load())
	fmt.Printf("Consecutive Shrinks  : %d\n", p.stats.consecutiveShrinks.Load())
	fmt.Printf("Blocked Gets         : %d\n", p.stats.blockedGets.Load())

	fmt.Println()
	fmt.Println("---------- Fast Get Stats ----------")
	fmt.Printf("Fast Path (L1) Hits  : %d\n", p.stats.l1HitCount.Load())
	fmt.Printf("Slow Path (L2) Hits  : %d\n", p.stats.l2HitCount.Load())
	fmt.Printf("Allocator Misses (L3): %d\n", p.stats.l3MissCount.Load())
	fmt.Println("---------------------------------------")
	fmt.Println()
	fastReturnHit := p.stats.FastReturnHit.Load()
	fastReturnMiss := p.stats.FastReturnMiss.Load()
	totalReturns := fastReturnHit + fastReturnMiss

	var l2SpillRate float64
	if totalReturns > 0 {
		l2SpillRate = float64(fastReturnMiss) / float64(totalReturns)
	}

	fmt.Println("---------- Fast Return Stats ----------")
	fmt.Printf("Fast Return Hit   : %d\n", fastReturnHit)
	fmt.Printf("Fast Return Miss  : %d\n", fastReturnMiss)
	fmt.Printf("L2 Spill Rate     : %.2f%%\n", l2SpillRate*100)
	fmt.Println("---------------------------------------")
	fmt.Println()

	p.stats.mu.RLock()
	fmt.Println("---------- Usage Stats ----------")
	fmt.Println()
	fmt.Printf("Request Per Object   : %.2f\n", p.stats.reqPerObj)
	fmt.Printf("Utilization %%       : %.2f%%\n", p.stats.utilization)
	fmt.Println("---------------------------------------")
	p.stats.mu.RUnlock()

	fmt.Println("---------- Time Stats ----------")
	fmt.Printf("Last Get Time        : %s\n", p.stats.lastTimeCalledGet.Format(time.RFC3339))
	fmt.Printf("Last Put Time        : %s\n", p.stats.lastTimeCalledPut.Format(time.RFC3339))
	fmt.Printf("Last Shrink Time     : %s\n", p.stats.lastShrinkTime.Format(time.RFC3339))
	fmt.Printf("Last Grow Time       : %s\n", p.stats.lastGrowTime.Format(time.RFC3339))
	fmt.Println("=================================")
}

func (p *pool[T]) updateAvailableObjs() {
	p.mu.RLock()
	defer p.mu.RUnlock()
	fmt.Printf("pool length: %d, cacheLen: %d\n", len(p.pool), len(p.cacheL1))
	p.stats.availableObjects.Store(uint64(len(p.pool) + len(p.cacheL1)))
	fmt.Printf("availableObjects: %d\n", p.stats.availableObjects.Load())
}

func (p *pool[T]) reduceObjsInUse() {
	for {
		old := p.stats.objectsInUse.Load()
		if old == 0 {
			break
		}

		if p.stats.objectsInUse.CompareAndSwap(old, old-1) {
			break
		}
	}
}

func (p *pool[T]) reduceL1Hit() {
	for {
		old := p.stats.l1HitCount.Load()
		if old == 0 {
			break
		}

		if p.stats.l1HitCount.CompareAndSwap(old, old-1) {
			break
		}
	}
}

func (p *pool[T]) reduceBlockedGets() {
	for {
		old := p.stats.blockedGets.Load()
		if old == 0 {
			break
		}

		if p.stats.blockedGets.CompareAndSwap(old, old-1) {
			break
		}
	}
}
