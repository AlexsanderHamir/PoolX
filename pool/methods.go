package pool

import (
	"fmt"
	"log"
	"reflect"
	"sync"
	"time"
)

type AggressivenessLevel int

const (
	defaultAggressiveness             AggressivenessLevel = AggressivenessBalanced
	AggressivenessDisabled            AggressivenessLevel = 0
	AggressivenessConservative        AggressivenessLevel = 1
	AggressivenessBalanced            AggressivenessLevel = 2
	AggressivenessAggressive          AggressivenessLevel = 3
	AggressivenessVeryAggressive      AggressivenessLevel = 4
	AggressivenessExtreme             AggressivenessLevel = 5
	defaultExponentialThresholdFactor                     = 10000000000000.0
	defaultGrowthPercent                                  = 0.00028
	defaultFixedGrowthFactor                              = 10.0
	defaultfillAggressiveness                             = fillAggressivenessExtreme
	fillAggressivenessExtreme                             = 1.0
	defaultRefillPercent                                  = 0.10
	defaultMinCapacity                                    = 128
	defaultPoolCapacity                                   = 128
	defaultL1MinCapacity                                  = defaultPoolCapacity // L1 doesn't go below its initial capacity
	defaultHardLimit                                      = 1_000_000_000
	defaultHardLimitBufferSize                            = 2
	defaultGrowthEventsTrigger                            = 3
	defaultShrinkEventsTrigger                            = 3
	defaultEnableChannelGrowth                            = true
)

const (
	NoRefillNeeded  = "no refill needed"
	RefillSucceeded = "refill succeeded"
	GrowthBlocked   = "growth is blocked"
	GrowthFailed    = "growth failed"
	NoItemsToMove   = "no items to move"
	RingBufferError = "error getting items from ring buffer"
)

func NewPool[T any](config *poolConfig, allocator func() T, cleaner func(T)) (*pool[T], error) {
	if config == nil {
		config = &poolConfig{
			initialCapacity:     defaultPoolCapacity,
			hardLimit:           defaultHardLimit,
			hardLimitBufferSize: defaultHardLimitBufferSize,
			shrink:              defaultShrinkParameters,
			growth:              defaultGrowthParameters,
			fastPath:            defaultFastPath,
		}
	}

	stats := &poolStats{mu: sync.RWMutex{}}
	stats.initialCapacity += config.initialCapacity
	stats.currentCapacity.Store(uint64(config.initialCapacity))
	stats.availableObjects.Store(uint64(config.initialCapacity))
	stats.currentL1Capacity.Store(uint64(config.fastPath.bufferSize))

	// WARNING - be smart about the L1 size, and the hardLimitBufferSize,
	// you will serve more requests faster, but you will also use more memory.
	poolObj := pool[T]{
		cacheL1:         make(chan T, config.fastPath.bufferSize),        // WARNING - careful with how much memory you allocate here.
		hardLimitResume: make(chan struct{}, config.hardLimitBufferSize), // WARNING - careful with how much memory you allocate here.
		allocator:       allocator,
		cleaner:         cleaner,
		mu:              sync.RWMutex{},
		config:          config,
		stats:           stats,
		pool:            New[T](config.initialCapacity),
	}

	// removed the mem allocation caused my pool.mu = &sync.RWMutex{}, but since cond needs a pointer,
	// it causes an allocation.
	poolObj.cond = sync.NewCond(&poolObj.mu)
	obj := poolObj.allocator()
	if reflect.TypeOf(obj).Kind() != reflect.Ptr {
		return nil, fmt.Errorf("allocator must return a pointer, got %T", obj)
	}

	fillTarget := int(float64(poolObj.config.fastPath.bufferSize) * poolObj.config.fastPath.fillAggressiveness)
	fastPathRemaining := fillTarget

	fastPathRemaining = poolObj.setPoolAndBuffer(obj, fastPathRemaining)
	for i := 1; i < config.initialCapacity; i++ {
		obj = poolObj.allocator()
		fastPathRemaining = poolObj.setPoolAndBuffer(obj, fastPathRemaining)
	}

	// Initialize the ring buffer with the initial objects
	poolObj.pool.SetBlocking(true)
	for range config.initialCapacity {
		if err := poolObj.pool.Write(poolObj.allocator()); err != nil {
			return nil, fmt.Errorf("failed to initialize ring buffer: %v", err)
		}
	}

	// go poolObj.shrink() // WARNING - it will mess up the benchmark

	return &poolObj, nil
}

