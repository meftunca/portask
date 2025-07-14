# Kafka Emulator Documentation

## Overview

The Portask Kafka emulator provides seamless compatibility with Kafka client libraries, supporting both the popular Sarama and kafka-go libraries. This flexibility allows developers to choose their preferred client while maintaining full compatibility with the Kafka protocol.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│            Kafka Client Libraries                       │
│    ┌─────────────────┐    ┌─────────────────┐         │
│    │     Sarama      │    │    kafka-go     │         │
│    │  (IBM/sarama)   │    │ (segmentio)     │         │
│    └─────────────────┘    └─────────────────┘         │
├─────────────────────────────────────────────────────────┤
│                 Kafka Protocol Layer                    │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────────┐   │
│  │  Producer   │ │  Consumer   │ │    Admin API    │   │
│  │   Handler   │ │   Handler   │ │    Handler      │   │
│  └─────────────┘ └─────────────┘ └─────────────────┘   │
├─────────────────────────────────────────────────────────┤
│                Portask Kafka Bridge                     │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────────┐   │
│  │   Topic     │ │ Partition   │ │   Consumer      │   │
│  │  Manager    │ │  Manager    │ │     Groups      │   │
│  └─────────────┘ └─────────────┘ └─────────────────┘   │
├─────────────────────────────────────────────────────────┤
│                 Message Engine Core                     │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────────┐   │
│  │   Topics    │ │ Partitions  │ │    Offsets      │   │
│  │ (Replicated │ │ (Leader/    │ │   (Commit       │   │
│  │  Metadata)  │ │ Follower)   │ │   Tracking)     │   │
│  └─────────────┘ └─────────────┘ └─────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

## Client Library Support

### Sarama Client (IBM/sarama)

Sarama is the most mature and feature-complete Kafka client library for Go, offering extensive configuration options and enterprise-grade features.

#### Features
- **Complete Kafka Protocol Support**: All Kafka APIs
- **Advanced Configuration**: Fine-tuned control over all aspects
- **Production Ready**: Battle-tested in large-scale deployments
- **Synchronous and Asynchronous**: Both sync and async producers/consumers
- **Consumer Groups**: Full consumer group implementation
- **Admin Operations**: Topic, partition, and configuration management

#### Installation
```bash
go get github.com/IBM/sarama
```

#### Basic Producer Example
```go
package main

import (
    "log"
    "github.com/IBM/sarama"
)

func main() {
    config := sarama.NewConfig()
    config.Producer.Return.Successes = true
    config.Producer.RequiredAcks = sarama.WaitForAll
    config.Producer.Retry.Max = 3
    config.Producer.Compression = sarama.CompressionSnappy

    // Connect to Portask instead of Kafka
    producer, err := sarama.NewSyncProducer([]string{"localhost:9092"}, config)
    if err != nil {
        log.Fatal("Failed to create producer:", err)
    }
    defer producer.Close()

    // Send message
    message := &sarama.ProducerMessage{
        Topic: "test-topic",
        Key:   sarama.StringEncoder("key-1"),
        Value: sarama.StringEncoder("Hello from Sarama!"),
        Headers: []sarama.RecordHeader{
            {
                Key:   []byte("source"),
                Value: []byte("sarama-client"),
            },
        },
        Timestamp: time.Now(),
    }

    partition, offset, err := producer.SendMessage(message)
    if err != nil {
        log.Fatal("Failed to send message:", err)
    }

    log.Printf("Message sent to partition %d, offset %d", partition, offset)
}
```

#### Consumer Group Example
```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "sync"
    "syscall"
    
    "github.com/IBM/sarama"
)

type Consumer struct {
    ready chan bool
}

func (consumer *Consumer) Setup(sarama.ConsumerGroupSession) error {
    close(consumer.ready)
    return nil
}

func (consumer *Consumer) Cleanup(sarama.ConsumerGroupSession) error {
    return nil
}

func (consumer *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
    for {
        select {
        case message := <-claim.Messages():
            if message == nil {
                return nil
            }
            log.Printf("Message claimed: value = %s, timestamp = %v, topic = %s, partition = %d, offset = %d",
                string(message.Value), message.Timestamp, message.Topic, message.Partition, message.Offset)
            
            // Mark message as processed
            session.MarkMessage(message, "")
            
        case <-session.Context().Done():
            return nil
        }
    }
}

func main() {
    config := sarama.NewConfig()
    config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
    config.Consumer.Offsets.Initial = sarama.OffsetOldest
    config.Consumer.Group.Session.Timeout = 10 * time.Second
    config.Consumer.Group.Heartbeat.Interval = 3 * time.Second

    consumer := Consumer{
        ready: make(chan bool),
    }

    ctx, cancel := context.WithCancel(context.Background())
    
    client, err := sarama.NewConsumerGroup([]string{"localhost:9092"}, "test-consumer-group", config)
    if err != nil {
        log.Fatal("Error creating consumer group client:", err)
    }

    wg := &sync.WaitGroup{}
    wg.Add(1)
    go func() {
        defer wg.Done()
        for {
            if err := client.Consume(ctx, []string{"test-topic"}, &consumer); err != nil {
                log.Printf("Error from consumer: %v", err)
            }
            if ctx.Err() != nil {
                return
            }
            consumer.ready = make(chan bool)
        }
    }()

    <-consumer.ready
    log.Println("Sarama consumer up and running!...")

    // Handle shutdown
    sigterm := make(chan os.Signal, 1)
    signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)
    
    <-sigterm
    log.Println("Terminating: via signal")
    
    cancel()
    wg.Wait()
    
    if err = client.Close(); err != nil {
        log.Printf("Error closing client: %v", err)
    }
}
```

