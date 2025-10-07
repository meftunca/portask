package benchmarks

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/kafka"
)

// Mock message store for throughput testing
type MockThroughputStore struct {
	mu            sync.RWMutex
	messageCount  int64
	requestCount  int64
	bytesWritten  int64
	topics        map[string]bool
}

func NewMockThroughputStore() *MockThroughputStore {
	return &MockThroughputStore{
		topics: make(map[string]bool),
	}
}

// ProduceMessage implements kafka.MessageStore
func (m *MockThroughputStore) ProduceMessage(topic string, partition int32, key, value []byte) (int64, error) {
	atomic.AddInt64(&m.messageCount, 1)
	atomic.AddInt64(&m.bytesWritten, int64(len(value)))
	return atomic.LoadInt64(&m.messageCount), nil
}

// ConsumeMessages implements kafka.MessageStore
func (m *MockThroughputStore) ConsumeMessages(topic string, partition int32, offset int64, maxBytes int32) ([]*kafka.Message, error) {
	messages := []*kafka.Message{
		{
			Offset:    offset,
			Key:       []byte("test-key"),
			Value:     []byte("test-message"),
			Timestamp: time.Now(),
		},
	}
	return messages, nil
}

// GetTopicMetadata implements kafka.MessageStore
func (m *MockThroughputStore) GetTopicMetadata(topics []string) (*kafka.TopicMetadata, error) {
	return &kafka.TopicMetadata{}, nil
}

// CreateTopic implements kafka.MessageStore
func (m *MockThroughputStore) CreateTopic(topic string, partitions int32, replication int16) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.topics[topic] = true
	return nil
}

// DeleteTopic implements kafka.MessageStore
func (m *MockThroughputStore) DeleteTopic(topic string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.topics, topic)
	return nil
}

func (m *MockThroughputStore) IncrementRequestCount() {
	atomic.AddInt64(&m.requestCount, 1)
}

// Kafka protocol helpers for testing
func createProduceRequest(topic string, partition int32, messages [][]byte) []byte {
	buf := new(bytes.Buffer)
	
	// API Key (0 = Produce)
	binary.Write(buf, binary.BigEndian, int16(0))
	// API Version
	binary.Write(buf, binary.BigEndian, int16(0))
	// Correlation ID
	binary.Write(buf, binary.BigEndian, int32(1))
	// Client ID length
	binary.Write(buf, binary.BigEndian, int16(len("throughput-test")))
	buf.WriteString("throughput-test")
	
	// RequiredAcks
	binary.Write(buf, binary.BigEndian, int16(1))
	// Timeout
	binary.Write(buf, binary.BigEndian, int32(5000))
	
	// Topic array length
	binary.Write(buf, binary.BigEndian, int32(1))
	// Topic name length
	binary.Write(buf, binary.BigEndian, int16(len(topic)))
	buf.WriteString(topic)
	
	// Partition array length
	binary.Write(buf, binary.BigEndian, int32(1))
	// Partition ID
	binary.Write(buf, binary.BigEndian, partition)
	
	// Message set size (placeholder)
	messageSetStart := buf.Len()
	binary.Write(buf, binary.BigEndian, int32(0))
	
	messageSetPos := buf.Len()
	
	// Write messages
	for i, msg := range messages {
		// Offset
		binary.Write(buf, binary.BigEndian, int64(i))
		// Message size (placeholder)
		msgSizePos := buf.Len()
		binary.Write(buf, binary.BigEndian, int32(0))
		msgStart := buf.Len()
		
		// CRC (placeholder)
		binary.Write(buf, binary.BigEndian, int32(0))
		// Magic byte
		binary.Write(buf, binary.BigEndian, int8(0))
		// Attributes
		binary.Write(buf, binary.BigEndian, int8(0))
		// Key length (-1 = null)
		binary.Write(buf, binary.BigEndian, int32(-1))
		// Value length
		binary.Write(buf, binary.BigEndian, int32(len(msg)))
		// Value
		buf.Write(msg)
		
		// Update message size
		msgEnd := buf.Len()
		msgSize := msgEnd - msgStart
		binary.BigEndian.PutUint32(buf.Bytes()[msgSizePos:], uint32(msgSize))
	}
	
	// Update message set size
	messageSetEnd := buf.Len()
	messageSetSize := messageSetEnd - messageSetPos
	binary.BigEndian.PutUint32(buf.Bytes()[messageSetStart:], uint32(messageSetSize))
	
	// Prepend total message size
	result := new(bytes.Buffer)
	binary.Write(result, binary.BigEndian, int32(buf.Len()))
	result.Write(buf.Bytes())
	
	return result.Bytes()
}

