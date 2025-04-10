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
	defaultExponentialThresholdFactor                     = 4.0
	defaultGrowthPercent                                  = 0.5
	defaultFixedGrowthFactor                              = 1.0
	defaultfillAggressiveness                             = fillAggressivenessExtreme
	fillAggressivenessExtreme                             = 1.0
	defaultRefillPercent                                  = 0.5
	defaultMinCapacity                                    = 8
	defaultL1MinCapacity                                  = defaultPoolCapacity // L1 doesn't go below its initial capacity
	defaultPoolCapacity                                   = 64                  // pool and L1 buffer
	defaultHardLimit                                      = 256
	defaultHardLimitBufferSize                            = 512
	defaultGrowthEventsTrigger                            = 2
	defaultShrinkEventsTrigger                            = 1
	defaultEnableChannelGrowth                            = true
)

func NewPool[T any](config *poolConfig, allocator func() T, cleaner func(T)) (*pool[T], error) {
	if config == nil {
		config = &poolConfig{
			initialCapacity:     defaultPoolCapacity,
			hardLimit:           defaultHardLimit,
			hardLimitBufferSize: defaultHardLimitBufferSize,
			shrink:              defaultPoolShrinkParameters(),
			growth:              defaultPoolGrowthParameters(),
			fastPath:            defaultFastPathParameters(),
		}
	}

	stats := &poolStats{mu: &sync.RWMutex{}}
	stats.initialCapacity += config.initialCapacity
	stats.currentCapacity.Store(uint64(config.initialCapacity))
	stats.availableObjects.Store(uint64(config.initialCapacity))
	stats.currentL1Capacity.Store(uint64(config.fastPath.bufferSize))

	poolObj := pool[T]{
		cacheL1:         make(chan T, config.fastPath.bufferSize),
		hardLimitResume: make(chan struct{}, config.hardLimitBufferSize),
		allocator:       allocator,
		cleaner:         cleaner,
		mu:              &sync.RWMutex{},
		config:          config,
		stats:           stats,
	}

	poolObj.cond = sync.NewCond(poolObj.mu)

	obj := poolObj.allocator()
	if reflect.TypeOf(obj).Kind() != reflect.Ptr {
		return nil, fmt.Errorf("allocator must return a pointer, got %T", obj)
	}

	fillTarget := int(float64(poolObj.config.fastPath.bufferSize) * poolObj.config.fastPath.fillAggressiveness)
	fastPathRemaining := fillTarget

	poolObj.setPoolAndBuffer(obj, &fastPathRemaining)
	for i := 1; i < config.initialCapacity; i++ {
		obj := poolObj.allocator()
		poolObj.setPoolAndBuffer(obj, &fastPathRemaining)
	}

	go poolObj.shrink()

	return &poolObj, nil
}

func (p *pool[T]) Get() T {
	if p.isShrinkBlocked {
		log.Println("[POOL] Shrink logic was blocked — unblocking via cond.Broadcast()")
		p.cond.Broadcast()
	}

	// If the hard limit is reached, we block until an object is returned.
	ableToRefill := p.tryRefillIfNeeded()
	if !ableToRefill {
		log.Printf("[POOL] Hard limit reached — blocking Get()")
		p.stats.blockedGets.Add(1)
		<-p.hardLimitResume
		p.reduceBlockedGets()
	}

	// WARNING - all goroutines wake up at <-p.hardLimitResume, but only one will be able to get the object from the fast path.
	// the rest will go to the slow path, which has a mechanism to allow all goroutines to get an object.

	select {
	case obj := <-p.cacheL1:
		p.stats.l1HitCount.Add(1)
		p.updateUsageStats()
		p.updateDerivedStats()
		return obj
	default:
		p.stats.l2HitCount.Add(1)
		log.Printf("[POOL] Slow path accessed — total L2 hits: %d", p.stats.l2HitCount.Load())
		obj := p.slowPath()
		p.updateUsageStats()
		return obj
	}
}

