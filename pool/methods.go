package pool

import (
	"context"
	"fmt"
	"log"
	"reflect"
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
	defaultExponentialThresholdFactor                     = 100.0
	defaultGrowthPercent                                  = 0.25
	defaultFixedGrowthFactor                              = 2.0
	defaultfillAggressiveness                             = fillAggressivenessExtreme
	fillAggressivenessExtreme                             = 1.0
	defaultRefillPercent                                  = 0.20
	defaultMinCapacity                                    = 128
	defaultPoolCapacity                                   = 128
	defaultL1MinCapacity                                  = defaultPoolCapacity // L1 doesn't go below its initial capacity
	defaultHardLimit                                      = 1_000_000
	defaultGrowthEventsTrigger                            = 3
	defaultShrinkEventsTrigger                            = 3
	defaultEnableChannelGrowth                            = true
)

const (
	// if any of the three happens, we likely can't get from L1.
	GrowthFailed    = "growth failed"
	RingBufferError = "error getting items from ring buffer"

	// if any of the three happens, we can try to get from L1.
	GrowthBlocked   = "growth is blocked"
	RefillSucceeded = "refill succeeded"
	NoRefillNeeded  = "no refill needed"
	NoItemsToMove   = "no items to move"
)

func NewPool[T any](config *poolConfig, allocator func() T, cleaner func(T), poolType reflect.Type) (*Pool[T], error) {
	if config == nil {
		config = createDefaultConfig()
	}

	stats := initializePoolStats(config)

	ringBuffer, err := NewWithConfig[T](config.initialCapacity, config.ringBufferConfig)
	if err != nil {
		return nil, err
	}

	if err := validateAllocator(allocator); err != nil {
		return nil, err
	}

	poolObj, err := initializePoolObject(config, allocator, cleaner, stats, ringBuffer, poolType)
	if err != nil {
		return nil, err
	}

	poolObj.ctx, poolObj.cancel = context.WithCancel(context.Background())

	if err := populateL1OrBuffer(poolObj); err != nil {
		return nil, err
	}

	go poolObj.shrink()

	return poolObj, nil
}

func (p *Pool[T]) Get() (zero T) {
	p.handleShrinkBlocked()

	ableToRefill, refillReason := p.tryRefillIfNeeded()
	if !ableToRefill {
		if obj, shouldContinue := p.handleRefillFailure(refillReason); !shouldContinue {
			return obj
		}
	}

	if obj, found := p.tryGetFromL1(); found {
		if p.config.verbose {
			var zero T
			if reflect.DeepEqual(obj, zero) {
				log.Printf("[GET] Warning: L1 hit returned zero value")
			}
		}
		return obj
	}

	obj := p.slowPath()
	if p.config.verbose {
		var zero T
		if reflect.DeepEqual(obj, zero) {
			log.Printf("[GET] Warning: slowPath returned zero value")
		}
	}

	p.stats.l2HitCount.Add(1)
	p.updateUsageStats()
	return obj
}

func (p *Pool[T]) Put(obj T) {
	p.releaseObj(obj)

	if p.config.verbose {
		log.Printf("[PUT] Releasing object")
	}

	if p.stats.blockedGets.Load() > 0 {
		p.slowPathPut(obj)
		return
	}

	if p.tryFastPathPut(obj) {
		return
	}

	p.slowPathPut(obj)
}

func (p *Pool[T]) tryRefillIfNeeded() (bool, string) {
	currentLength, currentCap, currentPercent := p.calculateL1Usage()
	p.logL1Usage(currentLength, currentCap, currentPercent)

	if currentPercent > p.config.fastPath.refillPercent {
		return true, NoRefillNeeded
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
	return true, RefillSucceeded
}

// It blocks if the ring buffer is empty and the ring buffer is in blocking mode.
// We always try to refill the pool before calling the slow path.
func (p *Pool[T]) slowPath() (obj T) {
	p.stats.blockedGets.Add(1)
	obj, err := p.pool.GetOne()
	if err != nil {
		if p.config.verbose {
			log.Printf("[SLOWPATH] Error getting object from ring buffer: %v", err)
		}
		return obj
	}

	p.stats.blockedGets.Add(^uint64(0))

	if p.config.verbose {
		log.Printf("[SLOWPATH] Object retrieved from ring buffer | Remaining: %d", p.pool.Length())
	}

	p.stats.mu.Lock()
	p.stats.lastTimeCalledGet = time.Now()
	p.stats.mu.Unlock()

	return obj
}

func (p *Pool[T]) shrink() {
	params := p.config.shrink
	ticker := time.NewTicker(params.checkInterval)
	defer ticker.Stop()

	var (
		idleCount, underutilCount int
		idleOK, utilOK            bool
	)

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.mu.Lock()

			if p.handleMaxConsecutiveShrinks(params) {
				p.mu.Unlock()
				continue
			}

			if p.handleShrinkCooldown(params) {
				p.mu.Unlock()
				continue
			}

			p.performShrinkChecks(params, &idleCount, &underutilCount, &idleOK, &utilOK)

			if idleOK || utilOK {
				p.executeShrink(&idleCount, &underutilCount, &idleOK, &utilOK)
			}

			p.mu.Unlock()
		}
	}
}

