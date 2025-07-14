package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// SerializationType defines the message serialization format
type SerializationType string

const (
	SerializationCBOR     SerializationType = "cbor"
	SerializationJSON     SerializationType = "json"
	SerializationMsgPack  SerializationType = "msgpack"
	SerializationProtobuf SerializationType = "protobuf"
	SerializationAvro     SerializationType = "avro"
)

// CompressionType defines the compression algorithm
type CompressionType string

const (
	CompressionNone   CompressionType = "none"
	CompressionZstd   CompressionType = "zstd"
	CompressionLZ4    CompressionType = "lz4"
	CompressionSnappy CompressionType = "snappy"
	CompressionGzip   CompressionType = "gzip"
	CompressionBrotli CompressionType = "brotli"
)

// CompressionStrategy defines when to apply compression
type CompressionStrategy string

const (
	StrategyAlways    CompressionStrategy = "always"
	StrategyThreshold CompressionStrategy = "threshold"
	StrategyAdaptive  CompressionStrategy = "adaptive"
	StrategyNever     CompressionStrategy = "never"
)

// SerializationConfig holds serialization settings
type SerializationConfig struct {
	Type          SerializationType `mapstructure:"type" yaml:"type" json:"type"`
	JSONConfig    JSONConfig        `mapstructure:"json" yaml:"json" json:"json"`
	CBORConfig    CBORConfig        `mapstructure:"cbor" yaml:"cbor" json:"cbor"`
	MsgPackConfig MsgPackConfig     `mapstructure:"msgpack" yaml:"msgpack" json:"msgpack"`
}

// JSONConfig holds JSON-specific settings
type JSONConfig struct {
	Library    string `mapstructure:"library" yaml:"library" json:"library"` // "standard" or "sonic"
	Compact    bool   `mapstructure:"compact" yaml:"compact" json:"compact"`
	EscapeHTML bool   `mapstructure:"escape_html" yaml:"escape_html" json:"escape_html"`
}

// CBORConfig holds CBOR-specific settings
type CBORConfig struct {
	DeterministicMode bool `mapstructure:"deterministic" yaml:"deterministic" json:"deterministic"`
	TimeMode          int  `mapstructure:"time_mode" yaml:"time_mode" json:"time_mode"`
}

// MsgPackConfig holds MessagePack-specific settings
type MsgPackConfig struct {
	UseArrayEncodedStructs bool `mapstructure:"use_array_encoded_structs" yaml:"use_array_encoded_structs" json:"use_array_encoded_structs"`
}

// CompressionConfig holds compression settings
type CompressionConfig struct {
	Type           CompressionType     `mapstructure:"type" yaml:"type" json:"type"`
	Strategy       CompressionStrategy `mapstructure:"strategy" yaml:"strategy" json:"strategy"`
	Level          int                 `mapstructure:"level" yaml:"level" json:"level"`
	ThresholdBytes int                 `mapstructure:"threshold_bytes" yaml:"threshold_bytes" json:"threshold_bytes"`
	BatchSize      int                 `mapstructure:"batch_size" yaml:"batch_size" json:"batch_size"`
	ZstdConfig     ZstdConfig          `mapstructure:"zstd" yaml:"zstd" json:"zstd"`
	LZ4Config      LZ4Config           `mapstructure:"lz4" yaml:"lz4" json:"lz4"`
	AdaptiveConfig AdaptiveConfig      `mapstructure:"adaptive" yaml:"adaptive" json:"adaptive"`
}

// ZstdConfig holds Zstandard-specific settings
type ZstdConfig struct {
	WindowSize      int  `mapstructure:"window_size" yaml:"window_size" json:"window_size"`
	UseDict         bool `mapstructure:"use_dict" yaml:"use_dict" json:"use_dict"`
	DictSize        int  `mapstructure:"dict_size" yaml:"dict_size" json:"dict_size"`
	ConcurrencyMode int  `mapstructure:"concurrency_mode" yaml:"concurrency_mode" json:"concurrency_mode"`
}

// LZ4Config holds LZ4-specific settings
type LZ4Config struct {
	CompressionLevel int  `mapstructure:"compression_level" yaml:"compression_level" json:"compression_level"`
	FastAcceleration int  `mapstructure:"fast_acceleration" yaml:"fast_acceleration" json:"fast_acceleration"`
	BlockMode        bool `mapstructure:"block_mode" yaml:"block_mode" json:"block_mode"`
}

