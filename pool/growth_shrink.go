package pool

import (
	"fmt"
	"log"
	"time"
)

func (p *Pool[T]) calculateNewPoolCapacity(currentCap, threshold, fixedStep uint64, cfg *growthParameters) uint64 {
	if currentCap < threshold {
		growth := maxUint64(uint64(float64(currentCap)*cfg.growthPercent), 1)
		newCap := currentCap + growth
		if p.config.verbose {
			log.Printf("[GROW] Strategy: exponential | Threshold: %d | Current: %d | Growth: %d | New capacity: %d", threshold, currentCap, growth, newCap)
		}
		return newCap
	}

	newCap := currentCap + fixedStep
	if p.config.verbose {
		log.Printf("[GROW] Strategy: fixed-step | Threshold: %d | Current: %d | Step: %d | New capacity: %d",
			threshold, currentCap, fixedStep, newCap)
	}
	return newCap
}

func (p *Pool[T]) needsToShrinkToHardLimit(newCapacity uint64) bool {
	return newCapacity > uint64(p.config.hardLimit)
}

func (p *Pool[T]) ShrinkExecution() {
	p.logShrinkHeader()

	currentCap := p.stats.currentCapacity.Load()
	inUse := int(p.stats.objectsInUse.Load())
	newCapacity := int(float64(currentCap) * (1.0 - p.config.shrink.shrinkPercent))

	if !p.shouldShrinkMainPool(currentCap, newCapacity, inUse) {
		return
	}

	newCapacity = p.adjustMainShrinkTarget(newCapacity, inUse)
	p.performShrink(newCapacity, inUse, currentCap)
	if p.config.verbose {
		log.Printf("[SHRINK] Shrink complete — Final capacity: %d", newCapacity)
		log.Println("[SHRINK] ----------------------------------------")
	}

	// Fast Path (L1)
	if !p.config.fastPath.enableChannelGrowth || !p.shouldShrinkFastPath() {
		return
	}

	currentCap = p.stats.currentL1Capacity.Load()
	newCapacity = p.adjustFastPathShrinkTarget(currentCap)

	p.logFastPathShrink(currentCap, newCapacity, inUse)
	p.shrinkFastPath(newCapacity, inUse)

	if p.config.verbose {
		log.Printf("[SHRINK | FAST PATH] Shrink complete — Final capacity: %d", newCapacity)
		log.Println("[SHRINK | FAST PATH] ----------------------------------------")
	}

	if p.config.verbose {
		log.Printf("[SHRINK] Final state | New capacity: %d | Ring buffer length: %d", newCapacity, p.pool.Length())
	}
}

func (p *Pool[T]) performShrink(newCapacity, inUse int, currentCap uint64) {
	availableToKeep := newCapacity - inUse
	if availableToKeep < 0 {
		if p.config.verbose {
			log.Printf("[SHRINK] Skipped — no room for available objects after shrink (in-use: %d, requested: %d)", inUse, newCapacity)
		}
		return
	}

	newRingBuffer := NewRingBuffer[T](int(newCapacity))
	newRingBuffer.CopyConfig(p.pool)

	itemsToKeep := min(availableToKeep, p.pool.Length())
	if itemsToKeep > 0 {
		part1, part2, err := p.pool.GetNView(itemsToKeep)
		if err != nil && err != errIsEmpty {
			if p.config.verbose {
				log.Printf("[SHRINK] Error getting items from old ring buffer: %v", err)
			}
			return
		}

		if _, err := newRingBuffer.WriteMany(part1); err != nil {
			if p.config.verbose {
				log.Printf("[SHRINK] Error writing items to new ring buffer: %v", err)
			}
			return
		}

		if _, err := newRingBuffer.WriteMany(part2); err != nil {
			if p.config.verbose {
				log.Printf("[SHRINK] Error writing items to new ring buffer: %v", err)
			}
			return
		}
	}

	p.pool = newRingBuffer

	p.stats.currentCapacity.Store(uint64(newCapacity))
	p.stats.totalShrinkEvents.Add(1)
	p.stats.lastShrinkTime = time.Now()
	p.stats.consecutiveShrinks.Add(1)

	if p.config.verbose {
		log.Printf("[SHRINK] Shrinking pool → From: %d → To: %d | Preserved: %d | In-use: %d",
			currentCap, newCapacity, itemsToKeep, inUse)
	}
}

func (p *Pool[T]) logShrinkHeader() {
	if p.config.verbose {
		log.Println("[SHRINK] ----------------------------------------")
		log.Println("[SHRINK] Starting shrink execution")
	}
}

func (p *Pool[T]) shouldShrinkMainPool(currentCap uint64, newCap int, inUse int) bool {
	minCap := p.config.shrink.minCapacity

	if p.config.verbose {
		log.Printf("[SHRINK] Current capacity       : %d", currentCap)
		log.Printf("[SHRINK] Requested new capacity : %d", newCap)
		log.Printf("[SHRINK] Minimum allowed        : %d", minCap)
		log.Printf("[SHRINK] Currently in use       : %d", inUse)
		log.Printf("[SHRINK] Ring buffer length     : %d", p.pool.Length())
	}

	switch {
	case newCap == 0:
		if p.config.verbose {
			log.Println("[SHRINK] Skipped — new capacity is zero (invalid)")
		}
		return false
	case currentCap == uint64(minCap):
		if p.config.verbose {
			log.Printf("[SHRINK] Skipped — current capacity (%d) is at or below MinCapacity (%d)", currentCap, minCap)
		}
		return false
	case newCap >= int(currentCap):
		if p.config.verbose {
			log.Printf("[SHRINK] Skipped — new capacity (%d) is not smaller than current (%d)", newCap, currentCap)
		}
		return false
	}

	l1Available := len(p.cacheL1)
	totalAvailable := p.pool.Length() + l1Available
	if totalAvailable == 0 {
		if p.config.verbose {
			log.Printf("[SHRINK] Skipped — all %d objects are currently in use, no shrink possible", inUse)
		}
		return false
	}

	return true
}

