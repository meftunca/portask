package config

import (
	"os"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("Expected default config to be created")
	}

	// Test serialization defaults
	if cfg.Serialization.Type != SerializationCBOR {
		t.Errorf("Expected default serialization type to be CBOR, got %s", cfg.Serialization.Type)
	}

	// Test compression defaults
	if cfg.Compression.Type != CompressionZstd {
		t.Errorf("Expected default compression type to be Zstd, got %s", cfg.Compression.Type)
	}

	// Test performance defaults
	if cfg.Performance.WorkerPoolSize <= 0 {
		t.Error("Expected worker pool size to be positive")
	}

	if cfg.Performance.BatchSize <= 0 {
		t.Error("Expected batch size to be positive")
	}
}

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	configContent := `
serialization:
  type: "json"
  json:
    compact: true
    escape_html: false

compression:
  type: "lz4"
  level: 1
  strategy: "always"
  threshold_bytes: 100

performance:
  worker_pool_size: 8
  batch_size: 50
  batch_processing: true

monitoring:
  enabled: true
  metrics_port: 9090
  enable_profiling: true

network:
  custom_port: 8080
  keep_alive: true
  no_delay: true

storage:
  type: "memory"
  connection_timeout: "5s"
  read_timeout: "3s"
  write_timeout: "3s"
  pool_size: 10

logging:
  level: "info"
  format: "json"
  output: "stdout"
`

	// Write config to temporary file
	tmpfile, err := os.CreateTemp("", "portask_test_config_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(configContent)); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpfile.Close()

	// Test loading the config
	cfg, err := LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify loaded values
	if cfg.Serialization.Type != SerializationJSON {
		t.Errorf("Expected serialization type JSON, got %s", cfg.Serialization.Type)
	}

	if cfg.Compression.Type != CompressionLZ4 {
		t.Errorf("Expected compression type LZ4, got %s", cfg.Compression.Type)
	}

	if cfg.Performance.WorkerPoolSize != 8 {
		t.Errorf("Expected worker pool size 8, got %d", cfg.Performance.WorkerPoolSize)
	}

	if cfg.Monitoring.MetricsPort != 9090 {
		t.Errorf("Expected metrics port 9090, got %d", cfg.Monitoring.MetricsPort)
	}
}

func TestLoadConfigWithEnvVars(t *testing.T) {
	// Set environment variables
	os.Setenv("PORTASK_SERIALIZATION_TYPE", "msgpack")
	os.Setenv("PORTASK_COMPRESSION_TYPE", "snappy")
	os.Setenv("PORTASK_PERFORMANCE_WORKER_POOL_SIZE", "16")
	defer func() {
		os.Unsetenv("PORTASK_SERIALIZATION_TYPE")
		os.Unsetenv("PORTASK_COMPRESSION_TYPE")
		os.Unsetenv("PORTASK_PERFORMANCE_WORKER_POOL_SIZE")
	}()

	// Create minimal config file
	configContent := `
serialization:
  type: "cbor"
compression:
  type: "zstd"
performance:
  worker_pool_size: 4
`

	tmpfile, err := os.CreateTemp("", "portask_env_test_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(configContent)); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpfile.Close()

	cfg, err := LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to load config with env vars: %v", err)
	}

	// Environment variables should override file values
	if cfg.Serialization.Type != SerializationMsgPack {
		t.Errorf("Expected serialization type MsgPack (from env), got %s", cfg.Serialization.Type)
	}

	if cfg.Compression.Type != CompressionSnappy {
		t.Errorf("Expected compression type Snappy (from env), got %s", cfg.Compression.Type)
	}

	if cfg.Performance.WorkerPoolSize != 16 {
		t.Errorf("Expected worker pool size 16 (from env), got %d", cfg.Performance.WorkerPoolSize)
	}
}

