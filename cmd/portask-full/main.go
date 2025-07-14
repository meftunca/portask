package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/meftunca/portask/pkg/amqp"
	"github.com/meftunca/portask/pkg/api"
	"github.com/meftunca/portask/pkg/compression"
	"github.com/meftunca/portask/pkg/config"
	portaskjson "github.com/meftunca/portask/pkg/json"
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

	log.Printf("🚀 Starting Portask v1.0.0 - Full Protocol Compatibility")
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

	// Initialize storage backend (Dragonfly for production, in-memory for demo)
	var storageBackend storage.MessageStore
	if cfg.Storage.Type == "dragonfly" {
		dragonflyConfig := &storage.DragonflyStorageConfig{
			Addr:         "localhost:6379", // Default Dragonfly address
			Password:     "",
			DB:           0,
			PoolSize:     10,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		}

		dragonflyStore, err := storage.NewDragonflyStorage(dragonflyConfig)
		if err != nil {
			log.Printf("⚠️  Failed to connect to Dragonfly, falling back to in-memory: %v", err)
			storageBackend = NewInMemoryStore()
		} else {
			log.Printf("✅ Connected to Dragonfly storage backend")
			storageBackend = dragonflyStore
		}
	} else {
		log.Printf("📦 Using in-memory storage for demo")
		storageBackend = NewInMemoryStore()
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

	// Create protocol adapters
	kafkaStore := &KafkaStorageAdapter{storage: storageBackend, messageBus: messageBus}
	amqpStore := &AMQPStorageAdapter{storage: storageBackend, messageBus: messageBus}

	// Start Kafka-compatible server
	kafkaAddr := fmt.Sprintf(":%d", cfg.Network.KafkaPort)
	kafkaServer := kafka.NewKafkaServer(kafkaAddr, kafkaStore)
	if err := kafkaServer.Start(); err != nil {
		log.Printf("⚠️  Failed to start Kafka server: %v", err)
	} else {
		log.Printf("🔗 Kafka-compatible server listening on %s", kafkaAddr)
		defer kafkaServer.Stop()
	}

	// Start RabbitMQ-compatible server
	rabbitmqAddr := fmt.Sprintf(":%d", cfg.Network.RabbitMQPort)
	rabbitmqServer := amqp.NewEnhancedAMQPServer(rabbitmqAddr, amqpStore)
	if err := rabbitmqServer.Start(); err != nil {
		log.Printf("⚠️  Failed to start RabbitMQ server: %v", err)
	} else {
		log.Printf("🐰 RabbitMQ-compatible server listening on %s", rabbitmqAddr)
		defer rabbitmqServer.Stop()
	}

	// Start Fiber v2 HTTP API Server
	fiberConfig := api.FiberConfig{
		EnableCORS:      true,
		EnableLogger:    true,
		EnableRecover:   true,
		EnableRequestID: true,
		JSONConfig: portaskjson.Config{
			Library:    portaskjson.JSONLibrary(cfg.Serialization.Type),
			Compact:    true,
			EscapeHTML: false,
		},
	}

	fiberServer := api.NewFiberServer(fiberConfig, nil, storageBackend)
	go func() {
		if err := fiberServer.Start(); err != nil {
			log.Printf("⚠️  Failed to start Fiber API server: %v", err)
		}
	}()
	defer fiberServer.Stop()
	log.Printf("⚡ Fiber v2 API server listening on :8080")

	log.Printf("✅ Portask started successfully with full protocol compatibility")
	log.Printf("🔧 Serialization: %s, Compression: %s, Storage: %s",
		cfg.Serialization.Type, cfg.Compression.Type, cfg.Storage.Type)
	log.Printf("👥 Workers: %d, Queue sizes: H=%d, N=%d, L=%d",
		cfg.Performance.WorkerPoolSize, 8192, 65536, 16384)

	// Start performance monitoring
	go performanceMonitor(ctx, messageBus, metricsCollector)

	// Start demo message generation
	go demoMessageGenerator(ctx, messageBus, 1000) // 1K messages per second for demo

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("🎯 Portask is running with protocol compatibility:")
	log.Printf("   🔗 Kafka clients can connect to %s", kafkaAddr)
	log.Printf("   🐰 RabbitMQ clients can connect to %s", rabbitmqAddr)
	log.Printf("   📊 Metrics available for monitoring")
	log.Printf("   ⏹️  Press Ctrl+C to stop")

	<-sigChan

	log.Printf("🛑 Shutting down Portask...")
	cancel()

	// Cleanup storage
	if err := storageBackend.Close(); err != nil {
		log.Printf("⚠️  Error closing storage: %v", err)
	}

	// Print final statistics
	printFinalStats(messageBus, metricsCollector)
	log.Printf("👋 Portask stopped gracefully")
}

