# PoolX Frequently Asked Questions

## General Questions

### Q: What is PoolX?

A: PoolX is a high-performance object pool implementation for Go that provides efficient object reuse with features like two-level caching, dynamic resizing, and detailed statistics tracking.

### Q: When should I use PoolX?

A: Use PoolX when:

- You need to manage expensive-to-create objects
- Your application has high concurrency requirements
- You want to reduce garbage collection pressure
- You need fine-grained control over object lifecycle
- You want to optimize memory usage

### Q: What types of objects can be pooled?

A: PoolX only works with pointer types. This is because:

- It needs to efficiently manage object lifecycle
- It requires the ability to clean and reset objects
- It needs to maintain object identity
- It optimizes memory usage

## Configuration and Usage

### Q: How do I choose the right initial capacity?

A: Consider:

- Expected concurrent usage
- Object creation cost
- Available memory
- Typical peak load
  Start with a conservative estimate and monitor pool behavior.

### Q: What's the difference between L1 cache and main pool?

A:

- L1 cache (Fast Path): Quick access channel for frequently used objects
- Main pool: Bulk storage using a ring buffer
  The L1 cache reduces contention on the main pool for better performance.

### Q: How do I handle object cleanup?

A: Implement a cleaner function that:

- Resets object state
- Clears references
- Prepares object for reuse
  Example:

```go
cleaner := func(obj *MyObject) {
    obj.Reset()
    obj.Data = nil
    obj.Cache = nil
}
```

### Q: What's the purpose of the cloner function?

A: The cloner function:

- Creates shallow copies of objects
- Delays initialization of reference types
- Helps manage object state
- Reduces allocation overhead

## Performance

### Q: How does PoolX handle high concurrency?

A: PoolX uses multiple mechanisms:

- Two-level caching to reduce contention
- Atomic operations for statistics
- Fine-grained locking
- Channel-based synchronization
- Ring buffer for efficient storage

### Q: When does the pool grow or shrink?

A: Growth occurs when:

- Pool is under pressure
- Utilization is high
- Available objects are low
  Shrink occurs when:
- Pool is underutilized
- Utilization is below threshold
- Cooldown period has passed
- Stable underutilization is detected

### Q: How do I monitor pool performance?

A: Use the built-in statistics:

```go
myPool.PrintPoolStats()
```

Monitor:

- Hit/miss rates
- Growth/shrink events
- Utilization
- Object creation/destruction
- L1 cache effectiveness

## Troubleshooting

### Q: Why am I getting "type T must be a pointer type" error?

A: This error occurs when:

- You're trying to pool non-pointer types
- Your generic type parameter isn't a pointer
  Fix by:
- Using pointer types (e.g., `*MyObject` instead of `MyObject`)
- Ensuring your type parameter is a pointer

### Q: Why is my pool growing too much?

A: Common causes:

- Initial capacity too small
- Growth factors too aggressive
- No hard limit set
  Solutions:
- Increase initial capacity
- Adjust growth factors
- Set appropriate hard limit
- Monitor and tune configuration

### Q: Why is my pool shrinking too aggressively?

A: Common causes:

- Shrink threshold too high
- Cooldown period too short
- Shrink percentage too large
  Solutions:
- Adjust shrink parameters
- Increase cooldown period
- Use less aggressive shrink settings
- Monitor utilization patterns

## Best Practices

### Q: How do I properly implement object cleanup?

A: Best practices:

- Reset all fields to zero values
- Clear all references
- Handle nested objects
- Consider finalization if needed
  Example:

```go
func (obj *MyObject) Reset() {
    obj.ID = 0
    obj.Data = nil
    obj.Cache = make(map[string]interface{})
    obj.Mutex = sync.Mutex{}
}
```

### Q: How do I handle errors from pool operations?

A: Always:

- Check error returns from Get/Put
- Handle pool closure properly
- Implement proper cleanup
- Use defer for Put operations
  Example:

```go
obj, err := pool.Get()
if err != nil {
    // Handle error
    return
}
defer func() {
    if err := pool.Put(obj); err != nil {
        // Handle error
    }
}()
```

### Q: What's the best way to configure the pool?

A: Follow these steps:

1. Start with conservative settings
2. Monitor pool behavior
3. Adjust based on usage patterns
4. Consider memory constraints
5. Use the builder pattern for type-safe configuration

## Advanced Topics

### Q: How does the two-level caching work?

A: The system uses:

- L1 cache (channel) for fast access
- Main pool (ring buffer) for bulk storage
  Objects move between levels based on:
- Usage patterns
- Pool pressure
- Configuration settings

### Q: What's the difference between growth strategies?

A: PoolX offers:

- Exponential growth (below threshold)
- Controlled growth (above threshold)
- Configurable factors for each
- Hard limits for safety

### Q: How do I implement custom allocation strategies?

A: You can:

- Use the allocation strategy configuration
- Implement custom allocator function
- Control pre-allocation percentage
- Set batch allocation amounts

## Contributing

### Q: How can I contribute to PoolX?

A: You can:

- Submit bug reports
- Propose new features
- Improve documentation
- Submit pull requests
- Share your use cases

### Q: What should I consider before contributing?

A: Consider:

- Code quality standards
- Test coverage
- Documentation updates
- Backward compatibility
- Performance implications