func (p *Pool[T]) adjustMainShrinkTarget(newCap, inUse int) int {
	minCap := p.config.shrink.minCapacity

	if newCap < minCap {
		if p.config.verbose {
			log.Printf("[SHRINK] Adjusting to min capacity: %d", minCap)
		}
		newCap = minCap
	}

	if newCap <= inUse {
		if p.config.verbose {
			log.Printf("[SHRINK] Adjusting to match in-use objects: %d", inUse)
		}
		newCap = inUse
	}

	if newCap < p.config.hardLimit && p.isGrowthBlocked {
		if p.config.verbose {
			log.Printf("[SHRINK] Allowing growth, capacity is lower than hard limit: %d", p.config.hardLimit)
		}
		p.isGrowthBlocked = false
	}

	return newCap
}

func (p *Pool[T]) createAndPopulateBuffer(newCapacity uint64) (*RingBuffer[T], error) {
	newRingBuffer := p.createNewBuffer(newCapacity)
	if newRingBuffer == nil {
		return nil, fmt.Errorf("failed to create ring buffer")
	}

	part1, part2, err := p.getItemsFromOldBuffer()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve items from old buffer: %w", err)
	}

	if err := p.validateAndWriteItems(newRingBuffer, part1, part2, newCapacity); err != nil {
		return nil, fmt.Errorf("failed to write items to new buffer: %w", err)
	}

	if err := p.fillRemainingCapacity(newRingBuffer, newCapacity); err != nil {
		return nil, fmt.Errorf("failed to fill remaining capacity: %w", err)
	}

	return newRingBuffer, nil
}

func (p *Pool[T]) createNewBuffer(newCapacity uint64) *RingBuffer[T] {
	newRingBuffer := NewRingBuffer[T](int(newCapacity))
	if p.pool == nil {
		return nil
	}
	newRingBuffer.CopyConfig(p.pool)
	return newRingBuffer
}

func (p *Pool[T]) getItemsFromOldBuffer() (part1, part2 []T, err error) {
	part1, part2, err = p.pool.getAll()
	if err != nil && err != errIsEmpty {
		if p.config.verbose {
			log.Printf("[GROW] Error getting items from old ring buffer: %v", err)
		}
		return nil, nil, err
	}
	return part1, part2, nil
}

func (p *Pool[T]) validateAndWriteItems(newRingBuffer *RingBuffer[T], part1, part2 []T, newCapacity uint64) error {
	if len(part1)+len(part2) > int(newCapacity) {
		if p.config.verbose {
			log.Printf("[GROW] Length mismatch | Expected: %d | Actual: %d", newCapacity, len(part1)+len(part2))
		}
		return errInvalidLength
	}

	if _, err := newRingBuffer.WriteMany(part1); err != nil {
		if p.config.verbose {
			log.Printf("[GROW] Error writing items to new ring buffer: %v", err)
		}
		return err
	}

	if _, err := newRingBuffer.WriteMany(part2); err != nil {
		if p.config.verbose {
			log.Printf("[GROW] Error writing items to new ring buffer: %v", err)
		}
		return err
	}

	return nil
}

func (p *Pool[T]) fillRemainingCapacity(newRingBuffer *RingBuffer[T], newCapacity uint64) error {
	currentCapacity := p.stats.currentCapacity.Load()
	toAdd := newCapacity - currentCapacity
	if toAdd <= 0 {
		if p.config.verbose {
			log.Printf("[GROW] No new items to add | New capacity: %d | Current capacity: %d", newCapacity, currentCapacity)
		}
		return nil
	}

	for range toAdd {
		if err := newRingBuffer.Write(p.allocator()); err != nil {
			if p.config.verbose {
				log.Printf("[GROW] Error writing new item to ring buffer: %v", err)
			}
			return err
		}
	}
	return nil
}

// calculateGrowthParameters computes the necessary parameters for pool growth
func (p *Pool[T]) calculateGrowthParameters() (uint64, uint64, uint64, uint64) {
	cfg := p.config.growth
	currentCap := p.stats.currentCapacity.Load()
	objectsInUse := p.stats.objectsInUse.Load()
	exponentialThreshold := uint64(float64(currentCap) * cfg.exponentialThresholdFactor)
	fixedStep := uint64(float64(currentCap) * cfg.fixedGrowthFactor)
	return currentCap, objectsInUse, exponentialThreshold, fixedStep
}

// updatePoolCapacity handles the core capacity update logic
func (p *Pool[T]) updatePoolCapacity(newCapacity uint64) error {
	if uint64(p.config.hardLimit) == newCapacity {
		p.isGrowthBlocked = true
		return nil
	}

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
		return err
	}

	p.pool = newRingBuffer
	p.stats.currentCapacity.Store(newCapacity)
	return nil
}

// updateGrowthStats updates all statistics related to pool growth
func (p *Pool[T]) updateGrowthStats(now time.Time) {
	p.stats.lastGrowTime = now
	p.stats.l3MissCount.Add(1)
	p.stats.totalGrowthEvents.Add(1)
	p.reduceL1Hit()
}

// logGrowthState logs the final state of the pool after growth
func (p *Pool[T]) logGrowthState(newCapacity, objectsInUse uint64) {
	if p.config.verbose {
		log.Printf("[GROW] Final state | New capacity: %d | Ring buffer length: %d | L1 length: %d | Objects in use: %d",
			newCapacity, p.pool.Length(), len(p.cacheL1), objectsInUse)
	}
}
