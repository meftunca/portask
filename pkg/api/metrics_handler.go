package api

import (
	"fmt"
	"runtime"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/meftunca/portask/pkg/monitoring"
	"github.com/meftunca/portask/pkg/storage"
)

// MetricsHandler handles metrics requests
type MetricsHandler struct {
	metricsCollector *monitoring.MetricsCollector
	storage          storage.MessageStore
	startTime        time.Time
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(metricsCollector *monitoring.MetricsCollector, storage storage.MessageStore) *MetricsHandler {
	return &MetricsHandler{
		metricsCollector: metricsCollector,
		storage:          storage,
		startTime:        time.Now(),
	}
}

// HandleMetrics returns Prometheus-compatible metrics
func (h *MetricsHandler) HandleMetrics(c *fiber.Ctx) error {
	metrics := h.collectAllMetrics()
	
	// Format as Prometheus metrics
	output := h.formatPrometheusMetrics(metrics)
	
	c.Set("Content-Type", "text/plain; version=0.0.4")
	return c.SendString(output)
}

// HandleMetricsJSON returns metrics in JSON format
func (h *MetricsHandler) HandleMetricsJSON(c *fiber.Ctx) error {
	metrics := h.collectAllMetrics()
	return c.JSON(metrics)
}

// HandleHealthMetrics returns health check with detailed metrics
func (h *MetricsHandler) HandleHealthMetrics(c *fiber.Ctx) error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	health := fiber.Map{
		"status":  "healthy",
		"uptime":  time.Since(h.startTime).String(),
		"version": "1.0.0",
		"system": fiber.Map{
			"goroutines": runtime.NumGoroutine(),
			"memory": fiber.Map{
				"alloc_mb":      m.Alloc / 1024 / 1024,
				"total_alloc_mb": m.TotalAlloc / 1024 / 1024,
				"sys_mb":        m.Sys / 1024 / 1024,
				"num_gc":        m.NumGC,
			},
			"cpu_count": runtime.NumCPU(),
		},
		"timestamp": time.Now().Unix(),
	}
	
	// Add storage health if available
	if h.storage != nil {
		storageStats, err := h.storage.Stats(c.Context())
		if err == nil {
			health["storage"] = fiber.Map{
				"status": "connected",
				"stats":  storageStats,
			}
		} else {
			health["storage"] = fiber.Map{
				"status": "error",
				"error":  err.Error(),
			}
		}
	}
	
	return c.JSON(health)
}

// collectAllMetrics collects all available metrics
func (h *MetricsHandler) collectAllMetrics() map[string]interface{} {
	metrics := make(map[string]interface{})
	
	// System metrics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	metrics["system"] = map[string]interface{}{
		"goroutines":       runtime.NumGoroutine(),
		"cpu_count":        runtime.NumCPU(),
		"memory_alloc_mb":  m.Alloc / 1024 / 1024,
		"memory_total_mb":  m.TotalAlloc / 1024 / 1024,
		"memory_sys_mb":    m.Sys / 1024 / 1024,
		"memory_heap_mb":   m.HeapAlloc / 1024 / 1024,
		"gc_runs":          m.NumGC,
		"gc_pause_total_ns": m.PauseTotalNs,
	}
	
	// Application metrics
	metrics["application"] = map[string]interface{}{
		"uptime_seconds": time.Since(h.startTime).Seconds(),
		"start_time":     h.startTime.Unix(),
	}
	
	// Collector metrics if available
	if h.metricsCollector != nil {
		// Note: Implement GetMetrics() in MetricsCollector if needed
		// For now, we'll skip collector metrics
		metrics["collector"] = map[string]interface{}{
			"status": "not_implemented",
		}
	}
	
	// Storage metrics if available (will be added when called with context)
	// Note: Storage stats require context parameter
	
	metrics["timestamp"] = time.Now().Unix()
	
	return metrics
}

