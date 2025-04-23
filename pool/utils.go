package pool

import (
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
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isShrinkBlocked {
		if p.config.verbose {
			log.Println("[GET] Shrink is blocked — broadcasting to cond")
		}
		p.cond.Broadcast()
	}

	p.stats.lastTimeCalledGet = time.Now()
}

func (p *Pool[T]) handleRefillFailure(refillReason string) (T, bool) {
	if p.config.verbose {
		log.Printf("[GET] Unable to refill — reason: %s", refillReason)
	}

	var zero T
	if refillReason == growthFailed || refillReason == ringBufferError {
		if p.config.verbose {
			log.Printf("[GET] Warning: unable to refill — reason: %s, returning nil", refillReason)
		}
		return zero, false
	}

	return zero, true
}

func (p *Pool[T]) recordSlowPathStats() {
	p.stats.l2HitCount.Add(1)
	p.updateUsageStats()
}

func (p *Pool[T]) IsShrunk() bool {
	return p.stats.currentCapacity.Load() < uint64(p.config.initialCapacity)
}

func (p *Pool[T]) IsGrowth() bool {
	return p.stats.currentCapacity.Load() > uint64(p.config.initialCapacity)
}

func (p *Pool[T]) Cleaner() func(T) {
	return p.cleaner
}

func (p *Pool[T]) releaseObj(obj T) {
	if reflect.ValueOf(obj).IsNil() {
		log.Println("[RELEASEOBJ] Object is nil")
		return
	}

	p.cleaner(obj)
	p.reduceObjectsInUse()
}

// Using a lock is necessary to avoid race conditions when reducing the number of objects in use.
// even with atomic operations, since we're using two of them.
func (p *Pool[T]) reduceObjectsInUse() {
	p.mu.Lock()
	defer p.mu.Unlock()

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

// This is a helper function to update the peak in use stats.
func (p *Pool[T]) updatePeakInUse(currentInUse uint64) {
	for {
		peak := p.stats.peakInUse.Load()
		if currentInUse <= peak {
			break
		}

		if p.stats.peakInUse.CompareAndSwap(peak, currentInUse) {
			break
		}
	}
}

func (p *Pool[T]) updateConsecutiveShrinks() {
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
