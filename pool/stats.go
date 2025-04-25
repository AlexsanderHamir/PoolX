package pool

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// poolStats contains all the statistics (essential and non-essential) for the pool
type poolStats struct {
	mu sync.RWMutex

	initialCapacity uint64
	currentCapacity uint64

	// Fast-path accessed fields — must be atomic
	objectsInUse      atomic.Uint64
	totalGets         atomic.Uint64
	totalGrowthEvents atomic.Uint64
	l1HitCount        atomic.Uint64
	l2HitCount        atomic.Uint64
	FastReturnHit     atomic.Uint64
	FastReturnMiss    atomic.Uint64

	peakInUse          uint64
	totalShrinkEvents  uint64
	consecutiveShrinks int
	l3MissCount        uint64

	lastTimeCalledGet time.Time
	lastShrinkTime    time.Time
	lastGrowTime      time.Time

	lastL1ResizeAtGrowthNum uint64
	lastResizeAtShrinkNum   uint64
	currentL1Capacity       uint64

	reqPerObj   float64
	utilization float64
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
	ConsecutiveShrinks int

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

func (p *Pool[T]) updateUsageStats() {
	currentInUse := p.stats.objectsInUse.Add(1)
	p.stats.totalGets.Add(1)

	if p.config.enableStats {
		p.stats.mu.Lock()
		p.stats.peakInUse = maxUint64(p.stats.peakInUse, currentInUse)
		p.stats.mu.Unlock()
	}
}

func (p *Pool[T]) updateDerivedStats() {
	if !p.config.enableStats {
		log.Println("Stats are disabled")
		return
	}

	totalGets := p.stats.totalGets.Load()
	objectsInUse := p.stats.objectsInUse.Load()

	currentCapacity := p.stats.currentCapacity

	initialCapacity := p.stats.initialCapacity
	totalCreated := currentCapacity - uint64(initialCapacity)

	p.stats.mu.Lock()
	availableObjects := p.pool.Length() + len(p.cacheL1)
	if totalCreated > 0 {
		p.stats.reqPerObj = float64(totalGets) / float64(totalCreated)
	}

	totalObjects := objectsInUse + uint64(availableObjects)
	if totalObjects > 0 {
		p.stats.utilization = (float64(objectsInUse) / float64(totalObjects)) * 100
	}
	p.stats.mu.Unlock()
}

func (p *Pool[T]) PrintPoolStats() {
	fmt.Println("========== Pool Stats ==========")
	fmt.Printf("Objects In Use					 : %d\n", p.stats.objectsInUse.Load())
	fmt.Printf("Current Ring Buffer Capacity	 : %d\n", p.stats.currentCapacity)
	fmt.Printf("Ring Buffer Length   			 : %d\n", p.pool.Length())
	fmt.Printf("Peak In Use          			 : %d\n", p.stats.peakInUse)
	fmt.Printf("Total Gets           			 : %d\n", p.stats.totalGets.Load())
	fmt.Printf("Total Growth Events  			 : %d\n", p.stats.totalGrowthEvents.Load())
	fmt.Printf("Total Shrink Events  			 : %d\n", p.stats.totalShrinkEvents)
	fmt.Printf("Consecutive Shrinks  			 : %d\n", p.stats.consecutiveShrinks)

	fmt.Println()
	fmt.Println("---------- Fast Path Resize Stats ----------")
	fmt.Printf("Last Resize At Growth Num: %d\n", p.stats.lastL1ResizeAtGrowthNum)
	fmt.Printf("Current L1 Capacity      : %d\n", p.stats.currentL1Capacity)
	fmt.Printf("L1 Length                : %d\n", len(p.cacheL1))
	fmt.Println("---------------------------------------")
	fmt.Println()

	fmt.Println()
	fmt.Println("---------- Fast Get Stats ----------")
	fmt.Printf("Fast Path (L1) Hits  : %d\n", p.stats.l1HitCount.Load())
	fmt.Printf("Slow Path (L2) Hits  : %d\n", p.stats.l2HitCount.Load())
	fmt.Printf("Allocator Misses (L3): %d\n", p.stats.l3MissCount)
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
	fmt.Println("---------- Usage Stats ----------")
	fmt.Println()
	fmt.Printf("Request Per Object   : %.2f\n", p.stats.reqPerObj)
	fmt.Printf("Utilization %%       : %.2f%%\n", p.stats.utilization)
	fmt.Println("---------------------------------------")

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
		AvailableObjects:   uint64(p.pool.Length() + len(p.cacheL1)),
		CurrentCapacity:    p.stats.currentCapacity,
		RingBufferLength:   uint64(p.pool.Length()),
		PeakInUse:          p.stats.peakInUse,
		TotalGets:          p.stats.totalGets.Load(),
		TotalGrowthEvents:  p.stats.totalGrowthEvents.Load(),
		TotalShrinkEvents:  p.stats.totalShrinkEvents,
		ConsecutiveShrinks: p.stats.consecutiveShrinks,

		// Fast Path Resize Stats
		LastResizeAtGrowthNum: p.stats.lastL1ResizeAtGrowthNum,
		CurrentL1Capacity:     p.stats.currentL1Capacity,
		L1Length:              uint64(len(p.cacheL1)),

		// Fast Get Stats
		L1HitCount:  p.stats.l1HitCount.Load(),
		L2HitCount:  p.stats.l2HitCount.Load(),
		L3MissCount: p.stats.l3MissCount,

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
	totalReturns := s.FastReturnHit + s.FastReturnMiss
	if totalReturns != s.TotalGets {
		return fmt.Errorf("total returns (%d) does not match total gets (%d)", totalReturns, s.TotalGets)
	}

	if s.TotalGets != uint64(reqNum) {
		return fmt.Errorf("total gets (%d) does not match request number (%d)", s.TotalGets, reqNum)
	}

	if s.AvailableObjects != s.CurrentCapacity {
		return fmt.Errorf("available objects (%d) does not match current capacity (%d)", s.AvailableObjects, s.CurrentCapacity)
	}

	return nil
}
