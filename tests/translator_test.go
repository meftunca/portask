package tests

import (
	"testing"

	"github.com/meftunca/portask/pkg/amqp"
	"github.com/meftunca/portask/pkg/kafka"
	"github.com/meftunca/portask/pkg/types"
)

// TestKafkaTranslator tests Kafka protocol translator
func TestKafkaTranslator(t *testing.T) {
	translator := kafka.NewKafkaTranslator()

	t.Run("TranslateProduce_Success", func(t *testing.T) {
		msg, err := translator.TranslateProduce("orders", 0, []byte("key1"), []byte("value1"))
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if msg.Topic != types.TopicName("orders") {
			t.Errorf("Expected topic 'orders', got '%s'", msg.Topic)
		}

		if string(msg.Payload) != "value1" {
			t.Errorf("Expected payload 'value1', got '%s'", msg.Payload)
		}

		if msg.Metadata["source"] != "kafka" {
			t.Errorf("Expected source 'kafka', got '%s'", msg.Metadata["source"])
		}
	})

	t.Run("TranslateProduce_EmptyTopic", func(t *testing.T) {
		_, err := translator.TranslateProduce("", 0, nil, []byte("test"))
		if err == nil {
			t.Error("Expected error for empty topic")
		}
	})

	t.Run("TranslateFetch_Success", func(t *testing.T) {
		req, err := translator.TranslateFetch("orders", 0, 100, 1024)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if req.Topic != types.TopicName("orders") {
			t.Errorf("Expected topic 'orders', got '%s'", req.Topic)
		}

		if req.Offset != 100 {
			t.Errorf("Expected offset 100, got %d", req.Offset)
		}
	})
}

// TestAMQPTranslator tests AMQP protocol translator
func TestAMQPTranslator(t *testing.T) {
	translator := amqp.NewAMQPTranslator()

	t.Run("TranslatePublish_Success", func(t *testing.T) {
		props := &amqp.MessageProperties{
			ContentType:   "application/json",
			CorrelationID: "corr-123",
			Priority:      5,
		}

		msg, err := translator.TranslatePublish("exchange1", "routing.key", []byte("payload"), props)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if string(msg.Topic) != "exchange1.routing.key" {
			t.Errorf("Expected topic 'exchange1.routing.key', got '%s'", msg.Topic)
		}

		if string(msg.Payload) != "payload" {
			t.Errorf("Expected payload 'payload', got '%s'", msg.Payload)
		}

		if msg.Metadata["source"] != "amqp" {
			t.Errorf("Expected source 'amqp', got '%s'", msg.Metadata["source"])
		}

		if msg.Metadata["content_type"] != "application/json" {
			t.Errorf("Expected content_type 'application/json', got '%s'", msg.Metadata["content_type"])
		}
	})

	t.Run("TranslatePublish_EmptyRoutingKey", func(t *testing.T) {
		_, err := translator.TranslatePublish("exchange1", "", []byte("test"), nil)
		if err == nil {
			t.Error("Expected error for empty routing key")
		}
	})

	t.Run("TranslateConsume_Success", func(t *testing.T) {
		req, err := translator.TranslateConsume("queue1", "consumer-tag", false, false, false, false)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if req.Topic != types.TopicName("queue1") {
			t.Errorf("Expected topic 'queue1', got '%s'", req.Topic)
		}
	})
}

// TestTranslatorConsistency verifies both translators produce consistent Portask messages
func TestTranslatorConsistency(t *testing.T) {
	kafkaTranslator := kafka.NewKafkaTranslator()
	amqpTranslator := amqp.NewAMQPTranslator()

	// Translate same logical message from both protocols
	kafkaMsg, _ := kafkaTranslator.TranslateProduce("test", 0, nil, []byte("data"))
	amqpMsg, _ := amqpTranslator.TranslatePublish("", "test", []byte("data"), nil)

	// Both should have Portask protocol metadata
	if kafkaMsg.Metadata == nil {
		t.Error("Kafka message missing metadata")
	}

	if amqpMsg.Metadata == nil {
		t.Error("AMQP message missing metadata")
	}

	// Both should have source identifier
	if kafkaMsg.Metadata["source"] == "" {
		t.Error("Kafka message missing source")
	}

	if amqpMsg.Metadata["source"] == "" {
		t.Error("AMQP message missing source")
	}

	// Both should have valid timestamps
	if kafkaMsg.Timestamp <= 0 {
		t.Error("Kafka message has invalid timestamp")
	}

	if amqpMsg.Timestamp <= 0 {
		t.Error("AMQP message has invalid timestamp")
	}

	t.Logf("✅ Both translators produce consistent Portask messages")
	t.Logf("   Kafka source: %s", kafkaMsg.Metadata["source"])
	t.Logf("   AMQP source: %s", amqpMsg.Metadata["source"])
}

// TestMetadataPreservation verifies protocol-specific metadata is preserved
func TestMetadataPreservation(t *testing.T) {
	t.Run("Kafka_Metadata", func(t *testing.T) {
		translator := kafka.NewKafkaTranslator()
		msg, _ := translator.TranslateProduce("topic1", 5, []byte("key"), []byte("value"))

		// Kafka-specific metadata should be preserved
		if msg.Partition != 5 {
			t.Errorf("Partition not preserved: expected 5, got %d", msg.Partition)
		}

		if msg.Metadata["protocol"] != "kafka-wire" {
			t.Error("Kafka protocol marker not set")
		}
	})

	t.Run("AMQP_Metadata", func(t *testing.T) {
		translator := amqp.NewAMQPTranslator()
		props := &amqp.MessageProperties{
			ContentType:   "text/plain",
			CorrelationID: "abc-123",
			ReplyTo:       "reply-queue",
		}

		msg, _ := translator.TranslatePublish("ex1", "rk1", []byte("data"), props)

		// AMQP-specific metadata should be preserved
		if msg.Metadata["content_type"] != "text/plain" {
			t.Error("Content type not preserved")
		}

		if msg.Metadata["correlation_id"] != "abc-123" {
			t.Error("Correlation ID not preserved")
		}

		if msg.Metadata["reply_to"] != "reply-queue" {
			t.Error("Reply-to not preserved")
		}

		if msg.Metadata["exchange"] != "ex1" {
			t.Error("Exchange not preserved")
		}

		if msg.Metadata["routing_key"] != "rk1" {
			t.Error("Routing key not preserved")
		}
	})
}