### kafka-go Client (segmentio/kafka-go)

kafka-go provides a more modern, idiomatic Go API with a focus on simplicity and ease of use.

#### Features
- **Simple API**: Clean, Go-idiomatic interface
- **Connection-based**: Direct TCP connections to Kafka
- **Context Support**: Full context.Context integration
- **Minimal Dependencies**: Fewer external dependencies
- **Type Safety**: Strongly typed APIs
- **Performance**: Optimized for speed and low allocations

#### Installation
```bash
go get github.com/segmentio/kafka-go
```

#### Basic Producer Example
```go
package main

import (
    "context"
    "log"
    "time"
    
    "github.com/segmentio/kafka-go"
)

func main() {
    // Create writer (producer)
    writer := &kafka.Writer{
        Addr:     kafka.TCP("localhost:9092"),
        Topic:    "test-topic",
        Balancer: &kafka.LeastBytes{},
        
        // Optional configuration
        BatchSize:    100,                // messages
        BatchBytes:   1048576,            // 1MB
        BatchTimeout: 10 * time.Millisecond,
        RequiredAcks: kafka.RequireAll,
        Compression:  kafka.Snappy,
    }
    defer writer.Close()

    // Send messages
    messages := []kafka.Message{
        {
            Key:   []byte("key-1"),
            Value: []byte("Hello from kafka-go!"),
            Headers: []kafka.Header{
                {Key: "source", Value: []byte("kafka-go-client")},
            },
            Time: time.Now(),
        },
        {
            Key:   []byte("key-2"), 
            Value: []byte("Another message"),
            Time:  time.Now(),
        },
    }

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    err := writer.WriteMessages(ctx, messages...)
    if err != nil {
        log.Fatal("Failed to write messages:", err)
    }

    log.Printf("Messages sent successfully")
}
```

#### Consumer Example
```go
package main

import (
    "context"
    "log"
    "time"
    
    "github.com/segmentio/kafka-go"
)

func main() {
    // Create reader (consumer)
    reader := kafka.NewReader(kafka.ReaderConfig{
        Brokers:     []string{"localhost:9092"},
        GroupID:     "test-consumer-group",
        Topic:       "test-topic",
        MinBytes:    10e3, // 10KB
        MaxBytes:    10e6, // 10MB
        MaxWait:     1 * time.Second,
        StartOffset: kafka.FirstOffset,
        
        // Consumer group configuration
        GroupBalancers: []kafka.GroupBalancer{
            &kafka.RoundRobinGroupBalancer{},
        },
    })
    defer reader.Close()

    log.Println("kafka-go consumer started...")

    for {
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        
        message, err := reader.ReadMessage(ctx)
        if err != nil {
            cancel()
            log.Printf("Error reading message: %v", err)
            continue
        }
        
        log.Printf("Message received: key = %s, value = %s, topic = %s, partition = %d, offset = %d",
            string(message.Key), string(message.Value), message.Topic, message.Partition, message.Offset)
        
        // Message is automatically committed
        cancel()
    }
}
```

## Portask Bridge Configuration

The Portask Kafka bridge allows you to choose between Sarama and kafka-go clients based on your requirements.

