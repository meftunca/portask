# Serialization Package (`serialization/`)

This package provides high-performance, extensible serialization and deserialization for Portask messages and message batches. It supports multiple codecs (CBOR, JSON, MessagePack) and offers a unified interface for encoding/decoding, batch operations, and codec management.

---

## Main Interfaces & Types

### `Codec` Interface
```go
// Codec defines the interface for message serialization/deserialization
 type Codec interface {
     Encode(message *types.PortaskMessage) ([]byte, error)
     Decode(data []byte) (*types.PortaskMessage, error)
     EncodeBatch(batch *types.MessageBatch) ([]byte, error)
     DecodeBatch(data []byte) (*types.MessageBatch, error)
     Name() string
     ContentType() string
 }
```

### `CodecFactory`
- Registers and manages available codecs by type.
- Methods: `RegisterCodec`, `GetCodec`, `GetAvailableCodecs`, `InitializeDefaultCodecs`.

### `CodecManager`
- High-level manager for active codec, switching, and pooling.
- Methods: `Encode`, `Decode`, `EncodeBatch`, `DecodeBatch`, `SwitchCodec`, `EncodeWithCodec`, `DecodeWithCodec`.

### Implementations
- **CBORCodec**: Fast, compact binary encoding (fxamacker/cbor)
- **JSONCodec**: Standard JSON encoding (encoding/json)
- **MsgPackCodec**: MessagePack encoding (vmihailenco/msgpack)

---

## Key Structs (from `types`)

### `PortaskMessage`
```go
type PortaskMessage struct {
    ID        MessageID
    Topic     TopicName
    Payload   []byte
    Timestamp int64
    Headers   MessageHeaders
    Priority  MessagePriority
    Status    MessageStatus
    Partition int32
    Key          string
    PartitionKey string
    ReplyTo      TopicName
    ExpiresAt int64
    TTL       int64
    Attempts  uint8
    MaxRetry  uint8
    CorrelationID string
    TraceID       string
    // ...internal fields omitted
}
```

### `MessageBatch`
```go
type MessageBatch struct {
    Messages  []*PortaskMessage
    BatchID   string
    CreatedAt int64
    // ...internal fields omitted
}
```

---

## Example Usage

```go
factory := serialization.NewCodecFactory()
err := factory.InitializeDefaultCodecs(cfg)
codec, err := factory.GetCodec(config.SerializationCBOR)
data, err := codec.Encode(message)
decoded, err := codec.Decode(data)
```

---

## Test & Benchmark Coverage
- Comprehensive unit tests for all codecs and manager logic (`codec_test.go`)
- Benchmarks for CBOR, JSON, and MessagePack encoding/decoding

---

## TODO
- [ ] Add new codec types (e.g., Avro, Protobuf)
- [ ] Advanced performance benchmarks and comparison
- [ ] Streaming/zero-copy serialization support
- [ ] Pluggable custom codecs
