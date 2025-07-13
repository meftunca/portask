package monitoring

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/meftunca/portask/pkg/types"
)

// MetricsCollector collects and aggregates performance metrics
type MetricsCollector struct {
	// Message metrics
	messagesProcessed int64
	messagesPerSecond float64
	totalLatency      int64
	minLatency        int64
	maxLatency        int64

	// Throughput metrics
	bytesProcessed int64
	bytesPerSecond float64

	// Error metrics
	totalErrors int64
	errorRate   float64

	// System metrics
	systemMetrics *SystemMetrics

	// Collection settings
	startTime      time.Time
	lastUpdate     time.Time
	updateInterval time.Duration

	// Thread safety
	mutex sync.RWMutex

	// Monitoring channels
	messageCh chan *MessageMetric
	systemCh  chan *SystemMetrics
	stopCh    chan struct{}
	running   int32
}

// MessageMetric represents metrics for a single message
type MessageMetric struct {
	MessageID      types.MessageID `json:"message_id"`
	Topic          types.TopicName `json:"topic"`
	Size           int             `json:"size"`
	CompressedSize int             `json:"compressed_size,omitempty"`
	ProcessingTime time.Duration   `json:"processing_time"`
	SerializeTime  time.Duration   `json:"serialize_time"`
	CompressTime   time.Duration   `json:"compress_time"`
	StorageTime    time.Duration   `json:"storage_time"`
	TotalTime      time.Duration   `json:"total_time"`
	ErrorCount     int             `json:"error_count"`
	RetryCount     int             `json:"retry_count"`
	Timestamp      time.Time       `json:"timestamp"`
}

// SystemMetrics represents current system performance metrics
type SystemMetrics struct {
	// CPU metrics
	CPUUsage float64 `json:"cpu_usage"`
	CPUCores int     `json:"cpu_cores"`

	// Memory metrics
	MemoryUsage     float64 `json:"memory_usage"`
	MemoryTotal     uint64  `json:"memory_total"`
	MemoryUsed      uint64  `json:"memory_used"`
	MemoryAvailable uint64  `json:"memory_available"`

	// Garbage collection metrics
	GCPauseTotalNs uint64  `json:"gc_pause_total_ns"`
	GCNumRuns      uint32  `json:"gc_num_runs"`
	GCCPUFraction  float64 `json:"gc_cpu_fraction"`

	// Latency metrics
	LatencyMs    float64 `json:"latency_ms"`
	P50LatencyMs float64 `json:"p50_latency_ms"`
	P95LatencyMs float64 `json:"p95_latency_ms"`
	P99LatencyMs float64 `json:"p99_latency_ms"`

	// Network metrics
	ConnectionsActive int    `json:"connections_active"`
	NetworkBytesIn    uint64 `json:"network_bytes_in"`
	NetworkBytesOut   uint64 `json:"network_bytes_out"`

	// Storage metrics
	StorageReads  int64 `json:"storage_reads"`
	StorageWrites int64 `json:"storage_writes"`
	StorageErrors int64 `json:"storage_errors"`

	// Timestamp
	Timestamp time.Time `json:"timestamp"`
}

// PerformanceReport represents a comprehensive performance report
type PerformanceReport struct {
	// Time range
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration"`

	// Message processing metrics
	TotalMessages     int64   `json:"total_messages"`
	MessagesPerSecond float64 `json:"messages_per_second"`
	TotalBytes        int64   `json:"total_bytes"`
	BytesPerSecond    float64 `json:"bytes_per_second"`

	// Latency metrics
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	MinLatencyMs float64 `json:"min_latency_ms"`
	MaxLatencyMs float64 `json:"max_latency_ms"`
	P50LatencyMs float64 `json:"p50_latency_ms"`
	P95LatencyMs float64 `json:"p95_latency_ms"`
	P99LatencyMs float64 `json:"p99_latency_ms"`

	// Error metrics
	TotalErrors int64   `json:"total_errors"`
	ErrorRate   float64 `json:"error_rate"`

	// Compression metrics
	CompressionRatio   float64 `json:"compression_ratio"`
	CompressionSavings int64   `json:"compression_savings"`

	// System metrics
	AvgCPUUsage    float64 `json:"avg_cpu_usage"`
	AvgMemoryUsage float64 `json:"avg_memory_usage"`
	GCOverhead     float64 `json:"gc_overhead"`

	// Top topics by volume
	TopTopics []TopicMetric `json:"top_topics"`
}

