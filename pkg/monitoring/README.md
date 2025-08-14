# monitoring/ (Monitoring and Metrics)

This package provides system monitoring and metrics collection for Portask. It includes collectors for system, message, and performance metrics, as well as adaptive compression and profiling utilities.

## Main Files
- `metrics.go`: Monitoring metrics, collectors, and profilers
- `monitoring_test.go`: Tests for monitoring and metrics

## Key Types, Interfaces, and Methods

### Types & Structs
- `MetricsCollector` — Collects and aggregates system and message metrics
- `MessageMetric`, `TopicMetric`, `SystemMetrics`, `PerformanceReport` — Metric types
- `AdaptiveCompressionDecider` — Decides compression based on metrics
- `PerformanceProfiler` — Tracks and profiles performance

### Main Methods (selected)
- `NewMetricsCollector(updateInterval time.Duration) *MetricsCollector`, `Start(ctx)`, `Stop()`, `RecordMessage()`, `RecordSystemMetrics()`, `GetCurrentMetrics()`, `GetPerformanceReport()` (MetricsCollector)
- `NewAdaptiveCompressionDecider(collector)`, `ShouldCompress()` (AdaptiveCompressionDecider)
- `NewPerformanceProfiler(sampleSize)`, `AddSample()`, `GetPercentile()`, `GetStats()`, `Reset()` (PerformanceProfiler)

## Usage Example
```go
import "github.com/meftunca/portask/pkg/monitoring"

collector := monitoring.NewMetricsCollector(5 * time.Second)
err := collector.Start(ctx)
```

## TODO / Missing Features
- [ ] Advanced dashboard integration
- [ ] Alerting and alarm systems
- [ ] Distributed metrics aggregation
- [ ] Custom metric exporters

---

For details, see the code and comments in each file.
