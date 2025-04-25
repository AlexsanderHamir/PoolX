package pool

import (
	"log"
)

func (p *Pool[T]) tryL1ResizeIfTriggered() {
	trigger := uint64(p.config.fastPath.growthEventsTrigger)
	if !p.config.fastPath.enableChannelGrowth {
		return
	}

	sinceLastResize := p.stats.totalGrowthEvents.Load() - p.stats.lastL1ResizeAtGrowthNum
	if sinceLastResize < trigger {
		return
	}

	cfg := p.config.fastPath.growth
	currentCap := p.stats.currentL1Capacity
	threshold := uint64(float64(currentCap) * cfg.exponentialThresholdFactor)
	step := uint64(float64(currentCap) * cfg.fixedGrowthFactor)

	var newCap uint64
	if currentCap < threshold {
		newCap = currentCap + maxUint64(uint64(float64(currentCap)*cfg.growthPercent), 1)
	} else {
		newCap = currentCap + step
	}

	// WARNING: There may be objects in use, we're counting on the PUT to not drop objects.
	newL1 := make(chan T, newCap)
	for {
		select {
		case obj := <-p.cacheL1:
			newL1 <- obj
		default:
			goto done
		}
	}

done:
	p.cacheL1 = newL1
	p.stats.currentL1Capacity = newCap
	p.stats.lastL1ResizeAtGrowthNum = p.stats.totalGrowthEvents.Load()
}

func (p *Pool[T]) tryGetFromL1() (T, bool) {
	select {
	case obj := <-p.cacheL1:
		if p.config.verbose {
			log.Println("[GET] L1 hit")
		}
		if p.config.enableStats {
			p.stats.l1HitCount.Add(1)
		}
		p.updateUsageStats()
		return obj, true
	default:
		if p.config.verbose {
			log.Println("[GET] L1 miss — falling back to slow path")
		}
		var zero T
		return zero, false
	}
}

func (p *Pool[T]) tryFastPathPut(obj T) bool {
	select {
	case p.cacheL1 <- obj:
		if p.config.enableStats {
			p.stats.FastReturnHit.Add(1)
		}
		p.logPut("Fast return hit")
		return true
	default:
		if p.config.verbose {
			p.logPut("L1 return miss — falling back to slow path")
		}
		return false
	}
}

func (p *Pool[T]) calculateL1Usage() (int, int, float64) {
	p.mu.RLock()
	currentCap := p.stats.currentL1Capacity
	currentLength := len(p.cacheL1)
	p.mu.RUnlock()

	var currentPercent float64
	if currentCap > 0 {
		currentPercent = float64(currentLength) / float64(currentCap)
	}
	return currentLength, int(currentCap), currentPercent
}

func (p *Pool[T]) logL1Usage(currentLength, currentCap int, currentPercent float64) {
	if p.config.verbose {
		log.Printf("[REFILL] L1 usage: %d/%d (%.2f%%), refill threshold: %.2f%%", currentLength, currentCap, currentPercent*100, p.config.fastPath.refillPercent*100)
	}
}

func (p *Pool[T]) calculateFillTarget(currentCap int) int {
	if currentCap <= 0 {
		return 0
	}

	targetFill := int(float64(currentCap) * p.config.fastPath.fillAggressiveness)

	p.mu.RLock()
	currentLength := len(p.cacheL1)
	p.mu.RUnlock()

	itemsNeeded := targetFill - currentLength

	if itemsNeeded < 0 {
		return 0
	}

	return itemsNeeded
}

func (p *Pool[T]) shouldShrinkFastPath() bool {
	sinceLast := p.stats.totalShrinkEvents - p.stats.lastResizeAtShrinkNum
	trigger := uint64(p.config.fastPath.shrinkEventsTrigger)

	return sinceLast >= trigger
}

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

func (p *Pool[T]) shrinkFastPath(newCapacity, inUse int) {
	availableObjsToCopy := newCapacity - inUse
	if availableObjsToCopy <= 0 {
		if p.config.verbose {
			log.Printf("[SHRINK] Skipped — no room for available objects after shrink (in-use: %d, requested: %d)", inUse, newCapacity)
		}
		return
	}

	copyCount := min(availableObjsToCopy, len(p.cacheL1))
	newL1 := make(chan T, newCapacity)

	for range copyCount {
		select {
		case obj := <-p.cacheL1:
			newL1 <- obj
		default:
			if p.config.verbose {
				log.Println("[SHRINK] - cacheL1 is empty, or newL1 is full")
			}
		}
	}

	p.cacheL1 = newL1
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
		log.Printf("[SHRINK | FAST PATH] Channel length (cached) : %d", len(p.cacheL1))
	}
}
