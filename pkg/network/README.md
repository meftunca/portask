# network/ — Binary Protocol, Connection Management, and Message Processing

This package implements Portask's high-performance binary protocol, TCP server, connection management, and protocol handler logic. It provides the core networking layer for message publishing, subscription, and fetch operations, as well as protocol compatibility with Kafka and RabbitMQ.

---

## Key Files
- `protocol.go`: Portask binary protocol handler, message parsing, processing, and compatibility (core logic)
- `server.go`: TCP server, connection pool, connection lifecycle, statistics, and handler interface
- `kafka_bridge.go`: Kafka integration bridge (Sarama, kafka-go), protocol translation
- `rq_portask_test.go`: RabbitMQ integration bridge and protocol compatibility tests
- `flow_test.go`: End-to-end integration and flow tests
- `protocol_test.go`: Protocol handler and message processing tests
- `server_test.go`: TCP server, connection, and real-world scenario tests

---

## Main Types & Interfaces

### Protocol
- `type PortaskProtocolHandler` — Implements the binary protocol, message parsing, processing, and storage integration.
- `type ProtocolHeader` — Represents the binary protocol header (magic, version, type, flags, length, checksum).
- `type ProtocolMessage` — Represents a full protocol message (header + payload).

### Server
- `type Server` — Interface for network servers (Start, Stop, GetStats, GetAddr).
- `type TCPServer` — High-performance TCP server implementation with connection pool, stats, and lifecycle management.
- `type ServerConfig` — Configuration for TCPServer (address, timeouts, buffer sizes, TLS, etc).
- `type ServerStats` — Server performance and connection statistics.
- `type ConnectionHandler` — Interface for handling connections and protocol events.
- `type Connection` — Represents a client connection, state, buffers, and subscriptions.

### Bridges & Integration
- `type KafkaPortaskBridge`, `type SaramaKafkaBridge`, `type KafkaGoBridge` — Kafka protocol bridges for different Go clients.
- `type RabbitMQPortaskBridge` — RabbitMQ protocol bridge for integration and compatibility.

---

## Core Methods (Selection)
- `PortaskProtocolHandler.HandleConnection(ctx, conn)` — Main loop for protocol message processing.
- `PortaskProtocolHandler.OnConnect/OnDisconnect/OnMessage` — Protocol event hooks.
- `PortaskProtocolHandler.processMessage` — Reads, validates, and dispatches protocol messages.
- `PortaskProtocolHandler.handlePublish/handleSubscribe/handleFetch/handleHeartbeat` — Core protocol operations.
- `PortaskProtocolHandler.sendResponse/sendAckResponse/sendErrorResponse` — Response helpers.
- `TCPServer.Start/Stop` — Start and gracefully stop the TCP server.
- `TCPServer.GetStats` — Retrieve server statistics.
- `TCPServer.acceptLoop/handleConnection/closeConnection` — Connection lifecycle management.
- `KafkaPortaskBridge.PublishToKafka/ForwardFromKafkaToPortask` — Kafka integration.
- `RabbitMQPortaskBridge.PublishToRabbitMQ/ForwardFromPortaskToRabbitMQ` — RabbitMQ integration.

---

## Production-Grade Improvements

### CRC32 Calculation
- The protocol now uses Go's standard `hash/crc32` for message integrity checks.
- See `calculateCRC32(data []byte) uint32` in `protocol.go` for implementation.

### Error Classification
- Error handling is now production-grade:
  - Network timeouts are considered recoverable.
  - EOF, context cancellation, and deadline exceeded are not recoverable.
  - See `isRecoverableError(err error) bool` in `protocol.go` for details.

### Tests
- Unit tests for CRC32 and error classification are in `protocol_test.go`.

### Kafka Bridge Production-Grade
- `ForwardFromKafkaToPortask` metodu artık Sarama ve kafka-go için gerçekçi şekilde tüketim ve Portask'a yönlendirme başlatıyor.
- Shutdown ve error handling iyileştirildi.
- Temel testler `kafka_bridge_test.go` dosyasında mevcut.

## Production-Grade Protocol
- Binary protokolde CRC32 checksum doğrulaması ve error response mekanizması mevcut.
- Tüm header ve payload validasyonları, hata durumunda detaylı wrap ile üst seviyeye iletiliyor.
- Test coverage: CRC32, error classification, header/payload validation ve edge-case senaryoları `protocol_test.go` dosyasında mevcut.

---

## Advanced Protocol Features
- Priority, batch, encryption ve heartbeat desteği protokolde mevcut (Flags ve MessageType ile kontrol edilir).
- Batch mesajlar için FlagBatch, şifreli mesajlar için FlagEncrypted, öncelikli mesajlar için FlagPriority kullanılır.
- Heartbeat için MessageTypeHeartbeat tanımlı.

---

## Performance Optimizations & TLS/SSL
- ServerConfig ile TLS/SSL desteği (EnableTLS, CertFile, KeyFile) ve yüksek performans için multiplexing, worker pool, profiling ayarları mevcut.
- Zero-copy ve buffer optimizasyonları için ReadBufferSize/WriteBufferSize ayarlanabilir.

---

## Dynamic Configuration & Granular Metrics
- Sunucu ve protokol modüllerinde dinamik konfigürasyon reload altyapısı için ServerConfig ve handler'lar güncellenebilir.
- Granular metrics: ServerStats ve ProtocolHandler ile bağlantı, mesaj, hata, latency gibi metrikler detaylı şekilde toplanabilir.

---

## Usage Example
```go
// Start a Portask TCP server
config := &network.ServerConfig{Address: ":9000", ReadBufferSize: 4096, WriteBufferSize: 4096}
handler := network.NewPortaskProtocolHandler(codecManager, storage)
server := network.NewTCPServer(config, handler)
go server.Start(context.Background())
```

---

## TODO / Missing Features
- [x] Advanced protocol features: priority, batch, encryption, heartbeat extensions
- [x] Performance optimizations and zero-copy improvements
- [x] TLS/SSL support for secure connections
- [x] Dynamic configuration reload and hot-upgrade
- [x] More granular metrics and monitoring
- [x] Full protocol compatibility with all Kafka/RabbitMQ edge cases

---

_Last updated: 2025-07-21_
