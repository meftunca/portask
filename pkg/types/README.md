# types/ (Core Data Types, Error Handling, ULID, Message Structure)

This package provides core data types, error handling, ULID generation, and message structure definitions for Portask.

## Main Files
- `errors.go`: Error handling and error codes
- `message.go`: Message structure and batch types
- `ulid.go`: ULID and ID generation

## Key Types, Interfaces, and Methods

### Error Handling
- `ErrorCode`, `PortaskError`, `ErrorCollector` — Error types and helpers
- `NewPortaskError`, `NewPortaskErrorWithCause`, `WithDetail`, `WithStack`, `Is`, `IsRetryable`, `IsPermanent`, etc.
- Error helpers: `ErrInvalidMessage`, `ErrMessageExpired`, `ErrMessageTooLarge`, `ErrTopicNotFound`, `ErrConsumerNotFound`, `ErrStorageError`, `ErrSerializationError`, `ErrDeserializationError`, `ErrCompressionError`, `ErrDecompressionError`, `ErrNetworkError`, `ErrTimeout`, `ErrUnauthorized`, `ErrResourceExhausted`

### Message Types
- `MessageID`, `TopicName`, `ConsumerID`, `MessagePriority`, `MessageStatus`, `MessageHeaders`, `PortaskMessage`, `MessageBatch`, `ConsumerOffset`, `TopicInfo`, `PartitionInfo`
- Methods: `NewMessage`, `WithHeader`, `WithPriority`, `WithTTL`, `WithPartitionKey`, `WithReplyTo`, `WithCorrelationID`, `WithTraceID`, `IsExpired`, `CanRetry`, `IncrementAttempts`, `GetHeader`, `GetStringHeader`, `GetSize`, `GetCompressedSize`, `SetCompressedSize`, `SetSerializedData`, `GetSerializedData`, `ClearSerializedData`, `markReceived`, `markProcessed`, `markAcknowledged`, `markFailed`, `GetProcessingTime`, `GetTotalTime`, etc.
- `MessageBatch` methods: `Add`, `Len`, `GetTotalSize`, `GetCompressedSize`, `SetCompressedSize`

### ULID & ID Generation
- `IDGenerator` interface, `ULIDGenerator`, `ShortIDGenerator`, `NumericIDGenerator`
- Methods: `Generate`, `GenerateWithTimestamp`

## Usage Example
```go
import "github.com/meftunca/portask/pkg/types"

msg := types.NewMessage("topic", []byte("payload"))
err := types.ErrInvalidMessage("bad format")
```

## TODO / Missing Features
- [ ] Add new error types
- [ ] Expand message structure
- [ ] Add more ID generation strategies

---

For details, see the code and comments in each file.
