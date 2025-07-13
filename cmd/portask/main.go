package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/meftunca/portask/pkg/compression"
	"github.com/meftunca/portask/pkg/config"
	"github.com/meftunca/portask/pkg/amqp"
	"github.com/meftunca/portask/pkg/kafka"
	"github.com/meftunca/portask/pkg/monitoring"
	"github.com/meftunca/portask/pkg/queue"
	"github.com/meftunca/portask/pkg/serialization"
	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/types"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig("configs/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("🚀 Starting Portask v1.0.0")
	log.Printf("📊 Target Performance: >%d messages/sec, <%dMB memory, <%s latency",
		1000000, 50, "1ms")

	// Initialize performance monitoring
	metricsCollector := monitoring.NewMetricsCollector(5 * time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := metricsCollector.Start(ctx); err != nil {
		log.Fatalf("Failed to start metrics collector: %v", err)
	}
	defer metricsCollector.Stop()

	// Initialize serialization
	codecFactory := serialization.NewCodecFactory()
	if err := codecFactory.InitializeDefaultCodecs(cfg); err != nil {
		log.Fatalf("Failed to initialize codecs: %v", err)
	}
	codec, err := codecFactory.GetCodec(cfg.Serialization.Type)
	if err != nil {
		log.Fatalf("Failed to create codec: %v", err)
	}

	// Initialize compression
	compressorFactory := compression.NewCompressorFactory()
	if err := compressorFactory.InitializeDefaultCompressors(cfg); err != nil {
		log.Fatalf("Failed to initialize compressors: %v", err)
	}
	compressor, err := compressorFactory.GetCompressor(cfg.Compression.Type)
	if err != nil {
		log.Fatalf("Failed to create compressor: %v", err)
	}

	// Initialize message processor
	processor := queue.NewDefaultMessageProcessor(codec, compressor)

	// Configure message bus with sensible defaults
	busConfig := queue.MessageBusConfig{
		HighPriorityQueueSize:   8192,  // High priority queue
		NormalPriorityQueueSize: 65536, // Normal priority queue
		LowPriorityQueueSize:    16384, // Low priority queue
		DropPolicy:              queue.DropOldest,
		WorkerPoolConfig: queue.WorkerPoolConfig{
			WorkerCount:      cfg.Performance.WorkerPoolSize,
			MessageProcessor: processor,
			BatchSize:        cfg.Performance.BatchSize,
			BatchTimeout:     1 * time.Millisecond,
			EnableProfiling:  cfg.Monitoring.EnableProfiling,
		},
		EnableTopicQueues: true,
	}

	// Create and start message bus
	messageBus := queue.NewMessageBus(busConfig)
	if err := messageBus.Start(); err != nil {
		log.Fatalf("Failed to start message bus: %v", err)
	}
	defer messageBus.Stop()

	// Initialize storage manager
	storageManager := storage.NewStorageManager(&storage.StorageConfig{
		MaxConnections:    cfg.Performance.WorkerPoolSize,
		ConnectionTimeout: 30 * time.Second,
		BatchSize:         cfg.Performance.BatchSize,
		EnableBatching:    true,
		BatchTimeout:      1 * time.Millisecond,
	})

	// Create a simple in-memory store for demo
	memoryStore := &SimpleMessageStore{messageBus: messageBus}
	storageManager.SetPrimary(memoryStore)

	// Start Kafka-compatible server
	kafkaAddr := fmt.Sprintf(":%d", cfg.Network.KafkaPort)
	kafkaServer := kafka.NewKafkaServer(kafkaAddr, memoryStore)
	if err := kafkaServer.Start(); err != nil {
		log.Printf("⚠️  Failed to start Kafka server: %v", err)
	} else {
		log.Printf("🔗 Kafka-compatible server listening on %s", kafkaAddr)
		defer kafkaServer.Stop()
	}

	// TODO: RabbitMQ server (Phase 3 - placeholder implementation)
	rabbitmqAddr := fmt.Sprintf(":%d", cfg.Network.RabbitMQPort)
	
	// Create a simple message store for RabbitMQ compatibility
	simpleAMQPStore := &SimpleAMQPStore{}
	rabbitmqServer := amqp.NewRabbitMQServer(rabbitmqAddr, simpleAMQPStore)
	if err := rabbitmqServer.Start(); err != nil {
		log.Printf("⚠️  Failed to start RabbitMQ server: %v", err)
	} else {
		log.Printf("🐰 RabbitMQ-compatible server (placeholder) listening on %s", rabbitmqAddr)
		defer rabbitmqServer.Stop()
	}

	log.Printf("✅ Portask started successfully")
	log.Printf("🔧 Serialization: %s, Compression: %s", cfg.Serialization.Type, cfg.Compression.Type)
	log.Printf("👥 Workers: %d, Queue sizes: H=%d, N=%d, L=%d",
		cfg.Performance.WorkerPoolSize,
		8192, 65536, 16384)

	// Start performance monitoring goroutine
	go performanceMonitor(ctx, messageBus, metricsCollector)

	// Start demo message generation
	go demoMessageGenerator(ctx, messageBus, 10000) // 10K messages per second

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("🎯 Portask is running. Press Ctrl+C to stop.")
	<-sigChan

	log.Printf("🛑 Shutting down Portask...")
	cancel()

	// Print final statistics
	printFinalStats(messageBus, metricsCollector)
	log.Printf("👋 Portask stopped gracefully")
}

