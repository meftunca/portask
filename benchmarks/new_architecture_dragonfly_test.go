package benchmarks

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/amqp"
	"github.com/meftunca/portask/pkg/kafka"
	"github.com/meftunca/portask/pkg/processor"
	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/storage/dragonfly"
	"github.com/meftunca/portask/pkg/types"
)

// TestNewArchitectureWithDragonfly tests the new architecture with real Dragonfly storage
func TestNewArchitectureWithDragonfly(t *testing.T) {
	// Setup Dragonfly
	dfConfig := &storage.DragonflyConfig{
		Addresses:         []string{"localhost:6379"},
		DB:                0,
		KeyPrefix:         "portask-new-arch-test",
		EnableCompression: true,
	}

	ctx := context.Background()
	dragonflyStore, err := dragonfly.NewDragonflyStore(dfConfig)
	if err != nil {
		t.Fatalf("Failed to create Dragonfly store: %v", err)
	}

	err = dragonflyStore.Connect(ctx)
	if err != nil {
		t.Skipf("Dragonfly not available: %v. Please ensure Dragonfly is running on localhost:6379", err)
		return
	}
	defer dragonflyStore.Close()

	// Clear test data
	dragonflyStore.GetClient().FlushDB(ctx)

	t.Log("✅ Connected to Dragonfly")

	t.Run("Kafka_Translator_Processor_Dragonfly", func(t *testing.T) {
		// 1. Create processor
		proc := processor.NewMessageProcessor(processor.DefaultProcessorConfig())
		if err := proc.Start(ctx); err != nil {
			t.Fatalf("Failed to start processor: %v", err)
		}
		defer proc.Stop()

		// 2. Create Kafka store adapter
		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)

		// 3. Create translator and bridge
		translator := kafka.NewKafkaTranslator()
		bridge := kafka.NewProcessorBridge(proc, kafkaStore)

		// 4. Simulate Kafka client producing message
		topic := "orders"
		partition := int32(0)
		key := []byte("order-123")
		value := []byte(`{"order_id": "123", "customer": "John Doe", "amount": 99.99}`)

		// 5. Translate to Portask message
		portaskMsg, err := translator.TranslateProduce(topic, partition, key, value)
		if err != nil {
			t.Fatalf("Translation failed: %v", err)
		}

		t.Logf("📝 Translated Kafka message to Portask message (ID: %s)", portaskMsg.ID)

		// 6. Process through processor and store to Dragonfly
		offset, err := bridge.ProduceMessage(ctx, portaskMsg)
		if err != nil {
			t.Fatalf("Processing failed: %v", err)
		}

		t.Logf("✅ Message processed and stored to Dragonfly (offset: %d)", offset)

		// 7. Verify message was stored
		stats, err := dragonflyStore.Stats(ctx)
		if err != nil {
			t.Fatalf("Failed to get stats: %v", err)
		}

		if stats.MessageCount == 0 {
			t.Error("Message was not stored to Dragonfly")
		}

		t.Logf("📊 Dragonfly Stats:")
		t.Logf("   Total Operations: %d", stats.TotalOperations)
		t.Logf("   Messages Stored: %d", stats.MessageCount)
		t.Logf("   Successful Operations: %d", stats.SuccessfulOperations)

		// 8. Verify processor metrics
		procMetrics := proc.GetMetrics()
		t.Logf("📊 Processor Stats:")
		t.Logf("   Total Tasks: %d", procMetrics.TotalTasks)
		t.Logf("   Success Count: %d", procMetrics.SuccessCount)
		t.Logf("   Error Count: %d", procMetrics.ErrorCount)
	})

	t.Run("AMQP_Translator_Processor_Dragonfly", func(t *testing.T) {
		// Clear previous test data
		dragonflyStore.GetClient().FlushDB(ctx)

		// 1. Create processor
		proc := processor.NewMessageProcessor(processor.DefaultProcessorConfig())
		if err := proc.Start(ctx); err != nil {
			t.Fatalf("Failed to start processor: %v", err)
		}
		defer proc.Stop()

		// 2. Create AMQP store adapter
		amqpStore := NewDragonflyAMQPStore(ctx, dragonflyStore)

		// 3. Create translator and bridge
		translator := amqp.NewAMQPTranslator()
		bridge := amqp.NewProcessorBridge(proc, amqpStore)

		// 4. Simulate AMQP client publishing message
		exchange := "notifications"
		routingKey := "email.sent"
		body := []byte(`{"recipient": "user@example.com", "subject": "Welcome!", "sent_at": "2024-01-01T12:00:00Z"}`)
		props := &amqp.MessageProperties{
			ContentType:   "application/json",
			CorrelationID: "notif-456",
			Priority:      5,
		}

		// 5. Translate to Portask message
		portaskMsg, err := translator.TranslatePublish(exchange, routingKey, body, props)
		if err != nil {
			t.Fatalf("Translation failed: %v", err)
		}

		t.Logf("📝 Translated AMQP message to Portask message (ID: %s)", portaskMsg.ID)

		// 6. Process through processor and store to Dragonfly
		offset, err := bridge.PublishMessage(ctx, portaskMsg)
		if err != nil {
			t.Fatalf("Processing failed: %v", err)
		}

		t.Logf("✅ Message processed and stored to Dragonfly (offset: %d)", offset)

		// 7. Verify message was stored
		stats, err := dragonflyStore.Stats(ctx)
		if err != nil {
			t.Fatalf("Failed to get stats: %v", err)
		}

		if stats.MessageCount == 0 {
			t.Error("Message was not stored to Dragonfly")
		}

		t.Logf("📊 Dragonfly Stats:")
		t.Logf("   Total Operations: %d", stats.TotalOperations)
		t.Logf("   Messages Stored: %d", stats.MessageCount)

		// 8. Verify processor metrics
		procMetrics := proc.GetMetrics()
		t.Logf("📊 Processor Stats:")
		t.Logf("   Total Tasks: %d", procMetrics.TotalTasks)
		t.Logf("   Success Count: %d", procMetrics.SuccessCount)
	})

	t.Run("Mixed_Protocols_Same_Processor", func(t *testing.T) {
		// Clear previous test data
		dragonflyStore.GetClient().FlushDB(ctx)

		// Single processor for ALL protocols
		proc := processor.NewMessageProcessor(processor.DefaultProcessorConfig())
		if err := proc.Start(ctx); err != nil {
			t.Fatalf("Failed to start processor: %v", err)
		}
		defer proc.Stop()

		// Kafka setup
		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
		kafkaTranslator := kafka.NewKafkaTranslator()
		kafkaBridge := kafka.NewProcessorBridge(proc, kafkaStore)

		// AMQP setup
		amqpStore := NewDragonflyAMQPStore(ctx, dragonflyStore)
		amqpTranslator := amqp.NewAMQPTranslator()
		amqpBridge := amqp.NewProcessorBridge(proc, amqpStore)

		// Send messages from both protocols
		messages := []struct {
			protocol string
			produce  func() error
		}{
			{"Kafka", func() error {
				msg, _ := kafkaTranslator.TranslateProduce("topic1", 0, nil, []byte("kafka-msg-1"))
				_, err := kafkaBridge.ProduceMessage(ctx, msg)
				return err
			}},
			{"AMQP", func() error {
				msg, _ := amqpTranslator.TranslatePublish("", "queue1", []byte("amqp-msg-1"), nil)
				_, err := amqpBridge.PublishMessage(ctx, msg)
				return err
			}},
			{"Kafka", func() error {
				msg, _ := kafkaTranslator.TranslateProduce("topic2", 0, nil, []byte("kafka-msg-2"))
				_, err := kafkaBridge.ProduceMessage(ctx, msg)
				return err
			}},
			{"AMQP", func() error {
				msg, _ := amqpTranslator.TranslatePublish("", "queue2", []byte("amqp-msg-2"), nil)
				_, err := amqpBridge.PublishMessage(ctx, msg)
				return err
			}},
		}

		// Process all messages
		for i, msg := range messages {
			if err := msg.produce(); err != nil {
				t.Errorf("Message %d (%s) failed: %v", i+1, msg.protocol, err)
			}
			t.Logf("✅ Processed %s message %d", msg.protocol, i+1)
		}

		// Verify all went through same processor
		procMetrics := proc.GetMetrics()

		t.Logf("📊 Final Processor Stats:")
		t.Logf("   Total Tasks: %d", procMetrics.TotalTasks)
		t.Logf("   Success Count: %d", procMetrics.SuccessCount)
		t.Logf("   Error Count: %d", procMetrics.ErrorCount)

		// Verify Dragonfly storage
		stats, _ := dragonflyStore.Stats(ctx)
		t.Logf("📊 Final Dragonfly Stats:")
		t.Logf("   Messages Stored: %d", stats.MessageCount)
		t.Logf("   Total Operations: %d", stats.TotalOperations)

		if stats.MessageCount < 4 {
			t.Errorf("Expected at least 4 messages, got %d", stats.MessageCount)
		}

		t.Log("✅ All protocols successfully used same processor!")
	})
}

