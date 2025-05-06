package pool

import (
	"errors"
	"log"

	"time"
)

func maxUint64(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

func (p *Pool[T]) handleShrinkBlocked() {
	if p.stats.consecutiveShrinks.Load() > 0 {
		p.reduceConsecutiveShrinks()
	}

	if p.isShrinkBlocked.CompareAndSwap(true, false) {
		p.shrinkCond.Signal()

		if p.config.verbose {
			log.Println("[GET] Shrink is blocked — broadcasting to cond")
		}
	}

	now := time.Now().UnixNano()
	p.stats.lastTimeCalledGet.Store(now)
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
	if errors.Is(refillError, errRingBufferFailed) || errors.Is(refillError, errNilObject) {
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
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stats.currentCapacity > uint64(p.config.initialCapacity)
}

func (p *Pool[T]) releaseObj(obj T) {
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

func (p *Pool[T]) reduceConsecutiveShrinks() {
	for {
		old := p.stats.consecutiveShrinks.Load()
		if old == 0 {
			break
		}

		if p.stats.consecutiveShrinks.CompareAndSwap(old, old-1) {
			break
		}
	}
}
