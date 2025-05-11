## 🌀 Put Mechanism and Reader Unblocking Behavior

### 🔍 Observation

When many goroutines are **concurrently active and hold objects for extended durations**, the `Put` mechanism in the pool increasingly favors **unblocking waiting readers** over replenishing the fast path.

### ⚙️ Mechanism

* If a `Put()` detects that one or more goroutines are **blocked on `Get()`**, it attempts to **directly hand off** the object to a waiting reader.
* This bypasses the **fast path cache** entirely, as priority is given to minimizing reader latency.
* As the number of blocked readers increases, more `Put()` operations are diverted to this **slow path**.

### 🚨 Consequence

* The **fast path (local cache)** becomes underutilized or completely bypassed.
* Contention increases as `Put()` operations synchronize with blocked `Get()` calls.
* Under sustained load, the pool effectively **devolves into a blocking queue**, negating the latency benefits of fast path reuse.

### 📌 Summary

> The more readers are blocked, the more likely `Put()` will bypass the fast path and go to the slow path to unblock them — leading to cascading contention and degraded throughput.