### Bridge Configuration
```go
// Choose client type
type KafkaClientType string

const (
    KafkaClientSarama  KafkaClientType = "sarama"
    KafkaClientKafkaGo KafkaClientType = "kafka-go"
)

// Configuration structure
type KafkaIntegrationConfig struct {
    ClientType        KafkaClientType `json:"client_type"`
    BrokerAddresses   []string        `json:"broker_addresses"`
    TopicPrefix       string          `json:"topic_prefix"`
    ConsumerGroup     string          `json:"consumer_group"`
    PartitionCount    int32           `json:"partition_count"`
    ReplicationFactor int16           `json:"replication_factor"`
}

// Create bridge with chosen client
func NewKafkaPortaskBridge(config *KafkaIntegrationConfig, portaskAddr string, topic string) (*KafkaPortaskBridge, error) {
    switch config.ClientType {
    case KafkaClientSarama:
        return NewSaramaKafkaBridge(config, portaskAddr, topic)
    case KafkaClientKafkaGo:
        return NewKafkaGoBridge(config, portaskAddr, topic)
    default:
        return nil, fmt.Errorf("unknown Kafka client type: %s", config.ClientType)
    }
}
```

### Example Bridge Usage
```go
// Configuration for Sarama client
saramaConfig := &KafkaIntegrationConfig{
    ClientType:        KafkaClientSarama,
    BrokerAddresses:   []string{"localhost:9092"},
    TopicPrefix:       "portask_test",
    ConsumerGroup:     "portask_group",
    PartitionCount:    3,
    ReplicationFactor: 1,
}

// Configuration for kafka-go client
kafkaGoConfig := &KafkaIntegrationConfig{
    ClientType:        KafkaClientKafkaGo,
    BrokerAddresses:   []string{"localhost:9092"},
    TopicPrefix:       "portask_test",
    ConsumerGroup:     "portask_group",
    PartitionCount:    3,
    ReplicationFactor: 1,
}

// Create bridge with preferred client
bridge, err := NewKafkaPortaskBridge(saramaConfig, "localhost:8080", "test-topic")
if err != nil {
    log.Fatal(err)
}
defer bridge.Stop()

// Start consuming
err = bridge.ConsumeFromKafka([]string{"topic1", "topic2"})
if err != nil {
    log.Fatal(err)
}

// Publish messages
err = bridge.PublishToKafka("test-topic", []byte("Hello Portask!"))
if err != nil {
    log.Fatal(err)
}
```

## Advanced Features

### Topic Management

#### Automatic Topic Creation
```go
// Sarama - Admin client for topic management
config := sarama.NewConfig()
config.Version = sarama.V2_6_0_0

admin, err := sarama.NewClusterAdmin([]string{"localhost:9092"}, config)
if err != nil {
    log.Fatal(err)
}
defer admin.Close()

// Create topic
topicDetail := &sarama.TopicDetail{
    NumPartitions:     3,
    ReplicationFactor: 1,
    ConfigEntries: map[string]*string{
        "cleanup.policy": stringPtr("delete"),
        "retention.ms":   stringPtr("86400000"), // 24 hours
    },
}

err = admin.CreateTopic("my-topic", topicDetail, false)
if err != nil {
    log.Printf("Error creating topic: %v", err)
}
```

#### Topic Configuration
```go
// kafka-go - Connection for admin operations
conn, err := kafka.Dial("tcp", "localhost:9092")
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

// Create topic
topicConfigs := []kafka.TopicConfig{
    {
        Topic:             "my-topic",
        NumPartitions:     3,
        ReplicationFactor: 1,
        ConfigEntries: []kafka.ConfigEntry{
            {ConfigName: "retention.ms", ConfigValue: "86400000"},
            {ConfigName: "cleanup.policy", ConfigValue: "delete"},
        },
    },
}

err = conn.CreateTopics(topicConfigs...)
if err != nil {
    log.Printf("Error creating topic: %v", err)
}
```

### Partition Management

#### Custom Partitioner (Sarama)
```go
type CustomPartitioner struct {
    random sarama.Partitioner
}

func (p *CustomPartitioner) Partition(message *sarama.ProducerMessage, numPartitions int32) (int32, error) {
    // Custom partitioning logic
    if key := message.Key; key != nil {
        keyBytes, _ := key.Encode()
        // Hash-based partitioning
        hash := fnv.New32a()
        hash.Write(keyBytes)
        return int32(hash.Sum32()) % numPartitions, nil
    }
    
    // Fall back to random partitioning
    return p.random.Partition(message, numPartitions)
}

func (p *CustomPartitioner) RequiresConsistency() bool {
    return true
}

// Use custom partitioner
config := sarama.NewConfig()
config.Producer.Partitioner = func(topic string) sarama.Partitioner {
    return &CustomPartitioner{
        random: sarama.NewRandomPartitioner(topic),
    }
}
```

#### Balancer (kafka-go)
```go
// Custom balancer for kafka-go
type CustomBalancer struct{}

func (b *CustomBalancer) Balance(msg kafka.Message, partitions ...int) int {
    // Custom balancing logic
    if len(msg.Key) > 0 {
        hash := fnv.New32a()
        hash.Write(msg.Key)
        return int(hash.Sum32()) % len(partitions)
    }
    
    // Random distribution
    return rand.Intn(len(partitions))
}

// Use custom balancer
writer := &kafka.Writer{
    Addr:     kafka.TCP("localhost:9092"),
    Topic:    "test-topic",
    Balancer: &CustomBalancer{},
}
```