// AdaptiveConfig holds adaptive compression settings
type AdaptiveConfig struct {
	CPUThresholdHigh    float64 `mapstructure:"cpu_threshold_high" yaml:"cpu_threshold_high" json:"cpu_threshold_high"`
	CPUThresholdLow     float64 `mapstructure:"cpu_threshold_low" yaml:"cpu_threshold_low" json:"cpu_threshold_low"`
	MemoryThresholdHigh float64 `mapstructure:"memory_threshold_high" yaml:"memory_threshold_high" json:"memory_threshold_high"`
	LatencyThresholdMs  int     `mapstructure:"latency_threshold_ms" yaml:"latency_threshold_ms" json:"latency_threshold_ms"`
	MonitorInterval     string  `mapstructure:"monitor_interval" yaml:"monitor_interval" json:"monitor_interval"`
}

// NetworkConfig holds network settings
type NetworkConfig struct {
	CustomPort   int  `mapstructure:"custom_port" yaml:"custom_port" json:"custom_port"`
	KafkaPort    int  `mapstructure:"kafka_port" yaml:"kafka_port" json:"kafka_port"`
	RabbitMQPort int  `mapstructure:"rabbitmq_port" yaml:"rabbitmq_port" json:"rabbitmq_port"`
	AdminPort    int  `mapstructure:"admin_port" yaml:"admin_port" json:"admin_port"`
	KeepAlive    bool `mapstructure:"keep_alive" yaml:"keep_alive" json:"keep_alive"`
	NoDelay      bool `mapstructure:"no_delay" yaml:"no_delay" json:"no_delay"`
	ReadTimeout  int  `mapstructure:"read_timeout_ms" yaml:"read_timeout_ms" json:"read_timeout_ms"`
	WriteTimeout int  `mapstructure:"write_timeout_ms" yaml:"write_timeout_ms" json:"write_timeout_ms"`
}

