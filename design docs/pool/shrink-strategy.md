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

shrink is a long-running goroutine responsible for periodically evaluating
and reducing the pool's capacity under certain conditions.

Shrink Logic Overview:

- The goroutine wakes up every `CheckInterval` to evaluate shrink conditions.
- Shrinking can be triggered by either:
  1. Extended idleness (`IdleThreshold` + `MinIdleBeforeShrink` checks)
  2. Sustained low utilization (`MinUtilizationBeforeShrink` + `StableUnderutilizationRounds`)

Shrink Execution:

- When triggered, the pool reduces its capacity by `ShrinkPercent`.
- It will never shrink below `MinCapacity` or below the number of in-use objects.
- After a successful shrink, a `ShrinkCooldown` prevents immediate consecutive shrinks.

Blocking Mechanism:

- The goroutine tracks `ConsecutiveShrinks`. If the number of consecutive
  shrink operations reaches `MaxConsecutiveShrinks`, it blocks itself using `sync.Cond`.
- While blocked, the goroutine halts all shrink activity and logs the state.
- It remains blocked until another goroutine calls `Get()` and signals the condition.

This approach ensures:

- Shrinks only happen under meaningful idle or underutilized conditions.
- Excessive shrinking is avoided by blocking and requiring real activity to resume.
- The system self-regulates based on both load and inactivity patterns.

---

## 🧩 Future Considerations

- **Hooks Support**: Adding hooks for events like `OnGrow`, `OnShrink`, `OnGet`, and `OnPut` may enhance observability and flexibility in production. This should be tested in high-throughput workloads to determine value before adding complexity.

---
