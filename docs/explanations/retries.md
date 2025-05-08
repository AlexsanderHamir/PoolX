## Why Retry on the Slow Path (`Get`/`Put`)?

### Overview

Even though we acquire a **read lock** (`RLock`) to access the current pool, high concurrency can still lead to **stale reads**. Specifically, there’s a narrow race window between:

1. Acquiring the pointer to the internal pool (`p.pool`), and
2. Performing the operation (e.g., `GetOne()`, `Write()`).

During this window, the pool might have been **closed or replaced** by a writer holding the full lock. As a result, the call to `GetOne()` may fail — commonly returning an `EOF` due to the old pool being closed.

To handle this gracefully, we **retry** the operation a few times. This gives the goroutine a chance to observe the **latest, valid version** of the pool.

---

### Code Example

```go
func (p *Pool[T]) SlowPathGet() (obj T, err error) {
	const maxRetries = 3
	const retryDelay = 10 * time.Millisecond

	for i := range maxRetries {
		p.mu.RLock()
		pool := p.pool
		p.mu.RUnlock()

		obj, err = pool.GetOne()
		if err == nil {
			p.stats.totalGets.Add(1)
			return obj, nil
		}

		if i < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}

	return obj, fmt.Errorf("%w: %w", errRingBufferFailed, err)
}

func (p *Pool[T]) slowPathPut(obj T) error {
	const maxRetries = 3
	const retryDelay = 10 * time.Millisecond
	var err error

	for i := range maxRetries {
		p.mu.RLock()
		pool := p.pool
		p.mu.RUnlock()

		if err = pool.Write(obj); err == nil {
			p.stats.FastReturnMiss.Add(1)
			return nil
		}

		if i < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}

	return fmt.Errorf("%w: %w", errRingBufferFailed, err)
}

```
