package main

import (
	"context"
	"log"
	"time"

	"github.com/meftunca/portask/pkg/monitoring"
	"github.com/meftunca/portask/pkg/queue"
)

func performanceMonitor(ctx context.Context, bus *queue.MessageBus, collector *monitoring.MetricsCollector) {
	ticker := time.NewTicker(30 * time.Second) // Much longer interval
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			busStats := bus.GetStats()

			log.Printf("📈 Performance Report:")
			log.Printf("   Total processed: %d messages", busStats.TotalMessages)

			// Only log if we have queue stats
			if len(busStats.QueueStats) > 0 {
				for name, stats := range busStats.QueueStats {
					if stats.Size > 0 { // Only log non-empty queues
						log.Printf("   Queue %s: %d items", name, stats.Size)
					}
				}
			}
		}
	}
}

func printFinalStats(bus *queue.MessageBus, collector *monitoring.MetricsCollector) {
	stats := bus.GetStats()
	log.Printf("📊 Final Statistics:")
	log.Printf("   Total Messages: %d", stats.TotalMessages)
	log.Printf("   Total Bytes: %.2f MB", float64(stats.TotalBytes)/(1024*1024))
	log.Printf("   Messages/sec: %.0f", stats.MessagesPerSecond)

	for name, queueStats := range stats.QueueStats {
		log.Printf("   Queue %s: %d enqueued, %d dequeued, %d dropped",
			name, queueStats.EnqueueCount, queueStats.DequeueCount, queueStats.DropCount)
	}
}

func printMinimalStats(bus *queue.MessageBus) {
	stats := bus.GetStats()
	log.Printf("📊 Server Statistics:")
	log.Printf("   Total Messages: %d", stats.TotalMessages)
	log.Printf("   Server stopped successfully")
}
