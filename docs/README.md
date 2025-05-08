# PoolX

[![GoDoc](https://pkg.go.dev/badge/github.com/username/project)](https://pkg.go.dev/github.com/AlexsanderHamir/PoolX)
[![Go Report Card](https://goreportcard.com/badge/github.com/AlexsanderHamir/PoolX)](https://goreportcard.com/report/github.com/AlexsanderHamir/PoolX)
[![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![CI](https://github.com/AlexsanderHamir/PoolX/actions/workflows/test.yml/badge.svg)](https://github.com/AlexsanderHamir/PoolX/actions/workflows/test.yml)

A highly configurable object pool implementation designed to control object creation under high-concurrency scenarios. PoolX provides fine-grained control over object lifecycle, memory management, and performance characteristics.

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
    // Basic pool configuration
    SetPoolBasicConfigs(128, 5000, false, true, true).
    // Ring buffer configuration
    SetRingBufferBasicConfigs(true, time.Second*5, time.Second*5, time.Second*5).
    SetRingBufferGrowthConfigs(150.0, 0.85, 1.2).
    SetRingBufferShrinkConfigs(
        time.Second*2,    // check interval
        time.Second*5,    // idle threshold
        time.Second*1,    // shrink cooldown
        2,                // min idle before shrink
        5,                // stable underutilization rounds
        15,               // min capacity
        3,                // max consecutive shrinks
        0.3,              // min utilization before shrink
        0.5,              // shrink percent
    ).
    // Fast path (L1 cache) configuration
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

- **Ring Buffer**: Automatically grows and shrinks based on usage patterns
  - Configurable growth thresholds and rates
  - Smart shrinking based on utilization metrics
  - Customizable cooldown periods and stability requirements
- **Fast Path (L1 Cache)**: Provides quick access to frequently used objects
  - Independent growth and shrink policies
  - Configurable event triggers for resizing
  - Automatic refill based on utilization

### 2. Configuration Options

#### Pool Settings

- Initial capacity and hard limits
- Channel growth control

#### Ring Buffer Configuration

- Blocking/non-blocking operations
- Read/write timeouts
- Growth parameters:
  - Exponential threshold factor
  - Growth percentage
  - Fixed growth factor
- Shrink parameters:
  - Check intervals
  - Idle thresholds
  - Utilization requirements
  - Capacity limits

#### Fast Path Configuration

- Initial size and capacity limits
- Growth triggers and rates
- Shrink policies
- Refill behavior
- Channel growth control

For a complete list of all available configuration options and their detailed descriptions, please check the [API file](../pool/api.go) in the repository.

## Common Use Cases

### High-Concurrency Web Server

```go
config, _ := pool.NewPoolConfigBuilder().
    SetPoolBasicConfigs(1000, 10000, false, true, true).
    SetRingBufferBasicConfigs(true, time.Second*5, time.Second*5, time.Second*5).
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
    SetFastPathGrowthConfigs(150.0, 1.5, 0.85).
    SetFastPathShrinkConfigs(0.7, 10).
    Build()
```

### Background Job Processor

```go
config, _ := pool.NewPoolConfigBuilder().
    SetPoolBasicConfigs(100, 1000, false, true, true).
    SetRingBufferBasicConfigs(true, time.Second*10, time.Second*10, time.Second*10).
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
    SetFastPathGrowthConfigs(100.0, 1.2, 0.90).
    SetFastPathShrinkConfigs(0.5, 5).
    Build()
```

## Important Notes

1. **Type Safety**: Only pointers can be stored in the pool
2. **Default Behavior**: Ring buffer operates in non-blocking mode by default
3. **Performance**: Resizing operations are expensive - tune growth/shrink parameters carefully
4. **Configuration**: Use `EnforceCustomConfig()` when you need precise control over all fields of the shrinking struct.
5. **Aggressiveness Levels**: Use `SetShrinkAggressiveness()` for preset configurations (levels 1-5)

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `go test ./...`
5. Submit a pull request

For bug reports and feature requests, please open a GitHub Issue.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
