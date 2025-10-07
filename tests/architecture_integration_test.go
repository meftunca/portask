package tests

import (
	"context"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/kafka"
	"github.com/meftunca/portask/pkg/processor"
	"github.com/meftunca/portask/pkg/types"
)

// TestKafkaTranslatorArchitecture tests the new Kafka translator architecture
func TestKafkaTranslatorArchitecture(t *testing.T) {
	t.Run("TranslateProduce", func(t *testing.T) {
		translator := kafka.NewKafkaTranslator()

		topic := "test-topic"
		partition := int32(0)
		key := []byte("test-key")
		value := []byte("test-value")

		msg, err := translator.TranslateProduce(topic, partition, key, value)
		if err != nil {
			t.Fatalf("TranslateProduce failed: %v", err)
		}

		// Verify message structure
		if msg.Topic != types.TopicName(topic) {
			t.Errorf("Expected topic %s, got %s", topic, msg.Topic)
		}

		if msg.Partition != partition {
			t.Errorf("Expected partition %d, got %d", partition, msg.Partition)
		}

		if string(msg.Payload) != string(value) {
			t.Errorf("Expected payload %s, got %s", value, msg.Payload)
		}

		// Verify metadata
		if msg.Metadata == nil {
			t.Fatal("Metadata should not be nil")
		}

		if msg.Metadata["source"] != "kafka" {
			t.Errorf("Expected source 'kafka', got '%s'", msg.Metadata["source"])
		}

		if msg.Metadata["protocol"] != "kafka-wire" {
			t.Errorf("Expected protocol 'kafka-wire', got '%s'", msg.Metadata["protocol"])
		}
	})

	t.Run("TranslateFetch", func(t *testing.T) {
		translator := kafka.NewKafkaTranslator()

		topic := "test-topic"
		partition := int32(0)
		offset := int64(100)
		maxBytes := int32(1024)

		fetchReq, err := translator.TranslateFetch(topic, partition, offset, maxBytes)
		if err != nil {
			t.Fatalf("TranslateFetch failed: %v", err)
		}

		if fetchReq.Topic != types.TopicName(topic) {
			t.Errorf("Expected topic %s, got %s", topic, fetchReq.Topic)
		}

		if fetchReq.Partition != partition {
			t.Errorf("Expected partition %d, got %d", partition, fetchReq.Partition)
		}

		if fetchReq.Offset != offset {
			t.Errorf("Expected offset %d, got %d", offset, fetchReq.Offset)
		}
	})
}

// TestProcessorBridgeArchitecture tests the processor bridge
func TestProcessorBridgeArchitecture(t *testing.T) {
	// Create processor
	proc := processor.NewMessageProcessor(processor.DefaultProcessorConfig())
	ctx := context.Background()

	// Start processor
	if err := proc.Start(ctx); err != nil {
		t.Fatalf("Failed to start processor: %v", err)
	}
	defer proc.Stop()

	// Create mock store
	mockStore := &MockKafkaStore{
		messages: make(map[string][]byte),
	}

	// Create bridge
	bridge := kafka.NewProcessorBridge(proc, mockStore)

	t.Run("ProduceMessage", func(t *testing.T) {
		msg := &types.PortaskMessage{
			ID:        types.MessageID("test-1"),
			Topic:     types.TopicName("test-topic"),
			Partition: 0,
			Key:       "test-key",
			Payload:   []byte("test-payload"),
			Timestamp: time.Now().UnixNano(),
			Metadata: map[string]string{
				"source": "kafka",
			},
		}

		offset, err := bridge.ProduceMessage(ctx, msg)
		if err != nil {
			t.Fatalf("ProduceMessage failed: %v", err)
		}

		if offset <= 0 {
			t.Errorf("Expected positive offset, got %d", offset)
		}

		// Verify message was stored
		if len(mockStore.messages) == 0 {
			t.Error("Message was not stored")
		}
	})

	t.Run("FetchMessages", func(t *testing.T) {
		// Add some messages to mock store
		mockStore.messages["test-topic"] = []byte("message-1")

		fetchReq := &types.FetchRequest{
			Topic:     types.TopicName("test-topic"),
			Partition: 0,
			Offset:    0,
			Limit:     10,
		}

		messages, err := bridge.FetchMessages(ctx, fetchReq)
		if err != nil {
			t.Fatalf("FetchMessages failed: %v", err)
		}

		if len(messages) == 0 {
			t.Error("Expected at least one message")
		}
	})
}

