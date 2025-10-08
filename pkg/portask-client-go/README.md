# Portask Go Client Library

Official Go client library for **Portask Native API**.

## ✨ Features

- **Unified API**: Single client for all messaging operations
- **Producer & Consumer**: Simple message publish/consume
- **Consumer Groups**: Full consumer group lifecycle management
- **Transactions**: Distributed transaction support
- **Batch Operations**: High-throughput batch publish/fetch/ack
- **Type-Safe**: Full Go type safety
- **Context Support**: All operations support `context.Context`

## 📦 Installation

```bash
go get github.com/meftunca/portask/pkg/portask-client-go
```

## 🚀 Quick Start

### Create Client

```go
package main

import (
    "context"
    "log"
    
    portask "github.com/meftunca/portask/pkg/portask-client-go"
)

func main() {
    // Create client
    client, err := portask.NewClient("http://localhost:8080")
    if err != nil {
        log.Fatal(err)
    }
    
    // Check health
    health, err := client.Health(context.Background())
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Connected to Portask %s", health.Version)
}
```

### Producer (Publish Messages)

```go
// Single message
result, err := client.Producer().Publish(ctx, portask.Message{
    Topic: "orders",
    Key:   "order-123",
    Value: map[string]interface{}{
        "order_id":   123,
        "customer":   "John Doe",
        "total":      99.99,
    },
    Headers: map[string]interface{}{
        "source": "web-app",
    },
})

if err != nil {
    log.Fatal(err)
}

log.Printf("Published: %s (offset: %d)", result.MessageID, result.Offset)
```

### Batch Producer

```go
// Batch publish
messages := []portask.Message{
    {Topic: "orders", Value: map[string]interface{}{"order_id": 1}},
    {Topic: "orders", Value: map[string]interface{}{"order_id": 2}},
    {Topic: "orders", Value: map[string]interface{}{"order_id": 3}},
}

results, err := client.Producer().PublishBatch(ctx, messages)
if err != nil {
    log.Fatal(err)
}

log.Printf("Published %d messages", len(results))
```

### Consumer (Fetch Messages)

```go
// Fetch messages
messages, err := client.Consumer().Fetch(ctx, portask.ConsumeOptions{
    Topic:       "orders",
    MaxMessages: 100,
    MaxWaitMs:   5000,
})

if err != nil {
    log.Fatal(err)
}

for _, msg := range messages {
    log.Printf("Received: %s - %v", msg.MessageID, msg.Value)
    
    // Acknowledge message
    err = client.Consumer().Acknowledge(ctx, msg.MessageID, "")
    if err != nil {
        log.Printf("Failed to ack: %v", err)
    }
}
```

### Consumer Groups

```go
// Create consumer group
group, err := client.ConsumerGroup().Create(ctx, "order-processors", []string{"orders"})
if err != nil {
    log.Fatal(err)
}

log.Printf("Created consumer group: %s", group.ID)

// Join group
joinResp, err := client.ConsumerGroup().Join(ctx, "order-processors", "client-1")
if err != nil {
    log.Fatal(err)
}

log.Printf("Joined as member: %s (generation: %d)", joinResp.MemberID, joinResp.Generation)

// Commit offsets
offsets := []portask.OffsetCommit{
    {Topic: "orders", Partition: 0, Offset: 100},
    {Topic: "orders", Partition: 1, Offset: 150},
}

err = client.ConsumerGroup().CommitOffsets(ctx, "order-processors", offsets)
if err != nil {
    log.Fatal(err)
}

// Get lag
lag, err := client.ConsumerGroup().GetLag(ctx, "order-processors")
if err != nil {
    log.Fatal(err)
}

log.Printf("Total lag: %d", lag.TotalLag)
```

### Transactions

```go
// Begin transaction
txn, err := client.Transaction().Begin(ctx, 60000, []string{"orders", "inventory"})
if err != nil {
    log.Fatal(err)
}

log.Printf("Transaction started: %s", txn.ID)

// Publish messages in transaction
// TODO: Add transaction ID to messages

// Commit transaction
err = client.Transaction().Commit(ctx, txn.ID)
if err != nil {
    log.Fatal(err)
}

log.Printf("Transaction committed: %s", txn.ID)
```

## 🛠️ Advanced Usage

### With API Key

```go
client, err := portask.NewClient(
    "http://localhost:8080",
    portask.WithAPIKey("your-api-key"),
)
```

### With Custom Timeout

```go
client, err := portask.NewClient(
    "http://localhost:8080",
    portask.WithTimeout(60 * time.Second),
)
```

