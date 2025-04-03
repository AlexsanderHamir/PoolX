# 🧠 `PoolConfigBuilder` – Memory Pool

The `PoolConfigBuilder` provides a validated way to configure memory pool behavior for **dynamic object reuse**.

It helps avoid excessive allocations, improve performance, and gives full control over memory behavior.

---

## 🚀 Quick Start

```go
builder := mem.NewPoolConfigBuilder()

config, err := builder.
	SetInitialCapacity(128).
	SetGrowthPercent(0.5).
	SetShrinkAggressiveness(mem.AggressivenessBalanced).
	Build()

if err != nil {
	log.Fatal(err)
}
```

---

## ⚙️ Builder Features

### ✅ Growth Parameters

Control how the pool expands when more objects are needed.

| Method                                           | Description                                                          |
| ------------------------------------------------ | -------------------------------------------------------------------- |
| `SetGrowthExponentialThresholdFactor(f float64)` | Switch from exponential to fixed growth after `InitialCapacity * f`. |
| `SetGrowthPercent(p float64)`                    | Grow by a percentage (e.g. `0.5 = 50%`). Must be `> 0`.              |
| `SetFixedGrowthFactor(f float64)`                | Fixed step multiplier used after the threshold.                      |

### ✅ Shrink Parameters

Control how and when the pool shrinks based on utilization and idleness.

| Method                                               | Description                                                          |
| ---------------------------------------------------- | -------------------------------------------------------------------- |
| `SetShrinkAggressiveness(level AggressivenessLevel)` | Sets auto shrink level (1-5). Applies preset shrink behavior.        |
| `DisableAutoShrink()`                                | Disables auto shrink. Requires all shrink values to be set manually. |
| `SetShrinkCheckInterval(d time.Duration)`            | How often to check if the pool should shrink.                        |
| `SetIdleThreshold(d time.Duration)`                  | Minimum idle duration before shrinking.                              |
| `SetMinIdleBeforeShrink(n int)`                      | Number of consecutive idle checks before shrinking is allowed.       |
| `SetShrinkCooldown(d time.Duration)`                 | Minimum time between two shrink operations.                          |
| `SetMinUtilizationBeforeShrink(v float64)`           | Trigger shrink if utilization is below this. Must be `0 < v <= 1.0`. |
| `SetStableUnderutilizationRounds(n int)`             | Rounds the pool must stay underutilized before shrinking.            |
| `SetShrinkPercent(p float64)`                        | Shrink by a percentage (e.g. `0.25 = shrink 25%`). Must be `<= 1.0`. |
| `SetMinShrinkCapacity(n int)`                        | Defines the lowest allowed capacity after shrinking.                 |

---

## ❗ Important Rules

- You can **change shrink parameters even if auto-shrink is enabled**.
- If you call `DisableAutoShrink()`, you **must manually set all shrink fields**.
- If `DisableAutoShrink()` was called, then calling `SetShrinkAggressiveness()` will panic.
- If `ShrinkPercent` is set to `1.0` (100%), we **do not block it**, but it's your responsibility to deal with it.
- `MinCapacity` must not be greater than `InitialCapacity`.

---

## 🧪 Example: Manual Shrink Configuration

```go
config, err := mem.NewPoolConfigBuilder().
	DisableAutoShrink().
	SetInitialCapacity(64).
	SetShrinkCheckInterval(5 * time.Second).
	SetIdleThreshold(15 * time.Second).
	SetMinIdleBeforeShrink(2).
	SetShrinkCooldown(30 * time.Second).
	SetMinUtilizationBeforeShrink(0.4).
	SetStableUnderutilizationRounds(3).
	SetShrinkStepPercent(0.25).
	SetMinShrinkCapacity(8).
	Build()
```

---

## 🧼 Validation Rules (Applied in `Build()`)

| Field                        | Rule                                                              |
| ---------------------------- | ----------------------------------------------------------------- |
| `InitialCapacity`            | Must be `> 0`                                                     |
| `GrowthPercent`              | Must be `> 0`                                                     |
| `FixedGrowthFactor`          | Must be `> 0`                                                     |
| `ExponentialThresholdFactor` | Must be `> 0`                                                     |
| `ShrinkPercent`              | Must be `> 0` and `<= 1.0`                                        |
| `MinUtilizationBeforeShrink` | Must be `> 0` and `<= 1.0`                                        |
| `MinCapacity`                | Must be `> 0` and `<= InitialCapacity`                            |
| `IdleThreshold`              | Must be `>= CheckInterval`                                        |
| Manual shrink mode           | All shrink values must be manually set if auto shrink is disabled |

---

## 📦 Shrink Aggressiveness Levels

Calling `SetShrinkAggressiveness(level)` applies a pre-defined shrink strategy based on the desired **aggressiveness level**. These levels tune the balance between **responsiveness** and **stability** when shrinking underutilized pools.

| Level | Constant                       | Description                                                           |
| ----- | ------------------------------ | --------------------------------------------------------------------- |
| `0`   | `AggressivenessDisabled`       | Disables auto-shrinking. You must set all shrink parameters manually. |
| `1`   | `AggressivenessConservative`   | Slow, safe shrinking for memory-sensitive applications.               |
| `2`   | `AggressivenessBalanced`       | A good middle-ground for general workloads.                           |
| `3`   | `AggressivenessAggressive`     | Faster shrink, suitable for low-latency systems.                      |
| `4`   | `AggressivenessVeryAggressive` | Highly responsive shrinking.                                          |
| `5`   | `AggressivenessExtreme`        | Shrinks as fast as possible under pressure.                           |

---
