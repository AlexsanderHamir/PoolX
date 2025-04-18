package contexts

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"maps"

	"github.com/AlexsanderHamir/memory_context/pool"
)

var (
	ErrContextNotFound    = errors.New("context not found")
	ErrInvalidContextType = errors.New("invalid context type")
	ErrContextClosed      = errors.New("context is closed")
)

type MemoryContextType string

type MemoryContext struct {
	closed bool
	mu     sync.RWMutex

	contextType MemoryContextType
	parent      *MemoryContext
	children    []*MemoryContext
	pools       map[reflect.Type]*pool.Pool[any]
	createdAt   time.Time
	lastUsed    time.Time
}

type MemoryContextConfig struct {
	Parent      *MemoryContext
	ContextType MemoryContextType
}

// getter for the children
func (mc *MemoryContext) GetChildren() []*MemoryContext {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.children
}

func NewMemoryContext(config MemoryContextConfig) *MemoryContext {
	now := time.Now()
	mc := &MemoryContext{
		parent:      config.Parent,
		contextType: config.ContextType,
		pools:       make(map[reflect.Type]*pool.Pool[any]),
		createdAt:   now,
		lastUsed:    now,
	}

	return mc
}

// Creates and registers a child context with the same
// context type as the parent.
func (mc *MemoryContext) CreateChild() (*MemoryContext, error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.closed {
		return nil, ErrContextClosed
	}

	memCtx := mc.allocate()
	mc.RegisterChild(memCtx)

	return memCtx, nil
}

func (mc *MemoryContext) allocate() *MemoryContext {
	child := NewMemoryContext(MemoryContextConfig{
		Parent:      mc,
		ContextType: mc.contextType,
	})

	if mc.pools != nil {
		maps.Copy(child.pools, mc.pools)
	}

	return child
}

// Register a child with custom contex type, not the same
// as its parent
func (mc *MemoryContext) RegisterChild(child *MemoryContext) {
	mc.children = append(mc.children, child)
}

// Object Methods
// checking github username update
func (mc *MemoryContext) CreatePool(objectType reflect.Type, config pool.PoolConfig, allocator func() any, cleaner func(any)) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.closed {
		return ErrContextClosed
	}

	poolConfig := pool.ToInternalConfig(config)
	poolObj, err := pool.NewPool(poolConfig, allocator, cleaner, objectType)
	if err != nil {
		return fmt.Errorf("NewPool failed: %w", err)
	}

	mc.pools[objectType] = poolObj
	return nil
}

func (mm *MemoryContext) Acquire(objectType reflect.Type) (any, func(any)) {
	mm.mu.RLock()
	if mm.closed {
		return nil, nil
	}

	poolObj, exists := mm.pools[objectType]
	if !exists {
		return nil, nil
	}
	mm.mu.RUnlock()

	obj := poolObj.Get()

	return obj, poolObj.Cleaner()
}

// get cleaner from pool
func (mm *MemoryContext) GetCleaner(objectType reflect.Type) func(any) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	return mm.pools[objectType].Cleaner()
}

func (mm *MemoryContext) Release(objectType reflect.Type, obj any) bool {
	mm.mu.RLock()

	if mm.closed {
		return false
	}

	poolObj, exists := mm.pools[objectType]
	if !exists {
		return false
	}

	mm.mu.RUnlock()

	if reflect.TypeOf(obj) != objectType {
		return false
	}

	poolObj.Put(obj)
	return true
}

func (mm *MemoryContext) GetPool(objectType reflect.Type) *pool.Pool[any] {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	poolObj, exists := mm.pools[objectType]
	if !exists {
		return nil
	}

	return poolObj
}

func (mc *MemoryContext) Close() error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.closed {
		return fmt.Errorf("context already closed")
	}

	for _, pool := range mc.pools {
		if err := pool.Close(); err != nil {
			return fmt.Errorf("failed to close pool: %w", err)
		}
	}

	for _, child := range mc.children {
		if err := child.Close(); err != nil {
			return fmt.Errorf("failed to close child context: %w", err)
		}
	}

	mc.closed = true
	mc.pools = nil
	mc.children = nil

	return nil
}