// StorageConfig holds storage settings
type StorageConfig struct {
	Type              string          `mapstructure:"type" yaml:"type" json:"type"`
	DragonflyConfig   DragonflyConfig `mapstructure:"dragonfly" yaml:"dragonfly" json:"dragonfly"`
	RedisConfig       RedisConfig     `mapstructure:"redis" yaml:"redis" json:"redis"`
	MemoryConfig      MemoryConfig    `mapstructure:"memory" yaml:"memory" json:"memory"`
	ConnectionTimeout time.Duration   `mapstructure:"connection_timeout" yaml:"connection_timeout" json:"connection_timeout"`
	ReadTimeout       time.Duration   `mapstructure:"read_timeout" yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout      time.Duration   `mapstructure:"write_timeout" yaml:"write_timeout" json:"write_timeout"`
	PoolSize          int             `mapstructure:"pool_size" yaml:"pool_size" json:"pool_size"`
	MinIdleConns      int             `mapstructure:"min_idle_conns" yaml:"min_idle_conns" json:"min_idle_conns"`
	MaxRetries        int             `mapstructure:"max_retries" yaml:"max_retries" json:"max_retries"`
	RetryBackoff      time.Duration   `mapstructure:"retry_backoff" yaml:"retry_backoff" json:"retry_backoff"`
}

// DragonflyConfig holds Dragonfly-specific settings
type DragonflyConfig struct {
	Addresses []string `mapstructure:"addresses" yaml:"addresses" json:"addresses"`
	Password  string   `mapstructure:"password" yaml:"password" json:"password"`
	DB        int      `mapstructure:"db" yaml:"db" json:"db"`
}

// RedisConfig holds Redis-specific settings
type RedisConfig struct {
	Addresses []string `mapstructure:"addresses" yaml:"addresses" json:"addresses"`
	Password  string   `mapstructure:"password" yaml:"password" json:"password"`
	DB        int      `mapstructure:"db" yaml:"db" json:"db"`
}

// MemoryConfig holds in-memory storage settings
type MemoryConfig struct {
	MaxMemoryMB     int    `mapstructure:"max_memory_mb" yaml:"max_memory_mb" json:"max_memory_mb"`
	EvictionPolicy  string `mapstructure:"eviction_policy" yaml:"eviction_policy" json:"eviction_policy"`
	PersistInterval int    `mapstructure:"persist_interval_sec" yaml:"persist_interval_sec" json:"persist_interval_sec"`
}

// PerformanceConfig holds performance tuning settings
type PerformanceConfig struct {
	WorkerPoolSize    int    `mapstructure:"worker_pool_size" yaml:"worker_pool_size" json:"worker_pool_size"`
	ChannelBufferSize int    `mapstructure:"channel_buffer_size" yaml:"channel_buffer_size" json:"channel_buffer_size"`
	BatchProcessing   bool   `mapstructure:"batch_processing" yaml:"batch_processing" json:"batch_processing"`
	BatchSize         int    `mapstructure:"batch_size" yaml:"batch_size" json:"batch_size"`
	BatchTimeout      string `mapstructure:"batch_timeout" yaml:"batch_timeout" json:"batch_timeout"`
	MemoryPoolEnabled bool   `mapstructure:"memory_pool_enabled" yaml:"memory_pool_enabled" json:"memory_pool_enabled"`
	ZeroCopyEnabled   bool   `mapstructure:"zero_copy_enabled" yaml:"zero_copy_enabled" json:"zero_copy_enabled"`
	UseMemoryMapping  bool   `mapstructure:"use_memory_mapping" yaml:"use_memory_mapping" json:"use_memory_mapping"`
}

// MonitoringConfig holds monitoring and metrics settings
type MonitoringConfig struct {
	Enabled         bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	MetricsPort     int    `mapstructure:"metrics_port" yaml:"metrics_port" json:"metrics_port"`
	MetricsPath     string `mapstructure:"metrics_path" yaml:"metrics_path" json:"metrics_path"`
	CollectInterval string `mapstructure:"collect_interval" yaml:"collect_interval" json:"collect_interval"`
	EnableProfiling bool   `mapstructure:"enable_profiling" yaml:"enable_profiling" json:"enable_profiling"`
	ProfilingPort   int    `mapstructure:"profiling_port" yaml:"profiling_port" json:"profiling_port"`
	HealthCheckPath string `mapstructure:"health_check_path" yaml:"health_check_path" json:"health_check_path"`
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level       string `mapstructure:"level" yaml:"level" json:"level"`
	Format      string `mapstructure:"format" yaml:"format" json:"format"`
	Output      string `mapstructure:"output" yaml:"output" json:"output"`
	EnableColor bool   `mapstructure:"enable_color" yaml:"enable_color" json:"enable_color"`
}

// Config represents the main configuration structure
type Config struct {
	Serialization SerializationConfig `mapstructure:"serialization" yaml:"serialization" json:"serialization"`
	Compression   CompressionConfig   `mapstructure:"compression" yaml:"compression" json:"compression"`
	Network       NetworkConfig       `mapstructure:"network" yaml:"network" json:"network"`
	Storage       StorageConfig       `mapstructure:"storage" yaml:"storage" json:"storage"`
	Performance   PerformanceConfig   `mapstructure:"performance" yaml:"performance" json:"performance"`
	Monitoring    MonitoringConfig    `mapstructure:"monitoring" yaml:"monitoring" json:"monitoring"`
	Logging       LoggingConfig       `mapstructure:"logging" yaml:"logging" json:"logging"`
}

// DefaultConfig returns a configuration with sensible defaults optimized for performance
func DefaultConfig() *Config {
	return &Config{
		Serialization: SerializationConfig{
			Type: SerializationCBOR,
			JSONConfig: JSONConfig{
				Library:    "standard",
				Compact:    true,
				EscapeHTML: false,
			},
			CBORConfig: CBORConfig{
				DeterministicMode: true,
				TimeMode:          1, // RFC3339
			},
			MsgPackConfig: MsgPackConfig{
				UseArrayEncodedStructs: true,
			},
		},
		Compression: CompressionConfig{
			Type:           CompressionZstd,
			Strategy:       StrategyAdaptive,
			Level:          3, // Balance between speed and compression
			ThresholdBytes: 256,
			BatchSize:      100,
			ZstdConfig: ZstdConfig{
				WindowSize:      20, // 2^20 = 1MB window (minimum 1024)
				UseDict:         false,
				DictSize:        16384,
				ConcurrencyMode: 1,
			},
			LZ4Config: LZ4Config{
				CompressionLevel: 0, // Use 0 for default/fastest
				FastAcceleration: 1,
				BlockMode:        true,
			},
			AdaptiveConfig: AdaptiveConfig{
				CPUThresholdHigh:    80.0,
				CPUThresholdLow:     40.0,
				MemoryThresholdHigh: 85.0,
				LatencyThresholdMs:  10,
				MonitorInterval:     "5s",
			},
		},
		Network: NetworkConfig{
			CustomPort:   8080,
			KafkaPort:    9092,
			RabbitMQPort: 5672,
			AdminPort:    8081,
			KeepAlive:    true,
			NoDelay:      true,
			ReadTimeout:  5000,
			WriteTimeout: 5000,
		},
		Storage: StorageConfig{
			Type: "dragonfly",
			DragonflyConfig: DragonflyConfig{
				Addresses: []string{"localhost:6379"},
				DB:        0,
			},
			ConnectionTimeout: 5 * time.Second,
			ReadTimeout:       3 * time.Second,
			WriteTimeout:      3 * time.Second,
			PoolSize:          100,
			MinIdleConns:      10,
			MaxRetries:        3,
			RetryBackoff:      100 * time.Millisecond,
		},
		Performance: PerformanceConfig{
			WorkerPoolSize:    100,
			ChannelBufferSize: 1000,
			BatchProcessing:   true,
			BatchSize:         100,
			BatchTimeout:      "10ms",
			MemoryPoolEnabled: true,
			ZeroCopyEnabled:   true,
			UseMemoryMapping:  false,
		},
		Monitoring: MonitoringConfig{
			Enabled:         true,
			MetricsPort:     9090,
			MetricsPath:     "/metrics",
			CollectInterval: "1s",
			EnableProfiling: false,
			ProfilingPort:   6060,
			HealthCheckPath: "/health",
		},
		Logging: LoggingConfig{
			Level:       "info",
			Format:      "json",
			Output:      "stdout",
			EnableColor: false,
		},
	}
}

// LoadConfig loads configuration from file
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()

	// Set default values
	config := DefaultConfig()

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./configs")
		v.AddConfigPath("/etc/portask")
	}

	// Enable reading from environment variables
	v.SetEnvPrefix("PORTASK")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Unmarshal config
	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate config
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate serialization type
	switch c.Serialization.Type {
	case SerializationCBOR, SerializationJSON, SerializationMsgPack, SerializationProtobuf, SerializationAvro:
		// Valid
	default:
		return fmt.Errorf("invalid serialization type: %s", c.Serialization.Type)
	}

	// Validate compression type
	switch c.Compression.Type {
	case CompressionNone, CompressionZstd, CompressionLZ4, CompressionSnappy, CompressionGzip, CompressionBrotli:
		// Valid
	default:
		return fmt.Errorf("invalid compression type: %s", c.Compression.Type)
	}

	// Validate compression strategy
	switch c.Compression.Strategy {
	case StrategyAlways, StrategyThreshold, StrategyAdaptive, StrategyNever:
		// Valid
	default:
		return fmt.Errorf("invalid compression strategy: %s", c.Compression.Strategy)
	}

	// Validate ports
	if c.Network.CustomPort <= 0 || c.Network.CustomPort > 65535 {
		return fmt.Errorf("invalid custom port: %d", c.Network.CustomPort)
	}

	if c.Network.KafkaPort <= 0 || c.Network.KafkaPort > 65535 {
		return fmt.Errorf("invalid kafka port: %d", c.Network.KafkaPort)
	}

	if c.Network.RabbitMQPort <= 0 || c.Network.RabbitMQPort > 65535 {
		return fmt.Errorf("invalid rabbitmq port: %d", c.Network.RabbitMQPort)
	}

	// Validate performance settings
	if c.Performance.WorkerPoolSize <= 0 {
		return fmt.Errorf("worker pool size must be greater than 0")
	}

	if c.Performance.BatchSize <= 0 {
		return fmt.Errorf("batch size must be greater than 0")
	}

	// Validate storage type
	switch c.Storage.Type {
	case "dragonfly", "redis", "memory":
		// Valid
	default:
		return fmt.Errorf("invalid storage type: %s", c.Storage.Type)
	}

	return nil
}

// IsCompressionEnabled returns true if compression should be applied
func (c *Config) IsCompressionEnabled() bool {
	return c.Compression.Type != CompressionNone && c.Compression.Strategy != StrategyNever
}

// ShouldCompress determines if a message should be compressed based on strategy and size
func (c *Config) ShouldCompress(messageSize int, systemMetrics *SystemMetrics) bool {
	if !c.IsCompressionEnabled() {
		return false
	}

	switch c.Compression.Strategy {
	case StrategyAlways:
		return true
	case StrategyNever:
		return false
	case StrategyThreshold:
		return messageSize >= c.Compression.ThresholdBytes
	case StrategyAdaptive:
		if systemMetrics == nil {
			return messageSize >= c.Compression.ThresholdBytes
		}

		// Don't compress if system is under high load
		if systemMetrics.CPUUsage > c.Compression.AdaptiveConfig.CPUThresholdHigh ||
			systemMetrics.MemoryUsage > c.Compression.AdaptiveConfig.MemoryThresholdHigh ||
			systemMetrics.LatencyMs > float64(c.Compression.AdaptiveConfig.LatencyThresholdMs) {
			return false
		}

		return messageSize >= c.Compression.ThresholdBytes
	default:
		return false
	}
}

// SystemMetrics represents current system performance metrics
type SystemMetrics struct {
	CPUUsage    float64
	MemoryUsage float64
	LatencyMs   float64
}
