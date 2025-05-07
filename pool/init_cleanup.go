package pool

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/AlexsanderHamir/ringbuffer"
)

// createDefaultConfig creates a new PoolConfig with default values for all parameters.
// It initializes the configuration with default values for pool capacity, hard limits,
// statistics, shrink/growth parameters, fast path settings, and ring buffer configuration.
// Returns a fully configured PoolConfig instance.
func createDefaultConfig() *PoolConfig {
	pgb := &poolConfigBuilder{
		config: &PoolConfig{
			initialCapacity:  defaultPoolCapacity,
			hardLimit:        defaultHardLimit,
			shrink:           defaultShrinkParameters,
			growth:           defaultGrowthParameters,
			fastPath:         defaultFastPath,
			ringBufferConfig: defaultRingBufferConfig,
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
func initializePoolStats(config *PoolConfig) *poolStats {
	stats := &poolStats{mu: sync.RWMutex{}}
	stats.initialCapacity = config.initialCapacity
	stats.currentCapacity = config.initialCapacity
	stats.currentL1Capacity = config.fastPath.initialSize
	return stats
}

// validateAllocator verifies that the provided allocator function returns a pointer type.
// This is a critical validation as the pool requires pointer types for proper object management.
// Returns an error if the allocator returns a non-pointer type.
func validateAllocator[T any](allocator func() T) error {
	var zero T
	obj := allocator()
	if reflect.TypeOf(obj).Kind() != reflect.Ptr {
		return fmt.Errorf("allocator must return a pointer, got %T", obj)
	}

	obj = zero
	_ = obj
	return nil
}

// initializePoolObject creates and initializes a new Pool instance with the provided
// configuration, allocator, cleaner, and ring buffer. It sets up the L1 cache channel
// and initializes all necessary synchronization primitives.
// Returns a fully initialized Pool instance or an error if initialization fails.
func initializePoolObject[T any](config *PoolConfig, allocator func() T, cleaner func(T), stats *poolStats, ringBuffer *ringbuffer.RingBuffer[T]) (*Pool[T], error) {
	ch := make(chan T, config.fastPath.initialSize)

	poolObj := &Pool[T]{
		cacheL1:   &ch,
		allocator: allocator,
		cleaner:   cleaner,
		config:    config,
		stats:     stats,
		pool:      ringBuffer,
	}

	poolObj.shrinkCond = sync.NewCond(&poolObj.mu)
	return poolObj, nil
}

// populateL1OrBuffer initializes the pool by creating and distributing objects between
// the L1 cache and main buffer. It uses the configured fill aggressiveness to determine
// how many objects should go to the L1 cache versus the main buffer.
// Returns an error if object allocation or distribution fails.
func populateL1OrBuffer[T any](poolObj *Pool[T]) error {
	fillTarget := poolObj.config.fastPath.initialSize * poolObj.config.fastPath.fillAggressiveness
	fastPathRemaining := fillTarget

	for range poolObj.config.initialCapacity {
		obj := poolObj.allocator()

		var err error
		fastPathRemaining, err = poolObj.setPoolAndBuffer(obj, fastPathRemaining)
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
