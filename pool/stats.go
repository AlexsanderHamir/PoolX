package pool

import (
	"fmt"
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
	consecutiveShrinks atomic.Int32
	l3MissCount        uint64

	lastTimeCalledGet atomic.Int64
	lastShrinkTime    time.Time
	lastGrowTime      time.Time

	lastL1ResizeAtGrowthNum uint64
	lastResizeAtShrinkNum   uint64
	currentL1Capacity       uint64

	reqPerObj   float64
	utilization float64
}

// PoolStatsSnapshot represents a snapshot of the pool's statistics at a given moment
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
	ConsecutiveShrinks int32

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

// PrintPoolStats prints the current statistics of the pool to stdout.
// This includes information about pool capacity, object usage, hit rates,
// and performance metrics.
func (p *Pool[T]) PrintPoolStats() {
	stats := p.GetPoolStatsSnapshot()
	fmt.Printf("\n=== Pool Statistics ===\n")
	fmt.Printf("Objects in use: %d\n", stats.ObjectsInUse)
	fmt.Printf("Available objects: %d\n", stats.AvailableObjects)
	fmt.Printf("Current capacity: %d\n", stats.CurrentCapacity)
	fmt.Printf("Ring buffer length: %d\n", stats.RingBufferLength)
	fmt.Printf("Peak objects in use: %d\n", stats.PeakInUse)
	fmt.Printf("Total gets: %d\n", stats.TotalGets)
	fmt.Printf("Total growth events: %d\n", stats.TotalGrowthEvents)
	fmt.Printf("Total shrink events: %d\n", stats.TotalShrinkEvents)
	fmt.Printf("Consecutive shrinks: %d\n", stats.ConsecutiveShrinks)
	fmt.Printf("L1 cache capacity: %d\n", stats.CurrentL1Capacity)
	fmt.Printf("L1 cache length: %d\n", stats.L1Length)
	fmt.Printf("L1 hit count: %d\n", stats.L1HitCount)
	fmt.Printf("L2 hit count: %d\n", stats.L2HitCount)
	fmt.Printf("L3 miss count: %d\n", stats.L3MissCount)
	fmt.Printf("Fast return hit: %d\n", stats.FastReturnHit)
	fmt.Printf("Fast return miss: %d\n", stats.FastReturnMiss)
	fmt.Printf("L2 spill rate: %.2f%%\n", stats.L2SpillRate*100)
	fmt.Printf("Request per object: %.2f\n", stats.RequestPerObject)
	fmt.Printf("Utilization: %.2f%%\n", stats.Utilization)
	fmt.Printf("Last get time: %v\n", stats.LastGetTime)
	fmt.Printf("Last shrink time: %v\n", stats.LastShrinkTime)
	fmt.Printf("Last growth time: %v\n", stats.LastGrowTime)
	fmt.Println("===================")
}

// GetPoolStatsSnapshot returns a snapshot of the current pool statistics
func (p *Pool[T]) GetPoolStatsSnapshot() *PoolStatsSnapshot {
	fastReturnHit := p.stats.FastReturnHit.Load()
	fastReturnMiss := p.stats.FastReturnMiss.Load()
	totalReturns := fastReturnHit + fastReturnMiss

	var l2SpillRate float64
	if totalReturns > 0 {
		l2SpillRate = float64(fastReturnMiss) / float64(totalReturns)
	}

	chPtr := p.cacheL1.Load()
	var l1Len int
	if chPtr != nil {
		l1Len = len(*chPtr)
	}

	return &PoolStatsSnapshot{
		// Basic Pool Stats
		ObjectsInUse:       p.stats.objectsInUse.Load(),
		AvailableObjects:   uint64(p.pool.Length(false) + l1Len),
		CurrentCapacity:    p.stats.currentCapacity,
		RingBufferLength:   uint64(p.pool.Length(false)),
		PeakInUse:          p.stats.peakInUse,
		TotalGets:          p.stats.totalGets.Load(),
		TotalGrowthEvents:  p.stats.totalGrowthEvents.Load(),
		TotalShrinkEvents:  p.stats.totalShrinkEvents,
		ConsecutiveShrinks: p.stats.consecutiveShrinks.Load(),

		// Fast Path Resize Stats
		LastResizeAtGrowthNum: p.stats.lastL1ResizeAtGrowthNum,
		CurrentL1Capacity:     p.stats.currentL1Capacity,
		L1Length:              uint64(l1Len),

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
		LastGetTime:    time.Unix(p.stats.lastTimeCalledGet.Load(), 0),
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

	if s.ObjectsInUse != 0 {
		return fmt.Errorf("objects in use (%d) is not 0", s.ObjectsInUse)
	}

	if s.AvailableObjects != s.CurrentCapacity {
		return fmt.Errorf("available objects (%d) does not match current capacity (%d)", s.AvailableObjects, s.CurrentCapacity)
	}

	return nil
}
