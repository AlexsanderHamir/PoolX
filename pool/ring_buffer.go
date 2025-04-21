package pool

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"
)

var (
	// ErrTooMuchDataToWrite is returned when the data to write is more than the buffer size.
	ErrTooMuchDataToWrite = errors.New("too much data to write")

	// ErrIsFull is returned when the buffer is full and not blocking.
	ErrIsFull = errors.New("ringbuffer is full")

	// ErrIsEmpty is returned when the buffer is empty and not blocking.
	ErrIsEmpty = errors.New("ringbuffer is empty")

	// ErrIsNotEmpty is returned when the buffer is not empty and not blocking.
	ErrIsNotEmpty = errors.New("ringbuffer is not empty")

	// ErrAcquireLock is returned when the lock is not acquired on Try operations.
	ErrAcquireLock = errors.New("unable to acquire lock")

	// ErrWriteOnClosed is returned when write on a closed ringbuffer.
	ErrWriteOnClosed = errors.New("write on closed ringbuffer")

	// ErrReaderClosed is returned when a ReadClosed closed the ringbuffer.
	ErrReaderClosed = errors.New("reader closed")

	// ErrInvalidLength is returned when the length of the buffer is invalid.
	ErrInvalidLength = errors.New("invalid length")

	// ErrNilValue is returned when a nil value is written to the ring buffer.
	ErrNilValue = errors.New("cannot write nil value to ring buffer")
)

// RingBuffer is a circular buffer that implements io.ReaderWriter interface.
// It operates like a buffered pipe, where data is written to a RingBuffer
// and can be read back from another goroutine.
// It is safe to concurrently read and write RingBuffer.
type RingBuffer[T any] struct {
	buf       []T
	size      int
	r         int // next position to read
	w         int // next position to write
	isFull    bool
	err       error
	block     bool
	rTimeout  time.Duration // Applies to writes (waits for the read condition)
	wTimeout  time.Duration // Applies to read (wait for the write condition)
	mu        sync.Mutex
	readCond  *sync.Cond // Signaled when data has been read.
	writeCond *sync.Cond // Signaled when data has been written.
	closed    bool       // Tracks if the writer is closed

	blockedReaders int
	blockedWriters int
}

// New returns a new RingBuffer whose buffer has the given size.
func NewRingBuffer[T any](size int) *RingBuffer[T] {
	if size <= 0 {
		return nil
	}

	return &RingBuffer[T]{
		buf:  make([]T, size),
		size: size,
	}
}

// NewWithConfig creates a new RingBuffer with the given size and configuration.
// It returns an error if the size is less than or equal to 0.
func newRingBufferWithConfig[T any](size int, config *RingBufferConfig) (*RingBuffer[T], error) {
	if size <= 0 {
		return nil, errors.New("size must be greater than 0")
	}

	rb := NewRingBuffer[T](size)

	rb.WithBlocking(config.block)

	if config.rTimeout > 0 {
		rb.WithReadTimeout(config.rTimeout)
	}

	if config.wTimeout > 0 {
		rb.WithWriteTimeout(config.wTimeout)
	}

	return rb, nil
}

// QithBlocking sets the blocking mode of the ring buffer.
// If block is true, Read and Write will block when there is no data to read or no space to write.
// If block is false, Read and Write will return ErrIsEmpty or ErrIsFull immediately.
// By default, the ring buffer is not blocking.
// This setting should be called before any Read or Write operation or after a Reset.
func (r *RingBuffer[T]) WithBlocking(block bool) *RingBuffer[T] {
	r.block = block
	if block {
		r.readCond = sync.NewCond(&r.mu)
		r.writeCond = sync.NewCond(&r.mu)
	}
	return r
}

// WithTimeout will set a blocking read/write timeout.
// If no reads or writes occur within the timeout,
// the ringbuffer will be closed and context.DeadlineExceeded will be returned.
// A timeout of 0 or less will disable timeouts (default).
func (r *RingBuffer[T]) WithTimeout(d time.Duration) *RingBuffer[T] {
	r.mu.Lock()
	r.rTimeout = d
	r.wTimeout = d
	r.mu.Unlock()
	return r
}

