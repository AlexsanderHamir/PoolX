package mem

import (
	"reflect"
	"sync"
	"sync/atomic"
	"unsafe"
)

type MemoryContext struct {
	active bool
	mu     sync.RWMutex

	name     string
	parent   *MemoryContext
	children []*MemoryContext
	pools    map[reflect.Type]Pool

	stats *MemContextStats

	contextType   MemoryContextType
	allocStrategy AllocationStrategy
}

type MemContextStats struct {
	totalAllocated uint64
	totalFreed     uint64
	peakUsage      uint64
	currentUsage   uint64
}

type MemoryContextConfig struct {
	Name            string
	Parent          *MemoryContext
	ContextType     MemoryContextType
	AllocationStrat AllocationStrategy
}

func NewMemoryContext(config MemoryContextConfig) *MemoryContext {
	mc := &MemoryContext{
		name:          config.Name,
		parent:        config.Parent,
		contextType:   config.ContextType,
		allocStrategy: config.AllocationStrat,
		pools:         make(map[reflect.Type]Pool),
		stats:         &MemContextStats{},
	}

	return mc
}

// Creates and registers a child context with the same
// context type as the parent.
func (mc *MemoryContext) CreateChild(name string) {
	memCtx := mc.allocate(name)
	mc.RegisterChild(memCtx)
}

func (mc *MemoryContext) allocate(name string) *MemoryContext {
	return NewMemoryContext(MemoryContextConfig{
		Name:        name,
		Parent:      mc,
		ContextType: mc.contextType,
	})
}

// Register a child with custom contex type, not the same
// as its parent
func (mc *MemoryContext) RegisterChild(child *MemoryContext) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.children = append(mc.children, child)
}

// Object Methods
// checking github username update
func (mc *MemoryContext) CreatePool(objectType reflect.Type, allocator func() any, cleaner func(any), capacity int) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	poolObj := Pool{
		pool:      make([]any, 0, capacity),
		capacity:  capacity,
		mu:        &sync.Mutex{},
		allocator: allocator,
		cleaner:   cleaner,
	}

	for range capacity {
		obj := allocator()
		size := unsafe.Sizeof(obj)

		atomic.AddUint64(&mc.stats.totalAllocated, uint64(size))
		atomic.AddUint64(&mc.stats.currentUsage, uint64(size))

		for {
			current := atomic.LoadUint64(&mc.stats.currentUsage)
			peak := atomic.LoadUint64(&mc.stats.peakUsage)
			if current <= peak {
				break
			}
			if atomic.CompareAndSwapUint64(&mc.stats.peakUsage, peak, current) {
				break
			}
		}

		poolObj.pool = append(poolObj.pool, obj)
	}

	mc.pools[objectType] = poolObj
}

func (mm *MemoryContext) Acquire(objectType reflect.Type) any {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	poolObj, exists := mm.pools[objectType]
	if !exists {
		return nil
	}

	obj := poolObj.get()
	mm.pools[objectType] = poolObj

	size := unsafe.Sizeof(obj)
	atomic.AddUint64(&mm.stats.totalAllocated, uint64(size))
	current := atomic.AddUint64(&mm.stats.currentUsage, uint64(size))

	for {
		peak := atomic.LoadUint64(&mm.stats.peakUsage)
		if current <= peak {
			break
		}
		if atomic.CompareAndSwapUint64(&mm.stats.peakUsage, peak, current) {
			break
		}
	}

	return obj
}

func (mm *MemoryContext) Release(objectType reflect.Type, obj any) bool {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	poolObj, exists := mm.pools[objectType]
	if !exists {
		return false
	}

	poolObj.put(obj)
	mm.pools[objectType] = poolObj

	size := unsafe.Sizeof(obj)
	atomic.AddUint64(&mm.stats.totalFreed, uint64(size))
	atomic.AddUint64(&mm.stats.currentUsage, -uint64(size))

	return true
}

func (mm *MemoryContext) GetPool(objectType reflect.Type) *Pool {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	poolObj, exists := mm.pools[objectType]
	if !exists {
		return nil
	}

	return &poolObj
}

// ### Stats
func MemorySnap(rootCtx *MemoryContext) *MemContextStats {
	var s MemContextStats

	rootCtx.mu.RLock()
	defer rootCtx.mu.RUnlock()

	collectStats(rootCtx.stats, &s)

	for _, child := range rootCtx.children {
		childStats(child, &s)
	}

	return &s
}

func childStats(ctx *MemoryContext, destination *MemContextStats) {
	if ctx == nil {
		return
	}

	ctx.mu.RLock()

	collectStats(ctx.stats, destination)

	children := make([]*MemoryContext, len(ctx.children))
	copy(children, ctx.children)

	ctx.mu.RUnlock()

	for _, child := range children {
		childStats(child, destination)
	}
}

func collectStats(source, destination *MemContextStats) {
	destination.currentUsage += source.currentUsage
	destination.totalAllocated += source.totalAllocated
	destination.totalFreed += source.totalFreed
	destination.peakUsage += source.peakUsage
}
