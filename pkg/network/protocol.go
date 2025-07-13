package network

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/meftunca/portask/pkg/serialization"
	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/types"
)

// PortaskProtocolHandler implements ConnectionHandler for Portask protocol
type PortaskProtocolHandler struct {
	// Dependencies
	codecManager *serialization.CodecManager
	storage      storage.MessageStore

	// Configuration
	maxMessageSize int64
	enableMetrics  bool

	// Statistics
	totalMessages  int64
	totalErrors    int64
	avgProcessTime time.Duration
}

// PortaskProtocol defines the binary protocol structure
// Message Format:
// [4 bytes] - Magic Number (0x504F5254 = "PORT")
// [1 byte]  - Protocol Version
// [1 byte]  - Message Type
// [2 bytes] - Flags
// [4 bytes] - Payload Length
// [N bytes] - Payload Data
// [4 bytes] - CRC32 Checksum

const (
	// Protocol constants
	ProtocolMagic   = 0x504F5254 // "PORT"
	ProtocolVersion = 0x01
	HeaderSize      = 16 // Magic + Version + Type + Flags + Length + CRC

	// Message types
	MessageTypePublish   = 0x01
	MessageTypeSubscribe = 0x02
	MessageTypeFetch     = 0x03
	MessageTypeAck       = 0x04
	MessageTypeHeartbeat = 0x05
	MessageTypeError     = 0x06
	MessageTypeResponse  = 0x07

	// Flags
	FlagCompressed = 0x01
	FlagEncrypted  = 0x02
	FlagBatch      = 0x04
	FlagPriority   = 0x08

	// Limits
	MaxMessageSize = 64 * 1024 * 1024 // 64MB
)

// ProtocolHeader represents the message header
type ProtocolHeader struct {
	Magic    uint32
	Version  uint8
	Type     uint8
	Flags    uint16
	Length   uint32
	Checksum uint32
}

// ProtocolMessage represents a complete protocol message
type ProtocolMessage struct {
	Header  ProtocolHeader
	Payload []byte
}

// NewPortaskProtocolHandler creates a new protocol handler
func NewPortaskProtocolHandler(codecManager *serialization.CodecManager, storage storage.MessageStore) *PortaskProtocolHandler {
	return &PortaskProtocolHandler{
		codecManager:   codecManager,
		storage:        storage,
		maxMessageSize: MaxMessageSize,
		enableMetrics:  true,
	}
}

// HandleConnection handles a client connection using Portask protocol
func (h *PortaskProtocolHandler) HandleConnection(ctx context.Context, conn *Connection) error {
	reader := bufio.NewReader(conn.conn)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Set read deadline
			conn.conn.SetReadDeadline(time.Now().Add(30 * time.Second))

			// Read and process message
			if err := h.processMessage(ctx, conn, reader); err != nil {
				if err == io.EOF {
					return nil // Client disconnected
				}

				atomic.AddInt64(&h.totalErrors, 1)

				// Send error response
				h.sendErrorResponse(conn, err)

				// Check if error is recoverable
				if isRecoverableError(err) {
					continue
				}

				return err
			}
		}
	}
}

// OnConnect is called when a client connects
func (h *PortaskProtocolHandler) OnConnect(conn *Connection) error {
	// Send welcome message or perform handshake if needed
	return nil
}

// OnDisconnect is called when a client disconnects
func (h *PortaskProtocolHandler) OnDisconnect(conn *Connection) error {
	// Cleanup connection-specific resources
	return nil
}

// OnMessage is called when a message is received (used by HandleConnection)
func (h *PortaskProtocolHandler) OnMessage(conn *Connection, message *types.PortaskMessage) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		h.updateProcessTime(duration)
	}()

	atomic.AddInt64(&h.totalMessages, 1)

	// Store message
	ctx := context.Background()
	if err := h.storage.Store(ctx, message); err != nil {
		return fmt.Errorf("failed to store message: %w", err)
	}

	// Send acknowledgment
	return h.sendAckResponse(conn, message.ID)
}

// processMessage reads and processes a single message
func (h *PortaskProtocolHandler) processMessage(ctx context.Context, conn *Connection, reader *bufio.Reader) error {
	// Read protocol header
	header, err := h.readHeader(reader)
	if err != nil {
		return fmt.Errorf("failed to read header: %w", err)
	}

	// Validate header
	if err := h.validateHeader(header); err != nil {
		return fmt.Errorf("invalid header: %w", err)
	}

	// Read payload
	payload := make([]byte, header.Length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return fmt.Errorf("failed to read payload: %w", err)
	}

	// Validate checksum
	if err := h.validateChecksum(header, payload); err != nil {
		return fmt.Errorf("checksum validation failed: %w", err)
	}

	// Process message based on type
	switch header.Type {
	case MessageTypePublish:
		return h.handlePublish(ctx, conn, header, payload)
	case MessageTypeSubscribe:
		return h.handleSubscribe(ctx, conn, header, payload)
	case MessageTypeFetch:
		return h.handleFetch(ctx, conn, header, payload)
	case MessageTypeHeartbeat:
		return h.handleHeartbeat(ctx, conn)
	default:
		return fmt.Errorf("unsupported message type: %d", header.Type)
	}
}

// readHeader reads the protocol header
func (h *PortaskProtocolHandler) readHeader(reader *bufio.Reader) (*ProtocolHeader, error) {
	headerBytes := make([]byte, HeaderSize)
	if _, err := io.ReadFull(reader, headerBytes); err != nil {
		return nil, err
	}

	header := &ProtocolHeader{
		Magic:    binary.BigEndian.Uint32(headerBytes[0:4]),
		Version:  headerBytes[4],
		Type:     headerBytes[5],
		Flags:    binary.BigEndian.Uint16(headerBytes[6:8]),
		Length:   binary.BigEndian.Uint32(headerBytes[8:12]),
		Checksum: binary.BigEndian.Uint32(headerBytes[12:16]),
	}

	return header, nil
}

