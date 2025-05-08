package pool

import (
	"errors"
)

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