// InMemoryStore simple in-memory storage for demo
type InMemoryStore struct {
	messages map[string]*types.PortaskMessage
	topics   map[string]*types.TopicInfo
	offsets  map[string]*types.ConsumerOffset
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		messages: make(map[string]*types.PortaskMessage),
		topics:   make(map[string]*types.TopicInfo),
		offsets:  make(map[string]*types.ConsumerOffset),
	}
}

// Implement required MessageStore methods for InMemoryStore
func (s *InMemoryStore) Store(ctx context.Context, message *types.PortaskMessage) error {
	s.messages[string(message.ID)] = message
	return nil
}

func (s *InMemoryStore) StoreBatch(ctx context.Context, batch *types.MessageBatch) error {
	for _, msg := range batch.Messages {
		s.messages[string(msg.ID)] = msg
	}
	return nil
}

func (s *InMemoryStore) Fetch(ctx context.Context, topic types.TopicName, partition int32, offset int64, limit int) ([]*types.PortaskMessage, error) {
	var messages []*types.PortaskMessage
	for _, msg := range s.messages {
		if msg.Topic == topic {
			messages = append(messages, msg)
			if len(messages) >= limit {
				break
			}
		}
	}
	return messages, nil
}

func (s *InMemoryStore) FetchByID(ctx context.Context, messageID types.MessageID) (*types.PortaskMessage, error) {
	if msg, ok := s.messages[string(messageID)]; ok {
		return msg, nil
	}
	return nil, fmt.Errorf("message not found: %s", messageID)
}

func (s *InMemoryStore) Delete(ctx context.Context, messageID types.MessageID) error {
	delete(s.messages, string(messageID))
	return nil
}

func (s *InMemoryStore) DeleteBatch(ctx context.Context, messageIDs []types.MessageID) error {
	for _, id := range messageIDs {
		delete(s.messages, string(id))
	}
	return nil
}

func (s *InMemoryStore) CreateTopic(ctx context.Context, topicInfo *types.TopicInfo) error {
	s.topics[string(topicInfo.Name)] = topicInfo
	return nil
}

func (s *InMemoryStore) DeleteTopic(ctx context.Context, topic types.TopicName) error {
	delete(s.topics, string(topic))
	return nil
}

func (s *InMemoryStore) GetTopicInfo(ctx context.Context, topic types.TopicName) (*types.TopicInfo, error) {
	if info, ok := s.topics[string(topic)]; ok {
		return info, nil
	}
	return nil, fmt.Errorf("topic not found: %s", topic)
}

func (s *InMemoryStore) ListTopics(ctx context.Context) ([]*types.TopicInfo, error) {
	topics := make([]*types.TopicInfo, 0, len(s.topics))
	for _, topic := range s.topics {
		topics = append(topics, topic)
	}
	return topics, nil
}

func (s *InMemoryStore) TopicExists(ctx context.Context, topic types.TopicName) (bool, error) {
	_, exists := s.topics[string(topic)]
	return exists, nil
}

func (s *InMemoryStore) GetPartitionInfo(ctx context.Context, topic types.TopicName, partition int32) (*types.PartitionInfo, error) {
	return &types.PartitionInfo{
		Topic:     topic,
		Partition: partition,
		Leader:    "node-0",
		Replicas:  []string{"node-0"},
		ISR:       []string{"node-0"},
	}, nil
}

func (s *InMemoryStore) GetPartitionCount(ctx context.Context, topic types.TopicName) (int32, error) {
	return 1, nil
}

func (s *InMemoryStore) GetLatestOffset(ctx context.Context, topic types.TopicName, partition int32) (int64, error) {
	count := 0
	for _, msg := range s.messages {
		if msg.Topic == topic {
			count++
		}
	}
	return int64(count), nil
}

