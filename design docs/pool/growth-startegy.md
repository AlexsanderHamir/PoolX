## 🚀 Growth Strategy

The memory pool uses a **hybrid growth strategy** to balance flexibility, performance, and memory efficiency. Growth starts in **exponential mode** and transitions to **fixed-step mode** after reaching a configured threshold.

This strategy is fully customizable through the `PoolGrowthParameters` struct.

---

### ⚙️ Growth Phases

#### 🔹 1. **Exponential Growth**

While the pool is below the threshold (`InitialCapacity * ExponentialThresholdFactor`), it grows by a percentage of its current size:

```go
Growth = CurrentCapacity * GrowthPercent
NewCapacity = CurrentCapacity + Growth
```

This is ideal for rapid scaling in early phases.

---

#### 🔹 2. **Fixed Growth**

Once the pool reaches or exceeds the exponential threshold, it switches to fixed growth:

```go
Growth = InitialCapacity * FixedGrowthFactor
NewCapacity = CurrentCapacity + Growth
```

This phase ensures that pool size doesn’t grow uncontrollably in long-lived or high-load applications.

---

### 🧩 Growth Parameters

| Field                        | Description                                                                                                                                                                                                 |
| ---------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `ExponentialThresholdFactor` | Multiplier applied to `InitialCapacity` to compute the capacity threshold where exponential growth ends and fixed growth begins.<br>**Example:** `InitialCapacity = 12`, `Factor = 4.0` → threshold = `48`. |
| `GrowthPercent`              | Percentage used for growth while in exponential mode. <br>**Example:** `0.5` means the pool grows by 50% of its current capacity.                                                                           |
| `FixedGrowthFactor`          | Multiplier applied to `InitialCapacity` once exponential growth ends.<br>**Example:** `InitialCapacity = 12`, `Factor = 1.0` → each fixed growth step = `12`.                                               |

---

### 🧠 Recommended Settings

| Workload Type      | GrowthPercent | FixedGrowthFactor | Threshold Factor |
| ------------------ | ------------- | ----------------- | ---------------- |
| Fast-scaling apps  | `0.5` – `1.0` | `1.0` – `2.0`     | `4.0`            |
| Memory-constrained | `0.2` – `0.5` | `0.5` – `1.0`     | `2.0` – `3.0`    |
| HFT / Real-time    | `1.0` – `2.0` | `2.0` – `4.0`     | `5.0+`           |

---

### 🧪 Example

```go
builder.
	SetInitialCapacity(64).
	SetGrowthPercent(0.75).
	SetFixedGrowthFactor(1.5).
	SetGrowthExponentialThresholdFactor(4.0)
```

**Resulting behavior:**

- Exponential growth until capacity ≥ `64 * 4 = 256`
- After that, grows by `64 * 1.5 = 96` each time
- Sanity checks are enforced in `.Build()` to prevent extreme configurations

---