// performanceMonitor monitors and reports performance metrics
func performanceMonitor(ctx context.Context, bus *queue.MessageBus, collector *monitoring.MetricsCollector) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Get bus statistics
			busStats := bus.GetStats()

			// Log performance metrics
			log.Printf("📈 Performance Report:")
			log.Printf("   Messages/sec: %.0f | Bytes/sec: %.0f KB",
				busStats.MessagesPerSecond, busStats.BytesPerSecond/1024)
			log.Printf("   Total messages: %d",
				busStats.TotalMessages)

			// Log queue statistics
			log.Printf("   Queue sizes - High: %d, Normal: %d, Low: %d",
				busStats.QueueStats["high-priority"].Size,
				busStats.QueueStats["normal-priority"].Size,
				busStats.QueueStats["low-priority"].Size)
		}
	}
}

// demoMessageGenerator generates demo messages for testing
func demoMessageGenerator(ctx context.Context, bus *queue.MessageBus, messagesPerSecond int) {
	log.Printf("🎮 Starting demo message generator: %d msg/sec", messagesPerSecond)

	ticker := time.NewTicker(time.Second / time.Duration(messagesPerSecond))
	defer ticker.Stop()

	counter := 0
	topics := []types.TopicName{"orders", "payments", "notifications", "analytics"}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			counter++

			// Create demo message
			message := &types.PortaskMessage{
				ID:        types.MessageID(fmt.Sprintf("demo-%d-%d", time.Now().Unix(), counter)),
				Topic:     topics[counter%len(topics)],
				Timestamp: time.Now().Unix(),
				Payload:   []byte(fmt.Sprintf("Demo message #%d with some payload data", counter)),
			}

			// Publish message
			if err := bus.Publish(message); err != nil {
				log.Printf("❌ Failed to publish demo message: %v", err)
			}
		}
	}
}

// printFinalStats prints final performance statistics
func printFinalStats(bus *queue.MessageBus, collector *monitoring.MetricsCollector) {
	stats := bus.GetStats()

	log.Printf("📊 Final Performance Statistics:")
	log.Printf("═══════════════════════════════════════")
	log.Printf("Total Messages Processed: %d", stats.TotalMessages)
	log.Printf("Total Bytes Processed: %d MB", stats.TotalBytes/1024/1024)
	log.Printf("Peak Messages/sec: %.0f", stats.MessagesPerSecond)
	log.Printf("Peak Bytes/sec: %.0f KB", stats.BytesPerSecond/1024)

	// Print queue statistics
	log.Printf("\nQueue Statistics:")
	for name, qStats := range stats.QueueStats {
		log.Printf("  %s: processed=%d, dropped=%d, final_size=%d",
			name, qStats.DequeueCount, qStats.DropCount, qStats.Size)
	}

	// Print worker statistics
	log.Printf("\nWorker Statistics:")
	for _, wStats := range stats.WorkerStats {
		log.Printf("  Worker %d: processed=%d, errors=%d, avg_time=%v",
			wStats.ID, wStats.MessagesProcessed, wStats.ErrorCount, wStats.AvgProcessingTime)
	}

	// Print topic statistics
	if len(stats.TopicStats) > 0 {
		log.Printf("\nTopic Statistics:")
		for topic, count := range stats.TopicStats {
			log.Printf("  %s: %d messages", topic, count)
		}
	}

	// Performance analysis
	log.Printf("\n🎯 Performance Analysis:")
	if stats.MessagesPerSecond >= 1000000 {
		log.Printf("✅ RPS TARGET ACHIEVED: %.0f msg/sec (≥1M target)", stats.MessagesPerSecond)
	} else {
		log.Printf("⚠️  RPS below target: %.0f msg/sec (<1M target)", stats.MessagesPerSecond)
	}

	log.Printf("═══════════════════════════════════════")
}

