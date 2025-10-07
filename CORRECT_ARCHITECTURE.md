# ✅ Correct Architecture: Portask as Central Controller

## Core Principle

**Portask is the ONLY system that writes, reads, and deletes data.**

Kafka and RabbitMQ APIs are **protocol translators ONLY** - they convert external protocols to Portask's native protocol.

---

## The Right Way

```
┌────────────────────────────────────────────────────────────────┐
│  UNIFIED ARCHITECTURE (Correct)                                │
├────────────────────────────────────────────────────────────────┤
│                                                                │
│  Kafka Client                                                  │
│       ↓ (Kafka Wire Protocol)                                  │
│  ┌─────────────────────┐                                       │
│  │  Kafka Translator   │  Convert Kafka → Portask             │
│  └─────────────────────┘                                       │
│       ↓ (Portask Protocol)                                     │
│                                                                │
│  RabbitMQ Client                                               │
│       ↓ (AMQP Protocol)                                        │
│  ┌─────────────────────┐                                       │
│  │  AMQP Translator    │  Convert AMQP → Portask              │
│  └─────────────────────┘                                       │
│       ↓ (Portask Protocol)                                     │
│                                                                │
│  Native Client                                                 │
│       ↓ (Portask Protocol)                                     │
│  ┌─────────────────────┐                                       │
│  │  No Translation     │  Already Portask                     │
│  └─────────────────────┘                                       │
│       ↓                                                        │
│                                                                │
│  ╔══════════════════════════════════════════╗                 │
│  ║  PORTASK CORE (Central Controller)       ║                 │
│  ╠══════════════════════════════════════════╣                 │
│  ║  • Protocol validation                   ║                 │
│  ║  • CRC32 checksum                        ║                 │
│  ║  • Serialization (CodecManager)          ║                 │
│  ║  • Business logic                        ║                 │
│  ║  • Metrics & monitoring                  ║                 │
│  ║  • Rate limiting                         ║                 │
│  ║  • Authentication                        ║                 │
│  ║  • Storage operations                    ║                 │
│  ╚══════════════════════════════════════════╝                 │
│       ↓                                                        │
│  ┌─────────────┐                                              │
│  │  Dragonfly  │  ONLY Portask writes here                    │
│  └─────────────┘                                              │
│                                                                │
└────────────────────────────────────────────────────────────────┘
```

---

## Implementation Strategy

### Phase 1: Define Translator Interface

```go
// pkg/protocol/translator.go
package protocol

import (
    "github.com/meftunca/portask/pkg/types"
)

// ProtocolTranslator converts external protocols to Portask protocol
type ProtocolTranslator interface {
    // Translate external message to Portask message
    Translate(externalMsg interface{}) (*types.PortaskMessage, error)

    // Translate Portask response to external protocol
    TranslateResponse(portaskResp *types.PortaskResponse) (interface{}, error)
}
```

### Phase 2: Kafka Translator

```go
// pkg/kafka/translator.go
package kafka

import (
    "fmt"
    "time"

    "github.com/meftunca/portask/pkg/protocol"
    "github.com/meftunca/portask/pkg/types"
)

type KafkaTranslator struct {
    // No storage, no business logic!
    // Just translation
}

func NewKafkaTranslator() *KafkaTranslator {
    return &KafkaTranslator{}
}

// Translate Kafka Produce request to Portask message
func (t *KafkaTranslator) TranslateProduce(
    topic string,
    partition int32,
    key []byte,
    value []byte,
) (*types.PortaskMessage, error) {

    return &types.PortaskMessage{
        ID:        types.MessageID(fmt.Sprintf("kafka-%d", time.Now().UnixNano())),
        Topic:     types.TopicName(topic),
        Partition: partition,
        Key:       string(key),
        Payload:   value,
        Timestamp: time.Now().UnixNano(),
        TTL:       0, // Use default
        Metadata: map[string]string{
            "source": "kafka",
            "protocol": "kafka-wire",
        },
    }, nil
}

// Translate Portask response to Kafka Produce response
func (t *KafkaTranslator) TranslateProduceResponse(
    offset int64,
    err error,
) *KafkaProduceResponse {

    if err != nil {
        return &KafkaProduceResponse{
            ErrorCode: UnknownError,
            Offset:    -1,
        }
    }

    return &KafkaProduceResponse{
        ErrorCode: NoError,
        Offset:    offset,
    }
}

// Translate Kafka Fetch request to Portask fetch
func (t *KafkaTranslator) TranslateFetch(
    topic string,
    partition int32,
    offset int64,
    maxBytes int32,
) (*types.FetchRequest, error) {

    return &types.FetchRequest{
        Topic:     types.TopicName(topic),
        Partition: partition,
        Offset:    offset,
        Limit:     int(maxBytes / 1024), // Approximate
    }, nil
}
```

