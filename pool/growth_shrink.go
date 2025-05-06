package pool

import (
	"fmt"
	"log"
	"time"

	"github.com/AlexsanderHamir/ringbuffer"
	"github.com/AlexsanderHamir/ringbuffer/errors"
)

// calculateNewPoolCapacity determines the new capacity for the pool based on the current capacity
// and growth strategy. It uses either exponential growth (below threshold) or fixed-step growth
// (above threshold) to calculate the new size.
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

// ShrinkExecution orchestrates the complete shrinking process for both the main pool and L1 cache.
// It handles capacity calculations, validation, and performs the actual shrinking operations
// while maintaining proper logging and statistics.
func (p *Pool[T]) shrinkExecution() {
	p.logShrinkHeader()

	currentCap := p.stats.currentCapacity
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

	currentCap = p.stats.currentL1Capacity
	newCapacity = p.adjustFastPathShrinkTarget(currentCap)

	p.logFastPathShrink(currentCap, newCapacity, inUse)
	p.shrinkFastPath(newCapacity, inUse)

	if p.config.verbose {
		log.Printf("[SHRINK | FAST PATH] Shrink complete — Final capacity: %d", newCapacity)
		log.Println("[SHRINK | FAST PATH] ----------------------------------------")
	}

	if p.config.verbose {
		log.Printf("[SHRINK] Final state | New capacity: %d | Ring buffer length: %d", newCapacity, p.pool.Length(false))
	}
}

// performShrink executes the actual shrinking of the main pool by creating a new ring buffer
// with the target capacity and copying available objects from the old buffer.
// It preserves in-use objects and updates pool statistics.
func (p *Pool[T]) performShrink(newCapacity, inUse int, currentCap uint64) {
	if !p.canShrink(newCapacity, inUse) {
		return
	}

	newRingBuffer := p.createShrinkBuffer(newCapacity)
	itemsToKeep := p.calculateItemsToKeep(newCapacity, inUse)

	if err := p.migrateItems(newRingBuffer, itemsToKeep); err != nil {
		if p.config.verbose {
			log.Printf("[SHRINK] Failed to migrate items: %v", err)
		}
		return
	}

	p.finalizeShrink(newRingBuffer, newCapacity, currentCap, itemsToKeep, inUse)
}

// canShrink checks if the pool can be shrunk based on the new capacity and in-use objects
func (p *Pool[T]) canShrink(newCapacity, inUse int) bool {
	availableToKeep := newCapacity - inUse
	if availableToKeep < 0 {
		if p.config.verbose {
			log.Printf("[SHRINK] Skipped — no room for available objects after shrink (in-use: %d, requested: %d)", inUse, newCapacity)
		}
		return false
	}
	return true
}

// createShrinkBuffer creates a new ring buffer with the specified capacity
func (p *Pool[T]) createShrinkBuffer(newCapacity int) *ringbuffer.RingBuffer[T] {
	newRingBuffer := ringbuffer.New[T](newCapacity)
	newRingBuffer.CopyConfig(p.pool)
	return newRingBuffer
}

// calculateItemsToKeep determines how many items can be kept during the shrink operation
func (p *Pool[T]) calculateItemsToKeep(newCapacity, inUse int) int {
	availableToKeep := newCapacity - inUse
	return min(availableToKeep, p.pool.Length(false))
}

// migrateItems moves items from the old buffer to the new buffer
func (p *Pool[T]) migrateItems(newRingBuffer *ringbuffer.RingBuffer[T], itemsToKeep int) error {
	if itemsToKeep <= 0 {
		return nil
	}

	part1, part2, err := p.pool.GetNView(itemsToKeep)
	if err != nil && err != errors.ErrIsEmpty {
		if p.config.verbose {
			log.Printf("[SHRINK] Error getting items from old ring buffer: %v", err)
		}
		return err
	}

	if err := p.writeItemsToNewBuffer(newRingBuffer, part1, part2); err != nil {
		return err
	}

	return nil
}

