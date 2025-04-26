# Memory Context

A highly configurable object pool implementation designed to control object creation under high-concurrency scenarios.

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

if err != nil {
    // handle error
}

poolType := reflect.TypeOf(&Example{})

pool, err := pool.NewPool(poolConfig, allocator, cleaner, poolType)
if err != nil {
    panic(err)
}
```

## Configuration

### SetPoolBasicConfigs

- `initialCapacity`: Initial capacity of the ring buffer and fast path. If no capacity is provided for the fast path, it uses the same value.
- `hardLimit`: Maximum number of objects the pool can grow to. The fast path and ring buffer grow together; once the ring buffer reaches the hard limit, the fast path stops growing as well.
- `verbose`: Enables logging for pool activity.
- `enableChannelGrowth`: When disabled, the fast path will not grow even if the ring buffer does.

### SetRingBufferBasicConfigs

- `block`: Whether the ring buffer blocks on reads/writes when there are no objects to read or no space to write.
- `rTimeout`: Read timeout.
- `wTimeout`: Write timeout.
- `bothTimeout`: Timeout for both reads and writes.

### SetRingBufferGrowthConfigs

- `exponentialThresholdFactor`: Threshold at which the growth strategy switches from exponential to fixed; this is a multiplier of the initial capacity.
- `growthPercent`: Percentage increase during exponential growth.
- `fixedGrowthFactor`: Multiplier of initial capacity used during fixed growth after the threshold is reached.

### SetRingBufferShrinkConfigs

- `checkInterval`: Frequency at which the pool checks whether it should shrink.
- `idleThreshold`: Duration of inactivity (since the last get request) before the pool is considered idle and eligible for shrinking.
- `shrinkCooldown`: Minimum time to wait between consecutive shrink operations.
- `minIdleBeforeShrink`: Minimum number of idle checks before shrinking is allowed.
- `minCapacity`: Minimum number of objects the pool can shrink to.
- `maxConsecutiveShrinks`: Maximum number of consecutive shrink operations allowed before further shrinking is blocked. Blocking is lifted by the next get request.
- `minUtilizationBeforeShrink`: Minimum utilization percentage that allows shrinking.
- `stableUnderutilizationRounds`: Number of consecutive underutilization checks required before shrinking.
- `shrinkPercent`: Percentage by which capacity is reduced during shrinking.

### SetFastPathBasicConfigs

- `initialSize`: Initial size of the fast path (L1 cache).
- `growthEventsTrigger`: Number of ring buffer growth events required before the fast path grows.
- `shrinkEventsTrigger`: Number of ring buffer shrink events required before the fast path shrinks.
- `fillAggressiveness`: Degree of aggressiveness when filling the fast path (range: 0.0 to 1.0).
- `refillPercent`: Threshold at which the fast path is refilled, based on emptiness percentage.

### SetFastPathGrowthConfigs

- `exponentialThresholdFactor`: Threshold for switching from exponential to fixed growth in the fast path.
- `fixedGrowthFactor`: Multiplier used for fixed growth.
- `growthPercent`: Percentage by which the fast path grows during exponential growth.

### SetFastPathShrinkConfigs

- `shrinkPercent`: Percentage by which the fast path is shrunk.
- `minCapacity`: Minimum number of objects the fast path can shrink to.

## Pool Behavior

1. Depending on the L1 size, the ring buffer may remain mostly empty unless frequent spills from L1 occur. This indicates that L1 or the ring buffer are not large enough to sustain the load.
2. Object retrieval and return operations first attempt to use L1. Only if L1 is blocked (empty/full) will the ring buffer be accessed.
3. If the ring buffer is non-blocking, it returns `ErrIsEmpty` for reads and `ErrIsFull` for writes. (if it applies)
4. If the ring buffer is set to blocking, it only blocks when the `hardLimit` is reached.
5. When getting an object, the system first attempts to retrieve from L1. If unsuccessful, it tries to refill L1. If that also fails, it falls back to the ring buffer.
6. When returning an object, it attempts to place it in L1. If there are blocked get requests, the object is instead returned to the ring buffer.
7. Although many optimizations are in place, resizing is very expensive—minimize it when possible.
8. The `minUtilizationBeforeShrink` configuration is crucial. When the ring buffer's utilization is at or below this threshold, it becomes eligible for shrinking. The system tries to find the most efficient size that can handle a high request volume without wasting space. Still, remember that resizing is costly.
9. If growth has been blocked due to reaching the `hardLimit`, it will be re-enabled once the pool shrinks below that limit.
10. If the number of consecutive shrinks reaches `maxConsecutiveShrinks`, further shrinking will be blocked until a new get request is received.
11. Keep in mind that objects are reused. For example, with 50,000 requests and only 1,000 objects in blocking mode, those 1,000 objects are rotated across all 50,000 requests.

## Warnings

- Look at the default values before going with them.
- The ring buffer is not on blocking mode by default.
- Only pointers can be stored in the pool (ring buffer / L1), anything else will throw an error.
