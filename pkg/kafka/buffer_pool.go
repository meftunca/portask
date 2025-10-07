package kafka

import (
	"sync"
)

// BufferPool manages pooled byte buffers for reduced allocations
type BufferPool struct {
	small  *sync.Pool // 128 bytes
	medium *sync.Pool // 4KB
	large  *sync.Pool // 64KB
	xlarge *sync.Pool // 256KB
}

// Global buffer pool instance
var globalBufferPool = NewBufferPool()

// NewBufferPool creates a new buffer pool with different size classes
func NewBufferPool() *BufferPool {
	return &BufferPool{
		small: &sync.Pool{
			New: func() interface{} {
				buf := make([]byte, 128)
				return &buf
			},
		},
		medium: &sync.Pool{
			New: func() interface{} {
				buf := make([]byte, 4096)
				return &buf
			},
		},
		large: &sync.Pool{
			New: func() interface{} {
				buf := make([]byte, 65536)
				return &buf
			},
		},
		xlarge: &sync.Pool{
			New: func() interface{} {
				buf := make([]byte, 262144)
				return &buf
			},
		},
	}
}

// Get retrieves a buffer of appropriate size from the pool
func (p *BufferPool) Get(size int) *[]byte {
	switch {
	case size <= 128:
		return p.small.Get().(*[]byte)
	case size <= 4096:
		return p.medium.Get().(*[]byte)
	case size <= 65536:
		return p.large.Get().(*[]byte)
	default:
		return p.xlarge.Get().(*[]byte)
	}
}

// Put returns a buffer to the pool
func (p *BufferPool) Put(buf *[]byte) {
	if buf == nil {
		return
	}
	
	size := cap(*buf)
	switch {
	case size == 128:
		p.small.Put(buf)
	case size == 4096:
		p.medium.Put(buf)
	case size == 65536:
		p.large.Put(buf)
	case size == 262144:
		p.xlarge.Put(buf)
	// If size doesn't match, let it be garbage collected
	}
}

// GetBuffer gets a buffer from the global pool
func GetBuffer(size int) *[]byte {
	return globalBufferPool.Get(size)
}

// PutBuffer returns a buffer to the global pool
func PutBuffer(buf *[]byte) {
	globalBufferPool.Put(buf)
}

// MessageBuffer is a reusable buffer for message handling
type MessageBuffer struct {
	data []byte
	pool *BufferPool
}

// NewMessageBuffer creates a new message buffer with the specified size
func NewMessageBuffer(size int) *MessageBuffer {
	buf := globalBufferPool.Get(size)
	return &MessageBuffer{
		data: (*buf)[:0], // Reset length but keep capacity
		pool: globalBufferPool,
	}
}

// Bytes returns the underlying byte slice
func (mb *MessageBuffer) Bytes() []byte {
	return mb.data
}

// Reset resets the buffer for reuse
func (mb *MessageBuffer) Reset() {
	mb.data = mb.data[:0]
}

// Write appends data to the buffer
func (mb *MessageBuffer) Write(p []byte) (n int, err error) {
	mb.data = append(mb.data, p...)
	return len(p), nil
}

// Len returns the current length
func (mb *MessageBuffer) Len() int {
	return len(mb.data)
}

// Cap returns the capacity
func (mb *MessageBuffer) Cap() int {
	return cap(mb.data)
}

// Release returns the buffer to the pool
func (mb *MessageBuffer) Release() {
	if mb.pool != nil {
		mb.pool.Put(&mb.data)
	}
}

