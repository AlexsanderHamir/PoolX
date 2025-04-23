package pool

import (
	"context"
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
	defaultGrowthPercent                                  = 0.5
	defaultFixedGrowthFactor                              = 1.5
	defaultfillAggressiveness                             = 0.8
	fillAggressivenessExtreme                             = 1.0
	defaultRefillPercent                                  = 0.2
	defaultMinCapacity                                    = 32
	defaultPoolCapacity                                   = 64
	defaultL1MinCapacity                                  = defaultPoolCapacity // L1 doesn't go below its initial capacity
	defaultHardLimit                                      = 10_000
	defaultGrowthEventsTrigger                            = 3
	defaultShrinkEventsTrigger                            = 3
	defaultEnableChannelGrowth                            = true
)

const (
	// if any of these happens, we likely can't get from L1.
	growthFailed    = "growth failed"
	ringBufferError = "error getting items from ring buffer"

	// if any of the these happens, we can try to get from L1.
	growthBlocked   = "growth is blocked"
	refillSucceeded = "refill succeeded"
	noRefillNeeded  = "no refill needed"
	noItemsToMove   = "no items to move"
)

// Creates a new pool with the given config, allocator, cleaner, and pool type, only pointers can be stored.
func NewPool[T any](config *PoolConfig, allocator func() T, cleaner func(T), poolType reflect.Type) (*Pool[T], error) {
	if config == nil {
		config = createDefaultConfig()
	}

	stats := initializePoolStats(config)

	ringBuffer, err := NewRingBufferWithConfig[T](config.initialCapacity, config.ringBufferConfig)
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

// Get returns an object from the pool, either from L1 or the ring buffer, preferring L1.
func (p *Pool[T]) Get() T {
	p.handleShrinkBlocked()

	if obj, found := p.tryGetFromL1(); found {
		p.warningIfZero(obj, "L1")
		return obj
	}

	if obj := p.tryRefillAndGetL1(); !isZero(obj) {
		return obj
	}

	obj := p.slowPath()
	p.warningIfZero(obj, "slowPath")
	p.recordSlowPathStats()
	return obj
}

// Put returns an object to the pool, either to L1 or the ring buffer, preferring L1.
func (p *Pool[T]) Put(obj T) {
	p.releaseObj(obj)

	if p.config.verbose {
		log.Printf("[PUT] Releasing object")
	}

	blockedReaders := p.pool.GetBlockedReaders()
	if blockedReaders > 0 {
		p.slowPathPut(obj)
		return
	}

	if p.tryFastPathPut(obj) {
		return
	}

	p.slowPathPut(obj)
}

// shrink is a background goroutine that checks the pool for idle and underutilized objects, and shrinks the pool if necessary.
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

			if p.config.verbose {
				p.PrintPoolStats()
			}
		}
	}
}

// grow is a background goroutine that grows the pool if necessary.
func (p *Pool[T]) grow(now time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	cfg := p.config.growth
	currentCap := p.stats.currentCapacity.Load()
	objectsInUse := p.stats.objectsInUse.Load()
	exponentialThreshold := uint64(float64(p.config.initialCapacity) * cfg.exponentialThresholdFactor)
	fixedStep := uint64(float64(p.config.initialCapacity) * cfg.fixedGrowthFactor)

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

	p.tryL1ResizeIfTriggered()
	if p.config.verbose {
		log.Printf("[GROW] Final state | New capacity: %d | Ring buffer length: %d | L1 length: %d | Objects in use: %d",
			newCapacity, p.pool.Length(), len(p.cacheL1), objectsInUse)
	}

	return true
}

// Close closes the pool, releasing all resources.
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