// TopicMetric represents metrics for a specific topic
type TopicMetric struct {
	Topic        types.TopicName `json:"topic"`
	MessageCount int64           `json:"message_count"`
	TotalBytes   int64           `json:"total_bytes"`
	AvgLatencyMs float64         `json:"avg_latency_ms"`
	ErrorCount   int64           `json:"error_count"`
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(updateInterval time.Duration) *MetricsCollector {
	return &MetricsCollector{
		startTime:      time.Now(),
		lastUpdate:     time.Now(),
		updateInterval: updateInterval,
		messageCh:      make(chan *MessageMetric, 1000),
		systemCh:       make(chan *SystemMetrics, 100),
		stopCh:         make(chan struct{}),
		systemMetrics:  &SystemMetrics{},
		minLatency:     int64(^uint64(0) >> 1), // Max int64
	}
}

// Start begins metric collection
func (mc *MetricsCollector) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&mc.running, 0, 1) {
		return types.NewPortaskError(types.ErrCodeSystemError, "metrics collector already running")
	}

	// Start collection goroutines
	go mc.messageCollector(ctx)
	go mc.systemCollector(ctx)
	go mc.aggregator(ctx)

	return nil
}

// Stop stops metric collection
func (mc *MetricsCollector) Stop() error {
	if !atomic.CompareAndSwapInt32(&mc.running, 1, 0) {
		return types.NewPortaskError(types.ErrCodeSystemError, "metrics collector not running")
	}

	close(mc.stopCh)
	return nil
}

// RecordMessage records metrics for a processed message
func (mc *MetricsCollector) RecordMessage(metric *MessageMetric) {
	if atomic.LoadInt32(&mc.running) == 0 {
		return
	}

	select {
	case mc.messageCh <- metric:
	default:
		// Channel full, drop metric to avoid blocking
		atomic.AddInt64(&mc.totalErrors, 1)
	}
}

// RecordSystemMetrics records system-level metrics
func (mc *MetricsCollector) RecordSystemMetrics(metrics *SystemMetrics) {
	if atomic.LoadInt32(&mc.running) == 0 {
		return
	}

	select {
	case mc.systemCh <- metrics:
	default:
		// Channel full, drop metric
	}
}

// GetCurrentMetrics returns current system metrics
func (mc *MetricsCollector) GetCurrentMetrics() *SystemMetrics {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()

	// Make a copy to avoid data races
	metrics := *mc.systemMetrics
	return &metrics
}

// GetPerformanceReport generates a comprehensive performance report
func (mc *MetricsCollector) GetPerformanceReport() *PerformanceReport {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()

	now := time.Now()
	duration := now.Sub(mc.startTime)

	report := &PerformanceReport{
		StartTime:      mc.startTime,
		EndTime:        now,
		Duration:       duration,
		TotalMessages:  atomic.LoadInt64(&mc.messagesProcessed),
		TotalBytes:     atomic.LoadInt64(&mc.bytesProcessed),
		TotalErrors:    atomic.LoadInt64(&mc.totalErrors),
		AvgCPUUsage:    mc.systemMetrics.CPUUsage,
		AvgMemoryUsage: mc.systemMetrics.MemoryUsage,
		GCOverhead:     mc.systemMetrics.GCCPUFraction,
	}

	// Calculate rates
	if duration.Seconds() > 0 {
		report.MessagesPerSecond = float64(report.TotalMessages) / duration.Seconds()
		report.BytesPerSecond = float64(report.TotalBytes) / duration.Seconds()
	}

	// Calculate error rate
	if report.TotalMessages > 0 {
		report.ErrorRate = float64(report.TotalErrors) / float64(report.TotalMessages)
	}

	// Calculate latency metrics
	if mc.messagesProcessed > 0 {
		report.AvgLatencyMs = float64(mc.totalLatency) / float64(mc.messagesProcessed) / 1e6
		report.MinLatencyMs = float64(atomic.LoadInt64(&mc.minLatency)) / 1e6
		report.MaxLatencyMs = float64(atomic.LoadInt64(&mc.maxLatency)) / 1e6
	}

	return report
}

