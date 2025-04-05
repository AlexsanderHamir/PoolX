# Why We Switched from Interfaces to Generics

## Background

In the earlier version of our pool implementation, we used `interface{}` (aka `any`) to support storing arbitrary types. While this provided flexibility, it introduced several notable downsides:

- Required unsafe type assertions (e.g., `obj.(*MyType)`)
- Introduced runtime overhead from boxing and heap allocations
- Reduced code readability and maintainability
- Missed the benefits of compile-time type checking

With the introduction of **generics in Go 1.18**, we were able to address these issues while maintaining — and even improving — flexibility and performance.

---

## Advantages of Using Generics

### ✅ Type Safety at Compile Time

Generics allow us to define the pool using a type parameter like `pool[T any]`, ensuring that every operation on the pool is type-checked by the compiler. This eliminates the need for manual type assertions and catches errors early:

```go
pool := NewPool[*MyStruct](...)
obj := pool.Get()      // obj is already *MyStruct
pool.Put(obj)          // No casting required
```

### ✅ Improved Performance

Interfaces in Go often require values to be boxed (i.e., allocated on the heap) to fit into the `interface{}` abstraction. Generics avoid this by allowing the compiler to work directly with concrete types, leading to:

- Fewer allocations
- Better cache performance
- Reduced garbage collection pressure

### ✅ Cleaner, More Intuitive API

By removing the need for type assertions and conversions, the API becomes more straightforward. Developers interact with the pool using clear, strongly typed operations — making it easier to reason about and harder to misuse.

### ✅ Enhanced IDE Support

Generics also improve tooling and developer experience:

- IDEs can provide accurate autocompletion and type hints
- Refactoring becomes safer and easier
- Errors are caught at compile time instead of at runtime

---
