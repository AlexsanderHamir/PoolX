# Memory Context

A high-performance Go library for managing memory through memory contexts and object pools, the idea is to reduce the unecessary creation of memory.

## Features

- **Context Manager**:

  - Caches memory contexts.
  - Returns a reference to the context.
  - Cleans idle contexts if configured to do so.
  - Controls how many references are a allowed for all contexts.

- **Memory Context**:

  - Hierarchical context management with parent-child relationships
  - Child inherits the parent pools.
  - Object pooling with type-specific pools
  - Reference counting for memory management
  - Support for custom pool configurations and allocators

- **Pool**

  - **Type-Specific Pools**: Create pools for specific data types
  - **Custom Allocators**: Implement custom allocation strategies for objects
  - **Pool Lifecycle Management**: Configurable cleanup and resource management
  - **Performance Monitoring**: Track pool utilization and performance metrics
  - **Ring Buffer Storage**: Uses efficient ring buffer implementation for object storage and retrieval
  - **Fast Path Optimization**: Specialized high-performance path for frequent operations

## Installation

```bash
go get github.com/AlexsanderHamir/memory_context
```

## Basic Usage

```go
import "github.com/AlexsanderHamir/memory_context"

// Create a new memory context with default settings
ctx := memory_context.NewContext()

// Use the context for memory operations
// ...
```

## Advanced Configuration

The library provides extensive configuration options through the `PoolConfigBuilder`:

```go
// Create a custom pool configuration
config := pool.NewPoolConfigBuilder().
    SetInitialCapacity(1000).                    // Initial pool size
    SetGrowthPercent(50).                        // Growth percentage when expanding
    SetShrinkAggressiveness(pool.AggressivenessModerate). // Shrink behavior
    SetHardLimit(10000).                         // Maximum pool size
    SetFastPathInitialSize(100).                 // Fast path initial size
    SetVerbose(true).                            // Enable verbose logging
    Build()

// Use the configuration with your memory context
```

### Key Configuration Options

- **Pool Size Management**:

  - `SetInitialCapacity`: Initial number of items in the pool
  - `SetHardLimit`: Maximum pool size
  - `SetGrowthPercent`: Percentage increase when growing
  - `SetFixedGrowthFactor`: Fixed growth multiplier

- **Shrink Behavior**:

  - `SetShrinkAggressiveness`: Control how aggressively the pool shrinks (1-5)

    - Level 1 (Conservative): Minimal shrinking, preserves capacity for potential load spikes
    - Level 2 (Balanced): Default balanced approach to memory management
    - Level 3 (Aggressive): More frequent shrinking to optimize memory usage
    - Level 4 (Very Aggressive): Rapid capacity reduction when underutilized
    - Level 5 (Extreme): Maximum memory optimization, may impact performance

  - `SetShrinkCheckInterval`: How often to check for shrinking opportunities

    - Defines the frequency of background checks for pool shrinking conditions
    - Shorter intervals enable more responsive memory management
    - Longer intervals reduce overhead but may delay memory reclamation

  - `SetIdleThreshold`: How long items must be idle before considering shrink

    - Minimum duration the pool must remain idle before shrinking
    - Helps prevent premature shrinking during temporary usage dips
    - Longer thresholds provide more stability but may retain memory longer

  - `SetShrinkPercent`: Percentage to shrink by when reducing size

    - Controls how much capacity is reduced during each shrink operation
    - Example: 0.25 means reduce capacity by 25% when shrinking
    - Higher values release memory faster but may require more frequent growth

  - `SetMinShrinkCapacity`: Minimum capacity after shrinking operations
    - Prevents the pool from shrinking below a specified size
    - Ensures a baseline capacity for sudden usage spikes
    - Helps maintain performance by keeping a minimum buffer of available objects
    - Setting it to be equal to the initial size will block shrinking.

- **Fast Path Optimization**:

  - `SetFastPathInitialSize`: Initial size of the fast path
  - `SetFastPathFillAggressiveness`: How aggressively to fill the fast path
  - `SetFastPathRefillPercent`: Percentage to refill when depleted

- **Ring Buffer Configuration**:
  - `SetRingBufferBlocking`: Enable/disable blocking operations
  - `WithTimeOut`: Set timeout for buffer operations
  - `SetRingBufferReadTimeout`: Specific read timeout
  - `SetRingBufferWriteTimeout`: Specific write timeout

## Performance Considerations

- Use appropriate growth and shrink settings based on your workload
- Consider enabling the fast path for high-frequency operations
- Monitor pool utilization to tune configuration parameters
- Use verbose mode during development to understand pool behavior

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
