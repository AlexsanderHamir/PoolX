package contexts

// import (
// 	"sync"
// )

// type MemoryContextType int

// const (
// 	TupleLevel MemoryContextType = iota + 1
// 	AccountingLevel
// )

// type AllocationStrategy int

// const (
// 	DefaultAllocation AllocationStrategy = iota + 1
// 	SmallObjectAllocation
// 	MediumObjectAllocation
// 	LargeObjectAllocation
// )

// type ContextManager struct {
// 	mu       *sync.RWMutex
// 	ctxCache map[MemoryContextType][]*MemoryContext // x
// }

// func NewContextManager() *ContextManager {
// 	return &ContextManager{
// 		mu:       &sync.RWMutex{},
// 		ctxCache: map[MemoryContextType][]*MemoryContext{}, // x
// 	}
// }

// // if cache type isn't cached, the method will return create a new context.
// func (cm *ContextManager) GetOrCreateContext(ctxType MemoryContextType, config MemoryContextConfig) (*MemoryContext, bool) {
// 	cm.mu.Lock()
// 	defer cm.mu.Unlock()

// 	var ctx *MemoryContext
// 	var wasCached bool

// 	ctxs, ok := cm.ctxCache[ctxType]
// 	if len(ctxs) == 0 || !ok {
// 		ctx = NewMemoryContext(config)
// 		ctx.active = true

// 		return ctx, wasCached
// 	}

// 	ctx = ctxs[0]
// 	ctx.active = true

// 	cm.ctxCache[ctxType] = ctxs[1:]
// 	wasCached = true

// 	return ctx, wasCached
// }

// // caches the memory context structure for similar queries
// func (cm *ContextManager) ReturnContext(rootCtx *MemoryContext) {
// 	rootCtx.mu.Lock()
// 	rootCtx.active = false
// 	rootCtx.mu.Unlock()

// 	cm.mu.Lock()
// 	cm.ctxCache[rootCtx.contextType] = append(cm.ctxCache[rootCtx.contextType], rootCtx)
// 	cm.mu.Unlock()
// }