func (p *pool[T]) Get() T {
	if p.isShrinkBlocked {
		if p.config.verbose {
			log.Println("[GET] Shrink is blocked — broadcasting to cond")
		}
		p.cond.Broadcast()
	}

	ableToRefill, refillReason := p.tryRefillIfNeeded()
	if !ableToRefill {
		if p.config.verbose {
			log.Printf("[GET] Unable to refill — reason: %s", refillReason)
		}

		// everything outside GrowthBlocked is an error, return nil (since T is always a pointer, its zero value is nil)
		if refillReason != GrowthBlocked {
			var zero T
			return zero
		}
	}

	if p.config.verbose {
		log.Println("[GET] Attempting to get object from L1 cache")
	}

	select {
	case obj := <-p.cacheL1:
		if p.config.verbose {
			log.Println("[GET] L1 hit")
		}

		p.stats.l1HitCount.Add(1)
		p.updateUsageStats()
		return obj
	default:
		if p.config.verbose {
			log.Println("[GET] L1 miss — falling back to slow path")
		}

		obj := p.slowPath()

		p.stats.l2HitCount.Add(1)
		p.updateUsageStats()
		return obj
	}
}

func (p *pool[T]) Put(obj T) {
	p.releaseObj(obj)

	select {
	case p.cacheL1 <- obj:
		p.stats.FastReturnHit.Add(1)
		if p.config.verbose {
			log.Println("[PUT] Fast return hit")
		}
	default:
		if err := p.pool.Write(obj); err != nil { // WARNING - writing to a full ring buffer will block.
			if p.config.verbose {
				log.Printf("[PUT] Error writing to ring buffer: %v", err)
			}
		}
		p.stats.FastReturnMiss.Add(1)

		if p.config.verbose {
			log.Println("[PUT] Fast return miss")
		}
	}
}

func (p *pool[T]) tryRefillIfNeeded() (bool, string) {
	bufferSize := p.config.fastPath.bufferSize
	currentLength := len(p.cacheL1)
	currentPercent := float64(currentLength) / float64(bufferSize)

	if p.config.verbose {
		log.Printf("[REFILL] L1 usage: %d/%d (%.2f%%), refill threshold: %.2f%%", currentLength, bufferSize, currentPercent*100, p.config.fastPath.refillPercent*100)
	}

	if currentPercent <= p.config.fastPath.refillPercent {
		fillTarget := int(float64(bufferSize)*p.config.fastPath.fillAggressiveness) - len(p.cacheL1)

		if p.config.verbose {
			log.Printf("[REFILL] Triggering refill — fillTarget: %d", fillTarget)
		}

		result := p.refill(fillTarget)
		if !result.Success {
			if p.config.verbose {
				log.Printf("[REFILL] Refill failed: %s", result.Reason)
			}
			return false, result.Reason
		}

		if p.config.verbose {
			log.Printf("[REFILL] Refill succeeded: %s | Moved: %d | Failed: %d | Growth needed: %v | Growth blocked: %v",
				result.Reason, result.ItemsMoved, result.ItemsFailed, result.GrowthNeeded, result.GrowthBlocked)
		}

		return true, RefillSucceeded
	}

	return true, NoRefillNeeded
}

// It blocks if the ring buffer is empty and the ring buffer is in blocking mode.
// We always try to refill the pool before calling the slow path.
func (p *pool[T]) slowPath() T {
	var zero T
	obj, err := p.pool.GetOne()
	if err != nil {
		if p.config.verbose {
			log.Printf("[SLOWPATH] Error getting object from ring buffer: %v", err)
		}
		return zero
	}

	if p.config.verbose {
		log.Printf("[SLOWPATH] Object retrieved from ring buffer | Remaining: %d", p.pool.Length())
	}

	p.stats.lastTimeCalledGet = time.Now()
	return obj
}