func (s *InMemoryStore) GetEarliestOffset(ctx context.Context, topic types.TopicName, partition int32) (int64, error) {
	return 0, nil
}

func (s *InMemoryStore) CommitOffset(ctx context.Context, offset *types.ConsumerOffset) error {
	key := fmt.Sprintf("%s_%s_%d", offset.ConsumerID, offset.Topic, offset.Partition)
	s.offsets[key] = offset
	return nil
}

func (s *InMemoryStore) CommitOffsetBatch(ctx context.Context, offsets []*types.ConsumerOffset) error {
	for _, offset := range offsets {
		key := fmt.Sprintf("%s_%s_%d", offset.ConsumerID, offset.Topic, offset.Partition)
		s.offsets[key] = offset
	}
	return nil
}

func (s *InMemoryStore) GetOffset(ctx context.Context, consumerID types.ConsumerID, topic types.TopicName, partition int32) (*types.ConsumerOffset, error) {
	key := fmt.Sprintf("%s_%s_%d", consumerID, topic, partition)
	if offset, ok := s.offsets[key]; ok {
		return offset, nil
	}
	return &types.ConsumerOffset{
		ConsumerID: consumerID,
		Topic:      topic,
		Partition:  partition,
		Offset:     0,
	}, nil
}

func (s *InMemoryStore) GetConsumerOffsets(ctx context.Context, consumerID types.ConsumerID) ([]*types.ConsumerOffset, error) {
	var offsets []*types.ConsumerOffset
	for _, offset := range s.offsets {
		if offset.ConsumerID == consumerID {
			offsets = append(offsets, offset)
		}
	}
	return offsets, nil
}

func (s *InMemoryStore) ListConsumers(ctx context.Context, topic types.TopicName) ([]types.ConsumerID, error) {
	consumerMap := make(map[types.ConsumerID]bool)
	for _, offset := range s.offsets {
		if offset.Topic == topic {
			consumerMap[offset.ConsumerID] = true
		}
	}

	consumers := make([]types.ConsumerID, 0, len(consumerMap))
	for consumerID := range consumerMap {
		consumers = append(consumers, consumerID)
	}
	return consumers, nil
}

func (s *InMemoryStore) Ping(ctx context.Context) error {
	return nil
}

func (s *InMemoryStore) Stats(ctx context.Context) (*storage.StorageStats, error) {
	return &storage.StorageStats{
		MessageCount:    int64(len(s.messages)),
		TopicCount:      int64(len(s.topics)),
		Status:          "connected",
		LastHealthCheck: time.Now(),
	}, nil
}

func (s *InMemoryStore) Cleanup(ctx context.Context, retentionPolicy *storage.RetentionPolicy) error {
	return nil // No cleanup needed for in-memory
}

func (s *InMemoryStore) Close() error {
	return nil
}

// KafkaStorageAdapter adapts storage interface to Kafka MessageStore
type KafkaStorageAdapter struct {
	storage    storage.MessageStore
	messageBus *queue.MessageBus
}

func (k *KafkaStorageAdapter) ProduceMessage(topic string, partition int32, key, value []byte) (int64, error) {
	message := &types.PortaskMessage{
		ID:        types.MessageID(fmt.Sprintf("kafka-%d", time.Now().UnixNano())),
		Topic:     types.TopicName(topic),
		Timestamp: time.Now().Unix(),
		Payload:   value,
	}

	// Store in backend
	ctx := context.Background()
	if err := k.storage.Store(ctx, message); err != nil {
		return 0, err
	}

	// Also publish to message bus for real-time processing
	if err := k.messageBus.Publish(message); err != nil {
		log.Printf("⚠️  Failed to publish to message bus: %v", err)
	}

	return time.Now().UnixNano(), nil
}

func (k *KafkaStorageAdapter) ConsumeMessages(topic string, partition int32, offset int64, maxBytes int32) ([]*kafka.Message, error) {
	ctx := context.Background()
	messages, err := k.storage.Fetch(ctx, types.TopicName(topic), partition, offset, 100)
	if err != nil {
		return nil, err
	}

	kafkaMessages := make([]*kafka.Message, len(messages))
	for i, msg := range messages {
		kafkaMessages[i] = &kafka.Message{
			Offset:    int64(i) + offset,
			Key:       []byte(msg.ID),
			Value:     msg.Payload,
			Timestamp: time.Unix(msg.Timestamp, 0),
		}
	}

	return kafkaMessages, nil
}

