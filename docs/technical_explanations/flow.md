## Get/Put Flow

### Get Method Flow

The `Get` method implements a multi-level caching strategy to efficiently retrieve objects from the pool:

1. **Fast Path (L1 Cache)**

   - First attempts to get an object from the L1 cache (channel-based)
   - Uses non-blocking select operation for immediate access
   - If successful, returns immediately with the object

2. **Refill and Retry**

   - If L1 cache is empty, attempts to refill it from the main pool
   - Uses a semaphore to prevent multiple concurrent refill operations
   - After refill, retries getting from L1 cache

3. **Slow Path (Ring Buffer)**
   - If both L1 attempts fail, falls back to the main pool (ring buffer)
   - Implements retry mechanism for resilience (see retries.md)
   - Uses read locks for thread-safe access
   - May trigger pool growth if capacity is reached

### Put Method Flow

The `Put` method handles returning objects to the pool with a focus on efficient reuse:

1. **Fast Path (L1 Cache)**

   - First attempts to return object to L1 cache
   - Uses non-blocking select operation
   - If successful, updates hit statistics and returns immediately

2. **Slow Path (Ring Buffer)**
   - If L1 cache is full, falls back to main pool (ring buffer)
   - Implements retry mechanism for resilience
   - Uses read locks for thread-safe access
   - Updates miss statistics on successful return