### Consumer Groups

#### Rebalancing Strategies
```go
// Sarama - Different rebalancing strategies
config := sarama.NewConfig()
config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategySticky
// Other options: BalanceStrategyRange, BalanceStrategyRoundRobin

// Custom rebalance strategy
type CustomStrategy struct{}

func (s *CustomStrategy) Name() string {
    return "custom"
}

func (s *CustomStrategy) Plan(members map[string]sarama.ConsumerGroupMemberMetadata, topics map[string][]int32) (sarama.BalanceStrategyPlan, error) {
    // Custom rebalancing logic
    plan := make(sarama.BalanceStrategyPlan)
    // ... implement custom logic
    return plan, nil
}

func (s *CustomStrategy) AssignmentData(memberID string, topics map[string][]int32, generationID int32) ([]byte, error) {
    return nil, nil
}

config.Consumer.Group.Rebalance.Strategy = &CustomStrategy{}
```

#### Session Management
```go
// Handle consumer group sessions
func (consumer *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
    // Session information
    log.Printf("Session ID: %s", session.Context().Value("sessionID"))
    log.Printf("Member ID: %s", session.MemberID())
    log.Printf("Generation ID: %d", session.GenerationID())
    log.Printf("Claims: %v", session.Claims())
    
    for {
        select {
        case message := <-claim.Messages():
            if message == nil {
                return nil
            }
            
            // Process message
            err := processMessage(message)
            if err != nil {
                log.Printf("Error processing message: %v", err)
                continue
            }
            
            // Manual offset management
            session.MarkOffset(message.Topic, message.Partition, message.Offset+1, "")
            
            // Commit offsets periodically
            if message.Offset%100 == 0 {
                session.Commit()
            }
            
        case <-session.Context().Done():
            return nil
        }
    }
}
```

### Error Handling and Resilience

#### Producer Error Handling
```go
// Sarama - Async producer with error handling
config := sarama.NewConfig()
config.Producer.Return.Successes = true
config.Producer.Return.Errors = true
config.Producer.Retry.Max = 3
config.Producer.Retry.Backoff = 100 * time.Millisecond

producer, err := sarama.NewAsyncProducer([]string{"localhost:9092"}, config)
if err != nil {
    log.Fatal(err)
}
defer producer.Close()

// Handle successes and errors
go func() {
    for {
        select {
        case success := <-producer.Successes():
            log.Printf("Message sent: partition=%d, offset=%d", success.Partition, success.Offset)
            
        case err := <-producer.Errors():
            log.Printf("Failed to send message: %v", err.Err)
            
            // Implement retry logic
            if err.Err == sarama.ErrRequestTimedOut {
                // Retry with exponential backoff
                time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)
                producer.Input() <- err.Msg
            }
        }
    }
}()

// Send messages
producer.Input() <- &sarama.ProducerMessage{
    Topic: "test-topic",
    Value: sarama.StringEncoder("test message"),
}
```

#### Consumer Error Handling
```go
// kafka-go - Consumer with retry logic
func createReaderWithRetry(config kafka.ReaderConfig, maxRetries int) *kafka.Reader {
    var reader *kafka.Reader
    
    for attempt := 0; attempt < maxRetries; attempt++ {
        reader = kafka.NewReader(config)
        
        // Test connection
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        _, err := reader.ReadMessage(ctx)
        cancel()
        
        if err == nil || attempt == maxRetries-1 {
            break
        }
        
        reader.Close()
        log.Printf("Connection attempt %d failed, retrying...", attempt+1)
        time.Sleep(time.Duration(1<<attempt) * time.Second) // Exponential backoff
    }
    
    return reader
}

// Consumer with circuit breaker
type CircuitBreaker struct {
    failureCount    int
    maxFailures     int
    resetTimeout    time.Duration
    lastFailureTime time.Time
    state           string // "closed", "open", "half-open"
    mutex           sync.Mutex
}

func (cb *CircuitBreaker) Call(fn func() error) error {
    cb.mutex.Lock()
    defer cb.mutex.Unlock()
    
    if cb.state == "open" {
        if time.Since(cb.lastFailureTime) > cb.resetTimeout {
            cb.state = "half-open"
        } else {
            return fmt.Errorf("circuit breaker is open")
        }
    }
    
    err := fn()
    if err != nil {
        cb.failureCount++
        cb.lastFailureTime = time.Now()
        
        if cb.failureCount >= cb.maxFailures {
            cb.state = "open"
        }
        
        return err
    }
    
    // Success - reset circuit breaker
    cb.failureCount = 0
    cb.state = "closed"
    return nil
}
```

