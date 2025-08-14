# queue/ — Lock-free Queue, Worker Pool, and Routing

This package provides Portask's high-performance lock-free queue implementation, message bus, worker pool, and advanced message processing/routing logic. It is the core of Portask's scalable, concurrent message delivery and processing system.

---

## Key Files
- `lockfree.go`: Lock-free queue, message bus, worker pool, and processor interfaces/structs
- `processors.go`: Built-in message processors (default, batch, echo, filter, retry, chained, benchmark)
- `queue_test.go`: Unit and integration tests for queue, bus, and processors
- `lockfree_test.go`: Lock-free queue tests
- `queue_benchmark_test.go`: Benchmarks for queue and message bus

---

## Main Types & Interfaces

### Queue & Bus
- `type LockFreeQueue` — High-performance, lock-free queue for Portask messages
- `type QueueStats` — Statistics for a queue (size, enqueues, dequeues, drops, etc)
- `type MessageBus` — Topic-based message bus with per-topic lock-free queues
- `type BusStats` — Statistics for the message bus (per-topic, global)
- `type MessageBusConfig` — Configuration for message bus (topics, capacity, drop policy, etc)

### Worker Pool
- `type Worker` — Worker goroutine for processing messages from queues
- `type WorkerStats` — Per-worker statistics
- `type WorkerPool` — Pool of workers for concurrent message processing
- `type WorkerPoolConfig` — Configuration for worker pool (size, affinity, etc)

### Message Processing
- `type MessageProcessor` (interface) — Abstracts message processing logic
- Built-in processors: `DefaultMessageProcessor`, `BatchMessageProcessor`, `EchoMessageProcessor`, `BenchmarkMessageProcessor`, `ChainedMessageProcessor`, `FilterMessageProcessor`, `RetryMessageProcessor`

---

## Core Methods (Selection)
- `LockFreeQueue.Enqueue/Dequeue/TryDequeue/IsEmpty/IsFull/Stats` — Queue operations
- `MessageBus.Publish/PublishToTopic/GetStats` — Publish messages and retrieve stats
- `WorkerPool.Start/Stop/GetStats` — Manage worker pool lifecycle
- `MessageProcessor.ProcessMessage` — Process a message (implemented by all processors)
- `BatchMessageProcessor.Flush` — Flushes batch for batch processing
- `ChainedMessageProcessor.Add` — Add processors to a chain

---

## Usage Example
```go
// Create a lock-free queue
queue := queue.NewLockFreeQueue("my-topic", 1024, queue.DropPolicy(0))
queue.Enqueue(msg)
msg, ok := queue.Dequeue()

// Start a message bus and worker pool
bus := queue.NewMessageBus(queue.MessageBusConfig{/* ... */})
pool := queue.NewWorkerPool(queue.WorkerPoolConfig{/* ... */}, bus)
go pool.Start(context.Background())
```

---

## TODO / Missing Features
- [ ] Advanced routing algorithms (priority, sharding, sticky sessions)
- [ ] Performance optimizations and zero-copy
- [ ] Dynamic scaling of worker pool
- [ ] Backpressure and flow control
- [ ] Pluggable processor pipelines

---

_Last updated: 2025-07-20_
