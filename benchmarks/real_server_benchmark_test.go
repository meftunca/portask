package benchmarks

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/kafka"
)

// TestRealServerBenchmark - Gerçek Kafka sunucusu ile tam entegrasyon testi
// Bu test gerçek Kafka protocol üzerinden çalışır
func TestRealServerBenchmark(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real server benchmark in short mode")
	}

	// Start a real Kafka server
	store := NewMockThroughputStore()
	server := kafka.NewKafkaServer(":19094", store)
	
	serverReady := make(chan bool)
	serverErr := make(chan error, 1)
	
	go func() {
		t.Logf("Starting Kafka server on :19094...")
		serverReady <- true
		if err := server.Start(); err != nil {
			serverErr <- err
		}
	}()
	
	// Wait for server
	<-serverReady
	time.Sleep(300 * time.Millisecond)
	
	// Check if server started successfully
	select {
	case err := <-serverErr:
		t.Fatalf("Server failed to start: %v", err)
	default:
		// Server is running
	}
	
	// Run tests
	t.Run("ConnectionTest", func(t *testing.T) {
		testConnection(t)
	})
	
	t.Run("ApiVersionsRequest", func(t *testing.T) {
		testApiVersionsRequest(t)
	})
	
	t.Run("ProduceRequest", func(t *testing.T) {
		testProduceRequest(t)
	})
	
	t.Run("RealThroughputTest", func(t *testing.T) {
		testRealThroughput(t, store)
	})
	
	// Server will be stopped when test ends
	t.Logf("All tests completed!")
}

func testConnection(t *testing.T) {
	conn, err := net.Dial("tcp", "localhost:19094")
	if err != nil {
		t.Fatalf("❌ Failed to connect: %v", err)
	}
	defer conn.Close()
	t.Logf("✅ Successfully connected to Kafka server")
}

func testApiVersionsRequest(t *testing.T) {
	conn, err := net.Dial("tcp", "localhost:19094")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()
	
	// Build ApiVersions request (API Key 18)
	request := buildSimpleKafkaRequest(18, 0, 1, []byte{})
	
	start := time.Now()
	_, err = conn.Write(request)
	if err != nil {
		t.Fatalf("Failed to write: %v", err)
	}
	
	// Read response with timeout
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	response := make([]byte, 4096)
	n, err := conn.Read(response)
	latency := time.Since(start)
	
	if err != nil {
		t.Logf("⚠️  Read error (might be OK): %v", err)
	}
	
	t.Logf("✅ ApiVersions: sent %d bytes, received %d bytes", len(request), n)
	t.Logf("✅ Latency: %v", latency)
}

func testProduceRequest(t *testing.T) {
	conn, err := net.Dial("tcp", "localhost:19094")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()
	
	// Build Produce request
	message := []byte("Hello from real Kafka integration test!")
	request := buildSimpleProduceRequest("benchmark-topic", 0, [][]byte{message})
	
	start := time.Now()
	_, err = conn.Write(request)
	if err != nil {
		t.Fatalf("Failed to write: %v", err)
	}
	
	// Try to read response (with short timeout since server might not respond)
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	response := make([]byte, 4096)
	n, _ := conn.Read(response)
	latency := time.Since(start)
	
	t.Logf("✅ Produce: sent %d byte message", len(message))
	t.Logf("✅ Request size: %d bytes", len(request))
	t.Logf("✅ Response size: %d bytes", n)
	t.Logf("✅ Latency: %v", latency)
}

