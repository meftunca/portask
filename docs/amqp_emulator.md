# AMQP (RabbitMQ) Emulator Documentation

## Overview

The Portask AMQP emulator provides complete wire-protocol compatibility with RabbitMQ, implementing the AMQP 0-9-1 specification. This means any existing RabbitMQ client can connect to Portask without modification, making it an ideal drop-in replacement for development, testing, and lightweight production scenarios.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                RabbitMQ Client Libraries                │
├─────────────────────────────────────────────────────────┤
│                   AMQP 0-9-1 Protocol                   │
├─────────────────────────────────────────────────────────┤
│                  Portask AMQP Server                    │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────────┐   │
│  │ Connection  │ │   Channel   │ │  Frame Parser   │   │
│  │  Manager    │ │   Handler   │ │   & Builder     │   │
│  └─────────────┘ └─────────────┘ └─────────────────┘   │
├─────────────────────────────────────────────────────────┤
│                 Message Engine Core                     │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────────┐   │
│  │ Exchanges   │ │   Queues    │ │    Bindings     │   │
│  │  (Topic,    │ │ (Durable,   │ │  (Routing Key   │   │
│  │  Direct,    │ │  Temporary, │ │   Patterns)     │   │
│  │  Fanout)    │ │  Exclusive) │ │                 │   │
│  └─────────────┘ └─────────────┘ └─────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

## Protocol Support

### Implemented AMQP Methods

#### Connection Methods
- `Connection.Start` / `Connection.StartOk`
- `Connection.Tune` / `Connection.TuneOk` 
- `Connection.Open` / `Connection.OpenOk`
- `Connection.Close` / `Connection.CloseOk`

#### Channel Methods
- `Channel.Open` / `Channel.OpenOk`
- `Channel.Close` / `Channel.CloseOk`
- `Channel.Flow` / `Channel.FlowOk`

#### Exchange Methods
- `Exchange.Declare` / `Exchange.DeclareOk`
- `Exchange.Delete` / `Exchange.DeleteOk`
- `Exchange.Bind` / `Exchange.BindOk` 
- `Exchange.Unbind` / `Exchange.UnbindOk`

#### Queue Methods
- `Queue.Declare` / `Queue.DeclareOk`
- `Queue.Bind` / `Queue.BindOk`
- `Queue.Unbind` / `Queue.UnbindOk`
- `Queue.Purge` / `Queue.PurgeOk`
- `Queue.Delete` / `Queue.DeleteOk`

#### Basic Methods
- `Basic.Qos` / `Basic.QosOk`
- `Basic.Consume` / `Basic.ConsumeOk`
- `Basic.Cancel` / `Basic.CancelOk`
- `Basic.Publish`
- `Basic.Return`
- `Basic.Deliver`
- `Basic.Get` / `Basic.GetOk` / `Basic.GetEmpty`
- `Basic.Ack`
- `Basic.Nack`
- `Basic.Reject`
- `Basic.Recover` / `Basic.RecoverOk`

#### Transaction Methods
- `Tx.Select` / `Tx.SelectOk`
- `Tx.Commit` / `Tx.CommitOk`
- `Tx.Rollback` / `Tx.RollbackOk`

### Exchange Types

#### Direct Exchange
Routes messages with exact routing key matches.

```go
// Example: Declare direct exchange
err = channel.ExchangeDeclare(
    "orders.direct",    // name
    "direct",          // type
    true,              // durable
    false,             // auto-delete
    false,             // internal
    false,             // no-wait
    nil,               // arguments
)

// Bind queue with exact routing key
err = channel.QueueBind(
    "order.processing", // queue name
    "order.new",       // routing key
    "orders.direct",   // exchange
    false,             // no-wait
    nil,               // arguments
)
```

#### Topic Exchange
Routes messages using pattern matching with routing keys.

```go
// Example: Topic exchange with wildcards
err = channel.ExchangeDeclare(
    "logs.topic",      // name
    "topic",           // type
    true,              // durable
    false,             // auto-delete
    false,             // internal
    false,             // no-wait
    nil,               // arguments
)

// Bind with wildcard patterns
// * matches exactly one word
// # matches zero or more words
err = channel.QueueBind(
    "error.logs",      // queue name
    "log.error.*",     // routing key pattern
    "logs.topic",      // exchange
    false,             // no-wait
    nil,               // arguments
)
```

