package network

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/meftunca/portask/pkg/types"
)

// Server defines the network server interface
type Server interface {
	// Start starts the server
	Start(ctx context.Context) error

	// Stop stops the server gracefully
	Stop(ctx context.Context) error

	// GetStats returns server statistics
	GetStats() *ServerStats

	// GetAddr returns the server address
	GetAddr() net.Addr
}

// TCPServer implements a high-performance TCP server
type TCPServer struct {
	// Configuration
	config   *ServerConfig
	listener net.Listener
	handler  ConnectionHandler

	// Connection management
	connections sync.Map // map[string]*Connection
	connCount   int64

	// Server state
	running int32
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	// Statistics
	stats *ServerStats

	// Connection pool
	connPool sync.Pool
}

// ServerConfig holds server configuration
type ServerConfig struct {
	// Network settings
	Address string `yaml:"address" json:"address"`
	Port    int    `yaml:"port" json:"port"`
	Network string `yaml:"network" json:"network"` // tcp, tcp4, tcp6

	// Connection settings
	MaxConnections  int           `yaml:"max_connections" json:"max_connections"`
	ReadTimeout     time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout" json:"write_timeout"`
	IdleTimeout     time.Duration `yaml:"idle_timeout" json:"idle_timeout"`
	KeepAlive       bool          `yaml:"keep_alive" json:"keep_alive"`
	KeepAlivePeriod time.Duration `yaml:"keep_alive_period" json:"keep_alive_period"`
	NoDelay         bool          `yaml:"no_delay" json:"no_delay"`

	// Buffer settings
	ReadBufferSize  int `yaml:"read_buffer_size" json:"read_buffer_size"`
	WriteBufferSize int `yaml:"write_buffer_size" json:"write_buffer_size"`

	// Performance settings
	EnableMultiplex bool `yaml:"enable_multiplex" json:"enable_multiplex"`
	WorkerPoolSize  int  `yaml:"worker_pool_size" json:"worker_pool_size"`
	EnableProfiling bool `yaml:"enable_profiling" json:"enable_profiling"`

	// TLS settings (for future)
	EnableTLS bool   `yaml:"enable_tls" json:"enable_tls"`
	CertFile  string `yaml:"cert_file" json:"cert_file"`
	KeyFile   string `yaml:"key_file" json:"key_file"`
}

// ServerStats holds server performance metrics
type ServerStats struct {
	// Connection stats
	TotalConnections    int64 `json:"total_connections"`
	ActiveConnections   int64 `json:"active_connections"`
	AcceptedConnections int64 `json:"accepted_connections"`
	RejectedConnections int64 `json:"rejected_connections"`
	ClosedConnections   int64 `json:"closed_connections"`

	// Traffic stats
	BytesRead        int64 `json:"bytes_read"`
	BytesWritten     int64 `json:"bytes_written"`
	MessagesReceived int64 `json:"messages_received"`
	MessagesSent     int64 `json:"messages_sent"`

	// Performance stats
	AvgConnectionTime time.Duration `json:"avg_connection_time"`
	AvgResponseTime   time.Duration `json:"avg_response_time"`
	RequestsPerSecond float64       `json:"requests_per_second"`

	// Error stats
	TotalErrors   int64 `json:"total_errors"`
	ReadErrors    int64 `json:"read_errors"`
	WriteErrors   int64 `json:"write_errors"`
	TimeoutErrors int64 `json:"timeout_errors"`

	// Server status
	Status       string        `json:"status"`
	StartTime    time.Time     `json:"start_time"`
	Uptime       time.Duration `json:"uptime"`
	LastActivity time.Time     `json:"last_activity"`
}

// ConnectionHandler defines interface for handling connections
type ConnectionHandler interface {
	HandleConnection(ctx context.Context, conn *Connection) error
	OnConnect(conn *Connection) error
	OnDisconnect(conn *Connection) error
	OnMessage(conn *Connection, message *types.PortaskMessage) error
}

