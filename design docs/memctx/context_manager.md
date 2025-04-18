# Context Manager

The Context Manager is responsible for managing the lifecycle of memory contexts, including their creation, retrieval, and cleanup.

## Overview

The Context Manager (`ctx-manager.go`) provides a centralized way to manage memory contexts, ensuring proper resource allocation and cleanup. It maintains a cache of active contexts and handles their lifecycle.

## Key Features

- Context creation and retrieval
- Automatic cleanup of idle contexts
- Reference counting
- Thread-safe operations
- Configurable behavior

## Configuration

The Context Manager can be configured through the `ContextConfig` struct:

```go
type ContextConfig struct {
    MaxReferences   int32         // Maximum number of concurrent references
    MaxIdleTime     time.Duration // Time before idle context cleanup
    CleanupInterval time.Duration // Interval for cleanup routine
}
```

Default values:

- `DefaultMaxReferences`: 10
- `DefaultMaxIdleTime`: 5 minutes
- `DefaultCleanupInterval`: 1 minute

## API

### Creation

```go
// Create a new context manager with default configuration
manager := contexts.NewContextManager(nil)

// Create with custom configuration
config := &contexts.ContextConfig{
    MaxReferences:   5,
    MaxIdleTime:     10 * time.Minute,
    CleanupInterval: 2 * time.Minute,
}
manager := contexts.NewContextManager(config)
```

### Context Management

```go
// Get or create a context
ctx, exists, err := manager.GetOrCreateContext("myContext", contexts.MemoryContextConfig{})

// Return a context (decrement reference count)
err := manager.ReturnContext(ctx)

// Close a specific context
err := manager.CloseContext("myContext", false)

// Validate a context
isValid := manager.ValidateContext(ctx)
```

### Cleanup

```go

// Close all contexts and cleanup
err := manager.Close()
```

## Error Handling

The Context Manager defines several error types:

- `ErrContextNotFound`
- `ErrContextInUse`
- `ErrInvalidContextType`
- `ErrContextClosed`
- `ErrContextManagerClosed`
- `ErrMaxReferencesReached`
- `ErrInvalidConfig`

## Thread Safety

All operations on the Context Manager are thread-safe, protected by appropriate mutexes:

- Read operations use `RLock()`
- Write operations use `Lock()`
- Context-specific operations maintain their own locks

## Best Practices

1. Always check for errors when creating or retrieving contexts
2. Use `ReturnContext` when done with a context to prevent resource leaks
3. Configure appropriate timeouts based on your application's needs
4. Close the manager when it's no longer needed
5. Handle cleanup errors appropriately
