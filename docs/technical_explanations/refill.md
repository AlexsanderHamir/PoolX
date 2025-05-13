## Refill Logic

### Explanation

Previously, when the refill logic was triggered, multiple goroutines would attempt to acquire the main lock, lining up one after another. This led to contention, as several goroutines tried to refill or grow the pool concurrently—but only the first would succeed. The rest, upon acquiring the lock, would quickly return after observing that the pool had been refilled, making their attempts redundant and inefficient.

With the updated design, only **one** goroutine performs the refill, while the others **wait** for the refill to complete. This removes contention on the refill path and eliminates redundant refill attempts.

### Code

```go
func (p *Pool[T]) tryRefillAndGetL1() (zero T, canProceed bool) {
	select {
	case p.refillSemaphore <- struct{}{}:
		defer func() {
			// Notify all waiting goroutines once the refill is complete
			p.refillCond.Broadcast()
			// Release the semaphore
			<-p.refillSemaphore
		}()
		// Perform the refill logic and return the result
		obj, canProceed := p.handleRefillScenarios()
		return obj, canProceed
	default:
		// Wait for the ongoing refill to complete
		p.refillMu.Lock()
		p.refillCond.Wait()
		p.refillMu.Unlock()

		// Retry from the L1 cache after waiting
		if obj, found := p.tryGetFromL1(false); found {
			return obj, true
		}

		// No object was available even after refill
		return zero, false
	}
}
```

---

### Key Improvements

* Only **one** goroutine is allowed to initiate the refill; others wait for the result.
* `Broadcast` wakes up all waiting goroutines once the refill is completed.
* A **semaphore** ensures mutual exclusion for the refill path, without blocking unnecessarily on a mutex.

This update significantly reduces contention by preventing multiple redundant refill attempts, improving throughput and making object access more efficient under high concurrency.
