## sync.Pool vs Poolx

BenchmarkPoolX-8 145224 8261 ns/op 0 B/op 0 allocs/op
BenchmarkSyncPool-8 140463 8099 ns/op 0 B/op 0 allocs/op

### 🔄 `sync.Pool` is a _latency-optimized_, GC-integrated cache.

- It shines in _tight, short-lived object reuse cycles_, like encoding/decoding or per-request buffers that live/die quickly.
- If you **get/put immediately**, it benefits from **per-P local storage** and avoids locks or cross-P contention almost entirely.
- But it's not designed to retain objects across **longer or delayed lifetimes** — they get GC’d aggressively.

---

### 🚀 `PoolX` excels under async-heavy conditions.

PoolX wins when:

- There's **a non-trivial delay** between `Get` and `Put` (e.g., network round-trip, I/O wait).
- Objects are **not returned fast enough** for `sync.Pool` to take advantage of its local fast path.
- You need **more control**: capacity, eviction, custom refill logic — which `sync.Pool` simply doesn’t expose.

---

### 📌 Why `sync.Pool` Fails with Workload Delay

Here’s what happens under the hood:

1. Each `P` has a local fast-path pool.
2. If the object is returned _too late_ (after a GC), the object gets collected.
3. Delays like `Sleep`, I/O, or large computations **break the temporal locality**.
4. As a result, `sync.Pool` may miss reuse opportunities and start allocating again.

The workload:

```go
time.Sleep(time.Microsecond * 50)
```

...is _just enough_ to disrupt sync.Pool’s GC-sensitive reuse pattern.

---

### 💡 Recommendation

- **Use `sync.Pool`** when:

  - You’re reusing within tight CPU-bound loops.
  - Lifetime between `Get`/`Put` is microseconds, and there’s little chance of GC.
  - You don’t care about size, preallocation, or fine-grained control.

- **Use `PoolX`** when:

  - Objects are "in-flight" longer.
  - You want fine-tuned behavior.
