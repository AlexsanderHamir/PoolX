    `# Memory Context Implementation

The Memory Context (`memctx.go`) represents an individual memory context that manages object pools and maintains relationships with other contexts.

## Overview

Each Memory Context instance:

- Maintains its own set of object pools
- Can have parent-child relationships
- Tracks usage and lifecycle state
- Provides thread-safe access to pooled objects

## Structure

```go
type MemoryContext struct {
    active         bool
    closed         bool
    referenceCount int32
    mu             sync.RWMutex
    contextType    MemoryContextType
    parent         *MemoryContext
    children       []*MemoryContext
    pools          map[reflect.Type]*pool.Pool[any]
    createdAt      time.Time
    lastUsed       time.Time
}
```

## Key Features

- Object pool management
- Hierarchical context relationships
- Reference counting
- Thread-safe operations
- Lifecycle tracking

## API

### Creation

```go
// Create a new memory context
config := contexts.MemoryContextConfig{
    Parent:      parentCtx,
    ContextType: "myContext",
}
ctx := contexts.NewMemoryContext(config)
```

### Context Hierarchy

```go
// Create a child context
child, err := ctx.CreateChild()

// Register a custom child context
ctx.RegisterChild(customChild)

// Get all children
children := ctx.GetChildren()
```

### Object Pool Management

```go
// Create a new object pool
err := ctx.CreatePool(
    reflect.TypeOf(MyObject{}),
    pool.PoolConfig{},
    func() any { return &MyObject{} },
    func(obj any) { /* cleanup */ },
)

// Acquire an object from the pool
obj := ctx.Acquire(reflect.TypeOf(MyObject{}))

// Release an object back to the pool
success := ctx.Release(reflect.TypeOf(MyObject{}), obj)

// Get a pool by type
pool := ctx.GetPool(reflect.TypeOf(MyObject{}))
```

### Lifecycle Management

```go
// Close the context and all its resources
err := ctx.Close()
```

## Thread Safety

All operations on the Memory Context are thread-safe:

- Read operations use `RLock()`
- Write operations use `Lock()`
- Pool operations maintain their own locks

## Best Practices

1. Always release objects back to the pool when done
2. Create child contexts for independent resource management
3. Close contexts when they're no longer needed
4. Use appropriate pool configurations for your objects
5. Handle errors from pool operations

## Error Handling

The Memory Context can return the following errors:

- `ErrContextClosed` when operating on a closed context
- Pool-specific errors from the underlying pool implementation

## Configuration

The Memory Context can be configured through the `MemoryContextConfig` struct:

```go
type MemoryContextConfig struct {
    Parent      *MemoryContext
    ContextType MemoryContextType
}
```
