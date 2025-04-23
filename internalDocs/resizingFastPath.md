This part of the L1 resizing logic doesn't account for objects currently in use. Since this function is called during ring buffer resizing, we already hold a lock while performing the operation. However, the code assumes that the Put function will handle cases where the buffer is full or inaccessible, rather than handling those scenarios explicitly here.

```go
func (p *Pool[T]) tryL1ResizeIfTriggered() {
	// WARNING: There may be objects in use, we're counting on the PUT to not drop objects.
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