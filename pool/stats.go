package pool

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// poolStats contains all the statistics for the pool
type poolStats struct {
	mu sync.RWMutex

	initialCapacity       uint64        // essential && static
	currentCapacity       atomic.Uint64 // essential && dynamic
	availableObjects      atomic.Uint64 // essential && dynamic
	objectsInUse          atomic.Uint64 // essential && dynamic
	peakInUse             atomic.Uint64 // no essential && is dynamic
	totalGets             atomic.Uint64 // essential && dynamic
	totalGrowthEvents     atomic.Uint64 // essential && dynamic
	totalShrinkEvents     atomic.Uint64 // essential && dynamic
	consecutiveShrinks    atomic.Uint64 // essential && dynamic
	l1HitCount            atomic.Uint64 // no essential && is dynamic
	l2HitCount            atomic.Uint64 // no essential && is dynamic
	l3MissCount           atomic.Uint64 // no essential && is dynamic
	FastReturnHit         atomic.Uint64 // no essential && is dynamic
	FastReturnMiss        atomic.Uint64 // no essential && is dynamic
	lastTimeCalledGet     time.Time     // essential && is dynamic
	lastShrinkTime        time.Time     // essential && is dynamic
	lastGrowTime          time.Time     // no essential && is dynamic
	lastResizeAtGrowthNum atomic.Uint64 // essential && is dynamic
	lastResizeAtShrinkNum atomic.Uint64 // essential && is dynamic
	currentL1Capacity     atomic.Uint64 // essential && is dynamic
	reqPerObj             float64       // no essential && is dynamic
	utilization           float64       // essential && is dynamic
}

// PoolStats represents a snapshot of the pool's statistics at a given moment
type PoolStatsSnapshot struct {
	// Basic Pool Stats
	ObjectsInUse       uint64
	AvailableObjects   uint64
	CurrentCapacity    uint64
	RingBufferLength   uint64
	PeakInUse          uint64
	TotalGets          uint64
	TotalGrowthEvents  uint64
	TotalShrinkEvents  uint64
	ConsecutiveShrinks uint64

	// Fast Path Resize Stats
	LastResizeAtGrowthNum uint64
	CurrentL1Capacity     uint64
	L1Length              uint64

	// Fast Get Stats
	L1HitCount  uint64
	L2HitCount  uint64
	L3MissCount uint64

	// Fast Return Stats
	FastReturnHit  uint64
	FastReturnMiss uint64
	L2SpillRate    float64

	// Usage Stats
	RequestPerObject float64
	Utilization      float64

	// Time Stats
	LastGetTime    time.Time
	LastShrinkTime time.Time
	LastGrowTime   time.Time
}

func (p *Pool[T]) updateAvailableObjs() {
	ringBufferLength := p.pool.Length()
	l1Length := len(p.cacheL1)

	p.stats.availableObjects.Store(uint64(ringBufferLength + l1Length))
}

func (p *Pool[T]) updateUsageStats() {
	currentInUse := p.stats.objectsInUse.Add(1)
	p.stats.totalGets.Add(1)

	p.mu.Lock()
	p.updatePeakInUse(currentInUse)
	p.updateConsecutiveShrinks()
	p.mu.Unlock()
}

func (p *Pool[T]) updateDerivedStats() {
	totalGets := p.stats.totalGets.Load()
	objectsInUse := p.stats.objectsInUse.Load()
	availableObjects := p.stats.availableObjects.Load()

	currentCapacity := p.stats.currentCapacity.Load()

	initialCapacity := p.stats.initialCapacity
	totalCreated := currentCapacity - uint64(initialCapacity)

	p.stats.mu.Lock()
	if totalCreated > 0 {
		p.stats.reqPerObj = float64(totalGets) / float64(totalCreated)
	}

	totalObjects := objectsInUse + availableObjects
	if totalObjects > 0 {
		p.stats.utilization = (float64(objectsInUse) / float64(totalObjects)) * 100
	}

	p.stats.mu.Unlock()
}