// WithReadTimeout will set a blocking read timeout.
// Reads refers to any call that reads data from the buffer.
// If no writes occur within the timeout,
// the ringbuffer will be closed and context.DeadlineExceeded will be returned.
// A timeout of 0 or less will disable timeouts (default).
func (r *RingBuffer[T]) WithReadTimeout(d time.Duration) *RingBuffer[T] {
	r.mu.Lock()
	// Read operations wait for writes to complete,
	// therefore we set the wTimeout.
	r.wTimeout = d
	r.mu.Unlock()
	return r
}

// WithWriteTimeout will set a blocking write timeout.
// Write refers to any call that writes data into the buffer.
// If no reads occur within the timeout,
// the ringbuffer will be closed and context.DeadlineExceeded will be returned.
// A timeout of 0 or less will disable timeouts (default).
func (r *RingBuffer[T]) WithWriteTimeout(d time.Duration) *RingBuffer[T] {
	r.mu.Lock()
	// Write operations wait for reads to complete,
	// therefore we set the rTimeout.
	r.rTimeout = d
	r.mu.Unlock()
	return r
}

func (r *RingBuffer[T]) setErr(err error, locked bool) error {
	if !locked {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	if r.err != nil && r.err != io.EOF {
		return r.err
	}

	switch err {
	// Internal errors are temporary
	case nil, ErrIsEmpty, ErrIsFull, ErrAcquireLock, ErrTooMuchDataToWrite, ErrIsNotEmpty, context.DeadlineExceeded:
		return err
	default:
		r.err = err
		if r.block {
			r.readCond.Broadcast()
			r.writeCond.Broadcast()
		}
	}
	return err
}

func (r *RingBuffer[T]) readErr(locked bool) error {
	if !locked {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	if r.err != nil {
		if r.err == io.EOF {
			if r.w == r.r && !r.isFull {
				return io.EOF
			}
			return nil
		}
		return r.err
	}
	return nil
}

// Returns true if a read may have happened.
// Returns false if waited longer than rTimeout.
// Must be called when locked and returns locked.
func (r *RingBuffer[T]) waitRead() (ok bool) {
	r.blockedWriters++
	if r.rTimeout <= 0 {
		r.readCond.Wait()
		r.blockedWriters--
		return true
	}

	start := time.Now()
	defer time.AfterFunc(r.rTimeout, r.readCond.Broadcast).Stop()

	r.readCond.Wait()
	if time.Since(start) >= r.rTimeout {
		r.setErr(context.DeadlineExceeded, true)
		r.blockedWriters--
		return false
	}

	return true
}

// waitWrite will wait for a write event.
// Returns true if a write may have happened.
// Returns false if waited longer than wTimeout.
// Must be called when locked and returns locked.
func (r *RingBuffer[T]) waitWrite() (ok bool) {
	if r.wTimeout <= 0 {
		r.writeCond.Wait()
		return true
	}

	start := time.Now()
	defer time.AfterFunc(r.wTimeout, r.writeCond.Broadcast).Stop()

	r.writeCond.Wait()
	if time.Since(start) >= r.wTimeout {
		r.setErr(context.DeadlineExceeded, true)
		return false
	}

	return true
}

// Write writes a single item to the buffer.
// Blocks if the buffer is full and the ring buffer is in blocking mode, only a read will unblock it.
func (r *RingBuffer[T]) Write(item T) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return ErrWriteOnClosed
	}

	if r.err != nil {
		return r.err
	}

	if any(item) == nil {
		return ErrNilValue
	}

	if r.isFull {
		if r.block {
			if !r.waitRead() {
				return context.DeadlineExceeded
			}

			if r.isFull {
				return ErrIsFull
			}
		} else {
			return ErrIsFull
		}
	}

	r.buf[r.w] = item
	r.w = (r.w + 1) % r.size
	if r.w == r.r {
		r.isFull = true
	}

	if r.block && r.blockedReaders > 0 {
		r.writeCond.Broadcast()
	}

	return nil
}

