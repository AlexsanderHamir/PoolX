package pool

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/AlexsanderHamir/ringbuffer"
)

var (
	errGrowthBlocked    = errors.New("growth is blocked")
	errRingBufferFailed = errors.New("ring buffer failed core operation")
	errNoItemsToMove    = errors.New("no items to move")
	errNilObject        = errors.New("object is nil")
	errNilConfig        = errors.New("config is nil")
)

// NewPool creates a new object pool with the given configuration, allocator, cleaner, and pool type.
// Only pointers can be stored in the pool. The allocator function creates new objects,
// and the cleaner function resets objects before they are reused.
func NewPool[T any](config *PoolConfig[T], allocator func() T, cleaner func(T)) (PoolObj[T], error) {
	if err := validateAllocator(allocator); err != nil {
		return nil, err
	}

	if config == nil {
		config = createDefaultConfig[T]()
	}

	if err := checkConfigForNil(config); err != nil {
		return nil, err
	}

	ringBuffer, err := ringbuffer.NewWithConfig(config.initialCapacity, config.ringBufferConfig)
	if err != nil {
		return nil, err
	}

	stats := initializePoolStats(config)

	poolObj, err := initializePoolObject(config, allocator, cleaner, stats, ringBuffer)
	if err != nil {
		return nil, err
	}

	ringBuffer.WithPreReadBlockHook(poolObj.preReadBlockHook)

	poolObj.ctx, poolObj.cancel = context.WithCancel(context.Background())

	allocationStrategy := poolObj.config.allocationStrategy
	preAllocAmount := poolObj.stats.currentCapacity * allocationStrategy.AllocPercent / 100

	if err := poolObj.populateL1OrBuffer(preAllocAmount); err != nil {
		return nil, err
	}

	go poolObj.shrink()

	return poolObj, nil
}

// Get returns an object from the pool, either from L1 cache or the ring buffer, preferring L1.
func (p *Pool[T]) Get() (zero T, err error) {
	if obj, found := p.tryGetFromL1(false); found {
		return obj, nil
	}

	if obj, found := p.tryRefillAndGetL1(); found {
		return obj, nil
	}

	obj, err := p.SlowPathGet()
	if err != nil {
		return zero, err
	}

	return obj, nil
}

// Put returns an object to the pool. The object will be cleaned using the cleaner function
// before being made available for reuse.
func (p *Pool[T]) Put(obj T) error {
	defer p.waiter.WakeOne()
	p.cleaner(obj)

	if p.tryFastPathPut(obj) {
		p.pool.WakeUpOneReader()
		return nil
	}

	return p.slowPathPut(obj)
}

// Close closes the pool and releases all resources. If there are outstanding objects,
// it will wait for them to be returned before closing.
func (p *Pool[T]) Close() error {
	if p.hasOutstandingObjects() {
		p.closeAsync()
	}

	p.performClosure()
	return nil
}

// shrink is a background goroutine that periodically checks the pool for idle and underutilized objects,
// and shrinks the pool if necessary to free up memory.
func (p *Pool[T]) shrink() {
	params := p.config.shrink
	ticker := time.NewTicker(params.checkInterval)
	defer ticker.Stop()

	var (
		underutilCount int
		utilOK         bool
	)

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.mu.Lock()

			if p.handleMaxConsecutiveShrinks(params.maxConsecutiveShrinks) {
				p.mu.Unlock()
				continue
			}

			if p.handleShrinkCooldown(params.shrinkCooldown) {
				p.mu.Unlock()
				continue
			}

			if p.isUnderUtilized() {
				underutilCount++
				if underutilCount >= params.stableUnderutilizationRounds {
					utilOK = true
				}
			} else {
				if underutilCount > 0 {
					underutilCount--
				}
			}

			if utilOK {
				p.shrinkExecution()
				underutilCount = 0
				utilOK = false
			}

			p.mu.Unlock()
		}
	}
}

// grow is called when the demand for objects exceeds the current capacity, if enabled.
// It increases the pool's capacity according to the growth configuration.
func (p *Pool[T]) grow() error {
	defer func() {
		p.shrinkCond.Signal()
	}()

	if p.isGrowthBlocked.Load() {
		return errGrowthBlocked
	}

	newCapacity := p.calculateNewPoolCapacity()

	if err := p.updatePoolCapacity(newCapacity); err != nil {
		return fmt.Errorf("%w: %w", errRingBufferFailed, err)
	}

	p.stats.totalGrowthEvents++
	err := p.tryL1ResizeIfTriggered()
	if err != nil {
		return err
	}

	return nil
}

// preReadBlockHook is called before a read operation blocks on the ring buffer.
// It attempts to get an object from L1 cache to avoid blocking.
func (p *Pool[T]) preReadBlockHook() (zero T, tryAgain bool, success bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	attempts := p.config.fastPath.preReadBlockHookAttempts
	if attempts == 0 {
		return zero, false, false
	}

	chPtr := p.cacheL1
	ch := *chPtr

	for i := range attempts {
		select {
		case obj, ok := <-ch:
			if !ok {
				return zero, false, false
			}
			return obj, false, true
		default:
			if i == attempts-1 {
				return zero, true, false
			}
		}
	}
	return zero, false, false
}
