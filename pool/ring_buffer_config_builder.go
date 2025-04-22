package pool

import (
	"errors"
	"time"
)

type ringBufferConfig struct {
	block    bool
	rTimeout time.Duration
	wTimeout time.Duration
}

// Getter methods for RingBufferConfig
func (c *ringBufferConfig) IsBlocking() bool {
	return c.block
}

func (c *ringBufferConfig) GetReadTimeout() time.Duration {
	return c.rTimeout
}

func (c *ringBufferConfig) GetWriteTimeout() time.Duration {
	return c.wTimeout
}

type ringBufferConfigBuilder struct {
	config *ringBufferConfig
}

func NewRingBufferConfigBuilder() *ringBufferConfigBuilder {
	return &ringBufferConfigBuilder{
		config: defaultRingBufferConfig,
	}
}

// WithBlocking sets whether the RingBuffer should operate in blocking mode.
// If true, Read and Write operations will block when there is no data to read or no space to write.
// If false, Read and Write operations will return ErrIsEmpty or ErrIsFull immediately.
func (b *ringBufferConfigBuilder) WithBlocking(block bool) *ringBufferConfigBuilder {
	b.config.block = block
	return b
}

// WithReadTimeout sets the timeout for read operations.
// If no writes occur within this timeout, the RingBuffer will be closed and context.DeadlineExceeded will be returned.
// A timeout of 0 or less will disable timeouts.
func (b *ringBufferConfigBuilder) WithReadTimeout(d time.Duration) *ringBufferConfigBuilder {
	b.config.rTimeout = d
	return b
}

// WithWriteTimeout sets the timeout for write operations.
// If no reads occur within this timeout, the RingBuffer will be closed and context.DeadlineExceeded will be returned.
// A timeout of 0 or less will disable timeouts.
func (b *ringBufferConfigBuilder) WithWriteTimeout(d time.Duration) *ringBufferConfigBuilder {
	b.config.wTimeout = d
	return b
}

// Build creates a new RingBuffer with the configured settings.
func (b *ringBufferConfigBuilder) Build(size int) (*RingBuffer[any], error) {
	if size <= 0 {
		return nil, errors.New("size must be greater than 0")
	}

	rb := NewRingBuffer[any](size)

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