## Performance Optimization

### Producer Optimization

#### Batching and Compression
```go
// Sarama - Optimized producer configuration
config := sarama.NewConfig()
config.Producer.Flush.Frequency = 100 * time.Millisecond
config.Producer.Flush.Messages = 100
config.Producer.Flush.Bytes = 1024 * 1024 // 1MB
config.Producer.Compression = sarama.CompressionLZ4
config.Producer.CompressionLevel = 6
config.Net.MaxOpenRequests = 5
config.Producer.Idempotent = true

// kafka-go - Optimized writer configuration
writer := &kafka.Writer{
    Addr:         kafka.TCP("localhost:9092"),
    Topic:        "test-topic",
    Balancer:     &kafka.Hash{},
    BatchSize:    1000,
    BatchBytes:   1048576, // 1MB
    BatchTimeout: 10 * time.Millisecond,
    Compression:  kafka.Lz4,
    RequiredAcks: kafka.RequireOne,
}
```

#### Connection Pooling
```go
// Sarama - Connection pool
type SaramaPool struct {
    producers []sarama.AsyncProducer
    current   int64
    mutex     sync.RWMutex
}

func NewSaramaPool(size int, config *sarama.Config, brokers []string) *SaramaPool {
    pool := &SaramaPool{
        producers: make([]sarama.AsyncProducer, size),
    }
    
    for i := 0; i < size; i++ {
        producer, _ := sarama.NewAsyncProducer(brokers, config)
        pool.producers[i] = producer
    }
    
    return pool
}

func (p *SaramaPool) GetProducer() sarama.AsyncProducer {
    index := atomic.AddInt64(&p.current, 1) % int64(len(p.producers))
    return p.producers[index]
}

// kafka-go - Writer pool
type WriterPool struct {
    writers []*kafka.Writer
    current int64
}

func NewWriterPool(size int, config kafka.WriterConfig) *WriterPool {
    pool := &WriterPool{
        writers: make([]*kafka.Writer, size),
    }
    
    for i := 0; i < size; i++ {
        pool.writers[i] = &kafka.Writer{
            Addr:         config.Addr,
            Topic:        config.Topic,
            Balancer:     config.Balancer,
            BatchSize:    config.BatchSize,
            BatchBytes:   config.BatchBytes,
            BatchTimeout: config.BatchTimeout,
        }
    }
    
    return pool
}

func (p *WriterPool) GetWriter() *kafka.Writer {
    index := atomic.AddInt64(&p.current, 1) % int64(len(p.writers))
    return p.writers[index]
}
```

### Consumer Optimization

#### Parallel Processing
```go
// Sarama - Parallel message processing
func (consumer *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
    // Worker pool for message processing
    workerCount := 10
    messageChan := make(chan *sarama.ConsumerMessage, 100)
    
    // Start workers
    var wg sync.WaitGroup
    for i := 0; i < workerCount; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for msg := range messageChan {
                err := processMessage(msg)
                if err != nil {
                    log.Printf("Error processing message: %v", err)
                    continue
                }
                session.MarkMessage(msg, "")
            }
        }()
    }
    
    // Feed messages to workers
    for {
        select {
        case message := <-claim.Messages():
            if message == nil {
                close(messageChan)
                wg.Wait()
                return nil
            }
            messageChan <- message
            
        case <-session.Context().Done():
            close(messageChan)
            wg.Wait()
            return nil
        }
    }
}

// kafka-go - Parallel consumer
func parallelConsumer(reader *kafka.Reader, workerCount int) {
    messageChan := make(chan kafka.Message, 100)
    
    // Start workers
    var wg sync.WaitGroup
    for i := 0; i < workerCount; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            for msg := range messageChan {
                err := processKafkaMessage(msg)
                if err != nil {
                    log.Printf("Worker %d error: %v", workerID, err)
                }
            }
        }(i)
    }
    
    // Read messages
    go func() {
        defer close(messageChan)
        for {
            msg, err := reader.ReadMessage(context.Background())
            if err != nil {
                log.Printf("Error reading message: %v", err)
                return
            }
            messageChan <- msg
        }
    }()
    
    wg.Wait()
}
```

## Monitoring and Metrics

### Built-in Metrics

