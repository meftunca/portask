package benchmarks

import (
	"context"
	"fmt"
	"testing"

	"github.com/meftunca/portask/pkg/kafka"
	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/storage/dragonfly"
	"github.com/meftunca/portask/pkg/types"
)

// TestSimpleNewArchitectureFlow tests the new architecture with Dragonfly in a simplified way
func TestSimpleNewArchitectureFlow(t *testing.T) {
	// Setup Dragonfly
	dfConfig := &storage.DragonflyConfig{
		Addresses:         []string{"localhost:6379"},
		DB:                0,
		KeyPrefix:         "portask-simple-test",
		EnableCompression: false,
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

	t.Run("DirectDragonflyWrite", func(t *testing.T) {
		// 1. Create Kafka translator
		translator := kafka.NewKafkaTranslator()

		// 2. Translate Kafka message to Portask message
		portaskMsg, err := translator.TranslateProduce("test-topic", 0, []byte("key1"), []byte("Hello Dragonfly!"))
		if err != nil {
			t.Fatalf("Translation failed: %v", err)
		}

		t.Logf("✅ Translated: Kafka → Portask (ID: %s)", portaskMsg.ID)

		// 3. Write directly to Dragonfly
		err = dragonflyStore.Store(ctx, portaskMsg)
		if err != nil {
			t.Fatalf("Store failed: %v", err)
		}

		t.Log("✅ Stored to Dragonfly")

		// 4. Read back
		messages, err := dragonflyStore.Fetch(ctx, portaskMsg.Topic, 0, 0, 10)
		if err != nil {
			t.Fatalf("Fetch failed: %v", err)
		}

		if len(messages) == 0 {
			t.Error("No messages fetched")
		} else {
			t.Logf("✅ Fetched %d messages", len(messages))
			t.Logf("   First message: %s", string(messages[0].Payload))
		}

		// 5. Check Dragonfly stats
		stats, err := dragonflyStore.Stats(ctx)
		if err != nil {
			t.Fatalf("Stats failed: %v", err)
		}

		t.Logf("📊 Dragonfly Stats:")
		t.Logf("   Total Operations: %d", stats.TotalOperations)
		t.Logf("   Successful Operations: %d", stats.SuccessfulOperations)
		t.Logf("   Failed Operations: %d", stats.FailedOperations)
	})

	t.Run("MultipleMessages", func(t *testing.T) {
		dragonflyStore.GetClient().FlushDB(ctx)

		translator := kafka.NewKafkaTranslator()

		// Write multiple messages
		for i := 0; i < 10; i++ {
			msg, _ := translator.TranslateProduce(
				"orders",
				0,
				[]byte(fmt.Sprintf("order-%d", i)),
				[]byte(fmt.Sprintf(`{"id": %d, "amount": %d.99}`, i, i*10)),
			)

			err := dragonflyStore.Store(ctx, msg)
			if err != nil {
				t.Errorf("Failed to store message %d: %v", i, err)
			}
		}

		t.Log("✅ Stored 10 messages")

		// Fetch them back
		messages, err := dragonflyStore.Fetch(ctx, types.TopicName("orders"), 0, 0, 20)
		if err != nil {
			t.Fatalf("Fetch failed: %v", err)
		}

		t.Logf("✅ Fetched %d messages from 'orders' topic", len(messages))

		if len(messages) < 10 {
			t.Errorf("Expected at least 10 messages, got %d", len(messages))
		}

		// Verify metadata preservation
		if len(messages) > 0 {
			firstMsg := messages[0]
			if firstMsg.Metadata == nil {
				t.Error("Metadata was lost")
			} else {
				t.Logf("✅ Metadata preserved:")
				t.Logf("   Source: %s", firstMsg.Metadata["source"])
				t.Logf("   Protocol: %s", firstMsg.Metadata["protocol"])
			}
		}
	})

	t.Run("ArchitectureFlow", func(t *testing.T) {
		dragonflyStore.GetClient().FlushDB(ctx)

		// Simulate complete flow: Client → Translator → Storage
		translator := kafka.NewKafkaTranslator()

		// Client sends Kafka produce request
		topic := "payments"
		payload := []byte(`{"payment_id": "pay-123", "amount": 99.99, "currency": "USD"}`)

		// 1. Translate
		portaskMsg, err := translator.TranslateProduce(topic, 0, nil, payload)
		if err != nil {
			t.Fatalf("Translation failed: %v", err)
		}

		t.Logf("Step 1: Kafka → Portask ✅")
		t.Logf("  Topic: %s", portaskMsg.Topic)
		t.Logf("  Metadata: source=%s, protocol=%s",
			portaskMsg.Metadata["source"],
			portaskMsg.Metadata["protocol"])

		// 2. Store
		err = dragonflyStore.Store(ctx, portaskMsg)
		if err != nil {
			t.Fatalf("Storage failed: %v", err)
		}

		t.Log("Step 2: Portask → Dragonfly ✅")

		// 3. Fetch
		messages, err := dragonflyStore.Fetch(ctx, types.TopicName(topic), 0, 0, 10)
		if err != nil {
			t.Fatalf("Fetch failed: %v", err)
		}

		t.Logf("Step 3: Dragonfly → Portask ✅ (%d messages)", len(messages))

		// 4. Translate back to Kafka
		kafkaResp := translator.TranslateFetchResponse(messages, nil)

		t.Logf("Step 4: Portask → Kafka ✅ (%d messages)", len(kafkaResp.Messages))

		if len(kafkaResp.Messages) == 0 {
			t.Error("No messages in Kafka response")
		} else {
			t.Logf("✅ Complete flow successful!")
			t.Logf("   Original payload: %s", string(payload))
			t.Logf("   Returned payload: %s", string(kafkaResp.Messages[0].Value))
		}
	})
}

// BenchmarkSimpleNewArchitecture benchmarks the simple flow
func BenchmarkSimpleNewArchitecture(b *testing.B) {
	// Setup Dragonfly
	dfConfig := &storage.DragonflyConfig{
		Addresses:         []string{"localhost:6379"},
		DB:                0,
		KeyPrefix:         "portask-bench-simple",
		EnableCompression: false,
	}

	ctx := context.Background()
	dragonflyStore, err := dragonfly.NewDragonflyStore(dfConfig)
	if err != nil {
		b.Skipf("Dragonfly not available: %v", err)
		return
	}

	if err := dragonflyStore.Connect(ctx); err != nil {
		b.Skipf("Dragonfly connection failed: %v", err)
		return
	}
	defer dragonflyStore.Close()

	dragonflyStore.GetClient().FlushDB(ctx)

	translator := kafka.NewKafkaTranslator()
	payload := make([]byte, 1024) // 1KB

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Translate
		msg, _ := translator.TranslateProduce("bench-topic", 0, nil, payload)

		// Store
		dragonflyStore.Store(ctx, msg)
	}

	b.StopTimer()

	// Report metrics
	stats, _ := dragonflyStore.Stats(ctx)
	b.ReportMetric(float64(stats.TotalOperations), "total_ops")
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "msgs/sec")
}
