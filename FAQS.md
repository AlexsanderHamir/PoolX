# Frequently Asked Questions (FAQ)

### Q: If the ring buffer has capacity of 128, and so does the fast path, we have 128 objects total, not 256, why?

A: The fastpath uses the ring buffer to refill itself, not to create new objects. It only creates new objects when the ring buffer is empty and needs resizing to fulfill the demand of requests.

### Q: What is the point of the fast path?

A: The ring buffer is fast but requires a mutex. The fast path doesn't need a mutex, which provides better performance. Instead of making 20,000-30,000 requests to the ring buffer, we make less than 1% of that number of requests. We get more objects at once to avoid the mutex overhead.

### Q: What is the point of the ring buffer?

A: Using append and slicing operations are very expensive. The ring buffer provides a more efficient way to get and insert objects.

### Q: Resizing & Growing, how often ?

A: The least possible, growing and shrinking are expensive operations both in CPU and memory, keep it to the least amount possible.

### Q: Ring buffer blocking

A: When set to block, which will happen when the hard limit is reached, no new objects will be created, and all goroutines trying to get objects wil block and wait if none are available, a time out can be set.