// writeItemsToNewBuffer writes items to the new ring buffer
func (p *Pool[T]) writeItemsToNewBuffer(newRingBuffer *ringbuffer.RingBuffer[T], part1, part2 []T) error {
	if _, err := newRingBuffer.WriteMany(part1); err != nil {
		if p.config.verbose {
			log.Printf("[SHRINK] Error writing items to new ring buffer: %v", err)
		}
		return err
	}

	if _, err := newRingBuffer.WriteMany(part2); err != nil {
		if p.config.verbose {
			log.Printf("[SHRINK] Error writing items to new ring buffer: %v", err)
		}
		return err
	}

	return nil
}

// finalizeShrink updates the pool with the new buffer and updates statistics
func (p *Pool[T]) finalizeShrink(newRingBuffer *ringbuffer.RingBuffer[T], newCapacity int, currentCap uint64, itemsToKeep, inUse int) {
	p.pool = newRingBuffer
	p.stats.currentCapacity = uint64(newCapacity)
	p.stats.totalShrinkEvents++
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

// shouldShrinkMainPool determines if the main pool should be shrunk based on various conditions:
// - Current capacity vs minimum capacity
// - New capacity vs current capacity
// - Available objects vs in-use objects
// Returns false if any condition prevents shrinking.
func (p *Pool[T]) shouldShrinkMainPool(currentCap uint64, newCap int, inUse int) bool {
	minCap := p.config.shrink.minCapacity

	if p.config.verbose {
		log.Printf("[SHRINK] Current capacity       : %d", currentCap)
		log.Printf("[SHRINK] Requested new capacity : %d", newCap)
		log.Printf("[SHRINK] Minimum allowed        : %d", minCap)
		log.Printf("[SHRINK] Currently in use       : %d", inUse)
		log.Printf("[SHRINK] Ring buffer length     : %d", p.pool.Length(false))
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

	chPtr := p.cacheL1.Load()
	if chPtr == nil {
		return false
	}
	ch := *chPtr
	l1Available := len(ch)
	totalAvailable := p.pool.Length(false) + l1Available
	if totalAvailable == 0 {
		if p.config.verbose {
			log.Printf("[SHRINK] Skipped — all %d objects are currently in use, no shrink possible", inUse)
		}
		return false
	}

	return true
}

// adjustMainShrinkTarget adjusts the target capacity for shrinking to ensure it respects
// minimum capacity limits and in-use object counts. It also handles growth blocking
// based on hard limits.
func (p *Pool[T]) adjustMainShrinkTarget(newCap, inUse int) int {
	minCap := p.config.shrink.minCapacity
	adjustedCap := newCap

	// Ensure we don't go below minimum capacity
	if adjustedCap < minCap {
		adjustedCap = minCap
	}

	// Ensure we have enough capacity for in-use objects
	if adjustedCap <= inUse {
		adjustedCap = inUse
	}

	// Unblock growth if we're below hard limit
	if adjustedCap < p.config.hardLimit && p.isGrowthBlocked.Load() {
		p.isGrowthBlocked.Store(false)
	}

	if p.config.verbose {
		if adjustedCap != newCap {
			log.Printf("[SHRINK] Capacity adjusted from %d to %d (min: %d, in-use: %d, hard-limit: %d)",
				newCap, adjustedCap, minCap, inUse, p.config.hardLimit)
		}
	}

	return adjustedCap
}

// createAndPopulateBuffer creates a new ring buffer with the specified capacity and
// populates it with objects from the old buffer. It handles the complete migration
// process including validation and error handling.
func (p *Pool[T]) createAndPopulateBuffer(newCapacity uint64) (*ringbuffer.RingBuffer[T], error) {
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

	p.pool.Close()

	if err := p.fillRemainingCapacity(newRingBuffer, newCapacity); err != nil {
		return nil, fmt.Errorf("failed to fill remaining capacity: %w", err)
	}

	return newRingBuffer, nil
}

func (p *Pool[T]) createNewBuffer(newCapacity uint64) *ringbuffer.RingBuffer[T] {
	newRingBuffer := ringbuffer.New[T](int(newCapacity))
	if p.pool == nil {
		return nil
	}
	newRingBuffer.CopyConfig(p.pool)
	return newRingBuffer
}

func (p *Pool[T]) getItemsFromOldBuffer() (part1, part2 []T, err error) {
	part1, part2, err = p.pool.GetAllView()
	if err != nil && err != errors.ErrIsEmpty {
		if p.config.verbose {
			log.Printf("[GROW] Error getting items from old ring buffer: %v", err)
		}
		return nil, nil, err
	}
	return part1, part2, nil
}

func (p *Pool[T]) validateAndWriteItems(newRingBuffer *ringbuffer.RingBuffer[T], part1, part2 []T, newCapacity uint64) error {
	if len(part1)+len(part2) > int(newCapacity) {
		if p.config.verbose {
			log.Printf("[GROW] Length mismatch | Expected: %d | Actual: %d", newCapacity, len(part1)+len(part2))
		}
		return errors.ErrInvalidLength
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

func (p *Pool[T]) fillRemainingCapacity(newRingBuffer *ringbuffer.RingBuffer[T], newCapacity uint64) error {
	currentCapacity := p.stats.currentCapacity

	toAdd := newCapacity - currentCapacity
	if toAdd <= 0 {
		if p.config.verbose {
			log.Printf("[GROW] No new items to add | New capacity: %d | Current capacity: %d", newCapacity, currentCapacity)
		}
		return nil
	}

	for range toAdd {
		obj := p.allocator()

		if err := newRingBuffer.Write(obj); err != nil {
			if p.config.verbose {
				log.Printf("[GROW] Error writing new item to ring buffer: %v", err)
			}
			return err
		}
	}

	return nil
}

// calculateGrowthParameters computes all necessary parameters for pool growth including
// current capacity, objects in use, exponential threshold, and fixed growth step.
// These parameters are used to determine the growth strategy and new capacity.
func (p *Pool[T]) calculateGrowthParameters() (uint64, uint64, uint64, uint64) {
	cfg := p.config.growth
	currentCap := p.stats.currentCapacity
	objectsInUse := p.stats.objectsInUse.Load()
	exponentialThreshold := uint64(float64(currentCap) * cfg.exponentialThresholdFactor)
	fixedStep := uint64(float64(currentCap) * cfg.fixedGrowthFactor)
	return currentCap, objectsInUse, exponentialThreshold, fixedStep
}

// updatePoolCapacity handles the core capacity update logic, including hard limit checks
// and the creation/population of the new buffer. It's the main entry point for
// capacity changes in the pool.
func (p *Pool[T]) updatePoolCapacity(newCapacity uint64) error {
	if p.needsToShrinkToHardLimit(newCapacity) {
		newCapacity = uint64(p.config.hardLimit)
		if p.config.verbose {
			log.Printf("[GROW] Capacity (%d) > hard limit (%d); shrinking to fit limit", newCapacity, p.config.hardLimit)
		}
		p.isGrowthBlocked.Store(true)
	}

	if newCapacity == uint64(p.config.hardLimit) {
		p.isGrowthBlocked.Store(true)
	}

	newRingBuffer, err := p.createAndPopulateBuffer(newCapacity)
	if err != nil {
		if p.config.verbose {
			log.Printf("[GROW] Failed to create and populate buffer: %v", err)
		}
		return err
	}

	p.pool = newRingBuffer
	p.stats.currentCapacity = newCapacity
	return nil
}

// updateGrowthStats updates all statistics related to pool growth, including
// timing information and hit/miss counters.
func (p *Pool[T]) updateGrowthStats(now time.Time) {
	p.stats.lastGrowTime = now
	p.stats.l3MissCount++
	p.reduceL1Hit()
}

// logGrowthState logs the final state of the pool after growth
func (p *Pool[T]) logGrowthState(newCapacity, objectsInUse uint64) {
	if p.config.verbose {
		chPtr := p.cacheL1.Load()
		if chPtr == nil {
			log.Printf("[GROW] Final state | New capacity: %d | Ring buffer length: %d | L1 length: nil | Objects in use: %d",
				newCapacity, p.pool.Length(false), objectsInUse)
		} else {
			ch := *chPtr
			log.Printf("[GROW] Final state | New capacity: %d | Ring buffer length: %d | L1 length: %d | Objects in use: %d",
				newCapacity, p.pool.Length(false), len(ch), objectsInUse)
		}
	}
}
