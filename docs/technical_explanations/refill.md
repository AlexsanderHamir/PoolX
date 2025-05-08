## Refill Logic

### Explanation

Prior to this change, when the function was called, multiple goroutines would attempt to acquire the main lock, effectively queuing up one after the other. This caused contention, as several goroutines tried to refill or grow the pool, but only the first one would succeed in doing so, while the rest were blocked. In such cases, the initial check inside `handleRefillScenarios()` would return because the L1 cache still had objects. The issue was the unnecessary contention for the lock.

Now, only one goroutine attempts to refill, while the others block and wait for the refill process to complete.

### Code

```go
func (p *Pool[T]) tryRefillAndGetL1() (zero T, canProceed bool) {
	select {
	case p.refillSemaphore <- struct{}{}:
		defer func() {
			// Broadcast to unblock waiting goroutines after refill is done
			p.refillCond.Broadcast()
			// Release the semaphore once done
			<-p.refillSemaphore
		}()
		// Handle refill scenarios and return the result
		obj, canProceed := p.handleRefillScenarios()
		return obj, canProceed
	default:
		// Block until a refill is done if the semaphore is already taken
		p.refillMu.Lock()
		p.refillCond.Wait()
		p.refillMu.Unlock()

		// Try to get from L1 cache after waiting
		if obj, found := p.tryGetFromL1(false); found {
			return obj, true
		}

		// Return zero if no object is found
		return zero, false
	}
}
```

---

### Key Improvements:

* Only one goroutine will attempt to refill the pool, while the others will wait.
* The `Broadcast` ensures that waiting goroutines wake up after the refill process is done.
* Semaphore is used to ensure that only one goroutine acquires the lock for refilling at any given time.

This revision improves efficiency by minimizing contention and ensures that only one goroutine refills the pool, reducing unnecessary operations.
