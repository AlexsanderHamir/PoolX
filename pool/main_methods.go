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
	if err := validateAllocator(allocator); err != nil {
		return nil, err
	}

	if config == nil {
		config = createDefaultConfig()
	}

	ringBuffer, err := NewRingBufferWithConfig[T](config.initialCapacity, config.ringBufferConfig)
	if err != nil {
		return nil, err
	}

	stats := initializePoolStats(config)

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
		return obj
	}

	if obj := p.tryRefillAndGetL1(); !isZero(obj) {
		return obj
	}

	obj, err := p.slowPath()
	if err != nil {
		p.logIfVerbose("[GET] Error in slowPath: %v", err)
	}

	p.warningIfZero(obj, "slowPath")
	p.recordSlowPathStats()
	return obj
}

// Put returns an object to the pool, either to L1 or the ring buffer, preferring L1.
func (p *Pool[T]) Put(obj T) error {
	p.releaseObj(obj)

	if p.config.verbose {
		log.Printf("[PUT] Releasing object")
	}

	blockedReaders := p.pool.GetBlockedReaders()
	if blockedReaders > 0 {
		err := p.slowPathPut(obj)
		if err != nil {
			p.logIfVerbose("[PUT] Error in slowPathPut: %v", err)
		}
		return err
	}

	if p.tryFastPathPut(obj) {
		return nil
	}

	return p.slowPathPut(obj)
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

			cannotShrink := p.handleMaxConsecutiveShrinks(params)
			if cannotShrink {
				p.mu.Unlock()
				continue
			}

			cooldownActive := p.handleShrinkCooldown(params)
			if cooldownActive {
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

// grow is called when the demand for objects exceeds the current capacity, if enabled.
func (p *Pool[T]) grow(now time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	currentCap, objectsInUse, exponentialThreshold, fixedStep := p.calculateGrowthParameters()
	newCapacity := p.calculateNewPoolCapacity(currentCap, exponentialThreshold, fixedStep, p.config.growth)

	if err := p.updatePoolCapacity(newCapacity); err != nil {
		p.logIfVerbose("[GROW] Error in updatePoolCapacity: %v", err)
		return false
	}

	p.updateGrowthStats(now)
	p.tryL1ResizeIfTriggered()
	p.logGrowthState(newCapacity, objectsInUse)

	return true
}

// closes the pool, releasing all resources.
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
