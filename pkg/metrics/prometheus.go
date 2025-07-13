package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusMetrics provides Prometheus-compatible metrics collection
type PrometheusMetrics struct {
	// Message metrics
	messagesProduced *prometheus.CounterVec
	messagesConsumed *prometheus.CounterVec
	messageLatency   *prometheus.HistogramVec
	messageSize      *prometheus.HistogramVec

	// Topic metrics
	topicCount     prometheus.Gauge
	partitionCount *prometheus.GaugeVec

	// Connection metrics
	activeConnections  prometheus.Gauge
	totalConnections   prometheus.Counter
	connectionDuration prometheus.Histogram

	// Request metrics
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	requestSize     *prometheus.HistogramVec
	responseSize    *prometheus.HistogramVec

	// Storage metrics
	storageOperations *prometheus.CounterVec
	storageLatency    *prometheus.HistogramVec
	storageSize       *prometheus.GaugeVec

	// Authentication metrics
	authAttempts *prometheus.CounterVec
	authLatency  prometheus.Histogram

	// Error metrics
	errorsTotal *prometheus.CounterVec

	// System metrics
	goRoutines  prometheus.Gauge
	memoryUsage prometheus.Gauge
	cpuUsage    prometheus.Gauge

	registry *prometheus.Registry
}

// NewPrometheusMetrics creates a new Prometheus metrics instance
func NewPrometheusMetrics(namespace string) *PrometheusMetrics {
	if namespace == "" {
		namespace = "portask"
	}

	metrics := &PrometheusMetrics{
		registry: prometheus.NewRegistry(),
	}

	// Initialize metrics
	metrics.initMessageMetrics(namespace)
	metrics.initTopicMetrics(namespace)
	metrics.initConnectionMetrics(namespace)
	metrics.initRequestMetrics(namespace)
	metrics.initStorageMetrics(namespace)
	metrics.initAuthMetrics(namespace)
	metrics.initErrorMetrics(namespace)
	metrics.initSystemMetrics(namespace)

	// Register all metrics
	metrics.registerMetrics()

	return metrics
}

func (m *PrometheusMetrics) initMessageMetrics(namespace string) {
	m.messagesProduced = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "messages_produced_total",
			Help:      "Total number of messages produced",
		},
		[]string{"topic", "partition"},
	)

	m.messagesConsumed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "messages_consumed_total",
			Help:      "Total number of messages consumed",
		},
		[]string{"topic", "partition", "consumer_group"},
	)

	m.messageLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "message_latency_seconds",
			Help:      "Message processing latency in seconds",
			Buckets:   prometheus.ExponentialBuckets(0.0001, 2, 15),
		},
		[]string{"operation", "topic"},
	)

	m.messageSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "message_size_bytes",
			Help:      "Message size in bytes",
			Buckets:   prometheus.ExponentialBuckets(64, 2, 15),
		},
		[]string{"direction", "topic"},
	)
}

func (m *PrometheusMetrics) initTopicMetrics(namespace string) {
	m.topicCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "topics_total",
			Help:      "Total number of topics",
		},
	)

	m.partitionCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "partitions_total",
			Help:      "Total number of partitions per topic",
		},
		[]string{"topic"},
	)
}

func (m *PrometheusMetrics) initConnectionMetrics(namespace string) {
	m.activeConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "connections_active",
			Help:      "Number of active connections",
		},
	)

	m.totalConnections = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "connections_total",
			Help:      "Total number of connections established",
		},
	)

	m.connectionDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "connection_duration_seconds",
			Help:      "Connection duration in seconds",
			Buckets:   prometheus.ExponentialBuckets(1, 2, 15),
		},
	)
}