// BenchmarkNewArchitectureWithDragonfly benchmarks the new architecture with real Dragonfly
func BenchmarkNewArchitectureWithDragonfly(b *testing.B) {
	// Setup Dragonfly
	dfConfig := &storage.DragonflyConfig{
		Addresses:         []string{"localhost:6379"},
		DB:                0,
		KeyPrefix:         "portask-bench",
		EnableCompression: false, // Disable for raw performance
	}

	ctx := context.Background()
	dragonflyStore, err := dragonfly.NewDragonflyStore(dfConfig)
	if err != nil {
		b.Fatalf("Failed to create Dragonfly store: %v", err)
	}

	if err := dragonflyStore.Connect(ctx); err != nil {
		b.Skipf("Dragonfly not available: %v", err)
		return
	}
	defer dragonflyStore.Close()

	// Clear test data
	dragonflyStore.GetClient().FlushDB(ctx)

	b.Run("Kafka_WithProcessor", func(b *testing.B) {
		// Setup
		proc := processor.NewMessageProcessor(processor.DefaultProcessorConfig())
		proc.Start(ctx)
		defer proc.Stop()

		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
		translator := kafka.NewKafkaTranslator()
		bridge := kafka.NewProcessorBridge(proc, kafkaStore)

		payload := make([]byte, 1024) // 1KB payload

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			msg, _ := translator.TranslateProduce("bench-topic", 0, nil, payload)
			bridge.ProduceMessage(ctx, msg)
		}

		b.StopTimer()

		// Report stats
		stats, _ := dragonflyStore.Stats(ctx)
		b.ReportMetric(float64(stats.MessageCount), "messages_stored")
		b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "ops/sec")
	})
}

