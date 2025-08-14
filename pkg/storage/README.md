# storage/ (Dragonfly/Redis, In-memory, WAL Storage Adapters)

This package provides adapters and interfaces for different storage backends, including Dragonfly/Redis, in-memory, and write-ahead log (WAL) storage. It is designed for extensibility and reliability in message storage and retrieval.

## Main Files
- `dragonfly/`: Dragonfly/Redis adapter and tests
- `dragonfly.go`: Dragonfly/Redis storage implementation
- `dragonfly_test.go`: Tests for Dragonfly/Redis adapter
- `in_memory.go`: In-memory storage implementation
- `interface.go`: Storage interfaces and types
- `serializer.go`: Serialization and compression helpers
- `wal.go`: Write-ahead log storage implementation

## Key Types, Interfaces, and Methods

### Interfaces
- `MessageStore` — Interface for message storage operations
- `Serializer` — Interface for message serialization
- `Compressor` — Interface for data compression
- `StorageFactory` — Factory for creating storage backends
- `MetricsCollector` — Interface for collecting storage metrics
- `Transaction` — Transactional storage interface
- `TransactionalStore` — Store supporting transactions
- `QueryBuilder` — Query builder for storage
- `IndexManager` — Index management interface
- `CacheStore` — Cache storage interface

### Structs & Types
- `DragonflyStorage`, `DragonflyStorageConfig`, `DragonflyConfig` — Dragonfly/Redis storage
- `InMemoryStorage` — In-memory storage
- `StorageStats`, `StorageMetrics` — Storage statistics and metrics
- `RetentionPolicy`, `CleanupStrategy`, `StorageConfig` — Retention and config types
- `StorageManager` — Manages multiple storage backends
- `CompositeStore` — Replicated/fallback store
- `WALEntry`, `WALSegment`, `WALWriter`, `WALConfig`, `WALReader` — WAL storage
- `JSONSerializer`, `NoOpCompressor`, `ZstdCompressor` — Serialization and compression helpers

### Main Methods (selected)
- `NewDragonflyStorage(config *DragonflyStorageConfig) (*DragonflyStorage, error)`
- `Store`, `StoreBatch`, `Fetch`, `FetchByID`, `Delete`, `DeleteBatch`, `CreateTopic`, `DeleteTopic`, `GetTopicInfo`, `ListTopics`, `TopicExists`, `GetPartitionInfo`, `GetPartitionCount`, `GetLatestOffset`, `GetEarliestOffset`, `CommitOffset`, `CommitOffsetBatch`, `GetOffset`, `GetConsumerOffsets`, `ListConsumers`, `Ping`, `Stats`, `Cleanup`, `Close` (DragonflyStorage)
- `NewInMemoryStorage() *InMemoryStorage`, `StoreMessage`, `GetMessages`, `GetTopics` (InMemoryStorage)
- `NewStorageManager`, `SetPrimary`, `GetPrimary`, `AddStore`, `GetStore`, `CloseAll`, `HealthCheck` (StorageManager)
- `NewCompositeStore`, `Store`, `Fetch`, `Close` (CompositeStore)
- `DefaultWALConfig`, `NewWALWriter`, `Write`, `WriteBatch`, `Sync`, `Close` (WALWriter)
- `NewWALReader`, `ReadEntry`, `Close` (WALReader)
- `NewJSONSerializer`, `Serialize`, `Deserialize` (JSONSerializer)
- `NewNoOpCompressor`, `Compress`, `Decompress` (NoOpCompressor)
- `NewZstdCompressor`, `Compress`, `Decompress` (ZstdCompressor)

## Usage Example
```go
import "github.com/meftunca/portask/pkg/storage"

// Create a new in-memory storage
store := storage.NewInMemoryStorage()
err := store.StoreMessage("topic", []byte("message"))
```

## TODO / Missing Features
- [ ] High-availability (HA) / replication support
- [ ] Storage health monitoring
- [ ] Advanced query and indexing support
- [ ] Improved transaction support
- [ ] More storage adapters (e.g., S3, SQL)

## Production-Grade Improvements

### Retention & Cleanup
- `Cleanup` metodu ile zaman tabanlı mesaj silme (retention) destekleniyor.
- Dragonfly/Redis streamlerinde eski mesajlar otomatik olarak temizleniyor.

### Error Handling
- Tüm storage işlemlerinde hata durumları detaylı şekilde wrap edilip üst seviyeye iletiliyor.

### Testler
- Dragonfly ve in-memory storage için kapsamlı testler mevcut.

---

For details, see the code and comments in each file.