#### Sarama Metrics
```go
// Enable metrics in Sarama
config := sarama.NewConfig()
config.MetricRegistry = metrics.NewRegistry()

producer, err := sarama.NewAsyncProducer([]string{"localhost:9092"}, config)
if err != nil {
    log.Fatal(err)
}

// Access metrics
registry := config.MetricRegistry
go func() {
    for {
        time.Sleep(10 * time.Second)
        
        registry.Each(func(name string, i interface{}) {
            switch metric := i.(type) {
            case metrics.Counter:
                log.Printf("Counter %s: %d", name, metric.Count())
            case metrics.Gauge:
                log.Printf("Gauge %s: %d", name, metric.Value())
            case metrics.Histogram:
                log.Printf("Histogram %s: count=%d, mean=%.2f", 
                    name, metric.Count(), metric.Mean())
            }
        })
    }
}()
```

#### Custom Metrics Collection
```go
// Custom metrics for Portask bridge
type BridgeMetrics struct {
    MessagesProduced  *prometheus.CounterVec
    MessagesConsumed  *prometheus.CounterVec
    ProduceLatency    *prometheus.HistogramVec
    ConsumeLatency    *prometheus.HistogramVec
    ActiveConnections prometheus.Gauge
}

func NewBridgeMetrics() *BridgeMetrics {
    return &BridgeMetrics{
        MessagesProduced: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "portask_kafka_messages_produced_total",
                Help: "Total number of messages produced to Kafka",
            },
            []string{"topic", "client_type"},
        ),
        MessagesConsumed: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "portask_kafka_messages_consumed_total", 
                Help: "Total number of messages consumed from Kafka",
            },
            []string{"topic", "client_type"},
        ),
        ProduceLatency: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "portask_kafka_produce_duration_seconds",
                Help:    "Time spent producing messages to Kafka",
                Buckets: prometheus.DefBuckets,
            },
            []string{"topic", "client_type"},
        ),
        ConsumeLatency: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "portask_kafka_consume_duration_seconds",
                Help:    "Time spent consuming messages from Kafka",
                Buckets: prometheus.DefBuckets,
            },
            []string{"topic", "client_type"},
        ),
        ActiveConnections: prometheus.NewGauge(
            prometheus.GaugeOpts{
                Name: "portask_kafka_active_connections",
                Help: "Number of active Kafka connections",
            },
        ),
    }
}

// Use metrics in bridge
func (b *KafkaPortaskBridge) PublishToKafka(topic string, message []byte) error {
    start := time.Now()
    defer func() {
        b.metrics.ProduceLatency.WithLabelValues(topic, string(b.clientType)).Observe(time.Since(start).Seconds())
    }()
    
    var err error
    switch b.clientType {
    case KafkaClientSarama:
        err = b.saramaBridge.PublishToKafka(topic, message)
    case KafkaClientKafkaGo:
        err = b.kafkaGoBridge.PublishToKafka(topic, message)
    }
    
    if err == nil {
        b.metrics.MessagesProduced.WithLabelValues(topic, string(b.clientType)).Inc()
    }
    
    return err
}
```

## Configuration Reference

### Server Configuration
```json
{
  "kafka": {
    "port": 9092,
    "advertised_listeners": ["localhost:9092"],
    "num_network_threads": 8,
    "num_io_threads": 16,
    "socket_send_buffer_bytes": 102400,
    "socket_receive_buffer_bytes": 102400,
    "socket_request_max_bytes": 104857600,
    "num_partitions": 3,
    "default_replication_factor": 1,
    "min_insync_replicas": 1,
    "unclean_leader_election_enable": false,
    "auto_create_topics_enable": true,
    "delete_topic_enable": true,
    "compression_type": "snappy",
    "log_retention_hours": 168,
    "log_retention_bytes": 1073741824,
    "log_segment_bytes": 1073741824,
    "log_cleanup_policy": "delete",
    "message_max_bytes": 1000000
  }
}
```

### Client-Specific Configuration

#### Sarama Configuration
```json
{
  "sarama": {
    "version": "2.6.0",
    "client_id": "portask-bridge",
    "channel_buffer_size": 256,
    "producer": {
      "max_message_bytes": 1000000,
      "required_acks": "WaitForAll",
      "timeout": "10s",
      "compression": "snappy",
      "compression_level": 6,
      "flush_frequency": "100ms",
      "flush_messages": 100,
      "flush_bytes": 1048576,
      "retry_max": 3,
      "retry_backoff": "100ms",
      "return_successes": true,
      "return_errors": true,
      "idempotent": true
    },
    "consumer": {
      "fetch_min": 1,
      "fetch_default": 1048576,
      "fetch_max": 10485760,
      "max_wait_time": "250ms",
      "max_processing_time": "1s",
      "return_errors": true,
      "offsets_commit_interval": "1s",
      "group": {
        "session_timeout": "10s",
        "heartbeat_interval": "3s",
        "rebalance_strategy": "sticky",
        "rebalance_timeout": "60s",
        "rebalance_retry_max": 4,
        "rebalance_retry_backoff": "2s"
      }
    },
    "net": {
      "max_open_requests": 5,
      "dial_timeout": "30s",
      "read_timeout": "30s",
      "write_timeout": "30s",
      "keepalive": "30s"
    }
  }
}
```