func (m *PrometheusMetrics) initRequestMetrics(namespace string) {
	m.requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "requests_total",
			Help:      "Total number of requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	m.requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "request_duration_seconds",
			Help:      "Request duration in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	m.requestSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "request_size_bytes",
			Help:      "Request size in bytes",
			Buckets:   prometheus.ExponentialBuckets(64, 2, 15),
		},
		[]string{"method", "endpoint"},
	)

	m.responseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "response_size_bytes",
			Help:      "Response size in bytes",
			Buckets:   prometheus.ExponentialBuckets(64, 2, 15),
		},
		[]string{"method", "endpoint"},
	)
}

func (m *PrometheusMetrics) initStorageMetrics(namespace string) {
	m.storageOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "storage_operations_total",
			Help:      "Total number of storage operations",
		},
		[]string{"operation", "status"},
	)

	m.storageLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "storage_latency_seconds",
			Help:      "Storage operation latency in seconds",
			Buckets:   prometheus.ExponentialBuckets(0.0001, 2, 15),
		},
		[]string{"operation"},
	)

	m.storageSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "storage_size_bytes",
			Help:      "Storage size in bytes",
		},
		[]string{"type", "topic"},
	)
}

func (m *PrometheusMetrics) initAuthMetrics(namespace string) {
	m.authAttempts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "auth_attempts_total",
			Help:      "Total number of authentication attempts",
		},
		[]string{"method", "status"},
	)

	m.authLatency = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "auth_latency_seconds",
			Help:      "Authentication latency in seconds",
			Buckets:   prometheus.DefBuckets,
		},
	)
}

func (m *PrometheusMetrics) initErrorMetrics(namespace string) {
	m.errorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "errors_total",
			Help:      "Total number of errors",
		},
		[]string{"type", "component"},
	)
}

func (m *PrometheusMetrics) initSystemMetrics(namespace string) {
	m.goRoutines = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "goroutines_total",
			Help:      "Number of active goroutines",
		},
	)

	m.memoryUsage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "memory_usage_bytes",
			Help:      "Memory usage in bytes",
		},
	)

	m.cpuUsage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "cpu_usage_percent",
			Help:      "CPU usage percentage",
		},
	)
}

func (m *PrometheusMetrics) registerMetrics() {
	// Message metrics
	m.registry.MustRegister(m.messagesProduced)
	m.registry.MustRegister(m.messagesConsumed)
	m.registry.MustRegister(m.messageLatency)
	m.registry.MustRegister(m.messageSize)

	// Topic metrics
	m.registry.MustRegister(m.topicCount)
	m.registry.MustRegister(m.partitionCount)

	// Connection metrics
	m.registry.MustRegister(m.activeConnections)
	m.registry.MustRegister(m.totalConnections)
	m.registry.MustRegister(m.connectionDuration)

	// Request metrics
	m.registry.MustRegister(m.requestsTotal)
	m.registry.MustRegister(m.requestDuration)
	m.registry.MustRegister(m.requestSize)
	m.registry.MustRegister(m.responseSize)

	// Storage metrics
	m.registry.MustRegister(m.storageOperations)
	m.registry.MustRegister(m.storageLatency)
	m.registry.MustRegister(m.storageSize)

	// Auth metrics
	m.registry.MustRegister(m.authAttempts)
	m.registry.MustRegister(m.authLatency)

	// Error metrics
	m.registry.MustRegister(m.errorsTotal)

	// System metrics
	m.registry.MustRegister(m.goRoutines)
	m.registry.MustRegister(m.memoryUsage)
	m.registry.MustRegister(m.cpuUsage)
}

// Public methods for recording metrics

// RecordMessageProduced records a produced message
func (m *PrometheusMetrics) RecordMessageProduced(topic string, partition int32, size int64) {
	m.messagesProduced.WithLabelValues(topic, strconv.Itoa(int(partition))).Inc()
	m.messageSize.WithLabelValues("produced", topic).Observe(float64(size))
}

// RecordMessageConsumed records a consumed message
func (m *PrometheusMetrics) RecordMessageConsumed(topic string, partition int32, consumerGroup string, size int64) {
	m.messagesConsumed.WithLabelValues(topic, strconv.Itoa(int(partition)), consumerGroup).Inc()
	m.messageSize.WithLabelValues("consumed", topic).Observe(float64(size))
}

