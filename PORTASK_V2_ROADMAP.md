# Portask v2.0 Roadmap - Native-First Platform

**Created:** October 8, 2025  
**Status:** Planning Phase  
**Goal:** Transform Portask from translator-centric to native-first unified messaging platform

---

## 🎯 Vision for v2.0

**Current State (v1.0):**

> Portask = "Kafka and RabbitMQ compatible server with good performance"

**Target State (v2.0):**

> Portask = "Unified messaging platform with native API, that also supports Kafka/RabbitMQ protocols"

---

## 📋 Development Phases

### Phase 1: Backend Native API 🔧 (Week 1-2)

**Goal:** Expose all core features through Portask Native REST API

### Phase 2: Client Libraries 📚 (Week 3-4)

**Goal:** Create Go and TypeScript client libraries for Portask Native API

### Phase 3: Admin UI Refactoring 🎨 (Week 5-6)

**Goal:** Rebuild UI as Portask-centric with protocol compatibility as secondary feature

---

# 🔧 PHASE 1: Backend Native API

## 1.1 Consumer Groups API (Critical)

### Current State Analysis

```go
// ✅ Already exists in pkg/kafka/consumer_groups.go
type ConsumerGroupCoordinator struct {
    groups      map[string]*ConsumerGroup
    mu          sync.RWMutex
    rebalancer  *Rebalancer
    offsetMgr   *OffsetManager
}

// ✅ Already has methods:
- JoinGroup()
- SyncGroup()
- Heartbeat()
- LeaveGroup()
- DescribeGroups()
- ListGroups()
```

### Problem

❌ These are only accessible via Kafka wire protocol (port 9092)  
❌ No REST API endpoints expose this functionality

### Solution: Native REST API

#### 1.1.1 New File: `pkg/api/consumer_groups_native.go`

```go
package api

import (
    "github.com/gofiber/fiber/v2"
    "github.com/meftunca/portask/pkg/types"
)

// Native Portask Consumer Group structure
type NativeConsumerGroup struct {
    ID            string                  `json:"id"`
    Name          string                  `json:"name"`
    State         string                  `json:"state"`  // Stable, Rebalancing, Dead, Empty
    Protocol      string                  `json:"protocol"`
    ProtocolType  string                  `json:"protocol_type"`
    Leader        string                  `json:"leader"`
    Generation    int32                   `json:"generation"`
    Members       []NativeGroupMember     `json:"members"`
    Subscriptions []string                `json:"subscriptions"`  // Topics subscribed
    CreatedAt     string                  `json:"created_at"`
    UpdatedAt     string                  `json:"updated_at"`
}

type NativeGroupMember struct {
    ID            string                     `json:"id"`
    ClientID      string                     `json:"client_id"`
    ClientHost    string                     `json:"client_host"`
    SessionTimeout int32                     `json:"session_timeout_ms"`
    Assignment    []NativePartitionAssignment `json:"assignment"`
    JoinedAt      string                     `json:"joined_at"`
    LastHeartbeat string                     `json:"last_heartbeat"`
}

type NativePartitionAssignment struct {
    Topic      string  `json:"topic"`
    Partitions []int32 `json:"partitions"`
}

// Request/Response types
type CreateGroupRequest struct {
    Name         string   `json:"name" validate:"required"`
    Protocol     string   `json:"protocol"`  // Default: "range"
    ProtocolType string   `json:"protocol_type"`  // Default: "consumer"
    Topics       []string `json:"topics" validate:"required"`
}

type JoinGroupRequest struct {
    MemberID       string `json:"member_id"`  // Empty on first join
    ClientID       string `json:"client_id" validate:"required"`
    ClientHost     string `json:"client_host"`
    SessionTimeout int32  `json:"session_timeout_ms"`  // Default: 10000
}

type JoinGroupResponse struct {
    MemberID    string                  `json:"member_id"`
    Generation  int32                   `json:"generation"`
    Leader      bool                    `json:"is_leader"`
    Members     []NativeGroupMember     `json:"members,omitempty"`  // Only for leader
    Assignment  []NativePartitionAssignment `json:"assignment"`
}

type CommitOffsetRequest struct {
    Topic     string `json:"topic" validate:"required"`
    Partition int32  `json:"partition" validate:"min=0"`
    Offset    int64  `json:"offset" validate:"min=0"`
    Metadata  string `json:"metadata"`
}

type FetchOffsetsResponse struct {
    Offsets map[string]map[int32]int64 `json:"offsets"`  // topic -> partition -> offset
}

type GroupLagInfo struct {
    Topic        string `json:"topic"`
    Partition    int32  `json:"partition"`
    CurrentOffset int64  `json:"current_offset"`
    LogEndOffset  int64  `json:"log_end_offset"`
    Lag          int64  `json:"lag"`
}

type GroupLagResponse struct {
    GroupID   string         `json:"group_id"`
    TotalLag  int64          `json:"total_lag"`
    Lag       []GroupLagInfo `json:"partitions"`
}
```

#### 1.1.2 API Endpoints

```go
// Consumer Groups Management
POST   /api/v1/consumer-groups                    // Create group
GET    /api/v1/consumer-groups                    // List all groups
GET    /api/v1/consumer-groups/:id                // Get group details
DELETE /api/v1/consumer-groups/:id                // Delete group
PUT    /api/v1/consumer-groups/:id                // Update group (topics)

// Group Membership
POST   /api/v1/consumer-groups/:id/join           // Join group
POST   /api/v1/consumer-groups/:id/leave          // Leave group
POST   /api/v1/consumer-groups/:id/heartbeat      // Send heartbeat

// Offset Management
GET    /api/v1/consumer-groups/:id/offsets        // Fetch committed offsets
POST   /api/v1/consumer-groups/:id/offsets/commit // Commit offsets (batch)
POST   /api/v1/consumer-groups/:id/offsets/reset  // Reset offsets to earliest/latest

// Monitoring
GET    /api/v1/consumer-groups/:id/lag            // Get consumer lag per partition
GET    /api/v1/consumer-groups/:id/members        // List active members
GET    /api/v1/consumer-groups/:id/state          // Get group state

// Bulk Operations
POST   /api/v1/consumer-groups/bulk-offsets/commit  // Commit multiple offsets at once
GET    /api/v1/consumer-groups/bulk-lag             // Get lag for all groups
```

#### 1.1.3 Implementation Strategy

```go
// Step 1: Create adapter from Kafka coordinator to Native API
// File: pkg/api/consumer_groups_native.go

func (s *FiberServer) handleCreateConsumerGroup(c *fiber.Ctx) error {
    var req CreateGroupRequest
    if err := c.BodyParser(&req); err != nil {
        return c.Status(400).JSON(fiber.Map{"error": err.Error()})
    }

    // Get Kafka coordinator (already exists)
    coordinator := s.kafkaServer.GetCoordinator()

    // Create group through coordinator
    group := coordinator.CreateGroup(req.Name, req.Protocol, req.ProtocolType)

    // Subscribe to topics
    for _, topic := range req.Topics {
        group.Subscribe(topic)
    }

    return c.Status(201).JSON(toNativeConsumerGroup(group))
}

func (s *FiberServer) handleListConsumerGroups(c *fiber.Ctx) error {
    coordinator := s.kafkaServer.GetCoordinator()
    groups := coordinator.ListGroups()

    nativeGroups := make([]NativeConsumerGroup, len(groups))
    for i, g := range groups {
        nativeGroups[i] = toNativeConsumerGroup(g)
    }

    return c.JSON(fiber.Map{
        "groups": nativeGroups,
        "count":  len(nativeGroups),
    })
}

// Helper: Convert Kafka ConsumerGroup to Native format
func toNativeConsumerGroup(kg *kafka.ConsumerGroup) NativeConsumerGroup {
    return NativeConsumerGroup{
        ID:            kg.GroupID,
        Name:          kg.GroupID,
        State:         kg.State.String(),
        Protocol:      kg.Protocol,
        ProtocolType:  kg.ProtocolType,
        Leader:        kg.LeaderID,
        Generation:    kg.Generation,
        Members:       convertMembers(kg.Members),
        Subscriptions: kg.Topics,
        CreatedAt:     kg.CreatedAt.Format(time.RFC3339),
        UpdatedAt:     kg.UpdatedAt.Format(time.RFC3339),
    }
}
```

#### 1.1.4 Testing Strategy