// isShrunk returns true if the pool is shrunk.
func (p *Pool[T]) IsShrunk() bool {
	return p.stats.currentCapacity.Load() < uint64(p.config.initialCapacity)
}

// isGrowth, return true if the pool has grown.
func (p *Pool[T]) IsGrowth() bool {
	return p.stats.currentCapacity.Load() > uint64(p.config.initialCapacity)
}

// createAndPopulateBuffer creates a new ring buffer with the specified capacity and populates it
// with existing items from the old buffer and new items from the allocator.
func (p *Pool[T]) createAndPopulateBuffer(newCapacity uint64) (*RingBuffer[T], error) {
	newRingBuffer := p.createNewBuffer(newCapacity)
	if newRingBuffer == nil {
		return nil, fmt.Errorf("failed to create ring buffer")
	}

	items, err := p.getItemsFromOldBuffer()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve items from old buffer: %w", err)
	}

	if err := p.validateAndWriteItems(newRingBuffer, items, newCapacity); err != nil {
		return nil, fmt.Errorf("failed to write items to new buffer: %w", err)
	}

	if err := p.fillRemainingCapacity(newRingBuffer, newCapacity); err != nil {
		return nil, fmt.Errorf("failed to fill remaining capacity: %w", err)
	}

	return newRingBuffer, nil
}

func (p *Pool[T]) grow(now time.Time) bool {
	cfg := p.config.growth
	currentCap := p.stats.currentCapacity.Load()
	objectsInUse := p.stats.objectsInUse.Load()
	exponentialThreshold := uint64(float64(currentCap) * cfg.exponentialThresholdFactor)
	fixedStep := uint64(float64(currentCap) * cfg.fixedGrowthFactor)

	newCapacity := p.calculateNewPoolCapacity(currentCap, exponentialThreshold, fixedStep, cfg)

	if p.needsToShrinkToHardLimit(newCapacity) {
		newCapacity = uint64(p.config.hardLimit)
		if p.config.verbose {
			log.Printf("[GROW] Capacity (%d) > hard limit (%d); shrinking to fit limit", newCapacity, p.config.hardLimit)
		}

		p.isGrowthBlocked = true
	}

	newRingBuffer, err := p.createAndPopulateBuffer(newCapacity)
	if err != nil {
		if p.config.verbose {
			log.Printf("[GROW] Failed to create and populate buffer: %v", err)
		}
		return false
	}

	p.pool = newRingBuffer
	p.stats.currentCapacity.Store(newCapacity)
	p.stats.lastGrowTime = now
	p.stats.l3MissCount.Add(1)
	p.stats.totalGrowthEvents.Add(1)
	p.reduceL1Hit()

	p.tryL1ResizeIfTriggered() // WARNING - heavy operation, avoid resizing too often.

	if p.config.verbose {
		log.Printf("[GROW] Final state | New capacity: %d | Ring buffer length: %d | L1 length: %d | Objects in use: %d",
			newCapacity, p.pool.Length(), len(p.cacheL1), objectsInUse)
	}
	return true
}

func (p *Pool[T]) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.pool == nil && p.cacheL1 == nil {
		return nil
	}

	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}

	if p.cond != nil {
		p.cond.Broadcast()
	}

	if err := p.closeMainPool(); err != nil {
		return err
	}

	p.cleanupCacheL1()

	p.resetPoolState()

	return nil
}
