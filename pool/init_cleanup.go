package pool

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/AlexsanderHamir/ringbuffer"
	config "github.com/AlexsanderHamir/ringbuffer/config"
)

// createDefaultConfig creates a new PoolConfig with default values for all parameters.
// It initializes the configuration with default values for pool capacity, hard limits,
// statistics, shrink/growth parameters, fast path settings, and ring buffer configuration.
// Returns a fully configured PoolConfig instance.
func createDefaultConfig[T any]() *PoolConfig[T] {
	pgb := &poolConfigBuilder[T]{
		config: &PoolConfig[T]{
			initialCapacity: defaultPoolCapacity,
			hardLimit:       defaultHardLimit,
			shrink:          defaultShrinkParameters,
			growth:          defaultGrowthParameters,
			fastPath:        defaultFastPath,
			ringBufferConfig: &config.RingBufferConfig[T]{
				Block:    Block,
				RTimeout: RTimeout,
				WTimeout: WTimeout,
			},
			allocationStrategy: defaultAllocationStrategy,
		},
	}

	pgb.config.shrink.ApplyDefaults(getShrinkDefaultsMap())

	copiedShrink := *pgb.config.shrink
	pgb.config.fastPath.shrink = &copiedShrink
	pgb.config.fastPath.shrink.minCapacity = defaultL1MinCapacity

	return pgb.config
}

// initializePoolStats creates and initializes the pool statistics structure with
// the provided configuration values. It sets up initial capacity values for both
// the main pool and L1 cache.
func initializePoolStats[T any](config *PoolConfig[T]) *poolStats {
	stats := &poolStats{mu: sync.RWMutex{}}
	stats.initialCapacity = config.initialCapacity
	stats.currentCapacity = config.initialCapacity
	stats.currentL1Capacity = config.fastPath.initialSize
	return stats
}

// validateType validates the provided allocator, cleaner, and cloneTemplate functions.
// This is a critical validation as the pool requires pointer types for proper object management.
// Returns an error if the allocator returns a non-pointer type.
func validate[T any](allocator func() T, cleaner func(T), cloner func(T) T) error {
	var zero T
	if reflect.TypeOf(zero).Kind() != reflect.Ptr {
		return fmt.Errorf("type T must be a pointer type, got %T", zero)
	}

	obj := allocator()
	if reflect.TypeOf(obj).Kind() != reflect.Ptr {
		return fmt.Errorf("type returned by allocator must be a pointer type, got %T", obj)
	}

	if cleaner == nil {
		return fmt.Errorf("cleaner function is nil")
	}

	if reflect.TypeOf(cloner(obj)).Kind() != reflect.Ptr {
		return fmt.Errorf("type returned by cloner must be a pointer type, got %T", cloner(obj))
	}

	return nil
}

// initializePoolObject creates and initializes a new Pool instance with the provided
// configuration, allocator, cleaner, and ring buffer. It sets up the L1 cache channel
// and initializes all necessary synchronization primitives.
// Returns a fully initialized Pool instance or an error if initialization fails.
func initializePoolObject[T any](config *PoolConfig[T], allocator func() T, cleaner func(T), cloneTemplate func(T) T, stats *poolStats, ringBuffer *ringbuffer.RingBuffer[T]) (*Pool[T], error) {
	ch := make(chan T, config.fastPath.initialSize)

	template := allocator()
	poolObj := &Pool[T]{
		cacheL1:         &ch,
		refillSemaphore: make(chan struct{}, 1),
		allocator:       allocator,
		cleaner:         cleaner,
		cloneTemplate:   cloneTemplate,
		config:          config,
		stats:           stats,
		pool:            ringBuffer,
		template:        template,
		refillCond:      sync.NewCond(&sync.Mutex{}),
	}

	poolObj.shrinkCond = sync.NewCond(&poolObj.mu)
	return poolObj, nil
}

// populateL1OrBuffer initializes the pool by creating and distributing objects between
// the L1 cache and main buffer. It uses the configured fill aggressiveness to determine
// how many objects should go to the L1 cache versus the main buffer.
// Returns an error if object allocation or distribution fails.
func (p *Pool[T]) populateL1OrBuffer(allocAmount int) error {
	fillTarget := p.config.fastPath.initialSize * p.config.fastPath.fillAggressiveness / 100
	fastPathRemaining := fillTarget

	for range allocAmount {
		var obj T
		if p.cloneTemplate != nil {
			obj = p.cloneTemplate(p.template)
		} else {
			obj = p.allocator()
		}
		p.stats.objectsCreated++

		var err error
		fastPathRemaining, err = p.setPoolAndBuffer(obj, fastPathRemaining)
		if err != nil {
			return fmt.Errorf("failed to set pool and buffer: %w", err)
		}
	}
	return nil
}

// cleanupCacheL1 performs cleanup of the L1 cache by:
// 1. Draining all objects from the cache
// 2. Calling the cleaner function on each object
// 3. Zeroing out the objects
// 4. Closing the channel
// This method is called during pool shutdown to ensure proper resource cleanup.
func (p *Pool[T]) cleanupCacheL1() {
	var zero T
	chPtr := p.cacheL1
	ch := *chPtr

	for {
		select {
		case obj, ok := <-ch:
			if !ok {
				return
			}
			p.cleaner(obj)
			obj = zero
			_ = obj
		default:
			close(ch)
			return
		}
	}
}
