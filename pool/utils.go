package pool

import (
	"errors"
)

func maxUint64(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

func (p *Pool[T]) handleRefillFailure(refillError error) (T, bool) {
	var zero T
	if errors.Is(refillError, errRingBufferFailed) || errors.Is(refillError, errNilObject) {
		return zero, false
	}

	return zero, true
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