// Connection represents a client connection
type Connection struct {
	// Network connection
	conn net.Conn
	id   string
	addr net.Addr

	// Connection state
	connected   int32
	lastActive  time.Time
	connectTime time.Time

	// Buffers
	readBuf  []byte
	writeBuf []byte

	// Statistics
	bytesRead    int64
	bytesWritten int64
	messagesIn   int64
	messagesOut  int64
	errors       int64

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Synchronization
	writeMutex sync.Mutex
	readMutex  sync.Mutex

	// Subscription management
	mu            sync.RWMutex
	subscriptions map[string]bool
}

// NewTCPServer creates a new TCP server
func NewTCPServer(config *ServerConfig, handler ConnectionHandler) *TCPServer {
	ctx, cancel := context.WithCancel(context.Background())

	server := &TCPServer{
		config:  config,
		handler: handler,
		ctx:     ctx,
		cancel:  cancel,
		stats: &ServerStats{
			Status: "initialized",
		},
	}

	// Initialize connection pool
	server.connPool.New = func() interface{} {
		return &Connection{
			readBuf:       make([]byte, config.ReadBufferSize),
			writeBuf:      make([]byte, config.WriteBufferSize),
			subscriptions: make(map[string]bool),
		}
	}

	return server
}

// Start starts the TCP server
func (s *TCPServer) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
		return fmt.Errorf("server already running")
	}

	// Create listener
	address := fmt.Sprintf("%s:%d", s.config.Address, s.config.Port)
	listener, err := net.Listen(s.config.Network, address)
	if err != nil {
		atomic.StoreInt32(&s.running, 0)
		return fmt.Errorf("failed to listen on %s: %w", address, err)
	}

	s.listener = listener
	s.stats.Status = "running"
	s.stats.StartTime = time.Now()

	// Start accepting connections
	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// Stop stops the server gracefully
func (s *TCPServer) Stop(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&s.running, 1, 0) {
		return nil // Already stopped
	}

	s.stats.Status = "stopping"

	// Close listener to stop accepting new connections
	if s.listener != nil {
		s.listener.Close()
	}

	// Cancel context to stop all goroutines
	s.cancel()

	// Close all active connections
	s.connections.Range(func(key, value interface{}) bool {
		if conn, ok := value.(*Connection); ok {
			conn.Close()
		}
		return true
	})

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.stats.Status = "stopped"
		return nil
	case <-ctx.Done():
		s.stats.Status = "force_stopped"
		return ctx.Err()
	}
}

// acceptLoop accepts incoming connections
func (s *TCPServer) acceptLoop() {
	defer s.wg.Done()

	for atomic.LoadInt32(&s.running) == 1 {
		conn, err := s.listener.Accept()
		if err != nil {
			if atomic.LoadInt32(&s.running) == 0 {
				return // Server is stopping
			}

			s.stats.RejectedConnections++
			s.stats.TotalErrors++
			continue
		}

		// Check connection limit
		if s.config.MaxConnections > 0 && atomic.LoadInt64(&s.connCount) >= int64(s.config.MaxConnections) {
			conn.Close()
			s.stats.RejectedConnections++
			continue
		}

		// Create connection wrapper
		connection := s.newConnection(conn)

		// Handle connection in goroutine
		s.wg.Add(1)
		go s.handleConnection(connection)

		s.stats.AcceptedConnections++
		atomic.AddInt64(&s.connCount, 1)
	}
}

// newConnection creates a new connection wrapper
func (s *TCPServer) newConnection(conn net.Conn) *Connection {
	// Get connection from pool or create new one
	connection := s.connPool.Get().(*Connection)

	// Initialize connection
	ctx, cancel := context.WithCancel(s.ctx)
	connection.conn = conn
	connection.id = generateConnectionID()
	connection.addr = conn.RemoteAddr()
	connection.ctx = ctx
	connection.cancel = cancel
	connection.connectTime = time.Now()
	connection.lastActive = time.Now()
	atomic.StoreInt32(&connection.connected, 1)

	// Initialize subscription map
	connection.subscriptions = make(map[string]bool)

	// Configure connection
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		if s.config.KeepAlive {
			tcpConn.SetKeepAlive(true)
			tcpConn.SetKeepAlivePeriod(s.config.KeepAlivePeriod)
		}
		if s.config.NoDelay {
			tcpConn.SetNoDelay(true)
		}
	}

	// Set timeouts
	if s.config.ReadTimeout > 0 {
		conn.SetReadDeadline(time.Now().Add(s.config.ReadTimeout))
	}
	if s.config.WriteTimeout > 0 {
		conn.SetWriteDeadline(time.Now().Add(s.config.WriteTimeout))
	}

	// Store connection
	s.connections.Store(connection.id, connection)

	return connection
}

