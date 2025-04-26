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
func (p *Pool[T]) logIfVerbose(format string, args ...any) {
	if p.config.verbose {
		log.Printf(format, args...)
	}
}

func (p *Pool[T]) setPoolAndBuffer(obj T, fastPathRemaining int) (int, error) {
	if fastPathRemaining > 0 {
		select {
		case p.cacheL1 <- obj:
			fastPathRemaining--
			return fastPathRemaining, nil
		default:
		}
	}

	if err := p.pool.Write(obj); err != nil {
		p.logIfVerbose("[SETPOOL] Error writing to ring buffer: %v", err)
		return fastPathRemaining, fmt.Errorf("failed to write to ring buffer: %w", err)
	}

	return fastPathRemaining, nil
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

// calculateUtilization calculates the current utilization percentage of the pool.
// Returns 0 if there are no objects in the pool.
func (p *Pool[T]) calculateUtilization() float64 {
	inUse := p.stats.objectsInUse.Load()
	available := p.pool.Length() + len(p.cacheL1)
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

func (p *Pool[T]) checkGrowthNeeded(fillTarget int) (bool, refillResult) {
	var result refillResult
	noObjsAvailable := p.pool.Length() == 0
	biggerRequestThanAvailable := fillTarget > p.pool.Length()

	if noObjsAvailable || biggerRequestThanAvailable {
		if p.isGrowthBlocked {
			result.Reason = growthBlocked
			return false, result
		}

		now := time.Now()
		if !p.grow(now) {
			result.Reason = growthFailed
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
	if err != nil && err != errIsEmpty {
		p.logIfVerbose("[REFILL] Error getting items from ring buffer: %v", err)
		return nil, nil, fmt.Sprintf("%s: %v", ringBufferError, err), fmt.Errorf("failed to get items from ring buffer: %w", err)
	}

	if part1 == nil && part2 == nil && err == nil || len(part1) == 0 && len(part2) == 0 {
		p.logIfVerbose("[REFILL] No items to move. (fillTarget == 0 || currentObjsAvailable == 0)")
		return nil, nil, noItemsToMove, nil
	}

	return part1, part2, "", nil
}

func (p *Pool[T]) moveItemsToL1(items []T, result *refillResult) error {
	for _, item := range items {
		select {
		case p.cacheL1 <- item:
			result.ItemsMoved++
			p.logIfVerbose("[REFILL] Moved object to L1 | Remaining: %d", p.pool.Length())
		default:
			result.ItemsFailed++
			if err := p.pool.Write(item); err != nil {
				p.logIfVerbose("[REFILL] Error putting object back in ring buffer: %v", err)
				return fmt.Errorf("failed to put object back in ring buffer: %w", err)
			}
		}
	}
	return nil
}

// refill attempts to refill the L1 cache with objects from the pool.
// It returns a refillResult containing information about the refill operation.
func (p *Pool[T]) refill(fillTarget int) refillResult {
	var result refillResult

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

	err = p.moveItemsToL1(part1, &result)
	if err != nil {
		result.Success = false
		result.Reason = fmt.Sprintf("%s: %v", ringBufferError, err)
		return result
	}

	err = p.moveItemsToL1(part2, &result)
	if err != nil {
		result.Success = false
		result.Reason = fmt.Sprintf("%s: %v", ringBufferError, err)
		return result
	}

	result.Success = true
	result.Reason = refillSucceeded
	return result
}

func (p *Pool[T]) slowPathPut(obj T) error {
	if err := p.pool.Write(obj); err != nil {
		return fmt.Errorf("(write slow path) failed to write to ring buffer: %w", err)
	}

	p.stats.FastReturnMiss.Add(1)

	p.logPut("slow path put unblocked")
	return nil
}

func (p *Pool[T]) logRefillResult(result refillResult) {
	if p.config.verbose {
		log.Printf("[REFILL] Refill succeeded: %s | Moved: %d | Failed: %d | Growth needed: %v | Growth blocked: %v",
			result.Reason, result.ItemsMoved, result.ItemsFailed, result.GrowthNeeded, result.GrowthBlocked)
	}
}

func (p *Pool[T]) handleMaxConsecutiveShrinks(params *shrinkParameters) (cannotShrink bool) {
	if params == nil {
		return false
	}

	if p.stats.consecutiveShrinks == params.maxConsecutiveShrinks {
		if p.config.verbose {
			log.Println("[SHRINK] Max consecutive shrinks reached — waiting for Get() call")
		}
		p.isShrinkBlocked = true
		p.shrinkCond.Wait()
		p.isShrinkBlocked = false
		return true
	}

	return false
}

func (p *Pool[T]) handleShrinkCooldown(params *shrinkParameters) (cooldownActive bool) {
	if params == nil {
		return false
	}

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

func (p *Pool[T]) tryRefillAndGetL1() T {
	ableToRefill, refillReason := p.tryRefillIfNeeded()
	if !ableToRefill {
		if obj, shouldContinue := p.handleRefillFailure(refillReason); !shouldContinue {
			return obj
		}
	}

	if obj, found := p.tryGetFromL1(); found {
		return obj
	}

	var zero T
	return zero
}

func (p *Pool[T]) tryRefillIfNeeded() (bool, string) {
	currentLength, currentCap, currentPercent := p.calculateL1Usage()
	p.logL1Usage(currentLength, currentCap, currentPercent)

	if currentPercent > p.config.fastPath.refillPercent {
		return true, noRefillNeeded
	}

	fillTarget := p.calculateFillTarget(currentCap)
	if p.config.verbose {
		log.Printf("[REFILL] Triggering refill — fillTarget: %d", fillTarget)
	}

	result := p.refill(fillTarget)
	if !result.Success {
		if p.config.verbose {
			log.Printf("[REFILL] Refill failed: %s", result.Reason)
		}
		return false, result.Reason
	}

	p.logRefillResult(result)
	return true, refillSucceeded
}

// It blocks if the ring buffer is empty and the ring buffer is in blocking mode.
// We always try to refill the ring buffer before calling the slow path.
func (p *Pool[T]) SlowPath() (obj T, err error) {
	obj, err = p.pool.GetOne()
	if err != nil {
		return obj, fmt.Errorf("(read slow path) failed to get object from ring buffer: %w", err)
	}

	p.logIfVerbose("[SLOWPATH] Object retrieved from ring buffer | Remaining: %d", p.pool.Length())

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

	p.logIfVerbose("[CLOSE] Timed out waiting for all objects to return after %d seconds", maxAttempts)
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

func (p *Pool[T]) L1Length() int {
	return len(p.cacheL1)
}