```go
// File: pkg/api/consumer_groups_native_test.go

func TestNativeConsumerGroupAPI(t *testing.T) {
    // Test 1: Create group
    t.Run("CreateGroup", func(t *testing.T) {
        req := CreateGroupRequest{
            Name:   "test-group",
            Topics: []string{"test-topic"},
        }
        // POST /api/v1/consumer-groups
        // Assert: 201 Created, group ID returned
    })

    // Test 2: Join group
    t.Run("JoinGroup", func(t *testing.T) {
        // POST /api/v1/consumer-groups/test-group/join
        // Assert: Member ID assigned, generation incremented
    })

    // Test 3: Commit offset
    t.Run("CommitOffset", func(t *testing.T) {
        // POST /api/v1/consumer-groups/test-group/offsets/commit
        // Assert: Offset committed successfully
    })

    // Test 4: Fetch offsets
    t.Run("FetchOffsets", func(t *testing.T) {
        // GET /api/v1/consumer-groups/test-group/offsets
        // Assert: Committed offsets returned
    })

    // Test 5: Get lag
    t.Run("GetLag", func(t *testing.T) {
        // GET /api/v1/consumer-groups/test-group/lag
        // Assert: Lag calculated correctly
    })
}
```

---

## 1.2 Batch Operations API (Critical)

### Current State

✅ Internal batch writer exists in `pkg/processor/async_batch_writer.go`  
❌ No REST API to publish/fetch messages in batches

### Solution

#### 1.2.1 New File: `pkg/api/batch_operations.go`

```go
package api

type BatchPublishRequest struct {
    Messages []PublishMessage `json:"messages" validate:"required,min=1,max=1000"`
}

type PublishMessage struct {
    Topic     string                 `json:"topic" validate:"required"`
    Partition int32                  `json:"partition"`
    Key       string                 `json:"key"`
    Value     interface{}            `json:"value" validate:"required"`
    Headers   map[string]interface{} `json:"headers"`
    TTL       *int64                 `json:"ttl_ms"`  // Time-to-live in milliseconds
}

type BatchPublishResponse struct {
    Published int                `json:"published"`
    Failed    int                `json:"failed"`
    Results   []PublishResult    `json:"results"`
    Duration  string             `json:"duration"`
}

type PublishResult struct {
    Index     int    `json:"index"`
    MessageID string `json:"message_id"`
    Topic     string `json:"topic"`
    Partition int32  `json:"partition"`
    Offset    int64  `json:"offset"`
    Error     string `json:"error,omitempty"`
}

type BatchFetchRequest struct {
    Topics         []TopicFetchRequest `json:"topics" validate:"required"`
    MaxMessages    int                 `json:"max_messages"`  // Default: 100, Max: 1000
    MaxWaitMs      int                 `json:"max_wait_ms"`   // Default: 1000
    MinBytes       int                 `json:"min_bytes"`     // Default: 1
    IsolationLevel string              `json:"isolation_level"`  // "read_uncommitted" or "read_committed"
}

type TopicFetchRequest struct {
    Topic      string              `json:"topic" validate:"required"`
    Partitions []PartitionFetchRequest `json:"partitions"`
}

type PartitionFetchRequest struct {
    Partition   int32 `json:"partition" validate:"min=0"`
    FetchOffset int64 `json:"fetch_offset" validate:"min=0"`
    MaxBytes    int   `json:"max_bytes"`  // Max bytes to fetch from this partition
}

type BatchFetchResponse struct {
    Topics   []TopicFetchResponse `json:"topics"`
    Total    int                  `json:"total_messages"`
    Duration string               `json:"duration"`
}

type TopicFetchResponse struct {
    Topic      string                   `json:"topic"`
    Partitions []PartitionFetchResponse `json:"partitions"`
}

type PartitionFetchResponse struct {
    Partition     int32             `json:"partition"`
    HighWaterMark int64             `json:"high_water_mark"`
    Messages      []FetchedMessage  `json:"messages"`
    Error         string            `json:"error,omitempty"`
}

type FetchedMessage struct {
    MessageID string                 `json:"message_id"`
    Offset    int64                  `json:"offset"`
    Key       string                 `json:"key"`
    Value     interface{}            `json:"value"`
    Headers   map[string]interface{} `json:"headers"`
    Timestamp string                 `json:"timestamp"`
    Size      int                    `json:"size_bytes"`
}
```

#### 1.2.2 API Endpoints

```go
// Batch Publishing
POST   /api/v1/messages/batch/publish              // Publish multiple messages
POST   /api/v1/messages/batch/publish/async        // Publish async (fire-and-forget)

// Batch Fetching
POST   /api/v1/messages/batch/fetch                // Fetch multiple messages
POST   /api/v1/messages/batch/fetch/poll           // Long-polling fetch

// Batch Acknowledgment
POST   /api/v1/messages/batch/ack                  // Acknowledge multiple messages
POST   /api/v1/messages/batch/nack                 // Negative acknowledge (retry)
```

#### 1.2.3 Implementation

```go
func (s *FiberServer) handleBatchPublish(c *fiber.Ctx) error {
    startTime := time.Now()

    var req BatchPublishRequest
    if err := c.BodyParser(&req); err != nil {
        return c.Status(400).JSON(fiber.Map{"error": err.Error()})
    }

    if len(req.Messages) > 1000 {
        return c.Status(400).JSON(fiber.Map{"error": "Max 1000 messages per batch"})
    }

    results := make([]PublishResult, len(req.Messages))
    published, failed := 0, 0

    // Use existing processor for batch write
    batch := make([]*types.PortaskMessage, 0, len(req.Messages))

    for i, msg := range req.Messages {
        portaskMsg := &types.PortaskMessage{
            ID:        types.MessageID(generateMessageID(msg.Topic)),
            Topic:     types.TopicName(msg.Topic),
            Partition: msg.Partition,
            Key:       msg.Key,
            Payload:   serializeValue(msg.Value),
            Headers:   msg.Headers,
            Timestamp: time.Now().UnixNano(),
        }

        if msg.TTL != nil {
            portaskMsg.TTL = *msg.TTL
        }

        batch = append(batch, portaskMsg)

        results[i] = PublishResult{
            Index:     i,
            MessageID: string(portaskMsg.ID),
            Topic:     msg.Topic,
            Partition: msg.Partition,
        }
    }

    // Write batch to storage
    if err := s.processor.ProcessBatch(c.Context(), batch); err != nil {
        // Handle partial failures
        for i := range results {
            results[i].Error = err.Error()
            failed++
        }
    } else {
        published = len(batch)
    }

    duration := time.Since(startTime)

    return c.Status(201).JSON(BatchPublishResponse{
        Published: published,
        Failed:    failed,
        Results:   results,
        Duration:  duration.String(),
    })
}

func (s *FiberServer) handleBatchFetch(c *fiber.Ctx) error {
    startTime := time.Now()

    var req BatchFetchRequest
    if err := c.BodyParser(&req); err != nil {
        return c.Status(400).JSON(fiber.Map{"error": err.Error()})
    }

    // Set defaults
    if req.MaxMessages == 0 {
        req.MaxMessages = 100
    }
    if req.MaxWaitMs == 0 {
        req.MaxWaitMs = 1000
    }

    topicResponses := make([]TopicFetchResponse, 0)
    totalMessages := 0

    for _, topicReq := range req.Topics {
        partitionResponses := make([]PartitionFetchResponse, 0)

        for _, partReq := range topicReq.Partitions {
            // Fetch messages from storage
            messages, err := s.storage.FetchMessages(c.Context(), storage.FetchRequest{
                Topic:       topicReq.Topic,
                Partition:   partReq.Partition,
                StartOffset: partReq.FetchOffset,
                MaxMessages: req.MaxMessages,
                MaxBytes:    partReq.MaxBytes,
            })

            if err != nil {
                partitionResponses = append(partitionResponses, PartitionFetchResponse{
                    Partition: partReq.Partition,
                    Error:     err.Error(),
                })
                continue
            }

            fetchedMsgs := convertToFetchedMessages(messages)
            totalMessages += len(fetchedMsgs)

            partitionResponses = append(partitionResponses, PartitionFetchResponse{
                Partition:     partReq.Partition,
                HighWaterMark: getHighWaterMark(topicReq.Topic, partReq.Partition),
                Messages:      fetchedMsgs,
            })
        }

        topicResponses = append(topicResponses, TopicFetchResponse{
            Topic:      topicReq.Topic,
            Partitions: partitionResponses,
        })
    }

    duration := time.Since(startTime)

    return c.JSON(BatchFetchResponse{
        Topics:   topicResponses,
        Total:    totalMessages,
        Duration: duration.String(),
    })
}
```