// Version information
const (
	PortaskVersion   = "1.0.0"
	PortaskBuildTime = "2024-01-01T00:00:00Z"
	PortaskGitCommit = "phase1-complete"
)

// SimpleMessageStore implements kafka.MessageStore for demo purposes
type SimpleMessageStore struct {
	messageBus *queue.MessageBus
}

func (s *SimpleMessageStore) ProduceMessage(topic string, partition int32, key, value []byte) (int64, error) {
	message := &types.PortaskMessage{
		ID:        types.MessageID(fmt.Sprintf("kafka-%d", time.Now().UnixNano())),
		Topic:     types.TopicName(topic),
		Timestamp: time.Now().Unix(),
		Payload:   value,
	}

	if err := s.messageBus.Publish(message); err != nil {
		return 0, err
	}

	return time.Now().UnixNano(), nil
}

func (s *SimpleMessageStore) ConsumeMessages(topic string, partition int32, offset int64, maxBytes int32) ([]*kafka.Message, error) {
	// For demo, return empty - real implementation would fetch from storage
	return []*kafka.Message{}, nil
}

func (s *SimpleMessageStore) GetTopicMetadata(topics []string) (*kafka.TopicMetadata, error) {
	// Simple metadata response for demo
	return &kafka.TopicMetadata{
		Name: "demo-topic",
		Partitions: []kafka.PartitionMetadata{
			{
				ID:     0,
				Leader: 0,
			},
		},
	}, nil
}

func (s *SimpleMessageStore) CreateTopic(topic string, partitions int32, replication int16) error {
	log.Printf("📝 CreateTopic called: %s (partitions: %d)", topic, partitions)
	return nil
}

func (s *SimpleMessageStore) DeleteTopic(topic string) error {
	log.Printf("🗑️  DeleteTopic called: %s", topic)
	return nil
}

// SimpleAMQPStore implements amqp.MessageStore for demo purposes  
type SimpleAMQPStore struct{}

func (s *SimpleAMQPStore) DeclareExchange(name, exchangeType string, durable, autoDelete bool) error {
	log.Printf("📝 DeclareExchange called: %s (type: %s)", name, exchangeType)
	return nil
}

func (s *SimpleAMQPStore) DeleteExchange(name string) error {
	log.Printf("🗑️  DeleteExchange called: %s", name)
	return nil
}

func (s *SimpleAMQPStore) DeclareQueue(name string, durable, autoDelete, exclusive bool) error {
	log.Printf("📝 DeclareQueue called: %s", name)
	return nil
}

func (s *SimpleAMQPStore) DeleteQueue(name string) error {
	log.Printf("🗑️  DeleteQueue called: %s", name)
	return nil
}

func (s *SimpleAMQPStore) BindQueue(queueName, exchangeName, routingKey string) error {
	log.Printf("🔗 BindQueue called: %s -> %s (%s)", queueName, exchangeName, routingKey)
	return nil
}

func (s *SimpleAMQPStore) PublishMessage(exchange, routingKey string, body []byte) error {
	log.Printf("📤 PublishMessage called: %s/%s (%d bytes)", exchange, routingKey, len(body))
	return nil
}

func (s *SimpleAMQPStore) ConsumeMessages(queueName string, autoAck bool) ([][]byte, error) {
	log.Printf("📥 ConsumeMessages called: %s", queueName)
	return [][]byte{}, nil
}
