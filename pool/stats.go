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
	totalGrowthEvents int

	FastReturnHit  atomic.Uint64
	FastReturnMiss atomic.Uint64

	totalShrinkEvents  int
	consecutiveShrinks int

	lastShrinkTime time.Time

	lastL1ResizeAtGrowthNum int
	lastResizeAtShrinkNum   int
	currentL1Capacity       uint64
}

// PoolStatsSnapshot represents a snapshot of the pool's statistics at a given moment
type PoolStatsSnapshot struct {
	// Basic Pool Stats
	InitialCapacity   uint64
	CurrentCapacity   uint64
	ObjectsInUse      uint64
	TotalGets         uint64
	TotalGrowthEvents int

	// Fast Return Stats
	FastReturnHit  uint64
	FastReturnMiss uint64

	// Shrink Stats
	TotalShrinkEvents  int
	ConsecutiveShrinks int
	LastShrinkTime     time.Time

	// L1 Cache Stats
	LastL1ResizeAtGrowthNum int
	LastResizeAtShrinkNum   int
	CurrentL1Capacity       uint64

	// Derived Stats (computed from other fields)
	AvailableObjects uint64
	RingBufferLength uint64
	L1Length         uint64
	L2SpillRate      float64
	Utilization      float64
}

func (p *Pool[T]) updateUsageStats() {
	p.stats.objectsInUse.Add(1)
	p.stats.totalGets.Add(1)
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
	fmt.Printf("Total gets: %d\n", stats.TotalGets)
	fmt.Printf("Total growth events: %d\n", stats.TotalGrowthEvents)
	fmt.Printf("Total shrink events: %d\n", stats.TotalShrinkEvents)
	fmt.Printf("Consecutive shrinks: %d\n", stats.ConsecutiveShrinks)
	fmt.Printf("L1 cache capacity: %d\n", stats.CurrentL1Capacity)
	fmt.Printf("L1 cache length: %d\n", stats.L1Length)
	fmt.Printf("Fast return hit: %d\n", stats.FastReturnHit)
	fmt.Printf("Fast return miss: %d\n", stats.FastReturnMiss)
	fmt.Printf("L2 spill rate: %.2f%%\n", stats.L2SpillRate*100)
	fmt.Printf("Utilization: %.2f%%\n", stats.Utilization)
	fmt.Printf("Last shrink time: %v\n", stats.LastShrinkTime)
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

	chPtr := p.cacheL1
	var l1Len int
	if chPtr != nil {
		l1Len = len(*chPtr)
	}

	return &PoolStatsSnapshot{
		// Basic Pool Stats
		InitialCapacity:   p.stats.initialCapacity,
		CurrentCapacity:   p.stats.currentCapacity,
		ObjectsInUse:      p.stats.objectsInUse.Load(),
		TotalGets:         p.stats.totalGets.Load(),
		TotalGrowthEvents: p.stats.totalGrowthEvents,

		// Fast Return Stats
		FastReturnHit:  fastReturnHit,
		FastReturnMiss: fastReturnMiss,

		// Shrink Stats
		TotalShrinkEvents:  p.stats.totalShrinkEvents,
		ConsecutiveShrinks: p.stats.consecutiveShrinks,
		LastShrinkTime:     p.stats.lastShrinkTime,

		// L1 Cache Stats
		LastL1ResizeAtGrowthNum: p.stats.lastL1ResizeAtGrowthNum,
		LastResizeAtShrinkNum:   p.stats.lastResizeAtShrinkNum,
		CurrentL1Capacity:       p.stats.currentL1Capacity,

		// Derived Stats (computed from other fields)
		AvailableObjects: p.stats.currentCapacity - p.stats.objectsInUse.Load(),
		RingBufferLength: uint64(l1Len),
		L1Length:         uint64(l1Len),
		L2SpillRate:      l2SpillRate,
		Utilization:      float64(p.stats.objectsInUse.Load()) / float64(p.stats.currentCapacity),
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