---

## 1.3 WebSocket Subscribe API (High Priority)

### Goal

Real-time message consumption via WebSocket for web clients

#### 1.3.1 New File: `pkg/api/websocket_subscribe.go`

```go
package api

type SubscribeRequest struct {
    Topics        []string `json:"topics" validate:"required"`
    GroupID       string   `json:"group_id"`  // Optional: for consumer group
    FromBeginning bool     `json:"from_beginning"`
    AutoCommit    bool     `json:"auto_commit"`  // Default: true
}

type MessageEvent struct {
    Type      string      `json:"type"`  // "message", "error", "ack", "commit"
    MessageID string      `json:"message_id"`
    Topic     string      `json:"topic"`
    Partition int32       `json:"partition"`
    Offset    int64       `json:"offset"`
    Key       string      `json:"key"`
    Value     interface{} `json:"value"`
    Headers   map[string]interface{} `json:"headers"`
    Timestamp string      `json:"timestamp"`
}

type AckRequest struct {
    Type      string `json:"type"`  // "ack" or "nack"
    MessageID string `json:"message_id"`
}
```

#### 1.3.2 WebSocket Protocol

```javascript
// Client side (TypeScript example)
const ws = new WebSocket("ws://localhost:8080/api/v1/messages/subscribe");

// Subscribe to topics
ws.send(
  JSON.stringify({
    topics: ["orders", "payments"],
    group_id: "my-group",
    auto_commit: false,
  })
);

// Receive messages
ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  if (msg.type === "message") {
    console.log("Received:", msg.value);

    // Send acknowledgment
    ws.send(
      JSON.stringify({
        type: "ack",
        message_id: msg.message_id,
      })
    );
  }
};
```

#### 1.3.3 Server Implementation

```go
func (s *FiberServer) handleWebSocketSubscribe(c *websocket.Conn) {
    defer c.Close()

    // Read subscribe request
    var req SubscribeRequest
    if err := c.ReadJSON(&req); err != nil {
        c.WriteJSON(fiber.Map{"error": err.Error()})
        return
    }

    // Create consumer
    consumer := s.createConsumer(req)
    defer consumer.Close()

    // Message pump
    go func() {
        for msg := range consumer.Messages() {
            event := MessageEvent{
                Type:      "message",
                MessageID: string(msg.ID),
                Topic:     string(msg.Topic),
                Partition: msg.Partition,
                Offset:    msg.Offset,
                Key:       msg.Key,
                Value:     msg.Payload,
                Headers:   msg.Headers,
                Timestamp: time.Unix(0, msg.Timestamp).Format(time.RFC3339),
            }

            if err := c.WriteJSON(event); err != nil {
                return
            }

            if req.AutoCommit {
                consumer.CommitOffset(msg.Topic, msg.Partition, msg.Offset)
            }
        }
    }()

    // Handle acks from client
    for {
        var ack AckRequest
        if err := c.ReadJSON(&ack); err != nil {
            break
        }

        if ack.Type == "ack" {
            consumer.Acknowledge(ack.MessageID)
        } else if ack.Type == "nack" {
            consumer.NegativeAcknowledge(ack.MessageID)
        }
    }
}
```

---

## 1.4 Transaction API (Medium Priority)

### Goal

Support distributed transactions across multiple topics/partitions

#### 1.4.1 New File: `pkg/api/transactions.go`

```go
package api

type BeginTransactionRequest struct {
    TransactionID    string `json:"transaction_id"`  // Optional: auto-generated if empty
    TimeoutMs        int32  `json:"timeout_ms"`  // Default: 60000 (60 seconds)
    ProducerID       string `json:"producer_id"`
    ProducerEpoch    int16  `json:"producer_epoch"`
}

type BeginTransactionResponse struct {
    TransactionID string `json:"transaction_id"`
    ProducerID    int64  `json:"producer_id"`
    Epoch         int16  `json:"epoch"`
    ExpiresAt     string `json:"expires_at"`
}

type CommitTransactionRequest struct {
    TransactionID string `json:"transaction_id" validate:"required"`
}

type RollbackTransactionRequest struct {
    TransactionID string `json:"transaction_id" validate:"required"`
}

type TransactionStatus struct {
    TransactionID string `json:"transaction_id"`
    State         string `json:"state"`  // Ongoing, PrepareCommit, CompleteCommit, PrepareAbort, CompleteAbort
    StartedAt     string `json:"started_at"`
    ExpiresAt     string `json:"expires_at"`
    Topics        []string `json:"topics"`
    Partitions    int    `json:"partitions"`
}
```

#### 1.4.2 API Endpoints

```go
POST   /api/v1/transactions/begin               // Begin transaction
POST   /api/v1/transactions/:id/commit          // Commit transaction
POST   /api/v1/transactions/:id/rollback        // Rollback transaction
GET    /api/v1/transactions/:id/status          // Get transaction status
GET    /api/v1/transactions                     // List active transactions
POST   /api/v1/transactions/:id/add-partitions  // Add partitions to transaction
```

#### 1.4.3 Usage Example

```bash
# 1. Begin transaction
curl -X POST http://localhost:8080/api/v1/transactions/begin \
  -H "Content-Type: application/json" \
  -d '{"producer_id": "client-1"}'
# Response: {"transaction_id": "txn-123", "producer_id": 1001, "epoch": 0}

# 2. Publish messages in transaction
curl -X POST http://localhost:8080/api/v1/messages/batch/publish \
  -H "Content-Type: application/json" \
  -d '{
    "transaction_id": "txn-123",
    "messages": [
      {"topic": "orders", "value": {"order_id": 1}},
      {"topic": "inventory", "value": {"product_id": 100, "qty": -1}}
    ]
  }'

# 3. Commit transaction
curl -X POST http://localhost:8080/api/v1/transactions/txn-123/commit

# Or rollback on error
curl -X POST http://localhost:8080/api/v1/transactions/txn-123/rollback
```

---

## 1.5 Extended Topic Management (Medium Priority)

#### New Endpoints

```go
POST   /api/v1/topics                             // Create topic with config
PUT    /api/v1/topics/:name                       // Update topic config
DELETE /api/v1/topics/:name                       // Delete topic
GET    /api/v1/topics/:name/partitions            // List partitions
POST   /api/v1/topics/:name/partitions            // Add partitions
GET    /api/v1/topics/:name/consumers             // Active consumers for topic
GET    /api/v1/topics/:name/stats                 // Topic statistics (msgs, size, rate)
GET    /api/v1/topics/:name/config                // Get topic configuration
PUT    /api/v1/topics/:name/config                // Update topic configuration
```

#### Topic Configuration

```go
type TopicConfig struct {
    NumPartitions     int32  `json:"num_partitions"`
    ReplicationFactor int16  `json:"replication_factor"`
    RetentionMs       int64  `json:"retention_ms"`  // -1 = unlimited
    MaxMessageBytes   int32  `json:"max_message_bytes"`
    CompressionType   string `json:"compression_type"`  // "none", "gzip", "snappy", "lz4", "zstd"
    CleanupPolicy     string `json:"cleanup_policy"`  // "delete", "compact"
}

type TopicStats struct {
    Name             string  `json:"name"`
    Partitions       int32   `json:"partitions"`
    TotalMessages    int64   `json:"total_messages"`
    TotalSizeBytes   int64   `json:"total_size_bytes"`
    MessagesPerSec   float64 `json:"messages_per_sec"`
    BytesPerSec      float64 `json:"bytes_per_sec"`
    ConsumersActive  int     `json:"consumers_active"`
    ProducersActive  int     `json:"producers_active"`
    OldestMessage    string  `json:"oldest_message_timestamp"`
    NewestMessage    string  `json:"newest_message_timestamp"`
}
```

---

## 1.6 Message Acknowledgment API (Medium Priority)

#### New File: `pkg/api/acknowledgment.go`

```go
package api

type AcknowledgeRequest struct {
    MessageIDs []string `json:"message_ids" validate:"required,min=1,max=100"`
    GroupID    string   `json:"group_id"`  // Optional: for consumer groups
}

type AcknowledgeResponse struct {
    Acknowledged int      `json:"acknowledged"`
    Failed       int      `json:"failed"`
    Errors       []string `json:"errors,omitempty"`
}

type NegativeAckRequest struct {
    MessageIDs []string `json:"message_ids" validate:"required"`
    Reason     string   `json:"reason"`
    Requeue    bool     `json:"requeue"`  // Requeue for retry
}
```

