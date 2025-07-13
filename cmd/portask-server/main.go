package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/meftunca/portask/pkg/api"
	"github.com/meftunca/portask/pkg/network"
	"github.com/meftunca/portask/pkg/serialization"
	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/storage/dragonfly"
)

// ServerConfig holds configuration for the Portask server
type ServerConfig struct {
	// Network configuration
	TCPPort  int    `json:"tcp_port"`
	HTTPPort int    `json:"http_port"`
	Host     string `json:"host"`

	// Storage configuration
	StorageType string                  `json:"storage_type"`
	Dragonfly   storage.DragonflyConfig `json:"dragonfly"`

	// Performance tuning
	MaxConnections int           `json:"max_connections"`
	ReadTimeout    time.Duration `json:"read_timeout"`
	WriteTimeout   time.Duration `json:"write_timeout"`
	MaxMessageSize int64         `json:"max_message_size"`
}

// DefaultConfig returns default server configuration
func DefaultConfig() *ServerConfig {
	return &ServerConfig{
		TCPPort:  9092,
		HTTPPort: 8080,
		Host:     "0.0.0.0",

		StorageType: "dragonfly",
		Dragonfly: storage.DragonflyConfig{
			Addresses:         []string{"localhost:6379"},
			Username:          "",
			Password:          "",
			DB:                0,
			EnableCluster:     false,
			KeyPrefix:         "portask:",
			EnableCompression: true,
			CompressionLevel:  3,
		},

		MaxConnections: 10000,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		MaxMessageSize: 64 * 1024 * 1024, // 64MB
	}
}

// PortaskServer represents the main Portask server
type PortaskServer struct {
	config     *ServerConfig
	storage    storage.MessageStore
	tcpServer  *network.TCPServer
	httpServer *api.HTTPServer

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
}

// NewPortaskServer creates a new Portask server instance
func NewPortaskServer(config *ServerConfig) (*PortaskServer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	server := &PortaskServer{
		config: config,
		ctx:    ctx,
		cancel: cancel,
	}

	// Initialize storage
	if err := server.initStorage(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Initialize network server
	if err := server.initNetworkServer(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize network server: %w", err)
	}

	// Initialize HTTP API server
	if err := server.initHTTPServer(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize HTTP server: %w", err)
	}

	return server, nil
}

// initStorage initializes the storage backend
func (s *PortaskServer) initStorage() error {
	log.Printf("Initializing %s storage backend...", s.config.StorageType)

	switch s.config.StorageType {
	case "dragonfly":
		store, err := dragonfly.NewDragonflyStore(&s.config.Dragonfly)
		if err != nil {
			return fmt.Errorf("failed to create Dragonfly store: %w", err)
		}

		// Test connection
		ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
		defer cancel()

		if err := store.Ping(ctx); err != nil {
			return fmt.Errorf("failed to connect to Dragonfly: %w", err)
		}

		s.storage = store
		log.Printf("Successfully connected to Dragonfly at %v", s.config.Dragonfly.Addresses)

	default:
		return fmt.Errorf("unsupported storage type: %s", s.config.StorageType)
	}

	return nil
}

// initNetworkServer initializes the TCP server
func (s *PortaskServer) initNetworkServer() error {
	log.Printf("Initializing TCP server on %s:%d...", s.config.Host, s.config.TCPPort)

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.TCPPort)

	// Create a simple codec manager for now
	codecManager := &serialization.CodecManager{}

	// Create protocol handler
	protocolHandler := network.NewPortaskProtocolHandler(codecManager, s.storage)

	// Create TCP server configuration
	serverConfig := &network.ServerConfig{
		Address:         addr,
		MaxConnections:  s.config.MaxConnections,
		ReadTimeout:     s.config.ReadTimeout,
		WriteTimeout:    s.config.WriteTimeout,
		Network:         "tcp",
		KeepAlive:       true,
		NoDelay:         true,
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		WorkerPoolSize:  100,
	}

	// Create TCP server
	s.tcpServer = network.NewTCPServer(serverConfig, protocolHandler)

	log.Printf("TCP server initialized on %s", addr)
	return nil
}

