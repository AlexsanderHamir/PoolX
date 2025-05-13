## Cloner Function

To optimize object creation, the cloner function is used instead of the allocator(if provided). The cloner performs a shallow copy of the object, which is more efficient than repeatedly calling the allocator. However, if the object contains reference types, all instances will, by default, share the same reference. In such cases, you'll need to initialize the fields that need to be independent—this is extreme late initialization.

If your struct doesn’t contain any reference types, you'll enjoy the full performance benefits without requiring additional initialization later on.

### Example Internal Usage:

```go
func (p *Pool[T]) populateL1OrBuffer(allocAmount int) error {
	fillTarget := p.config.fastPath.initialSize * p.config.fastPath.fillAggressiveness / 100
	fastPathRemaining := fillTarget

	for range allocAmount {
        // The allocator provides the template object, which is stored on the Pool struct,
        // to avoid calling it everytime we need a templat eobject.
		// Use the cloner to create a shallow copy of the template object
		obj := p.cloneTemplate(p.template)
		p.stats.objectsCreated++

		// Set the object in the pool and buffer
		var err error
		fastPathRemaining, err = p.setPoolAndBuffer(obj, fastPathRemaining)
		if err != nil {
			return fmt.Errorf("failed to set pool and buffer: %w", err)
		}
	}
	return nil
}
```