func TestConfigValidation(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		cfg := DefaultConfig()
		if err := cfg.Validate(); err != nil {
			t.Errorf("Default config should be valid: %v", err)
		}
	})

	t.Run("InvalidSerializationType", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Serialization.Type = "invalid"
		if err := cfg.Validate(); err == nil {
			t.Error("Expected validation error for invalid serialization type")
		}
	})

	t.Run("InvalidCompressionType", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Compression.Type = "invalid"
		if err := cfg.Validate(); err == nil {
			t.Error("Expected validation error for invalid compression type")
		}
	})

	t.Run("InvalidWorkerPoolSize", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Performance.WorkerPoolSize = 0
		if err := cfg.Validate(); err == nil {
			t.Error("Expected validation error for zero worker pool size")
		}
	})

	t.Run("InvalidBatchSize", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Performance.BatchSize = 0
		if err := cfg.Validate(); err == nil {
			t.Error("Expected validation error for zero batch size")
		}
	})

	t.Run("InvalidPort", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Network.CustomPort = -1
		if err := cfg.Validate(); err == nil {
			t.Error("Expected validation error for negative port")
		}

		cfg.Network.CustomPort = 70000
		if err := cfg.Validate(); err == nil {
			t.Error("Expected validation error for port > 65535")
		}
	})
}

func TestConfigSerialization(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Serialization.Type = SerializationJSON
	cfg.Compression.Type = CompressionLZ4
	cfg.Performance.WorkerPoolSize = 12

	// Test that config can be created and values are set correctly
	if cfg.Serialization.Type != SerializationJSON {
		t.Errorf("Expected serialization type JSON, got %s", cfg.Serialization.Type)
	}

	if cfg.Compression.Type != CompressionLZ4 {
		t.Errorf("Expected compression type LZ4, got %s", cfg.Compression.Type)
	}

	if cfg.Performance.WorkerPoolSize != 12 {
		t.Errorf("Expected worker pool size 12, got %d", cfg.Performance.WorkerPoolSize)
	}
}

func TestSerializationConfig(t *testing.T) {
	t.Run("CBORConfig", func(t *testing.T) {
		cfg := CBORConfig{
			DeterministicMode: true,
			TimeMode:          1,
		}

		if !cfg.DeterministicMode {
			t.Error("Expected deterministic mode to be true")
		}
		if cfg.TimeMode != 1 {
			t.Errorf("Expected time mode 1, got %d", cfg.TimeMode)
		}
	})

	t.Run("JSONConfig", func(t *testing.T) {
		cfg := JSONConfig{
			Compact:    true,
			EscapeHTML: false,
		}

		if !cfg.Compact {
			t.Error("Expected compact mode to be true")
		}
		if cfg.EscapeHTML {
			t.Error("Expected escape HTML to be false")
		}
	})

	t.Run("MsgPackConfig", func(t *testing.T) {
		cfg := MsgPackConfig{
			UseArrayEncodedStructs: true,
		}

		if !cfg.UseArrayEncodedStructs {
			t.Error("Expected array encoded structs to be true")
		}
	})
}

func TestCompressionConfig(t *testing.T) {
	t.Run("ZstdConfig", func(t *testing.T) {
		cfg := ZstdConfig{
			WindowSize:      19,
			UseDict:         false,
			DictSize:        16384,
			ConcurrencyMode: 1,
		}

		if cfg.WindowSize != 19 {
			t.Errorf("Expected window size 19, got %d", cfg.WindowSize)
		}
		if cfg.UseDict {
			t.Error("Expected use dict to be false")
		}
	})

	t.Run("LZ4Config", func(t *testing.T) {
		cfg := LZ4Config{
			CompressionLevel: 1,
			FastAcceleration: 1,
			BlockMode:        true,
		}

		if cfg.CompressionLevel != 1 {
			t.Errorf("Expected compression level 1, got %d", cfg.CompressionLevel)
		}
		if !cfg.BlockMode {
			t.Error("Expected block mode to be true")
		}
	})

	t.Run("CompressionStrategyConfig", func(t *testing.T) {
		// Test compression strategy configuration
		strategy := StrategyAlways
		if strategy != StrategyAlways {
			t.Errorf("Expected compression strategy always, got %s", strategy)
		}

		strategy = StrategyAdaptive
		if strategy != StrategyAdaptive {
			t.Errorf("Expected compression strategy adaptive, got %s", strategy)
		}

		strategy = StrategyThreshold
		if strategy != StrategyThreshold {
			t.Errorf("Expected compression strategy threshold, got %s", strategy)
		}

		strategy = StrategyNever
		if strategy != StrategyNever {
			t.Errorf("Expected compression strategy never, got %s", strategy)
		}
	})
}