// WriteMany writes multiple items to the buffer
func (r *RingBuffer[T]) WriteMany(items []T) (n int, err error) {
	if len(items) == 0 {
		return 0, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.err != nil {
		return 0, r.err
	}

	for i := range items {
		if any(items[i]) == nil {
			return 0, ErrNilValue
		}

		if r.isFull {
			if r.block {
				if !r.waitRead() {
					return n, context.DeadlineExceeded
				}
				if r.isFull {
					return n, ErrIsFull
				}
			} else {
				return n, ErrIsFull
			}
		}

		r.buf[r.w] = items[i]
		r.w = (r.w + 1) % r.size
		if r.w == r.r {
			r.isFull = true
		}
		n++
	}

	if r.block && n > 0 {
		r.writeCond.Broadcast()
	}

	return n, nil
}

// Length returns the number of items that can be read
func (r *RingBuffer[T]) Length() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.w == r.r {
		if r.isFull {
			return r.size
		}
		return 0
	}

	if r.w > r.r {
		return r.w - r.r
	}

	return r.size - r.r + r.w
}

// Capacity returns the size of the underlying buffer
func (r *RingBuffer[T]) Capacity() int {
	return r.size
}

// Free returns the number of items that can be written without blocking.
func (r *RingBuffer[T]) Free() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.w == r.r {
		if r.isFull {
			return 0
		}
		return r.size
	}

	if r.w < r.r {
		return r.r - r.w
	}

	return r.size - r.w + r.r
}

// IsFull returns true when the ringbuffer is full.
func (r *RingBuffer[T]) IsFull() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.isFull
}

// IsEmpty returns true when the ringbuffer is empty.
func (r *RingBuffer[T]) IsEmpty() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return !r.isFull && r.w == r.r
}

// Reset resets the buffer to empty state
func (r *RingBuffer[T]) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	var zero T
	for i := range r.buf {
		r.buf[i] = zero
	}

	r.r = 0
	r.w = 0
	r.isFull = false
}

// PeekOne returns the next item without removing it from the buffer
func (r *RingBuffer[T]) PeekOne() (item T, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.readErr(true); err != nil {
		return item, err
	}

	if r.w == r.r && !r.isFull {
		return item, ErrIsEmpty
	}

	return r.buf[r.r], nil
}

