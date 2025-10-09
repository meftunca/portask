package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

func main() {
	fmt.Println("🧪 Testing Raw Kafka Produce Request")

	// Connect to Kafka port
	conn, err := net.Dial("tcp", "localhost:9092")
	if err != nil {
		fmt.Printf("❌ Failed to connect: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Println("✅ Connected to localhost:9092")

	// Build ApiVersions request first (to test connection)
	apiVersionsReq := buildApiVersionsRequest()
	fmt.Printf("📤 Sending ApiVersions request (%d bytes)...\n", len(apiVersionsReq))

	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err = conn.Write(apiVersionsReq)
	if err != nil {
		fmt.Printf("❌ Failed to send: %v\n", err)
		return
	}

	// Read response
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	respHeader := make([]byte, 4)
	_, err = conn.Read(respHeader)
	if err != nil {
		fmt.Printf("❌ Failed to read response: %v\n", err)
		return
	}

	responseSize := int(binary.BigEndian.Uint32(respHeader))
	fmt.Printf("📥 ApiVersions response size: %d bytes\n", responseSize)

	responseBody := make([]byte, responseSize)
	_, err = conn.Read(responseBody)
	if err != nil {
		fmt.Printf("❌ Failed to read body: %v\n", err)
		return
	}

	fmt.Printf("✅ ApiVersions OK\n\n")

	// Now test Produce request
	produceReq := buildProduceRequest()
	fmt.Printf("📤 Sending Produce request (%d bytes)...\n", len(produceReq))

	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err = conn.Write(produceReq)
	if err != nil {
		fmt.Printf("❌ Failed to send Produce: %v\n", err)
		return
	}

	// Read Produce response
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	prodRespHeader := make([]byte, 4)
	n, err := conn.Read(prodRespHeader)
	if err != nil {
		fmt.Printf("❌ Failed to read Produce response header: %v (read %d bytes)\n", err, n)
		return
	}

	if n != 4 {
		fmt.Printf("❌ Incomplete header: got %d bytes, expected 4\n", n)
		return
	}

	prodResponseSize := int(binary.BigEndian.Uint32(prodRespHeader))
	fmt.Printf("📥 Produce response size: %d bytes\n", prodResponseSize)

	if prodResponseSize == 0 || prodResponseSize > 1024*1024 {
		fmt.Printf("❌ Invalid response size: %d\n", prodResponseSize)
		return
	}

	prodResponseBody := make([]byte, prodResponseSize)
	n, err = conn.Read(prodResponseBody)
	if err != nil {
		fmt.Printf("❌ Failed to read Produce body: %v (read %d/%d bytes)\n", err, n, prodResponseSize)
		return
	}

	fmt.Printf("✅ Produce response received: %d bytes\n", n)
	fmt.Printf("📊 Response body (hex): % x\n", prodResponseBody[:min(n, 100)])

	// Parse response
	if len(prodResponseBody) >= 4 {
		correlationID := int32(binary.BigEndian.Uint32(prodResponseBody[0:4]))
		fmt.Printf("   Correlation ID: %d\n", correlationID)
	}

	if len(prodResponseBody) >= 8 {
		throttleTime := int32(binary.BigEndian.Uint32(prodResponseBody[4:8]))
		fmt.Printf("   Throttle Time: %d ms\n", throttleTime)
	}

	fmt.Println("\n🎉 Produce test completed!")
}

func buildApiVersionsRequest() []byte {
	req := make([]byte, 0, 100)

	// Request header
	size := int32(23)                      // Will be updated
	req = append(req, 0, 0, 0, byte(size)) // message size

	apiKey := int16(18) // ApiVersions
	req = appendInt16(req, apiKey)

	apiVersion := int16(3)
	req = appendInt16(req, apiVersion)

	correlationID := int32(1)
	req = appendInt32(req, correlationID)

	// Client ID (empty)
	req = appendInt16(req, -1)

	// Update size
	binary.BigEndian.PutUint32(req[0:4], uint32(len(req)-4))

	return req
}

func buildProduceRequest() []byte {
	req := make([]byte, 0, 200)

	// Request header (placeholder for size)
	req = append(req, 0, 0, 0, 0)

	// API Key: Produce (0)
	req = appendInt16(req, 0)

	// API Version: 8
	req = appendInt16(req, 8)

	// Correlation ID
	req = appendInt32(req, 2)

	// Client ID: "test-client"
	clientID := "test-client"
	req = appendInt16(req, int16(len(clientID)))
	req = append(req, []byte(clientID)...)

	// ==== REQUEST BODY ====

	// Required Acks: 1 (leader only)
	req = appendInt16(req, 1)

	// Timeout: 10000 ms
	req = appendInt32(req, 10000)

	// Topic Data Array
	topicCount := int32(1)
	req = appendInt32(req, topicCount)

	// Topic: "test-topic"
	topic := "test-topic"
	req = appendInt16(req, int16(len(topic)))
	req = append(req, []byte(topic)...)

	// Partition Data Array
	partitionCount := int32(1)
	req = appendInt32(req, partitionCount)

	// Partition: 0
	req = appendInt32(req, 0)

	// Message Set
	messageSet := buildMessageSet()
	messageSetSize := int32(len(messageSet))
	req = appendInt32(req, messageSetSize)
	req = append(req, messageSet...)

	// Update total size
	binary.BigEndian.PutUint32(req[0:4], uint32(len(req)-4))

	return req
}

func buildMessageSet() []byte {
	msg := make([]byte, 0, 100)

	// Offset: 0
	msg = appendInt64(msg, 0)

	// Message size (placeholder)
	messageSizePos := len(msg)
	msg = appendInt32(msg, 0)

	// CRC (placeholder - will be wrong but OK for test)
	msg = appendInt32(msg, 0)

	// Magic byte: 1 (v1)
	msg = append(msg, 1)

	// Attributes: 0
	msg = append(msg, 0)

	// Timestamp: current time
	msg = appendInt64(msg, time.Now().UnixMilli())

	// Key: null
	msg = appendInt32(msg, -1)

	// Value: "hello kafka"
	value := []byte("hello kafka")
	msg = appendInt32(msg, int32(len(value)))
	msg = append(msg, value...)

	// Update message size
	messageSize := len(msg) - messageSizePos - 4
	binary.BigEndian.PutUint32(msg[messageSizePos:], uint32(messageSize))

	return msg
}

func appendInt16(b []byte, v int16) []byte {
	tmp := make([]byte, 2)
	binary.BigEndian.PutUint16(tmp, uint16(v))
	return append(b, tmp...)
}

func appendInt32(b []byte, v int32) []byte {
	tmp := make([]byte, 4)
	binary.BigEndian.PutUint32(tmp, uint32(v))
	return append(b, tmp...)
}

func appendInt64(b []byte, v int64) []byte {
	tmp := make([]byte, 8)
	binary.BigEndian.PutUint64(tmp, uint64(v))
	return append(b, tmp...)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
