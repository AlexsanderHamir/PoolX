package pool

import (
	"fmt"
	"log"
	"reflect"
	"sync"
	"time"
)

func (p *Pool[T]) updateAvailableObjs() {
	p.mu.RLock()
	p.stats.availableObjects.Store(uint64(p.pool.Length() + len(p.cacheL1)))
	p.mu.RUnlock()
}

// better name this function
func (p *Pool[T]) releaseObj(obj T) {
	if reflect.ValueOf(obj).IsNil() {
		log.Println("[RELEASEOBJ] Object is nil")
		return
	}

	p.cleaner(obj)
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

func (p *Pool[T]) reduceL1Hit() {
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

func (p *Pool[T]) calculateNewPoolCapacity(currentCap, threshold, fixedStep uint64, cfg *growthParameters) uint64 {
	if currentCap < threshold {
		growth := maxUint64(uint64(float64(currentCap)*cfg.growthPercent), 1)
		newCap := currentCap + growth
		if p.config.verbose {
			log.Printf("[GROW] Strategy: exponential | Threshold: %d | Current: %d | Growth: %d | New capacity: %d", threshold, currentCap, growth, newCap)
		}
		return newCap
	}
	newCap := currentCap + fixedStep
	if p.config.verbose {
		log.Printf("[GROW] Strategy: fixed-step | Threshold: %d | Current: %d | Step: %d | New capacity: %d",
			threshold, currentCap, fixedStep, newCap)
	}
	return newCap
}

func (p *Pool[T]) needsToShrinkToHardLimit(newCapacity uint64) bool {
	return newCapacity > uint64(p.config.hardLimit)
}

func (p *Pool[T]) tryL1ResizeIfTriggered() {
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

func (p *Pool[T]) PrintPoolStats() {
	p.updateAvailableObjs()

	fmt.Println("========== Pool Stats ==========")
	fmt.Printf("Objects In Use					 : %d\n", p.stats.objectsInUse.Load())
	fmt.Printf("Total Available Objects			 : %d\n", p.stats.availableObjects.Load())
	fmt.Printf("Current Ring Buffer Capacity	 : %d\n", p.stats.currentCapacity.Load())
	fmt.Printf("Ring Buffer Length   			 : %d\n", p.pool.Length())
	fmt.Printf("Peak In Use          			 : %d\n", p.stats.peakInUse.Load())
	fmt.Printf("Total Gets           			 : %d\n", p.stats.totalGets.Load())
	fmt.Printf("Total Growth Events  			 : %d\n", p.stats.totalGrowthEvents.Load())
	fmt.Printf("Total Shrink Events  			 : %d\n", p.stats.totalShrinkEvents.Load())
	fmt.Printf("Consecutive Shrinks  			 : %d\n", p.stats.consecutiveShrinks.Load())
	fmt.Printf("Blocked Gets         			 : %d\n", p.stats.blockedGets.Load())

	fmt.Println()
	fmt.Println("---------- Fast Path Resize Stats ----------")
	fmt.Printf("Last Resize At Growth Num: %d\n", p.stats.lastResizeAtGrowthNum.Load())
	fmt.Printf("Current L1 Capacity      : %d\n", p.stats.currentL1Capacity.Load())
	fmt.Printf("L1 Length                : %d\n", len(p.cacheL1))
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

	p.updateDerivedStats()
	p.stats.mu.RLock()
	fmt.Println("---------- Usage Stats ----------")
	fmt.Println()
	fmt.Printf("Request Per Object   : %.2f\n", p.stats.reqPerObj)
	fmt.Printf("Utilization %%       : %.2f%%\n", p.stats.utilization)
	fmt.Println("---------------------------------------")
	p.stats.mu.RUnlock()

	fmt.Println("---------- Time Stats ----------")
	fmt.Printf("Last Get Time        : %s\n", p.stats.lastTimeCalledGet.Format(time.RFC3339))
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

// this is blocking
func (p *Pool[T]) setPoolAndBuffer(obj T, fastPathRemaining int) int {
	if fastPathRemaining > 0 {
		select {
		case p.cacheL1 <- obj:
			fastPathRemaining--
			return fastPathRemaining
		default:
		}
	}

	if err := p.pool.Write(obj); err != nil {
		if p.config.verbose {
			log.Printf("[SETPOOL] Error writing to ring buffer: %v", err)
		}
	}

	return fastPathRemaining
}

func (p *Pool[T]) IdleCheck(idles *int, shrinkPermissionIdleness *bool) {
	idleDuration := time.Since(p.stats.lastTimeCalledGet)
	threshold := p.config.shrink.idleThreshold
	required := p.config.shrink.minIdleBeforeShrink

	if idleDuration >= threshold {
		*idles += 1
		if *idles >= required {
			*shrinkPermissionIdleness = true
		}
		if p.config.verbose {
			log.Printf("[SHRINK] IdleCheck passed — idleDuration: %v | idles: %d/%d", idleDuration, *idles, required)
		}
	} else {
		if *idles > 0 {
			*idles -= 1
		}
		if p.config.verbose {
			log.Printf("[SHRINK] IdleCheck reset — recent activity detected | idles: %d/%d", *idles, required)
		}
	}
}

// calculateUtilization calculates the current utilization percentage of the pool
func (p *Pool[T]) calculateUtilization() float64 {
	inUse := p.stats.objectsInUse.Load()
	available := p.stats.availableObjects.Load()
	total := inUse + available

	if total == 0 {
		return 0
	}
	return (float64(inUse) / float64(total)) * 100
}

// logUtilizationStats logs the current utilization statistics if verbose mode is enabled
func (p *Pool[T]) logUtilizationStats(utilization, minUtil float64, requiredRounds int) {
	if !p.config.verbose {
		return
	}
	fmt.Printf("[DEBUG] UtilizationCheck - current utilization: %.2f%%, minUtil: %.2f%%, requiredRounds: %d\n",
		utilization, minUtil, requiredRounds)
}

// handleUnderutilization handles the case when utilization is below the minimum threshold
func (p *Pool[T]) handleUnderutilization(utilization, minUtil float64, underutilizationRounds *int, requiredRounds int, shrinkPermissionUtilization *bool) {
	if p.config.verbose {
		fmt.Println("[DEBUG] UtilizationCheck - utilization below threshold")
	}
	*underutilizationRounds++
	if p.config.verbose {
		log.Printf("[SHRINK] UtilizationCheck — utilization: %.2f%% (threshold: %.2f%%) | round: %d/%d",
			utilization, minUtil, *underutilizationRounds, requiredRounds)
	}

	if *underutilizationRounds >= requiredRounds {
		if p.config.verbose {
			fmt.Println("[DEBUG] UtilizationCheck - reached required rounds, setting shrink permission")
		}
		*shrinkPermissionUtilization = true
		if p.config.verbose {
			log.Println("[SHRINK] UtilizationCheck — underutilization stable, shrink allowed")
		}
	}
}

// handleHealthyUtilization handles the case when utilization is above the minimum threshold
func (p *Pool[T]) handleHealthyUtilization(utilization, minUtil float64, underutilizationRounds *int, requiredRounds int) {
	if p.config.verbose {
		fmt.Println("[DEBUG] UtilizationCheck - utilization above threshold")
	}
	if *underutilizationRounds > 0 {
		if p.config.verbose {
			fmt.Println("[DEBUG] UtilizationCheck - reducing underutilization rounds")
		}
		*underutilizationRounds--
		if p.config.verbose {
			log.Printf("[SHRINK] UtilizationCheck — usage recovered, reducing rounds: %d/%d",
				*underutilizationRounds, requiredRounds)
		}
	} else {
		if p.config.verbose {
			fmt.Println("[DEBUG] UtilizationCheck - healthy utilization")
			log.Printf("[SHRINK] UtilizationCheck — usage healthy: %.2f%% > %.2f%%", utilization, minUtil)
		}
	}
}

func (p *Pool[T]) UtilizationCheck(underutilizationRounds *int, shrinkPermissionUtilization *bool) {
	p.mu.Unlock()
	p.updateAvailableObjs()
	p.mu.Lock()

	utilization := p.calculateUtilization()
	minUtil := p.config.shrink.minUtilizationBeforeShrink * 100
	requiredRounds := p.config.shrink.stableUnderutilizationRounds

	p.logUtilizationStats(utilization, minUtil, requiredRounds)

	if utilization <= minUtil {
		p.handleUnderutilization(utilization, minUtil, underutilizationRounds, requiredRounds, shrinkPermissionUtilization)
	} else {
		p.handleHealthyUtilization(utilization, minUtil, underutilizationRounds, requiredRounds)
	}

	if p.config.verbose {
		fmt.Println("[DEBUG] Ending UtilizationCheck")
	}
}

func (p *Pool[T]) updateUsageStats() {
	currentInUse := p.stats.objectsInUse.Add(1)
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

func (p *Pool[T]) checkGrowthNeeded(fillTarget int) (bool, RefillResult) {
	var result RefillResult
	noObjsAvailable := p.pool.Length() == 0
	biggerRequestThanAvailable := fillTarget > p.pool.Length()

	if noObjsAvailable || biggerRequestThanAvailable {
		if p.isGrowthBlocked {
			result.Reason = GrowthBlocked
			return false, result
		}

		now := time.Now()
		if !p.grow(now) {
			result.Reason = GrowthFailed
			return false, result
		}

		result.GrowthNeeded = true
	}
	return true, result
}

func (p *Pool[T]) getItemsToMove(fillTarget int) ([]T, []T, string, error) {
	currentObjsAvailable := p.pool.Length()
	toMove := min(fillTarget, currentObjsAvailable)

	part1, part2, err := p.pool.GetNView(toMove)
	if err != nil && err != ErrIsEmpty {
		if p.config.verbose {
			log.Printf("[REFILL] Error getting items from ring buffer: %v", err)
		}
		return nil, nil, fmt.Sprintf("%s: %v", RingBufferError, err), err
	}

	// TODO: improve this hacky check
	if part1 == nil && part2 == nil && err == nil || len(part1) == 0 && len(part2) == 0 {
		if p.config.verbose {
			log.Println("[REFILL] No items to move. (fillTarget == 0 || currentObjsAvailable == 0)")
		}
		return nil, nil, NoItemsToMove, nil
	}

	return part1, part2, "", nil
}

func (p *Pool[T]) moveItemsToL1(items []T, result *RefillResult) {
	for _, item := range items {
		select {
		case p.cacheL1 <- item:
			result.ItemsMoved++
			if p.config.verbose {
				log.Printf("[REFILL] Moved object to L1 | Remaining: %d", p.pool.Length())
			}
		default:
			result.ItemsFailed++
			if err := p.pool.Write(item); err != nil {
				if p.config.verbose {
					log.Printf("[REFILL] Error putting object back in ring buffer: %v", err)
				}
			}
		}
	}
}

func (p *Pool[T]) refill(fillTarget int) RefillResult {
	var result RefillResult

	shouldContinue, growthResult := p.checkGrowthNeeded(fillTarget)
	if !shouldContinue {
		return growthResult
	}
	result = growthResult

	part1, part2, reason, err := p.getItemsToMove(fillTarget)
	if err != nil || reason != "" {
		result.Reason = reason
		return result
	}

	p.moveItemsToL1(part1, &result)
	p.moveItemsToL1(part2, &result)

	result.Success = true
	result.Reason = RefillSucceeded
	return result
}

func (p *Pool[T]) ShrinkExecution() {
	p.logShrinkHeader()

	currentCap := p.stats.currentCapacity.Load()
	inUse := int(p.stats.objectsInUse.Load())
	newCapacity := int(float64(currentCap) * (1.0 - p.config.shrink.shrinkPercent))

	if !p.shouldShrinkMainPool(currentCap, newCapacity, inUse) {
		return
	}

	newCapacity = p.adjustMainShrinkTarget(newCapacity, inUse)
	p.performShrink(newCapacity, inUse, currentCap)
	if p.config.verbose {
		log.Printf("[SHRINK] Shrink complete — Final capacity: %d", newCapacity)
		log.Println("[SHRINK] ----------------------------------------")
	}

	// Fast Path (L1)
	if !p.config.fastPath.enableChannelGrowth || !p.shouldShrinkFastPath() {
		return
	}

	currentCap = p.stats.currentL1Capacity.Load()
	newCapacity = p.adjustFastPathShrinkTarget(currentCap)

	p.logFastPathShrink(currentCap, newCapacity, inUse)
	p.shrinkFastPath(newCapacity, inUse)

	if p.config.verbose {
		log.Printf("[SHRINK | FAST PATH] Shrink complete — Final capacity: %d", newCapacity)
		log.Println("[SHRINK | FAST PATH] ----------------------------------------")
	}

	if p.config.verbose {
		log.Printf("[SHRINK] Final state | New capacity: %d | Ring buffer length: %d", newCapacity, p.pool.Length())
	}
}

func (p *Pool[T]) performShrink(newCapacity, inUse int, currentCap uint64) {
	availableToKeep := newCapacity - inUse
	if availableToKeep < 0 {
		if p.config.verbose {
			log.Printf("[SHRINK] Skipped — no room for available objects after shrink (in-use: %d, requested: %d)", inUse, newCapacity)
		}
		return
	}

	newRingBuffer := NewRingBuffer[T](int(newCapacity))
	newRingBuffer.CopyConfig(p.pool)

	itemsToKeep := min(availableToKeep, p.pool.Length())
	if itemsToKeep > 0 {
		part1, part2, err := p.pool.GetNView(itemsToKeep)
		if err != nil && err != ErrIsEmpty {
			if p.config.verbose {
				log.Printf("[SHRINK] Error getting items from old ring buffer: %v", err)
			}
			return
		}

		if _, err := newRingBuffer.WriteMany(part1); err != nil {
			if p.config.verbose {
				log.Printf("[SHRINK] Error writing items to new ring buffer: %v", err)
			}
			return
		}

		if _, err := newRingBuffer.WriteMany(part2); err != nil {
			if p.config.verbose {
				log.Printf("[SHRINK] Error writing items to new ring buffer: %v", err)
			}
			return
		}
	}

	p.pool = newRingBuffer

	p.stats.currentCapacity.Store(uint64(newCapacity))
	p.stats.totalShrinkEvents.Add(1)
	p.stats.lastShrinkTime = time.Now()
	p.stats.consecutiveShrinks.Add(1)

	if p.config.verbose {
		log.Printf("[SHRINK] Shrinking pool → From: %d → To: %d | Preserved: %d | In-use: %d",
			currentCap, newCapacity, itemsToKeep, inUse)
	}
}

func (p *Pool[T]) logShrinkHeader() {
	if p.config.verbose {
		log.Println("[SHRINK] ----------------------------------------")
		log.Println("[SHRINK] Starting shrink execution")
	}
}

func (p *Pool[T]) shouldShrinkMainPool(currentCap uint64, newCap int, inUse int) bool {
	minCap := p.config.shrink.minCapacity

	if p.config.verbose {
		log.Printf("[SHRINK] Current capacity       : %d", currentCap)
		log.Printf("[SHRINK] Requested new capacity : %d", newCap)
		log.Printf("[SHRINK] Minimum allowed        : %d", minCap)
		log.Printf("[SHRINK] Currently in use       : %d", inUse)
		log.Printf("[SHRINK] Ring buffer length     : %d", p.pool.Length())
	}

	switch {
	case newCap == 0:
		if p.config.verbose {
			log.Println("[SHRINK] Skipped — new capacity is zero (invalid)")
		}
		return false
	case currentCap == uint64(minCap):
		if p.config.verbose {
			log.Printf("[SHRINK] Skipped — current capacity (%d) is at or below MinCapacity (%d)", currentCap, minCap)
		}
		return false
	case newCap >= int(currentCap):
		if p.config.verbose {
			log.Printf("[SHRINK] Skipped — new capacity (%d) is not smaller than current (%d)", newCap, currentCap)
		}
		return false
	}

	l1Available := len(p.cacheL1)
	totalAvailable := p.pool.Length() + l1Available
	if totalAvailable == 0 {
		if p.config.verbose {
			log.Printf("[SHRINK] Skipped — all %d objects are currently in use, no shrink possible", inUse)
		}
		return false
	}

	return true
}

func (p *Pool[T]) adjustMainShrinkTarget(newCap, inUse int) int {
	minCap := p.config.shrink.minCapacity

	if newCap < minCap {
		if p.config.verbose {
			log.Printf("[SHRINK] Adjusting to min capacity: %d", minCap)
		}
		newCap = minCap
	}

	if newCap <= inUse {
		if p.config.verbose {
			log.Printf("[SHRINK] Adjusting to match in-use objects: %d", inUse)
		}
		newCap = inUse
	}

	if newCap < p.config.hardLimit && p.isGrowthBlocked {
		if p.config.verbose {
			log.Printf("[SHRINK] Allowing growth, capacity is lower than hard limit: %d", p.config.hardLimit)
		}
		p.isGrowthBlocked = false
	}

	return newCap
}

func (p *Pool[T]) shouldShrinkFastPath() bool {
	sinceLast := p.stats.totalShrinkEvents.Load() - p.stats.lastResizeAtShrinkNum.Load()
	trigger := uint64(p.config.fastPath.shrinkEventsTrigger)

	return sinceLast >= trigger
}

func (p *Pool[T]) adjustFastPathShrinkTarget(currentCap uint64) int {
	cfg := p.config.fastPath.shrink
	newCap := int(float64(currentCap) * (1.0 - cfg.shrinkPercent))

	if newCap < cfg.minCapacity {
		if p.config.verbose {
			log.Printf("[SHRINK | FAST PATH] Adjusting to min capacity: %d", cfg.minCapacity)
		}
		newCap = cfg.minCapacity
	}
	return newCap
}

func (p *Pool[T]) shrinkFastPath(newCapacity, inUse int) {
	availableObjsToCopy := newCapacity - inUse
	if availableObjsToCopy <= 0 {
		if p.config.verbose {
			log.Printf("[SHRINK] Skipped — no room for available objects after shrink (in-use: %d, requested: %d)", inUse, newCapacity)
		}
		return
	}

	copyCount := min(availableObjsToCopy, len(p.cacheL1))
	newL1 := make(chan T, newCapacity)

	for range copyCount {
		select {
		case obj := <-p.cacheL1:
			newL1 <- obj
		default:
			if p.config.verbose {
				log.Println("[SHRINK] - cacheL1 is empty, or newL1 is full")
			}
		}
	}

	p.cacheL1 = newL1
	p.stats.lastResizeAtShrinkNum.Store(p.stats.totalShrinkEvents.Load())
	p.stats.currentL1Capacity.Store(uint64(newCapacity))
}

func (p *Pool[T]) logFastPathShrink(currentCap uint64, newCap int, inUse int) {
	if p.config.verbose {
		log.Println("[SHRINK | FAST PATH] ----------------------------------------")
		log.Printf("[SHRINK | FAST PATH] Starting shrink execution")
		log.Printf("[SHRINK | FAST PATH] Current L1 capacity     : %d", currentCap)
		log.Printf("[SHRINK | FAST PATH] Requested new capacity  : %d", newCap)
		log.Printf("[SHRINK | FAST PATH] Minimum allowed         : %d", p.config.fastPath.shrink.minCapacity)
		log.Printf("[SHRINK | FAST PATH] Currently in use        : %d", inUse)
		log.Printf("[SHRINK | FAST PATH] Channel length (cached) : %d", len(p.cacheL1))
	}
}

func (p *Pool[T]) updateDerivedStats() {
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

func createDefaultConfig() *poolConfig {
	pgb := &poolConfigBuilder{
		config: &poolConfig{
			initialCapacity:  defaultPoolCapacity,
			hardLimit:        defaultHardLimit,
			shrink:           defaultShrinkParameters,
			growth:           defaultGrowthParameters,
			fastPath:         defaultFastPath,
			ringBufferConfig: defaultRingBufferConfig,
		},
	}

	copiedShrink := *defaultShrinkParameters
	pgb.config.fastPath.shrink.ApplyDefaults(getShrinkDefaultsMap())
	pgb.config.fastPath.shrink.minCapacity = defaultL1MinCapacity
	pgb.config.fastPath.shrink = &copiedShrink

	pgb.config.shrink.ApplyDefaults(getShrinkDefaultsMap())

	return pgb.config
}

func initializePoolStats(config *poolConfig) *poolStats {
	stats := &poolStats{mu: sync.RWMutex{}}
	stats.initialCapacity += config.initialCapacity
	stats.currentCapacity.Store(uint64(config.initialCapacity))
	stats.availableObjects.Store(uint64(config.initialCapacity))
	stats.currentL1Capacity.Store(uint64(config.fastPath.initialSize))
	return stats
}

func validateAllocator[T any](allocator func() T) error {
	obj := allocator()
	if reflect.TypeOf(obj).Kind() != reflect.Ptr {
		return fmt.Errorf("allocator must return a pointer, got %T", obj)
	}
	return nil
}

func initializePoolObject[T any](config *poolConfig, allocator func() T, cleaner func(T), stats *poolStats, ringBuffer *RingBuffer[T], poolType reflect.Type) (*Pool[T], error) {
	poolObj := &Pool[T]{
		cacheL1:   make(chan T, config.fastPath.initialSize),
		allocator: allocator,
		cleaner:   cleaner,
		mu:        sync.RWMutex{},
		config:    config,
		stats:     stats,
		pool:      ringBuffer,
		poolType:  poolType,
	}

	poolObj.cond = sync.NewCond(&poolObj.mu)
	return poolObj, nil
}

func populateL1OrBuffer[T any](poolObj *Pool[T]) error {
	fillTarget := int(float64(poolObj.config.fastPath.initialSize) * poolObj.config.fastPath.fillAggressiveness)
	fastPathRemaining := fillTarget

	for range poolObj.config.initialCapacity {
		obj := poolObj.allocator()
		fastPathRemaining = poolObj.setPoolAndBuffer(obj, fastPathRemaining)
	}
	return nil
}

func (p *Pool[T]) handleShrinkBlocked() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isShrinkBlocked {
		if p.config.verbose {
			log.Println("[GET] Shrink is blocked — broadcasting to cond")
		}
		p.cond.Broadcast()
	}
}

func (p *Pool[T]) handleRefillFailure(refillReason string) (T, bool) {
	if p.config.verbose {
		log.Printf("[GET] Unable to refill — reason: %s", refillReason)
	}

	var zero T
	if refillReason == GrowthFailed || refillReason == RingBufferError {
		if p.config.verbose {
			log.Printf("[GET] Warning: unable to refill — reason: %s, returning nil", refillReason)
		}
		return zero, false
	}

	return zero, true
}

func (p *Pool[T]) tryGetFromL1() (T, bool) {
	select {
	case obj := <-p.cacheL1:
		if p.config.verbose {
			log.Println("[GET] L1 hit")
		}
		p.stats.l1HitCount.Add(1)
		p.updateUsageStats()
		return obj, true
	default:
		if p.config.verbose {
			log.Println("[GET] L1 miss — falling back to slow path")
		}
		var zero T
		return zero, false
	}
}

func (p *Pool[T]) tryFastPathPut(obj T) bool {
	select {
	case p.cacheL1 <- obj:
		p.stats.FastReturnHit.Add(1)
		p.logPut("Fast return hit")
		return true
	default:
		if p.config.verbose {
			p.logPut("L1 return miss — falling back to slow path")
		}
		return false
	}
}

func (p *Pool[T]) slowPathPut(obj T) {
	if err := p.pool.Write(obj); err != nil {
		panic(err)
	}

	p.stats.FastReturnMiss.Add(1)
	p.logPut("slow path put unblocked")
}

func (p *Pool[T]) logPut(message string) {
	if p.config.verbose {
		log.Printf("[PUT] %s", message)
	}
}

func (p *Pool[T]) calculateL1Usage() (int, int, float64) {
	currentCap := p.stats.currentL1Capacity.Load()

	p.mu.RLock()
	currentLength := len(p.cacheL1)
	p.mu.RUnlock()

	var currentPercent float64
	if currentCap > 0 {
		currentPercent = float64(currentLength) / float64(currentCap)
	}
	return currentLength, int(currentCap), currentPercent
}

func (p *Pool[T]) logL1Usage(currentLength, currentCap int, currentPercent float64) {
	if p.config.verbose {
		log.Printf("[REFILL] L1 usage: %d/%d (%.2f%%), refill threshold: %.2f%%", currentLength, currentCap, currentPercent*100, p.config.fastPath.refillPercent*100)
	}
}

func (p *Pool[T]) calculateFillTarget(currentCap int) int {
	if currentCap <= 0 {
		return 0
	}

	targetFill := int(float64(currentCap) * p.config.fastPath.fillAggressiveness)

	p.mu.RLock()
	currentLength := len(p.cacheL1)
	p.mu.RUnlock()

	// Calculate how many more items we need to reach target fill
	itemsNeeded := targetFill - currentLength

	// Ensure we don't return negative values
	if itemsNeeded < 0 {
		return 0
	}

	return itemsNeeded
}

func (p *Pool[T]) logRefillResult(result RefillResult) {
	if p.config.verbose {
		log.Printf("[REFILL] Refill succeeded: %s | Moved: %d | Failed: %d | Growth needed: %v | Growth blocked: %v",
			result.Reason, result.ItemsMoved, result.ItemsFailed, result.GrowthNeeded, result.GrowthBlocked)
	}
}

func (p *Pool[T]) handleMaxConsecutiveShrinks(params *shrinkParameters) bool {
	if p.stats.consecutiveShrinks.Load() == uint64(params.maxConsecutiveShrinks) {
		if p.config.verbose {
			log.Println("[SHRINK] Max consecutive shrinks reached — waiting for Get() call")
		}
		p.isShrinkBlocked = true
		p.cond.Wait()
		p.isShrinkBlocked = false
		return true
	}
	return false
}

func (p *Pool[T]) handleShrinkCooldown(params *shrinkParameters) bool {
	timeSinceLastShrink := time.Since(p.stats.lastShrinkTime)
	if timeSinceLastShrink < params.shrinkCooldown {
		if p.config.verbose {
			log.Printf("[SHRINK] Cooldown active: %v remaining", params.shrinkCooldown-timeSinceLastShrink)
		}
		return true
	}
	return false
}

func (p *Pool[T]) performShrinkChecks(params *shrinkParameters, idleCount, underutilCount *int, idleOK, utilOK *bool) {
	p.IdleCheck(idleCount, idleOK)
	if p.config.verbose {
		log.Printf("[SHRINK] IdleCheck — idles: %d / min: %d | allowed: %v", *idleCount, params.minIdleBeforeShrink, *idleOK)
	}

	p.UtilizationCheck(underutilCount, utilOK)
	if p.config.verbose {
		log.Printf("[SHRINK] UtilCheck — rounds: %d / required: %d | allowed: %v", *underutilCount, params.stableUnderutilizationRounds, *utilOK)
	}
}

func (p *Pool[T]) executeShrink(idleCount, underutilCount *int, idleOK, utilOK *bool) {
	if p.config.verbose {
		log.Println("[SHRINK] Shrink conditions met — executing shrink.")
	}

	p.ShrinkExecution()
	*idleCount, *underutilCount = 0, 0
	*idleOK, *utilOK = false, false
}

func (p *Pool[T]) createNewBuffer(newCapacity uint64) *RingBuffer[T] {
	newRingBuffer := NewRingBuffer[T](int(newCapacity))
	if p.pool == nil {
		return nil
	}
	newRingBuffer.CopyConfig(p.pool)
	return newRingBuffer
}

func (p *Pool[T]) getItemsFromOldBuffer() (part1, part2 []T, err error) {
	part1, part2, err = p.pool.getAll()
	if err != nil && err != ErrIsEmpty {
		if p.config.verbose {
			log.Printf("[GROW] Error getting items from old ring buffer: %v", err)
		}
		return nil, nil, err
	}
	return part1, part2, nil
}

func (p *Pool[T]) validateAndWriteItems(newRingBuffer *RingBuffer[T], part1, part2 []T, newCapacity uint64) error {
	if len(part1) > int(newCapacity) {
		if p.config.verbose {
			log.Printf("[GROW] Length mismatch | Expected: %d | Actual: %d", newCapacity, len(part1))
		}
		return ErrInvalidLength
	}

	if _, err := newRingBuffer.WriteMany(part1); err != nil {
		if p.config.verbose {
			log.Printf("[GROW] Error writing items to new ring buffer: %v", err)
		}
		return err
	}

	if _, err := newRingBuffer.WriteMany(part2); err != nil {
		if p.config.verbose {
			log.Printf("[GROW] Error writing items to new ring buffer: %v", err)
		}
		return err
	}

	return nil
}

func (p *Pool[T]) fillRemainingCapacity(newRingBuffer *RingBuffer[T], newCapacity uint64) error {
	currentCapacity := p.stats.currentCapacity.Load()
	toAdd := newCapacity - currentCapacity
	if toAdd <= 0 {
		if p.config.verbose {
			log.Printf("[GROW] No new items to add | New capacity: %d | Current capacity: %d", newCapacity, currentCapacity)
		}
		return nil
	}

	for range toAdd {
		if err := newRingBuffer.Write(p.allocator()); err != nil {
			if p.config.verbose {
				log.Printf("[GROW] Error writing new item to ring buffer: %v", err)
			}
			return err
		}
	}
	return nil
}

func (p *Pool[T]) closeMainPool() error {
	if p.pool == nil {
		return nil
	}

	if err := p.pool.Close(); err != nil {
		return fmt.Errorf("failed to close main pool: %w", err)
	}

	p.pool = nil
	return nil
}

func (p *Pool[T]) cleanupCacheL1() {
	if p.cacheL1 == nil {
		return
	}

	for {
		select {
		case obj, ok := <-p.cacheL1:
			if !ok {
				return
			}
			p.cleaner(obj)
		default:
			close(p.cacheL1)
			p.cacheL1 = nil
			return
		}
	}
}

func (p *Pool[T]) resetPoolState() {
	p.stats = nil
	p.isGrowthBlocked = false
	p.isShrinkBlocked = false
	p.allocator = nil
	p.cleaner = nil
}

func (b *poolConfigBuilder) validateBasicConfig() error {
	if b.config.initialCapacity <= 0 {
		return fmt.Errorf("initialCapacity must be greater than 0, got %d", b.config.initialCapacity)
	}

	if b.config.hardLimit <= 0 {
		return fmt.Errorf("hardLimit must be greater than 0, got %d", b.config.hardLimit)
	}

	if b.config.hardLimit < b.config.initialCapacity {
		return fmt.Errorf("hardLimit (%d) must be >= initialCapacity (%d)", b.config.hardLimit, b.config.initialCapacity)
	}

	if b.config.hardLimit < b.config.shrink.minCapacity {
		return fmt.Errorf("hardLimit (%d) must be >= minCapacity (%d)", b.config.hardLimit, b.config.shrink.minCapacity)
	}

	return nil
}

// validateShrinkConfig validates the shrink configuration parameters
func (b *poolConfigBuilder) validateShrinkConfig() error {
	sp := b.config.shrink

	if sp.maxConsecutiveShrinks <= 0 {
		return fmt.Errorf("maxConsecutiveShrinks must be greater than 0, got %d", sp.maxConsecutiveShrinks)
	}

	if sp.checkInterval <= 0 {
		return fmt.Errorf("checkInterval must be greater than 0, got %v", sp.checkInterval)
	}

	if sp.idleThreshold <= 0 {
		return fmt.Errorf("idleThreshold must be greater than 0, got %v", sp.idleThreshold)
	}

	if sp.minIdleBeforeShrink <= 0 {
		return fmt.Errorf("minIdleBeforeShrink must be greater than 0, got %d", sp.minIdleBeforeShrink)
	}

	if sp.idleThreshold < sp.checkInterval {
		return fmt.Errorf("idleThreshold (%v) must be >= checkInterval (%v)", sp.idleThreshold, sp.checkInterval)
	}

	if sp.minCapacity > b.config.initialCapacity {
		return fmt.Errorf("minCapacity (%d) must be <= initialCapacity (%d)", sp.minCapacity, b.config.initialCapacity)
	}

	if sp.shrinkCooldown <= 0 {
		return fmt.Errorf("shrinkCooldown must be greater than 0, got %v", sp.shrinkCooldown)
	}

	if sp.minUtilizationBeforeShrink <= 0 || sp.minUtilizationBeforeShrink > 1.0 {
		return fmt.Errorf("minUtilizationBeforeShrink must be between 0 and 1.0, got %.2f", sp.minUtilizationBeforeShrink)
	}

	if sp.stableUnderutilizationRounds <= 0 {
		return fmt.Errorf("stableUnderutilizationRounds must be greater than 0, got %d", sp.stableUnderutilizationRounds)
	}

	if sp.shrinkPercent <= 0 || sp.shrinkPercent > 1.0 {
		return fmt.Errorf("shrinkPercent must be between 0 and 1.0, got %.2f", sp.shrinkPercent)
	}

	if sp.minCapacity <= 0 {
		return fmt.Errorf("minCapacity must be greater than 0, got %d", sp.minCapacity)
	}

	return nil
}

// validateGrowthConfig validates the growth configuration parameters
func (b *poolConfigBuilder) validateGrowthConfig() error {
	gp := b.config.growth

	if gp.exponentialThresholdFactor <= 0 {
		return fmt.Errorf("exponentialThresholdFactor must be greater than 0, got %.2f", gp.exponentialThresholdFactor)
	}

	if gp.growthPercent <= 0 {
		return fmt.Errorf("growthPercent must be greater than 0, got %.2f", gp.growthPercent)
	}

	if gp.fixedGrowthFactor <= 0 {
		return fmt.Errorf("fixedGrowthFactor must be greater than 0, got %.2f", gp.fixedGrowthFactor)
	}

	return nil
}

// validateFastPathConfig validates the fast path configuration parameters
func (b *poolConfigBuilder) validateFastPathConfig() error {
	fp := b.config.fastPath

	if fp.initialSize <= 0 {
		return fmt.Errorf("fastPath.initialSize must be greater than 0, got %d", fp.initialSize)
	}

	if fp.fillAggressiveness <= 0 || fp.fillAggressiveness > 1.0 {
		return fmt.Errorf("fastPath.fillAggressiveness must be between 0 and 1.0, got %.2f", fp.fillAggressiveness)
	}

	if fp.refillPercent <= 0 || fp.refillPercent >= 1.0 {
		return fmt.Errorf("fastPath.refillPercent must be between 0 and 0.99, got %.2f", fp.refillPercent)
	}

	if fp.growthEventsTrigger <= 0 {
		return fmt.Errorf("fastPath.growthEventsTrigger must be greater than 0, got %d", fp.growthEventsTrigger)
	}

	if fp.growth.exponentialThresholdFactor <= 0 {
		return fmt.Errorf("fastPath.growth.exponentialThresholdFactor must be greater than 0, got %.2f", fp.growth.exponentialThresholdFactor)
	}

	if fp.growth.growthPercent <= 0 {
		return fmt.Errorf("fastPath.growth.growthPercent must be greater than 0, got %.2f", fp.growth.growthPercent)
	}

	if fp.growth.fixedGrowthFactor <= 0 {
		return fmt.Errorf("fastPath.growth.fixedGrowthFactor must be greater than 0, got %.2f", fp.growth.fixedGrowthFactor)
	}

	if fp.shrinkEventsTrigger <= 0 {
		return fmt.Errorf("fastPath.shrinkEventsTrigger must be greater than 0, got %d", fp.shrinkEventsTrigger)
	}

	if fp.shrink.minCapacity <= 0 {
		return fmt.Errorf("fastPath.shrink.minCapacity must be greater than 0, got %d", fp.shrink.minCapacity)
	}

	if fp.shrink.shrinkPercent <= 0 {
		return fmt.Errorf("fastPath.shrink.shrinkPercent must be greater than 0, got %.2f", fp.shrink.shrinkPercent)
	}

	return nil
}
