# PoolX

[![Go Version](https://img.shields.io/badge/Go-1.21%2B-blue)](https://golang.org)
[![Build](https://github.com/AlexsanderHamir/PoolX/actions/workflows/test.yml/badge.svg)](https://github.com/AlexsanderHamir/PoolX/actions)
[![Coverage Status](https://coveralls.io/repos/github/AlexsanderHamir/PoolX/badge.svg?branch=main)](https://coveralls.io/github/AlexsanderHamir/PoolX?branch=main)
[![Go Report Card](https://goreportcard.com/badge/github.com/AlexsanderHamir/PoolX)](https://goreportcard.com/report/github.com/AlexsanderHamir/PoolX)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![GoDoc](https://godoc.org/github.com/AlexsanderHamir/PoolX?status.svg)](https://godoc.org/github.com/AlexsanderHamir/PoolX)
![Issues](https://img.shields.io/github/issues/AlexsanderHamir/PoolX)
![Last Commit](https://img.shields.io/github/last-commit/AlexsanderHamir/PoolX)
![Code Size](https://img.shields.io/github/languages/code-size/AlexsanderHamir/PoolX)
![Version](https://img.shields.io/github/v/tag/AlexsanderHamir/PoolX?sort=semver)

## Overview

PoolX is a high-performance, generic object pool implementation for Go that provides fine-grained control over object lifecycle management, memory allocation, and resource utilization.

![Flow](../assets/flow.png)

## Table of Contents

- [Why PoolX?](#why-poolx)
- [Performance](#performance)
- [Features](#features)
- [Quick Start](#quick-start)
- [Configuration Options](#configuration-options)
- [Use Cases](#use-cases)
- [Best Practices](#best-practices)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [License](#license)

## Why PoolX?

PoolX offers several advantages over standard Go object pools:

- **Fine-grained Control**: Extensive configuration options for different use cases, control object lifecycle
- **Memory Efficiency**: Intelligent object reuse and cleanup strategies
- **Type Safety**: Generic implementation ensures compile-time type checking

## Performance

PoolX is not a direct replacement for `sync.Pool`, it's supposed to be used to control resource consumption, if used in the same way as `sync.Pool` you will get worse performance:

```
BenchmarkSyncPoolHighContention         748899              1598 ns/op               4 B/op          0 allocs/op
BenchmarkPoolXHighContention            605977              2023 ns/op              22 B/op          0 allocs/op
```

## Features

- **FastPath**: Fast L1 cache for frequently accessed objects and a ring buffer for bulk storage
- **Configurable Behavior**: Fine-grained control over growth, shrinking, and performance characteristics
- **Thread-Safe**: All operations are safe for concurrent use
- **Object Cleanup**: Built-in support for object cleanup when returned to the pool
- **Generic Implementation**: Type-safe pool for any pointer type
- **Ring Buffer Backend**: Efficient storage using a ring buffer implementation

## Quick Start

```go
import "github.com/AlexsanderHamir/PoolX/v2/pool"

// Create a pool configuration
config := pool.NewPoolConfigBuilder[MyObject]().
    SetPoolBasicConfigs(64, 1000, true).  // initial capacity, hard limit, enable channel growth
    Build()

// Create the pool with required functions
myPool, err := pool.NewPool(
    config,
    func() *MyObject { return &MyObject{} },    // allocator
    func(obj *MyObject) { obj.Reset() },        // cleaner

      // For large structs, prefer cloning over allocating to save memory.
     // Note: This will delay initialization, as reference types will share underlying data.
    // It's your responsibility to initialize when yoou need it.
    func(obj *MyObject) *MyObject {             // cloner 
        dst := *obj
	return &dst
     },
)
if err != nil {
    log.Fatal(err)
}
defer myPool.Close()

// Use the pool
obj, err := myPool.Get()
if err != nil {
    log.Fatal(err)
}
defer myPool.Put(obj)

// Use the object
obj.DoSomething()
```

## Configuration Options

### Basic Pool Configuration

```go
config := pool.NewPoolConfigBuilder[MyObject]().
    SetPoolBasicConfigs(
        initialCapacity,    // Initial size of the pool
        hardLimit,         // Maximum number of objects
        enableChannelGrowth // Enable dynamic L1 cache growth
    )
```

### Growth Configuration

```go
config := pool.NewPoolConfigBuilder[MyObject]().
    SetRingBufferGrowthConfigs(
        thresholdFactor,        // When to switch from exponential to controlled growth
        bigGrowthFactor,        // Growth rate below threshold (e.g., 0.75 for 75% growth)
        controlledGrowthFactor  // Fixed growth rate above threshold
    )
```

### Shrink Configuration

```go
config := pool.NewPoolConfigBuilder[MyObject]().
    SetRingBufferShrinkConfigs(
        checkInterval,              // Time between shrink checks
        shrinkCooldown,            // Minimum time between shrinks
        stableUnderutilizationRounds, // Required stable underutilization rounds
        minCapacity,               // Minimum pool capacity
        maxConsecutiveShrinks,     // Maximum consecutive shrink operations
        minUtilizationBeforeShrink, // Utilization threshold for shrinking
        shrinkPercent              // Percentage to shrink by
    )
```

### Fast Path (L1 Cache) Configuration

```go
config := pool.NewPoolConfigBuilder[MyObject]().
    SetFastPathBasicConfigs(
        initialSize,           // Initial L1 cache size
        growthEventsTrigger,   // Growth events before L1 grows
        shrinkEventsTrigger,   // Shrink events before L1 shrinks
        fillAggressiveness,    // How aggressively to fill L1 (percentage)
        refillPercent         // When to refill L1 (percentage)
    ).
    SetFastPathGrowthConfigs(
        growthFactor,          // How much to grow L1 by when growing (e.g., 0.5 for 50% growth)
        maxGrowthSize,         // Maximum size L1 can grow to
        growthCooldown        // Minimum time between L1 growth operations
    ).
    SetFastPathShrinkConfigs(
        shrinkFactor,          // How much to shrink L1 by when shrinking (e.g., 0.25 for 25% reduction)
        minShrinkSize,         // Minimum size L1 can shrink to
        shrinkCooldown,        // Minimum time between L1 shrink operations
        utilizationThreshold   // Utilization threshold below which L1 can shrink
    ).
    SetFastPathShrinkAggressiveness(level AggressivenessLevel)  // Configure L1 shrink behavior using predefined levels (1-5)
```

### Allocation Strategy

The allocation strategy controls how objects are created and pre-allocated in the pool. This is particularly useful for optimizing performance in high-throughput scenarios.

```go
config := pool.NewPoolConfigBuilder[MyObject]().
    SetAllocationStrategy(
        allocPercent,  // Percentage of objects to preallocate at initialization and when growing
        allocAmount    // Amount of objects to create per request when L1 is empty
    )
```

The allocation strategy provides two key benefits:

1. **Pre-allocation**: By setting `allocPercent`, you can pre-allocate a percentage of objects when:

   - The pool is first initialized
   - The pool grows to accommodate more objects
     This helps reduce allocation overhead during peak usage.

2. **Batch Creation**: The `allocAmount` parameter controls how many objects are created at once when:
   - The L1 cache is empty
   - New objects are needed
     This reduces the frequency of allocation operations and can improve performance.

Example usage patterns:

```go
// Aggressive pre-allocation (75% of capacity)
config.SetAllocationStrategy(75, 10)  // Pre-allocate 75%, create 10 at a time

// Moderate pre-allocation (50% of capacity)
config.SetAllocationStrategy(50, 5)   // Pre-allocate 50%, create 5 at a time

// Minimal pre-allocation (25% of capacity)
config.SetAllocationStrategy(25, 1)   // Pre-allocate 25%, create 1 at a time
```

## Statistics and Monitoring

The pool provides basic statistics through the `PrintPoolStats()` method:

```go
myPool.PrintPoolStats()
```

Statistics include:

- Pool capacity and utilization
- Object creation and destruction counts
- Fast path hit/miss rates
- Growth and shrink events
- L1 cache metrics

## Use Cases

PoolX is particularly useful in the following scenarios:

1. **Resource Limitation**: The system restricts the creation to a maximum of X objects.
2. **Rapid Cleanup**: Objects must be aggressively removed as soon as they become unnecessary.
3. **Object Caching**: To minimize creation overhead, objects are reused whenever possible—even if this results in increased memory usage, since cleanup is less aggressive in this mode.

## Documentation

> **Documentation**:
>
> - **API Reference**: For detailed API reference and interface definitions, see [pool/api.go](../pool/api.go)
> - **Technical Details**: For in-depth technical explanations and implementation details, see [docs/technical_explanations/](technical_explanations/)
> - **Design**: For overall design decisions, see [docs/ARCHITECTURE.md](ARCHITECTURE.md)
> - **Code Examples**: For practical usage examples and implementation patterns, see [pool/code_examples/](../code_examples)
> - **FAQ**: For frequently asked questions and common troubleshooting, see [docs/FAQS.md](FAQS.md)

## Best Practices

1. **Object Types**: Always use pointer types for pooled objects
2. **Cleanup**: Implement proper cleanup in the cleaner function
3. **Cloning**: Ensure proper object state initialization in the cloner function
4. **Capacity Planning**: Set appropriate initial capacity and hard limits
5. **Resource Management**: Always call `Close()` when done with the pool

## Advanced Usage

### Custom Growth/Shrink Behavior

```go
config := pool.NewPoolConfigBuilder[MyObject]().
    SetShrinkAggressiveness(pool.AggressivenessBalanced)  // Predefined levels
    // or
    EnforceCustomConfig()  // For complete control
```

## Contributing

We welcome contributions! Here's how you can help:

1. **Report Bugs**: Open an issue with detailed reproduction steps
2. **Feature Requests**: Suggest new features or improvements
3. **Code Contributions**: Submit pull requests with clear descriptions
4. **Documentation**: Help improve documentation and examples
5. **Testing**: Add test cases or improve test coverage

See our [Contributing Guidelines](CONTRIBUTING.md) for more details.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

**Keywords**: Go object pool, generic pool, high-performance pool, memory optimization, resource management, concurrent pool, Go generics, object reuse, memory efficiency, high-throughput, low-latency, connection pool, buffer pool, sync.Pool alternative
