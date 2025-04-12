package pool

import (
	"fmt"
	"log"
	"time"
)

func (p *pool[T]) updateAvailableObjs() {
	p.mu.RLock()
	p.stats.availableObjects.Store(uint64(len(p.pool) + len(p.cacheL1)))
	p.mu.RUnlock()
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

func (p *pool[T]) calculateNewPoolCapacity(currentCap, threshold, fixedStep uint64, cfg *growthParameters) uint64 {
	if currentCap < threshold {
		growth := maxUint64(uint64(float64(currentCap)*cfg.growthPercent), 1)
		newCap := currentCap + growth
		log.Printf("[GROW] Strategy: exponential | Threshold: %d | Current: %d | Growth: %d | New capacity: %d",
			threshold, currentCap, growth, newCap)
		return newCap
	}
	newCap := currentCap + fixedStep
	log.Printf("[GROW] Strategy: fixed-step | Threshold: %d | Current: %d | Step: %d | New capacity: %d",
		threshold, currentCap, fixedStep, newCap)
	return newCap
}

func (p *pool[T]) growthWouldExceedHardLimit(newCapacity uint64) bool {
	return newCapacity > uint64(p.config.hardLimit) && p.stats.currentCapacity.Load() >= uint64(p.config.hardLimit)
}

func (p *pool[T]) needsToShrinkToHardLimit(currentCap, newCapacity uint64) bool {
	return currentCap < uint64(p.config.hardLimit) && newCapacity > uint64(p.config.hardLimit)
}

func (p *pool[T]) resizePool(currentCap, newCap uint64) {
	toAdd := newCap - currentCap
	for range toAdd {
		p.pool = append(p.pool, p.allocator())
	}
	p.stats.currentCapacity.Store(newCap)
}

func (p *pool[T]) tryL1ResizeIfTriggered() {
	trigger := uint64(p.config.fastPath.growthEventsTrigger)
	if !p.config.fastPath.enableChannelGrowth {
		return
	}

	sinceLastResize := p.stats.totalGrowthEvents.Load() - p.stats.lastResizeAtGrowthNum.Load()
	if sinceLastResize < trigger {
		return
	}

	cfg := p.config.fastPath.growth
	currentCap := p.stats.currentL1Capacity.Load()
	threshold := uint64(float64(currentCap) * cfg.exponentialThresholdFactor)
	step := uint64(float64(currentCap) * cfg.fixedGrowthFactor)

	var newCap uint64
	if currentCap < threshold {
		newCap = currentCap + maxUint64(uint64(float64(currentCap)*cfg.growthPercent), 1)
	} else {
		newCap = currentCap + step
	}

	newL1 := make(chan T, newCap)
	for {
		select {
		case obj := <-p.cacheL1:
			newL1 <- obj
		default:
			goto done
		}
	}
done:

	p.cacheL1 = newL1
	p.stats.currentL1Capacity.Store(newCap)
	p.stats.lastResizeAtGrowthNum.Store(p.stats.totalGrowthEvents.Load())
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
	fmt.Println("---------- Fast Path Resize Stats ----------")
	fmt.Printf("Last Resize At Growth Num: %d\n", p.stats.lastResizeAtGrowthNum.Load())
	fmt.Printf("Current L1 Capacity     : %d\n", p.stats.currentL1Capacity.Load())
	fmt.Println("---------------------------------------")
	fmt.Println()

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

func getShrinkDefaultsMap() map[AggressivenessLevel]*shrinkDefaults {
	return defaultShrinkMap
}

func maxUint64(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

func (p *pool[T]) setPoolAndBuffer(obj T, fastPathRemaining int) int {
	if fastPathRemaining > 0 {
		select {
		case p.cacheL1 <- obj:
			fastPathRemaining--
			return fastPathRemaining
		default:
			// avoids blocking
		}
	}

	p.pool = append(p.pool, obj)
	return fastPathRemaining
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

func (p *pool[T]) updateUsageStats() {
	currentInUse := p.stats.objectsInUse.Add(1)
	p.stats.totalGets.Add(1)
	p.updateAvailableObjs()

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

func (p *pool[T]) refill(n int) bool {
	batch := p.slowPathGetObjBatch(n)
	if batch == nil {
		return false
	}

	p.updateAvailableObjs()

	p.fastRefill(batch)
	return true
}

// even if the growth is blocked, we still need to refill the L1 buffer.
func (p *pool[T]) slowPathGetObjBatch(requiredPrefill int) []T {
	p.mu.Lock()
	defer p.mu.Unlock()

	noObjsAvailable := len(p.pool) == 0
	if noObjsAvailable {
		if p.isGrowthBlocked {
			log.Printf("[POOL] Growth is blocked — returning nil")
			return nil
		}

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

	if start > len(p.pool) {
		start = len(p.pool) - 1
	}

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

func (p *pool[T]) performShrink(newCapacity, inUse int, currentCap uint64) {
	var zero T

	availableToKeep := newCapacity - inUse
	if availableToKeep <= 0 {
		log.Printf("[SHRINK] Skipped — no room for available objects after shrink (in-use: %d, requested: %d)", inUse, newCapacity)
		return
	}

	copyCount := min(availableToKeep, len(p.pool))

	for i := copyCount; i < len(p.pool); i++ {
		p.pool[i] = zero
	}

	p.pool = p.pool[:copyCount]

	log.Printf("[SHRINK] Shrinking pool → From: %d → To: %d | Preserved: %d | In-use: %d", currentCap, newCapacity, copyCount, inUse)

	p.stats.currentCapacity.Store(uint64(newCapacity))
	p.stats.totalShrinkEvents.Add(1)
	p.stats.lastShrinkTime = time.Now()
	p.stats.consecutiveShrinks.Add(1)
}

func (p *pool[T]) ShrinkExecution() {
	p.logShrinkHeader()

	currentCap := p.stats.currentCapacity.Load()
	inUse := int(p.stats.objectsInUse.Load())
	newCapacity := int(float64(currentCap) * (1.0 - p.config.shrink.shrinkPercent))

	if !p.shouldShrinkMainPool(currentCap, newCapacity, inUse) {
		return
	}

	newCapacity = p.adjustMainShrinkTarget(newCapacity, inUse)

	p.performShrink(newCapacity, inUse, currentCap)
	log.Printf("[SHRINK] Shrink complete — Final capacity: %d", newCapacity)
	log.Println("[SHRINK] ----------------------------------------")

	// Fast Path (L1)
	if !p.config.fastPath.enableChannelGrowth || !p.shouldShrinkFastPath() {
		return
	}

	currentCap = p.stats.currentL1Capacity.Load()
	newCapacity = p.adjustFastPathShrinkTarget(currentCap)

	p.logFastPathShrink(currentCap, newCapacity, inUse)
	p.shrinkFastPath(newCapacity, inUse)

	log.Printf("[SHRINK | FAST PATH] Shrink complete — Final capacity: %d", newCapacity)
	log.Println("[SHRINK | FAST PATH] ----------------------------------------")
}

func (p *pool[T]) logShrinkHeader() {
	log.Println("[SHRINK] ----------------------------------------")
	log.Println("[SHRINK] Starting shrink execution")
}

func (p *pool[T]) shouldShrinkMainPool(currentCap uint64, newCap int, inUse int) bool {
	minCap := p.config.shrink.minCapacity

	log.Printf("[SHRINK] Current capacity       : %d", currentCap)
	log.Printf("[SHRINK] Requested new capacity : %d", newCap)
	log.Printf("[SHRINK] Minimum allowed        : %d", minCap)
	log.Printf("[SHRINK] Currently in use       : %d", inUse)
	log.Printf("[SHRINK] Pool length            : %d", len(p.pool))

	switch {
	case newCap == 0:
		log.Println("[SHRINK] Skipped — new capacity is zero (invalid)")
		return false
	case currentCap <= uint64(minCap):
		log.Printf("[SHRINK] Skipped — current capacity (%d) is at or below MinCapacity (%d)", currentCap, minCap)
		return false
	case newCap >= int(currentCap):
		log.Printf("[SHRINK] Skipped — new capacity (%d) is not smaller than current (%d)", newCap, currentCap)
		return false
	}

	totalAvailable := len(p.pool) + len(p.cacheL1)
	if totalAvailable == 0 {
		log.Printf("[SHRINK] Skipped — all %d objects are currently in use, no shrink possible", inUse)
		return false
	}

	return true
}

func (p *pool[T]) adjustMainShrinkTarget(newCap, inUse int) int {
	minCap := p.config.shrink.minCapacity

	if newCap < minCap {
		log.Printf("[SHRINK] Adjusting to min capacity: %d", minCap)
		newCap = minCap
	}
	if newCap < inUse {
		log.Printf("[SHRINK] Adjusting to match in-use objects: %d", inUse)
		newCap = inUse
	}

	if newCap < p.config.hardLimit {
		log.Printf("[SHRINK] Allowing growth, capacity is lower than hard limit: %d", p.config.hardLimit)
		p.isGrowthBlocked = false
	}
	return newCap
}

func (p *pool[T]) shouldShrinkFastPath() bool {
	sinceLast := p.stats.totalShrinkEvents.Load() - p.stats.lastResizeAtShrinkNum.Load()
	trigger := uint64(p.config.fastPath.shrinkEventsTrigger)

	return sinceLast >= trigger
}

func (p *pool[T]) adjustFastPathShrinkTarget(currentCap uint64) int {
	cfg := p.config.fastPath.shrink
	newCap := int(float64(currentCap) * (1.0 - cfg.shrinkPercent))

	if newCap < cfg.minCapacity {
		log.Printf("[SHRINK | FAST PATH] Adjusting to min capacity: %d", cfg.minCapacity)
		newCap = cfg.minCapacity
	}
	return newCap
}

func (p *pool[T]) shrinkFastPath(newCapacity, inUse int) {
	availableObjsToCopy := newCapacity - inUse
	if availableObjsToCopy <= 0 {
		log.Printf("[SHRINK] Skipped — no room for available objects after shrink (in-use: %d, requested: %d)", inUse, newCapacity)
		return
	}

	copyCount := min(availableObjsToCopy, len(p.cacheL1))
	newL1 := make(chan T, newCapacity)

	for range copyCount {
		select {
		case obj := <-p.cacheL1:
			newL1 <- obj
		default:
			fmt.Println("[SHRINK] - cacheL1 is empty, or newL1 is full")
		}
	}

	p.cacheL1 = newL1
	p.stats.lastResizeAtShrinkNum.Store(p.stats.totalShrinkEvents.Load())
}

func (p *pool[T]) logFastPathShrink(currentCap uint64, newCap int, inUse int) {
	log.Println("[SHRINK | FAST PATH] ----------------------------------------")
	log.Printf("[SHRINK | FAST PATH] Starting shrink execution")
	log.Printf("[SHRINK | FAST PATH] Current L1 capacity     : %d", currentCap)
	log.Printf("[SHRINK | FAST PATH] Requested new capacity  : %d", newCap)
	log.Printf("[SHRINK | FAST PATH] Minimum allowed         : %d", p.config.fastPath.shrink.minCapacity)
	log.Printf("[SHRINK | FAST PATH] Currently in use        : %d", inUse)
	log.Printf("[SHRINK | FAST PATH] Channel length (cached) : %d", len(p.cacheL1))
}

// Set default fields
func InitDefaultFields() {
	defaultFastPath.shrink.minCapacity = defaultL1MinCapacity
	defaultShrinkParameters.ApplyDefaults(getShrinkDefaultsMap())
}
