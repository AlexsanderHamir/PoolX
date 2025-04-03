package mem

import (
	"fmt"
	"reflect"
	"sync"
)

type MemoryContext struct {
	active bool
	mu     sync.RWMutex

	name     string
	parent   *MemoryContext
	children []*MemoryContext
	pools    map[reflect.Type]*Pool

	contextType   MemoryContextType
	allocStrategy AllocationStrategy
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
		pools:         make(map[reflect.Type]*Pool),
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
func (mc *MemoryContext) CreatePool(objectType reflect.Type, config *PoolConfig, allocator func() any, cleaner func(any)) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	poolObj, err := NewPool(config, allocator, cleaner)
	if err != nil {
		return fmt.Errorf("NewPool failed: %w", err)
	}

	mc.pools[objectType] = poolObj
	return nil
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
