# processor/ (Message Processing, Diagnostics, and Metrics)

This package provides high-performance message processing, worker management, diagnostics/event logging, and metrics for the Portask system. It supports batch and single-message processing, compression, extensible processing strategies, and advanced observability.

## Main Files
- `basic.go`: Basic processor implementation
- `basic_test.go`: Tests for the basic processor
- `metrics.go`: Processor and worker metrics, compression pool
- `processor.go`: Main message processor, worker, diagnostics/event log, and processing logic
- `processor_test.go`: Tests for processor logic and diagnostics

## Key Types, Interfaces, and Methods

### Interfaces
- `Compressor` — Interface for compression algorithms (Compress, Decompress, Reset, Algorithm)

### Structs & Types
- `ProcessorMetrics`, `WorkerMetrics` — Metrics for processors and workers
- `CompressionPool` — Pool for compressor instances
- `ZstdCompressor`, `LZ4Compressor`, `SnappyCompressor`, `NoOpCompressor` — Compression implementations
- `BasicProcessor` — Basic message processor
- `MessageProcessor` — Main processor with queue, workers, config, diagnostics/event log, and worker health
- `ProcessorConfig` — Configuration for processors
- `ProcessTask`, `ProcessResult` — Task and result types for processing
- `Worker` — Worker for processing tasks
- `ProcessType` — Enum for processing type (single, batch, etc.)
- `ProcessorEvent` — Diagnostics/event log entry (worker start/stop, panic, error, batch, backpressure, custom)
- `WorkerHealth` — Health and last error for each worker

### Main Methods (selected)
- `NewProcessorMetrics() *ProcessorMetrics`, `GetAvgLatency`, `GetThroughput`, `GetCompressionRatio`, `GetSuccessRate` (ProcessorMetrics)
- `NewWorkerMetrics() *WorkerMetrics`, `GetAvgLatency` (WorkerMetrics)
- `NewCompressionPool(algorithm string) *CompressionPool`, `Get`, `Put` (CompressionPool)
- `NewCompressor(algorithm string) Compressor` (factory)
- `Compress`, `Decompress`, `Reset`, `Algorithm` (all Compressor implementations)
- `NewBasicProcessor() *BasicProcessor`, `Process` (BasicProcessor)
- `NewMessageProcessor(config *ProcessorConfig) *MessageProcessor`, `Start`, `Stop`, `ProcessMessage`, `ProcessBatch`, `GetMetrics`, `GetQueueLength`, `GetBackpressureRatio`, `AddEvent`, `GetEvents`, `GetWorkerHealth` (MessageProcessor)
- `NewWorker(id int, processor *MessageProcessor) *Worker`, `Run`, `processTask`, `processSingleMessage`, `processBatchMessages`, `compressMessage`, `decompressMessage`, `convertToPortaskMessage`, `copyMessage`, `validateMessage`, `compressMessageData`, `calculateChecksum` (Worker)

## Usage Example
```go
import "github.com/meftunca/portask/pkg/processor"

// Create a new message processor with diagnostics
defaultConfig := processor.DefaultProcessorConfig()
proc := processor.NewMessageProcessor(defaultConfig)
proc.Start(ctx)
// ... process messages ...
// Access diagnostics/events
for _, ev := range proc.GetEvents() {
    fmt.Println(ev.Time, ev.Type, ev.WorkerID, ev.Message)
}
// Access worker health
for id, h := range proc.GetWorkerHealth() {
    fmt.Printf("Worker %d healthy: %v, last error: %v\n", id, h.Healthy, h.LastError)
}
```

## Best Practices
- Use diagnostics/event log to monitor worker lifecycle, panics, errors, and backpressure.
- Monitor worker health for early detection of stuck or unhealthy workers.
- Tune processor config (worker count, queue size, batch size) for your workload.
- Use metrics and event log together for full observability.

## TODO / Missing Features
- [ ] Advanced processing strategies (custom pipelines, plugins)
- [ ] Enhanced performance and error handling
- [ ] Dynamic worker scaling
- [ ] More compression algorithms
- [ ] Real-time metrics and diagnostics export (Prometheus, etc.)

---

For details, see the code and comments in each file.

_Last updated: 2025-07-21_