#### Fanout Exchange
Routes messages to all bound queues regardless of routing key.

```go
// Example: Fanout for broadcasting
err = channel.ExchangeDeclare(
    "notifications.fanout", // name
    "fanout",              // type
    true,                  // durable
    false,                 // auto-delete
    false,                 // internal
    false,                 // no-wait
    nil,                   // arguments
)

// All bound queues receive all messages
err = channel.QueueBind(
    "email.notifications",    // queue name
    "",                      // routing key (ignored)
    "notifications.fanout",  // exchange
    false,                   // no-wait
    nil,                     // arguments
)
```

#### Headers Exchange
Routes messages based on header attributes instead of routing keys.

```go
// Example: Headers exchange
err = channel.ExchangeDeclare(
    "tasks.headers",   // name
    "headers",         // type
    true,              // durable
    false,             // auto-delete
    false,             // internal
    false,             // no-wait
    nil,               // arguments
)

// Bind with header matching
headers := amqp.Table{
    "x-match": "all",        // "all" or "any"
    "priority": "high",
    "type":     "urgent",
}
err = channel.QueueBind(
    "urgent.tasks",    // queue name
    "",                // routing key (ignored)
    "tasks.headers",   // exchange
    false,             // no-wait
    headers,           // arguments
)
```

### Queue Features

#### Queue Declaration Options
```go
queue, err := channel.QueueDeclare(
    "my.queue",        // name (empty for auto-generated)
    true,              // durable (survives server restart)
    false,             // delete when unused
    false,             // exclusive (only this connection)
    false,             // no-wait
    amqp.Table{        // arguments
        "x-message-ttl":             60000,     // message TTL in ms
        "x-expires":                 1800000,   // queue TTL in ms
        "x-max-length":              1000,      // max queue length
        "x-max-length-bytes":        1048576,   // max queue size in bytes
        "x-dead-letter-exchange":    "dlx",     // dead letter exchange
        "x-dead-letter-routing-key": "failed",  // dead letter routing key
        "x-max-priority":            10,        // priority queue (0-255)
    },
)
```

#### Queue Binding
```go
// Simple binding
err = channel.QueueBind(
    queue.Name,        // queue name
    "routing.key",     // routing key
    "my.exchange",     // exchange name
    false,             // no-wait
    nil,               // arguments
)

// Binding with arguments
err = channel.QueueBind(
    queue.Name,        // queue name
    "routing.key",     // routing key  
    "my.exchange",     // exchange name
    false,             // no-wait
    amqp.Table{        // arguments
        "x-match": "any",
        "format":  "json",
    },
)
```

### Message Publishing

#### Basic Publishing
```go
err = channel.Publish(
    "my.exchange",     // exchange
    "routing.key",     // routing key
    false,             // mandatory
    false,             // immediate
    amqp.Publishing{
        Headers:         amqp.Table{
            "x-custom-header": "value",
        },
        ContentType:     "application/json",
        ContentEncoding: "utf-8",
        DeliveryMode:    amqp.Persistent, // or amqp.Transient
        Priority:        5,               // 0-9
        CorrelationId:   "request-123",
        ReplyTo:         "response.queue",
        Expiration:      "60000",         // TTL in ms
        MessageId:       "msg-456",
        Timestamp:       time.Now(),
        Type:            "order.created",
        UserId:          "user123",
        AppId:           "order-service",
        Body:            []byte(`{"order_id": 123, "amount": 99.99}`),
    },
)
```

#### Publisher Confirms
```go
// Enable publisher confirms
err = channel.Confirm(false)
if err != nil {
    log.Fatal(err)
}

confirmations := channel.NotifyPublish(make(chan amqp.Confirmation, 1))

// Publish message
err = channel.Publish(/* ... */)

// Wait for confirmation
confirmed := <-confirmations
if confirmed.Ack {
    log.Println("Message confirmed")
} else {
    log.Println("Message nacked")
}
```

#### Mandatory and Immediate
```go
// Mandatory: return message if no queue is bound
// Immediate: return message if no consumer can immediately process it
returns := channel.NotifyReturn(make(chan amqp.Return, 1))

err = channel.Publish(
    "my.exchange",     // exchange
    "routing.key",     // routing key
    true,              // mandatory
    false,             // immediate (deprecated but supported)
    publishing,
)

// Handle returned messages
select {
case returned := <-returns:
    log.Printf("Message returned: %s", returned.ReplyText)
case <-time.After(time.Second):
    log.Println("Message delivered successfully")
}
```

