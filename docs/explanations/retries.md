## Why retry on slowpath ? (get/put)

### Explanation
Even thought we acquire a reader lock, when the concurrency is very high there's a change that we may be reading an old pointer, and the write will return EOF because the old ring buffer was closed, if that happens we retry a couple times to give request a change of reading the current version of the pointer.

### Code Example
```go
func (p *Pool[T]) SlowPath() (obj T, err error) {
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
``
```