func (p *pool[T]) shrink() {
	params := p.config.shrink
	ticker := time.NewTicker(params.checkInterval)
	defer ticker.Stop()

	var (
		idleCount, underutilCount int
		idleOK, utilOK            bool
	)

	for range ticker.C {
		p.mu.Lock()

		if p.stats.consecutiveShrinks.Load() == uint64(params.maxConsecutiveShrinks) {
			if p.config.verbose {
				log.Println("[SHRINK] Max consecutive shrinks reached — waiting for Get() call")
			}
			p.isShrinkBlocked = true
			p.cond.Wait()
			p.isShrinkBlocked = false
		}

		timeSinceLastShrink := time.Since(p.stats.lastShrinkTime)
		if timeSinceLastShrink < params.shrinkCooldown {
			if p.config.verbose {
				log.Printf("[SHRINK] Cooldown active: %v remaining", params.shrinkCooldown-timeSinceLastShrink)
			}
			p.mu.Unlock()
			continue
		}

		p.IndleCheck(&idleCount, &idleOK)
		if p.config.verbose {
			log.Printf("[SHRINK] IdleCheck — idles: %d / min: %d | allowed: %v", idleCount, params.minIdleBeforeShrink, idleOK)
		}

		p.UtilizationCheck(&underutilCount, &utilOK)
		if p.config.verbose {
			log.Printf("[SHRINK] UtilCheck — rounds: %d / required: %d | allowed: %v", underutilCount, params.stableUnderutilizationRounds, utilOK)
		}

		if idleOK || utilOK {
			if p.config.verbose {
				log.Println("[SHRINK] STATS (before shrink)")
				p.PrintPoolStats()
				log.Println("[SHRINK] Shrink conditions met — executing shrink.")
			}
			p.ShrinkExecution() // WARNING - contains heavy operations.
			idleCount, underutilCount = 0, 0
			idleOK, utilOK = false, false
		}

		p.mu.Unlock()
	}
}

// createAndPopulateBuffer creates a new ring buffer with the specified capacity and populates it
// with existing items from the old buffer and new items from the allocator.
func (p *pool[T]) createAndPopulateBuffer(newCapacity uint64) *RingBuffer[T] {
	newRingBuffer := New[T](int(newCapacity))
	newRingBuffer.CopyConfig(p.pool)

	if p.pool == nil {
		return nil
	}

	items, err := p.pool.GetN(int(p.pool.Length()))
	if err != nil && err != ErrIsEmpty {
		if p.config.verbose {
			log.Printf("[GROW] Error getting items from old ring buffer: %v", err)
		}
		return nil
	}

	if len(items) > int(newCapacity) {
		if p.config.verbose {
			log.Printf("[GROW] Length mismatch | Expected: %d | Actual: %d", newCapacity, len(items))
		}
		return nil
	}

	if _, err := newRingBuffer.WriteMany(items); err != nil {
		if p.config.verbose {
			log.Printf("[GROW] Error writing items to new ring buffer: %v", err)
		}
		return nil
	}

	toAdd := int(newCapacity) - newRingBuffer.Length()
	if toAdd <= 0 {
		if p.config.verbose {
			log.Printf("[GROW] No new items to add | New capacity: %d | Ring buffer length: %d", newCapacity, newRingBuffer.Length())
		}
		return newRingBuffer
	}

	for range toAdd {
		if err := newRingBuffer.Write(p.allocator()); err != nil {
			if p.config.verbose {
				log.Printf("[GROW] Error writing new item to ring buffer: %v", err)
			}
			return nil
		}
	}

	return newRingBuffer
}

func (p *pool[T]) grow(now time.Time) bool {
	cfg := p.config.growth
	initialCap := uint64(p.config.initialCapacity)
	currentCap := p.stats.currentCapacity.Load()
	exponentialThreshold := uint64(float64(initialCap) * cfg.exponentialThresholdFactor)
	fixedStep := uint64(float64(initialCap) * cfg.fixedGrowthFactor)

	newCapacity := p.calculateNewPoolCapacity(currentCap, exponentialThreshold, fixedStep, cfg)

	if p.growthWouldExceedHardLimit(newCapacity) {
		if p.config.verbose {
			log.Printf("[GROW] New capacity (%d) exceeds hard limit (%d)", newCapacity, p.config.hardLimit)
		}
		return false
	}

	if p.needsToShrinkToHardLimit(currentCap, newCapacity) {
		newCapacity = uint64(p.config.hardLimit)
		if p.config.verbose {
			log.Printf("[GROW] Capacity (%d) > hard limit (%d); shrinking to fit limit", newCapacity, p.config.hardLimit)
		}
		p.isGrowthBlocked = true
	}

	newRingBuffer := p.createAndPopulateBuffer(newCapacity)
	if newRingBuffer == nil {
		return false
	}

	p.pool = newRingBuffer
	p.stats.currentCapacity.Store(newCapacity)
	p.stats.lastGrowTime = now
	p.stats.l3MissCount.Add(1)
	p.stats.totalGrowthEvents.Add(1)

	p.tryL1ResizeIfTriggered() // WARNING - heavy operation, avoid resizing too often.

	if p.config.verbose {
		log.Printf("[GROW] Final state | New capacity: %d | Ring buffer length: %d", newCapacity, p.pool.Length())
	}
	return true
}
