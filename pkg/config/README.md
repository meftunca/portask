# config/ (Dynamic and Multi-Format Configuration System)

This package provides configuration management for the Portask system, supporting multiple formats and dynamic configuration options. It includes validation, serialization, and integration with other system components.

## Main Files
- `config.go`: Main configuration logic and types
- `config_test.go`: Tests for configuration loading and validation

## Key Types, Interfaces, and Methods

### Types & Structs
- `SerializationType` — Enum for serialization formats (CBOR, JSON, MsgPack, Protobuf, Avro)
- `CompressionType` — Enum for compression algorithms (None, Zstd, LZ4, Snappy, Gzip, Brotli)
- `CompressionStrategy` — Enum for compression strategies
- `SerializationConfig`, `JSONConfig`, `CBORConfig`, `MsgPackConfig` — Serialization configs
- `CompressionConfig`, `ZstdConfig`, `LZ4Config`, `AdaptiveConfig` — Compression configs
- `NetworkConfig` — Network settings
- `StorageConfig`, `DragonflyConfig`, `RedisConfig`, `MemoryConfig` — Storage settings
- `PerformanceConfig` — Performance tuning
- `MonitoringConfig` — Monitoring settings
- `LoggingConfig` — Logging settings
- `Config` — Main configuration struct
- `SystemMetrics` — System metrics for adaptive config

### Main Methods (selected)
- `DefaultConfig() *Config` — Returns default configuration
- `LoadConfig(configPath string) (*Config, error)` — Loads config from file
- `Validate() error` — Validates configuration
- `IsCompressionEnabled() bool` — Checks if compression is enabled
- `ShouldCompress(messageSize int, systemMetrics *SystemMetrics) bool` — Adaptive compression decision

## Usage Example
```go
import "github.com/meftunca/portask/pkg/config"

cfg := config.DefaultConfig()
err := cfg.Validate()
```

## TODO / Missing Features
- [ ] Dynamic reload support
- [ ] New format support (env, Consul, etc.)
- [ ] Hot-reload for running services
- [ ] Centralized config management

---

For details, see the code and comments in each file.
