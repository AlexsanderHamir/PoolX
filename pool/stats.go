package pool

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// poolStats contains all the statistics for the pool
type poolStats struct {
	mu                    sync.RWMutex
	initialCapacity       uint64
	currentCapacity       atomic.Uint64
	availableObjects      atomic.Uint64
	objectsInUse          atomic.Uint64
	peakInUse             atomic.Uint64
	totalGets             atomic.Uint64
	totalGrowthEvents     atomic.Uint64
	totalShrinkEvents     atomic.Uint64
	consecutiveShrinks    atomic.Uint64
	l1HitCount            atomic.Uint64
	l2HitCount            atomic.Uint64
	l3MissCount           atomic.Uint64
	FastReturnHit         atomic.Uint64
	FastReturnMiss        atomic.Uint64
	lastTimeCalledGet     time.Time
	lastShrinkTime        time.Time
	lastGrowTime          time.Time
	lastResizeAtGrowthNum atomic.Uint64
	lastResizeAtShrinkNum atomic.Uint64
	currentL1Capacity     atomic.Uint64
	reqPerObj             float64
	utilization           float64
}

func (p *Pool[T]) updateAvailableObjs() {
	p.mu.RLock()
	p.stats.availableObjects.Store(uint64(p.pool.Length() + len(p.cacheL1)))
	p.mu.RUnlock()
}

func (p *Pool[T]) updateUsageStats() {
	currentInUse := p.stats.objectsInUse.Add(1)
	p.stats.totalGets.Add(1)

	for {
		peak := p.stats.peakInUse.Load()
		if currentInUse <= peak {
			break
		}

		if p.stats.peakInUse.CompareAndSwap(peak, currentInUse) {
			break
		}
	}

	for {
		current := p.stats.consecutiveShrinks.Load()
		if current == 0 {
			break
		}
		if p.stats.consecutiveShrinks.CompareAndSwap(current, current-1) {
			break
		}
	}
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