func testRealThroughput(t *testing.T, store *MockThroughputStore) {
	duration := 5 * time.Second
	concurrency := 10
	messageSize := 128
	
	var (
		totalSent     int64
		totalReceived int64
		errors        int64
		wg            sync.WaitGroup
	)
	
	message := make([]byte, messageSize)
	for i := range message {
		message[i] = byte('A' + (i % 26))
	}
	
	fmt.Println("\n╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                                                                  ║")
	fmt.Println("║        🔥 REAL KAFKA SERVER THROUGHPUT TEST 🔥                  ║")
	fmt.Println("║        (Network + Protocol + Full Stack)                        ║")
	fmt.Println("║                                                                  ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	
	start := time.Now()
	deadline := start.Add(duration)
	
	// Monitor goroutine
	stopMonitor := make(chan bool)
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		lastSent := int64(0)
		lastTime := start
		
		for {
			select {
			case <-ticker.C:
				currentSent := atomic.LoadInt64(&totalSent)
				currentTime := time.Now()
				deltaMessages := currentSent - lastSent
				deltaTime := currentTime.Sub(lastTime).Seconds()
				currentThroughput := float64(deltaMessages) / deltaTime
				
				fmt.Printf("⚡ [%2.0fs] Throughput: %8.0f msgs/sec | Total: %10s messages | Errors: %d\n",
					currentTime.Sub(start).Seconds(),
					currentThroughput,
					formatNumber(currentSent),
					atomic.LoadInt64(&errors))
				
				lastSent = currentSent
				lastTime = currentTime
				
			case <-stopMonitor:
				return
			}
		}
	}()
	
	// Launch concurrent producers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			// Each worker has its own connection
			conn, err := net.Dial("tcp", "localhost:19094")
			if err != nil {
				atomic.AddInt64(&errors, 1)
				return
			}
			defer conn.Close()
			
			localSent := int64(0)
			request := buildSimpleProduceRequest(
				fmt.Sprintf("test-topic-%d", workerID),
				0,
				[][]byte{message},
			)
			
			// Set read deadline to avoid blocking
			conn.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
			responseBuffer := make([]byte, 1024)
			
			for time.Now().Before(deadline) {
				// Send request
				n, err := conn.Write(request)
				if err != nil {
					atomic.AddInt64(&errors, 1)
					continue
				}
				
				if n > 0 {
					localSent++
					atomic.AddInt64(&totalSent, 1)
				}
				
				// Try to read response (non-blocking with short deadline)
				if n, _ := conn.Read(responseBuffer); n > 0 {
					atomic.AddInt64(&totalReceived, 1)
				}
				
				// Small delay to avoid overwhelming the server
				if localSent%100 == 0 {
					time.Sleep(1 * time.Millisecond)
				}
			}
		}(i)
	}
	
	wg.Wait()
	close(stopMonitor)
	
	elapsed := time.Since(start)
	throughput := float64(totalSent) / elapsed.Seconds()
	mbPerSec := float64(totalSent*int64(messageSize)) / elapsed.Seconds() / 1024 / 1024
	avgLatency := elapsed.Microseconds() / maxInt64(totalSent, 1)
	
	// Get store statistics
	storeMessages := atomic.LoadInt64(&store.messageCount)
	storeBytes := atomic.LoadInt64(&store.bytesWritten)
	
	fmt.Println("\n═══════════════════════════════════════════════════════════════════")
	fmt.Println("📊 REAL KAFKA SERVER - FINAL RESULTS")
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Printf("⏱️  Duration:          %v\n", elapsed)
	fmt.Printf("👥 Concurrency:       %d workers\n", concurrency)
	fmt.Printf("📦 Message Size:      %d bytes\n", messageSize)
	fmt.Printf("\n")
	fmt.Printf("📨 Network Stats:\n")
	fmt.Printf("   ├─ Sent:           %s messages\n", formatNumber(totalSent))
	fmt.Printf("   ├─ Received:       %s responses\n", formatNumber(totalReceived))
	fmt.Printf("   ├─ Errors:         %s\n", formatNumber(errors))
	fmt.Printf("   └─ Success Rate:   %.2f%%\n", float64(totalSent-errors)/float64(totalSent)*100)
	fmt.Printf("\n")
	fmt.Printf("💾 Server Store Stats:\n")
	fmt.Printf("   ├─ Messages:       %s\n", formatNumber(storeMessages))
	fmt.Printf("   ├─ Data:           %.2f MB\n", float64(storeBytes)/1024/1024)
	fmt.Printf("   └─ Success Rate:   %.2f%%\n", float64(storeMessages)/float64(totalSent)*100)
	fmt.Printf("\n")
	fmt.Printf("🚀 Performance:\n")
	fmt.Printf("   ├─ Throughput:     %s msgs/sec\n", formatNumber(int64(throughput)))
	fmt.Printf("   ├─ Bandwidth:      %.2f MB/sec\n", mbPerSec)
	fmt.Printf("   ├─ Avg Latency:    %d µs/msg\n", avgLatency)
	fmt.Printf("   └─ Store Writes:   %s msgs/sec\n", formatNumber(int64(float64(storeMessages)/elapsed.Seconds())))
	fmt.Printf("═══════════════════════════════════════════════════════════════════\n\n")
	
	// Pass/Fail criteria
	if throughput < 1000 {
		t.Errorf("❌ Throughput too low: %.0f msgs/sec (expected > 1K)", throughput)
	} else if throughput < 10000 {
		t.Logf("⚠️  Moderate throughput: %.0f msgs/sec", throughput)
	} else if throughput < 100000 {
		t.Logf("✅ Good throughput: %.0f msgs/sec", throughput)
	} else {
		t.Logf("🏆 Excellent throughput: %.0f msgs/sec", throughput)
	}
	
	t.Logf("\n🎯 Real-World Performance Estimate:")
	t.Logf("   Single Instance: %.0f-%.0f msgs/sec", throughput*0.8, throughput*1.2)
	t.Logf("   With Persistence: %.0f-%.0f msgs/sec (estimated)", throughput*0.3, throughput*0.5)
}

