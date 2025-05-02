# PoolX

[![GoDoc](https://pkg.go.dev/badge/github.com/username/project)](https://pkg.go.dev/github.com/AlexsanderHamir/PoolX)
[![Go Report Card](https://goreportcard.com/badge/github.com/AlexsanderHamir/PoolX)](https://goreportcard.com/report/github.com/AlexsanderHamir/PoolX)
[![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![CI](https://github.com/AlexsanderHamir/PoolX/actions/workflows/test.yml/badge.svg)](https://github.com/AlexsanderHamir/PoolX/actions/workflows/test.yml)

A highly configurable object pool implementation designed to control object creation under high-concurrency scenarios.

![Flow](../assets/flow.png)

## Installation

```bash
go get github.com/AlexsanderHamir/PoolX
```

## Quick Start

```go
import (
    "github.com/AlexsanderHamir/PoolX/src/pool"
    "time"
)

// Create a new pool configuration
config, err := pool.NewPoolConfigBuilder().
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

if err != nil {
    panic(err)
}

// Define your object type
type Example struct {
    ID   int
    Name string
}

// Create allocator and cleaner functions
allocator := func() *Example {
    return &Example{}
}

cleaner := func(obj *Example) {
    obj.ID = 0
    obj.Name = ""
}

// Create and use the pool
pool, err := pool.NewPool(config, allocator, cleaner)
if err != nil {
    panic(err)
}
```

## Key Features

### 1. Dynamic Resizing

- **Ring Buffer**: Automatically grows and shrinks based on usage patterns (applies to ring buffer and channel L1)
- **Fast Path (L1 Cache)**: Provides quick access to frequently used objects
- **Smart Growth**: Exponential growth until threshold, then fixed growth
- **Efficient Shrinking**: Reduces size when pool is underutilized

### 2. Performance Optimizations

- Non-blocking operations by default
- Configurable timeouts for blocking operations
- Automatic L1 cache management
- Memory-efficient object reuse

### 3. Configuration Options

- **Pool Settings**: Initial capacity, hard limits, verbosity
- **Growth Control**: Thresholds, growth rates, limits
- **Shrink Control**: Idle detection, utilization thresholds
- **Fast Path**: Size, growth triggers, refill behavior

## Common Use Cases

### High-Concurrency Web Server

```go
config, _ := pool.NewPoolConfigBuilder().
    SetPoolBasicConfigs(1000, 10000, false, true).
    SetRingBufferGrowthConfigs(200.0, 0.85, 1.2).
    SetRingBufferShrinkConfigs(
        time.Second*5,
        time.Second*30,
        time.Second*10,
        3,
        1000,
        5,
        5,
        0.3,
        0.5,
    ).
    SetFastPathBasicConfigs(256, 4, 4, 0.8, 0.30).
    Build()
```

### Background Job Processor

```go
config, _ := pool.NewPoolConfigBuilder().
    SetPoolBasicConfigs(100, 1000, false, true).
    SetRingBufferGrowthConfigs(150.0, 0.50, 1.1).
    SetRingBufferShrinkConfigs(
        time.Second*10,
        time.Second*60,
        time.Second*20,
        5,
        100,
        3,
        10,
        0.2,
        0.7,
    ).
    SetFastPathBasicConfigs(64, 2, 2, 0.5, 0.50).
    Build()
```

## Important Notes

1. **Type Safety**: Only pointers can be stored in the pool
2. **Default Behavior**: Ring buffer operates in non-blocking mode by default
3. **Performance**: Resizing operations are expensive - tune growth/shrink parameters carefully

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `go test ./...`
5. Submit a pull request

For bug reports and feature requests, please open a GitHub Issue.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
