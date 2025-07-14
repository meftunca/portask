package amqp

import (
	"fmt"
	"log"
	"net"
	"sync"
)

// Basic AMQP Server for simple messaging
type BasicAMQPServer struct {
	addr     string
	listener net.Listener
	running  bool
	mutex    sync.RWMutex
}

func NewBasicAMQPServer(addr string) *BasicAMQPServer {
	return &BasicAMQPServer{
		addr: addr,
	}
}

func (s *BasicAMQPServer) Start() error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.addr, err)
	}

	s.listener = listener
	s.running = true

	log.Printf("🐰 Basic AMQP Server listening on %s", s.addr)

	for s.running {
		conn, err := listener.Accept()
		if err != nil {
			if s.running {
				log.Printf("Failed to accept connection: %v", err)
			}
			continue
		}

		go s.handleConnection(conn)
	}

	return nil
}

func (s *BasicAMQPServer) handleConnection(conn net.Conn) {
	defer conn.Close()
	log.Printf("📞 New basic AMQP connection from %s", conn.RemoteAddr())
}

func (s *BasicAMQPServer) Stop() error {
	s.running = false
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}
