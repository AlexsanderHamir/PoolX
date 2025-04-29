package pool

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"
)

var (
	errPoolClosed       = errors.New("pool is closed")
	errGrowthBlocked    = errors.New("growth is blocked")
	errRingBufferFailed = errors.New("ring buffer failed core operation")
	errNoItemsToMove    = errors.New("no items to move")
	errNilObject        = errors.New("object is nil")
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

	ringBuffer.WithPreReadBlockHook(poolObj.preReadBlockHook)

	poolObj.ctx, poolObj.cancel = context.WithCancel(context.Background())

	if err := populateL1OrBuffer(poolObj); err != nil {
		return nil, err
	}

	go poolObj.shrink()

	return poolObj, nil
}

// Get returns an object from the pool, either from L1 or the ring buffer, preferring L1.
func (p *Pool[T]) Get() (zero T, err error) {
	if p.closed.Load() {
		return zero, errPoolClosed
	}

	p.handleShrinkBlocked()

	p.mu.RLock()
	if obj, found := p.tryGetFromL1(); found { // potential race condition
		p.mu.RUnlock()
		return obj, nil
	}
	p.mu.RUnlock()

	if obj := p.tryRefillAndGetL1(); !isNil(obj) {
		return obj, nil
	}

	obj, err := p.SlowPath()
	if err != nil {
		return obj, err
	}

	p.recordSlowPathStats()
	return obj, nil
}

func (p *Pool[T]) Put(obj T) error {
	if isNil(obj) {
		return fmt.Errorf("from Put: %w", errNilObject)
	}

	if p.closed.Load() {
		return errPoolClosed
	}

	p.releaseObj(obj)

	if p.config.verbose {
		p.logVerbose("[PUT] Releasing object")
	}

	blockedReaders := p.pool.GetBlockedReaders()
	if blockedReaders > 0 {
		err := p.slowPathPut(obj)
		if err != nil {
			if p.config.verbose {
				p.logVerbose("[PUT] Error in slowPathPut: %v", err)
			}
			return err
		}
		return nil
	}

	if ok := p.tryFastPathPut(obj); ok { // potential race condition
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
			if p.closed.Load() {
				p.mu.Unlock()
				return
			}

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
func (p *Pool[T]) grow(now time.Time) error {
	if p.closed.Load() {
		return errPoolClosed
	}

	if p.isGrowthBlocked {
		return errGrowthBlocked
	}

	currentCap, objectsInUse, exponentialThreshold, fixedStep := p.calculateGrowthParameters()
	newCapacity := p.calculateNewPoolCapacity(currentCap, exponentialThreshold, fixedStep, p.config.growth)

	if err := p.updatePoolCapacity(newCapacity); err != nil {
		if p.config.verbose {
			p.logVerbose("[GROW] Error in updatePoolCapacity: %v", err)
		}
		return fmt.Errorf("%w: %w", errRingBufferFailed, err)
	}

	p.stats.totalGrowthEvents.Add(1)
	if p.config.enableStats {
		p.updateGrowthStats(now)
	}

	err := p.tryL1ResizeIfTriggered()
	if err != nil {
		if p.config.verbose {
			p.logVerbose("[GROW] Error in tryL1ResizeIfTriggered: %v", err)
		}
		return err
	}
	if p.config.verbose {
		p.logGrowthState(newCapacity, objectsInUse)
	}
	return nil
}

func (p *Pool[T]) Close() error {
	if p.closed.Load() {
		return errors.New("pool is already closed")
	}

	if p.hasOutstandingObjects() {
		return p.closeAsync()
	}

	return p.closeImmediate()
}

// Hook to Attempt to get an object from L1
func (p *Pool[T]) preReadBlockHook() bool {
	attempts := p.config.fastPath.preReadBlockHookAttempts
	if attempts <= 0 {
		attempts = 1
	}

	chPtr := p.cacheL1.Load()
	if chPtr == nil {
		return false
	}
	ch := *chPtr

	for i := range attempts {
		select {
		case obj, ok := <-ch:
			if !ok {
				return false
			}
			err := p.pool.Write(obj)
			if err != nil {
				if p.config.verbose {
					p.logVerbose("[PREBLOCKHOOK] Error in pool.Write: %v", err)
				}
				continue
			}
			return true
		default:
			if i == attempts-1 {
				return false
			}
		}
	}
	return false
}

// GetBlockedReaders returns the number of readers currently blocked waiting for objects
func (p *Pool[T]) GetBlockedReaders() int {
	return p.pool.GetBlockedReaders()
}
