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
	defaultExponentialThresholdFactor                     = 1000.0
	defaultGrowthPercent                                  = 1.0
	defaultFixedGrowthFactor                              = 1000.0
	defaultfillAggressiveness                             = fillAggressivenessExtreme
	fillAggressivenessExtreme                             = 1.0
	defaultRefillPercent                                  = 0.10
	defaultMinCapacity                                    = 128
	defaultPoolCapacity                                   = 128
	defaultL1MinCapacity                                  = defaultPoolCapacity // L1 doesn't go below its initial capacity
	defaultHardLimit                                      = 100_000_000
	defaultHardLimitBufferSize                            = 100_000_000
	defaultGrowthEventsTrigger                            = 3
	defaultShrinkEventsTrigger                            = 3
	defaultEnableChannelGrowth                            = true
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

	ableToRefill := p.tryRefillIfNeeded() // WARNING - contains heavy operations (resizePool, tryL1ResizeIfTriggered)
	if !ableToRefill {
		if p.config.verbose {
			log.Println("[GET] Unable to refill — blocking on hardLimitResume")
		}
		p.stats.blockedGets.Add(1)
		<-p.hardLimitResume
		if p.config.verbose {
			log.Println("[GET] Unblocked from hardLimitResume")
		}
		p.reduceBlockedGets()
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
		p.mu.Lock()
		p.pool = append(p.pool, obj)  // WARNING - memory bottleneck.
		p.stats.FastReturnMiss.Add(1) // WARNING - 90% faster if done inside the lock.
		p.mu.Unlock()

		if p.config.verbose {
			log.Println("[PUT] Fast return miss")
		}
	}

	if p.stats.blockedGets.Load() > 0 {
		if p.config.verbose {
			log.Println("[PUT] Unblocking goroutines")
		}
		p.hardLimitResume <- struct{}{}
	}
}

func (p *pool[T]) tryRefillIfNeeded() bool {
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

		ableToRefill := p.refill(fillTarget)

		if !ableToRefill {
			if p.config.verbose {
				log.Println("[REFILL] Refill failed")
			}
			return false
		}

		if p.config.verbose {
			log.Println("[REFILL] Refill succeeded")
		}
	}

	return true
}

func (p *pool[T]) slowPath() T {
	var now time.Time

	for {
		p.mu.Lock()
		lenPool := len(p.pool)

		if lenPool > 0 {
			now = time.Now()
			p.stats.lastTimeCalledGet = now

			obj := p.pool[0]
			p.pool = p.pool[1:]
			p.mu.Unlock()

			if p.config.verbose {
				log.Printf("[SLOWPATH] Object retrieved from pool | Remaining: %d", len(p.pool))
			}

			return obj
		}

		if p.config.verbose {
			log.Printf("[SLOWPATH] Pool empty — blocking on hardLimitResume")
		}

		p.stats.blockedGets.Add(1)
		p.mu.Unlock()

		<-p.hardLimitResume

		if p.config.verbose {
			log.Println("[SLOWPATH] Unblocked from hardLimitResume — retrying")
		}

		p.reduceBlockedGets()
	}
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

// 1. grow is called when the pool is empty.
// 2. grow is called in one place, if you call in another one, you need to check if p.isGrowthBlocked is true before calling.
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
		p.isGrowthBlocked = true
		return false
	}

	if p.needsToShrinkToHardLimit(currentCap, newCapacity) {
		newCapacity = uint64(p.config.hardLimit)
		if p.config.verbose {
			log.Printf("[GROW] Capacity (%d) > hard limit (%d); shrinking to fit limit", newCapacity, p.config.hardLimit)
		}
	}

	p.resizePool(currentCap, newCapacity) // WARNING - heavy operation, avoid resizing too often.
	p.stats.lastGrowTime = now
	p.stats.l3MissCount.Add(1)
	p.stats.totalGrowthEvents.Add(1)

	p.tryL1ResizeIfTriggered() // WARNING - heavy operation, avoid resizing too often.

	if p.config.verbose {
		log.Printf("[GROW] Final state | New capacity: %d | Pool size after grow: %d", newCapacity, len(p.pool))
	}
	return true
}
