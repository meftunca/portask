# metrics/ (Prometheus Integration)

This package provides Prometheus-based metrics collection and monitoring for Portask. It exposes system, message, topic, connection, request, storage, authentication, and error metrics to Prometheus for observability and alerting.

## Main Files
- `prometheus.go`: Prometheus integration and metric registration

## Key Types, Interfaces, and Methods

### Structs & Types
- `PrometheusMetrics` — Main Prometheus metrics collector and registry
- `responseWriter` — HTTP response wrapper for metrics

### Main Methods (selected)
- `NewPrometheusMetrics(namespace string) *PrometheusMetrics` — Create a new Prometheus metrics collector
- `initMessageMetrics()`, `initTopicMetrics()`, `initConnectionMetrics()`, `initRequestMetrics()`, `initStorageMetrics()`, `initAuthMetrics()`, `initErrorMetrics()`, `initSystemMetrics()` — Metric initialization
- `registerMetrics()` — Register all metrics with Prometheus
- `RecordMessageProduced()`, `RecordMessageConsumed()`, `RecordMessageLatency()`, `SetTopicCount()`, `SetPartitionCount()`, `RecordConnection()`, `RecordConnectionClosed()`, `RecordRequest()`, `RecordStorageOperation()`, `SetStorageSize()`, `RecordAuthAttempt()`, `RecordError()`, `UpdateSystemMetrics()` — Metric recording
- `GetRegistry() *prometheus.Registry` — Get Prometheus registry
- `GetHTTPHandler() http.Handler` — Expose metrics endpoint
- `MetricsMiddleware(next http.Handler) http.Handler` — HTTP middleware for metrics

## Usage Example
```go
import "github.com/meftunca/portask/pkg/metrics"

metrics := metrics.NewPrometheusMetrics("portask")
http.Handle("/metrics", metrics.GetHTTPHandler())
```

## TODO / Missing Features
- [ ] Add new metrics
- [ ] Integrate with non-Prometheus monitoring systems
- [ ] Custom metric exporters
- [ ] Distributed metrics support

---

For details, see the code and comments in each file.
