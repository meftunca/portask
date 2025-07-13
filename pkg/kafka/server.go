package kafka

import (
	"fmt"
	"log"
	"net"
	"time"
)

// KafkaServer provides Kafka wire protocol compatibility
type KafkaServer struct {
	addr     string
	handler  *KafkaProtocolHandler
	listener net.Listener
	running  bool
}

// NewKafkaServer creates a new Kafka-compatible server
func NewKafkaServer(addr string, store MessageStore) *KafkaServer {
	// Create simple implementations for demo
	auth := &SimpleAuthProvider{}
	metrics := &SimpleMetricsCollector{}

	handler := NewKafkaProtocolHandler(store, auth, metrics)

	return &KafkaServer{
		addr:    addr,
		handler: handler,
	}
}

// Start starts the Kafka server
func (s *KafkaServer) Start() error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.addr, err)
	}

	s.listener = listener
	s.running = true

	log.Printf("🔗 Kafka-compatible server listening on %s", s.addr)

	go s.acceptConnections()

	return nil
}

// Stop stops the Kafka server
func (s *KafkaServer) Stop() error {
	s.running = false
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// acceptConnections accepts and handles client connections
func (s *KafkaServer) acceptConnections() {
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.running {
				log.Printf("Error accepting connection: %v", err)
			}
			continue
		}

		go s.handler.HandleConnection(conn)
	}
}

// SimpleAuthProvider implements AuthProvider for demo
type SimpleAuthProvider struct{}

func (a *SimpleAuthProvider) Authenticate(mechanism string, username, password string) (*User, error) {
	// For demo, always succeed
	return &User{
		Username: username,
		Groups:   []string{"users"},
	}, nil
}

func (a *SimpleAuthProvider) Authorize(user *User, operation string, resource string) bool {
	// For demo, always allow
	return true
}

// SimpleMetricsCollector implements MetricsCollector for demo
type SimpleMetricsCollector struct{}

func (m *SimpleMetricsCollector) IncrementRequestCount(apiKey int16) {
	// For demo, just log
	log.Printf("API request: %d", apiKey)
}

func (m *SimpleMetricsCollector) RecordRequestLatency(apiKey int16, duration time.Duration) {
	// For demo, just log
	log.Printf("API %d latency: %v", apiKey, duration)
}

func (m *SimpleMetricsCollector) RecordBytesIn(bytes int64) {
	// For demo, just log
	log.Printf("Bytes in: %d", bytes)
}

func (m *SimpleMetricsCollector) RecordBytesOut(bytes int64) {
	// For demo, just log
	log.Printf("Bytes out: %d", bytes)
}