### Message Consumption

#### Basic Consumer
```go
messages, err := channel.Consume(
    queue.Name,        // queue
    "consumer-tag",    // consumer tag
    false,             // auto-ack
    false,             // exclusive
    false,             // no-local
    false,             // no-wait
    nil,               // args
)
if err != nil {
    log.Fatal(err)
}

// Process messages
for msg := range messages {
    log.Printf("Received: %s", msg.Body)
    
    // Manual acknowledgment
    err = msg.Ack(false) // false = single message
    if err != nil {
        log.Printf("Failed to ack: %v", err)
    }
}
```

#### QoS (Quality of Service)
```go
// Set prefetch count to limit unacknowledged messages
err = channel.Qos(
    10,    // prefetch count
    0,     // prefetch size (0 = no limit)
    false, // global (false = per-consumer)
)
```

#### Message Properties Access
```go
for msg := range messages {
    log.Printf("Headers: %v", msg.Headers)
    log.Printf("Content-Type: %s", msg.ContentType)
    log.Printf("Delivery Tag: %d", msg.DeliveryTag)
    log.Printf("Redelivered: %t", msg.Redelivered)
    log.Printf("Exchange: %s", msg.Exchange)
    log.Printf("Routing Key: %s", msg.RoutingKey)
    log.Printf("Message ID: %s", msg.MessageId)
    log.Printf("Timestamp: %v", msg.Timestamp)
    log.Printf("Body: %s", msg.Body)
    
    // Process message
    err := processMessage(msg.Body)
    if err != nil {
        // Negative acknowledgment with requeue
        msg.Nack(false, true)
    } else {
        // Positive acknowledgment
        msg.Ack(false)
    }
}
```

### Advanced Features

#### Dead Letter Exchanges
```go
// Declare main queue with dead letter exchange
_, err = channel.QueueDeclare(
    "main.queue",
    true,  // durable
    false, // delete when unused
    false, // exclusive
    false, // no-wait
    amqp.Table{
        "x-dead-letter-exchange":    "dlx",
        "x-dead-letter-routing-key": "failed",
        "x-message-ttl":             30000, // 30 seconds
    },
)

// Declare dead letter exchange and queue
err = channel.ExchangeDeclare("dlx", "direct", true, false, false, false, nil)
_, err = channel.QueueDeclare("failed.queue", true, false, false, false, nil)
err = channel.QueueBind("failed.queue", "failed", "dlx", false, nil)
```

#### Priority Queues
```go
// Declare priority queue
_, err = channel.QueueDeclare(
    "priority.queue",
    true,  // durable
    false, // delete when unused  
    false, // exclusive
    false, // no-wait
    amqp.Table{
        "x-max-priority": 10, // priorities 0-10
    },
)

// Publish with priority
err = channel.Publish(
    "",               // exchange
    "priority.queue", // routing key
    false,            // mandatory
    false,            // immediate
    amqp.Publishing{
        Priority: 8,  // high priority
        Body:     []byte("High priority message"),
    },
)
```

#### Transactions
```go
// Start transaction
err = channel.Tx()
if err != nil {
    log.Fatal(err)
}

// Publish messages in transaction
err = channel.Publish(/* ... */)
err = channel.Publish(/* ... */)

// Commit or rollback
if allSuccessful {
    err = channel.TxCommit()
} else {
    err = channel.TxRollback()
}
```

## Configuration

### AMQP Server Configuration
```json
{
  "amqp": {
    "port": 5672,
    "max_frame_size": 131072,
    "heartbeat_interval": 60,
    "max_channels": 2048,
    "max_connections": 1000,
    "default_user": "guest",
    "default_password": "guest",
    "default_vhost": "/",
    "channel_max": 65535,
    "frame_max": 131072,
    "product": "Portask",
    "version": "1.0.0",
    "platform": "Go"
  }
}
```

### Connection Parameters
```go
// Custom connection parameters
config := amqp.Config{
    Heartbeat: 10 * time.Second,
    Locale:    "en_US",
    Properties: amqp.Table{
        "connection_name": "my-app",
        "client_info":     "my-client v1.0",
    },
}

conn, err := amqp.DialConfig("amqp://localhost:5672/", config)
```

## Performance Optimization

