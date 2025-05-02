# Memory Context

A highly configurable object pool implementation designed to control object creation under high-concurrency scenarios.

![Flow](../assets/flow.png)

## Installation

```bash
go get github.com/AlexsanderHamir/memory_context
```

## Quick Example

```go
import (
    "github.com/AlexsanderHamir/memory_context/pool"
    "time"
)

// Create a new pool configuration
// Bulk functions
poolConfig, err := pool.NewPoolConfigBuilder().
    SetPoolBasicConfigs(128, 5000, false, true).
    SetRingBufferBasicConfigs(true, 0, 0, time.Second*5).
    SetRingBufferGrowthConfigs(150.0, 0.85, 1.2).
    SetRingBufferShrinkConfigs(
        time.Second*2,
        time.Second*5,
        time.Second*1,
        2,
        5,
        15,
        3,
        0.3,
        0.5,
    ).
    SetFastPathBasicConfigs(128, 4, 4, 1.0, 0.20).
    SetFastPathGrowthConfigs(100.0, 1.5, 0.85).
    SetFastPathShrinkConfigs(0.7, 5).
    Build()

// Use individual setter methods if you only need to change one field
func (b *poolConfigBuilder) SetInitialCapacity(cap int) *poolConfigBuilder {
	b.config.initialCapacity = cap
	return b
}

func (b *poolConfigBuilder) SetHardLimit(count int) *poolConfigBuilder {
	b.config.hardLimit = count
	return b
}

if err != nil {
    // handle error
}

// Create a new pool with the configuration
poolType := reflect.TypeOf(&Example{})
pool, err := pool.NewPool(poolConfig, allocator, cleaner, poolType)
if err != nil {
    panic(err)
}
```

## ⚠️ Important Warnings

- **Type Safety**: Only pointers can be stored in the pool (ring buffer / L1). Non-pointer types will result in an error.
- **Default Behavior**: The ring buffer operates in non-blocking mode by default. Review default values carefully before using them.
- **Performance Impact**: Resizing operations are expensive. Minimize them when possible by carefully tuning growth and shrink parameters.

## Configuration

### Pool Basic Configuration (`SetPoolBasicConfigs`)

- `initialCapacity`: Initial capacity of the ring buffer. If no capacity is provided for the fast path, it uses the same value.
- `hardLimit`: Maximum number of objects the pool can grow to. The fast path and ring buffer grow together; once the ring buffer reaches the hard limit, the fast path stops growing as well.
- `verbose`: Enables detailed logging for pool activity.
- `enableChannelGrowth`: When disabled, the fast path will not grow even if the ring buffer does.
- `enableStats`: Collect non-essential statistics when enabled.

### Ring Buffer Basic Configuration (`SetRingBufferBasicConfigs`)

- `block`: Whether the ring buffer blocks on reads/writes when there are no objects to read or no space to write.
- `rTimeout`: Read timeout duration.
- `wTimeout`: Write timeout duration.
- `bothTimeout`: Timeout duration for both reads and writes.

### Ring Buffer Growth Configuration (`SetRingBufferGrowthConfigs`)

- `exponentialThresholdFactor`: Threshold at which the growth strategy switches from exponential to fixed; this is a multiplier of the initial capacity.
- `growthPercent`: Percentage increase during exponential growth (e.g., 0.85 means 85% increase).
- `fixedGrowthFactor`: Multiplier of initial capacity used during fixed growth after the threshold is reached.

### Ring Buffer Shrink Configuration (`SetRingBufferShrinkConfigs`)

- `checkInterval`: Frequency at which the pool checks whether it should shrink.
- `idleThreshold`: Duration of inactivity (since the last get request) before the pool is considered idle and eligible for shrinking.
- `shrinkCooldown`: Minimum time to wait between consecutive shrink operations.
- `minIdleBeforeShrink`: Minimum number of idle checks before shrinking is allowed.
- `minCapacity`: Minimum number of objects the pool can shrink to.
- `maxConsecutiveShrinks`: Maximum number of consecutive shrink operations allowed before further shrinking is blocked. Blocking is lifted by the next get request.
- `minUtilizationBeforeShrink`: Minimum utilization percentage that allows shrinking.
- `stableUnderutilizationRounds`: Number of consecutive underutilization checks required before shrinking.
- `shrinkPercent`: Percentage by which capacity is reduced during shrinking.
- `aggressivenessLevel`: Implement default config based on chosen level.
- `enforceCustomConfig`: Disables all default configs and only runs if all custom configs are set.

