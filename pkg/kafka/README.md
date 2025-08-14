# kafka/ (Kafka Protocol Handler and Emulation)

This package provides Kafka protocol compatibility and emulation for Portask, including protocol handlers, server logic, and compatibility notes.

## Main Files
- `handlers.go`: Kafka protocol handlers
- `missing_compatibility.md`: Compatibility notes
- `protocol.go`: Protocol implementation
- `server.go`: Kafka server logic

## Key Types, Interfaces, and Methods

### Interfaces
- `MessageStore` — Interface for message storage (Kafka compatibility)
- `AuthProvider` — Interface for authentication
- `MetricsCollector` — Interface for metrics

### Structs & Types
- `KafkaProtocolHandler` — Main protocol handler
- `RequestHeader`, `ResponseHeader`, `Message`, `TopicMetadata`, `PartitionMetadata`, `User`, `KafkaRequest`, `KafkaResponse`, `ConsumerGroupState`, `ConsumerGroupMember`, `InMemoryMessageStore`, `InMemoryTopic`, `KafkaServer`, `SimpleAuthProvider`, `SimpleMetricsCollector` — Kafka protocol and server types

### Main Methods (selected)
- `NewKafkaProtocolHandler(store, auth, metrics) *KafkaProtocolHandler`, `HandleConnection()`, `readRequest()`, `writeResponse()`, `handleRequest()` (KafkaProtocolHandler)
- `handleApiVersions()`, `handleMetadata()`, `handleProduce()`, `handleFetch()`, `handleListOffsets()`, `handleCreateTopics()`, `handleDeleteTopics()`, `handleSaslHandshake()`, `handleSaslAuthenticate()` (KafkaProtocolHandler)
- `NewKafkaServer(addr, store) *KafkaServer`, `Start()`, `Stop()` (KafkaServer)
- `Authenticate()`, `Authorize()` (SimpleAuthProvider)
- `IncrementRequestCount()`, `RecordRequestLatency()`, `RecordBytesIn()`, `RecordBytesOut()` (SimpleMetricsCollector)

## Usage Example
```go
import "github.com/meftunca/portask/pkg/kafka"

server := kafka.NewKafkaServer(":9092", nil)
err := server.Start()
```

## TODO / Missing Features
- [ ] Advanced Kafka protocol support
- [ ] Complete compatibility tests
- [ ] Schema registry emulation
- [ ] Partition reassignment

---

For details, see the code and comments in each file.