// BenchmarkKafka_ProduceThroughput measures produce throughput
func BenchmarkKafka_ProduceThroughput(b *testing.B) {
	store := NewMockThroughputStore()
	
	// Create test message
	message := []byte("test-message-" + string(make([]byte, 100)))
	
	b.ResetTimer()
	b.ReportAllocs()
	
	var processed int64
	start := time.Now()
	
	for i := 0; i < b.N; i++ {
		// Simulate produce request
		store.ProduceMessage("test-topic", 0, nil, message)
		atomic.AddInt64(&processed, 1)
	}
	
	duration := time.Since(start)
	throughput := float64(processed) / duration.Seconds()
	
	b.ReportMetric(throughput, "msgs/sec")
	b.ReportMetric(float64(processed*int64(len(message)))/duration.Seconds()/1024/1024, "MB/sec")
}

// BenchmarkKafka_ProduceThroughput_Parallel measures parallel produce throughput
func BenchmarkKafka_ProduceThroughput_Parallel(b *testing.B) {
	store := NewMockThroughputStore()
	message := []byte("test-message-" + string(make([]byte, 100)))
	
	b.ResetTimer()
	b.ReportAllocs()
	
	var processed int64
	start := time.Now()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			store.ProduceMessage("test-topic", 0, nil, message)
			atomic.AddInt64(&processed, 1)
		}
	})
	
	duration := time.Since(start)
	throughput := float64(processed) / duration.Seconds()
	
	b.ReportMetric(throughput, "msgs/sec")
	b.ReportMetric(float64(processed*int64(len(message)))/duration.Seconds()/1024/1024, "MB/sec")
}

// BenchmarkKafka_OffsetCommitThroughput measures offset commit throughput
func BenchmarkKafka_OffsetCommitThroughput(b *testing.B) {
	offsetManager := kafka.NewOffsetManagerWithMetadata()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	var processed int64
	start := time.Now()
	
	for i := 0; i < b.N; i++ {
		offsetManager.CommitOffset("test-group", "test-topic", 0, int64(i))
		atomic.AddInt64(&processed, 1)
	}
	
	duration := time.Since(start)
	throughput := float64(processed) / duration.Seconds()
	
	b.ReportMetric(throughput, "commits/sec")
}

// BenchmarkKafka_OffsetCommitThroughput_Parallel measures parallel offset commit throughput
func BenchmarkKafka_OffsetCommitThroughput_Parallel(b *testing.B) {
	offsetManager := kafka.NewOffsetManagerWithMetadata()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	var processed int64
	start := time.Now()
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			offsetManager.CommitOffset(fmt.Sprintf("group-%d", i%10), "test-topic", 0, int64(i))
			atomic.AddInt64(&processed, 1)
			i++
		}
	})
	
	duration := time.Since(start)
	throughput := float64(processed) / duration.Seconds()
	
	b.ReportMetric(throughput, "commits/sec")
}

// BenchmarkKafka_GroupCoordinatorThroughput measures group operations throughput
func BenchmarkKafka_GroupCoordinatorThroughput(b *testing.B) {
	groupCoordinator := kafka.NewGroupCoordinator()
	
	// Setup initial group
	groupCoordinator.JoinGroup("test-group", "member-1", "consumer", "roundrobin", "range", 30000*time.Millisecond, 5000*time.Millisecond, []string{"test-topic"}, nil)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	var processed int64
	start := time.Now()
	
	for i := 0; i < b.N; i++ {
		// Simulate heartbeat operations
		groupCoordinator.Heartbeat("test-group", "member-1", 0)
		atomic.AddInt64(&processed, 1)
	}
	
	duration := time.Since(start)
	throughput := float64(processed) / duration.Seconds()
	
	b.ReportMetric(throughput, "heartbeats/sec")
}

// BenchmarkKafka_GroupCoordinatorThroughput_Parallel measures parallel group operations
func BenchmarkKafka_GroupCoordinatorThroughput_Parallel(b *testing.B) {
	groupCoordinator := kafka.NewGroupCoordinator()
	
	// Setup initial groups
	for i := 0; i < 10; i++ {
		groupID := fmt.Sprintf("test-group-%d", i)
		memberID := fmt.Sprintf("member-%d", i)
		groupCoordinator.JoinGroup(groupID, memberID, "consumer", "roundrobin", "range", 30000*time.Millisecond, 5000*time.Millisecond, []string{"test-topic"}, nil)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	var processed int64
	start := time.Now()
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			groupID := fmt.Sprintf("test-group-%d", i%10)
			memberID := fmt.Sprintf("member-%d", i%10)
			groupCoordinator.Heartbeat(groupID, memberID, 0)
			atomic.AddInt64(&processed, 1)
			i++
		}
	})
	
	duration := time.Since(start)
	throughput := float64(processed) / duration.Seconds()
	
	b.ReportMetric(throughput, "heartbeats/sec")
}

