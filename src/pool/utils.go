package pool

import (
	"errors"
	"log"
	"reflect"
	"time"
)

func maxUint64(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

func isZero[T any](obj T) bool {
	var zero T
	return reflect.DeepEqual(obj, zero)
}

func (p *Pool[T]) warningIfZero(obj T, source string) {
	if isZero(obj) && p.config.verbose {
		log.Printf("[GET] Warning: %s returned zero value", source)
	}
}

func (p *Pool[T]) logPut(message string) {
	if p.config.verbose {
		log.Printf("[PUT] %s", message)
	}
}

func (p *Pool[T]) handleShrinkBlocked() {
	p.stats.mu.Lock()
	defer p.stats.mu.Unlock()

	if p.isShrinkBlocked {
		if p.config.verbose {
			log.Println("[GET] Shrink is blocked — broadcasting to cond")
		}
		p.shrinkCond.Broadcast()
	}

	p.stats.lastTimeCalledGet = time.Now()
	if p.stats.consecutiveShrinks > 0 {
		p.stats.consecutiveShrinks--
	}
}

func (p *Pool[T]) handleRefillFailure(refillError error) (T, bool) {
	if p.closed.Load() {
		var zero T
		return zero, false
	}

	if p.config.verbose {
		log.Printf("[GET] Unable to refill — reason: %s", refillError)
	}

	var zero T
	if errors.Is(refillError, errRingBufferFailed) {
		if p.config.verbose {
			log.Printf("[GET] Warning: unable to refill — reason: %s, returning nil", refillError)
		}
		return zero, false
	}

	return zero, true
}

func (p *Pool[T]) recordSlowPathStats() {
	p.updateUsageStats()

	if p.config.enableStats {
		p.stats.l2HitCount.Add(1)
	}
}

func (p *Pool[T]) IsShrunk() bool {
	return p.stats.currentCapacity < uint64(p.config.initialCapacity)
}

func (p *Pool[T]) IsGrowth() bool {
	return p.stats.currentCapacity > uint64(p.config.initialCapacity)
}

func (p *Pool[T]) Cleaner() func(T) {
	return p.cleaner
}

func (p *Pool[T]) releaseObj(obj T) {
	if reflect.ValueOf(obj).IsNil() {
		p.logIfVerbose("[RELEASEOBJ] Object is nil")
		return
	}

	p.cleaner(obj)
	p.reduceObjectsInUse()
}

func (p *Pool[T]) reduceObjectsInUse() {
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