func (p *pool[T]) Put(obj T) {
	p.cleaner(obj)

	select {
	case p.cacheL1 <- obj:
		p.stats.FastReturnHit.Add(1)
	default:
		p.mu.Lock()
		p.stats.lastTimeCalledPut = time.Now()
		p.pool = append(p.pool, obj)
		p.mu.Unlock()
		p.stats.FastReturnMiss.Add(1)
	}

	p.stats.totalPuts.Add(1)
	p.reduceObjsInUse()
	p.updateAvailableObjs()

	if p.stats.blockedGets.Load() > 0 {
		log.Printf("[POOL] Unblocking Get()")
		p.hardLimitResume <- struct{}{}
	}

	log.Printf("[POOL] Put() called | Object returned to pool | pool size: %d | In-use: %d", len(p.pool), p.stats.objectsInUse.Load())
}

func (p *pool[T]) tryRefillIfNeeded() bool {
	bufferSize := p.config.fastPath.bufferSize
	currentLength := len(p.cacheL1)

	currentPercent := float64(currentLength) / float64(bufferSize)

	// Even though multiple goroutines block right before calling tryRefillIfNeeded
	// this condition will not be true right after a refill, so only one of them will refill.
	if currentPercent <= p.config.fastPath.refillPercent {
		fillTarget := int(float64(bufferSize)*p.config.fastPath.fillAggressiveness) - len(p.cacheL1)
		ableToRefill := p.refill(fillTarget)
		if !ableToRefill {
			return false
		}
	}

	return true
}

// WARNING - how can we avoid an infinit loop ? there may be a case where the L1 buffer is big enough,
// where objects aren't returned to the pool.
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

			p.updateDerivedStats()
			log.Printf("[POOL] Object checked out | New pool size: %d", len(p.pool))
			return obj
		}

		log.Printf("[POOL] Pool is empty — blocking Get(slowpath)")
		p.stats.blockedGets.Add(1)
		p.mu.Unlock()
		<-p.hardLimitResume
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
			log.Println("[SHRINK] Max consecutive shrinks reached — waiting for Get() call")
			p.isShrinkBlocked = true
			p.cond.Wait()
			p.isShrinkBlocked = false
		}

		if time.Since(p.stats.lastShrinkTime) < params.shrinkCooldown {
			remaining := params.shrinkCooldown - time.Since(p.stats.lastShrinkTime)
			log.Printf("[SHRINK] Cooldown active: %v remaining", remaining)
			p.mu.Unlock()
			continue
		}

		p.IndleCheck(&idleCount, &idleOK)
		log.Printf("[SHRINK] IdleCheck — idles: %d / min: %d | allowed: %v", idleCount, params.minIdleBeforeShrink, idleOK)

		p.UtilizationCheck(&underutilCount, &utilOK)
		log.Printf("[SHRINK] UtilCheck — rounds: %d / required: %d | allowed: %v", underutilCount, params.stableUnderutilizationRounds, utilOK)

		if idleOK || utilOK {
			log.Println("[SHRINK] STATS (before shrink)")
			p.PrintPoolStats()
			log.Println("[SHRINK] Shrink conditions met — executing shrink.")
			p.ShrinkExecution()

			idleCount, underutilCount = 0, 0
			idleOK, utilOK = false, false
		}

		p.mu.Unlock()
		p.updateAvailableObjs()
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
		log.Printf("[GROW] New capacity (%d) exceeds hard limit (%d)", newCapacity, p.config.hardLimit)
		p.isGrowthBlocked = true
		return false
	}

	if p.needsToShrinkToHardLimit(currentCap, newCapacity) {
		newCapacity = uint64(p.config.hardLimit)
		log.Printf("[GROW] Capacity (%d) > hard limit (%d); shrinking to fit limit", newCapacity, p.config.hardLimit)
	}

	p.resizePool(currentCap, newCapacity)
	p.stats.lastGrowTime = now
	p.stats.l3MissCount.Add(1)
	p.stats.totalGrowthEvents.Add(1)

	p.tryL1ResizeIfTriggered()

	log.Printf("[GROW] Final state | New capacity: %d | Pool size after grow: %d", newCapacity, len(p.pool))
	return true
}