#### API Endpoints

```go
POST   /api/v1/messages/ack                      // Acknowledge message(s)
POST   /api/v1/messages/nack                     // Negative acknowledge (retry/DLQ)
POST   /api/v1/messages/batch/ack                // Bulk acknowledge
POST   /api/v1/messages/:id/ack                  // Acknowledge single message
POST   /api/v1/messages/:id/nack                 // Negative acknowledge single message
```

---

## 1.7 API Version Header & Compatibility

#### Add API Versioning

```go
// All responses should include:
X-Portask-API-Version: 2.0
X-Portask-Server-Version: 2.0.0

// Client can request specific version:
Accept: application/vnd.portask.v2+json
```

---

# 📚 PHASE 2: Client Libraries

## 2.1 Go Client Library

### File Structure

```
pkg/portask-client-go/
├── client.go              # Main client
├── consumer.go            # Consumer operations
├── producer.go            # Producer operations
├── consumer_group.go      # Consumer group operations
├── transaction.go         # Transaction support
├── types.go               # Shared types
├── examples/
│   ├── simple_producer.go
│   ├── simple_consumer.go
│   ├── consumer_group.go
│   └── transactions.go
└── README.md
```

### 2.1.1 Core Client (`client.go`)

```go
package portask

import (
    "context"
    "net/http"
    "time"
)

// Client is the main Portask client
type Client struct {
    baseURL    string
    httpClient *http.Client
    apiKey     string

    // Lazy-initialized components
    producer      *Producer
    consumer      *Consumer
    consumerGroup *ConsumerGroupClient
    transaction   *TransactionClient
}

// NewClient creates a new Portask client
func NewClient(baseURL string, opts ...Option) (*Client, error) {
    client := &Client{
        baseURL: baseURL,
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
        },
    }

    for _, opt := range opts {
        opt(client)
    }

    return client, nil
}

// Option is a client configuration option
type Option func(*Client)

// WithAPIKey sets the API key for authentication
func WithAPIKey(key string) Option {
    return func(c *Client) {
        c.apiKey = key
    }
}

// WithTimeout sets the HTTP client timeout
func WithTimeout(timeout time.Duration) Option {
    return func(c *Client) {
        c.httpClient.Timeout = timeout
    }
}

// Producer returns a producer instance
func (c *Client) Producer() *Producer {
    if c.producer == nil {
        c.producer = &Producer{client: c}
    }
    return c.producer
}

// Consumer returns a consumer instance
func (c *Client) Consumer() *Consumer {
    if c.consumer == nil {
        c.consumer = &Consumer{client: c}
    }
    return c.consumer
}

// ConsumerGroup returns a consumer group client
func (c *Client) ConsumerGroup() *ConsumerGroupClient {
    if c.consumerGroup == nil {
        c.consumerGroup = &ConsumerGroupClient{client: c}
    }
    return c.consumerGroup
}

// Transaction returns a transaction client
func (c *Client) Transaction() *TransactionClient {
    if c.transaction == nil {
        c.transaction = &TransactionClient{client: c}
    }
    return c.transaction
}

// Health checks server health
func (c *Client) Health(ctx context.Context) (*HealthStatus, error) {
    var status HealthStatus
    err := c.get(ctx, "/health", &status)
    return &status, err
}

// Internal HTTP methods
func (c *Client) get(ctx context.Context, path string, result interface{}) error {
    // Implementation
}

func (c *Client) post(ctx context.Context, path string, body, result interface{}) error {
    // Implementation
}

func (c *Client) put(ctx context.Context, path string, body, result interface{}) error {
    // Implementation
}

func (c *Client) delete(ctx context.Context, path string) error {
    // Implementation
}
```

### 2.1.2 Producer (`producer.go`)

```go
package portask

// Producer handles message production
type Producer struct {
    client *Client
}

// Message represents a message to publish
type Message struct {
    Topic     string
    Key       string
    Value     interface{}
    Headers   map[string]interface{}
    Partition *int32  // Optional: specific partition
    TTL       *int64  // Optional: TTL in milliseconds
}

// ProduceResult contains the result of a publish operation
type ProduceResult struct {
    MessageID string
    Topic     string
    Partition int32
    Offset    int64
}

// Publish publishes a single message
func (p *Producer) Publish(ctx context.Context, msg Message) (*ProduceResult, error) {
    req := map[string]interface{}{
        "topic": msg.Topic,
        "value": msg.Value,
    }

    if msg.Key != "" {
        req["key"] = msg.Key
    }
    if msg.Headers != nil {
        req["headers"] = msg.Headers
    }
    if msg.Partition != nil {
        req["partition"] = *msg.Partition
    }
    if msg.TTL != nil {
        req["ttl_ms"] = *msg.TTL
    }

    var result ProduceResult
    err := p.client.post(ctx, "/api/v1/messages/publish", req, &result)
    return &result, err
}

// PublishBatch publishes multiple messages in a single request
func (p *Producer) PublishBatch(ctx context.Context, messages []Message) ([]ProduceResult, error) {
    req := map[string]interface{}{
        "messages": messages,
    }

    var response struct {
        Results []ProduceResult `json:"results"`
    }

    err := p.client.post(ctx, "/api/v1/messages/batch/publish", req, &response)
    return response.Results, err
}

// PublishAsync publishes a message asynchronously (fire-and-forget)
func (p *Producer) PublishAsync(ctx context.Context, msg Message) error {
    go func() {
        p.Publish(context.Background(), msg)
    }()
    return nil
}
```

### 2.1.3 Consumer (`consumer.go`)

```go
package portask

// Consumer handles message consumption
type Consumer struct {
    client *Client
}

// ConsumeOptions configures message consumption
type ConsumeOptions struct {
    Topic         string
    Partition     *int32
    StartOffset   *int64
    MaxMessages   int
    MaxWaitMs     int
    GroupID       string  // Optional: for consumer groups
    AutoCommit    bool
}

// FetchedMessage represents a consumed message
type FetchedMessage struct {
    MessageID string
    Topic     string
    Partition int32
    Offset    int64
    Key       string
    Value     interface{}
    Headers   map[string]interface{}
    Timestamp time.Time
    Size      int
}

// Fetch fetches messages from a topic
func (c *Consumer) Fetch(ctx context.Context, opts ConsumeOptions) ([]FetchedMessage, error) {
    req := map[string]interface{}{
        "topics": []map[string]interface{}{
            {
                "topic": opts.Topic,
                "partitions": []map[string]interface{}{
                    {
                        "partition":    opts.Partition,
                        "fetch_offset": opts.StartOffset,
                    },
                },
            },
        },
        "max_messages": opts.MaxMessages,
        "max_wait_ms":  opts.MaxWaitMs,
    }

    var response struct {
        Topics []struct {
            Topic      string `json:"topic"`
            Partitions []struct {
                Messages []FetchedMessage `json:"messages"`
            } `json:"partitions"`
        } `json:"topics"`
    }

    err := c.client.post(ctx, "/api/v1/messages/batch/fetch", req, &response)
    if err != nil {
        return nil, err
    }

    // Flatten messages
    var messages []FetchedMessage
    for _, topic := range response.Topics {
        for _, partition := range topic.Partitions {
            messages = append(messages, partition.Messages...)
        }
    }

    return messages, nil
}

// Subscribe subscribes to topics via WebSocket
func (c *Consumer) Subscribe(ctx context.Context, topics []string, handler func(FetchedMessage) error) error {
    // WebSocket implementation
    // Connect to ws://localhost:8080/api/v1/messages/subscribe
    // Send subscribe request
    // Handle incoming messages with handler
    // TODO: Implementation
    return nil
}

// Acknowledge acknowledges a message
func (c *Consumer) Acknowledge(ctx context.Context, messageID string) error {
    req := map[string]interface{}{
        "message_ids": []string{messageID},
    }

    return c.client.post(ctx, "/api/v1/messages/ack", req, nil)
}

// AcknowledgeBatch acknowledges multiple messages
func (c *Consumer) AcknowledgeBatch(ctx context.Context, messageIDs []string) error {
    req := map[string]interface{}{
        "message_ids": messageIDs,
    }

    return c.client.post(ctx, "/api/v1/messages/batch/ack", req, nil)
}
```