// messageCollector processes message metrics
func (mc *MetricsCollector) messageCollector(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-mc.stopCh:
			return
		case metric := <-mc.messageCh:
			mc.processMessageMetric(metric)
		}
	}
}

// processMessageMetric processes a single message metric
func (mc *MetricsCollector) processMessageMetric(metric *MessageMetric) {
	atomic.AddInt64(&mc.messagesProcessed, 1)
	atomic.AddInt64(&mc.bytesProcessed, int64(metric.Size))

	// Update latency metrics
	latencyNs := metric.TotalTime.Nanoseconds()
	atomic.AddInt64(&mc.totalLatency, latencyNs)

	// Update min latency
	for {
		current := atomic.LoadInt64(&mc.minLatency)
		if latencyNs >= current || atomic.CompareAndSwapInt64(&mc.minLatency, current, latencyNs) {
			break
		}
	}

	// Update max latency
	for {
		current := atomic.LoadInt64(&mc.maxLatency)
		if latencyNs <= current || atomic.CompareAndSwapInt64(&mc.maxLatency, current, latencyNs) {
			break
		}
	}

	// Count errors
	if metric.ErrorCount > 0 {
		atomic.AddInt64(&mc.totalErrors, int64(metric.ErrorCount))
	}
}

// systemCollector processes system metrics
func (mc *MetricsCollector) systemCollector(ctx context.Context) {
	ticker := time.NewTicker(mc.updateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-mc.stopCh:
			return
		case <-ticker.C:
			metrics := mc.collectSystemMetrics()
			mc.updateSystemMetrics(metrics)
		case metrics := <-mc.systemCh:
			mc.updateSystemMetrics(metrics)
		}
	}
}

// collectSystemMetrics collects current system metrics
func (mc *MetricsCollector) collectSystemMetrics() *SystemMetrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return &SystemMetrics{
		CPUCores:        runtime.NumCPU(),
		MemoryTotal:     m.Sys,
		MemoryUsed:      m.Alloc,
		MemoryAvailable: m.Sys - m.Alloc,
		MemoryUsage:     float64(m.Alloc) / float64(m.Sys) * 100,
		GCPauseTotalNs:  m.PauseTotalNs,
		GCNumRuns:       m.NumGC,
		GCCPUFraction:   m.GCCPUFraction,
		Timestamp:       time.Now(),
	}
}

// updateSystemMetrics updates system metrics with thread safety
func (mc *MetricsCollector) updateSystemMetrics(metrics *SystemMetrics) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	*mc.systemMetrics = *metrics
	mc.lastUpdate = time.Now()
}

// aggregator aggregates metrics and calculates derived values
func (mc *MetricsCollector) aggregator(ctx context.Context) {
	ticker := time.NewTicker(mc.updateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-mc.stopCh:
			return
		case <-ticker.C:
			mc.calculateRates()
		}
	}
}

// calculateRates calculates various rates and derived metrics
func (mc *MetricsCollector) calculateRates() {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	now := time.Now()
	duration := now.Sub(mc.lastUpdate)

	if duration.Seconds() > 0 {
		// Calculate messages per second
		messages := atomic.LoadInt64(&mc.messagesProcessed)
		mc.messagesPerSecond = float64(messages) / duration.Seconds()

		// Calculate bytes per second
		bytes := atomic.LoadInt64(&mc.bytesProcessed)
		mc.bytesPerSecond = float64(bytes) / duration.Seconds()

		// Calculate error rate
		errors := atomic.LoadInt64(&mc.totalErrors)
		if messages > 0 {
			mc.errorRate = float64(errors) / float64(messages) * 100
		}
	}
}

// AdaptiveCompressionDecider makes compression decisions based on system metrics
type AdaptiveCompressionDecider struct {
	collector        *MetricsCollector
	cpuThresholdHigh float64
	cpuThresholdLow  float64
	memThresholdHigh float64
	latencyThreshold float64

	// Decision history for smoothing
	recentDecisions []bool
	decisionIndex   int
	smoothingWindow int
}