// Helper: Dragonfly Kafka Store Adapter
type DragonflyKafkaStoreAdapter struct {
	ctx   context.Context
	store *dragonfly.DragonflyStore
}

func NewDragonflyKafkaStore(ctx context.Context, store *dragonfly.DragonflyStore) *DragonflyKafkaStoreAdapter {
	return &DragonflyKafkaStoreAdapter{ctx: ctx, store: store}
}

func (d *DragonflyKafkaStoreAdapter) ProduceMessage(topic string, partition int32, key, value []byte) (int64, error) {
	msg := &types.PortaskMessage{
		ID:        types.MessageID(fmt.Sprintf("%d", time.Now().UnixNano())),
		Topic:     types.TopicName(topic),
		Partition: partition,
		Key:       string(key),
		Payload:   value,
		Timestamp: time.Now().UnixNano(),
		TTL:       int64(time.Hour),
	}

	if err := d.store.Store(d.ctx, msg); err != nil {
		return 0, err
	}

	return msg.Timestamp, nil
}

func (d *DragonflyKafkaStoreAdapter) ConsumeMessages(topic string, partition int32, offset int64, maxBytes int32) ([]*kafka.Message, error) {
	messages, err := d.store.Fetch(d.ctx, types.TopicName(topic), partition, offset, 100)
	if err != nil {
		return nil, err
	}

	kafkaMessages := make([]*kafka.Message, 0, len(messages))
	for _, msg := range messages {
		kafkaMessages = append(kafkaMessages, &kafka.Message{
			Offset: msg.Timestamp,
			Key:    []byte(msg.Key),
			Value:  msg.Payload,
		})
	}

	return kafkaMessages, nil
}

func (d *DragonflyKafkaStoreAdapter) GetTopicMetadata(topics []string) (*kafka.TopicMetadata, error) {
	return &kafka.TopicMetadata{}, nil
}

func (d *DragonflyKafkaStoreAdapter) CreateTopic(topic string, partitions int32, replication int16) error {
	return nil
}

func (d *DragonflyKafkaStoreAdapter) DeleteTopic(topic string) error {
	return nil
}

// Helper: Dragonfly AMQP Store Adapter
type DragonflyAMQPStoreAdapter struct {
	ctx   context.Context
	store *dragonfly.DragonflyStore
}

func NewDragonflyAMQPStore(ctx context.Context, store *dragonfly.DragonflyStore) *DragonflyAMQPStoreAdapter {
	return &DragonflyAMQPStoreAdapter{ctx: ctx, store: store}
}

func (d *DragonflyAMQPStoreAdapter) StoreMessage(topic string, message []byte) error {
	msg := &types.PortaskMessage{
		ID:        types.MessageID(fmt.Sprintf("%d", time.Now().UnixNano())),
		Topic:     types.TopicName(topic),
		Payload:   message,
		Timestamp: time.Now().UnixNano(),
		TTL:       int64(time.Hour),
	}

	return d.store.Store(d.ctx, msg)
}

func (d *DragonflyAMQPStoreAdapter) GetMessages(topic string, offset int64) ([][]byte, error) {
	messages, err := d.store.Fetch(d.ctx, types.TopicName(topic), 0, offset, 100)
	if err != nil {
		return nil, err
	}

	result := make([][]byte, 0, len(messages))
	for _, msg := range messages {
		result = append(result, msg.Payload)
	}

	return result, nil
}

func (d *DragonflyAMQPStoreAdapter) GetTopics() []string {
	return []string{}
}
