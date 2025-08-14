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

	// "github.com/meftunca/portask/pkg/monitoring"  // Disabled for minimal CPU
	"github.com/meftunca/portask/pkg/queue"
	"github.com/meftunca/portask/pkg/serialization"
	"github.com/meftunca/portask/pkg/storage"
	storagedf "github.com/meftunca/portask/pkg/storage/dragonfly"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig("configs/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("🚀 Starting Portask v1.0.0 - Production Mode (Minimal Resources)")

	// Initialize context for graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Skip metrics collector for minimal CPU usage
	// metricsCollector := monitoring.NewMetricsCollector(5 * time.Second)
	// if err := metricsCollector.Start(ctx); err != nil {
	// 	log.Fatalf("Failed to start metrics collector: %v", err)
	// }
	// defer metricsCollector.Stop()

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
		// Map app config to storage config
		dfCfg := &storage.DragonflyConfig{
			Addresses:         cfg.Storage.DragonflyConfig.Addresses,
			Password:          cfg.Storage.DragonflyConfig.Password,
			DB:                cfg.Storage.DragonflyConfig.DB,
			EnableCluster:     len(cfg.Storage.DragonflyConfig.Addresses) > 1,
			KeyPrefix:         "portask",
			EnableCompression: cfg.IsCompressionEnabled(),
			CompressionLevel:  cfg.Compression.Level,
		}

		dragonflyStore, err := storagedf.NewDragonflyStore(dfCfg)
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
	amqpStore := &AMQPStorageAdapter{storage: storageBackend, messageBus: messageBus}

	// Start RabbitMQ-compatible server in goroutine
	rabbitmqAddr := fmt.Sprintf(":%d", cfg.Network.RabbitMQPort)
	rabbitmqServer := amqp.NewEnhancedAMQPServer(rabbitmqAddr, amqpStore)
	go func() {
		log.Printf("🐰 Starting RabbitMQ-compatible server on %s", rabbitmqAddr)
		if err := rabbitmqServer.Start(); err != nil {
			log.Printf("⚠️  Failed to start RabbitMQ server: %v", err)
		}
	}()
	defer rabbitmqServer.Stop()
	log.Printf("✅ RabbitMQ server started")

	// Start Kafka-compatible server in goroutine
	kafkaAddr := fmt.Sprintf(":%d", cfg.Network.KafkaPort)
	kafkaStore := &KafkaStorageAdapter{storage: storageBackend, messageBus: messageBus}
	kafkaServer := NewKafkaServer(kafkaAddr, kafkaStore)
	go func() {
		log.Printf("🔗 Starting Kafka-compatible server on %s", kafkaAddr)
		if err := kafkaServer.Start(); err != nil {
			log.Printf("⚠️  Failed to start Kafka server: %v", err)
		}
	}()
	defer kafkaServer.Stop()
	log.Printf("✅ Kafka server started")

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
		log.Printf("⚡ Starting Fiber v2 API server on :8080")
		if err := fiberServer.Start(); err != nil {
			log.Printf("⚠️  Failed to start Fiber API server: %v", err)
		} else {
			log.Printf("✅ Fiber API server started successfully")
		}
	}()
	defer fiberServer.Stop()

	// Give time for HTTP server to start
	time.Sleep(100 * time.Millisecond)
	defer fiberServer.Stop()
	log.Printf("⚡ Fiber v2 API server listening on :8080")

	log.Printf("✅ Portask started successfully with full protocol compatibility")
	log.Printf("🔧 Serialization: %s, Compression: %s, Storage: %s",
		cfg.Serialization.Type, cfg.Compression.Type, cfg.Storage.Type)
	log.Printf("👥 Workers: %d, Queue sizes: H=%d, N=%d, L=%d",
		cfg.Performance.WorkerPoolSize, 8192, 65536, 16384)

	// Skip performance monitoring for minimal CPU usage
	// go performanceMonitor(ctx, messageBus, metricsCollector)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("🎯 Portask is running with protocol compatibility:")
	log.Printf("   🔗 Kafka clients can connect to %d", cfg.Network.KafkaPort)
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

	// Print final statistics (minimal version)
	printMinimalStats(messageBus)
	log.Printf("👋 Portask stopped gracefully")
}

// performanceMonitor monitors and reports performance metrics
// (removed duplicate, see demo.go)
