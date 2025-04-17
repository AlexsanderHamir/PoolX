package pool

import (
	"errors"
	"time"
)

type RingBufferConfig struct {
	block    bool
	rTimeout time.Duration
	wTimeout time.Duration
}

// Getter methods for RingBufferConfig
func (c *RingBufferConfig) IsBlocking() bool {
	return c.block
}

func (c *RingBufferConfig) GetReadTimeout() time.Duration {
	return c.rTimeout
}

func (c *RingBufferConfig) GetWriteTimeout() time.Duration {
	return c.wTimeout
}

type RingBufferConfigBuilder struct {
	config *RingBufferConfig
}

func NewRingBufferConfigBuilder() *RingBufferConfigBuilder {
	return &RingBufferConfigBuilder{
		config: defaultRingBufferConfig,
	}
}

// WithBlocking sets whether the RingBuffer should operate in blocking mode.
// If true, Read and Write operations will block when there is no data to read or no space to write.
// If false, Read and Write operations will return ErrIsEmpty or ErrIsFull immediately.
func (b *RingBufferConfigBuilder) WithBlocking(block bool) *RingBufferConfigBuilder {
	b.config.block = block
	return b
}

// WithReadTimeout sets the timeout for read operations.
// If no writes occur within this timeout, the RingBuffer will be closed and context.DeadlineExceeded will be returned.
// A timeout of 0 or less will disable timeouts.
func (b *RingBufferConfigBuilder) WithReadTimeout(d time.Duration) *RingBufferConfigBuilder {
	b.config.rTimeout = d
	return b
}

// WithWriteTimeout sets the timeout for write operations.
// If no reads occur within this timeout, the RingBuffer will be closed and context.DeadlineExceeded will be returned.
// A timeout of 0 or less will disable timeouts.
func (b *RingBufferConfigBuilder) WithWriteTimeout(d time.Duration) *RingBufferConfigBuilder {
	b.config.wTimeout = d
	return b
}

// Build creates a new RingBuffer with the configured settings.
func (b *RingBufferConfigBuilder) Build(size int) (*RingBuffer[any], error) {
	if size <= 0 {
		return nil, errors.New("size must be greater than 0")
	}

	rb := New[any](size)

	if b.config.block {
		rb.WithBlocking(true)
	}

	if b.config.rTimeout > 0 {
		rb.WithReadTimeout(b.config.rTimeout)
	}

	if b.config.wTimeout > 0 {
		rb.WithWriteTimeout(b.config.wTimeout)
	}

	return rb, nil
}