### Phase 3: RabbitMQ Translator

```go
// pkg/amqp/translator.go
package amqp

import (
    "fmt"
    "time"

    "github.com/meftunca/portask/pkg/types"
)

type AMQPTranslator struct {
    // Just translation, no business logic
}

func NewAMQPTranslator() *AMQPTranslator {
    return &AMQPTranslator{}
}

// Translate AMQP Basic.Publish to Portask message
func (t *AMQPTranslator) TranslatePublish(
    exchange string,
    routingKey string,
    body []byte,
    properties map[string]interface{},
) (*types.PortaskMessage, error) {

    // Convert AMQP routing to Portask topic/partition
    topic := exchange
    if topic == "" {
        topic = "default"
    }

    return &types.PortaskMessage{
        ID:        types.MessageID(fmt.Sprintf("amqp-%d", time.Now().UnixNano())),
        Topic:     types.TopicName(topic),
        Partition: 0, // AMQP doesn't have partitions
        Key:       routingKey,
        Payload:   body,
        Timestamp: time.Now().UnixNano(),
        TTL:       getMessageTTL(properties),
        Metadata: map[string]string{
            "source":      "amqp",
            "protocol":    "amqp-0-9-1",
            "exchange":    exchange,
            "routingKey":  routingKey,
        },
    }, nil
}

// Translate Portask message to AMQP delivery
func (t *AMQPTranslator) TranslateDeliver(
    portaskMsg *types.PortaskMessage,
) (*AMQPDelivery, error) {

    return &AMQPDelivery{
        ConsumerTag:  "portask-consumer",
        DeliveryTag:  uint64(portaskMsg.Timestamp),
        Redelivered:  false,
        Exchange:     string(portaskMsg.Topic),
        RoutingKey:   portaskMsg.Key,
        Body:         portaskMsg.Payload,
        Timestamp:    time.Unix(0, portaskMsg.Timestamp),
    }, nil
}
```

### Phase 4: Refactor Handlers

**Before (Wrong):**

```go
// pkg/kafka/handlers.go
func (h *KafkaProtocolHandler) handleProduce(request *KafkaRequest) []byte {
    // ❌ Direct storage write
    offset, err := h.messageStore.ProduceMessage(topic, partition, key, value)
    // ...
}
```

**After (Correct):**

```go
// pkg/kafka/handlers.go
func (h *KafkaProtocolHandler) handleProduce(request *KafkaRequest) []byte {
    // Parse Kafka request
    topic, partition, key, value := parseKafkaProduceRequest(request)

    // ✅ Translate to Portask message
    portaskMsg, err := h.translator.TranslateProduce(topic, partition, key, value)
    if err != nil {
        return h.buildErrorResponse(err)
    }

    // ✅ Send to Portask core for processing
    offset, err := h.portaskCore.ProcessMessage(portaskMsg)
    if err != nil {
        return h.translator.TranslateProduceResponse(0, err)
    }

    // ✅ Translate response back to Kafka format
    response := h.translator.TranslateProduceResponse(offset, nil)
    return h.encodeKafkaResponse(response)
}
```

