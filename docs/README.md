# PoolX

[![GoDoc](https://pkg.go.dev/badge/github.com/username/project)](https://pkg.go.dev/github.com/AlexsanderHamir/PoolX/v2)
[![Go Report Card](https://goreportcard.com/badge/github.com/AlexsanderHamir/PoolX)](https://goreportcard.com/report/github.com/AlexsanderHamir/PoolX)
[![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![CI](https://github.com/AlexsanderHamir/PoolX/actions/workflows/test.yml/badge.svg)](https://github.com/AlexsanderHamir/PoolX/actions/workflows/test.yml)

A highly configurable object pool implementation designed to control object creation under high-concurrency scenarios.

> **Documentation**:
>
> - **API Reference**: For detailed API reference and interface definitions, see [pool/api.go](../pool/api.go)
> - **Technical Details**: For in-depth technical explanations and implementation details, see [docs/technical_explanations/](technical_explanations/)
> - **Design**: For design decisions, architecture overview, and component interactions, see [docs/design_docs.md](design_docs.md)
> - **Code Examples**: For practical usage examples and implementation patterns, see [pool/code_examples/](../code_examples)

![Flow](../assets/flow.png)

## Installation

```bash
go get github.com/AlexsanderHamir/PoolX/v2
```

## Quick Start

```go
import (
    "github.com/AlexsanderHamir/PoolX/v2/pool/code_examples/configs"
    "time"
)

// Define your object type
type Example struct {
    ID   int
    Name string
    Data []byte
}

// Create a new pool configuration using one of the preset configs
// Don't use this in production, use it as inspiration.
config := configs.CreateHighThroughputConfig()

// Create allocator and cleaner functions
allocator := func() *Example {
    return &Example{}
}

cleaner := func(obj *Example) {
    obj.ID = 0
    obj.Name = ""
    obj.Data = nil
}

// If provided, this function replaces the allocator. Useful when allocation is expensive.
// The example below performs a shallow copy.
// If the struct contains reference types, you'll need to manually reinitialize them as needed,
// since all instances will share the same underlying references.
cloner := func(obj *configs.Example) *configs.Example {
			dst := *obj
			return &dst
}

// Create and use the pool
pool, err := pool.NewPool(config, allocator, cleaner, cloner)
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
- **Allocation Strategy**: Controls how objects are allocated in bulk
  - Configurable allocation percentage
  - Batch size control for efficient allocation

## Example custom configuration:

```go
config := NewPoolConfigBuilder[Example]().
    SetPoolBasicConfigs(1000, 10000, true).
    SetRingBufferBasicConfigs(true, 100*time.Millisecond, 100*time.Millisecond, 0).
    SetRingBufferGrowthConfigs(3000, 0.75, 0.5).
    SetRingBufferShrinkConfigs(
        5*time.Second,    // check interval
        1*time.Second,    // cooldown
        3,                // stable rounds
        32,               // min capacity
        3,                // max consecutive shrinks
        20,               // min utilization
        25,               // shrink percent
    ).
    SetFastPathBasicConfigs(64, 3, 3, 80, 20).
    SetFastPathGrowthConfigs(3000, 0.75, 0.5).
    SetFastPathShrinkConfigs(25, 32).
    SetAllocationStrategy(80, 10).
    Build()
```

## Important Notes

1. **Type Safety**: Only pointers can be stored in the pool
2. **Default Behavior**: Ring buffer operates in blocking mode by default
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