### Connection Pooling
```go
type ConnectionPool struct {
    connections []*amqp.Connection
    mutex       sync.Mutex
    size        int
}

func NewConnectionPool(size int, url string) (*ConnectionPool, error) {
    pool := &ConnectionPool{
        connections: make([]*amqp.Connection, 0, size),
        size:        size,
    }
    
    for i := 0; i < size; i++ {
        conn, err := amqp.Dial(url)
        if err != nil {
            return nil, err
        }
        pool.connections = append(pool.connections, conn)
    }
    
    return pool, nil
}

func (p *ConnectionPool) GetConnection() *amqp.Connection {
    p.mutex.Lock()
    defer p.mutex.Unlock()
    
    if len(p.connections) > 0 {
        conn := p.connections[0]
        p.connections = p.connections[1:]
        return conn
    }
    return nil
}
```

### Channel Management
```go
// Use separate channels for publishing and consuming
publishChannel, err := conn.Channel()
consumeChannel, err := conn.Channel()

// Set appropriate QoS for consumers
err = consumeChannel.Qos(10, 0, false)

// Use channel pools for high-throughput scenarios
type ChannelPool struct {
    conn     *amqp.Connection
    channels chan *amqp.Channel
}

func NewChannelPool(conn *amqp.Connection, size int) *ChannelPool {
    pool := &ChannelPool{
        conn:     conn,
        channels: make(chan *amqp.Channel, size),
    }
    
    for i := 0; i < size; i++ {
        ch, _ := conn.Channel()
        pool.channels <- ch
    }
    
    return pool
}
```

## Monitoring and Debugging

### Connection Events
```go
// Monitor connection state
closed := make(chan *amqp.Error)
conn.NotifyClose(closed)

go func() {
    err := <-closed
    if err != nil {
        log.Printf("Connection closed: %v", err)
        // Implement reconnection logic
    }
}()

// Monitor channel state  
channelClosed := make(chan *amqp.Error)
channel.NotifyClose(channelClosed)

go func() {
    err := <-channelClosed
    if err != nil {
        log.Printf("Channel closed: %v", err)
    }
}()
```

### Flow Control
```go
// Monitor flow control
flows := make(chan bool)
channel.NotifyFlow(flows)

go func() {
    for flow := range flows {
        if flow {
            log.Println("Flow control: active")
        } else {
            log.Println("Flow control: inactive")
        }
    }
}()
```

### Message Tracking
```go
// Track message delivery
type MessageTracker struct {
    pending map[uint64]time.Time
    mutex   sync.RWMutex
}

func (t *MessageTracker) TrackMessage(deliveryTag uint64) {
    t.mutex.Lock()
    defer t.mutex.Unlock()
    t.pending[deliveryTag] = time.Now()
}

func (t *MessageTracker) AckMessage(deliveryTag uint64) {
    t.mutex.Lock()
    defer t.mutex.Unlock()
    if start, exists := t.pending[deliveryTag]; exists {
        duration := time.Since(start)
        log.Printf("Message %d processed in %v", deliveryTag, duration)
        delete(t.pending, deliveryTag)
    }
}
```

## Error Handling

### Common Error Scenarios
```go
// Handle connection errors
conn, err := amqp.Dial("amqp://localhost:5672/")
if err != nil {
    switch {
    case strings.Contains(err.Error(), "connection refused"):
        log.Fatal("AMQP server not running")
    case strings.Contains(err.Error(), "authentication"):
        log.Fatal("Invalid credentials")
    default:
        log.Fatalf("Connection error: %v", err)
    }
}

// Handle channel errors
channel, err := conn.Channel()
if err != nil {
    log.Fatalf("Failed to open channel: %v", err)
}

// Handle publishing errors
err = channel.Publish(/* ... */)
if err != nil {
    switch e := err.(type) {
    case *amqp.Error:
        log.Printf("AMQP error: %d - %s", e.Code, e.Reason)
    default:
        log.Printf("Publish error: %v", err)
    }
}
```

### Graceful Shutdown
```go
func gracefulShutdown(conn *amqp.Connection) {
    // Close channels first
    for _, ch := range channels {
        if err := ch.Close(); err != nil {
            log.Printf("Error closing channel: %v", err)
        }
    }
    
    // Close connection
    if err := conn.Close(); err != nil {
        log.Printf("Error closing connection: %v", err)
    }
}

// Handle shutdown signals
c := make(chan os.Signal, 1)
signal.Notify(c, os.Interrupt, syscall.SIGTERM)

go func() {
    <-c
    log.Println("Shutting down gracefully...")
    gracefulShutdown(conn)
    os.Exit(0)
}()
```

