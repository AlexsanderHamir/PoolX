## Pool Blocking Behavior Explanation

In scenarios where the **fast path is empty**, goroutines fallback to retrieving objects from the **ring buffer**. If **blocking mode** is enabled and the ring buffer is also empty, the goroutine **blocks**, waiting for a writer to return an object.

At **high concurrency**, this can lead to a situation where one or more goroutines remain **indefinitely blocked**, since the preference is to return to the fastpath, not to the ring buffer.

To mitigate this, we wake up any one blocked reader on the ring buffer to attempt to get the returned object from the channel using the pre-read-block hook.

**Pool object return**

```go
if p.tryFastPathPut(obj) {
		p.pool.WakeUpOneReader()
		return nil
	}
```

**Ring buffer hook usage**

```go
rblockAttempts := 1
	for r.w == r.r && !r.isFull {
		if r.preReadBlockHook != nil {
			r.mu.Unlock()
			obj, tryAgain, success := r.preReadBlockHook()
			r.mu.Lock()
			if tryAgain && rblockAttempts > 0 {
				rblockAttempts--
				continue
			}

			if success {
				return obj, nil
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