---

## New Architecture Components

### 1. Protocol Layer (`pkg/protocol/`)

```
pkg/protocol/
├── translator.go       # Translator interface
├── portask.go          # Native Portask protocol
└── registry.go         # Register translators
```

### 2. Kafka Layer (`pkg/kafka/`)

```
pkg/kafka/
├── translator.go       # Kafka → Portask translation
├── server.go           # TCP server (no business logic)
├── handlers.go         # Handle Kafka wire protocol
└── protocol.go         # Kafka wire format definitions
```

### 3. AMQP Layer (`pkg/amqp/`)

```
pkg/amqp/
├── translator.go       # AMQP → Portask translation
├── server.go           # AMQP server
├── handlers.go         # Handle AMQP commands
└── protocol.go         # AMQP frame definitions
```

### 4. Portask Core (`pkg/core/`)

```
pkg/core/
├── processor.go        # Central message processor
├── validator.go        # Protocol validation
├── storage.go          # Storage orchestration
└── metrics.go          # Unified metrics
```

---

## Benefits of This Architecture

### 1. **Single Source of Truth**

```
❌ Before: 3 different write paths
✅ After:  1 central write path (Portask)
```

### 2. **Easier Maintenance**

```
Bug fix needed:
❌ Before: Fix in Kafka, AMQP, and Native handlers
✅ After:  Fix ONCE in Portask core
```

### 3. **Feature Consistency**

```
New feature (e.g., encryption):
❌ Before: Implement 3 times
✅ After:  Implement ONCE, all protocols get it
```

### 4. **Unified Metrics**

```go
// ALL protocols tracked in one place
portask.metrics.totalMessages++
portask.metrics.avgProcessTime = ...
portask.metrics.byProtocol["kafka"]++
portask.metrics.byProtocol["amqp"]++
portask.metrics.byProtocol["native"]++
```

### 5. **Protocol Validation**

```
Every message, regardless of source:
✅ CRC32 checksum
✅ Protocol version check
✅ Magic number validation
✅ Size limits
✅ Rate limiting
```

---

## Implementation Plan

### Step 1: Create Core Processor (1 day)

```go
// pkg/core/processor.go
type MessageProcessor struct {
    validator    *Validator
    codecManager *serialization.CodecManager
    storage      storage.MessageStore
    metrics      *Metrics
}

func (p *MessageProcessor) ProcessMessage(msg *types.PortaskMessage) (int64, error) {
    // 1. Validate protocol
    if err := p.validator.Validate(msg); err != nil {
        p.metrics.validationErrors++
        return 0, err
    }

    // 2. Encode with CodecManager
    encoded, err := p.codecManager.Encode(msg)
    if err != nil {
        p.metrics.encodingErrors++
        return 0, err
    }

    // 3. Add protocol frame (magic, version, CRC32)
    frame := p.buildProtocolFrame(encoded)

    // 4. Store
    offset, err := p.storage.Store(frame)
    if err != nil {
        p.metrics.storageErrors++
        return 0, err
    }

    // 5. Update metrics
    p.metrics.totalMessages++
    p.metrics.UpdateProcessTime(time.Since(start))

    return offset, nil
}
```

### Step 2: Create Translators (2 days)

- `pkg/kafka/translator.go` - Kafka translation
- `pkg/amqp/translator.go` - AMQP translation
- Tests for each translator

### Step 3: Refactor Handlers (2 days)

- Update `pkg/kafka/handlers.go` to use translator + core
- Update `pkg/amqp/handlers.go` to use translator + core
- Remove direct storage access

### Step 4: Integration Tests (1 day)