func (p *Pool[T]) PrintPoolStats() {
	p.updateAvailableObjs()

	fmt.Println("========== Pool Stats ==========")
	fmt.Printf("Objects In Use					 : %d\n", p.stats.objectsInUse.Load())
	fmt.Printf("Total Available Objects			 : %d\n", p.stats.availableObjects.Load())
	fmt.Printf("Current Ring Buffer Capacity	 : %d\n", p.stats.currentCapacity.Load())
	fmt.Printf("Ring Buffer Length   			 : %d\n", p.pool.Length())
	fmt.Printf("Peak In Use          			 : %d\n", p.stats.peakInUse.Load())
	fmt.Printf("Total Gets           			 : %d\n", p.stats.totalGets.Load())
	fmt.Printf("Total Growth Events  			 : %d\n", p.stats.totalGrowthEvents.Load())
	fmt.Printf("Total Shrink Events  			 : %d\n", p.stats.totalShrinkEvents.Load())
	fmt.Printf("Consecutive Shrinks  			 : %d\n", p.stats.consecutiveShrinks.Load())

	fmt.Println()
	fmt.Println("---------- Fast Path Resize Stats ----------")
	fmt.Printf("Last Resize At Growth Num: %d\n", p.stats.lastResizeAtGrowthNum.Load())
	fmt.Printf("Current L1 Capacity      : %d\n", p.stats.currentL1Capacity.Load())
	fmt.Printf("L1 Length                : %d\n", len(p.cacheL1))
	fmt.Println("---------------------------------------")
	fmt.Println()

	fmt.Println()
	fmt.Println("---------- Fast Get Stats ----------")
	fmt.Printf("Fast Path (L1) Hits  : %d\n", p.stats.l1HitCount.Load())
	fmt.Printf("Slow Path (L2) Hits  : %d\n", p.stats.l2HitCount.Load())
	fmt.Printf("Allocator Misses (L3): %d\n", p.stats.l3MissCount.Load())
	fmt.Println("---------------------------------------")
	fmt.Println()

	fastReturnHit := p.stats.FastReturnHit.Load()
	fastReturnMiss := p.stats.FastReturnMiss.Load()
	totalReturns := fastReturnHit + fastReturnMiss

	var l2SpillRate float64
	if totalReturns > 0 {
		l2SpillRate = float64(fastReturnMiss) / float64(totalReturns)
	}

	fmt.Println("---------- Fast Return Stats ----------")
	fmt.Printf("Fast Return Hit   : %d\n", fastReturnHit)
	fmt.Printf("Fast Return Miss  : %d\n", fastReturnMiss)
	fmt.Printf("L2 Spill Rate     : %.2f%%\n", l2SpillRate*100)
	fmt.Println("---------------------------------------")
	fmt.Println()

	p.updateDerivedStats()
	p.stats.mu.RLock()
	fmt.Println("---------- Usage Stats ----------")
	fmt.Println()
	fmt.Printf("Request Per Object   : %.2f\n", p.stats.reqPerObj)
	fmt.Printf("Utilization %%       : %.2f%%\n", p.stats.utilization)
	fmt.Println("---------------------------------------")
	p.stats.mu.RUnlock()

	fmt.Println("---------- Time Stats ----------")
	fmt.Printf("Last Get Time        : %s\n", p.stats.lastTimeCalledGet.Format(time.RFC3339))
	fmt.Printf("Last Shrink Time     : %s\n", p.stats.lastShrinkTime.Format(time.RFC3339))
	fmt.Printf("Last Grow Time       : %s\n", p.stats.lastGrowTime.Format(time.RFC3339))
	fmt.Println("=================================")
}

// GetPoolStatsSnapshot returns a snapshot of the current pool statistics
func (p *Pool[T]) GetPoolStatsSnapshot() *PoolStatsSnapshot {
	p.stats.mu.RLock()
	defer p.stats.mu.RUnlock()

	p.updateAvailableObjs()

	fastReturnHit := p.stats.FastReturnHit.Load()
	fastReturnMiss := p.stats.FastReturnMiss.Load()
	totalReturns := fastReturnHit + fastReturnMiss

	var l2SpillRate float64
	if totalReturns > 0 {
		l2SpillRate = float64(fastReturnMiss) / float64(totalReturns)
	}

	// p.updateDerivedStats()
	return &PoolStatsSnapshot{
		// Basic Pool Stats
		ObjectsInUse:       p.stats.objectsInUse.Load(),
		AvailableObjects:   p.stats.availableObjects.Load(),
		CurrentCapacity:    p.stats.currentCapacity.Load(),
		RingBufferLength:   uint64(p.pool.Length()),
		PeakInUse:          p.stats.peakInUse.Load(),
		TotalGets:          p.stats.totalGets.Load(),
		TotalGrowthEvents:  p.stats.totalGrowthEvents.Load(),
		TotalShrinkEvents:  p.stats.totalShrinkEvents.Load(),
		ConsecutiveShrinks: p.stats.consecutiveShrinks.Load(),

		// Fast Path Resize Stats
		LastResizeAtGrowthNum: p.stats.lastResizeAtGrowthNum.Load(),
		CurrentL1Capacity:     p.stats.currentL1Capacity.Load(),
		L1Length:              uint64(len(p.cacheL1)),

		// Fast Get Stats
		L1HitCount:  p.stats.l1HitCount.Load(),
		L2HitCount:  p.stats.l2HitCount.Load(),
		L3MissCount: p.stats.l3MissCount.Load(),

		// Fast Return Stats
		FastReturnHit:  fastReturnHit,
		FastReturnMiss: fastReturnMiss,
		L2SpillRate:    l2SpillRate,

		// Usage Stats
		RequestPerObject: p.stats.reqPerObj,
		Utilization:      p.stats.utilization,

		// Time Stats
		LastGetTime:    p.stats.lastTimeCalledGet,
		LastShrinkTime: p.stats.lastShrinkTime,
		LastGrowTime:   p.stats.lastGrowTime,
	}
}

func (s *PoolStatsSnapshot) Validate(reqNum int) error {
	if s.TotalGets != uint64(reqNum) {
		return fmt.Errorf("total gets (%d) does not match request number (%d)", s.TotalGets, reqNum)
	}

	if s.AvailableObjects != s.CurrentCapacity {
		return fmt.Errorf("available objects (%d) does not match current capacity (%d)", s.AvailableObjects, s.CurrentCapacity)
	}

	return nil
}