#### kafka-go Configuration
```json
{
  "kafka_go": {
    "writer": {
      "batch_size": 100,
      "batch_bytes": 1048576,
      "batch_timeout": "10ms",
      "read_timeout": "10s",
      "write_timeout": "10s",
      "required_acks": "RequireAll",
      "compression": "snappy",
      "balancer": "LeastBytes",
      "max_attempts": 3,
      "write_backoff_min": "100ms",
      "write_backoff_max": "1s"
    },
    "reader": {
      "min_bytes": 10240,
      "max_bytes": 10485760,
      "max_wait": "1s",
      "read_batch_timeout": "0s",
      "read_lag_interval": "0s",
      "group_balancers": ["RoundRobin", "Range"],
      "start_offset": "FirstOffset",
      "commit_interval": "1s",
      "partition_watch_interval": "5s",
      "watch_partition_changes": true,
      "read_backoff_min": "100ms",
      "read_backoff_max": "1s"
    }
  }
}
```

## Testing with Portask

### Unit Testing
```go
func TestKafkaBridgeClientSelection(t *testing.T) {
    testCases := []struct {
        name       string
        clientType KafkaClientType
        expectErr  bool
    }{
        {"Sarama Client", KafkaClientSarama, false},
        {"kafka-go Client", KafkaClientKafkaGo, false},
        {"Invalid Client", "invalid", true},
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            config := DefaultKafkaConfig(tc.clientType)
            bridge, err := NewKafkaPortaskBridge(config, "localhost:8080", "test-topic")
            
            if tc.expectErr {
                assert.Error(t, err)
                assert.Nil(t, bridge)
            } else {
                assert.NoError(t, err)
                assert.NotNil(t, bridge)
                if bridge != nil {
                    bridge.Stop()
                }
            }
        })
    }
}
```

### Integration Testing
```go
func TestKafkaClientCompatibility(t *testing.T) {
    // Start Portask server
    server := startPortaskServer(t)
    defer server.Stop()
    
    clientTypes := []KafkaClientType{KafkaClientSarama, KafkaClientKafkaGo}
    
    for _, clientType := range clientTypes {
        t.Run(string(clientType), func(t *testing.T) {
            config := DefaultKafkaConfig(clientType)
            bridge, err := NewKafkaPortaskBridge(config, server.Addr(), "test-topic")
            require.NoError(t, err)
            defer bridge.Stop()
            
            // Test produce and consume
            testMessage := []byte("test message for " + string(clientType))
            
            err = bridge.PublishToKafka("test-topic", testMessage)
            assert.NoError(t, err)
            
            // Start consumer
            err = bridge.ConsumeFromKafka([]string{"test-topic"})
            assert.NoError(t, err)
            
            // Wait for message processing
            time.Sleep(2 * time.Second)
            
            // Verify statistics
            stats := bridge.GetStats()
            assert.Greater(t, stats["messageCount"], int64(0))
        })
    }
}
```

### Performance Testing
```go
func BenchmarkKafkaClients(b *testing.B) {
    server := startPortaskServer(b)
    defer server.Stop()
    
    clientTypes := []KafkaClientType{KafkaClientSarama, KafkaClientKafkaGo}
    
    for _, clientType := range clientTypes {
        b.Run(string(clientType), func(b *testing.B) {
            config := DefaultKafkaConfig(clientType)
            bridge, err := NewKafkaPortaskBridge(config, server.Addr(), "benchmark-topic")
            require.NoError(b, err)
            defer bridge.Stop()
            
            message := make([]byte, 1024) // 1KB message
            
            b.ResetTimer()
            b.RunParallel(func(pb *testing.PB) {
                for pb.Next() {
                    err := bridge.PublishToKafka("benchmark-topic", message)
                    if err != nil {
                        b.Fatal(err)
                    }
                }
            })
        })
    }
}
```

## Best Practices

### Client Selection Guidelines

#### Choose Sarama When:
- You need extensive configuration options
- You're building enterprise-grade applications
- You need all Kafka features and APIs
- You have complex consumer group requirements
- You need fine-grained control over connections

#### Choose kafka-go When:
- You prefer simple, idiomatic Go APIs
- You want minimal dependencies
- You're building simple producer/consumer applications
- You value performance and low allocations
- You prefer context-based APIs

### Production Recommendations

