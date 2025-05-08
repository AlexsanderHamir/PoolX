## Overview

In scenarios where the **fast path is empty**, goroutines fallback to retrieving objects from the **ring buffer**. If **blocking mode** is enabled and the ring buffer is also empty, the goroutine **blocks**, waiting for a writer to return an object.

At **high concurrency**, this can lead to a situation where one or more goroutines remain **indefinitely blocked**, waiting for an item that might not arrive — for instance, if the system becomes temporarily imbalanced with more readers than writers.

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

To further improve responsiveness under load, we introduced a **pre-blocking hook** called `preWriteBlockHook`. This hook allows the ring buffer to **attempt to unblock itself** just before actually blocking a reader. This might involve recycling objects from the fast path or performing any custom logic that can unblock waiting readers.

Here’s the implementation of `waitWrite`, showing the use of the hook:

```go
func (r *RingBuffer[T]) waitWrite() (ok bool) {
	r.blockedReaders++

	if r.wTimeout <= 0 {
		// Try hook before blocking
		if r.preWriteBlockHook != nil && r.preWriteBlockHook() {
			log.Println("[WAITWRITE] PreWriteBlockHook returned true")
			r.blockedReaders--
			return true
		}
		r.writeCond.Wait()
		r.blockedReaders--
		return true
	}

	start := time.Now()
	defer time.AfterFunc(r.wTimeout, r.writeCond.Broadcast).Stop()

	// Try hook before blocking
	if r.preWriteBlockHook != nil && r.preWriteBlockHook() {
		log.Println("[WAITWRITE] PreWriteBlockHook returned true")
		r.blockedReaders--
		return true
	}

	r.writeCond.Wait()
	if time.Since(start) >= r.wTimeout {
		// Try hook one last time before giving up
		if r.preWriteBlockHook != nil && r.preWriteBlockHook() {
			log.Println("[WAITWRITE] PreWriteBlockHook returned true")
			r.blockedReaders--
			return true
		}
		r.setErr(context.DeadlineExceeded, true)
		r.blockedReaders--
		return false
	}

	r.blockedReaders--
	return true
}
```

---

### 🔁 Summary

- **Blocked readers** are detected and prioritized via `slowPathPut`.
- **Hooks** are used to avoid unnecessary blocking and allow the system to auto-heal under contention.
- The design aims to **maximize throughput** while avoiding deadlocks or starvation.
- The hooks are used inside the ring buffer.