## Testing with Portask

### Unit Tests
```go
func TestAMQPPublishConsume(t *testing.T) {
    // Start Portask server for testing
    server := startPortaskServer(t)
    defer server.Stop()
    
    // Connect to Portask
    conn, err := amqp.Dial("amqp://localhost:5672/")
    require.NoError(t, err)
    defer conn.Close()
    
    ch, err := conn.Channel()
    require.NoError(t, err)
    defer ch.Close()
    
    // Test exchange declaration
    err = ch.ExchangeDeclare("test", "direct", false, true, false, false, nil)
    require.NoError(t, err)
    
    // Test queue declaration
    queue, err := ch.QueueDeclare("", false, true, true, false, nil)
    require.NoError(t, err)
    
    // Test queue binding
    err = ch.QueueBind(queue.Name, "test-key", "test", false, nil)
    require.NoError(t, err)
    
    // Test publishing
    err = ch.Publish("test", "test-key", false, false, amqp.Publishing{
        Body: []byte("test message"),
    })
    require.NoError(t, err)
    
    // Test consuming
    messages, err := ch.Consume(queue.Name, "", true, false, false, false, nil)
    require.NoError(t, err)
    
    select {
    case msg := <-messages:
        assert.Equal(t, "test message", string(msg.Body))
    case <-time.After(time.Second):
        t.Fatal("No message received")
    }
}
```

### Integration Tests
```go
func TestRabbitMQCompatibility(t *testing.T) {
    // Test against both Portask and real RabbitMQ
    testCases := []struct {
        name string
        url  string
    }{
        {"Portask", "amqp://localhost:5672/"},
        {"RabbitMQ", "amqp://localhost:5673/"}, // Real RabbitMQ instance
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            conn, err := amqp.Dial(tc.url)
            if err != nil {
                t.Skipf("Cannot connect to %s: %v", tc.name, err)
            }
            defer conn.Close()
            
            // Run same test suite against both
            runCompatibilityTests(t, conn)
        })
    }
}
```

## Limitations and Known Issues

### Current Limitations
1. **Clustering**: Single-node operation only
2. **Persistence**: In-memory storage only (messages lost on restart)
3. **Management API**: No web-based management interface
4. **Plugins**: No plugin system support
5. **Authentication**: Basic auth only (no LDAP, OAuth, etc.)

### Performance Considerations
1. **Memory Usage**: All messages stored in RAM
2. **Message Size**: Large messages impact performance
3. **Connection Limits**: Governed by system resources
4. **Throughput**: Optimized for development/testing scenarios

### Compatibility Notes
1. **AMQP 1.0**: Not supported (only AMQP 0-9-1)
2. **RabbitMQ Extensions**: Some proprietary features not implemented
3. **Client Libraries**: Tested with official Go, Python, Java, and .NET clients

## Migration Guide

### From RabbitMQ to Portask
1. **Configuration**: Update connection URLs to point to Portask
2. **Features**: Verify all used features are supported
3. **Testing**: Run existing test suites against Portask
4. **Performance**: Benchmark with your specific workload

### Example Migration
```bash
# Before (RabbitMQ)
export RABBITMQ_URL="amqp://user:pass@rabbitmq.example.com:5672/"

# After (Portask)
export RABBITMQ_URL="amqp://guest:guest@localhost:5672/"

# No code changes required!
```

## Best Practices

### Development
1. **Use unique queue names** to avoid conflicts
2. **Enable publisher confirms** for reliability
3. **Handle connection failures** gracefully
4. **Monitor memory usage** in long-running tests
5. **Clean up resources** (queues, exchanges) after tests

### Production Considerations
1. **Memory limits**: Set appropriate max_memory_mb
2. **Connection pooling**: Use connection/channel pools
3. **Monitoring**: Implement health checks and metrics
4. **Backup strategy**: Consider message persistence needs
5. **Load testing**: Validate performance under expected load

---

For more information, see:
- [Kafka Emulator Documentation](./kafka_emulator.md)
- [API Reference](./api_reference.md)
- [Performance Tuning Guide](./performance.md)
