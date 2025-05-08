## Why Retry on the Slow Path (`Get`/`Put`)?

### Overview

Although we acquire a **read lock** (`RLock`) to access the current pool, high concurrency can still cause **stale reads**. The issue arises from the narrow window between:

1. Acquiring the pointer to the internal pool (`p.pool`).
2. Performing the operation (e.g., `GetOne()`, `Write()`).

During this window, a writer holding the full lock might **close or replace** the pool pointer, causing the call to `GetOne()` or `Write()` to fail, typically returning an `EOF` due to the old pool being closed.

To mitigate this, we **retry** the operation a few times, giving the goroutine a chance to access the **latest, valid version** of the pool.

> **Note**: We update both the channel and the ring buffer pointer during resizing. When operations fail due to the channel being closed, the operation falls to the slow path. Thus, retries are only implemented on the slow path as a safeguard.

### Key Point

The **retry mechanism** allows the goroutine to attempt the operation again after a short delay, increasing the likelihood of successfully accessing the updated pool.

---

### Code Example

```go
func (p *Pool[T]) SlowPathGet() (obj T, err error) {
	const maxRetries = 3
	const retryDelay = 10 * time.Millisecond

	// Retry the operation a few times in case of failure
	for i := 0; i < maxRetries; i++ {
		// Acquire the read lock to access the pool
		p.mu.RLock()
		pool := p.pool
		p.mu.RUnlock()

		// Attempt to get an object from the pool
		obj, err = pool.GetOne()
		if err == nil {
			p.stats.totalGets.Add(1)  // Update statistics on successful get
			return obj, nil
		}

		// If not the last retry, wait before trying again
		if i < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}

	// Return the error if all retries fail
	return obj, fmt.Errorf("%w: %w", errRingBufferFailed, err)
}

func (p *Pool[T]) SlowPathPut(obj T) error {
	const maxRetries = 3
	const retryDelay = 10 * time.Millisecond
	var err error

	// Retry the operation a few times in case of failure
	for i := 0; i < maxRetries; i++ {
		// Acquire the read lock to access the pool
		p.mu.RLock()
		pool := p.pool
		p.mu.RUnlock()

		// Attempt to put an object into the pool
		if err = pool.Write(obj); err == nil {
			p.stats.FastReturnMiss.Add(1)  // Update statistics on successful write
			return nil
		}

		// If not the last retry, wait before trying again
		if i < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}

	// Return the error if all retries fail
	return fmt.Errorf("%w: %w", errRingBufferFailed, err)
}
```
