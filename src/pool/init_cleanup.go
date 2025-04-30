package pool

import (
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
)

func createDefaultConfig() *PoolConfig {
	pgb := &poolConfigBuilder{
		config: &PoolConfig{
			initialCapacity:  defaultPoolCapacity,
			hardLimit:        defaultHardLimit,
			enableStats:      defaultEnableStats,
			shrink:           defaultShrinkParameters,
			growth:           defaultGrowthParameters,
			fastPath:         defaultFastPath,
			ringBufferConfig: defaultRingBufferConfig,
		},
	}

	copiedShrink := *defaultShrinkParameters
	pgb.config.fastPath.shrink.ApplyDefaults(getShrinkDefaultsMap())
	pgb.config.fastPath.shrink.minCapacity = defaultL1MinCapacity
	pgb.config.fastPath.shrink = &copiedShrink

	pgb.config.shrink.ApplyDefaults(getShrinkDefaultsMap())

	return pgb.config
}

func initializePoolStats(config *PoolConfig) *poolStats {
	stats := &poolStats{mu: sync.RWMutex{}}
	stats.initialCapacity = uint64(config.initialCapacity)
	stats.currentCapacity = (uint64(config.initialCapacity))
	stats.currentL1Capacity = uint64(config.fastPath.initialSize)
	return stats
}

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

func initializePoolObject[T any](config *PoolConfig, allocator func() T, cleaner func(T), stats *poolStats, ringBuffer *RingBuffer[T], poolType reflect.Type) (*Pool[T], error) {
	ch := make(chan T, config.fastPath.initialSize)

	poolObj := &Pool[T]{
		cacheL1:   atomic.Pointer[chan T]{},
		allocator: allocator,
		cleaner:   cleaner,
		mu:        sync.RWMutex{},
		config:    config,
		stats:     stats,
		pool:      ringBuffer,
		poolType:  poolType,
	}

	poolObj.cacheL1.Store(&ch)

	poolObj.shrinkCond = sync.NewCond(&poolObj.mu)
	return poolObj, nil
}

func populateL1OrBuffer[T any](poolObj *Pool[T]) error {
	fillTarget := int(float64(poolObj.config.fastPath.initialSize) * poolObj.config.fastPath.fillAggressiveness)
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

func (p *Pool[T]) cleanupCacheL1() {
	var zero T
	chPtr := p.cacheL1.Load()
	if chPtr == nil {
		return
	}
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
