package pool

import (
	"fmt"
	"log"
	"time"
)

func getShrinkDefaultsMap() map[AggressivenessLevel]*shrinkDefaults {
	return defaultShrinkMap
}

// logIfVerbose is a helper method to reduce logging code duplication
func (p *Pool[T]) logVerbose(format string, args ...any) {
	log.Printf(format, args...)
}

func (p *Pool[T]) setPoolAndBuffer(obj T, fastPathRemaining int) (int, error) {
	chPtr := p.cacheL1.Load()
	if chPtr == nil {
		return fastPathRemaining, fmt.Errorf("cacheL1 is nil")
	}
	ch := *chPtr

	if fastPathRemaining > 0 {
		select {
		case ch <- obj:
			fastPathRemaining--
			return fastPathRemaining, nil
		default:
		}
	}

	if err := p.pool.Write(obj); err != nil {
		if p.config.verbose {
			p.logVerbose("[SETPOOL] Error writing to ring buffer: %v", err)
		}
		return fastPathRemaining, fmt.Errorf("failed to write to ring buffer: %w", err)
	}

	return fastPathRemaining, nil
}

func (p *Pool[T]) IdleCheck(idles *int, shrinkPermissionIdleness *bool) {
	last := p.stats.lastTimeCalledGet.Load()
	idleDuration := time.Since(time.Unix(0, last))

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

// calculateUtilization calculates the current utilization percentage of the pool.
// Returns 0 if there are no objects in the pool.
func (p *Pool[T]) calculateUtilization() float64 {
	chPtr := p.cacheL1.Load()
	if chPtr == nil {
		return 0
	}
	ch := *chPtr

	inUse := p.stats.objectsInUse.Load()
	available := p.pool.Length() + len(ch)
	total := inUse + uint64(available)

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

// ApplyDefaults applies default values to the shrink parameters based on the aggressiveness level.
// It ensures the aggressiveness level is within valid bounds and applies corresponding defaults.
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
func (p *Pool[T]) isGrowthNeeded(fillTarget int) bool {
	poolLength := p.pool.Length()
	noObjsAvailable := poolLength == 0

	return noObjsAvailable || fillTarget > poolLength
}

func (p *Pool[T]) poolGrowthNeeded(fillTarget int) (bool, *refillResult) {
	var result refillResult

	if p.isGrowthBlocked.Load() {
		result.Error = errGrowthBlocked
		return false, &result
	}

	if p.isGrowthNeeded(fillTarget) {
		now := time.Now()
		err := p.grow(now)
		if err != nil {
			result.Error = err
			return false, &result
		}

		result.Success = true
	}

	return true, &result
}

func (p *Pool[T]) getItemsToMove(fillTarget int) ([]T, []T, error) {
	currentObjsAvailable := p.pool.Length()
	toMove := min(fillTarget, currentObjsAvailable)

	part1, part2, err := p.pool.GetNView(toMove)
	if err != nil && err != errIsEmpty {
		if p.config.verbose {
			p.logVerbose("[REFILL] Error getting items from ring buffer: %v", err)
		}
		return nil, nil, errRingBufferFailed
	}

	if part1 == nil && part2 == nil && err == nil || len(part1) == 0 && len(part2) == 0 {
		if p.config.verbose {
			p.logVerbose("[REFILL] No items to move. (fillTarget == 0 || currentObjsAvailable == 0)")
		}
		return nil, nil, errNoItemsToMove
	}

	return part1, part2, nil
}

func (p *Pool[T]) moveItemsToL1(items []T, result *refillResult) {
	defer func() {
		if r := recover(); r != nil {
			if p.config.verbose {
				p.logVerbose("[PUT] panic on fast path put — channel closed")
			}
			result.Error = fmt.Errorf("panic on fast path put — channel closed")
		}
	}()

	chPtr := p.cacheL1.Load()
	if chPtr == nil {
		result.Error = fmt.Errorf("cacheL1 is nil")
		return
	}
	ch := *chPtr

	for _, item := range items {
		select {
		case ch <- item:
			result.ItemsMoved++
			if p.config.verbose {
				p.logVerbose("[REFILL] Moved object to L1 | Remaining: %d", len(ch))
			}
		default:
			result.ItemsFailed++
			if err := p.pool.Write(item); err != nil {
				if p.config.verbose {
					p.logVerbose("[REFILL] Error putting object back in ring buffer: %v", err)
				}
				result.Error = fmt.Errorf("%w: %w", errRingBufferFailed, err)
				return
			}
		}
	}
}

// refill attempts to refill the L1 cache with objects from the pool.
// It returns a refillResult containing information about the refill operation.
func (p *Pool[T]) refill(fillTarget int) *refillResult {
	var result *refillResult

	shouldContinue, growthResult := p.poolGrowthNeeded(fillTarget)
	if !shouldContinue {
		return growthResult
	}

	result = growthResult
	part1, part2, err := p.getItemsToMove(fillTarget)
	if err != nil {
		result.Success = false
		result.Error = err
		return result
	}

	p.moveItemsToL1(part1, result)
	if result.Error != nil {
		return result
	}

	p.moveItemsToL1(part2, result)
	if result.Error != nil {
		return result
	}

	result.Success = true
	return result
}

func (p *Pool[T]) slowPathPut(obj T) error {
	if err := p.pool.Write(obj); err != nil {
		return fmt.Errorf("%w: %w", errRingBufferFailed, err)
	}

	p.stats.FastReturnMiss.Add(1)

	if p.config.verbose {
		p.logVerbose("[PUT] slow path put unblocked")
	}
	return nil
}

func (p *Pool[T]) logRefillResult(result *refillResult) {
	if p.config.verbose {
		log.Printf("[REFILL] Refill succeeded: %s | Moved: %d | Failed: %d",
			result.Error, result.ItemsMoved, result.ItemsFailed)
	}
}

func (p *Pool[T]) handleMaxConsecutiveShrinks(params *shrinkParameters) (cannotShrink bool) {
	if params == nil {
		return cannotShrink
	}

	for p.stats.consecutiveShrinks.Load() == int32(params.maxConsecutiveShrinks) {
		if p.config.verbose {
			log.Println("[SHRINK] Max consecutive shrinks reached — waiting for Get() call")
		}
		p.isShrinkBlocked.Store(true)
		p.shrinkCond.Wait()
	}

	return cannotShrink
}

func (p *Pool[T]) handleShrinkCooldown(params *shrinkParameters) (cooldownActive bool) {
	if params == nil {
		return cooldownActive
	}

	timeSinceLastShrink := time.Since(p.stats.lastShrinkTime)
	if timeSinceLastShrink < params.shrinkCooldown {
		if p.config.verbose {
			log.Printf("[SHRINK] Cooldown active: %v remaining", params.shrinkCooldown-timeSinceLastShrink)
		}
		cooldownActive = true
		return cooldownActive
	}

	return cooldownActive
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

func (p *Pool[T]) isL1UsageBelowThreshold() bool {
	currentLength, currentCap, currentPercent := p.calculateL1Usage()
	p.logL1Usage(currentLength, currentCap, currentPercent)
	return currentPercent <= p.config.fastPath.refillPercent
}

func (p *Pool[T]) tryRefillIfNeeded() (bool, *refillResult) {
	currentLength, currentCap, currentPercent := p.calculateL1Usage()
	p.logL1Usage(currentLength, currentCap, currentPercent)

	if currentPercent > p.config.fastPath.refillPercent {
		return true, nil
	}

	fillTarget := p.calculateFillTarget(currentCap)
	if p.config.verbose {
		log.Printf("[REFILL] Triggering refill — fillTarget: %d", fillTarget)
	}

	refillResult := p.refill(fillTarget)
	if !refillResult.Success {
		if p.config.verbose {
			log.Printf("[REFILL] Refill failed: %s", refillResult.Error)
		}
		return false, refillResult
	}

	p.logRefillResult(refillResult)
	return true, refillResult
}

// It blocks if the ring buffer is empty and the ring buffer is in blocking mode.
// We always try to refill the ring buffer before calling the slow path.
func (p *Pool[T]) SlowPath() (obj T, err error) {
	var pool *RingBuffer[T]

	p.mu.RLock()
	pool = p.pool
	p.mu.RUnlock()

	if p.config.verbose {
		p.logVerbose("[SLOWPATH] Getting object from ring buffer")
	}

	obj, err = pool.GetOne()
	if err != nil {
		return obj, fmt.Errorf("%w: %w", errRingBufferFailed, err)
	}

	if p.config.verbose {
		p.logVerbose("[SLOWPATH] Object retrieved from ring buffer | Remaining: %d", p.pool.Length())
	}

	return obj, nil
}

func (p *Pool[T]) RingBufferCapacity() int {
	return p.pool.Capacity()
}

func (p *Pool[T]) RingBufferLength() int {
	return p.pool.Length()
}

func (p *Pool[T]) hasOutstandingObjects() bool {
	totalGets := p.stats.totalGets.Load()
	totalReturns := p.stats.FastReturnHit.Load() + p.stats.FastReturnMiss.Load()
	return totalReturns < totalGets
}

func (p *Pool[T]) closeAsync() error {
	p.waitAndClose()
	return nil
}

func (p *Pool[T]) waitAndClose() {
	maxAttempts := 3
	attempts := 0

	for attempts < maxAttempts {
		totalReturns := p.stats.FastReturnHit.Load() + p.stats.FastReturnMiss.Load()
		if totalReturns >= p.stats.totalGets.Load() {
			p.performClosure()
			return
		}
		time.Sleep(1 * time.Second)
		attempts++
	}

	if p.config.verbose {
		p.logVerbose("[CLOSE] Timed out waiting for all objects to return after %d seconds", maxAttempts)
	}
	p.performClosure()
}

func (p *Pool[T]) closeImmediate() error {
	p.performClosure()
	return nil
}

func (p *Pool[T]) performClosure() {
	p.closed.Store(true)
	p.cancel()
	p.shrinkCond.Broadcast()
	p.pool.Close()
	p.cleanupCacheL1()
}

// GetBlockedReaders returns the number of readers currently blocked waiting for objects
func (p *Pool[T]) GetBlockedReaders() int {
	return p.pool.GetBlockedReaders()
}

func (p *Pool[T]) tryRefillAndGetL1() (T, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ableToRefill, refillResult := p.tryRefillIfNeeded()
	if !ableToRefill && refillResult.Error != nil {
		if obj, shouldContinue := p.handleRefillFailure(refillResult.Error); !shouldContinue {
			return obj, false
		}
	}

	if obj, found := p.tryGetFromL1(true); found {
		return obj, true
	}

	var zero T
	return zero, false
}
