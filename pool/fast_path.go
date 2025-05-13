package pool

import (
	"fmt"
)

// calculateNewCapacity determines the new capacity based on current capacity and growth configuration
func (p *Pool[T]) calculateNewCapacity(currentCap int) int {
	cfg := p.config.fastPath.growth
	initialCap := p.config.fastPath.initialSize

	threshold := float64(initialCap) * cfg.thresholdFactor
	floatCurrentCap := float64(currentCap)

	if floatCurrentCap < threshold {
		exponentialStep := floatCurrentCap * cfg.bigGrowthFactor
		newCap := currentCap + int(exponentialStep)
		return newCap
	}

	fixedStep := floatCurrentCap * cfg.controlledGrowthFactor
	newCap := currentCap + int(fixedStep)
	return newCap
}

// drainOldChannel transfers objects from the old channel to the new channel or pool
func (p *Pool[T]) drainOldChannel(oldCh, newCh chan T) error {
	for {
		select {
		case obj, ok := <-oldCh:
			if !ok {
				return nil
			}

			if len(newCh) != cap(newCh) {
				newCh <- obj
			} else {
				if err := p.pool.Write(obj); err != nil {
					return fmt.Errorf("from channel transfer: %w", err)
				}
			}
		default:
			return nil
		}
	}
}

// tryL1ResizeIfTriggered attempts to resize the L1 cache channel if growth events have exceeded
// the configured trigger threshold. It implements an adaptive growth strategy that uses either
// exponential or fixed growth based on the current capacity relative to a threshold.
func (p *Pool[T]) tryL1ResizeIfTriggered() error {
	if !p.config.fastPath.enableChannelGrowth {
		return nil
	}

	trigger := p.config.fastPath.growthEventsTrigger
	sinceLastResize := p.stats.totalGrowthEvents - p.stats.lastL1ResizeAtGrowthNum
	if sinceLastResize < trigger {
		return nil
	}

	currentCap := p.stats.currentL1Capacity
	newCap := p.calculateNewCapacity(currentCap)

	oldChPtr := p.cacheL1
	if oldChPtr == nil {
		return fmt.Errorf("cacheL1 is nil")
	}

	oldCh := *oldChPtr
	newCh := make(chan T, newCap)

	close(oldCh)
	p.cacheL1 = &newCh

	p.stats.currentL1Capacity = newCap
	p.stats.lastL1ResizeAtGrowthNum = p.stats.totalGrowthEvents

	return p.drainOldChannel(oldCh, newCh)
}

// tryGetFromL1 attempts to retrieve an object from the L1 cache channel.
// Returns the object and true if found, otherwise returns zero value and false.
func (p *Pool[T]) tryGetFromL1(locked bool) (zero T, found bool) {
	var chPtr *chan T = p.cacheL1

	if !locked {
		p.mu.RLock()
		chPtr = p.cacheL1
		p.mu.RUnlock()
	}

	select {
	case obj, ok := <-*chPtr:
		if !ok {
			return zero, false
		}
		p.stats.totalGets.Add(1)

		return obj, true
	default:
		return zero, false
	}
}

// tryFastPathPut attempts to quickly return an object to the L1 cache channel using a non-blocking
// select operation. If successful, it updates hit statistics and returns true.
// If the channel is full, it returns false to indicate a miss.
func (p *Pool[T]) tryFastPathPut(obj T) bool {
	defer func() bool {
		if r := recover(); r != nil {
			err := p.slowPathPut(obj)
			return err == nil
		}
		return true
	}()

	p.mu.RLock()
	chPtr := p.cacheL1
	p.mu.RUnlock()

	ch := *chPtr

	select {
	case ch <- obj:
		p.stats.FastReturnHit.Add(1)
		return true
	default:
		return false
	}
}

// calculateL1Usage computes the current usage statistics of the L1 cache channel,
// returning the current length, capacity, and usage percentage.
func (p *Pool[T]) calculateL1Usage() (int, int) {
	currentCap := p.stats.currentL1Capacity
	chPtr := p.cacheL1
	ch := *chPtr
	currentLength := len(ch)

	var currentPercent int
	if currentCap > 0 {
		currentPercent = currentLength / currentCap * 100
	}

	return currentCap, currentPercent
}

// calculateFillTarget determines how many items need to be added to the L1 cache
// to reach the target fill level based on the configured fill aggressiveness.
// Returns 0 if no items are needed or if the target is already met.
func (p *Pool[T]) calculateFillTarget(currentCap int) int {
	if currentCap <= 0 {
		return 0
	}

	targetFill := currentCap * p.config.fastPath.fillAggressiveness / 100

	chPtr := p.cacheL1
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
	trigger := p.config.fastPath.shrinkEventsTrigger

	return sinceLast >= trigger
}

// adjustFastPathShrinkTarget calculates the new target capacity for the L1 cache
// when shrinking, ensuring it doesn't go below the minimum capacity or current in-use count.
func (p *Pool[T]) adjustFastPathShrinkTarget(currentCap int) int {
	cfg := p.config.fastPath.shrink
	newCap := currentCap * (100 - cfg.shrinkPercent) / 100
	inUse := int(p.stats.totalGets.Load() - (p.stats.FastReturnHit.Load() + p.stats.FastReturnMiss.Load()))

	if newCap < cfg.minCapacity {
		return cfg.minCapacity
	}

	if inUse > newCap {
		return inUse
	}

	return newCap
}

// copyObjectsToNewChannel copies objects from the old channel to the new channel
// up to the specified count. Returns the number of objects actually copied.
func (p *Pool[T]) copyObjectsToNewChannel(oldCh, newCh chan T, count int) int {
	copied := 0
	for range count {
		select {
		case obj, ok := <-oldCh:
			if !ok {
				return copied
			}
			newCh <- obj
			copied++
		default:
			return copied
		}
	}
	return copied
}

// createNewL1Channel creates a new L1 channel with the specified capacity
// and copies objects from the old channel if possible.
func (p *Pool[T]) createNewL1Channel(oldCh chan T, newCapacity, inUse int) chan T {
	availableObjsToCopy := newCapacity - inUse
	if availableObjsToCopy <= 0 {
		return nil
	}

	copyCount := min(availableObjsToCopy, len(oldCh))
	newL1 := make(chan T, newCapacity)

	if copyCount > 0 {
		p.copyObjectsToNewChannel(oldCh, newL1, copyCount)
	}

	return newL1
}

// updateShrinkStats updates the pool statistics after a shrink operation
func (p *Pool[T]) updateShrinkStats(newCapacity int) {
	p.stats.lastResizeAtShrinkNum = p.stats.totalShrinkEvents
	p.stats.currentL1Capacity = newCapacity
}

// shrinkFastPath shrinks the L1 cache channel by creating a new channel with the specified capacity
// and copying objects from the old channel if possible.
func (p *Pool[T]) shrinkFastPath(newCapacity, inUse int) {
	defer func() {
		if r := recover(); r != nil {
		}
	}()

	chPtr := p.cacheL1
	ch := *chPtr

	newL1 := p.createNewL1Channel(ch, newCapacity, inUse)
	if newL1 == nil {
		return
	}

	close(ch)
	p.cacheL1 = &newL1
	p.updateShrinkStats(newCapacity)
}
