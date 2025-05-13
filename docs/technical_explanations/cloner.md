## Cloner Function

The cloner function is a performance optimization mechanism that provides an efficient way to create new objects in the pool. Instead of repeatedly calling the allocator function, the cloner performs a shallow copy of a template object, which is significantly faster.

### Key Benefits:

1. **Performance**: Shallow copying is much faster than creating new objects from scratch
2. **Memory Efficiency**: Reuses the template object structure
3. **Flexibility**: Falls back to allocator if no cloner is provided

### Important Considerations:

- For value types (structs without references), the cloner provides optimal performance without any additional setup
- For reference types (pointers, slices, maps), all instances will share the same references by default
  - In such cases, you'll need to initialize the fields that require independent instances
  - This is known as "extreme late initialization" 😂

### Implementation Details:

The pool maintains a template object (created by the allocator during initialization) and uses the cloner to create new instances. This approach avoids the overhead of repeatedly calling the allocator function.

### Example Internal Usage:

```go
func (p *Pool[T]) populateL1OrBuffer(allocAmount int) error {
	fillTarget := p.config.fastPath.initialSize * p.config.fastPath.fillAggressiveness / 100
	fastPathRemaining := fillTarget

	for range allocAmount {
		var obj T
		if p.cloneTemplate != nil {
			obj = p.cloneTemplate(p.template)
		} else {
			obj = p.allocator()
		}

		p.stats.objectsCreated++

		var err error
		fastPathRemaining, err = p.setPoolAndBuffer(obj, fastPathRemaining)
		if err != nil {
			return fmt.Errorf("failed to set pool and buffer: %w", err)
		}
	}
	return nil
}
```
