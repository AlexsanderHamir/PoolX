package pool

import (
	"fmt"
	"log"
)

// tryL1ResizeIfTriggered attempts to resize the L1 cache channel if growth events have exceeded
// the configured trigger threshold. It implements an adaptive growth strategy that uses either
// exponential or fixed growth based on the current capacity relative to a threshold.
func (p *Pool[T]) tryL1ResizeIfTriggered() error {
	trigger := uint64(p.config.fastPath.growthEventsTrigger)
	if !p.config.fastPath.enableChannelGrowth {
		return nil
	}

	sinceLastResize := p.stats.totalGrowthEvents.Load() - p.stats.lastL1ResizeAtGrowthNum
	if sinceLastResize < trigger {
		return nil
	}

	currentCap := p.stats.currentL1Capacity
	cfg := p.config.fastPath.growth
	threshold := uint64(float64(currentCap) * cfg.exponentialThresholdFactor)
	step := uint64(float64(currentCap) * cfg.fixedGrowthFactor)

	var newCap uint64
	if currentCap < threshold {
		newCap = currentCap + maxUint64(uint64(float64(currentCap)*cfg.growthPercent), 1)
	} else {
		newCap = currentCap + step
	}

	if p.config.verbose {
		log.Printf("[RESIZE] Starting L1 resize from %d to %d capacity", currentCap, newCap)
	}

	oldChPtr := p.cacheL1.Load()
	if oldChPtr == nil {
		return fmt.Errorf("cacheL1 is nil")
	}

	oldCh := *oldChPtr
	newCh := make(chan T, newCap)

	close(oldCh)
	p.cacheL1.Store(&newCh)

	p.stats.currentL1Capacity = newCap
	p.stats.lastL1ResizeAtGrowthNum = p.stats.totalGrowthEvents.Load()

drainLoop:
	for {
		select {
		case obj, ok := <-oldCh:
			if !ok {
				break drainLoop
			}

			if len(newCh) != cap(newCh) {
				newCh <- obj
			} else {
				if err := p.pool.Write(obj); err != nil {
					return fmt.Errorf("from channel transfer: %w", err)
				}
			}
		default:
			break drainLoop
		}
	}

	return nil
}

// tryGetFromL1 attempts to retrieve an object from the L1 cache channel using a non-blocking
// select operation. If successful, it updates hit statistics and returns the object.
// If the channel is empty, it returns false to indicate a miss.
func (p *Pool[T]) tryGetFromL1(lock bool) (zero T, found bool) {
	if !lock {
		p.mu.RLock()
		defer p.mu.RUnlock()
	}

	chPtr := p.cacheL1.Load()
	if chPtr == nil {
		return zero, found
	}
	ch := *chPtr

	select {
	case obj, ok := <-ch:
		if !ok {
			return zero, found
		}

		if p.config.verbose {
			fmt.Printf("[GET] L1 hit")
		}
		if p.config.enableStats {
			p.stats.l1HitCount.Add(1)
		}

		p.updateUsageStats()
		found = true
		return obj, found
	default:
		if p.config.verbose {
			log.Println("[GET] L1 miss — falling back to slow path")
		}

		return zero, found
	}
}

// tryFastPathPut attempts to quickly return an object to the L1 cache channel using a non-blocking
// select operation. If successful, it updates hit statistics and returns true.
// If the channel is full, it returns false to indicate a miss.
func (p *Pool[T]) tryFastPathPut(obj T) (ok bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	defer func() {
		if r := recover(); r != nil {
			if p.config.verbose {
				fmt.Printf("[PUT] panic on fast path put — channel closed")
			}
		}
	}()

	if p.closed.Load() {
		return
	}

	chPtr := p.cacheL1.Load()
	if chPtr == nil {
		return
	}

	ch := *chPtr

	select {
	case <-p.ctx.Done():
		return
	case ch <- obj:
		p.stats.FastReturnHit.Add(1)
		if p.config.verbose {
			fmt.Printf("[PUT] Fast return hit")
		}
		ok = true
		return
	default:
		if p.config.verbose {
			fmt.Printf("[PUT] L1 return miss — falling back to slow path")
		}
		return
	}
}

// calculateL1Usage computes the current usage statistics of the L1 cache channel,
// returning the current length, capacity, and usage percentage.
func (p *Pool[T]) calculateL1Usage() (int, uint64, float64) {
	currentCap := p.stats.currentL1Capacity
	chPtr := p.cacheL1.Load()
	if chPtr == nil {
		return 0, 0, 0
	}
	ch := *chPtr
	currentLength := len(ch)

	var currentPercent float64
	if currentCap > 0 {
		currentPercent = float64(currentLength) / float64(currentCap)
	}
	return currentLength, currentCap, currentPercent
}

