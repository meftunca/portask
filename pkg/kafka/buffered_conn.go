package kafka

import (
	"bufio"
	"net"
	"sync"
	"time"
)

const (
	// Buffer sizes for optimized I/O
	ReadBufferSize  = 128 * 1024 // 128KB read buffer
	WriteBufferSize = 64 * 1024  // 64KB write buffer
	FlushInterval   = 1 * time.Millisecond
)

// BufferedConn wraps a net.Conn with buffered I/O for better performance
type BufferedConn struct {
	conn         net.Conn
	reader       *bufio.Reader
	writer       *bufio.Writer
	flushTicker  *time.Ticker
	flushDone    chan struct{}
	closeOnce    sync.Once
	closed       bool
	mu           sync.Mutex
	lastActivity time.Time
}

// NewBufferedConn creates a new buffered connection with optimized buffer sizes
func NewBufferedConn(conn net.Conn) *BufferedConn {
	// Set TCP options for better performance
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		// Increase OS-level buffers
		tcpConn.SetReadBuffer(ReadBufferSize)
		tcpConn.SetWriteBuffer(WriteBufferSize)
		
		// Disable Nagle's algorithm for lower latency
		// Set to false to enable Nagle for batching (trade latency for throughput)
		tcpConn.SetNoDelay(true)
		
		// Enable TCP keepalive
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	bc := &BufferedConn{
		conn:         conn,
		reader:       bufio.NewReaderSize(conn, ReadBufferSize),
		writer:       bufio.NewWriterSize(conn, WriteBufferSize),
		flushTicker:  time.NewTicker(FlushInterval),
		flushDone:    make(chan struct{}),
		lastActivity: time.Now(),
	}

	// Start auto-flush goroutine for periodic flushing
	go bc.autoFlush()

	return bc
}

// autoFlush periodically flushes the write buffer
func (bc *BufferedConn) autoFlush() {
	for {
		select {
		case <-bc.flushTicker.C:
			bc.mu.Lock()
			if !bc.closed && bc.writer.Buffered() > 0 {
				bc.writer.Flush()
			}
			bc.mu.Unlock()
		case <-bc.flushDone:
			return
		}
	}
}

// Read reads data from the buffered connection
func (bc *BufferedConn) Read(b []byte) (n int, err error) {
	bc.mu.Lock()
	bc.lastActivity = time.Now()
	bc.mu.Unlock()
	return bc.reader.Read(b)
}

// Write writes data to the buffered connection
func (bc *BufferedConn) Write(b []byte) (n int, err error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	
	if bc.closed {
		return 0, net.ErrClosed
	}
	
	bc.lastActivity = time.Now()
	n, err = bc.writer.Write(b)
	
	// Flush if buffer is getting full (> 75%)
	if bc.writer.Buffered() > WriteBufferSize*3/4 {
		bc.writer.Flush()
	}
	
	return n, err
}

// Flush explicitly flushes the write buffer
func (bc *BufferedConn) Flush() error {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	
	if bc.closed {
		return net.ErrClosed
	}
	
	return bc.writer.Flush()
}

// Close closes the connection and stops the auto-flush goroutine
func (bc *BufferedConn) Close() error {
	var err error
	bc.closeOnce.Do(func() {
		bc.mu.Lock()
		bc.closed = true
		bc.mu.Unlock()
		
		// Stop auto-flush
		close(bc.flushDone)
		bc.flushTicker.Stop()
		
		// Final flush
		bc.writer.Flush()
		
		// Close underlying connection
		err = bc.conn.Close()
	})
	return err
}

// RemoteAddr returns the remote network address
func (bc *BufferedConn) RemoteAddr() net.Addr {
	return bc.conn.RemoteAddr()
}

// LocalAddr returns the local network address
func (bc *BufferedConn) LocalAddr() net.Addr {
	return bc.conn.LocalAddr()
}

// SetDeadline sets the read and write deadlines
func (bc *BufferedConn) SetDeadline(t time.Time) error {
	return bc.conn.SetDeadline(t)
}

// SetReadDeadline sets the read deadline
func (bc *BufferedConn) SetReadDeadline(t time.Time) error {
	return bc.conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline
func (bc *BufferedConn) SetWriteDeadline(t time.Time) error {
	return bc.conn.SetWriteDeadline(t)
}

// Buffered returns the number of bytes buffered for writing
func (bc *BufferedConn) Buffered() int {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return bc.writer.Buffered()
}

// LastActivity returns the time of last read or write activity
func (bc *BufferedConn) LastActivity() time.Time {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return bc.lastActivity
}

