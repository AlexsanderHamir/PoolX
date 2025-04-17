package contexts

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrContextNotFound    = errors.New("context not found")
	ErrContextInUse       = errors.New("context is currently in use")
	ErrInvalidContextType = errors.New("invalid context type")
	ErrContextClosed      = errors.New("context is closed")
)

const (
	DefaultMaxReferences   = 10
	DefaultMaxIdleTime     = 5 * time.Minute
	DefaultCleanupInterval = 1 * time.Minute
)

type MemoryContextType string

type ContextManager struct {
	mu       *sync.RWMutex
	ctxCache map[MemoryContextType]*MemoryContext
	config   *ContextConfig
	stopChan chan struct{}
}

type ContextConfig struct {
	// how many goroutines can use the context at the same time.
	MaxReferences int32

	// if true, the context will be automatically cleaned up when it is not in use.
	AutoCleanup bool

	// how long the context can be idle before it is automatically cleaned up.
	MaxIdleTime time.Duration

	// the interval at which the context will be cleaned up when it is not in use.
	CleanupInterval time.Duration
}

// NewDefaultConfig creates a new ContextConfig with default values
func newDefaultConfig() *ContextConfig {
	return &ContextConfig{
		MaxReferences:   DefaultMaxReferences,
		AutoCleanup:     true,
		MaxIdleTime:     DefaultMaxIdleTime,
		CleanupInterval: DefaultCleanupInterval,
	}
}

// NewContextManager creates a new context manager with default configuration
func NewContextManager(config *ContextConfig) *ContextManager {
	if config == nil {
		config = newDefaultConfig()
	}

	cm := &ContextManager{
		mu:       &sync.RWMutex{},
		ctxCache: make(map[MemoryContextType]*MemoryContext),
		config:   config,
		stopChan: make(chan struct{}),
	}

	if config.AutoCleanup {
		go cm.startCleanupRoutine()
	}

	return cm
}

// startCleanupRoutine runs a background goroutine to clean up idle contexts
func (cm *ContextManager) startCleanupRoutine() {
	ticker := time.NewTicker(cm.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cm.cleanupIdleContexts()
		case <-cm.stopChan:
			return
		}
	}
}

// cleanupIdleContexts removes contexts that have been idle for too long
func (cm *ContextManager) cleanupIdleContexts() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	for ctxType, ctx := range cm.ctxCache {
		ctx.mu.RLock()
		if !ctx.active && now.Sub(ctx.lastUsed) > cm.config.MaxIdleTime {
			ctx.mu.RUnlock()
			if err := cm.CloseContext(ctxType); err != nil {
				fmt.Printf("Error closing idle context %s: %v\n", ctxType, err)
			}
		} else {
			ctx.mu.RUnlock()
		}
	}
}

// StopCleanup stops the auto-cleanup routine
func (cm *ContextManager) StopCleanup() {
	close(cm.stopChan)
}

func GetOrCreateTypedPool[T any](cm *ContextManager, ctxType MemoryContextType, config MemoryContextConfig) (*TypedPool[T], bool, error) {
	ctx, exists, err := cm.getOrCreateContext(ctxType, config)
	if err != nil {
		return nil, false, err
	}
	return NewTypedPool[T](ctx), exists, nil
}

// getOrCreateContext is the internal version of GetOrCreateContext
func (cm *ContextManager) getOrCreateContext(ctxType MemoryContextType, config MemoryContextConfig) (*MemoryContext, bool, error) {
	if ctxType == "" {
		return nil, false, ErrInvalidContextType
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	ctx, ok := cm.ctxCache[ctxType]
	if !ok {
		ctx = NewMemoryContext(config)
		ctx.active = true
		ctx.referenceCount = 1
		cm.ctxCache[ctxType] = ctx
		return ctx, false, nil
	}

	if ctx.closed {
		return nil, false, ErrContextClosed
	}

	ctx.active = true
	ctx.referenceCount++

	return ctx, true, nil
}

// ReturnContext returns a context to the manager and updates its state
func (cm *ContextManager) ReturnContext(ctx *MemoryContext) error {
	if ctx == nil {
		return errors.New("cannot return nil context")
	}

	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	if ctx.closed {
		return ErrContextClosed
	}

	if ctx.referenceCount > 0 {
		ctx.referenceCount--
	}

	ctx.active = ctx.referenceCount > 0

	return nil
}

// CloseContext closes a context and removes it from the manager
func (cm *ContextManager) CloseContext(ctxType MemoryContextType) error {
	cm.mu.RLock()
	ctx, ok := cm.ctxCache[ctxType]
	if !ok {
		return ErrContextNotFound
	}
	cm.mu.RUnlock()

	ctx.mu.RLock()
	if ctx.referenceCount > 0 {
		return ErrContextInUse
	}
	ctx.mu.RUnlock()

	if err := ctx.Close(); err != nil {
		return fmt.Errorf("error closing context %s: %v", ctxType, err)
	}

	cm.mu.Lock()
	delete(cm.ctxCache, ctxType)
	cm.mu.Unlock()

	return nil
}

// ValidateContext checks if a context is valid and active
func (cm *ContextManager) ValidateContext(ctx *MemoryContext) bool {
	if ctx == nil {
		return false
	}

	ctx.mu.RLock()
	defer ctx.mu.RUnlock()

	return !ctx.closed && ctx.active
}

// Close stops the cleanup routine and closes all managed contexts
func (cm *ContextManager) Close() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.config.AutoCleanup {
		cm.StopCleanup()
	}

	for ctxType := range cm.ctxCache {
		if err := cm.CloseContext(ctxType); err != nil {
			return fmt.Errorf("error closing context %s: %v", ctxType, err)
		}
	}

	if cm.stopChan != nil {
		close(cm.stopChan)
		cm.stopChan = nil
	}

	cm.ctxCache = nil
	cm.config = nil

	return nil
}
