package pool

import (
	"fmt"
	"time"

	"github.com/AlexsanderHamir/ringbuffer"
	"github.com/AlexsanderHamir/ringbuffer/errors"
)

// calculateNewPoolCapacity determines the new capacity for the pool based on the current capacity
// and growth strategy. It uses either exponential growth (below threshold) or fixed-step growth
// (above threshold) to calculate the new size.
func (p *Pool[T]) calculateNewPoolCapacity() int {
	cfg := p.config.growth
	currentCap := p.stats.currentCapacity
	initialCap := p.config.initialCapacity
	exponentialThreshold := initialCap * cfg.exponentialThresholdFactor
	fixedStep := initialCap * cfg.fixedGrowthFactor

	if currentCap < exponentialThreshold {
		growth := initialCap * cfg.growthPercent
		newCap := currentCap + growth
		return newCap
	}

	newCap := currentCap + fixedStep
	return newCap
}

func (p *Pool[T]) needsToShrinkToHardLimit(newCapacity int) bool {
	return newCapacity > p.config.hardLimit
}

// ShrinkExecution orchestrates the complete shrinking process for both the main pool and L1 cache.
// It handles capacity calculations, validation, and performs the actual shrinking operations
// while maintaining proper logging and statistics.
func (p *Pool[T]) shrinkExecution() {
	currentCap := p.stats.currentCapacity
	inUse := int(p.stats.totalGets.Load() - (p.stats.FastReturnHit.Load() + p.stats.FastReturnMiss.Load()))
	newCapacity := int(currentCap) * (100 - p.config.shrink.shrinkPercent) / 100

	if !p.shouldShrinkMainPool(currentCap, newCapacity) {
		return
	}

	newCapacity = p.adjustMainShrinkTarget(newCapacity, inUse)
	p.performShrink(newCapacity, inUse)

	if !p.config.fastPath.enableChannelGrowth || !p.shouldShrinkFastPath() {
		return
	}

	currentCap = p.stats.currentL1Capacity
	newCapacity = p.adjustFastPathShrinkTarget(currentCap)

	p.shrinkFastPath(newCapacity, inUse)

}

// performShrink executes the actual shrinking of the main pool by creating a new ring buffer
// with the target capacity and copying available objects from the old buffer.
// It preserves in-use objects and updates pool statistics.
func (p *Pool[T]) performShrink(newCapacity, inUse int) {
	if !p.canShrink(newCapacity, inUse) {
		return
	}

	newRingBuffer := p.createShrinkBuffer(newCapacity)
	itemsToKeep := p.calculateItemsToKeep(newCapacity, inUse)

	if err := p.migrateItems(newRingBuffer, itemsToKeep); err != nil {
		return
	}

	p.finalizeShrink(newRingBuffer, newCapacity)
}

// canShrink checks if the pool can be shrunk based on the new capacity and in-use objects
func (p *Pool[T]) canShrink(newCapacity, inUse int) bool {
	availableToKeep := newCapacity - inUse
	return availableToKeep >= 0
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
		return err
	}

	if _, err := newRingBuffer.WriteMany(part2); err != nil {
		return err
	}

	return nil
}

// finalizeShrink updates the pool with the new buffer and updates statistics
func (p *Pool[T]) finalizeShrink(newRingBuffer *ringbuffer.RingBuffer[T], newCapacity int) {
	p.pool = newRingBuffer
	p.stats.currentCapacity = newCapacity
	p.stats.totalShrinkEvents++
	p.stats.lastShrinkTime = time.Now()
	p.stats.consecutiveShrinks++
}

// shouldShrinkMainPool determines if the main pool should be shrunk based on various conditions:
// - Current capacity vs minimum capacity
// - New capacity vs current capacity
// - Available objects vs in-use objects
// Returns false if any condition prevents shrinking.
func (p *Pool[T]) shouldShrinkMainPool(currentCap int, newCap int) bool {
	minCap := p.config.shrink.minCapacity

	switch {
	case newCap == 0:
		return false
	case currentCap == minCap:
		return false
	case newCap >= currentCap:
		return false
	}

	chPtr := p.cacheL1
	ch := *chPtr
	l1Available := len(ch)
	totalAvailable := p.pool.Length(false) + l1Available

	return totalAvailable != 0
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

	return adjustedCap
}

// createAndPopulateBuffer creates a new ring buffer with the specified capacity and
// populates it with objects from the old buffer. It handles the complete migration
// process including validation and error handling.
func (p *Pool[T]) createAndPopulateBuffer(newCapacity int) (*ringbuffer.RingBuffer[T], error) {
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

func (p *Pool[T]) createNewBuffer(newCapacity int) *ringbuffer.RingBuffer[T] {
	newRingBuffer := ringbuffer.New[T](newCapacity)
	if p.pool == nil {
		return nil
	}
	newRingBuffer.CopyConfig(p.pool)
	return newRingBuffer
}

func (p *Pool[T]) getItemsFromOldBuffer() (part1, part2 []T, err error) {
	part1, part2, err = p.pool.GetAllView()
	if err != nil && err != errors.ErrIsEmpty {
		return nil, nil, err
	}
	return part1, part2, nil
}

func (p *Pool[T]) validateAndWriteItems(newRingBuffer *ringbuffer.RingBuffer[T], part1, part2 []T, newCapacity int) error {
	if len(part1)+len(part2) > newCapacity {
		return errors.ErrInvalidLength
	}

	if _, err := newRingBuffer.WriteMany(part1); err != nil {
		return err
	}

	if _, err := newRingBuffer.WriteMany(part2); err != nil {
		return err
	}

	return nil
}

func (p *Pool[T]) fillRemainingCapacity(newRingBuffer *ringbuffer.RingBuffer[T], newCapacity int) error {
	currentCapacity := p.stats.currentCapacity

	toAdd := newCapacity - currentCapacity
	if toAdd <= 0 {
		return nil
	}

	for range toAdd {
		obj := p.allocator()

		if err := newRingBuffer.Write(obj); err != nil {
			return err
		}
	}

	return nil
}

// calculateGrowthParameters computes all necessary parameters for pool growth including
// current capacity, objects in use, exponential threshold, and fixed growth step.
// These parameters are used to determine the growth strategy and new capacity.
func (p *Pool[T]) calculateGrowthParameters() (int, int, int) {
	cfg := p.config.growth
	currentCap := p.stats.currentCapacity
	initialCap := p.config.initialCapacity
	exponentialThreshold := initialCap * cfg.exponentialThresholdFactor
	fixedStep := initialCap * cfg.fixedGrowthFactor
	return currentCap, exponentialThreshold, fixedStep
}

// updatePoolCapacity handles the core capacity update logic, including hard limit checks
// and the creation/population of the new buffer. It's the main entry point for
// capacity changes in the pool.
func (p *Pool[T]) updatePoolCapacity(newCapacity int) error {
	if p.needsToShrinkToHardLimit(newCapacity) {
		newCapacity = p.config.hardLimit
		p.isGrowthBlocked.Store(true)
	}

	if newCapacity == p.config.hardLimit {
		p.isGrowthBlocked.Store(true)
	}

	newRingBuffer, err := p.createAndPopulateBuffer(newCapacity)
	if err != nil {
		return err
	}

	p.pool = newRingBuffer
	p.stats.currentCapacity = newCapacity
	return nil
}