// initHTTPServer initializes the HTTP API server
func (s *PortaskServer) initHTTPServer() error {
	log.Printf("Initializing HTTP API server on %s:%d...", s.config.Host, s.config.HTTPPort)

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.HTTPPort)

	s.httpServer = api.NewHTTPServer(addr, s.tcpServer, s.storage)

	log.Printf("HTTP API server initialized on %s", addr)
	return nil
}

// Start starts all server components
func (s *PortaskServer) Start() error {
	log.Println("Starting Portask server...")

	// Start TCP server
	go func() {
		log.Printf("Starting TCP server on %s:%d", s.config.Host, s.config.TCPPort)
		if err := s.tcpServer.Start(s.ctx); err != nil {
			log.Printf("TCP server error: %v", err)
		}
	}()

	// Wait a moment for TCP server to start
	time.Sleep(100 * time.Millisecond)

	// Start HTTP server
	go func() {
		log.Printf("Starting HTTP API server on %s:%d", s.config.Host, s.config.HTTPPort)
		if err := s.httpServer.Start(); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	log.Println("✅ Portask server started successfully!")
	log.Printf("📡 TCP Protocol: %s:%d", s.config.Host, s.config.TCPPort)
	log.Printf("🌐 HTTP API: http://%s:%d", s.config.Host, s.config.HTTPPort)
	log.Printf("⚡ WebSocket: ws://%s:%d/ws", s.config.Host, s.config.HTTPPort)
	log.Printf("💾 Storage: %s", s.config.StorageType)

	return nil
}

// Stop stops all server components gracefully
func (s *PortaskServer) Stop() error {
	log.Println("Stopping Portask server...")

	// Cancel context to stop all components
	s.cancel()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop HTTP server
	if s.httpServer != nil {
		log.Println("Stopping HTTP server...")
		if err := s.httpServer.Stop(ctx); err != nil {
			log.Printf("Error stopping HTTP server: %v", err)
		}
	}

	// Stop TCP server
	if s.tcpServer != nil {
		log.Println("Stopping TCP server...")
		if err := s.tcpServer.Stop(ctx); err != nil {
			log.Printf("Error stopping TCP server: %v", err)
		}
	}

	// Close storage
	if s.storage != nil {
		log.Println("Closing storage connection...")
		if err := s.storage.Close(); err != nil {
			log.Printf("Error closing storage: %v", err)
		}
	}

	log.Println("✅ Portask server stopped gracefully")
	return nil
}

// GetStats returns comprehensive server statistics
func (s *PortaskServer) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})

	// TCP server stats
	if s.tcpServer != nil {
		stats["tcp"] = s.tcpServer.GetStats()
	}

	// Storage stats
	if s.storage != nil {
		if storageStats, err := s.storage.Stats(context.Background()); err == nil {
			stats["storage"] = storageStats
		}
	}

	// Configuration
	stats["config"] = map[string]interface{}{
		"tcp_port":        s.config.TCPPort,
		"http_port":       s.config.HTTPPort,
		"storage_type":    s.config.StorageType,
		"max_connections": s.config.MaxConnections,
	}

	return stats
}

func main() {
	log.Println("🚀 Starting Portask - High-Performance Message Queue")
	log.Println("Phase 2A: Network Layer & Storage Backend")

	// Load configuration
	config := DefaultConfig()

	// Create server
	server, err := NewPortaskServer(config)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server
	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	// Wait for shutdown signal
	<-sigChan
	log.Println("Received shutdown signal...")

	// Stop server
	if err := server.Stop(); err != nil {
		log.Printf("Error during shutdown: %v", err)
		os.Exit(1)
	}
}