### 2.1.4 Consumer Group (`consumer_group.go`)

```go
package portask

// ConsumerGroupClient manages consumer groups
type ConsumerGroupClient struct {
    client *Client
}

// ConsumerGroup represents a consumer group
type ConsumerGroup struct {
    ID            string
    Name          string
    State         string
    Protocol      string
    ProtocolType  string
    Leader        string
    Generation    int32
    Members       []GroupMember
    Subscriptions []string
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

// GroupMember represents a member of a consumer group
type GroupMember struct {
    ID             string
    ClientID       string
    ClientHost     string
    SessionTimeout int32
    Assignment     []PartitionAssignment
    JoinedAt       time.Time
    LastHeartbeat  time.Time
}

// PartitionAssignment represents partition assignment for a member
type PartitionAssignment struct {
    Topic      string
    Partitions []int32
}

// Create creates a new consumer group
func (cg *ConsumerGroupClient) Create(ctx context.Context, name string, topics []string) (*ConsumerGroup, error) {
    req := map[string]interface{}{
        "name":   name,
        "topics": topics,
    }

    var group ConsumerGroup
    err := cg.client.post(ctx, "/api/v1/consumer-groups", req, &group)
    return &group, err
}

// List lists all consumer groups
func (cg *ConsumerGroupClient) List(ctx context.Context) ([]ConsumerGroup, error) {
    var response struct {
        Groups []ConsumerGroup `json:"groups"`
    }

    err := cg.client.get(ctx, "/api/v1/consumer-groups", &response)
    return response.Groups, err
}

// Get gets details of a consumer group
func (cg *ConsumerGroupClient) Get(ctx context.Context, groupID string) (*ConsumerGroup, error) {
    var group ConsumerGroup
    path := fmt.Sprintf("/api/v1/consumer-groups/%s", groupID)
    err := cg.client.get(ctx, path, &group)
    return &group, err
}

// Join joins a consumer group
func (cg *ConsumerGroupClient) Join(ctx context.Context, groupID, clientID string) (*JoinGroupResponse, error) {
    req := map[string]interface{}{
        "client_id": clientID,
    }

    var response JoinGroupResponse
    path := fmt.Sprintf("/api/v1/consumer-groups/%s/join", groupID)
    err := cg.client.post(ctx, path, req, &response)
    return &response, err
}

// Leave leaves a consumer group
func (cg *ConsumerGroupClient) Leave(ctx context.Context, groupID, memberID string) error {
    req := map[string]interface{}{
        "member_id": memberID,
    }

    path := fmt.Sprintf("/api/v1/consumer-groups/%s/leave", groupID)
    return cg.client.post(ctx, path, req, nil)
}

// CommitOffset commits an offset for a consumer group
func (cg *ConsumerGroupClient) CommitOffset(ctx context.Context, groupID, topic string, partition int32, offset int64) error {
    req := map[string]interface{}{
        "topic":     topic,
        "partition": partition,
        "offset":    offset,
    }

    path := fmt.Sprintf("/api/v1/consumer-groups/%s/offsets/commit", groupID)
    return cg.client.post(ctx, path, req, nil)
}

// FetchOffsets fetches committed offsets for a consumer group
func (cg *ConsumerGroupClient) FetchOffsets(ctx context.Context, groupID string) (map[string]map[int32]int64, error) {
    var response struct {
        Offsets map[string]map[int32]int64 `json:"offsets"`
    }

    path := fmt.Sprintf("/api/v1/consumer-groups/%s/offsets", groupID)
    err := cg.client.get(ctx, path, &response)
    return response.Offsets, err
}

// GetLag gets consumer lag for a consumer group
func (cg *ConsumerGroupClient) GetLag(ctx context.Context, groupID string) (*GroupLag, error) {
    var lag GroupLag
    path := fmt.Sprintf("/api/v1/consumer-groups/%s/lag", groupID)
    err := cg.client.get(ctx, path, &lag)
    return &lag, err
}

// Delete deletes a consumer group
func (cg *ConsumerGroupClient) Delete(ctx context.Context, groupID string) error {
    path := fmt.Sprintf("/api/v1/consumer-groups/%s", groupID)
    return cg.client.delete(ctx, path)
}
```

### 2.1.5 Transaction (`transaction.go`)

```go
package portask

// TransactionClient manages transactions
type TransactionClient struct {
    client *Client
}

// Transaction represents an active transaction
type Transaction struct {
    ID         string
    ProducerID int64
    Epoch      int16
    ExpiresAt  time.Time
}

// Begin begins a new transaction
func (tc *TransactionClient) Begin(ctx context.Context, producerID string) (*Transaction, error) {
    req := map[string]interface{}{
        "producer_id": producerID,
    }

    var txn Transaction
    err := tc.client.post(ctx, "/api/v1/transactions/begin", req, &txn)
    return &txn, err
}

// Commit commits a transaction
func (tc *TransactionClient) Commit(ctx context.Context, transactionID string) error {
    path := fmt.Sprintf("/api/v1/transactions/%s/commit", transactionID)
    return tc.client.post(ctx, path, nil, nil)
}

// Rollback rolls back a transaction
func (tc *TransactionClient) Rollback(ctx context.Context, transactionID string) error {
    path := fmt.Sprintf("/api/v1/transactions/%s/rollback", transactionID)
    return tc.client.post(ctx, path, nil, nil)
}

// GetStatus gets transaction status
func (tc *TransactionClient) GetStatus(ctx context.Context, transactionID string) (*TransactionStatus, error) {
    var status TransactionStatus
    path := fmt.Sprintf("/api/v1/transactions/%s/status", transactionID)
    err := tc.client.get(ctx, path, &status)
    return &status, err
}
```

### 2.1.6 Example Usage

```go
package main

import (
    "context"
    "log"

    "github.com/meftunca/portask/pkg/portask-client-go"
)

func main() {
    // Create client
    client, err := portask.NewClient("http://localhost:8080")
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()

    // Simple Producer
    result, err := client.Producer().Publish(ctx, portask.Message{
        Topic: "orders",
        Value: map[string]interface{}{
            "order_id": 123,
            "amount":   99.99,
        },
    })
    log.Printf("Published: %+v", result)

    // Batch Producer
    messages := []portask.Message{
        {Topic: "orders", Value: map[string]interface{}{"order_id": 1}},
        {Topic: "orders", Value: map[string]interface{}{"order_id": 2}},
    }
    results, _ := client.Producer().PublishBatch(ctx, messages)
    log.Printf("Batch published: %d messages", len(results))

    // Consumer Group
    group, _ := client.ConsumerGroup().Create(ctx, "my-group", []string{"orders"})
    log.Printf("Group created: %s", group.ID)

    joinResp, _ := client.ConsumerGroup().Join(ctx, group.ID, "client-1")
    log.Printf("Joined group as member: %s", joinResp.MemberID)

    // Fetch messages
    messages, _ := client.Consumer().Fetch(ctx, portask.ConsumeOptions{
        Topic:       "orders",
        MaxMessages: 10,
        GroupID:     group.ID,
        AutoCommit:  false,
    })

    for _, msg := range messages {
        log.Printf("Received: %+v", msg.Value)

        // Process message...

        // Acknowledge
        client.Consumer().Acknowledge(ctx, msg.MessageID)
    }

    // Get consumer lag
    lag, _ := client.ConsumerGroup().GetLag(ctx, group.ID)
    log.Printf("Total lag: %d", lag.TotalLag)
}
```

---

## 2.2 TypeScript Client Library

### File Structure

```
packages/portask-client-ts/
├── src/
│   ├── client.ts              # Main client
│   ├── producer.ts            # Producer operations
│   ├── consumer.ts            # Consumer operations
│   ├── consumer-group.ts      # Consumer group operations
│   ├── transaction.ts         # Transaction support
│   ├── websocket.ts           # WebSocket consumer
│   ├── types.ts               # TypeScript interfaces
│   └── index.ts               # Exports
├── examples/
│   ├── simple-producer.ts
│   ├── simple-consumer.ts
│   ├── consumer-group.ts
│   ├── websocket-consumer.ts
│   └── transactions.ts
├── package.json
├── tsconfig.json
└── README.md
```

### 2.2.1 Core Client (`client.ts`)