func (k *KafkaStorageAdapter) GetTopicMetadata(topics []string) (*kafka.TopicMetadata, error) {
	if len(topics) == 0 {
		return &kafka.TopicMetadata{
			Name: "default-topic",
			Partitions: []kafka.PartitionMetadata{
				{ID: 0, Leader: 0},
			},
		}, nil
	}

	return &kafka.TopicMetadata{
		Name: topics[0],
		Partitions: []kafka.PartitionMetadata{
			{ID: 0, Leader: 0},
		},
	}, nil
}

func (k *KafkaStorageAdapter) CreateTopic(topic string, partitions int32, replication int16) error {
	ctx := context.Background()
	topicInfo := &types.TopicInfo{
		Name:              types.TopicName(topic),
		Partitions:        partitions,
		ReplicationFactor: replication,
		CreatedAt:         time.Now().Unix(),
	}

	log.Printf("📝 Kafka CreateTopic: %s (partitions: %d)", topic, partitions)
	return k.storage.CreateTopic(ctx, topicInfo)
}

func (k *KafkaStorageAdapter) DeleteTopic(topic string) error {
	ctx := context.Background()
	log.Printf("🗑️ Kafka DeleteTopic: %s", topic)
	return k.storage.DeleteTopic(ctx, types.TopicName(topic))
}

// AMQPStorageAdapter adapts storage interface to AMQP MessageStore
type AMQPStorageAdapter struct {
	storage    storage.MessageStore
	messageBus *queue.MessageBus
}

func (a *AMQPStorageAdapter) DeclareExchange(name, exchangeType string, durable, autoDelete bool) error {
	log.Printf("📝 AMQP DeclareExchange: %s (type: %s)", name, exchangeType)
	return nil
}

func (a *AMQPStorageAdapter) DeleteExchange(name string) error {
	log.Printf("🗑️ AMQP DeleteExchange: %s", name)
	return nil
}

func (a *AMQPStorageAdapter) DeclareQueue(name string, durable, autoDelete, exclusive bool) error {
	log.Printf("📝 AMQP DeclareQueue: %s", name)
	ctx := context.Background()
	topicInfo := &types.TopicInfo{
		Name:              types.TopicName(name),
		Partitions:        1,
		ReplicationFactor: 1,
		CreatedAt:         time.Now().Unix(),
	}
	return a.storage.CreateTopic(ctx, topicInfo)
}

func (a *AMQPStorageAdapter) DeleteQueue(name string) error {
	log.Printf("🗑️ AMQP DeleteQueue: %s", name)
	ctx := context.Background()
	return a.storage.DeleteTopic(ctx, types.TopicName(name))
}

func (a *AMQPStorageAdapter) BindQueue(queueName, exchangeName, routingKey string) error {
	log.Printf("🔗 AMQP BindQueue: %s -> %s (%s)", queueName, exchangeName, routingKey)
	return nil
}

func (a *AMQPStorageAdapter) PublishMessage(exchange, routingKey string, body []byte) error {
	message := &types.PortaskMessage{
		ID:        types.MessageID(fmt.Sprintf("amqp-%d", time.Now().UnixNano())),
		Topic:     types.TopicName(fmt.Sprintf("%s.%s", exchange, routingKey)),
		Timestamp: time.Now().Unix(),
		Payload:   body,
	}

	ctx := context.Background()
	if err := a.storage.Store(ctx, message); err != nil {
		return err
	}

	log.Printf("📤 AMQP PublishMessage: %s/%s (%d bytes)", exchange, routingKey, len(body))
	return a.messageBus.Publish(message)
}

func (a *AMQPStorageAdapter) ConsumeMessages(queueName string, autoAck bool) ([][]byte, error) {
	ctx := context.Background()
	messages, err := a.storage.Fetch(ctx, types.TopicName(queueName), 0, 0, 10)
	if err != nil {
		return nil, err
	}

	result := make([][]byte, len(messages))
	for i, msg := range messages {
		result[i] = msg.Payload
	}

	log.Printf("📥 AMQP ConsumeMessages: %s (%d messages)", queueName, len(result))
	return result, nil
}

