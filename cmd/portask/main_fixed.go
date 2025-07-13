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
	"github.com/meftunca/portask/pkg/monitoring"
	"github.com/meftunca/portask/pkg/queue"
	"github.com/meftunca/portask/pkg/serialization"
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
	Version   = "1.0.0"
	BuildTime = "2024-01-01T00:00:00Z"
	GitCommit = "phase1-complete"
)