```typescript
// src/client.ts
export interface PortaskClientConfig {
  baseURL: string;
  apiKey?: string;
  timeout?: number;
}

export class PortaskClient {
  private baseURL: string;
  private apiKey?: string;
  private timeout: number;

  private _producer?: Producer;
  private _consumer?: Consumer;
  private _consumerGroup?: ConsumerGroupClient;
  private _transaction?: TransactionClient;

  constructor(config: PortaskClientConfig) {
    this.baseURL = config.baseURL;
    this.apiKey = config.apiKey;
    this.timeout = config.timeout || 30000;
  }

  get producer(): Producer {
    if (!this._producer) {
      this._producer = new Producer(this);
    }
    return this._producer;
  }

  get consumer(): Consumer {
    if (!this._consumer) {
      this._consumer = new Consumer(this);
    }
    return this._consumer;
  }

  get consumerGroup(): ConsumerGroupClient {
    if (!this._consumerGroup) {
      this._consumerGroup = new ConsumerGroupClient(this);
    }
    return this._consumerGroup;
  }

  get transaction(): TransactionClient {
    if (!this._transaction) {
      this._transaction = new TransactionClient(this);
    }
    return this._transaction;
  }

  async health(): Promise<HealthStatus> {
    return this.get<HealthStatus>("/health");
  }

  // Internal HTTP methods
  async get<T>(path: string): Promise<T> {
    const response = await fetch(`${this.baseURL}${path}`, {
      method: "GET",
      headers: this.getHeaders(),
    });

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${await response.text()}`);
    }

    return response.json();
  }

  async post<T>(path: string, body?: any): Promise<T> {
    const response = await fetch(`${this.baseURL}${path}`, {
      method: "POST",
      headers: this.getHeaders(),
      body: body ? JSON.stringify(body) : undefined,
    });

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${await response.text()}`);
    }

    return response.json();
  }

  async put<T>(path: string, body?: any): Promise<T> {
    const response = await fetch(`${this.baseURL}${path}`, {
      method: "PUT",
      headers: this.getHeaders(),
      body: body ? JSON.stringify(body) : undefined,
    });

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${await response.text()}`);
    }

    return response.json();
  }

  async delete(path: string): Promise<void> {
    const response = await fetch(`${this.baseURL}${path}`, {
      method: "DELETE",
      headers: this.getHeaders(),
    });

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${await response.text()}`);
    }
  }

  private getHeaders(): HeadersInit {
    const headers: HeadersInit = {
      "Content-Type": "application/json",
    };

    if (this.apiKey) {
      headers["Authorization"] = `Bearer ${this.apiKey}`;
    }

    return headers;
  }
}
```

### 2.2.2 Producer (`producer.ts`)

```typescript
// src/producer.ts
export interface Message {
  topic: string;
  key?: string;
  value: any;
  headers?: Record<string, any>;
  partition?: number;
  ttl?: number; // milliseconds
}

export interface ProduceResult {
  message_id: string;
  topic: string;
  partition: number;
  offset: number;
}

export class Producer {
  constructor(private client: PortaskClient) {}

  async publish(message: Message): Promise<ProduceResult> {
    return this.client.post<ProduceResult>("/api/v1/messages/publish", message);
  }

  async publishBatch(messages: Message[]): Promise<ProduceResult[]> {
    const response = await this.client.post<{ results: ProduceResult[] }>(
      "/api/v1/messages/batch/publish",
      { messages }
    );
    return response.results;
  }

  publishAsync(message: Message): void {
    // Fire-and-forget
    this.publish(message).catch(console.error);
  }
}
```

### 2.2.3 Consumer (`consumer.ts`)

```typescript
// src/consumer.ts
export interface ConsumeOptions {
  topic: string;
  partition?: number;
  startOffset?: number;
  maxMessages?: number;
  maxWaitMs?: number;
  groupId?: string;
  autoCommit?: boolean;
}

export interface FetchedMessage {
  message_id: string;
  topic: string;
  partition: number;
  offset: number;
  key?: string;
  value: any;
  headers?: Record<string, any>;
  timestamp: string;
  size: number;
}

export class Consumer {
  constructor(private client: PortaskClient) {}

  async fetch(options: ConsumeOptions): Promise<FetchedMessage[]> {
    const req = {
      topics: [
        {
          topic: options.topic,
          partitions: [
            {
              partition: options.partition || 0,
              fetch_offset: options.startOffset || 0,
            },
          ],
        },
      ],
      max_messages: options.maxMessages || 100,
      max_wait_ms: options.maxWaitMs || 1000,
    };

    const response = await this.client.post<any>(
      "/api/v1/messages/batch/fetch",
      req
    );

    // Flatten messages from response
    const messages: FetchedMessage[] = [];
    for (const topic of response.topics) {
      for (const partition of topic.partitions) {
        messages.push(...partition.messages);
      }
    }

    return messages;
  }

  subscribe(
    topics: string[],
    handler: (message: FetchedMessage) => Promise<void> | void
  ): WebSocketConsumer {
    return new WebSocketConsumer(this.client, topics, handler);
  }

  async acknowledge(messageId: string): Promise<void> {
    await this.client.post("/api/v1/messages/ack", {
      message_ids: [messageId],
    });
  }

  async acknowledgeBatch(messageIds: string[]): Promise<void> {
    await this.client.post("/api/v1/messages/batch/ack", {
      message_ids: messageIds,
    });
  }
}
```

### 2.2.4 WebSocket Consumer (`websocket.ts`)

```typescript
// src/websocket.ts
export type MessageHandler = (message: FetchedMessage) => Promise<void> | void;

export class WebSocketConsumer {
  private ws?: WebSocket;
  private reconnectTimeout?: NodeJS.Timeout;
  private isClosing = false;

  constructor(
    private client: PortaskClient,
    private topics: string[],
    private handler: MessageHandler
  ) {}

  connect(options: { groupId?: string; autoCommit?: boolean } = {}): void {
    const wsUrl = this.client.baseURL.replace("http", "ws");
    this.ws = new WebSocket(`${wsUrl}/api/v1/messages/subscribe`);

    this.ws.onopen = () => {
      console.log("[Portask WS] Connected");

      // Send subscribe request
      this.ws!.send(
        JSON.stringify({
          topics: this.topics,
          group_id: options.groupId,
          auto_commit: options.autoCommit !== false,
        })
      );
    };

    this.ws.onmessage = async (event) => {
      try {
        const msg = JSON.parse(event.data);

        if (msg.type === "message") {
          await this.handler(msg as FetchedMessage);

          // Send ack if not auto-commit
          if (options.autoCommit === false) {
            this.ws!.send(
              JSON.stringify({
                type: "ack",
                message_id: msg.message_id,
              })
            );
          }
        }
      } catch (err) {
        console.error("[Portask WS] Error handling message:", err);
      }
    };

    this.ws.onclose = (event) => {
      console.log("[Portask WS] Disconnected:", event.code, event.reason);

      // Reconnect after delay (unless explicitly closed)
      if (!this.isClosing) {
        this.reconnectTimeout = setTimeout(() => {
          console.log("[Portask WS] Reconnecting...");
          this.connect(options);
        }, 3000);
      }
    };

    this.ws.onerror = (error) => {
      console.error("[Portask WS] Error:", error);
    };
  }

  close(): void {
    this.isClosing = true;
    if (this.reconnectTimeout) {
      clearTimeout(this.reconnectTimeout);
    }
    if (this.ws) {
      this.ws.close();
    }
  }
}
```

### 2.2.5 Consumer Group (`consumer-group.ts`)

```typescript
// src/consumer-group.ts
export interface ConsumerGroup {
  id: string;
  name: string;
  state: string;
  protocol: string;
  protocol_type: string;
  leader: string;
  generation: number;
  members: GroupMember[];
  subscriptions: string[];
  created_at: string;
  updated_at: string;
}

export interface GroupMember {
  id: string;
  client_id: string;
  client_host: string;
  session_timeout: number;
  assignment: PartitionAssignment[];
  joined_at: string;
  last_heartbeat: string;
}

export interface PartitionAssignment {
  topic: string;
  partitions: number[];
}

export interface GroupLag {
  group_id: string;
  total_lag: number;
  partitions: Array<{
    topic: string;
    partition: number;
    current_offset: number;
    log_end_offset: number;
    lag: number;
  }>;
}

export class ConsumerGroupClient {
  constructor(private client: PortaskClient) {}

  async create(name: string, topics: string[]): Promise<ConsumerGroup> {
    return this.client.post<ConsumerGroup>("/api/v1/consumer-groups", {
      name,
      topics,
    });
  }

  async list(): Promise<ConsumerGroup[]> {
    const response = await this.client.get<{ groups: ConsumerGroup[] }>(
      "/api/v1/consumer-groups"
    );
    return response.groups;
  }

  async get(groupId: string): Promise<ConsumerGroup> {
    return this.client.get<ConsumerGroup>(`/api/v1/consumer-groups/${groupId}`);
  }

  async join(
    groupId: string,
    clientId: string
  ): Promise<{ member_id: string; generation: number }> {
    return this.client.post(`/api/v1/consumer-groups/${groupId}/join`, {
      client_id: clientId,
    });
  }

  async leave(groupId: string, memberId: string): Promise<void> {
    await this.client.post(`/api/v1/consumer-groups/${groupId}/leave`, {
      member_id: memberId,
    });
  }

  async commitOffset(
    groupId: string,
    topic: string,
    partition: number,
    offset: number
  ): Promise<void> {
    await this.client.post(
      `/api/v1/consumer-groups/${groupId}/offsets/commit`,
      { topic, partition, offset }
    );
  }

  async fetchOffsets(
    groupId: string
  ): Promise<Record<string, Record<number, number>>> {
    const response = await this.client.get<{
      offsets: Record<string, Record<number, number>>;
    }>(`/api/v1/consumer-groups/${groupId}/offsets`);
    return response.offsets;
  }

  async getLag(groupId: string): Promise<GroupLag> {
    return this.client.get<GroupLag>(`/api/v1/consumer-groups/${groupId}/lag`);
  }

  async delete(groupId: string): Promise<void> {
    await this.client.delete(`/api/v1/consumer-groups/${groupId}`);
  }
}
```

### 2.2.6 Example Usage (TypeScript)

```typescript
import { PortaskClient } from "portask-client-ts";

// Create client
const client = new PortaskClient({
  baseURL: "http://localhost:8080",
});

// Simple Producer
const result = await client.producer.publish({
  topic: "orders",
  value: { order_id: 123, amount: 99.99 },
});
console.log("Published:", result);

// Batch Producer
const results = await client.producer.publishBatch([
  { topic: "orders", value: { order_id: 1 } },
  { topic: "orders", value: { order_id: 2 } },
]);
console.log("Batch published:", results.length);

// Consumer Group
const group = await client.consumerGroup.create("my-group", ["orders"]);
console.log("Group created:", group.id);

const joinResp = await client.consumerGroup.join(group.id, "client-1");
console.log("Joined as member:", joinResp.member_id);

// Fetch messages
const messages = await client.consumer.fetch({
  topic: "orders",
  maxMessages: 10,
  groupId: group.id,
  autoCommit: false,
});

for (const msg of messages) {
  console.log("Received:", msg.value);

  // Process message...

  // Acknowledge
  await client.consumer.acknowledge(msg.message_id);
}

// WebSocket Consumer
const wsConsumer = client.consumer.subscribe(["orders"], async (msg) => {
  console.log("WS Received:", msg.value);
  // Process message...
});

wsConsumer.connect({ groupId: "my-group", autoCommit: false });

// Later: close connection
wsConsumer.close();

// Get consumer lag
const lag = await client.consumerGroup.getLag(group.id);
console.log("Total lag:", lag.total_lag);
```

---

# 🎨 PHASE 3: Admin UI Refactoring

## 3.1 New Dashboard Structure

### Old Structure (v1.0 - Translator-Centric)

```
Admin UI/
├─ Dashboard            (Generic metrics)
├─ Kafka Dashboard      ❌ Kafka as primary
├─ AMQP Dashboard       ❌ RabbitMQ as primary
├─ Consumer Groups      (Labeled as "Kafka Consumer Groups")
├─ Messages
├─ Monitoring
└─ Settings
```

### New Structure (v2.0 - Portask-Centric)

```
Admin UI/
├─ Dashboard ⭐         (Portask-centric, main landing page)
│  ├─ Topics & Partitions
│  ├─ Consumer Groups (Native Portask)
│  ├─ Message Flow & Throughput
│  ├─ Storage Backends
│  └─ Worker Pools
│
├─ Topics               (Unified topic management)
├─ Consumer Groups      (Native Portask, no "Kafka" label)
├─ Messages             (Browse/search messages)
├─ Producers            (Active producers, publish UI)
├─ Consumers            (Active consumers, subscribe UI)
│
├─ Protocol Compatibility (Collapsible section)
│  ├─ Kafka Protocol Stats
│  └─ AMQP Protocol Stats
│
├─ Monitoring           (Performance & metrics)
└─ Settings
```

## 3.2 Dashboard Refactoring

### File: `admin_ui/src/pages/PortaskDashboard.tsx` (NEW)

```typescript
// This replaces the current Dashboard.tsx

import React, { useEffect, useState } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Activity,
  Database,
  MessageSquare,
  Server,
  Users,
  TrendingUp,
  Layers,
  HardDrive,
} from "lucide-react";
import { apiBase } from "@/lib/api";
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  Line,
  LineChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

interface PortaskMetrics {
  topics: number;
  partitions: number;
  consumer_groups: number;
  active_consumers: number;
  active_producers: number;
  messages_total: number;
  messages_per_sec: number;
  throughput_bytes_per_sec: number;
  storage_backend: string;
  storage_used_gb: number;
  worker_pools: {
    total: number;
    active: number;
    idle: number;
  };
}

export default function PortaskDashboard() {
  const [metrics, setMetrics] = useState<PortaskMetrics | null>(null);
  const [loading, setLoading] = useState(true);

  const [throughputData, setThroughputData] = useState<
    Array<{
      time: string;
      messages: number;
      bytes_kb: number;
    }>
  >([]);

  const [topicDistribution, setTopicDistribution] = useState<
    Array<{
      name: string;
      messages: number;
      partitions: number;
    }>
  >([]);

  useEffect(() => {
    fetchMetrics();
    const interval = setInterval(fetchMetrics, 5000);
    return () => clearInterval(interval);
  }, []);

  const fetchMetrics = async () => {
    try {
      const response = await apiBase.get("/metrics");
      const data = response.data;

      // Transform data to Portask-centric metrics
      const newMetrics: PortaskMetrics = {
        topics: data.storage?.topic_count || 0,
        partitions: data.storage?.partition_count || 0,
        consumer_groups: data.kafka?.consumer_groups || 0,
        active_consumers: data.consumers?.active || 0,
        active_producers: data.producers?.active || 0,
        messages_total: data.storage?.total_messages || 0,
        messages_per_sec: data.core?.messages_rate || 0,
        throughput_bytes_per_sec: data.core?.bytes_rate || 0,
        storage_backend: data.storage?.backend || "unknown",
        storage_used_gb: (data.storage?.storage_used_bytes || 0) / 1024 ** 3,
        worker_pools: {
          total: data.workers?.total || 0,
          active: data.workers?.active || 0,
          idle: data.workers?.idle || 0,
        },
      };

      setMetrics(newMetrics);

      // Update charts
      const now = new Date().toLocaleTimeString();
      setThroughputData((prev) => {
        const newData = [
          ...prev,
          {
            time: now,
            messages: newMetrics.messages_per_sec,
            bytes_kb: Math.round(newMetrics.throughput_bytes_per_sec / 1024),
          },
        ];
        return newData.slice(-20); // Keep last 20 points
      });

      setLoading(false);
    } catch (error) {
      console.error("Failed to fetch metrics:", error);
      setLoading(false);
    }
  };

  if (loading || !metrics) {
    return <div>Loading...</div>;
  }

  return (
    <div className="flex-1 space-y-4 p-4 md:p-8 pt-6">
      <div className="flex items-center justify-between space-y-2">
        <h2 className="text-3xl font-bold tracking-tight">Portask Dashboard</h2>
        <Badge variant="outline" className="text-green-600 border-green-600">
          v2.0 - Native API
        </Badge>
      </div>

      {/* Core Metrics Cards */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Topics</CardTitle>
            <Layers className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{metrics.topics}</div>
            <p className="text-xs text-muted-foreground">
              {metrics.partitions} total partitions
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              Consumer Groups
            </CardTitle>
            <Users className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{metrics.consumer_groups}</div>
            <p className="text-xs text-muted-foreground">
              {metrics.active_consumers} active consumers
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              Messages Total
            </CardTitle>
            <MessageSquare className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {(metrics.messages_total / 1000).toFixed(1)}K
            </div>
            <p className="text-xs text-muted-foreground">
              {metrics.messages_per_sec.toLocaleString()} msgs/sec
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Storage</CardTitle>
            <HardDrive className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {metrics.storage_used_gb.toFixed(2)} GB
            </div>
            <p className="text-xs text-muted-foreground">
              Backend: {metrics.storage_backend}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Charts */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-7">
        <Card className="col-span-4">
          <CardHeader>
            <CardTitle>Message Throughput</CardTitle>
            <CardDescription>
              Real-time message flow (msgs/sec & KB/sec)
            </CardDescription>
          </CardHeader>
          <CardContent className="pl-2">
            <ResponsiveContainer width="100%" height={300}>
              <AreaChart data={throughputData}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="time" />
                <YAxis />
                <Tooltip />
                <Area
                  type="monotone"
                  dataKey="messages"
                  stroke="#8884d8"
                  fill="#8884d8"
                  fillOpacity={0.6}
                  name="Messages/sec"
                />
                <Area
                  type="monotone"
                  dataKey="bytes_kb"
                  stroke="#82ca9d"
                  fill="#82ca9d"
                  fillOpacity={0.6}
                  name="KB/sec"
                />
              </AreaChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>

        <Card className="col-span-3">
          <CardHeader>
            <CardTitle>Worker Pools</CardTitle>
            <CardDescription>Active vs. Idle workers</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <span className="text-sm font-medium">Total Workers</span>
                <span className="text-2xl font-bold">
                  {metrics.worker_pools.total}
                </span>
              </div>
              <div className="space-y-2">
                <div className="flex items-center justify-between text-sm">
                  <span>Active</span>
                  <span className="font-medium text-green-600">
                    {metrics.worker_pools.active}
                  </span>
                </div>
                <div className="w-full bg-gray-200 rounded-full h-2.5">
                  <div
                    className="bg-green-600 h-2.5 rounded-full"
                    style={{
                      width: `${
                        (metrics.worker_pools.active /
                          metrics.worker_pools.total) *
                        100
                      }%`,
                    }}
                  />
                </div>
              </div>
              <div className="space-y-2">
                <div className="flex items-center justify-between text-sm">
                  <span>Idle</span>
                  <span className="font-medium text-gray-600">
                    {metrics.worker_pools.idle}
                  </span>
                </div>
                <div className="w-full bg-gray-200 rounded-full h-2.5">
                  <div
                    className="bg-gray-600 h-2.5 rounded-full"
                    style={{
                      width: `${
                        (metrics.worker_pools.idle /
                          metrics.worker_pools.total) *
                        100
                      }%`,
                    }}
                  />
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Protocol Compatibility Section (Collapsible) */}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">Protocol Compatibility</CardTitle>
          <CardDescription>
            Portask supports Kafka and AMQP protocols for easy migration
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="border rounded-lg p-4">
              <h3 className="font-semibold mb-2">Kafka Protocol</h3>
              <p className="text-sm text-muted-foreground mb-3">
                Wire protocol compatible with Kafka clients
              </p>
              <Button variant="outline" size="sm" asChild>
                <a href="/kafka">View Kafka Stats →</a>
              </Button>
            </div>
            <div className="border rounded-lg p-4">
              <h3 className="font-semibold mb-2">AMQP Protocol</h3>
              <p className="text-sm text-muted-foreground mb-3">
                Compatible with RabbitMQ clients
              </p>
              <Button variant="outline" size="sm" asChild>
                <a href="/amqp">View AMQP Stats →</a>
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
```

## 3.3 Unified Navigation

### File: `admin_ui/src/components/layout/Layout.tsx`

```typescript
// Update navigation items

const navigation = [
  { name: "Dashboard", href: "/", icon: Home }, // Portask Dashboard
  { name: "Topics", href: "/topics", icon: Layers },
  { name: "Consumer Groups", href: "/consumer-groups", icon: Users },
  { name: "Messages", href: "/messages", icon: MessageSquare },
  { name: "Producers", href: "/producers", icon: Upload },
  { name: "Consumers", href: "/consumers", icon: Download },

  // Collapsible "Protocol Compatibility" section
  {
    name: "Protocol Compatibility",
    icon: Network,
    children: [
      { name: "Kafka Stats", href: "/kafka", icon: Zap },
      { name: "AMQP Stats", href: "/amqp", icon: Rabbit },
    ],
  },

  { name: "Monitoring", href: "/monitoring", icon: Activity },
  { name: "Settings", href: "/settings", icon: Settings },
];
```

## 3.4 Consumer Groups Page Refactoring

### File: `admin_ui/src/pages/ConsumerGroups.tsx` (Updated)

```typescript
// Remove "Kafka" label from title
<h2 className="text-3xl font-bold tracking-tight">Consumer Groups</h2>
// Instead of:
// <h2 className="text-3xl font-bold tracking-tight">Kafka Consumer Groups</h2>

// Update description
<p className="text-muted-foreground">
  Manage Portask consumer groups and monitor partition assignments
</p>
// Instead of:
// <p className="text-muted-foreground">
//   Manage Kafka consumer groups...
// </p>

// Use native API endpoints
const fetchConsumerGroups = async () => {
  const response = await apiBase.get('/api/v1/consumer-groups');
  // Native Portask API, not Kafka-specific
};
```

---

## 3.5 New Pages

### File: `admin_ui/src/pages/Producers.tsx` (NEW)

```typescript
// List active producers, publish messages directly from UI
// Includes:
// - List of active producer connections
// - "Publish Message" form (single and batch)
// - Producer statistics (throughput, errors)
```

### File: `admin_ui/src/pages/Consumers.tsx` (NEW)

```typescript
// List active consumers, subscribe to topics from UI
// Includes:
// - List of active consumer connections
// - "Subscribe to Topic" form (WebSocket)
// - Live message viewer
// - Consumer statistics (throughput, lag)
```

---

## Summary: Development Timeline

| Phase                | Task                      | Duration | Dependency |
| -------------------- | ------------------------- | -------- | ---------- |
| **Phase 1: Backend** | Consumer Groups API       | 2 days   | -          |
|                      | Batch Operations API      | 2 days   | -          |
|                      | WebSocket Subscribe       | 2 days   | -          |
|                      | Transaction API           | 2 days   | -          |
|                      | Extended Topic Management | 1 day    | -          |
|                      | Message Acknowledgment    | 1 day    | -          |
|                      | Testing & Documentation   | 2 days   | All above  |
| **Phase 2: Clients** | Go Client Library         | 3 days   | Phase 1    |
|                      | TypeScript Client Library | 3 days   | Phase 1    |
|                      | Client Examples & Docs    | 2 days   | Above      |
| **Phase 3: UI**      | New Dashboard             | 2 days   | Phase 1    |
|                      | Navigation Refactoring    | 1 day    | -          |
|                      | Consumer Groups Update    | 1 day    | Phase 1    |
|                      | New Producers Page        | 2 days   | Phase 1    |
|                      | New Consumers Page        | 2 days   | Phase 1    |
|                      | Protocol Stats Pages      | 1 day    | -          |
|                      | Testing & Polish          | 2 days   | All above  |

**Total Estimated Time: 6 weeks (30 working days)**

---

## Success Criteria for v2.0

### Backend

- ✅ All core features exposed via native REST API
- ✅ Consumer groups manageable via `/api/v1/consumer-groups`
- ✅ Batch operations support 1000+ messages per request
- ✅ WebSocket consumers can subscribe to topics
- ✅ Transactions work across multiple topics/partitions
- ✅ API documentation complete with OpenAPI spec

### Client Libraries

- ✅ Go client published to GitHub with examples
- ✅ TypeScript client published to npm with examples
- ✅ Both clients achieve 100% native API coverage
- ✅ Documentation includes migration guides

### Admin UI

- ✅ "Portask Dashboard" is the default landing page
- ✅ No "Kafka" or "AMQP" labels on core features
- ✅ Protocol compatibility shown as secondary feature
- ✅ Real-time WebSocket updates working
- ✅ Message publish/subscribe from UI functional

### Marketing & Positioning

- ✅ README.md positions Portask as platform, not translator
- ✅ Documentation structure reflects native-first approach
- ✅ Performance comparisons highlight Portask advantages
- ✅ Migration guides show how to move FROM Kafka/RabbitMQ TO Portask

---

**Next Steps:**

1. ✅ Review and approve this roadmap
2. ⬜ Create GitHub issues for each phase
3. ⬜ Set up project board for tracking
4. ⬜ Begin Phase 1: Backend Native API