func (p *Pool[T]) logL1Usage(currentLength int, currentCap uint64, currentPercent float64) {
	if p.config.verbose {
		log.Printf("[REFILL] L1 usage: %d/%d (%.2f%%), refill threshold: %.2f%%", currentLength, currentCap, currentPercent*100, p.config.fastPath.refillPercent*100)
	}
}

// calculateFillTarget determines how many items need to be added to the L1 cache
// to reach the target fill level based on the configured fill aggressiveness.
// Returns 0 if no items are needed or if the target is already met.
func (p *Pool[T]) calculateFillTarget(currentCap uint64) int {
	if currentCap <= 0 {
		return 0
	}

	targetFill := int(float64(currentCap) * p.config.fastPath.fillAggressiveness)

	chPtr := p.cacheL1.Load()
	if chPtr == nil {
		return 0
	}
	ch := *chPtr

	currentLength := len(ch)

	itemsNeeded := targetFill - currentLength

	if itemsNeeded < 0 {
		return 0
	}

	return itemsNeeded
}

// shouldShrinkFastPath determines if the L1 cache should be shrunk based on
// the number of shrink events since the last resize operation.
func (p *Pool[T]) shouldShrinkFastPath() bool {
	sinceLast := p.stats.totalShrinkEvents - p.stats.lastResizeAtShrinkNum
	trigger := uint64(p.config.fastPath.shrinkEventsTrigger)

	return sinceLast >= trigger
}

// adjustFastPathShrinkTarget calculates the new target capacity for the L1 cache
// when shrinking, ensuring it doesn't go below the minimum capacity or current in-use count.
func (p *Pool[T]) adjustFastPathShrinkTarget(currentCap uint64) int {
	cfg := p.config.fastPath.shrink
	newCap := int(float64(currentCap) * (1.0 - cfg.shrinkPercent))
	inUse := int(p.stats.objectsInUse.Load())

	if newCap < cfg.minCapacity {
		if p.config.verbose {
			log.Printf("[SHRINK | FAST PATH] Adjusting to min capacity: %d", cfg.minCapacity)
		}
		return cfg.minCapacity
	}

	if inUse > newCap {
		if p.config.verbose {
			log.Printf("[SHRINK | FAST PATH] Adjusting to in-use objects: %d", inUse)
		}
		return inUse
	}

	return newCap
}

// shrinkFastPath performs the actual shrinking of the L1 cache channel to the specified
// new capacity, copying available objects to the new channel and updating statistics.
func (p *Pool[T]) shrinkFastPath(newCapacity, inUse int) {
	availableObjsToCopy := newCapacity - inUse
	if availableObjsToCopy <= 0 {
		if p.config.verbose {
			log.Printf("[SHRINK] Skipped — no room for available objects after shrink (in-use: %d, requested: %d)", inUse, newCapacity)
		}
		return
	}

	chPtr := p.cacheL1.Load()
	if chPtr == nil {
		return
	}
	ch := *chPtr

	copyCount := min(availableObjsToCopy, len(ch))
	newL1 := make(chan T, newCapacity)

	for range copyCount {
		select {
		case obj, ok := <-ch:
			if !ok {
				return
			}
			newL1 <- obj
		default:
			if p.config.verbose {
				log.Println("[SHRINK] - cacheL1 is empty, or newL1 is full")
			}

		}

	}

	close(ch)
	p.cacheL1.Store(&newL1)
	p.stats.lastResizeAtShrinkNum = p.stats.totalShrinkEvents
	p.stats.currentL1Capacity = uint64(newCapacity)
}

func (p *Pool[T]) logFastPathShrink(currentCap uint64, newCap int, inUse int) {
	if p.config.verbose {
		log.Println("[SHRINK | FAST PATH] ----------------------------------------")
		log.Printf("[SHRINK | FAST PATH] Starting shrink execution")
		log.Printf("[SHRINK | FAST PATH] Current L1 capacity     : %d", currentCap)
		log.Printf("[SHRINK | FAST PATH] Requested new capacity  : %d", newCap)
		log.Printf("[SHRINK | FAST PATH] Minimum allowed         : %d", p.config.fastPath.shrink.minCapacity)
		log.Printf("[SHRINK | FAST PATH] Currently in use        : %d", inUse)
		chPtr := p.cacheL1.Load()
		if chPtr == nil {
			log.Printf("[SHRINK | FAST PATH] Channel length (cached) : nil")
		} else {
			ch := *chPtr
			log.Printf("[SHRINK | FAST PATH] Channel length (cached) : %d", len(ch))
		}
	}
}
