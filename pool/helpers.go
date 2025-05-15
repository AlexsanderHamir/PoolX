package pool

import (
	"errors"
	"fmt"
	"log"
	"time"

	ringbufferInternalErrs "github.com/AlexsanderHamir/ringbuffer/errors"
)

// shrinkDefaultsMap returns the default shrink configuration map based on aggressiveness levels
func getShrinkDefaultsMap() map[AggressivenessLevel]*shrinkDefaults {
	return defaultShrinkMap
}

// setPoolAndBuffer attempts to store an object in either the L1 cache or the main pool.
// It returns the remaining fast path capacity and any error that occurred.
func (p *Pool[T]) setPoolAndBuffer(obj T, fastPathRemaining int) (int, error) {
	defer func() {
		if r := recover(); r != nil {
		}
	}()

	chPtr := p.cacheL1
	ch := *chPtr

	if fastPathRemaining > 0 {
		select {
		case ch <- obj:
			fastPathRemaining--
			return fastPathRemaining, nil
		default:
		}
	}

	// Store in main pool
	if err := p.pool.Write(obj); err != nil {
		return fastPathRemaining, fmt.Errorf("failed to write to ring buffer: %w", err)
	}

	return fastPathRemaining, nil
}

// calculateUtilization calculates the current utilization percentage of the pool.
// Returns 0 if there are no objects in the pool or if the L1 cache is nil.
func (p *Pool[T]) calculateUtilization() int {
	inUse := p.stats.totalGets.Load() - (p.stats.FastReturnHit.Load() + p.stats.FastReturnMiss.Load())
	return (int(inUse) / p.pool.Capacity()) * 100
}

func (p *Pool[T]) isUnderUtilized() bool {
	utilization := p.calculateUtilization()
	minUtil := p.config.shrink.minUtilizationBeforeShrink
	return utilization <= minUtil
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
	p.shrinkCooldown = def.cooldown
	p.minUtilizationBeforeShrink = def.utilization
	p.stableUnderutilizationRounds = def.underutilized
	p.shrinkPercent = def.percent
	p.maxConsecutiveShrinks = def.maxShrinks
	p.minCapacity = defaultMinCapacity
}
func (p *Pool[T]) isGrowthNeeded(fillTarget int) bool {
	poolLength := p.pool.Length(false)
	noObjsAvailable := poolLength == 0

	return noObjsAvailable || fillTarget > poolLength
}

func (p *Pool[T]) poolGrowthNeeded(fillTarget int) (ableToGrow bool, err error) {
	if p.isGrowthBlocked.Load() {
		return false, errGrowthBlocked
	}

	if p.isGrowthNeeded(fillTarget) {
		err := p.grow()
		if err != nil {
			return false, err
		}
	}

	return true, nil
}

func (p *Pool[T]) getItemsToMove(fillTarget int) ([]T, []T, error) {
	currentObjsAvailable := p.pool.Length(false)
	toMove := min(fillTarget, currentObjsAvailable)

	part1, part2, err := p.pool.GetNView(toMove)
	if err != nil && err != ringbufferInternalErrs.ErrIsEmpty {
		return nil, nil, errRingBufferFailed
	}

	if len(part1) == 0 && len(part2) == 0 {
		return nil, nil, errNoItemsToMove
	}

	return part1, part2, nil
}

func (p *Pool[T]) moveItemsToL1(items []T) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[MOVE] panic on move items to L1: %v", r)
		}
	}()

	chPtr := p.cacheL1
	ch := *chPtr

	for _, item := range items {
		select {
		case ch <- item:
		default:
			if err := p.pool.Write(item); err != nil {
				return fmt.Errorf("%w: %w", errRingBufferFailed, err)
			}
		}
	}
	return nil
}

// refill attempts to refill the L1 cache with objects from the pool.
// Returns the number of items moved, number of items failed, and any error that occurred.
func (p *Pool[T]) refill(fillTarget int) error {
	ableToGrow, err := p.poolGrowthNeeded(fillTarget)
	if !ableToGrow && err != nil {
		return err
	}

	part1, part2, err := p.getItemsToMove(fillTarget)
	if err != nil {
		return err
	}

	err = p.moveItemsToL1(part1)
	if err != nil {
		return err
	}

	err = p.moveItemsToL1(part2)
	if err != nil {
		return err
	}

	return nil
}

func (p *Pool[T]) createOnDemand(fillTarget int, spaceAvailable int) error {
	allocAmount := p.config.allocationStrategy.AllocAmount
	allocAmount = min(allocAmount, spaceAvailable, fillTarget)

	if allocAmount == 0 {
		return nil
	}

	return p.populateL1OrBuffer(allocAmount)
}

func (p *Pool[T]) slowPathPut(obj T) error {
	const maxRetries = 5
	const retryDelay = 10 * time.Millisecond

	var err error

	for i := range maxRetries {
		p.mu.RLock()
		pool := p.pool
		p.mu.RUnlock()

		if err = pool.Write(obj); err == nil {
			p.stats.FastReturnMiss.Add(1)
			return nil
		}

		if i < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}

	return fmt.Errorf("%w: %w", errRingBufferFailed, err)
}

func (p *Pool[T]) handleMaxConsecutiveShrinks(maxConsecutiveShrinks int) (cannotShrink bool) {
	if p.stats.consecutiveShrinks == maxConsecutiveShrinks {
		p.shrinkCond.Wait()
		p.stats.consecutiveShrinks = 0
		return true
	}

	return cannotShrink
}

func (p *Pool[T]) handleShrinkCooldown(shrinkCooldown time.Duration) (cooldownActive bool) {
	timeSinceLastShrink := time.Since(p.stats.lastShrinkTime)
	return timeSinceLastShrink < shrinkCooldown
}

