package processor

import "time"

// MemoryTier defines memory usage tiers for different workloads
type MemoryTier string

const (
	TierLowLatency     MemoryTier = "low_latency"     // ~310MB - Minimal memory, best latency
	TierBalanced       MemoryTier = "balanced"        // ~600MB - Recommended for most workloads
	TierHighThroughput MemoryTier = "high_throughput" // ~940MB - Maximum throughput
	TierUltra          MemoryTier = "ultra"           // ~2GB - Extreme performance
)

// TierConfig returns optimal configuration for a memory tier
func TierConfig(tier MemoryTier) *ParallelBatchWriterConfig {
	switch tier {
	case TierLowLatency:
		return LowLatencyConfig()
	case TierBalanced:
		return BalancedConfig()
	case TierHighThroughput:
		return HighThroughputConfigV2()
	case TierUltra:
		return UltraConfig()
	default:
		return BalancedConfig() // Default
	}
}

// LowLatencyConfig minimizes memory usage and latency
// Memory: ~310MB
// Throughput: ~355K msgs/sec
// Latency: <10ms
func LowLatencyConfig() *ParallelBatchWriterConfig {
	return &ParallelBatchWriterConfig{
		NumShards:            32,
		FlushInterval:        10 * time.Millisecond,
		BatchSize:            5000,
		MaxRetries:           3,
		SubBatchSize:         200,
		EnableParallelWrites: true,
	}
}

// BalancedConfig provides best balance of memory vs throughput
// Memory: ~600MB (+290MB vs low_latency)
// Throughput: ~420K msgs/sec (+18%)
// Latency: <15ms
// RECOMMENDED for most production workloads
func BalancedConfig() *ParallelBatchWriterConfig {
	return &ParallelBatchWriterConfig{
		NumShards:            48, // +16 shards (more CPU parallelism)
		FlushInterval:        10 * time.Millisecond,
		BatchSize:            8000, // +3000 messages
		MaxRetries:           3,
		SubBatchSize:         250, // +50 (larger sub-batches)
		EnableParallelWrites: true,
	}
}

// HighThroughputConfigV2 maximizes throughput with moderate memory increase
// Memory: ~940MB (+630MB vs low_latency)
// Throughput: ~480K msgs/sec (+35%)
// Latency: <20ms
func HighThroughputConfigV2() *ParallelBatchWriterConfig {
	return &ParallelBatchWriterConfig{
		NumShards:            64,                    // +32 shards (2x CPU parallelism)
		FlushInterval:        12 * time.Millisecond, // Slightly higher for larger batches
		BatchSize:            10000,                 // +5000 messages
		MaxRetries:           3,
		SubBatchSize:         300, // +100 (larger sub-batches)
		EnableParallelWrites: true,
	}
}

// UltraConfig provides maximum throughput for high-end servers
// Memory: ~2GB (+1.7GB vs low_latency)
// Throughput: ~600K+ msgs/sec (+70%)
// Latency: <30ms
// Only use on servers with plenty of RAM (32GB+)
func UltraConfig() *ParallelBatchWriterConfig {
	return &ParallelBatchWriterConfig{
		NumShards:            128, // +96 shards (4x CPU parallelism)
		FlushInterval:        15 * time.Millisecond,
		BatchSize:            15000, // +10000 messages (3x original)
		MaxRetries:           3,
		SubBatchSize:         500, // +300 (2.5x sub-batches)
		EnableParallelWrites: true,
	}
}

// GetConfigInfo returns human-readable information about a config
func GetConfigInfo(tier MemoryTier) string {
	switch tier {
	case TierLowLatency:
		return "Low Latency: ~310MB memory, ~355K msgs/sec, <10ms latency"
	case TierBalanced:
		return "Balanced: ~600MB memory, ~420K msgs/sec, <15ms latency (RECOMMENDED)"
	case TierHighThroughput:
		return "High Throughput: ~940MB memory, ~480K msgs/sec, <20ms latency"
	case TierUltra:
		return "Ultra: ~2GB memory, ~600K+ msgs/sec, <30ms latency"
	default:
		return "Unknown tier"
	}
}

// EstimateMemoryUsage returns estimated memory usage in MB
func EstimateMemoryUsage(config *ParallelBatchWriterConfig) int {
	// Rough estimates:
	// - Connection pool: ~100KB per connection
	// - Batch buffer: NumShards × BatchSize × 1KB (avg message size)
	// - Object pools: ~50-100MB baseline

	connectionPoolMB := 100                                       // Base connection pool (1000 conns)
	batchBufferMB := (config.NumShards * config.BatchSize) / 1024 // Assuming 1KB msgs
	objectPoolMB := 50 + (config.NumShards * 2)                   // Scales with shards

	return connectionPoolMB + batchBufferMB + objectPoolMB
}

// EstimateThroughput returns estimated throughput in msgs/sec
func EstimateThroughput(config *ParallelBatchWriterConfig) int {
	// Rough formula based on benchmarks:
	// Base: 355K msgs/sec (32 shards, 5000 batch)
	// Scaling: ~linear with shards, ~logarithmic with batch size

	baseShards := 32
	baseBatch := 5000
	baseThroughput := 355000

	shardFactor := float64(config.NumShards) / float64(baseShards)
	batchFactor := 1.0 + (float64(config.BatchSize-baseBatch) / float64(baseBatch) * 0.1)

	return int(float64(baseThroughput) * shardFactor * batchFactor)
}