// validateHeader validates the protocol header
func (h *PortaskProtocolHandler) validateHeader(header *ProtocolHeader) error {
	if header.Magic != ProtocolMagic {
		return fmt.Errorf("invalid magic number: %x", header.Magic)
	}

	if header.Version != ProtocolVersion {
		return fmt.Errorf("unsupported protocol version: %d", header.Version)
	}

	if header.Length > uint32(h.maxMessageSize) {
		return fmt.Errorf("message too large: %d bytes", header.Length)
	}

	return nil
}

// validateChecksum validates message checksum
func (h *PortaskProtocolHandler) validateChecksum(header *ProtocolHeader, payload []byte) error {
	// Calculate CRC32 of payload
	expectedChecksum := calculateCRC32(payload)
	if header.Checksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %x, got %x", expectedChecksum, header.Checksum)
	}
	return nil
}

// handlePublish handles message publish requests
func (h *PortaskProtocolHandler) handlePublish(ctx context.Context, conn *Connection, header *ProtocolHeader, payload []byte) error {
	// Decompress if needed
	if header.Flags&FlagCompressed != 0 {
		// TODO: Implement decompression
	}

	// Deserialize message
	message, err := h.codecManager.Decode(payload)
	if err != nil {
		return fmt.Errorf("failed to deserialize message: %w", err)
	}

	// Handle the message
	return h.OnMessage(conn, message)
}

// handleSubscribe handles subscription requests
func (h *PortaskProtocolHandler) handleSubscribe(ctx context.Context, conn *Connection, header *ProtocolHeader, payload []byte) error {
	// TODO: Implement subscription logic
	// This would involve registering the connection for specific topics
	return h.sendResponse(conn, MessageTypeResponse, []byte("subscribed"))
}

// handleFetch handles message fetch requests
func (h *PortaskProtocolHandler) handleFetch(ctx context.Context, conn *Connection, header *ProtocolHeader, payload []byte) error {
	// TODO: Parse fetch request (topic, partition, offset, limit)
	// TODO: Fetch messages from storage
	// TODO: Send response with messages
	return h.sendResponse(conn, MessageTypeResponse, []byte("fetch_response"))
}

// handleHeartbeat handles heartbeat messages
func (h *PortaskProtocolHandler) handleHeartbeat(ctx context.Context, conn *Connection) error {
	return h.sendResponse(conn, MessageTypeHeartbeat, []byte("pong"))
}

// sendResponse sends a response message
func (h *PortaskProtocolHandler) sendResponse(conn *Connection, messageType uint8, payload []byte) error {
	header := ProtocolHeader{
		Magic:    ProtocolMagic,
		Version:  ProtocolVersion,
		Type:     messageType,
		Flags:    0,
		Length:   uint32(len(payload)),
		Checksum: calculateCRC32(payload),
	}

	return h.writeMessage(conn, &header, payload)
}

// sendAckResponse sends an acknowledgment response
func (h *PortaskProtocolHandler) sendAckResponse(conn *Connection, messageID types.MessageID) error {
	ackPayload := []byte(messageID)
	return h.sendResponse(conn, MessageTypeAck, ackPayload)
}

// sendErrorResponse sends an error response
func (h *PortaskProtocolHandler) sendErrorResponse(conn *Connection, err error) error {
	errorPayload := []byte(err.Error())
	return h.sendResponse(conn, MessageTypeError, errorPayload)
}

// writeMessage writes a message to the connection
func (h *PortaskProtocolHandler) writeMessage(conn *Connection, header *ProtocolHeader, payload []byte) error {
	// Create header bytes
	headerBytes := make([]byte, HeaderSize)
	binary.BigEndian.PutUint32(headerBytes[0:4], header.Magic)
	headerBytes[4] = header.Version
	headerBytes[5] = header.Type
	binary.BigEndian.PutUint16(headerBytes[6:8], header.Flags)
	binary.BigEndian.PutUint32(headerBytes[8:12], header.Length)
	binary.BigEndian.PutUint32(headerBytes[12:16], header.Checksum)

	// Write header
	if _, err := conn.Write(headerBytes); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write payload
	if len(payload) > 0 {
		if _, err := conn.Write(payload); err != nil {
			return fmt.Errorf("failed to write payload: %w", err)
		}
	}

	return nil
}

// updateProcessTime updates average processing time
func (h *PortaskProtocolHandler) updateProcessTime(duration time.Duration) {
	if h.avgProcessTime == 0 {
		h.avgProcessTime = duration
	} else {
		// Simple moving average
		h.avgProcessTime = (h.avgProcessTime + duration) / 2
	}
}

// GetStats returns protocol handler statistics
func (h *PortaskProtocolHandler) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"total_messages":   atomic.LoadInt64(&h.totalMessages),
		"total_errors":     atomic.LoadInt64(&h.totalErrors),
		"avg_process_time": h.avgProcessTime,
		"max_message_size": h.maxMessageSize,
	}
}

// Helper functions

// calculateCRC32 calculates CRC32 checksum
func calculateCRC32(data []byte) uint32 {
	// TODO: Implement actual CRC32 calculation
	// For now, use simple hash
	var hash uint32
	for _, b := range data {
		hash = hash*31 + uint32(b)
	}
	return hash
}

// isRecoverableError checks if an error is recoverable
func isRecoverableError(err error) bool {
	// TODO: Implement proper error classification
	return true
}
