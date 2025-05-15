# PoolX Technical Architecture

This document provides a detailed technical explanation of PoolX's architecture, design decisions, and internal workings.

## Core Components

### 1. Two-Level Caching System

#### L1 Cache (Channel-based Fast Path)

- Reduces contention on the main pool
- Automatically refills when utilization drops
- Can grow/shrink independently of the main pool

#### Main Pool (Ring Buffer)

- Uses a ring buffer implementation for efficient storage
- Supports blocking and non-blocking operations
- Implements timeout-based operations

### 2. Growth and Shrink Mechanisms

#### Growth Strategy

```go
// Two-phase growth approach based on capacity threshold
exponentialThreshold := float64(initialCapacity) * thresholdFactor
if float64(currentCapacity) < exponentialThreshold {
    // Exponential growth phase
    growthStep := float64(currentCapacity) * bigGrowthFactor
    newCapacity = currentCapacity + int(growthStep)
} else {
    // Controlled growth phase
    fixedStep := float64(currentCapacity) * controlledGrowthFactor
    newCapacity = currentCapacity + int(fixedStep)
}
```

The growth strategy uses a threshold-based approach where:

- Below threshold: Growth is exponential, calculated as a percentage of current capacity
- Above threshold: Growth is controlled, using a smaller fixed percentage
- Threshold is calculated as a factor of the initial capacity

#### Shrink Strategy

```go
// Multi-factor shrink decision
if utilization < minUtilization &&
   stableUnderutilizationRounds >= requiredRounds &&
   timeSinceLastShrink >= cooldownPeriod {
   newCapacity := int(currentCap) * (100 - p.config.shrink.shrinkPercent) / 100
}
```

### 3. Thread Safety

The pool uses multiple synchronization mechanisms:

- `sync.RWMutex` for pool-wide operations
- `sync.Cond` for blocking operations
- `atomic` operations for statistics

### 4. Object Lifecycle

1. **Creation**:

   - Objects are created by the user-provided allocator
   - Initial population uses the allocation strategy
   - Template object is maintained for cloning

2. **Usage**:

   - Objects are retrieved from L1 cache if available
   - Fallback to main pool if L1 is empty
   - Automatic refill of L1 when utilization is low

3. **Return**:

   - Objects are cleaned by user-provided cleaner
   - Attempted to return to L1 cache first
   - Fallback to main pool if L1 is full

4. **Cleanup**:
   - Objects are cleaned before being returned
   - Pool resources are released on Close()
   - Outstanding objects are tracked

## Configuration System

### 1. Builder Pattern

The configuration system uses a builder pattern for type-safe configuration:

```go
type PoolConfigBuilder[T any] interface {
    SetPoolBasicConfigs(...) PoolConfigBuilder[T]
    SetRingBufferBasicConfigs(...) PoolConfigBuilder[T]
    SetRingBufferGrowthConfigs(...) PoolConfigBuilder[T]
    // ... more configuration methods
    Build() (*PoolConfig[T], error)
}
```

### 2. Validation System

Configuration validation ensures:

- Logical consistency of parameters
- Type safety
- Resource limits
- Performance constraints

## Statistics and Monitoring

### 1. Essential Statistics

```go
type poolStats struct {
    objectsCreated   int
    objectsDestroyed int
    initialCapacity  int
    currentCapacity  int
    totalGets        atomic.Uint64
    FastReturnHit    atomic.Uint64
    FastReturnMiss   atomic.Uint64
    // ... more statistics
}
```

## Error Handling

### 1. Error Types

```go
var (
    errGrowthBlocked    = errors.New("growth is blocked")
    errRingBufferFailed = errors.New("ring buffer failed core operation")
    errNoItemsToMove    = errors.New("no items to move")
    errNilObject        = errors.New("object is nil")
    errNilConfig        = errors.New("config is nil")
)
```

### 2. Error Recovery

- **Retry Logic**: For transient failures
- **Fallback Paths**: Alternative operation paths
- **Resource Cleanup**: Proper cleanup on errors
- **State Recovery**: Consistent state maintenance

## Implementation Details

### 1. Synchronization and Contention Management

- A semaphore ensures that only one goroutine is responsible for refilling or expanding the pool's underlying data structures (e.g., channel or ring buffer), preventing redundant work and reducing contention.
- After all optimizations were made, using the channel became the only bottleneck, but even though it requires synchronization internally, it's still faster than using just the ring buffer which requires a Writer-Mutex.

### 2. Memory Optimization

- The cloner function is an optimization to reduce memory usage when using a heavy allocator. It performs a shallow copy, delegating the responsibility of initializing reference fields to the user.

### 3. Pointer Safety and Resizing

Since resizing may replace the underlying ring buffer or channel, existing pointers can become stale and unsafe to use. This is handled through:

- Fast path: Falling back to the slow path when contention is detected
- Slow path: Retrying the operation to ensure correctness

### 4. Blocking Behavior

The blocking behavior is carefully managed to prevent indefinite blocking of goroutines:

- When objects are returned to the pool, we wake up one blocked reader
- Retrieval attempts from the fast path use a pre-read block hook

### 5. Growth and Shrink Coordination

The growth and shrink operations are carefully coordinated to prevent thrashing:

- Growth is blocked when reaching hard limits
- Shrink operations respect minimum capacity and in-use object counts
- Consecutive shrink operations are limited to prevent excessive shrinking
- A cooldown period is enforced between shrink operations

### 6. Initialization Process

The pool implements a two-phase initialization process:

- First phase creates the basic structure with default or provided configuration
- Second phase pre-allocates objects based on the allocation strategy
- This ensures the pool is ready for immediate use with minimal runtime overhead

### 7. Type Safety and Cleanup

- The pool's type system enforces pointer types through runtime validation, ensuring safe object management and preventing common misuse patterns
- The pool's cleanup process is asynchronous when there are outstanding objects, implementing a graceful shutdown with a timeout to prevent indefinite waiting