// Additional methods to implement MessageStore interface
func (a *AMQPStorageAdapter) StoreMessage(topic string, message []byte) error {
	ctx := context.Background()
	msg := &types.PortaskMessage{
		ID:        types.MessageID(fmt.Sprintf("amqp_%d", time.Now().UnixNano())),
		Topic:     types.TopicName(topic),
		Payload:   message,
		Timestamp: time.Now().Unix(),
	}
	return a.storage.Store(ctx, msg)
}

func (a *AMQPStorageAdapter) GetMessages(topic string, offset int64) ([][]byte, error) {
	ctx := context.Background()
	messages, err := a.storage.Fetch(ctx, types.TopicName(topic), 0, offset, 100)
	if err != nil {
		return nil, err
	}

	result := make([][]byte, len(messages))
	for i, msg := range messages {
		result[i] = msg.Payload
	}
	return result, nil
}

func (a *AMQPStorageAdapter) GetTopics() []string {
	// Return empty slice for now since storage doesn't have GetTopics method
	return []string{}
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
			busStats := bus.GetStats()

			log.Printf("📈 Performance Report:")
			log.Printf("   Messages/sec: %.0f | Bytes/sec: %.0f KB",
				busStats.MessagesPerSecond, busStats.BytesPerSecond/1024)
			log.Printf("   Total processed: %d messages",
				busStats.TotalMessages)
			log.Printf("   Queue depths - H:%d N:%d L:%d",
				busStats.QueueStats["high-priority"].Size,
				busStats.QueueStats["normal-priority"].Size,
				busStats.QueueStats["low-priority"].Size)
		}
	}
}

// demoMessageGenerator generates demo messages for testing
func demoMessageGenerator(ctx context.Context, bus *queue.MessageBus, messagesPerSecond int) {
	log.Printf("🎮 Demo message generator: %d msg/sec", messagesPerSecond)

	ticker := time.NewTicker(time.Second / time.Duration(messagesPerSecond))
	defer ticker.Stop()

	counter := 0
	topics := []types.TopicName{"orders", "payments", "notifications", "analytics", "events"}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			counter++

			message := &types.PortaskMessage{
				ID:        types.MessageID(fmt.Sprintf("demo-%d-%d", time.Now().Unix(), counter)),
				Topic:     topics[counter%len(topics)],
				Timestamp: time.Now().Unix(),
				Payload:   []byte(fmt.Sprintf("Demo message #%d - timestamp: %d", counter, time.Now().Unix())),
			}

			if err := bus.Publish(message); err != nil {
				log.Printf("❌ Demo message publish failed: %v", err)
			}
		}
	}
}

// printFinalStats prints final performance statistics
func printFinalStats(bus *queue.MessageBus, collector *monitoring.MetricsCollector) {
	stats := bus.GetStats()

	log.Printf("📊 Final Performance Summary:")
	log.Printf("═══════════════════════════════════════")
	log.Printf("Total Messages: %d", stats.TotalMessages)
	log.Printf("Total Bytes: %d MB", stats.TotalBytes/1024/1024)
	log.Printf("Peak Throughput: %.0f msg/sec", stats.MessagesPerSecond)
	log.Printf("Peak Bandwidth: %.0f KB/sec", stats.BytesPerSecond/1024)

	log.Printf("\nQueue Performance:")
	for name, qStats := range stats.QueueStats {
		log.Printf("  %s: %d processed, %d dropped, %d remaining",
			name, qStats.DequeueCount, qStats.DropCount, qStats.Size)
	}

	log.Printf("\nWorker Performance:")
	for _, wStats := range stats.WorkerStats {
		log.Printf("  Worker-%d: %d processed, %d errors, %v avg",
			wStats.ID, wStats.MessagesProcessed, wStats.ErrorCount, wStats.AvgProcessingTime)
	}

	if len(stats.TopicStats) > 0 {
		log.Printf("\nTopic Distribution:")
		for topic, count := range stats.TopicStats {
			log.Printf("  %s: %d messages", topic, count)
		}
	}

	log.Printf("═══════════════════════════════════════")
}
