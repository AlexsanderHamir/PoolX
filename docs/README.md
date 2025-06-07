# PoolX

[![Go Version](https://img.shields.io/badge/Go-1.23%2B-blue)](https://golang.org)
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

PoolX is a high-performance, generic object pool implementation for Go that provides fine-grained control over object lifecycle management and resource utilization. It's designed for scenarios where you need precise control over resource consumption under high concurrency, rather than as a direct replacement for `sync.Pool`.

![Flow](../assets/flow.png)

## Table of Contents

- [Features](#features)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Use Cases](#use-cases)
- [Performance](#performance)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [License](#license)

## Features

- **FastPath**: L1 cache for frequent access + ring buffer for bulk storage
- **Fine-grained Control**: Extensive configuration for growth, shrinking, and performance
- **Thread-Safe**: All operations are safe for concurrent use
- **Type Safety**: Generic implementation with compile-time type checking
- **Memory Efficiency**: Intelligent object reuse and cleanup strategies
- **Built-in Monitoring**: Statistics and metrics for pool performance

## Quick Start

```go
import "github.com/AlexsanderHamir/PoolX/v2/pool"

// Create and configure the pool
config := pool.NewPoolConfigBuilder[MyObject]().
    SetPoolBasicConfigs(64, 1000, true).  // initial capacity, hard limit, enable channel growth
    Build()

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

obj.DoSomething()
```

## Configuration

PoolX offers extensive configuration options through a builder pattern:

### Basic Configuration

```go
config := pool.NewPoolConfigBuilder[MyObject]().
    SetPoolBasicConfigs(initialCapacity, hardLimit, enableChannelGrowth)
```

### Growth & Shrink Settings

```go
config := pool.NewPoolConfigBuilder[MyObject]().
    // Ring buffer growth
    SetRingBufferGrowthConfigs(thresholdFactor, bigGrowthFactor, controlledGrowthFactor).
    // Ring buffer shrink
    SetRingBufferShrinkConfigs(
        checkInterval, shrinkCooldown, stableUnderutilizationRounds,
        minCapacity, maxConsecutiveShrinks, minUtilizationBeforeShrink, shrinkPercent
    )
```

### Fast Path (L1 Cache) Settings

```go
config := pool.NewPoolConfigBuilder[MyObject]().
    // Basic L1 settings
    SetFastPathBasicConfigs(
        initialSize, growthEventsTrigger, shrinkEventsTrigger,
        fillAggressiveness, refillPercent
    ).
    // Growth behavior
    SetFastPathGrowthConfigs(growthFactor, maxGrowthSize, growthCooldown).
    // Shrink behavior
    SetFastPathShrinkConfigs(
        shrinkFactor, minShrinkSize, shrinkCooldown, utilizationThreshold
    ).
    // Shrink aggressiveness (1-5)
    SetFastPathShrinkAggressiveness(level AggressivenessLevel)
```

### Allocation Strategy

```go
config := pool.NewPoolConfigBuilder[MyObject]().
    SetAllocationStrategy(allocPercent, allocAmount)
```

For detailed configuration options and their effects, see the [API Reference](../pool/api.go).

## Use Cases

PoolX is ideal for:

1. **Resource Limitation**: When you need to restrict object creation to a maximum number
2. **Rapid Cleanup**: When objects must be aggressively removed when no longer needed
3. **Object Caching**: When minimizing creation overhead is priority over memory usage

## Performance

PoolX is designed for resource control rather than raw performance. When used like `sync.Pool`, it will be slower:

```
BenchmarkSyncPoolHighContention         748899              1598 ns/op               4 B/op          0 allocs/op
BenchmarkPoolXHighContention            605977              2023 ns/op              22 B/op          0 allocs/op
```

## Documentation

- **API Reference**: [pool/api.go](../pool/api.go)
- **Technical Details**: [docs/technical_explanations/](technical_explanations/)
- **Architecture**: [docs/ARCHITECTURE.md](ARCHITECTURE.md)
- **Examples**: [pool/code_examples/](../code_examples)
- **FAQ**: [docs/FAQS.md](FAQS.md)

## Best Practices

1. Use pointer types for pooled objects
2. Implement proper cleanup in the cleaner function
3. Ensure proper state initialization in the cloner function
4. Set appropriate initial capacity and hard limits
5. Always call `Close()` when done with the pool

## Contributing

We welcome contributions! See our [Contributing Guidelines](CONTRIBUTING.md) for details on:

- Bug reporting
- Feature requests
- Code contributions
- Documentation improvements
- Testing

## License

MIT License - see [LICENSE](LICENSE) for details.