### Fast Path Basic Configuration (`SetFastPathBasicConfigs`)

- `enableChannelGrowth`: Enables dynamic resizing of the fast path channel.
- `initialSize`: Initial size of the fast path (L1 cache).
- `growthEventsTrigger`: Number of ring buffer growth events required before the fast path grows.
- `shrinkEventsTrigger`: Number of ring buffer shrink events required before the fast path shrinks.
- `fillAggressiveness`: Degree of aggressiveness when filling the fast path (range: 0.0 to 1.0).
- `refillPercent`: Threshold at which the fast path is refilled, based on emptiness percentage.

### Fast Path Growth Configuration (`SetFastPathGrowthConfigs`)

- `exponentialThresholdFactor`: Threshold for switching from exponential to fixed growth in the fast path.
- `fixedGrowthFactor`: Multiplier used for fixed growth.
- `growthPercent`: Percentage by which the fast path grows during exponential growth.

### Fast Path Shrink Configuration (`SetFastPathShrinkConfigs`)

- `shrinkPercent`: Percentage by which the fast path is shrunk.
- `minCapacity`: Minimum number of objects the fast path can shrink to.

## Pool Behavior

### Object Lifecycle

1. **Object Retrieval**:

   - First attempts to retrieve from L1 (fast path)
   - If L1 is empty, attempts to refill L1
   - If refill fails, falls back to ring buffer
   - If ring buffer is non-blocking and empty, returns `ErrIsEmpty`

2. **Object Return**:
   - First attempts to place in L1
   - If there are blocked get requests, returns to ring buffer
   - If ring buffer is non-blocking and full, returns `ErrIsFull`

### Performance Considerations

1. **L1 Cache Behavior**:

   - Depending on the L1 size, the ring buffer may remain mostly empty unless frequent spills from L1 occur
   - This indicates that L1 or the ring buffer are not large enough to sustain the load

2. **Growth and Shrink Behavior**:

   - Growth is blocked when `hardLimit` is reached
   - Growth is re-enabled once the pool shrinks below the limit
   - After `maxConsecutiveShrinks` consecutive shrinks, further shrinking is blocked until a new get request
   - The `minUtilizationBeforeShrink` threshold is crucial for efficient pool sizing

3. **Blocking Behavior**:
   - If the ring buffer is set to blocking, it only blocks when the `hardLimit` is reached
   - Non-blocking mode returns errors instead of blocking

## Contributing

We welcome contributions to Memory Context! Here's how you can help:

### Development Setup

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/AlexsanderHamir/memory_context.git
   cd memory_context
   ```
3. Create a new branch for your changes:
   ```bash
   git checkout -b feature/your-feature-name
   ```

### Making Changes

1. Ensure you have Go 1.23.4 or later installed
2. Run tests before making changes:
   ```bash
   go test ./...
   ```
3. Make your changes, following these guidelines:
   - Write clear, descriptive commit messages
   - Add tests for new functionality
   - Update documentation for any new features or changes
   - Follow the existing code style and patterns

### Testing

- Run all tests:
  ```bash
  go test ./...
  ```

### Submitting Changes

1. Push your changes to your fork
2. Create a Pull Request against the main repository
3. Ensure your PR:
   - Has a clear description of the changes
   - Includes tests for new functionality
   - Updates documentation if needed
   - Passes all CI checks

### Reporting Issues

When reporting issues, please include:

- Go version
- Operating system
- Steps to reproduce
- Expected behavior
- Actual behavior
- Any relevant logs or error messages

### Questions and Discussion

- For general questions, please open a GitHub Discussion
- For bug reports, please open a GitHub Issue
- For feature requests, please open a GitHub Issue with the "enhancement" label

Thank you for contributing to Memory Context!
