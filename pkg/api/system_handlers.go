package api

import (
	"log"
	"runtime"

	"github.com/gofiber/fiber/v2"
)

// ============================================
// SYSTEM API HANDLERS
// ============================================

// handleSystemWorkers gets worker pool statistics
func (s *FiberServer) handleSystemWorkers(c *fiber.Ctx) error {
	log.Printf("[API] Get worker pool stats")

	// Get actual goroutine count
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// TODO: Get actual worker pool stats from processor
	// For now, return sample data with real system info

	workerStats := map[string]interface{}{
		"workers": map[string]interface{}{
			"total":  4,
			"active": 2,
			"idle":   2,
		},
		"queues": map[string]interface{}{
			"high": map[string]interface{}{
				"capacity": 8192,
				"size":     150,
				"usage":    1.8, // percentage
			},
			"normal": map[string]interface{}{
				"capacity": 65536,
				"size":     1200,
				"usage":    1.8,
			},
			"low": map[string]interface{}{
				"capacity": 16384,
				"size":     50,
				"usage":    0.3,
			},
		},
		"system": map[string]interface{}{
			"goroutines": runtime.NumGoroutine(),
			"cpus":       runtime.NumCPU(),
			"gc_count":   m.NumGC,
		},
		"performance": map[string]interface{}{
			"throughput": 1500, // msgs/sec
			"latency": map[string]interface{}{
				"p50": 2.5,
				"p95": 8.3,
				"p99": 15.2,
			},
		},
	}

	return c.JSON(fiber.Map{
		"success": true,
		"workers": workerStats,
	})
}

// handleSystemStorage gets storage backend information
func (s *FiberServer) handleSystemStorage(c *fiber.Ctx) error {
	log.Printf("[API] Get storage backend info")

	// Get storage stats from actual storage
	var stats map[string]interface{}
	if s.storage != nil {
		storageStats, err := s.storage.Stats(c.Context())
		if err == nil && storageStats != nil {
			stats = map[string]interface{}{
				"total_messages":   storageStats.MessageCount,
				"topic_count":      storageStats.TopicCount,
				"total_operations": storageStats.TotalOperations,
				"consumer_count":   storageStats.ConsumerCount,
			}
		}
	}

	if stats == nil {
		stats = map[string]interface{}{
			"total_messages":   0,
			"total_size":       0,
			"total_operations": 0,
		}
	}

	storageInfo := map[string]interface{}{
		"backend": map[string]interface{}{
			"type":    "dragonfly", // TODO: Get from actual config
			"version": "1.0.0",
			"status":  "healthy",
		},
		"stats": stats,
		"available_backends": []map[string]interface{}{
			{
				"type":        "dragonfly",
				"description": "In-memory with Redis compatibility",
				"performance": "355K msgs/sec",
				"status":      "active",
			},
			{
				"type":        "badgerdb",
				"description": "Pure Go embedded key-value store",
				"performance": "207K msgs/sec",
				"status":      "available",
			},
			{
				"type":        "rocksdb",
				"description": "High-performance persistent storage",
				"performance": "218K msgs/sec",
				"status":      "available",
			},
			{
				"type":        "duckdb",
				"description": "Analytics-grade column-store",
				"performance": "TBD",
				"status":      "available",
			},
		},
		"configuration": map[string]interface{}{
			"batch_writer": map[string]interface{}{
				"enabled":         true,
				"batch_size":      5000,
				"flush_interval":  "10ms",
				"parallel_writes": true,
				"sub_batch_size":  200,
			},
		},
	}

	return c.JSON(fiber.Map{
		"success": true,
		"storage": storageInfo,
	})
}