// handleConnection handles a single connection
func (s *TCPServer) handleConnection(conn *Connection) {
	defer s.wg.Done()
	defer s.closeConnection(conn)

	// Call handler's OnConnect
	if err := s.handler.OnConnect(conn); err != nil {
		s.stats.TotalErrors++
		return
	}

	// Handle connection
	if err := s.handler.HandleConnection(conn.ctx, conn); err != nil {
		s.stats.TotalErrors++
	}

	// Call handler's OnDisconnect
	s.handler.OnDisconnect(conn)
}

// closeConnection closes a connection and cleans up
func (s *TCPServer) closeConnection(conn *Connection) {
	if !atomic.CompareAndSwapInt32(&conn.connected, 1, 0) {
		return // Already closed
	}

	// Close network connection
	conn.conn.Close()

	// Cancel context
	conn.cancel()

	// Remove from connections map
	s.connections.Delete(conn.id)

	// Update stats
	atomic.AddInt64(&s.connCount, -1)
	s.stats.ClosedConnections++

	// Return to pool
	conn.reset()
	s.connPool.Put(conn)
}

// GetStats returns server statistics
func (s *TCPServer) GetStats() *ServerStats {
	stats := *s.stats
	stats.ActiveConnections = atomic.LoadInt64(&s.connCount)
	stats.Uptime = time.Since(stats.StartTime)
	return &stats
}

// GetAddr returns server address
func (s *TCPServer) GetAddr() net.Addr {
	if s.listener != nil {
		return s.listener.Addr()
	}
	return nil
}

// Connection methods

// Close closes the connection
func (c *Connection) Close() error {
	if atomic.CompareAndSwapInt32(&c.connected, 1, 0) {
		c.cancel()
		return c.conn.Close()
	}
	return nil
}

// IsConnected returns connection status
func (c *Connection) IsConnected() bool {
	return atomic.LoadInt32(&c.connected) == 1
}

// Read reads data from connection
func (c *Connection) Read(buf []byte) (int, error) {
	c.readMutex.Lock()
	defer c.readMutex.Unlock()

	n, err := c.conn.Read(buf)
	if n > 0 {
		atomic.AddInt64(&c.bytesRead, int64(n))
		c.lastActive = time.Now()
	}
	if err != nil {
		atomic.AddInt64(&c.errors, 1)
	}
	return n, err
}

// Write writes data to connection
func (c *Connection) Write(buf []byte) (int, error) {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()

	n, err := c.conn.Write(buf)
	if n > 0 {
		atomic.AddInt64(&c.bytesWritten, int64(n))
		c.lastActive = time.Now()
	}
	if err != nil {
		atomic.AddInt64(&c.errors, 1)
	}
	return n, err
}

// reset resets connection for pool reuse
func (c *Connection) reset() {
	c.conn = nil
	c.id = ""
	c.addr = nil
	c.ctx = nil
	c.cancel = nil
	atomic.StoreInt32(&c.connected, 0)
	atomic.StoreInt64(&c.bytesRead, 0)
	atomic.StoreInt64(&c.bytesWritten, 0)
	atomic.StoreInt64(&c.messagesIn, 0)
	atomic.StoreInt64(&c.messagesOut, 0)
	atomic.StoreInt64(&c.errors, 0)

	// Clear subscriptions
	c.mu.Lock()
	c.subscriptions = make(map[string]bool)
	c.mu.Unlock()
}

// generateConnectionID generates a unique connection ID
func generateConnectionID() string {
	// Use ULID or simple timestamp-based ID
	return fmt.Sprintf("conn_%d", time.Now().UnixNano())
}