// formatPrometheusMetrics formats metrics in Prometheus format
func (h *MetricsHandler) formatPrometheusMetrics(metrics map[string]interface{}) string {
	var output string
	
	// Helper function to add metric
	addMetric := func(name, metricType, help string, value interface{}, labels map[string]string) {
		output += fmt.Sprintf("# HELP %s %s\n", name, help)
		output += fmt.Sprintf("# TYPE %s %s\n", name, metricType)
		
		labelStr := ""
		if len(labels) > 0 {
			labelStr = "{"
			first := true
			for k, v := range labels {
				if !first {
					labelStr += ","
				}
				labelStr += fmt.Sprintf(`%s="%s"`, k, v)
				first = false
			}
			labelStr += "}"
		}
		
		output += fmt.Sprintf("%s%s %v\n", name, labelStr, value)
	}
	
	// System metrics
	if system, ok := metrics["system"].(map[string]interface{}); ok {
		addMetric("portask_goroutines", "gauge", "Number of goroutines", system["goroutines"], nil)
		addMetric("portask_cpu_count", "gauge", "Number of CPU cores", system["cpu_count"], nil)
		addMetric("portask_memory_alloc_bytes", "gauge", "Allocated memory in bytes", 
			system["memory_alloc_mb"].(uint64)*1024*1024, nil)
		addMetric("portask_memory_sys_bytes", "gauge", "System memory in bytes",
			system["memory_sys_mb"].(uint64)*1024*1024, nil)
		addMetric("portask_gc_runs_total", "counter", "Total number of GC runs", system["gc_runs"], nil)
	}
	
	// Application metrics
	if app, ok := metrics["application"].(map[string]interface{}); ok {
		addMetric("portask_uptime_seconds", "gauge", "Application uptime in seconds", app["uptime_seconds"], nil)
		addMetric("portask_start_time_seconds", "gauge", "Application start time in Unix seconds", app["start_time"], nil)
	}
	
	// Collector metrics
	if collector, ok := metrics["collector"].(map[string]interface{}); ok {
		if messagesProcessed, ok := collector["messages_processed"].(int64); ok {
			addMetric("portask_messages_processed_total", "counter", "Total messages processed", messagesProcessed, nil)
		}
		if messagesPerSecond, ok := collector["messages_per_second"].(float64); ok {
			addMetric("portask_messages_per_second", "gauge", "Messages processed per second", messagesPerSecond, nil)
		}
		if bytesProcessed, ok := collector["bytes_processed"].(int64); ok {
			addMetric("portask_bytes_processed_total", "counter", "Total bytes processed", bytesProcessed, nil)
		}
		if totalErrors, ok := collector["total_errors"].(int64); ok {
			addMetric("portask_errors_total", "counter", "Total errors", totalErrors, nil)
		}
	}
	
	// Storage metrics
	if storage, ok := metrics["storage"].(map[string]interface{}); ok {
		if totalMessages, ok := storage["total_messages"].(int64); ok {
			addMetric("portask_storage_messages_total", "gauge", "Total messages in storage", totalMessages, nil)
		}
		if totalTopics, ok := storage["total_topics"].(int); ok {
			addMetric("portask_storage_topics_total", "gauge", "Total topics in storage", totalTopics, nil)
		}
	}
	
	// Add timestamp
	addMetric("portask_scrape_timestamp_seconds", "gauge", "Timestamp of this scrape", 
		time.Now().Unix(), nil)
	
	return output
}

// MetricsMiddleware tracks request metrics
type MetricsMiddleware struct {
	requestCount   int64
	requestLatency []time.Duration
}

// NewMetricsMiddleware creates a new metrics middleware
func NewMetricsMiddleware() *MetricsMiddleware {
	return &MetricsMiddleware{
		requestLatency: make([]time.Duration, 0, 1000),
	}
}

// Middleware returns a Fiber middleware handler
func (m *MetricsMiddleware) Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		
		// Process request
		err := c.Next()
		
		// Record metrics
		duration := time.Since(start)
		m.requestLatency = append(m.requestLatency, duration)
		
		// Keep only last 1000 requests
		if len(m.requestLatency) > 1000 {
			m.requestLatency = m.requestLatency[1:]
		}
		
		// Add metrics headers
		c.Set("X-Request-Duration", duration.String())
		
		return err
	}
}

// GetStats returns middleware statistics
func (m *MetricsMiddleware) GetStats() map[string]interface{} {
	if len(m.requestLatency) == 0 {
		return map[string]interface{}{
			"request_count": m.requestCount,
			"avg_latency":   0,
		}
	}
	
	// Calculate average latency
	var total time.Duration
	for _, d := range m.requestLatency {
		total += d
	}
	avg := total / time.Duration(len(m.requestLatency))
	
	return map[string]interface{}{
		"request_count":  m.requestCount,
		"avg_latency_ms": avg.Milliseconds(),
		"sample_size":    len(m.requestLatency),
	}
}

