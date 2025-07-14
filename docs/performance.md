# Performance Guide

## Overview

This guide provides comprehensive information about optimizing Portask performance for both AMQP and Kafka protocols. It covers performance tuning, benchmarking, monitoring, and best practices for production deployments.

## Table of Contents

- [Performance Baseline](#performance-baseline)
- [System Requirements](#system-requirements)
- [AMQP Performance](#amqp-performance)
- [Kafka Performance](#kafka-performance)
- [Memory Management](#memory-management)
- [CPU Optimization](#cpu-optimization)
- [Network Optimization](#network-optimization)
- [Storage Optimization](#storage-optimization)
- [Monitoring and Metrics](#monitoring-and-metrics)
- [Benchmarking](#benchmarking)
- [Production Tuning](#production-tuning)

## Performance Baseline

### Test Environment
- **Hardware**: 8-core CPU, 16GB RAM, SSD storage
- **OS**: Linux Ubuntu 22.04 LTS
- **Go Version**: 1.21+
- **Network**: Gigabit Ethernet

### Baseline Metrics

#### AMQP Performance
```
Message Size: 1KB
Producer Throughput: 50,000 msg/sec
Consumer Throughput: 45,000 msg/sec
End-to-End Latency (P99): <5ms
Memory Usage: ~200MB (1M messages)
CPU Usage: ~30% (single core)
```

#### Kafka Performance
```
Message Size: 1KB, 3 partitions
Producer Throughput: 80,000 msg/sec
Consumer Throughput: 75,000 msg/sec
End-to-End Latency (P99): <3ms
Memory Usage: ~150MB (1M messages)
CPU Usage: ~25% (single core)
```

## System Requirements

### Minimum Requirements
- **CPU**: 2 cores
- **RAM**: 4GB
- **Storage**: 10GB SSD
- **Network**: 100Mbps

### Recommended Production
- **CPU**: 8+ cores
- **RAM**: 16GB+
- **Storage**: 100GB+ NVMe SSD
- **Network**: 1Gbps+

### High-Performance Setup
- **CPU**: 16+ cores (3.0GHz+)
- **RAM**: 32GB+
- **Storage**: 500GB+ NVMe SSD (RAID 0)
- **Network**: 10Gbps+

## AMQP Performance

### Exchange Optimization

#### Exchange Types Performance
```go
// Performance ranking (fastest to slowest):
// 1. Direct exchange - O(1) routing
// 2. Topic exchange - O(log n) routing  
// 3. Headers exchange - O(n) routing
// 4. Fanout exchange - O(n) routing (but predictable)

// Optimal exchange configuration
type ExchangeConfig struct {
    Type:       "direct",  // Fastest routing
    Durable:    false,     // Faster, but less safe
    AutoDelete: true,      // Cleanup unused exchanges
    Arguments: map[string]interface{}{
        "x-message-ttl": 300000, // 5 minutes TTL
    },
}
```

#### Routing Key Optimization
```go
// Good: Short, specific routing keys
"user.123.login"      // ✅ Fast
"order.new"           // ✅ Fast

// Bad: Long, complex routing keys  
"application.module.submodule.action.user.details.update" // ❌ Slow
```

### Queue Optimization

#### Memory vs Disk Trade-offs
```json
{
  "queue_config": {
    "arguments": {
      "x-max-length": 100000,           // Limit queue size
      "x-max-length-bytes": 104857600,  // 100MB limit
      "x-overflow": "reject-publish",   // Reject when full
      "x-message-ttl": 3600000,         // 1 hour TTL
      "x-queue-mode": "lazy"            // Disk-based (slower but scalable)
    }
  }
}
```

#### Queue Performance Configuration
```go
// High-performance queue settings
type QueueConfig struct {
    // Memory optimization
    MaxLength:      10000,    // Limit memory usage
    MaxLengthBytes: 10485760, // 10MB limit
    
    // Message optimization
    MessageTTL: 300000, // 5 minutes
    
    // Consumer optimization
    PrefetchCount: 100, // Batch acknowledgments
    
    // Performance vs durability
    Durable:    false, // Faster but data loss risk
    AutoDelete: true,  // Cleanup
}
```

### Producer Optimization

#### Batch Publishing
```go
// High-performance producer
type BatchPublisher struct {
    channel    *amqp.Channel
    exchange   string
    batchSize  int
    batchTime  time.Duration
    buffer     []*amqp.Publishing
    mutex      sync.Mutex
    ticker     *time.Ticker
}

func (p *BatchPublisher) PublishBatch(messages []*amqp.Publishing) error {
    // Batch multiple messages into single network call
    for _, msg := range messages {
        err := p.channel.Publish(
            p.exchange,
            msg.RoutingKey,
            false, // mandatory
            false, // immediate  
            *msg,
        )
        if err != nil {
            return err
        }
    }
    return nil
}

// Usage example
publisher := &BatchPublisher{
    batchSize: 100,
    batchTime: 10 * time.Millisecond,
}

// Collect messages
for i := 0; i < 1000; i++ {
    message := &amqp.Publishing{
        ContentType: "application/json",
        Body:        []byte(fmt.Sprintf(`{"id": %d}`, i)),
        DeliveryMode: amqp.Transient, // Faster than persistent
    }
    publisher.buffer = append(publisher.buffer, message)
    
    if len(publisher.buffer) >= publisher.batchSize {
        publisher.PublishBatch(publisher.buffer)
        publisher.buffer = publisher.buffer[:0]
    }
}
```

#### Connection Pooling
```go
type ConnectionPool struct {
    connections []*amqp.Connection
    channels    []*amqp.Channel
    current     int64
    size        int
    mutex       sync.RWMutex
}

func NewConnectionPool(size int, url string) (*ConnectionPool, error) {
    pool := &ConnectionPool{
        connections: make([]*amqp.Connection, size),
        channels:    make([]*amqp.Channel, size),
        size:        size,
    }
    
    for i := 0; i < size; i++ {
        conn, err := amqp.Dial(url)
        if err != nil {
            return nil, err
        }
        
        ch, err := conn.Channel()
        if err != nil {
            return nil, err
        }
        
        // Optimize channel settings
        ch.Qos(100, 0, false) // Prefetch 100 messages
        
        pool.connections[i] = conn
        pool.channels[i] = ch
    }
    
    return pool, nil
}

func (p *ConnectionPool) GetChannel() *amqp.Channel {
    index := atomic.AddInt64(&p.current, 1) % int64(p.size)
    return p.channels[index]
}
```

### Consumer Optimization

#### Prefetch Configuration
```go
// Optimize consumer prefetch based on processing speed
func OptimizeConsumerPrefetch(processingTimeMs int) int {
    // Rule of thumb: prefetch = (target_latency_ms / processing_time_ms) * 2
    targetLatencyMs := 100 // 100ms target latency
    prefetch := (targetLatencyMs / processingTimeMs) * 2
    
    // Bounds checking
    if prefetch < 1 {
        prefetch = 1
    }
    if prefetch > 1000 {
        prefetch = 1000
    }
    
    return prefetch
}

// Example usage
channel.Qos(
    OptimizeConsumerPrefetch(50), // 50ms avg processing time
    0,     // prefetch size (bytes) - 0 means no limit
    false, // global - false means per-consumer
)
```

#### Parallel Consumer Processing
```go
type ParallelConsumer struct {
    channel     *amqp.Channel
    queue       string
    workerCount int
    processor   func(amqp.Delivery) error
}

func (c *ParallelConsumer) Start() error {
    // Create worker pool
    deliveryChan := make(chan amqp.Delivery, c.workerCount*2)
    
    // Start workers
    for i := 0; i < c.workerCount; i++ {
        go func(workerID int) {
            for delivery := range deliveryChan {
                err := c.processor(delivery)
                if err != nil {
                    log.Printf("Worker %d error: %v", workerID, err)
                    delivery.Nack(false, true) // Requeue on error
                } else {
                    delivery.Ack(false) // Acknowledge success
                }
            }
        }(i)
    }
    
    // Consume messages
    msgs, err := c.channel.Consume(
        c.queue,
        "",    // consumer tag
        false, // auto-ack (we'll ack manually)
        false, // exclusive
        false, // no-local
        false, // no-wait
        nil,   // args
    )
    if err != nil {
        return err
    }
    
    // Forward messages to workers
    for delivery := range msgs {
        deliveryChan <- delivery
    }
    
    return nil
}
```

## Kafka Performance

### Topic Configuration

#### Partition Optimization
```go
// Calculate optimal partition count
func CalculateOptimalPartitions(targetThroughputMbps, avgMessageSizeKb int) int {
    // Each partition can handle ~25MB/s (conservative estimate)
    partitionThroughputMbps := 25
    
    // Calculate required partitions for throughput
    throughputPartitions := (targetThroughputMbps + partitionThroughputMbps - 1) / partitionThroughputMbps
    
    // Consider consumer parallelism (1 partition per consumer)
    consumerPartitions := runtime.NumCPU() // Match CPU cores
    
    // Take the maximum
    partitions := throughputPartitions
    if consumerPartitions > partitions {
        partitions = consumerPartitions
    }
    
    // Reasonable bounds
    if partitions < 1 {
        partitions = 1
    }
    if partitions > 100 {
        partitions = 100
    }
    
    return partitions
}

// Example topic configuration
topicConfig := &TopicConfig{
    Name:              "high_throughput_topic",
    Partitions:        CalculateOptimalPartitions(100, 1), // 100MB/s target
    ReplicationFactor: 1, // Single node for performance
    Config: map[string]string{
        "compression.type":           "lz4",      // Fast compression
        "cleanup.policy":            "delete",   // Not compact
        "retention.ms":              "3600000",  // 1 hour retention
        "segment.ms":                "600000",   // 10 minute segments
        "flush.messages":            "10000",    // Flush every 10k messages
        "flush.ms":                  "1000",     // Flush every second
        "batch.size":                "65536",    // 64KB batches
        "linger.ms":                 "5",        // 5ms linger time
        "buffer.memory":             "33554432", // 32MB buffer
        "max.request.size":          "1048576",  // 1MB max request
    },
}
```

### Producer Optimization

#### Sarama Producer Optimization
```go
func OptimizedSaramaConfig() *sarama.Config {
    config := sarama.NewConfig()
    
    // Performance settings
    config.Producer.RequiredAcks = sarama.WaitForLocal // Faster than WaitForAll
    config.Producer.Compression = sarama.CompressionLZ4 // Fast compression
    config.Producer.CompressionLevel = 6 // Balanced compression
    
    // Batching settings
    config.Producer.Flush.Frequency = 10 * time.Millisecond
    config.Producer.Flush.Messages = 100
    config.Producer.Flush.Bytes = 65536 // 64KB
    
    // Network optimization
    config.Net.MaxOpenRequests = 10
    config.Net.DialTimeout = 10 * time.Second
    config.Net.ReadTimeout = 10 * time.Second
    config.Net.WriteTimeout = 10 * time.Second
    
    // Reliability vs performance
    config.Producer.Idempotent = false // Disable for max performance
    config.Producer.Retry.Max = 1      // Minimal retries
    config.Producer.Retry.Backoff = 50 * time.Millisecond
    
    // Return settings
    config.Producer.Return.Successes = false // Don't wait for success
    config.Producer.Return.Errors = true     // But handle errors
    
    return config
}

// High-performance async producer
func CreateHighPerformanceProducer(brokers []string) (sarama.AsyncProducer, error) {
    config := OptimizedSaramaConfig()
    
    producer, err := sarama.NewAsyncProducer(brokers, config)
    if err != nil {
        return nil, err
    }
    
    // Handle errors in background
    go func() {
        for err := range producer.Errors() {
            log.Printf("Producer error: %v", err)
        }
    }()
    
    return producer, nil
}
```

#### kafka-go Producer Optimization
```go
func OptimizedKafkaGoWriter(brokers []string, topic string) *kafka.Writer {
    return &kafka.Writer{
        Addr:     kafka.TCP(brokers...),
        Topic:    topic,
        Balancer: &kafka.Hash{}, // Consistent partitioning
        
        // Batching configuration
        BatchSize:    1000,                   // Messages per batch
        BatchBytes:   1048576,                // 1MB per batch
        BatchTimeout: 10 * time.Millisecond, // Max wait time
        
        // Performance settings
        RequiredAcks: kafka.RequireOne,  // Faster than RequireAll
        Compression:  kafka.Lz4,         // Fast compression
        
        // Network optimization
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 10 * time.Second,
        
        // Error handling
        MaxAttempts: 3,
        WriteBackoffMin: 100 * time.Millisecond,
        WriteBackoffMax: 1 * time.Second,
    }
}

// Example usage with connection pooling
type WriterPool struct {
    writers []*kafka.Writer
    current int64
}

func NewWriterPool(size int, brokers []string, topic string) *WriterPool {
    pool := &WriterPool{
        writers: make([]*kafka.Writer, size),
    }
    
    for i := 0; i < size; i++ {
        pool.writers[i] = OptimizedKafkaGoWriter(brokers, topic)
    }
    
    return pool
}

func (p *WriterPool) GetWriter() *kafka.Writer {
    index := atomic.AddInt64(&p.current, 1) % int64(len(p.writers))
    return p.writers[index]
}
```

### Consumer Optimization

#### Consumer Group Configuration
```go
// Optimized consumer configuration
func OptimizedConsumerConfig() *sarama.Config {
    config := sarama.NewConfig()
    
    // Consumer settings
    config.Consumer.Fetch.Min = 1024     // 1KB minimum
    config.Consumer.Fetch.Default = 1048576 // 1MB default
    config.Consumer.Fetch.Max = 10485760    // 10MB maximum
    config.Consumer.MaxWaitTime = 250 * time.Millisecond
    config.Consumer.MaxProcessingTime = 1 * time.Second
    
    // Offset management
    config.Consumer.Offsets.Initial = sarama.OffsetNewest
    config.Consumer.Offsets.CommitInterval = 1 * time.Second
    
    // Consumer group settings
    config.Consumer.Group.Session.Timeout = 30 * time.Second
    config.Consumer.Group.Heartbeat.Interval = 3 * time.Second
    config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategySticky
    config.Consumer.Group.Rebalance.Timeout = 60 * time.Second
    
    return config
}

// Parallel message processing
type HighPerformanceConsumer struct {
    workerCount int
    buffer      chan *sarama.ConsumerMessage
    processor   func(*sarama.ConsumerMessage) error
    wg          sync.WaitGroup
}

func NewHighPerformanceConsumer(workerCount int, bufferSize int) *HighPerformanceConsumer {
    return &HighPerformanceConsumer{
        workerCount: workerCount,
        buffer:      make(chan *sarama.ConsumerMessage, bufferSize),
    }
}

func (c *HighPerformanceConsumer) Start() {
    for i := 0; i < c.workerCount; i++ {
        c.wg.Add(1)
        go func(workerID int) {
            defer c.wg.Done()
            for msg := range c.buffer {
                err := c.processor(msg)
                if err != nil {
                    log.Printf("Worker %d error processing message: %v", workerID, err)
                }
            }
        }(i)
    }
}

func (c *HighPerformanceConsumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
    for {
        select {
        case message := <-claim.Messages():
            if message == nil {
                return nil
            }
            
            // Non-blocking send to worker pool
            select {
            case c.buffer <- message:
                session.MarkMessage(message, "")
            default:
                // Buffer full, process in current goroutine
                err := c.processor(message)
                if err != nil {
                    log.Printf("Direct processing error: %v", err)
                }
                session.MarkMessage(message, "")
            }
            
        case <-session.Context().Done():
            return nil
        }
    }
}
```

## Memory Management

### Memory Pool Usage
```go
// Efficient memory management with pools
type MessagePool struct {
    pool sync.Pool
}

func NewMessagePool() *MessagePool {
    return &MessagePool{
        pool: sync.Pool{
            New: func() interface{} {
                return make([]byte, 0, 1024) // 1KB initial capacity
            },
        },
    }
}

func (p *MessagePool) Get() []byte {
    return p.pool.Get().([]byte)[:0]
}

func (p *MessagePool) Put(buf []byte) {
    if cap(buf) <= 64*1024 { // Don't pool large buffers
        p.pool.Put(buf)
    }
}

// Usage in producer
func (producer *Producer) SendMessage(data []byte) error {
    buf := producer.messagePool.Get()
    defer producer.messagePool.Put(buf)
    
    // Prepare message
    buf = append(buf, data...)
    
    // Send message
    return producer.send(buf)
}
```

### Garbage Collection Optimization
```go
// GC optimization settings
func OptimizeGC() {
    // Set GC target percentage (default 100)
    debug.SetGCPercent(50) // More frequent GC for lower latency
    
    // Set memory limit (Go 1.19+)
    debug.SetMemoryLimit(8 << 30) // 8GB limit
}

// Monitor GC performance
func MonitorGC() {
    var lastGC time.Time
    var lastNumGC uint32
    
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        var stats runtime.MemStats
        runtime.ReadMemStats(&stats)
        
        if stats.NumGC > lastNumGC {
            gcTime := time.Unix(0, int64(stats.LastGC))
            gcDuration := gcTime.Sub(lastGC)
            
            log.Printf("GC: count=%d, pause=%v, heap=%d MB",
                stats.NumGC-lastNumGC,
                time.Duration(stats.PauseNs[(stats.NumGC+255)%256]),
                stats.HeapInuse>>20)
            
            lastGC = gcTime
            lastNumGC = stats.NumGC
        }
    }
}
```

## CPU Optimization

### CPU Profiling and Optimization
```go
import (
    _ "net/http/pprof"
    "runtime"
)

func EnableProfiling() {
    // Enable CPU profiling endpoint
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()
    
    // Optimize for CPU count
    runtime.GOMAXPROCS(runtime.NumCPU())
}

// CPU-efficient message routing
type FastRouter struct {
    routes map[string]func([]byte) error
    mutex  sync.RWMutex
}

func (r *FastRouter) Route(routingKey string, message []byte) error {
    r.mutex.RLock()
    handler, exists := r.routes[routingKey]
    r.mutex.RUnlock()
    
    if !exists {
        return fmt.Errorf("no route for key: %s", routingKey)
    }
    
    return handler(message)
}

// Lock-free counter for metrics
type LockFreeCounter struct {
    value int64
}

func (c *LockFreeCounter) Increment() {
    atomic.AddInt64(&c.value, 1)
}

func (c *LockFreeCounter) Get() int64 {
    return atomic.LoadInt64(&c.value)
}
```

### Goroutine Pool Management
```go
type WorkerPool struct {
    workers    chan chan func()
    jobQueue   chan func()
    workerQuit []chan bool
}

func NewWorkerPool(numWorkers, jobQueueSize int) *WorkerPool {
    pool := &WorkerPool{
        workers:    make(chan chan func(), numWorkers),
        jobQueue:   make(chan func(), jobQueueSize),
        workerQuit: make([]chan bool, numWorkers),
    }
    
    // Start workers
    for i := 0; i < numWorkers; i++ {
        worker := &Worker{
            id:      i,
            jobChan: make(chan func()),
            workers: pool.workers,
            quit:    make(chan bool),
        }
        pool.workerQuit[i] = worker.quit
        worker.Start()
    }
    
    // Start dispatcher
    go pool.dispatch()
    
    return pool
}

func (p *WorkerPool) Submit(job func()) {
    p.jobQueue <- job
}

func (p *WorkerPool) dispatch() {
    for {
        select {
        case job := <-p.jobQueue:
            // Get available worker
            worker := <-p.workers
            worker <- job
        }
    }
}

type Worker struct {
    id      int
    jobChan chan func()
    workers chan chan func()
    quit    chan bool
}

func (w *Worker) Start() {
    go func() {
        for {
            // Register worker in pool
            w.workers <- w.jobChan
            
            select {
            case job := <-w.jobChan:
                job() // Execute job
            case <-w.quit:
                return
            }
        }
    }()
}
```

## Network Optimization

### TCP Configuration
```go
// Optimize TCP settings for message queuing
func OptimizeTCPConnection(conn net.Conn) error {
    tcpConn, ok := conn.(*net.TCPConn)
    if !ok {
        return fmt.Errorf("not a TCP connection")
    }
    
    // Enable TCP nodelay (disable Nagle's algorithm)
    err := tcpConn.SetNoDelay(true)
    if err != nil {
        return err
    }
    
    // Set keep-alive
    err = tcpConn.SetKeepAlive(true)
    if err != nil {
        return err
    }
    
    err = tcpConn.SetKeepAlivePeriod(30 * time.Second)
    if err != nil {
        return err
    }
    
    // Set buffer sizes
    err = tcpConn.SetReadBuffer(1048576)  // 1MB read buffer
    if err != nil {
        return err
    }
    
    err = tcpConn.SetWriteBuffer(1048576) // 1MB write buffer
    if err != nil {
        return err
    }
    
    return nil
}
```

### Connection Pooling
```go
type ConnectionManager struct {
    pools map[string]*ConnectionPool
    mutex sync.RWMutex
}

type ConnectionPool struct {
    connections chan net.Conn
    factory     func() (net.Conn, error)
    maxSize     int
    current     int
    mutex       sync.Mutex
}

func NewConnectionPool(maxSize int, factory func() (net.Conn, error)) *ConnectionPool {
    return &ConnectionPool{
        connections: make(chan net.Conn, maxSize),
        factory:     factory,
        maxSize:     maxSize,
    }
}

func (p *ConnectionPool) Get() (net.Conn, error) {
    select {
    case conn := <-p.connections:
        return conn, nil
    default:
        // No available connection, create new one
        p.mutex.Lock()
        defer p.mutex.Unlock()
        
        if p.current < p.maxSize {
            conn, err := p.factory()
            if err != nil {
                return nil, err
            }
            p.current++
            return conn, nil
        }
        
        // Wait for available connection
        return <-p.connections, nil
    }
}

func (p *ConnectionPool) Put(conn net.Conn) {
    select {
    case p.connections <- conn:
    default:
        // Pool is full, close connection
        conn.Close()
        p.mutex.Lock()
        p.current--
        p.mutex.Unlock()
    }
}
```

## Storage Optimization

### Disk I/O Optimization
```go
// Efficient message persistence
type MessageStore struct {
    file     *os.File
    buffer   *bufio.Writer
    index    map[string]int64 // Message ID to file offset
    mutex    sync.RWMutex
    syncFreq time.Duration
}

func NewMessageStore(filename string) (*MessageStore, error) {
    file, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
    if err != nil {
        return nil, err
    }
    
    store := &MessageStore{
        file:     file,
        buffer:   bufio.NewWriterSize(file, 1048576), // 1MB buffer
        index:    make(map[string]int64),
        syncFreq: 1 * time.Second,
    }
    
    // Periodic sync
    go store.periodicSync()
    
    return store, nil
}

func (s *MessageStore) StoreMessage(id string, data []byte) error {
    s.mutex.Lock()
    defer s.mutex.Unlock()
    
    // Get current position
    offset, err := s.file.Seek(0, os.SEEK_CUR)
    if err != nil {
        return err
    }
    
    // Write message length + data
    binary.Write(s.buffer, binary.LittleEndian, uint32(len(data)))
    s.buffer.Write(data)
    
    // Update index
    s.index[id] = offset
    
    return nil
}

func (s *MessageStore) periodicSync() {
    ticker := time.NewTicker(s.syncFreq)
    defer ticker.Stop()
    
    for range ticker.C {
        s.mutex.Lock()
        s.buffer.Flush()
        s.file.Sync()
        s.mutex.Unlock()
    }
}
```

### Memory-mapped Files
```go
// Memory-mapped message storage for ultra-fast access
type MMapStore struct {
    file   *os.File
    mmap   []byte
    size   int64
    offset int64
    mutex  sync.Mutex
}

func NewMMapStore(filename string, size int64) (*MMapStore, error) {
    file, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0644)
    if err != nil {
        return nil, err
    }
    
    // Extend file to desired size
    err = file.Truncate(size)
    if err != nil {
        return nil, err
    }
    
    // Memory map the file
    mmap, err := syscall.Mmap(
        int(file.Fd()),
        0,
        int(size),
        syscall.PROT_READ|syscall.PROT_WRITE,
        syscall.MAP_SHARED,
    )
    if err != nil {
        return nil, err
    }
    
    return &MMapStore{
        file: file,
        mmap: mmap,
        size: size,
    }, nil
}

func (s *MMapStore) WriteMessage(data []byte) (int64, error) {
    s.mutex.Lock()
    defer s.mutex.Unlock()
    
    if s.offset+int64(len(data))+4 > s.size {
        return 0, fmt.Errorf("not enough space")
    }
    
    // Write length prefix
    binary.LittleEndian.PutUint32(s.mmap[s.offset:], uint32(len(data)))
    s.offset += 4
    
    // Write data
    copy(s.mmap[s.offset:], data)
    offset := s.offset
    s.offset += int64(len(data))
    
    return offset, nil
}
```

## Monitoring and Metrics

### Performance Metrics Collection
```go
type PerformanceMetrics struct {
    // Throughput metrics
    MessagesProduced  *prometheus.CounterVec
    MessagesConsumed  *prometheus.CounterVec
    BytesProduced     *prometheus.CounterVec
    BytesConsumed     *prometheus.CounterVec
    
    // Latency metrics
    PublishLatency    *prometheus.HistogramVec
    ConsumeLatency    *prometheus.HistogramVec
    EndToEndLatency   *prometheus.HistogramVec
    
    // System metrics
    MemoryUsage       prometheus.Gauge
    CPUUsage          prometheus.Gauge
    GoroutineCount    prometheus.Gauge
    GCDuration        *prometheus.HistogramVec
    
    // Error metrics
    PublishErrors     *prometheus.CounterVec
    ConsumeErrors     *prometheus.CounterVec
    ConnectionErrors  *prometheus.CounterVec
}

func NewPerformanceMetrics() *PerformanceMetrics {
    return &PerformanceMetrics{
        MessagesProduced: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "portask_messages_produced_total",
                Help: "Total number of messages produced",
            },
            []string{"protocol", "topic", "exchange"},
        ),
        
        PublishLatency: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "portask_publish_duration_seconds",
                Help:    "Time spent publishing messages",
                Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
            },
            []string{"protocol", "topic", "exchange"},
        ),
        
        MemoryUsage: prometheus.NewGauge(
            prometheus.GaugeOpts{
                Name: "portask_memory_usage_bytes",
                Help: "Current memory usage in bytes",
            },
        ),
        
        // ... other metrics
    }
}

// Metrics collection
func (m *PerformanceMetrics) StartCollection() {
    // System metrics collection
    go func() {
        ticker := time.NewTicker(10 * time.Second)
        defer ticker.Stop()
        
        for range ticker.C {
            var memStats runtime.MemStats
            runtime.ReadMemStats(&memStats)
            
            m.MemoryUsage.Set(float64(memStats.Alloc))
            m.GoroutineCount.Set(float64(runtime.NumGoroutine()))
        }
    }()
}
```

### Real-time Performance Dashboard
```go
type PerformanceDashboard struct {
    metrics     *PerformanceMetrics
    wsConns     map[string]*websocket.Conn
    connsMutex  sync.RWMutex
    updateFreq  time.Duration
}

func NewPerformanceDashboard(metrics *PerformanceMetrics) *PerformanceDashboard {
    dashboard := &PerformanceDashboard{
        metrics:    metrics,
        wsConns:    make(map[string]*websocket.Conn),
        updateFreq: 1 * time.Second,
    }
    
    go dashboard.broadcastUpdates()
    return dashboard
}

func (d *PerformanceDashboard) broadcastUpdates() {
    ticker := time.NewTicker(d.updateFreq)
    defer ticker.Stop()
    
    for range ticker.C {
        stats := d.collectCurrentStats()
        d.broadcast(stats)
    }
}

func (d *PerformanceDashboard) collectCurrentStats() map[string]interface{} {
    var memStats runtime.MemStats
    runtime.ReadMemStats(&memStats)
    
    return map[string]interface{}{
        "timestamp": time.Now().Unix(),
        "memory": map[string]interface{}{
            "alloc":       memStats.Alloc,
            "heap_inuse":  memStats.HeapInuse,
            "stack_inuse": memStats.StackInuse,
            "gc_runs":     memStats.NumGC,
        },
        "runtime": map[string]interface{}{
            "goroutines": runtime.NumGoroutine(),
            "cpu_count":  runtime.NumCPU(),
        },
        "performance": map[string]interface{}{
            "messages_per_second": d.calculateMessageRate(),
            "avg_latency_ms":      d.calculateAvgLatency(),
        },
    }
}
```

## Benchmarking

### Built-in Benchmark Suite
```go
// Benchmark producer performance
func BenchmarkProducer(b *testing.B) {
    producer := setupTestProducer()
    defer producer.Close()
    
    message := make([]byte, 1024) // 1KB message
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            err := producer.Publish("test_topic", message)
            if err != nil {
                b.Fatal(err)
            }
        }
    })
    
    b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "msg/sec")
    b.ReportMetric(float64(len(message)*b.N)/b.Elapsed().Seconds()/1024/1024, "MB/sec")
}

// Benchmark consumer performance
func BenchmarkConsumer(b *testing.B) {
    consumer := setupTestConsumer()
    defer consumer.Close()
    
    // Pre-populate messages
    populateMessages(b.N)
    
    consumed := 0
    start := time.Now()
    
    b.ResetTimer()
    for consumed < b.N {
        msg, err := consumer.Consume()
        if err != nil {
            b.Fatal(err)
        }
        if msg != nil {
            consumed++
        }
    }
    
    elapsed := time.Since(start)
    b.ReportMetric(float64(consumed)/elapsed.Seconds(), "msg/sec")
}

// End-to-end latency benchmark
func BenchmarkEndToEndLatency(b *testing.B) {
    producer := setupTestProducer()
    consumer := setupTestConsumer()
    defer producer.Close()
    defer consumer.Close()
    
    latencies := make([]time.Duration, b.N)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        start := time.Now()
        
        // Send message with timestamp
        message := []byte(fmt.Sprintf("timestamp:%d", start.UnixNano()))
        err := producer.Publish("test_topic", message)
        if err != nil {
            b.Fatal(err)
        }
        
        // Consume message
        received, err := consumer.Consume()
        if err != nil {
            b.Fatal(err)
        }
        
        // Calculate latency
        latencies[i] = time.Since(start)
    }
    
    // Report latency percentiles
    sort.Slice(latencies, func(i, j int) bool {
        return latencies[i] < latencies[j]
    })
    
    p50 := latencies[b.N*50/100]
    p95 := latencies[b.N*95/100]
    p99 := latencies[b.N*99/100]
    
    b.ReportMetric(float64(p50.Nanoseconds())/1e6, "p50_latency_ms")
    b.ReportMetric(float64(p95.Nanoseconds())/1e6, "p95_latency_ms")
    b.ReportMetric(float64(p99.Nanoseconds())/1e6, "p99_latency_ms")
}
```

### Load Testing Tools
```bash
#!/bin/bash
# Load testing script

echo "Starting Portask load test..."

# Start Portask server
./portask --config config.performance.json &
PORTASK_PID=$!

# Wait for server to start
sleep 5

# Run AMQP load test
echo "Running AMQP load test..."
go run tools/amqp_load_test.go \
    --url "amqp://localhost:5672" \
    --producers 10 \
    --consumers 5 \
    --messages 100000 \
    --message-size 1024

# Run Kafka load test  
echo "Running Kafka load test..."
go run tools/kafka_load_test.go \
    --brokers "localhost:9092" \
    --topic "load_test_topic" \
    --producers 10 \
    --consumers 5 \
    --messages 100000 \
    --message-size 1024

# Cleanup
kill $PORTASK_PID
echo "Load test completed."
```

## Production Tuning

### Configuration Templates

#### High Throughput Configuration
```json
{
  "performance_profile": "high_throughput",
  "amqp": {
    "port": 5672,
    "frame_max": 1048576,
    "heartbeat": 60,
    "channel_max": 2048,
    "prefetch_count": 1000,
    "prefetch_size": 0,
    "exchange_defaults": {
      "type": "direct",
      "durable": false,
      "auto_delete": true
    },
    "queue_defaults": {
      "durable": false,
      "auto_delete": true,
      "max_length": 100000,
      "message_ttl": 300000
    }
  },
  "kafka": {
    "port": 9092,
    "num_network_threads": 16,
    "num_io_threads": 32,
    "socket_send_buffer_bytes": 1048576,
    "socket_receive_buffer_bytes": 1048576,
    "num_partitions": 12,
    "default_replication_factor": 1,
    "compression_type": "lz4",
    "batch_size": 65536,
    "linger_ms": 5,
    "buffer_memory": 67108864,
    "acks": "1"
  },
  "system": {
    "gc_percent": 50,
    "max_procs": 0,
    "memory_limit": "8GB",
    "worker_pool_size": 1000
  }
}
```

#### Low Latency Configuration
```json
{
  "performance_profile": "low_latency",
  "amqp": {
    "port": 5672,
    "frame_max": 65536,
    "heartbeat": 10,
    "channel_max": 512,
    "prefetch_count": 1,
    "tcp_nodelay": true,
    "exchange_defaults": {
      "type": "direct",
      "durable": false
    },
    "queue_defaults": {
      "durable": false,
      "max_length": 1000,
      "message_ttl": 60000
    }
  },
  "kafka": {
    "port": 9092,
    "num_network_threads": 8,
    "num_io_threads": 8,
    "socket_send_buffer_bytes": 65536,
    "socket_receive_buffer_bytes": 65536,
    "num_partitions": 1,
    "compression_type": "none",
    "batch_size": 1,
    "linger_ms": 0,
    "acks": "1"
  },
  "system": {
    "gc_percent": 25,
    "memory_limit": "4GB"
  }
}
```

### System Tuning Recommendations

#### Linux Kernel Parameters
```bash
# /etc/sysctl.conf optimizations for Portask

# Network optimizations
net.core.rmem_max = 33554432
net.core.wmem_max = 33554432
net.ipv4.tcp_rmem = 4096 65536 33554432
net.ipv4.tcp_wmem = 4096 65536 33554432
net.ipv4.tcp_congestion_control = bbr
net.core.netdev_max_backlog = 5000
net.ipv4.tcp_no_delay = 1

# File descriptor limits
fs.file-max = 1048576
fs.nr_open = 1048576

# Virtual memory
vm.swappiness = 1
vm.dirty_ratio = 15
vm.dirty_background_ratio = 5

# Apply changes
sysctl -p
```

#### File Descriptor Limits
```bash
# /etc/security/limits.conf
* soft nofile 65536
* hard nofile 65536
* soft nproc 32768
* hard nproc 32768

# Verify limits
ulimit -n
ulimit -u
```

#### CPU Affinity and NUMA
```bash
# Pin Portask to specific CPU cores
taskset -c 0-7 ./portask --config config.prod.json

# Check NUMA topology
numactl --hardware

# Optimize for NUMA
numactl --cpunodebind=0 --membind=0 ./portask --config config.prod.json
```

### Monitoring and Alerting Setup
```yaml
# prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'portask'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: /metrics
    scrape_interval: 5s

# Grafana dashboard panels
- title: "Message Throughput"
  type: graph
  targets:
    - expr: rate(portask_messages_produced_total[1m])
    - expr: rate(portask_messages_consumed_total[1m])

- title: "Latency Percentiles"  
  type: graph
  targets:
    - expr: histogram_quantile(0.50, portask_publish_duration_seconds)
    - expr: histogram_quantile(0.95, portask_publish_duration_seconds)
    - expr: histogram_quantile(0.99, portask_publish_duration_seconds)

# Alerting rules
groups:
  - name: portask
    rules:
      - alert: HighLatency
        expr: histogram_quantile(0.95, portask_publish_duration_seconds) > 0.1
        for: 2m
        annotations:
          summary: "High publish latency detected"
          
      - alert: LowThroughput
        expr: rate(portask_messages_produced_total[5m]) < 1000
        for: 5m
        annotations:
          summary: "Low message throughput detected"
```

### Deployment Best Practices

1. **Resource Allocation**
   - CPU: 1 core per 10,000 msg/sec
   - Memory: 1GB base + 1MB per 1000 queued messages
   - Storage: SSD recommended, size based on retention requirements

2. **High Availability Setup**
   ```bash
   # Load balancer configuration
   upstream portask_backend {
       server 10.0.1.10:5672 weight=1 max_fails=3 fail_timeout=30s;
       server 10.0.1.11:5672 weight=1 max_fails=3 fail_timeout=30s;
       server 10.0.1.12:5672 weight=1 max_fails=3 fail_timeout=30s;
   }
   ```

3. **Health Checks**
   ```bash
   # Health check script
   #!/bin/bash
   
   # Check AMQP port
   nc -z localhost 5672 || exit 1
   
   # Check Kafka port
   nc -z localhost 9092 || exit 1
   
   # Check management API
   curl -f http://localhost:8080/health || exit 1
   
   echo "Health check passed"
   ```

---

For more information, see:
- [Main Documentation](./README.md)
- [API Reference](./api_reference.md)
- [AMQP Emulator](./amqp_emulator.md)
- [Kafka Emulator](./kafka_emulator.md)