```go
func TestKafkaUsesPortaskCore(t *testing.T) {
    // Send via Kafka
    kafkaClient.Produce("test", []byte("data"))

    // Verify Portask protocol used
    stored := storage.Get(...)
    assert.Contains(stored, PortaskMagicNumber)
    assert.Contains(stored, CRC32)

    // Verify metrics
    assert.Equal(1, portask.metrics.totalMessages)
    assert.Equal(1, portask.metrics.byProtocol["kafka"])
}

func TestAMQPUsesPortaskCore(t *testing.T) {
    // Send via AMQP
    amqpClient.Publish("exchange", "key", []byte("data"))

    // Verify same protocol
    stored := storage.Get(...)
    assert.Contains(stored, PortaskMagicNumber)
    assert.Contains(stored, CRC32)

    // Verify metrics
    assert.Equal(1, portask.metrics.totalMessages)
    assert.Equal(1, portask.metrics.byProtocol["amqp"])
}
```

### Step 5: Documentation (1 day)

- Update architecture docs
- Add translator examples
- Update API docs

**Total: 7 days (1 sprint)**

---

## Migration Strategy

### Phase 1: Create New Components (Non-breaking)

1. Create `pkg/core/processor.go`
2. Create translators
3. Keep old code working

### Phase 2: Switch Kafka API

1. Update Kafka handlers to use translator + core
2. Run A/B test
3. Verify metrics
4. Deploy

### Phase 3: Switch AMQP API

1. Update AMQP handlers to use translator + core
2. Run A/B test
3. Verify metrics
4. Deploy

### Phase 4: Remove Old Code

1. Delete direct storage access from Kafka
2. Delete direct storage access from AMQP
3. Cleanup

---

## Monitoring During Migration

```go
// Track both paths during migration
type MigrationMetrics struct {
    // Old path
    oldPathMessages   int64
    oldPathLatency    time.Duration
    oldPathErrors     int64

    // New path (via Portask core)
    newPathMessages   int64
    newPathLatency    time.Duration
    newPathErrors     int64

    // Validation
    consistencyChecks int64
    inconsistencies   int64
}
```

---

## Final Architecture

```
┌─────────────────────────────────────────────────────────┐
│  CLEAN, MAINTAINABLE ARCHITECTURE                       │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  Protocol APIs (Thin Layer):                            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐             │
│  │  Kafka   │  │   AMQP   │  │  Native  │             │
│  │  (Wire)  │  │  (Wire)  │  │  (Wire)  │             │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘             │
│       │             │             │                     │
│       ↓             ↓             ↓                     │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐             │
│  │ Kafka    │  │  AMQP    │  │   No     │             │
│  │Translator│  │Translator│  │Translation│             │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘             │
│       │             │             │                     │
│       └──────────┬──┴─────────────┘                     │
│                  ↓                                       │
│  ╔═══════════════════════════════════════════╗         │
│  ║  PORTASK CORE (Single Controller)        ║         │
│  ╠═══════════════════════════════════════════╣         │
│  ║  • Validation                             ║         │
│  ║  • Serialization                          ║         │
│  ║  • Protocol framing (CRC32)               ║         │
│  ║  • Business logic                         ║         │
│  ║  • Metrics (unified)                      ║         │
│  ║  • Auth, rate limiting                    ║         │
│  ║  • Storage orchestration                  ║         │
│  ╚═════════════════╤═════════════════════════╝         │
│                    ↓                                     │
│               ┌─────────┐                               │
│               │Dragonfly│                               │
│               └─────────┘                               │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

---

## Summary

### What We're Fixing

❌ **Before:** Kafka and AMQP do their own thing

- Direct storage access
- No protocol validation
- Duplicate business logic
- Inconsistent metrics

✅ **After:** Portask is the boss

- Protocol translators ONLY translate
- ALL logic in Portask core
- Single write path
- Unified metrics

### Key Principle

> **Kafka and RabbitMQ are just different languages speaking to the same system (Portask). They translate their requests to Portask's language, and Portask does all the work.**

---

**Status:** Architecture redesign required  
**Priority:** High  
**Effort:** 1 sprint (7 days)  
**Impact:** Massive improvement in maintainability and consistency
