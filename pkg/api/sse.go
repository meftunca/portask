package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
)

// handleSSEMetrics sends real-time metrics via Server-Sent Events
// GET /api/v1/sse/metrics
func (s *FiberServer) handleSSEMetrics(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")
	c.Set("Transfer-Encoding", "chunked")

	c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		// Send initial connection message
		fmt.Fprintf(w, "event: connected\n")
		fmt.Fprintf(w, "data: {\"message\":\"Connected to Portask metrics stream\"}\n\n")
		w.Flush()

		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-c.Context().Done():
				// Client disconnected
				return
			case <-ticker.C:
				// Collect metrics
				uptime := time.Since(s.startTime)
				var m runtime.MemStats
				runtime.ReadMemStats(&m)

				// Get storage stats
				var storageStats map[string]interface{}
				if s.storage != nil {
					ctx := c.Context()
					if stats, err := s.storage.Stats(ctx); err == nil {
						storageStats = map[string]interface{}{
							"total_messages":     stats.MessageCount,
							"storage_used_bytes": stats.StorageUsedBytes,
							"avg_latency_ms":     stats.AvgLatencyMs,
							"total_operations":   stats.TotalOperations,
						}
					}
				}

				// Build metrics response
				metrics := map[string]interface{}{
					"timestamp": time.Now().Unix(),
					"core": map[string]interface{}{
						"uptime_seconds": uptime.Seconds(),
						"requests_total": atomic.LoadInt64(&s.requestCount),
						"errors_total":   atomic.LoadInt64(&s.errorCount),
						"avg_latency_ms": s.avgLatency.Milliseconds(),
					},
					"system": map[string]interface{}{
						"go_version":     runtime.Version(),
						"num_cpu":        runtime.NumCPU(),
						"num_goroutines": runtime.NumGoroutine(),
						"alloc_mb":       bToMb(m.Alloc),
						"sys_mb":         bToMb(m.Sys),
						"num_gc":         m.NumGC,
					},
					"storage": storageStats,
					"network": map[string]interface{}{
						"connections_active": 0,
					},
				}

				if s.networkServer != nil {
					stats := s.networkServer.GetStats()
					metrics["network"] = map[string]interface{}{
						"connections_active": stats.ActiveConnections,
						"total_connections":  stats.TotalConnections,
						"messages_received":  stats.MessagesReceived,
						"messages_sent":      stats.MessagesSent,
					}
				}

				// Send metrics as SSE
				data, err := json.Marshal(metrics)
				if err != nil {
					continue
				}

				fmt.Fprintf(w, "event: metrics\n")
				fmt.Fprintf(w, "data: %s\n\n", string(data))

				if err := w.Flush(); err != nil {
					// Client disconnected
					return
				}
			}
		}
	}))

	return nil
}

// handleSSEHealth sends health status via SSE
// GET /api/v1/sse/health
func (s *FiberServer) handleSSEHealth(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")
	c.Set("Transfer-Encoding", "chunked")

	c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-c.Context().Done():
				return
			case <-ticker.C:
				uptime := time.Since(s.startTime)
				var m runtime.MemStats
				runtime.ReadMemStats(&m)

				health := map[string]interface{}{
					"status":      "healthy",
					"version":     "2.0.0-fiber",
					"uptime":      uptime.Seconds(),
					"connections": 0,
					"memory": map[string]interface{}{
						"alloc_mb": bToMb(m.Alloc),
						"sys_mb":   bToMb(m.Sys),
						"num_gc":   m.NumGC,
					},
				}

				data, _ := json.Marshal(health)
				fmt.Fprintf(w, "event: health\n")
				fmt.Fprintf(w, "data: %s\n\n", string(data))

				if err := w.Flush(); err != nil {
					return
				}
			}
		}
	}))

	return nil
}
