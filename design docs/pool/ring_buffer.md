# Ring Buffer Documentation

## Overview

The Ring Buffer is a circular buffer implementation that provides a thread-safe, generic data structure for efficient data transfer between concurrent operations.

## Key Features

### Generic Implementation

- Supports any data type through Go generics
- Type-safe operations with compile-time checks

### Thread Safety

- Uses mutex-based synchronization
- Safe for concurrent read and write operations
- Implements condition variables for blocking operations

### Blocking Behavior

The Ring Buffer can operate in two modes:

1. **Non-blocking Mode (Default)**

   - `Write` operations return `ErrIsFull` immediately when the buffer is full
   - `Read` operations return `ErrIsEmpty` immediately when the buffer is empty
   - No waiting for space or data

2. **Blocking Mode**
   - `Write` operations block when the buffer is full, waiting for space to become available
   - `Read` operations block when the buffer is empty, waiting for data to be written
   - Uses condition variables (`sync.Cond`) for efficient waiting
   - Can be configured with timeouts to prevent indefinite blocking

### Timeout Configuration

- Supports configurable timeouts for both read and write operations
- When a timeout occurs, the operation returns `context.DeadlineExceeded`
- Timeouts can be set independently for read and write operations
- Zero or negative timeout values disable timeouts

### Error Handling

The Ring Buffer provides specific error types for different scenarios:

- `ErrIsFull`: Buffer is full (non-blocking mode)
- `ErrIsEmpty`: Buffer is empty (non-blocking mode)
- `ErrWriteOnClosed`: Attempt to write to a closed buffer
- `ErrNilValue`: Attempt to write a nil value
- `ErrTooMuchDataToWrite`: Data size exceeds buffer capacity

### Key Operations

#### Write Operations

- `Write(item T)`: Write a single item
- `WriteMany(items []T)`: Write multiple items
- Both operations respect the blocking mode and timeouts

#### Read Operations

- `GetOne()`: Read a single item
- `GetN(n int)`: Read up to n items
- `PeekOne()`: View the next item without removing it
- `PeekN(n int)`: View up to n items without removing them

#### Buffer Management

- `Length()`: Returns the number of items that can be read
- `Capacity()`: Returns the total buffer size
- `Free()`: Returns the number of items that can be written
- `Reset()`: Clears the buffer
- `ClearBuffer()`: Clears and zeroes all items
- `Close()`: Closes the buffer and cleans up resources

### Configuration

The Ring Buffer can be configured using builder-style methods:

- `WithBlocking(block bool)`: Enable/disable blocking mode
- `WithTimeout(d time.Duration)`: Set both read and write timeouts
- `WithReadTimeout(d time.Duration)`: Set read timeout
- `WithWriteTimeout(d time.Duration)`: Set write timeout
- `WithCancel(ctx context.Context)`: Set cancellation context

## Usage Example

```go
// Create a new ring buffer with size 10
rb := New[int](10)

// Configure as blocking with timeouts
rb.WithBlocking(true).
   WithReadTimeout(5 * time.Second).
   WithWriteTimeout(5 * time.Second)

// Write data
err := rb.Write(42)
if err != nil {
    // Handle error
}

// Read data
item, err := rb.GetOne()
if err != nil {
    // Handle error
}
```

## Best Practices

1. Always check for errors after operations
2. Use appropriate timeouts to prevent deadlocks
3. Consider buffer size based on expected data volume
4. Use `Close()` to properly clean up resources
5. Handle `io.EOF` appropriately when the buffer is closed
