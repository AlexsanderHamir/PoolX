## ♻️ Shrink Strategy (Idle & Utilization-Based)

The pool supports automatic shrinking through a combination of **idleness detection** and **utilization tracking**. These mechanisms allow the pool to reduce its memory footprint when it's no longer actively or efficiently used.

Shrink behavior is controlled via the `PoolShrinkParameters` configuration. The pool evaluates two primary signals:

### 1. **Idleness-Based Shrinking**

- If no `Get()` calls occur for a specified duration (`IdleThreshold`), the pool is considered idle.
- After a defined number of consecutive idle checks (`MinIdleBeforeShrink`), the pool is marked as shrink-eligible.
- Shrinking occurs unless blocked by a cooldown window (`ShrinkCooldown`).

### 2. **Utilization-Based Shrinking**

- The pool calculates utilization as:  
  `Utilization = ObjectsInUse / CurrentCapacity`
- If utilization remains below a defined threshold (`MinUtilizationBeforeShrink`) for several consecutive checks (`StableUnderutilizationRounds`), a shrink operation may be triggered.
- This ensures shrinking happens only when the pool is genuinely over-allocated and underused.

### Shrinking Behavior

- Once a shrink is triggered, the pool reduces its capacity based on the selected strategy:
  - `"step"`: Shrinks by a percentage (`ShrinkStepPercent`)
  - `"halve"`: Shrinks to 50% of the current capacity
  - `"reset"`: Resets capacity to the minimum (`MinCapacity`)
- Shrinks are never allowed to reduce capacity below `MinCapacity`.

---

## 🧩 Future Considerations

- **Hooks Support**: Adding hooks for events like `OnGrow`, `OnShrink`, `OnGet`, and `OnPut` may enhance observability and flexibility in production. This should be tested in high-throughput workloads to determine value before adding complexity.

- **Library Exposion**: There are many internal parts of the library that are exposed, we should be careful to what we're exposing to the user.

---