func (p *Pool[T]) tryRefill(fillTarget int) (bool, error) {
	err := p.refill(fillTarget)
	if err != nil {
		return false, err
	}

	return true, nil
}

// SlowPath retrieves an object from the ring buffer. It blocks if the ring buffer is empty
// and the ring buffer is in blocking mode. We always try to refill the ring buffer before
// calling the slow path.
func (p *Pool[T]) SlowPathGet() (obj T, err error) {
	const maxRetries = 5
	const retryDelay = 10 * time.Millisecond

	for i := range maxRetries {
		p.mu.RLock()
		pool := p.pool
		p.mu.RUnlock()

		obj, err = pool.GetOne()
		if err == nil {
			p.stats.totalGets.Add(1)
			return obj, nil
		}

		if i < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}

	return obj, fmt.Errorf("%w: %w", errRingBufferFailed, err)
}

func (p *Pool[T]) RingBufferCapacity() int {
	return p.pool.Capacity()
}

func (p *Pool[T]) RingBufferLength() int {
	return p.pool.Length(false)
}

func (p *Pool[T]) hasOutstandingObjects() bool {
	totalGets := p.stats.totalGets.Load()
	totalReturns := p.stats.FastReturnHit.Load() + p.stats.FastReturnMiss.Load()
	return totalReturns < totalGets
}

// closeAsync implements the waiting logic for closeAsync. It will attempt to wait for all
// outstanding objects to be returned to the pool before closing. If objects are not returned
// within the timeout period (10 seconds), it will force close the pool anyway.
func (p *Pool[T]) closeAsync() {
	maxAttempts := 10
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

	p.performClosure()
}

// performClosure handles the actual cleanup of pool resources. It:
// 1. Marks the pool as closed
// 2. Cancels the pool's context
// 3. Broadcasts to any waiting shrink operations
// 4. Closes the underlying ring buffer
// 5. Cleans up the L1 cache
func (p *Pool[T]) performClosure() {
	p.shrinkCond.Signal()
	p.cancel()
	p.pool.Close()
	p.cleanupCacheL1()
}

// GetBlockedReaders returns the number of readers currently blocked waiting for objects
func (p *Pool[T]) GetBlockedReaders() int {
	return p.pool.GetBlockedReaders()
}

// tryRefillAndGetL1 attempts to refill the pool, and get an object from L1 cache.
// It will grow in case it's allowed and needed.
func (p *Pool[T]) tryRefillAndFromGetL1() (zero T, canProceed bool) {
	select {
	case p.refillSemaphore <- struct{}{}:
		defer func() {
			p.refillCond.Broadcast()
			<-p.refillSemaphore
		}()
		obj, canProceed := p.handleRefillScenarios()
		return obj, canProceed
	default:
		p.refillCond.L.Lock()
		p.refillCond.Wait()
		p.refillCond.L.Unlock()

		if obj, found := p.tryGetFromL1(false); found {
			return obj, true
		}

		return zero, false
	}
}

// tryGetFromL1IfWellStocked attempts to get an object from L1 cache if it's well stocked
func (p *Pool[T]) tryGetFromL1IfWellStocked(currentPercent int) (obj T, found bool) {
	if currentPercent > p.config.fastPath.refillPercent {
		return p.tryGetFromL1(false)
	}
	return obj, false
}

// tryCreateAndGetFromL1 attempts to create new objects and get one from L1 cache
func (p *Pool[T]) tryCreateAndGetFromL1(fillTarget int) (obj T, found bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	spaceAvailable := p.pool.Capacity() - (p.stats.objectsCreated - p.stats.objectsDestroyed)
	if spaceAvailable <= 0 {
		return obj, false
	}

	if err := p.createOnDemand(fillTarget, spaceAvailable); err != nil {
		return obj, false
	}

	return p.tryGetFromL1(true)
}

// tryRefillAndGetFromL1 attempts to refill from main pool and get from L1 cache
func (p *Pool[T]) tryRefillAndGetFromL1(fillTarget int) (obj T, found bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	ableToRefill, err := p.tryRefill(fillTarget)
	if !ableToRefill && err != nil {
		if obj, shouldContinue := p.handleRefillFailure(err); !shouldContinue {
			return obj, false
		}
	}

	return p.tryGetFromL1(true)
}

func (p *Pool[T]) handleRefillScenarios() (zero T, canProceed bool) {
	p.mu.RLock()
	currentCap, currentPercent := p.calculateL1Usage()
	fillTarget := p.calculateFillTarget(currentCap)
	p.mu.RUnlock()

	if obj, found := p.tryGetFromL1IfWellStocked(currentPercent); found {
		return obj, true
	}

	if obj, found := p.tryCreateAndGetFromL1(fillTarget); found {
		return obj, true
	}

	if obj, found := p.tryRefillAndGetFromL1(fillTarget); found {
		return obj, true
	}

	return zero, false
}

func checkConfigForNil[T any](config *PoolConfig[T]) error {
	if config.ringBufferConfig == nil {
		return errNilConfig
	}

	if config.shrink == nil {
		return errNilConfig
	}

	if config.growth == nil {
		return errNilConfig
	}

	if config.allocationStrategy == nil {
		return errNilConfig
	}

	return nil
}

func (p *Pool[T]) handleRefillFailure(refillError error) (T, bool) {
	var zero T
	if errors.Is(refillError, errRingBufferFailed) || errors.Is(refillError, errNilObject) {
		return zero, false
	}

	return zero, true
}

func (p *Pool[T]) IsShrunk() bool {
	return p.stats.currentCapacity < p.config.initialCapacity
}

func (p *Pool[T]) IsGrowth() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stats.currentCapacity > p.config.initialCapacity
}