// PeekN returns up to n items without removing them from the buffer
func (r *RingBuffer[T]) PeekN(n int) (items []T, err error) {
	if n <= 0 {
		return nil, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.readErr(true); err != nil {
		return nil, err
	}

	if r.w == r.r && !r.isFull {
		return nil, ErrIsEmpty
	}

	var count int
	if r.w > r.r {
		count = r.w - r.r
	} else {
		count = r.size - r.r + r.w
	}

	if count > n {
		count = n
	}

	items = make([]T, count)
	if r.w > r.r {
		copy(items, r.buf[r.r:r.r+count])
	} else {
		c1 := r.size - r.r
		if c1 >= count {
			copy(items, r.buf[r.r:r.r+count])
		} else {
			copy(items, r.buf[r.r:r.size])
			copy(items[c1:], r.buf[0:count-c1])
		}
	}

	return items, nil
}

// GetOne returns a single item from the buffer.
// Returns ErrIsEmpty if the buffer is empty.
// Blocks if the buffer is empty and the ring buffer is in blocking mode, only a write will unblock it.
func (r *RingBuffer[T]) GetOne() (item T, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.readErr(true); err != nil {
		return item, err
	}

	// if multiple goroutines get unblocked by the condition variable
	// they will all try to read at the same time
	// by using a loop we make sure that only one goroutine will read the item
	// and the others will check if the buffer is empty again.
	for r.w == r.r && !r.isFull {
		if !r.block {
			return item, ErrIsEmpty
		}

		r.blockedReaders++
		if !r.waitWrite() {
			r.blockedReaders--
			return item, context.DeadlineExceeded
		}
		r.blockedReaders--

		if err := r.readErr(true); err != nil {
			return item, err
		}
	}

	item = r.buf[r.r]
	r.r = (r.r + 1) % r.size
	r.isFull = false

	if r.block && r.blockedWriters > 0 {
		r.readCond.Broadcast()
	}

	return item, r.readErr(true)
}

// GetN returns up to n items from the buffer.
// Returns ErrIsEmpty if the buffer is empty.
// The returned slice will have length between 0 and n.
func (r *RingBuffer[T]) GetN(n int) (items []T, err error) { // TODO: check why this is faster than GetAll
	if n <= 0 {
		return nil, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.readErr(true); err != nil {
		return nil, err
	}

	// Keep waiting while the buffer is empty
	for r.w == r.r && !r.isFull {
		if !r.block {
			return nil, ErrIsEmpty
		}
		if !r.waitWrite() {
			return nil, context.DeadlineExceeded
		}

		if err := r.readErr(true); err != nil {
			return nil, err
		}
	}

	var count int
	if r.w > r.r {
		count = r.w - r.r
	} else {
		count = r.size - r.r + r.w
	}

	if count > n {
		count = n
	}

	items = make([]T, count)
	if r.w > r.r {
		copy(items, r.buf[r.r:r.r+count])
	} else {
		c1 := r.size - r.r
		if c1 >= count {
			copy(items, r.buf[r.r:r.r+count])
		} else {
			copy(items, r.buf[r.r:r.size])
			copy(items[c1:], r.buf[0:count-c1])
		}
	}

	r.r = (r.r + count) % r.size
	r.isFull = false

	if r.block {
		r.readCond.Broadcast()
	}

	return items, r.readErr(true)
}

// CopyConfig copies the configuration settings from the source buffer to the target buffer.
// This includes blocking mode, timeouts, and cancellation context.
func (r *RingBuffer[T]) CopyConfig(source *RingBuffer[T]) *RingBuffer[T] {
	r.WithBlocking(source.block)

	if source.rTimeout > 0 {
		r.WithReadTimeout(source.rTimeout)
	}

	if source.wTimeout > 0 {
		r.WithWriteTimeout(source.wTimeout)
	}

	return r
}

// ClearBuffer clears all remaining items in the buffer and sets them to nil.
// This is useful when shrinking the buffer and we need to clean up items that won't fit.
func (r *RingBuffer[T]) clearBuffer() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.w == r.r && !r.isFull {
		return
	}

	var zero T
	if r.w > r.r {
		for i := r.r; i < r.w; i++ {
			r.buf[i] = zero
		}
	} else {
		for i := r.r; i < r.size; i++ {
			r.buf[i] = zero
		}
		for i := range r.w {
			r.buf[i] = zero
		}
	}

	r.r = 0
	r.w = 0
	r.isFull = false
}

// Close closes the ring buffer and cleans up resources.
// After closing, all subsequent operations will return io.EOF.
// Any pending items in the buffer will be cleared.
func (r *RingBuffer[T]) Close() error {
	r.mu.Lock()

	if r.closed {
		return nil
	}

	r.closed = true
	r.setErr(io.EOF, true)

	r.mu.Unlock()
	r.clearBuffer()
	r.mu.Lock()

	if r.block {
		r.readCond.Broadcast()
		r.writeCond.Broadcast()
	}

	r.mu.Unlock()
	return nil
}

func (r *RingBuffer[T]) getAll() (part1, part2 []T, err error) {
	part1, part2, err = r.GetAllView()
	if err != nil {
		return nil, nil, err
	}

	return part1, part2, nil
}

func (r *RingBuffer[T]) GetAllView() (part1, part2 []T, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.readErr(true); err != nil {
		return nil, nil, err
	}

	if r.w == r.r && !r.isFull {
		return nil, nil, nil
	}

	if r.w > r.r {
		part1 = r.buf[r.r:r.w]
	} else {
		part1 = r.buf[r.r:r.size]
		part2 = r.buf[0:r.w]
	}

	r.r = r.w
	r.isFull = false

	if r.block {
		r.readCond.Broadcast()
	}

	return part1, part2, r.readErr(true)
}

func (r *RingBuffer[T]) GetBlockedReaders() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.blockedReaders
}

func (r *RingBuffer[T]) GetBlockedWriters() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.blockedWriters
}
