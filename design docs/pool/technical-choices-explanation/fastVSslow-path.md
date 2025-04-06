### 🧠 Object Access Hierarchy

The pool is designed with a 3-layer hierarchy for optimized access and reuse — similar to a CPU memory hierarchy:

| Layer         | Description                              |
| ------------- | ---------------------------------------- |
| **Fast Path** | Lock-free channel for hot object reuse   |
| **Main Pool** | Shared slice protected by a mutex        |
| **Allocator** | Fallback mechanism to create new objects |

## Design Issues
- At the initialization, we're filling the fastPath and leaving the pool empty, and we may be constantly leving the pool empty