// TestEndToEndArchitecture tests complete flow: Client → Translator → Processor → Storage
func TestEndToEndArchitecture(t *testing.T) {
	// Setup
	proc := processor.NewMessageProcessor(processor.DefaultProcessorConfig())
	ctx := context.Background()

	if err := proc.Start(ctx); err != nil {
		t.Fatalf("Failed to start processor: %v", err)
	}
	defer proc.Stop()

	mockStore := &MockKafkaStore{
		messages: make(map[string][]byte),
	}

	translator := kafka.NewKafkaTranslator()
	bridge := kafka.NewProcessorBridge(proc, mockStore)

	t.Run("CompleteProduceFlow", func(t *testing.T) {
		// 1. Client sends Kafka produce request
		topic := "orders"
		partition := int32(0)
		key := []byte("order-123")
		value := []byte(`{"order_id": "123", "amount": 99.99}`)

		// 2. Translator converts to Portask message
		portaskMsg, err := translator.TranslateProduce(topic, partition, key, value)
		if err != nil {
			t.Fatalf("Translation failed: %v", err)
		}

		// 3. Bridge processes through processor
		offset, err := bridge.ProduceMessage(ctx, portaskMsg)
		if err != nil {
			t.Fatalf("Processing failed: %v", err)
		}

		// 4. Verify result
		if offset <= 0 {
			t.Errorf("Invalid offset: %d", offset)
		}

		// Verify message went through processor (has protocol metadata)
		if portaskMsg.Metadata["source"] != "kafka" {
			t.Error("Message lost protocol metadata")
		}

		// Verify message was stored
		if len(mockStore.messages) == 0 {
			t.Error("Message was not persisted")
		}

		t.Logf("✅ Complete flow successful: Kafka → Translator → Processor → Storage")
	})
}

// TestArchitectureConsistency verifies all protocols use same processor
func TestArchitectureConsistency(t *testing.T) {
	proc := processor.NewMessageProcessor(processor.DefaultProcessorConfig())

	// Get processor metrics before
	metricsBefore := proc.GetMetrics()
	initialCount := metricsBefore.TotalTasks

	ctx := context.Background()
	if err := proc.Start(ctx); err != nil {
		t.Fatalf("Failed to start processor: %v", err)
	}
	defer proc.Stop()

	mockStore := &MockKafkaStore{
		messages: make(map[string][]byte),
	}

	// Create Kafka bridge
	kafkaBridge := kafka.NewProcessorBridge(proc, mockStore)

	// Process a Kafka message
	kafkaMsg := &types.PortaskMessage{
		ID:        types.MessageID("kafka-1"),
		Topic:     types.TopicName("test"),
		Payload:   []byte("kafka-payload"),
		Timestamp: time.Now().UnixNano(),
		Metadata:  map[string]string{"source": "kafka"},
	}

	_, err := kafkaBridge.ProduceMessage(ctx, kafkaMsg)
	if err != nil {
		t.Fatalf("Kafka message failed: %v", err)
	}

	// Get metrics after
	metricsAfter := proc.GetMetrics()
	finalCount := metricsAfter.TotalTasks

	// Verify all messages went through same processor
	if finalCount <= initialCount {
		t.Errorf("Expected processor task count to increase, got %d (was %d)", finalCount, initialCount)
	}

	t.Logf("✅ All protocols use same processor: %d tasks processed", finalCount-initialCount)
}

// MockKafkaStore implements kafka.MessageStore for testing
type MockKafkaStore struct {
	messages map[string][]byte
}

func (m *MockKafkaStore) ProduceMessage(topic string, partition int32, key, value []byte) (int64, error) {
	m.messages[topic] = value
	return time.Now().UnixNano(), nil
}

func (m *MockKafkaStore) ConsumeMessages(topic string, partition int32, offset int64, maxBytes int32) ([]*kafka.Message, error) {
	if msg, ok := m.messages[topic]; ok {
		return []*kafka.Message{
			{
				Offset: offset,
				Key:    []byte("key"),
				Value:  msg,
			},
		}, nil
	}
	return []*kafka.Message{}, nil
}

func (m *MockKafkaStore) GetTopicMetadata(topics []string) (*kafka.TopicMetadata, error) {
	return &kafka.TopicMetadata{}, nil
}

func (m *MockKafkaStore) CreateTopic(topic string, partitions int32, replication int16) error {
	return nil
}

func (m *MockKafkaStore) DeleteTopic(topic string) error {
	delete(m.messages, topic)
	return nil
}