func TestNetworkConfig(t *testing.T) {
	cfg := NetworkConfig{
		CustomPort:   8080,
		KafkaPort:    9092,
		RabbitMQPort: 5672,
		AdminPort:    8081,
		KeepAlive:    true,
		NoDelay:      true,
		ReadTimeout:  5000,
		WriteTimeout: 5000,
	}

	if cfg.CustomPort != 8080 {
		t.Errorf("Expected custom port 8080, got %d", cfg.CustomPort)
	}
	if !cfg.KeepAlive {
		t.Error("Expected keep alive to be true")
	}
	if !cfg.NoDelay {
		t.Error("Expected no delay to be true")
	}
}

func TestStorageConfig(t *testing.T) {
	cfg := StorageConfig{
		Type:              "memory",
		ConnectionTimeout: 5 * time.Second,
		ReadTimeout:       3 * time.Second,
		WriteTimeout:      3 * time.Second,
		PoolSize:          10,
		MinIdleConns:      2,
		MaxRetries:        3,
		RetryBackoff:      100 * time.Millisecond,
	}

	if cfg.Type != "memory" {
		t.Errorf("Expected storage type 'memory', got %s", cfg.Type)
	}
	if cfg.PoolSize != 10 {
		t.Errorf("Expected pool size 10, got %d", cfg.PoolSize)
	}
	if cfg.ConnectionTimeout != 5*time.Second {
		t.Errorf("Expected connection timeout 5s, got %v", cfg.ConnectionTimeout)
	}
}

func TestPerformanceConfig(t *testing.T) {
	cfg := PerformanceConfig{
		WorkerPoolSize:    8,
		ChannelBufferSize: 1000,
		BatchProcessing:   true,
		BatchSize:         50,
		BatchTimeout:      "10ms",
		MemoryPoolEnabled: true,
		ZeroCopyEnabled:   true,
		UseMemoryMapping:  false,
	}

	if cfg.WorkerPoolSize != 8 {
		t.Errorf("Expected worker pool size 8, got %d", cfg.WorkerPoolSize)
	}
	if !cfg.BatchProcessing {
		t.Error("Expected batch processing to be true")
	}
	if !cfg.MemoryPoolEnabled {
		t.Error("Expected memory pool to be enabled")
	}
}

func TestMonitoringConfig(t *testing.T) {
	cfg := MonitoringConfig{
		Enabled:         true,
		MetricsPort:     9090,
		MetricsPath:     "/metrics",
		CollectInterval: "30s",
		EnableProfiling: true,
		ProfilingPort:   6060,
		HealthCheckPath: "/health",
	}

	if !cfg.Enabled {
		t.Error("Expected monitoring to be enabled")
	}
	if cfg.MetricsPort != 9090 {
		t.Errorf("Expected metrics port 9090, got %d", cfg.MetricsPort)
	}
	if cfg.MetricsPath != "/metrics" {
		t.Errorf("Expected metrics path '/metrics', got %s", cfg.MetricsPath)
	}
}

func TestLoggingConfig(t *testing.T) {
	cfg := LoggingConfig{
		Level:       "info",
		Format:      "json",
		Output:      "stdout",
		EnableColor: true,
	}

	if cfg.Level != "info" {
		t.Errorf("Expected log level 'info', got %s", cfg.Level)
	}
	if cfg.Format != "json" {
		t.Errorf("Expected log format 'json', got %s", cfg.Format)
	}
	if !cfg.EnableColor {
		t.Error("Expected color to be enabled")
	}
}

// Benchmark config loading
func BenchmarkLoadConfig(b *testing.B) {
	// Create a temporary config file
	configContent := `
serialization:
  type: "cbor"
compression:
  type: "zstd"
performance:
  worker_pool_size: 8
  batch_size: 50
`

	tmpfile, err := os.CreateTemp("", "portask_bench_*.yaml")
	if err != nil {
		b.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(configContent)); err != nil {
		b.Fatalf("Failed to write config: %v", err)
	}
	tmpfile.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := LoadConfig(tmpfile.Name())
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkConfigValidation(b *testing.B) {
	cfg := DefaultConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := cfg.Validate()
		if err != nil {
			b.Fatal(err)
		}
	}
}
