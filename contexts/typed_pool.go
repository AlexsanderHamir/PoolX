package contexts

import (
	"memctx/pool"
	"reflect"
)

// TypedPool provides type-safe access to a pool of objects of type T
type TypedPool[T any] struct {
	ctx        *MemoryContext
	objectType reflect.Type
}

// NewTypedPool creates a new type-safe pool wrapper
func NewTypedPool[T any](ctx *MemoryContext) *TypedPool[T] {
	return &TypedPool[T]{
		ctx:        ctx,
		objectType: reflect.TypeOf((*T)(nil)).Elem(),
	}
}

// CreatePool creates a new pool with the given configuration
func (tp *TypedPool[T]) CreatePool(config pool.PoolConfig, allocator func() T, cleaner func(T)) error {
	// Convert the type-safe allocator and cleaner to interface{} versions
	anyAllocator := func() any {
		return allocator()
	}
	anyCleaner := func(obj any) {
		cleaner(obj.(T))
	}
	return tp.ctx.CreatePool(tp.objectType, config, anyAllocator, anyCleaner)
}

// Acquire gets an object from the pool
func (tp *TypedPool[T]) Acquire() T {
	return tp.ctx.Acquire(tp.objectType).(T)
}

// Release returns an object to the pool
func (tp *TypedPool[T]) Release(obj T) bool {
	return tp.ctx.Release(tp.objectType, obj)
}

// GetPool returns the underlying pool
func (tp *TypedPool[T]) GetPool() *pool.Pool[any] {
	return tp.ctx.GetPool(tp.objectType)
}