### With Custom HTTP Client

```go
httpClient := &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        100,
        IdleConnTimeout:     90 * time.Second,
        TLSHandshakeTimeout: 10 * time.Second,
    },
}

client, err := portask.NewClient(
    "http://localhost:8080",
    portask.WithHTTPClient(httpClient),
)
```

### Async Publishing (Fire-and-Forget)

```go
err := client.Producer().PublishAsync(ctx, []portask.Message{
    {Topic: "logs", Value: map[string]interface{}{"level": "info", "msg": "Hello"}},
})
```

### Long-Polling Consumer

```go
// Blocks until messages available or timeout
messages, err := client.Consumer().FetchPoll(ctx, portask.ConsumeOptions{
    Topic:       "orders",
    MaxMessages: 10,
    MaxWaitMs:   30000, // Wait up to 30 seconds
})
```

## 📖 API Reference

### Client Methods

- `Producer() *Producer` - Get producer instance
- `Consumer() *Consumer` - Get consumer instance
- `ConsumerGroup() *ConsumerGroupClient` - Get consumer group client
- `Transaction() *TransactionClient` - Get transaction client
- `Health(ctx) (*HealthStatus, error)` - Check server health

### Producer Methods

- `Publish(ctx, msg) (*ProduceResult, error)` - Publish single message
- `PublishBatch(ctx, messages) ([]ProduceResult, error)` - Batch publish
- `PublishAsync(ctx, messages) error` - Async publish (fire-and-forget)

### Consumer Methods

- `Fetch(ctx, opts) ([]FetchedMessage, error)` - Fetch messages
- `FetchPoll(ctx, opts) ([]FetchedMessage, error)` - Long-polling fetch
- `Acknowledge(ctx, msgID, groupID) error` - Acknowledge message
- `AcknowledgeBatch(ctx, msgIDs, groupID) error` - Batch acknowledge
- `NegativeAcknowledge(ctx, msgID, reason, requeue, groupID) error` - Negative ack

### Consumer Group Methods

- `Create(ctx, name, topics) (*ConsumerGroup, error)` - Create group
- `List(ctx) ([]ConsumerGroup, error)` - List groups
- `Get(ctx, groupID) (*ConsumerGroup, error)` - Get group
- `Delete(ctx, groupID) error` - Delete group
- `Join(ctx, groupID, clientID) (*JoinGroupResponse, error)` - Join group
- `Leave(ctx, groupID, memberID) error` - Leave group
- `Heartbeat(ctx, groupID, memberID, generation) error` - Send heartbeat
- `CommitOffsets(ctx, groupID, offsets) error` - Commit offsets
- `FetchOffsets(ctx, groupID) (map[string]map[int32]OffsetInfo, error)` - Fetch offsets
- `ResetOffsets(ctx, groupID, topics, position) error` - Reset offsets
- `GetLag(ctx, groupID) (*GroupLag, error)` - Get consumer lag
- `ListMembers(ctx, groupID) ([]GroupMember, error)` - List members
- `GetState(ctx, groupID) (map[string]interface{}, error)` - Get state

### Transaction Methods

- `Begin(ctx, timeoutMs, topics) (*Transaction, error)` - Begin transaction
- `Commit(ctx, txnID) error` - Commit transaction
- `Abort(ctx, txnID, reason) error` - Abort transaction
- `GetStatus(ctx, txnID) (*TransactionStatus, error)` - Get status
- `List(ctx) ([]Transaction, error)` - List transactions
- `Delete(ctx, txnID) error` - Delete transaction

## 🌟 Why Portask Native API?

Unlike Kafka or RabbitMQ client libraries, Portask provides:

✅ **Unified Concepts**: Topic (not Queue), Consumer Group (not Consumer Tag)  
✅ **Protocol-Agnostic**: Same API works for Kafka, AMQP, or native clients  
✅ **Modern API**: RESTful HTTP/JSON + WebSocket  
✅ **Simple & Fast**: No complex protocols, no heavyweight dependencies  
✅ **Built-in Features**: Transactions, batching, consumer groups, lag monitoring  

## 📚 Examples

See `examples/` directory for more:

- `simple_producer.go` - Basic producer
- `simple_consumer.go` - Basic consumer
- `consumer_group.go` - Consumer group usage
- `transactions.go` - Transaction support

## 📄 License

MIT

## 🤝 Contributing

Contributions are welcome! Please open an issue or submit a PR.

## 📞 Support

- GitHub Issues: https://github.com/meftunca/portask/issues
- Documentation: https://github.com/meftunca/portask/tree/main/docs

