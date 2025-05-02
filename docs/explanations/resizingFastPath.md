## ⚠️ FastPath Resize Caveat

This part of the **L1 cache resizing logic** does **not account for objects currently in use**. Since the function is invoked as part of **ring buffer resizing**, it assumes the caller has already acquired the necessary locks to ensure thread safety during this operation.

However, there's an important caveat: the current implementation **blindly drains the old `cacheL1`** channel and transfers available objects into a newly sized one. It does **not track how many objects are currently checked out** (in use by consumers). This creates a subtle risk:

> **If the number of in-use objects exceeds the capacity of the new channel, and they are returned after the resize, the pool relies on `Put` logic to handle potential overflows.**

```go
func (p *Pool[T]) tryL1ResizeIfTriggered() {
	// WARNING: There may be objects in use. We're relying on Put() to handle overflow cases safely.
	newL1 := make(chan T, newCap)
	for {
		select {
		case obj := <-p.cacheL1:
			newL1 <- obj
		default:
			goto done
		}
	}

done:
	p.cacheL1 = newL1
	p.stats.currentL1Capacity.Store(newCap)
	p.stats.lastResizeAtGrowthNum.Store(p.stats.totalGrowthEvents.Load())
}
```

---

### 🧠 Design Assumption

This approach leans on the `Put()` method to **gracefully handle edge cases** such as:

- The resized buffer being at capacity.
- Objects being returned after the resize (possibly from goroutines that held them before resizing).

This keeps the resize logic simple but places responsibility on `Put()` to **avoid memory leaks or dropped objects** during and after resizing.