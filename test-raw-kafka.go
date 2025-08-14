// Raw TCP-based Kafka client for debugging ProduceAPI
package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"
)

func main() {
	log.Println("🔗 Testing raw Kafka ProduceAPI to Portask...")

	// Connect to Kafka server
	conn, err := net.Dial("tcp", "localhost:9092")
	if err != nil {
		log.Fatalf("❌ Failed to connect: %v", err)
	}
	defer conn.Close()

	log.Println("✅ TCP connection established")

	// Send ApiVersions request first (API 18)
	if err := sendApiVersionsRequest(conn); err != nil {
		log.Fatalf("❌ ApiVersions failed: %v", err)
	}

	// Send Metadata request (API 3)
	if err := sendMetadataRequest(conn); err != nil {
		log.Fatalf("❌ Metadata failed: %v", err)
	}

	// Send Produce request (API 0)
	if err := sendProduceRequest(conn); err != nil {
		log.Fatalf("❌ Produce failed: %v", err)
	}

	log.Println("✅ All requests completed successfully!")
}

func sendApiVersionsRequest(conn net.Conn) error {
	log.Println("📤 Sending ApiVersions request (API 18)")

	// Build ApiVersions request
	request := []byte{}

	// API key (18), version (0), correlation ID (1)
	request = append(request, 0, 18)      // API key = 18
	request = append(request, 0, 0)       // Version = 0
	request = append(request, 0, 0, 0, 1) // Correlation ID = 1

	// Client ID (empty string)
	request = append(request, 0, 0) // Client ID length = 0

	// Add message size header (4 bytes)
	message := make([]byte, 4+len(request))
	binary.BigEndian.PutUint32(message[0:4], uint32(len(request)))
	copy(message[4:], request)

	// Send request
	if _, err := conn.Write(message); err != nil {
		return err
	}

	// Read response
	return readResponse(conn, "ApiVersions")
}

func sendMetadataRequest(conn net.Conn) error {
	log.Println("📤 Sending Metadata request (API 3)")

	// Build Metadata request
	request := []byte{}

	// API key (3), version (0), correlation ID (2)
	request = append(request, 0, 3)       // API key = 3
	request = append(request, 0, 0)       // Version = 0
	request = append(request, 0, 0, 0, 2) // Correlation ID = 2

	// Client ID (empty string)
	request = append(request, 0, 0) // Client ID length = 0

	// Topics array (request all topics: -1)
	request = append(request, 255, 255, 255, 255) // Topic count = -1 (all topics)

	// Add message size header
	message := make([]byte, 4+len(request))
	binary.BigEndian.PutUint32(message[0:4], uint32(len(request)))
	copy(message[4:], request)

	// Send request
	if _, err := conn.Write(message); err != nil {
		return err
	}

	// Read response
	return readResponse(conn, "Metadata")
}

func sendProduceRequest(conn net.Conn) error {
	log.Println("📤 Sending Produce request (API 0)")

	// Build Produce request
	request := []byte{}

	// API key (0), version (0), correlation ID (3)
	request = append(request, 0, 0)       // API key = 0
	request = append(request, 0, 0)       // Version = 0
	request = append(request, 0, 0, 0, 3) // Correlation ID = 3

	// Client ID (empty string)
	request = append(request, 0, 0) // Client ID length = 0

	// Required acks (1)
	request = append(request, 0, 1) // Required acks = 1

	// Timeout (5000ms)
	request = append(request, 0, 0, 19, 136) // Timeout = 5000

	// Topic data
	request = append(request, 0, 0, 0, 1) // Topic count = 1

	// Topic name: "test-topic"
	topicName := "test-topic"
	request = append(request, 0, byte(len(topicName))) // Topic name length
	request = append(request, []byte(topicName)...)    // Topic name

	// Partition data
	request = append(request, 0, 0, 0, 1) // Partition count = 1

	// Partition number (0)
	request = append(request, 0, 0, 0, 0) // Partition = 0

	// Message set
	messageValue := "Hello from raw Kafka client!"
	messageSet := buildMessageSet(messageValue)

	// Message set size
	request = append(request, 0, 0, 0, byte(len(messageSet))) // Message set size

	// Message set content
	request = append(request, messageSet...)

	// Add message size header
	message := make([]byte, 4+len(request))
	binary.BigEndian.PutUint32(message[0:4], uint32(len(request)))
	copy(message[4:], request)

	log.Printf("🔍 Sending Produce request: %d bytes total", len(message))

	// Send request
	if _, err := conn.Write(message); err != nil {
		return err
	}

	// Read response
	return readResponse(conn, "Produce")
}

func buildMessageSet(value string) []byte {
	// Simple message format (not compressed)
	message := []byte{}

	// Offset (8 bytes) = 0
	message = append(message, 0, 0, 0, 0, 0, 0, 0, 0)

	// Message size (will be calculated)
	messageSizePos := len(message)
	message = append(message, 0, 0, 0, 0) // Placeholder for message size

	messageStart := len(message)

	// CRC (4 bytes) = 0 (simplified)
	message = append(message, 0, 0, 0, 0)

	// Magic byte (1 byte) = 0
	message = append(message, 0)

	// Attributes (1 byte) = 0
	message = append(message, 0)

	// Key length = -1 (no key)
	message = append(message, 255, 255, 255, 255)

	// Value length
	message = append(message, 0, 0, 0, byte(len(value)))

	// Value
	message = append(message, []byte(value)...)

	// Update message size
	messageSize := len(message) - messageStart
	binary.BigEndian.PutUint32(message[messageSizePos:messageSizePos+4], uint32(messageSize))

	return message
}

func readResponse(conn net.Conn, requestType string) error {
	// Set read timeout
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// Read response size
	sizeBytes := make([]byte, 4)
	if _, err := conn.Read(sizeBytes); err != nil {
		return fmt.Errorf("failed to read %s response size: %v", requestType, err)
	}

	size := binary.BigEndian.Uint32(sizeBytes)
	log.Printf("📥 %s response size: %d bytes", requestType, size)

	// Read response body
	response := make([]byte, size)
	if _, err := conn.Read(response); err != nil {
		return fmt.Errorf("failed to read %s response body: %v", requestType, err)
	}

	log.Printf("✅ %s response received successfully", requestType)
	return nil
}