// NewAdaptiveCompressionDecider creates a new adaptive compression decider
func NewAdaptiveCompressionDecider(collector *MetricsCollector) *AdaptiveCompressionDecider {
	return &AdaptiveCompressionDecider{
		collector:        collector,
		cpuThresholdHigh: 80.0,
		cpuThresholdLow:  40.0,
		memThresholdHigh: 85.0,
		latencyThreshold: 10.0,
		smoothingWindow:  10,
		recentDecisions:  make([]bool, 10),
	}
}

// ShouldCompress decides whether to compress based on current system state
func (acd *AdaptiveCompressionDecider) ShouldCompress(messageSize int, thresholdBytes int) bool {
	// Always check size threshold first
	if messageSize < thresholdBytes {
		return false
	}

	metrics := acd.collector.GetCurrentMetrics()
	shouldCompress := true

	// Check CPU usage
	if metrics.CPUUsage > acd.cpuThresholdHigh {
		shouldCompress = false
	} else if metrics.CPUUsage < acd.cpuThresholdLow {
		shouldCompress = true
	}

	// Check memory usage
	if metrics.MemoryUsage > acd.memThresholdHigh {
		shouldCompress = false
	}

	// Check latency
	if metrics.LatencyMs > acd.latencyThreshold {
		shouldCompress = false
	}

	// Apply smoothing based on recent decisions
	acd.recentDecisions[acd.decisionIndex] = shouldCompress
	acd.decisionIndex = (acd.decisionIndex + 1) % acd.smoothingWindow

	// Count recent true decisions
	trueCount := 0
	for _, decision := range acd.recentDecisions {
		if decision {
			trueCount++
		}
	}

	// Require majority consensus for compression
	return trueCount > acd.smoothingWindow/2
}

// PerformanceProfiler provides detailed performance profiling
type PerformanceProfiler struct {
	samples     []time.Duration
	sampleIndex int
	sampleSize  int
	percentiles map[int]time.Duration
	mutex       sync.RWMutex
}

// NewPerformanceProfiler creates a new performance profiler
func NewPerformanceProfiler(sampleSize int) *PerformanceProfiler {
	return &PerformanceProfiler{
		samples:     make([]time.Duration, sampleSize),
		sampleSize:  sampleSize,
		percentiles: make(map[int]time.Duration),
	}
}

// AddSample adds a performance sample
func (pp *PerformanceProfiler) AddSample(duration time.Duration) {
	pp.mutex.Lock()
	defer pp.mutex.Unlock()

	pp.samples[pp.sampleIndex] = duration
	pp.sampleIndex = (pp.sampleIndex + 1) % pp.sampleSize
}

// GetPercentile returns the specified percentile
func (pp *PerformanceProfiler) GetPercentile(percentile int) time.Duration {
	pp.mutex.RLock()
	defer pp.mutex.RUnlock()

	if val, exists := pp.percentiles[percentile]; exists {
		return val
	}

	// Calculate percentile (simplified implementation)
	sortedSamples := make([]time.Duration, len(pp.samples))
	copy(sortedSamples, pp.samples)

	// Simple bubble sort for small arrays
	for i := 0; i < len(sortedSamples); i++ {
		for j := 0; j < len(sortedSamples)-1-i; j++ {
			if sortedSamples[j] > sortedSamples[j+1] {
				sortedSamples[j], sortedSamples[j+1] = sortedSamples[j+1], sortedSamples[j]
			}
		}
	}

	index := (percentile * len(sortedSamples)) / 100
	if index >= len(sortedSamples) {
		index = len(sortedSamples) - 1
	}

	result := sortedSamples[index]
	pp.percentiles[percentile] = result
	return result
}

// GetStats returns comprehensive statistics
func (pp *PerformanceProfiler) GetStats() map[string]time.Duration {
	return map[string]time.Duration{
		"p50": pp.GetPercentile(50),
		"p90": pp.GetPercentile(90),
		"p95": pp.GetPercentile(95),
		"p99": pp.GetPercentile(99),
	}
}

// Reset clears all samples
func (pp *PerformanceProfiler) Reset() {
	pp.mutex.Lock()
	defer pp.mutex.Unlock()

	for i := range pp.samples {
		pp.samples[i] = 0
	}
	pp.sampleIndex = 0
	pp.percentiles = make(map[int]time.Duration)
}
