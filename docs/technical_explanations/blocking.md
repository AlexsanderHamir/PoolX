## Pool Blocking Behavior Explanation

In scenarios where the **fast path is empty**, goroutines fallback to retrieving objects from the **ring buffer**. If **blocking mode** is enabled and the ring buffer is also empty, the goroutine **blocks**, waiting for a writer to return an object.

At **high concurrency**, this can lead to a situation where one or more goroutines remain **indefinitely blocked**, since the preference is to return to the fastpath, not to the ring buffer.

To mitigate this, we check if there are any **blocked readers** before deciding how to return an object:

```go
blockedReaders := p.pool.GetBlockedReaders()
if blockedReaders > 0 {
	err := p.slowPathPut(obj)
	if err != nil {
		p.logIfVerbose("[PUT] Error in slowPathPut: %v", err)
	}
	return err
}
```

If blocked readers are detected, we trigger the **slow path**, which attempts to push the object back to the ring buffer to unblock them.

---

## 🛠️ Unblocking with Hooks

To further improve responsiveness under load, we introduced a **pre-blocking hook** called `preReadBlockHook`. This hook allows the ring buffer to **attempt to unblock itself** just before actually blocking a reader. This might involve recycling objects from the fast path or performing any custom logic that can unblock waiting readers.

Here’s the implementation inside `GetOne()`, showing the use of the hook:

```go
rblockAttempts := 1
	for r.w == r.r && !r.isFull {
		if r.preReadBlockHook != nil {
			r.mu.Unlock()
			tryAgain := r.preReadBlockHook()
			r.mu.Lock()
			if tryAgain && rblockAttempts > 0 {
				rblockAttempts--
				continue
			}
		}

		if !r.block {
			return item, errors.ErrIsEmpty
		}

		if !r.waitWrite() {
			return item, context.DeadlineExceeded
		}

		if err := r.readErr(true, false, "GetOne_InnerBlock"); err != nil {
			return item, err
		}
	}
```

---

### 🔁 Summary

- **Blocked readers** are detected and prioritized via `slowPathPut`.
- **Hooks** are used to avoid unnecessary blocking and allow the system to auto-heal under contention.
- The design aims to **maximize throughput** while avoiding deadlocks or starvation.
- The hooks are used inside the ring buffer.
