## Overview

The RingBuffer is a circular buffer implementation that provides thread-safe operations for reading and writing data. It's designed to be used as a communication channel between goroutines, particularly in the context of connection pooling.

## Basic Usage

### Creating a Buffer

```go
// Create a new buffer with size 10
buffer := pool.New[int](10)
```

### Configuration Options

#### Blocking Mode

By default, the buffer operates in non-blocking mode. To enable blocking:

```go
buffer.SetBlocking(true)
```

#### Timeouts

You can set timeouts for read and write operations:

```go
// Set both read and write timeouts
buffer.WithTimeout(5 * time.Second)

// Set read timeout only
buffer.WithReadTimeout(5 * time.Second)

// Set write timeout only
buffer.WithWriteTimeout(5 * time.Second)
```

#### Context Cancellation

```go
ctx, cancel := context.WithCancel(context.Background())
buffer.WithCancel(ctx)
```

## Buffer Behavior

### Blocking vs Non-blocking Mode

#### Non-blocking Mode (Default)

- `Write()` returns `ErrIsFull` immediately if the buffer is full
- `GetOne()`/`GetN()` returns `ErrIsEmpty` immediately if the buffer is empty
- No waiting for space or data

#### Blocking Mode

- `Write()` blocks until space becomes available or timeout occurs
- `GetOne()`/`GetN()` blocks until data becomes available or timeout occurs
- Uses condition variables to coordinate between readers and writers

### Error Conditions

1. **Buffer Full**

   - `ErrIsFull`: Returned when trying to write to a full buffer in non-blocking mode
   - `context.DeadlineExceeded`: Returned when write timeout occurs in blocking mode

2. **Buffer Empty**

   - `ErrIsEmpty`: Returned when trying to read from an empty buffer in non-blocking mode
   - `context.DeadlineExceeded`: Returned when read timeout occurs in blocking mode

3. **Closed Buffer**

   - `ErrWriteOnClosed`: Returned when writing to a closed buffer
   - `io.EOF`: Returned when reading from a closed buffer that's empty

4. **Other Errors**
   - `ErrTooMuchDataToWrite`: Returned when trying to write more data than buffer capacity
   - `ErrAcquireLock`: Returned when unable to acquire lock in Try operations

## Usage in Connection Pool

The RingBuffer is used in the pool as part of a two-tier caching system:

1. **L1 Cache (Fast Path)**

   - Uses a buffered channel for immediate access
   - Provides zero-allocation access to frequently used objects
   - Size is dynamically adjusted based on usage patterns

2. **L2 Cache (RingBuffer)**
   - Serves as the main storage for pooled objects
   - Provides blocking/non-blocking access to objects
   - Handles object lifecycle and cleanup

### Pool-Specific Behavior

1. **Object Storage**

   - The RingBuffer stores the main pool of objects
   - Size is determined by the pool's capacity configuration
   - Objects are only stored in the RingBuffer when the L1 cache is full

2. **Object Acquisition**

   - `Get()` first tries the L1 cache (fast path)
   - If L1 is empty, falls back to `GetOne()` from the RingBuffer (slow path)
   - Blocks if no objects are available (in blocking mode)

3. **Object Release**

   - `Put()` first tries to return to L1 cache
   - If L1 is full, falls back to `Write()` to the RingBuffer
   - Blocks if RingBuffer is full (in blocking mode)

4. **Pool Growth/Shrink**
   - The RingBuffer is resized during pool growth/shrink operations
   - Objects are transferred between old and new buffers
   - Configuration (blocking mode, timeouts) is preserved during resize

## Best Practices

1. **Size Selection**

   - Choose buffer size based on expected concurrent connections
   - Too small: May cause unnecessary blocking or unnecessary growth.
   - Too large: May waste memory

2. **Timeout Configuration**

   - Set appropriate timeouts based on application requirements
   - Consider network latency and typical operation duration

3. **Error Handling**

   - Always check for errors after operations
   - Handle timeouts appropriately in your application
   - Clean up resources when errors occur

4. **Context Usage**
   - Use context cancellation for graceful shutdown
   - Propagate context through your application

## Example Usage in Pool

```go
// Create a pool with RingBuffer as L2 cache
p, err := NewPool[Connection](&poolConfig{
    initialCapacity: 100,
    hardLimit: 1000,
    fastPath: &fastPathParameters{
        bufferSize: 10,
        refillPercent: 0.1,
    },
}, allocator, cleaner)

// Get a connection (tries L1 first, then RingBuffer)
conn := p.Get()

// Use connection
// ...

// Return connection (tries L1 first, then RingBuffer)
p.Put(conn)
```