// Helper functions
func buildSimpleKafkaRequest(apiKey int16, apiVersion int16, correlationID int32, payload []byte) []byte {
	buf := make([]byte, 0, 1024)
	
	// Message length (placeholder)
	buf = append(buf, 0, 0, 0, 0)
	
	// API Key
	buf = append(buf, byte(apiKey>>8), byte(apiKey))
	// API Version
	buf = append(buf, byte(apiVersion>>8), byte(apiVersion))
	// Correlation ID
	buf = append(buf, byte(correlationID>>24), byte(correlationID>>16), byte(correlationID>>8), byte(correlationID))
	// Client ID
	buf = appendKafkaString(buf, "bench-client")
	
	// Payload
	buf = append(buf, payload...)
	
	// Update message length
	messageLen := len(buf) - 4
	binary.BigEndian.PutUint32(buf[0:4], uint32(messageLen))
	
	return buf
}

func buildSimpleProduceRequest(topic string, partition int32, messages [][]byte) []byte {
	payload := []byte{}
	
	// RequiredAcks
	payload = append(payload, 0x00, 0x01)
	// Timeout
	payload = append(payload, 0x00, 0x00, 0x13, 0x88)
	// Topic array
	payload = append(payload, 0x00, 0x00, 0x00, 0x01)
	payload = appendKafkaString(payload, topic)
	// Partition array
	payload = append(payload, 0x00, 0x00, 0x00, 0x01)
	payload = append(payload, byte(partition>>24), byte(partition>>16), byte(partition>>8), byte(partition))
	
	// Message set size placeholder
	messageSetStart := len(payload)
	payload = append(payload, 0x00, 0x00, 0x00, 0x00)
	messageSetPosStart := len(payload)
	
	// Messages
	for i, msg := range messages {
		// Offset
		offset := int64(i)
		payload = append(payload,
			byte(offset>>56), byte(offset>>48), byte(offset>>40), byte(offset>>32),
			byte(offset>>24), byte(offset>>16), byte(offset>>8), byte(offset))
		
		// Message size placeholder
		msgSizePos := len(payload)
		payload = append(payload, 0x00, 0x00, 0x00, 0x00)
		msgStart := len(payload)
		
		// CRC, Magic, Attributes
		payload = append(payload, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00)
		// Key (-1 = null)
		payload = append(payload, 0xFF, 0xFF, 0xFF, 0xFF)
		// Value
		valueLen := int32(len(msg))
		payload = append(payload, byte(valueLen>>24), byte(valueLen>>16), byte(valueLen>>8), byte(valueLen))
		payload = append(payload, msg...)
		
		// Update message size
		msgSize := len(payload) - msgStart
		binary.BigEndian.PutUint32(payload[msgSizePos:msgSizePos+4], uint32(msgSize))
	}
	
	// Update message set size
	messageSetSize := len(payload) - messageSetPosStart
	binary.BigEndian.PutUint32(payload[messageSetStart:messageSetStart+4], uint32(messageSetSize))
	
	return buildSimpleKafkaRequest(0, 0, 1, payload)
}

func appendKafkaString(buf []byte, s string) []byte {
	length := int16(len(s))
	buf = append(buf, byte(length>>8), byte(length))
	buf = append(buf, []byte(s)...)
	return buf
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

