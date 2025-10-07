package benchmarks

import (
	"testing"

	"github.com/meftunca/portask/pkg/amqp"
	"github.com/meftunca/portask/pkg/kafka"
)

// BenchmarkKafkaTranslator benchmarks Kafka translator performance
func BenchmarkKafkaTranslator(b *testing.B) {
	translator := kafka.NewKafkaTranslator()
	topic := "benchmark-topic"
	partition := int32(0)
	key := []byte("bench-key")
	value := make([]byte, 1024) // 1KB payload

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := translator.TranslateProduce(topic, partition, key, value)
		if err != nil {
			b.Fatal(err)
		}
	}

	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "translations/sec")
}

// BenchmarkAMQPTranslator benchmarks AMQP translator performance
func BenchmarkAMQPTranslator(b *testing.B) {
	translator := amqp.NewAMQPTranslator()
	exchange := "benchmark-exchange"
	routingKey := "benchmark.key"
	body := make([]byte, 1024) // 1KB payload
	props := &amqp.MessageProperties{
		ContentType:   "application/octet-stream",
		CorrelationID: "bench-corr-id",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := translator.TranslatePublish(exchange, routingKey, body, props)
		if err != nil {
			b.Fatal(err)
		}
	}

	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "translations/sec")
}

// BenchmarkTranslatorComparison compares Kafka vs AMQP translator performance
func BenchmarkTranslatorComparison(b *testing.B) {
	payload := make([]byte, 1024) // 1KB

	b.Run("Kafka", func(b *testing.B) {
		translator := kafka.NewKafkaTranslator()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			translator.TranslateProduce("topic", 0, nil, payload)
		}
	})

	b.Run("AMQP", func(b *testing.B) {
		translator := amqp.NewAMQPTranslator()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			translator.TranslatePublish("", "routing.key", payload, nil)
		}
	})
}

// BenchmarkTranslatorPayloadSizes benchmarks different payload sizes
func BenchmarkTranslatorPayloadSizes(b *testing.B) {
	translator := kafka.NewKafkaTranslator()

	sizes := []int{
		64,     // 64 bytes
		256,    // 256 bytes
		1024,   // 1 KB
		4096,   // 4 KB
		16384,  // 16 KB
		65536,  // 64 KB
		262144, // 256 KB
	}

	for _, size := range sizes {
		payload := make([]byte, size)

		b.Run(formatSize(size), func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				translator.TranslateProduce("topic", 0, nil, payload)
			}
		})
	}
}

// BenchmarkMetadataOverhead benchmarks metadata handling overhead
func BenchmarkMetadataOverhead(b *testing.B) {
	b.Run("Kafka_NoMetadata", func(b *testing.B) {
		translator := kafka.NewKafkaTranslator()
		payload := make([]byte, 1024)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			translator.TranslateProduce("topic", 0, nil, payload)
		}
	})

	b.Run("AMQP_NoProperties", func(b *testing.B) {
		translator := amqp.NewAMQPTranslator()
		payload := make([]byte, 1024)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			translator.TranslatePublish("", "key", payload, nil)
		}
	})

	b.Run("AMQP_FullProperties", func(b *testing.B) {
		translator := amqp.NewAMQPTranslator()
		payload := make([]byte, 1024)
		props := &amqp.MessageProperties{
			ContentType:     "application/json",
			ContentEncoding: "utf-8",
			CorrelationID:   "corr-123",
			ReplyTo:         "reply-queue",
			MessageID:       "msg-456",
			AppID:           "app-789",
			UserID:          "user-001",
			Priority:        5,
			DeliveryMode:    2,
		}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			translator.TranslatePublish("ex", "key", payload, props)
		}
	})
}

// BenchmarkConcurrentTranslation benchmarks concurrent translation
func BenchmarkConcurrentTranslation(b *testing.B) {
	translator := kafka.NewKafkaTranslator()
	payload := make([]byte, 1024)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			translator.TranslateProduce("topic", 0, nil, payload)
		}
	})

	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "translations/sec")
}

// Helper function to format size
func formatSize(bytes int) string {
	if bytes < 1024 {
		return string(rune(bytes)) + "B"
	} else if bytes < 1024*1024 {
		return string(rune(bytes/1024)) + "KB"
	} else {
		return string(rune(bytes/(1024*1024))) + "MB"
	}
}
