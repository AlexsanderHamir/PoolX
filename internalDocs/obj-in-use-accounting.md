## Drawbacks

Atomic operations are safe when used individually on a single value, but combining them can lead to race conditions. While you don't need a lock to increase the value, you do need one when decreasing it to avoid potential underflow or inconsistent state.

## Pros

The advantage of using atomic operations is that you can avoid locking every operation. In this case, by locking only this function, we eliminated the race condition that occurred 8 out of 10 times.

```go
func (p *Pool[T]) reduceObjectsInUse() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for {
		old := p.stats.objectsInUse.Load()
		if old == 0 {
			break
		}

		if p.stats.objectsInUse.CompareAndSwap(old, old-1) {
			break
		}
	}
}
```