#### Connection Management
```go
// Good: Reuse connections
var (
    kafkaWriter *kafka.Writer
    once        sync.Once
)

func getWriter() *kafka.Writer {
    once.Do(func() {
        kafkaWriter = &kafka.Writer{
            Addr:         kafka.TCP("localhost:9092"),
            Topic:        "production-topic",
            Balancer:     &kafka.Hash{},
            BatchSize:    1000,
            BatchTimeout: 10 * time.Millisecond,
        }
    })
    return kafkaWriter
}

// Bad: Creating new connections for each operation
func publishMessage(msg []byte) error {
    writer := &kafka.Writer{...} // Don't do this!
    defer writer.Close()
    return writer.WriteMessages(context.Background(), kafka.Message{Value: msg})
}
```

#### Error Handling
```go
// Implement exponential backoff for retries
func publishWithRetry(writer *kafka.Writer, msg kafka.Message, maxRetries int) error {
    var err error
    for attempt := 0; attempt < maxRetries; attempt++ {
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        err = writer.WriteMessages(ctx, msg)
        cancel()
        
        if err == nil {
            return nil
        }
        
        // Exponential backoff
        backoff := time.Duration(1<<attempt) * 100 * time.Millisecond
        time.Sleep(backoff)
    }
    return err
}
```

#### Resource Management
```go
// Proper cleanup
func gracefulShutdown(writer *kafka.Writer, reader *kafka.Reader) {
    // Close writer
    if err := writer.Close(); err != nil {
        log.Printf("Error closing writer: %v", err)
    }
    
    // Close reader
    if err := reader.Close(); err != nil {
        log.Printf("Error closing reader: %v", err)
    }
}

// Use context for cancellation
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// Handle shutdown signals
c := make(chan os.Signal, 1)
signal.Notify(c, os.Interrupt, syscall.SIGTERM)

go func() {
    <-c
    log.Println("Shutting down gracefully...")
    cancel()
}()
```

## Troubleshooting

### Common Issues

#### Connection Problems
```go
// Test connectivity
func testKafkaConnection(brokers []string) error {
    // For kafka-go
    conn, err := kafka.Dial("tcp", brokers[0])
    if err != nil {
        return fmt.Errorf("failed to connect: %v", err)
    }
    defer conn.Close()
    
    // Test basic operation
    partitions, err := conn.ReadPartitions()
    if err != nil {
        return fmt.Errorf("failed to read partitions: %v", err)
    }
    
    log.Printf("Found %d partitions", len(partitions))
    return nil
}

// For Sarama
func testSaramaConnection(brokers []string) error {
    config := sarama.NewConfig()
    config.Net.DialTimeout = 10 * time.Second
    
    client, err := sarama.NewClient(brokers, config)
    if err != nil {
        return fmt.Errorf("failed to create client: %v", err)
    }
    defer client.Close()
    
    topics, err := client.Topics()
    if err != nil {
        return fmt.Errorf("failed to get topics: %v", err)
    }
    
    log.Printf("Found topics: %v", topics)
    return nil
}
```

#### Performance Issues
```go
// Monitor message processing rate
type RateMonitor struct {
    count     int64
    startTime time.Time
    mutex     sync.RWMutex
}

func NewRateMonitor() *RateMonitor {
    return &RateMonitor{startTime: time.Now()}
}

func (r *RateMonitor) Increment() {
    atomic.AddInt64(&r.count, 1)
}

func (r *RateMonitor) GetRate() float64 {
    r.mutex.RLock()
    defer r.mutex.RUnlock()
    
    count := atomic.LoadInt64(&r.count)
    elapsed := time.Since(r.startTime).Seconds()
    
    if elapsed == 0 {
        return 0
    }
    
    return float64(count) / elapsed
}

// Use in consumer
monitor := NewRateMonitor()

for msg := range messages {
    // Process message
    processMessage(msg)
    monitor.Increment()
    
    // Log rate every 1000 messages
    if monitor.count%1000 == 0 {
        log.Printf("Processing rate: %.2f msg/sec", monitor.GetRate())
    }
}
```

#### Memory Leaks
```go
// Monitor goroutine count
func monitorGoroutines() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        count := runtime.NumGoroutine()
        log.Printf("Active goroutines: %d", count)
        
        if count > 1000 { // Threshold
            log.Printf("High goroutine count detected!")
            // Take action: dump stack, restart, etc.
        }
    }
}

// Monitor memory usage
func monitorMemory() {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    log.Printf("Memory usage: Alloc=%d KB, TotalAlloc=%d KB, Sys=%d KB, NumGC=%d",
        bToKb(m.Alloc), bToKb(m.TotalAlloc), bToKb(m.Sys), m.NumGC)
}

func bToKb(b uint64) uint64 {
    return b / 1024
}
```

---

For more information, see:
- [AMQP Emulator Documentation](./amqp_emulator.md)
- [API Reference](./api_reference.md)
- [Performance Tuning Guide](./performance.md)