// RecordMessageLatency records message processing latency
func (m *PrometheusMetrics) RecordMessageLatency(operation, topic string, duration time.Duration) {
	m.messageLatency.WithLabelValues(operation, topic).Observe(duration.Seconds())
}

// SetTopicCount sets the total number of topics
func (m *PrometheusMetrics) SetTopicCount(count int) {
	m.topicCount.Set(float64(count))
}

// SetPartitionCount sets the number of partitions for a topic
func (m *PrometheusMetrics) SetPartitionCount(topic string, count int) {
	m.partitionCount.WithLabelValues(topic).Set(float64(count))
}

// RecordConnection records a new connection
func (m *PrometheusMetrics) RecordConnection() {
	m.totalConnections.Inc()
	m.activeConnections.Inc()
}

// RecordConnectionClosed records a closed connection
func (m *PrometheusMetrics) RecordConnectionClosed(duration time.Duration) {
	m.activeConnections.Dec()
	m.connectionDuration.Observe(duration.Seconds())
}

// RecordRequest records an HTTP request
func (m *PrometheusMetrics) RecordRequest(method, endpoint string, status int, duration time.Duration, requestSize, responseSize int64) {
	m.requestsTotal.WithLabelValues(method, endpoint, strconv.Itoa(status)).Inc()
	m.requestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
	m.requestSize.WithLabelValues(method, endpoint).Observe(float64(requestSize))
	m.responseSize.WithLabelValues(method, endpoint).Observe(float64(responseSize))
}

// RecordStorageOperation records a storage operation
func (m *PrometheusMetrics) RecordStorageOperation(operation string, success bool, duration time.Duration) {
	status := "success"
	if !success {
		status = "error"
	}
	m.storageOperations.WithLabelValues(operation, status).Inc()
	m.storageLatency.WithLabelValues(operation).Observe(duration.Seconds())
}

// SetStorageSize sets storage size
func (m *PrometheusMetrics) SetStorageSize(storageType, topic string, size int64) {
	m.storageSize.WithLabelValues(storageType, topic).Set(float64(size))
}

// RecordAuthAttempt records an authentication attempt
func (m *PrometheusMetrics) RecordAuthAttempt(method string, success bool, duration time.Duration) {
	status := "success"
	if !success {
		status = "failure"
	}
	m.authAttempts.WithLabelValues(method, status).Inc()
	m.authLatency.Observe(duration.Seconds())
}

// RecordError records an error
func (m *PrometheusMetrics) RecordError(errorType, component string) {
	m.errorsTotal.WithLabelValues(errorType, component).Inc()
}

// UpdateSystemMetrics updates system metrics
func (m *PrometheusMetrics) UpdateSystemMetrics(goroutines int, memoryBytes int64, cpuPercent float64) {
	m.goRoutines.Set(float64(goroutines))
	m.memoryUsage.Set(float64(memoryBytes))
	m.cpuUsage.Set(cpuPercent)
}

// GetRegistry returns the Prometheus registry
func (m *PrometheusMetrics) GetRegistry() *prometheus.Registry {
	return m.registry
}

// GetHTTPHandler returns an HTTP handler for metrics endpoint
func (m *PrometheusMetrics) GetHTTPHandler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// MetricsMiddleware returns HTTP middleware for recording request metrics
func (m *PrometheusMetrics) MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Capture response
		rw := &responseWriter{ResponseWriter: w}

		// Process request
		next.ServeHTTP(rw, r)

		// Record metrics
		duration := time.Since(start)
		requestSize := r.ContentLength
		if requestSize < 0 {
			requestSize = 0
		}

		m.RecordRequest(
			r.Method,
			r.URL.Path,
			rw.statusCode,
			duration,
			requestSize,
			int64(rw.bytesWritten),
		)
	})
}

// responseWriter wraps http.ResponseWriter to capture response metrics
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = 200
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}