// TestKafka_RealWorldThroughput tests real-world throughput with actual server
func TestKafka_RealWorldThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real-world throughput test in short mode")
	}

	// Start Kafka server
	store := NewMockThroughputStore()
	server := kafka.NewKafkaServer(":19092", store)
	
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	go func() {
		if err := server.Start(); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()
	
	// Wait for server to start
	time.Sleep(500 * time.Millisecond)
	
	// Test scenarios
	scenarios := []struct {
		name        string
		concurrency int
		duration    time.Duration
		messageSize int
	}{
		{"Low Load", 10, 5 * time.Second, 100},
		{"Medium Load", 50, 5 * time.Second, 100},
		{"High Load", 100, 5 * time.Second, 100},
		{"Large Messages", 50, 5 * time.Second, 1024},
	}
	
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			var (
				totalMessages int64
				totalBytes    int64
				errors        int64
				wg            sync.WaitGroup
			)
			
			start := time.Now()
			deadline := start.Add(scenario.duration)
			
			// Launch concurrent producers
			for i := 0; i < scenario.concurrency; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					
					// Connect to server
					conn, err := net.Dial("tcp", "localhost:19092")
					if err != nil {
						atomic.AddInt64(&errors, 1)
						return
					}
					defer conn.Close()
					
					message := make([]byte, scenario.messageSize)
					for i := range message {
						message[i] = byte('A' + (i % 26))
					}
					
					for time.Now().Before(deadline) {
						// Send produce request
						request := createProduceRequest("test-topic", 0, [][]byte{message})
						
						_, err := conn.Write(request)
						if err != nil {
							atomic.AddInt64(&errors, 1)
							continue
						}
						
						// Read response (with timeout)
						conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
						response := make([]byte, 1024)
						_, err = conn.Read(response)
						if err != nil {
							// Timeout is ok, server might be busy
							continue
						}
						
						atomic.AddInt64(&totalMessages, 1)
						atomic.AddInt64(&totalBytes, int64(len(message)))
					}
				}(i)
			}
			
			wg.Wait()
			duration := time.Since(start)
			
			messagesPerSec := float64(totalMessages) / duration.Seconds()
			mbPerSec := float64(totalBytes) / duration.Seconds() / 1024 / 1024
			
			t.Logf("Scenario: %s", scenario.name)
			t.Logf("  Concurrency: %d", scenario.concurrency)
			t.Logf("  Duration: %v", duration)
			t.Logf("  Total Messages: %d", totalMessages)
			t.Logf("  Total Bytes: %d (%.2f MB)", totalBytes, float64(totalBytes)/1024/1024)
			t.Logf("  Throughput: %.0f messages/sec", messagesPerSec)
			t.Logf("  Throughput: %.2f MB/sec", mbPerSec)
			t.Logf("  Errors: %d", errors)
			t.Logf("  Error Rate: %.2f%%", float64(errors)/float64(totalMessages+errors)*100)
		})
	}
	
	// Stop server
	cancel()
	time.Sleep(100 * time.Millisecond)
}

// TestKafka_SustainedLoad tests sustained load over longer period
func TestKafka_SustainedLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping sustained load test in short mode")
	}

	store := NewMockThroughputStore()
	
	duration := 30 * time.Second
	concurrency := 100
	messageSize := 256
	
	var (
		totalMessages int64
		totalBytes    int64
		wg            sync.WaitGroup
	)
	
	start := time.Now()
	deadline := start.Add(duration)
	
	message := make([]byte, messageSize)
	for i := range message {
		message[i] = byte('A' + (i % 26))
	}
	
	// Launch workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			for time.Now().Before(deadline) {
				store.ProduceMessage("test-topic", 0, nil, message)
				atomic.AddInt64(&totalMessages, 1)
				atomic.AddInt64(&totalBytes, int64(len(message)))
			}
		}()
	}
	
	// Monitor progress
	ticker := time.NewTicker(5 * time.Second)
	done := make(chan struct{})
	
	go func() {
		lastMessages := int64(0)
		lastTime := start
		
		for {
			select {
			case <-ticker.C:
				currentMessages := atomic.LoadInt64(&totalMessages)
				currentTime := time.Now()
				
				deltaMessages := currentMessages - lastMessages
				deltaTime := currentTime.Sub(lastTime).Seconds()
				currentThroughput := float64(deltaMessages) / deltaTime
				
				t.Logf("[%v] Current throughput: %.0f msgs/sec, Total: %d messages",
					currentTime.Sub(start).Round(time.Second),
					currentThroughput,
					currentMessages)
				
				lastMessages = currentMessages
				lastTime = currentTime
				
			case <-done:
				return
			}
		}
	}()
	
	wg.Wait()
	close(done)
	ticker.Stop()
	
	actualDuration := time.Since(start)
	
	messagesPerSec := float64(totalMessages) / actualDuration.Seconds()
	mbPerSec := float64(totalBytes) / actualDuration.Seconds() / 1024 / 1024
	
	t.Logf("\n=== Sustained Load Test Results ===")
	t.Logf("Duration: %v", actualDuration)
	t.Logf("Concurrency: %d goroutines", concurrency)
	t.Logf("Message Size: %d bytes", messageSize)
	t.Logf("Total Messages: %d", totalMessages)
	t.Logf("Total Data: %.2f MB", float64(totalBytes)/1024/1024)
	t.Logf("Average Throughput: %.0f messages/sec", messagesPerSec)
	t.Logf("Average Throughput: %.2f MB/sec", mbPerSec)
	t.Logf("Per-Goroutine: %.0f messages/sec", messagesPerSec/float64(concurrency))
}

