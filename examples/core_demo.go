package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/meftunca/portask/pkg/compression"
	"github.com/meftunca/portask/pkg/config"
	"github.com/meftunca/portask/pkg/serialization"
	"github.com/meftunca/portask/pkg/types"
)

func main() {
	fmt.Println("🚀 Portask Core Components Demo")
	fmt.Println("================================")

	// 1. Load Configuration
	fmt.Println("\n📋 Loading Configuration...")
	cfg, err := config.LoadConfig("configs/config.yaml")
	if err != nil {
		log.Printf("Warning: Could not load config file, using defaults: %v", err)
		cfg = config.DefaultConfig()
	}

	fmt.Printf("✅ Configuration loaded successfully")
	fmt.Printf("   - Serialization: %s\n", cfg.Serialization.Type)
	fmt.Printf("   - Compression: %s (%s strategy)\n", cfg.Compression.Type, cfg.Compression.Strategy)
	fmt.Printf("   - Network Port: %d\n", cfg.Network.CustomPort)

	// 2. Initialize Serialization
	fmt.Println("\n📦 Initializing Serialization...")
	codecManager, err := serialization.NewCodecManager(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize codec manager: %v", err)
	}

	activeCodec := codecManager.GetActiveCodec()
	fmt.Printf("✅ Active codec: %s (%s)\n", activeCodec.Name(), activeCodec.ContentType())

	// 3. Initialize Compression
	fmt.Println("\n🗜️  Initializing Compression...")
	compFactory := compression.NewCompressorFactory()
	if err := compFactory.InitializeDefaultCompressors(cfg); err != nil {
		log.Fatalf("Failed to initialize compressors: %v", err)
	}

	compressor, err := compFactory.GetCompressor(cfg.Compression.Type)
	if err != nil {
		log.Fatalf("Failed to get compressor: %v", err)
	}

	fmt.Printf("✅ Active compressor: %s (min size: %d bytes)\n",
		compressor.Name(), compressor.MinSize())

	// 4. Create and Process Messages
	fmt.Println("\n💌 Creating and Processing Messages...")

	// Create a sample message
	payload := []byte(`{
		"user_id": "12345",
		"action": "purchase",
		"product_id": "product-789",
		"amount": 99.99,
		"timestamp": "2025-07-12T10:30:00Z",
		"metadata": {
			"source": "web",
			"campaign": "summer-sale",
			"session_id": "sess-abcd1234"
		}
	}`)

	message := types.NewMessage("user-events", payload).
		WithHeader("content-type", "application/json").
		WithHeader("source", "demo").
		WithPriority(types.PriorityHigh).
		WithTTL(24 * time.Hour).
		WithPartitionKey("user-12345").
		WithCorrelationID("demo-corr-123").
		WithTraceID("demo-trace-456")

	fmt.Printf("✅ Created message: %s\n", message.ID)
	fmt.Printf("   - Topic: %s\n", message.Topic)
	fmt.Printf("   - Size: %d bytes\n", message.GetSize())
	fmt.Printf("   - Priority: %d\n", message.Priority)
	fmt.Printf("   - Headers: %d\n", len(message.Headers))

	// 5. Serialize Message
	fmt.Println("\n🔄 Serializing Message...")
	start := time.Now()
	serializedData, err := codecManager.Encode(message)
	if err != nil {
		log.Fatalf("Failed to serialize message: %v", err)
	}
	serializationTime := time.Since(start)

	fmt.Printf("✅ Serialized in %v\n", serializationTime)
	fmt.Printf("   - Original size: %d bytes\n", message.GetSize())
	fmt.Printf("   - Serialized size: %d bytes\n", len(serializedData))
	fmt.Printf("   - Serialization ratio: %.2f%%\n",
		float64(len(serializedData))/float64(message.GetSize())*100)

	// 6. Compress Data
	fmt.Println("\n🗜️  Compressing Data...")
	start = time.Now()
	compressedData, err := compressor.Compress(serializedData)
	if err != nil {
		log.Fatalf("Failed to compress data: %v", err)
	}
	compressionTime := time.Since(start)

	fmt.Printf("✅ Compressed in %v\n", compressionTime)
	fmt.Printf("   - Serialized size: %d bytes\n", len(serializedData))
	fmt.Printf("   - Compressed size: %d bytes\n", len(compressedData))
	fmt.Printf("   - Compression ratio: %.2f%%\n",
		float64(len(compressedData))/float64(len(serializedData))*100)
	fmt.Printf("   - Total reduction: %.2f%%\n",
		float64(len(compressedData))/float64(message.GetSize())*100)

	// 7. Decompress and Deserialize
	fmt.Println("\n🔄 Decompressing and Deserializing...")
	start = time.Now()
	decompressedData, err := compressor.Decompress(compressedData)
	if err != nil {
		log.Fatalf("Failed to decompress data: %v", err)
	}
	decompressionTime := time.Since(start)

	start = time.Now()
	deserializedMessage, err := codecManager.Decode(decompressedData)
	if err != nil {
		log.Fatalf("Failed to deserialize message: %v", err)
	}
	deserializationTime := time.Since(start)

	fmt.Printf("✅ Decompressed in %v\n", decompressionTime)
	fmt.Printf("✅ Deserialized in %v\n", deserializationTime)
	fmt.Printf("   - Message ID: %s\n", deserializedMessage.ID)
	fmt.Printf("   - Payload size: %d bytes\n", len(deserializedMessage.Payload))
	fmt.Printf("   - Headers: %d\n", len(deserializedMessage.Headers))

	// 8. Batch Processing Demo
	fmt.Println("\n📦 Batch Processing Demo...")

	// Create a batch of messages
	var messages []*types.PortaskMessage
	for i := 0; i < 10; i++ {
		batchPayload := []byte(fmt.Sprintf(`{"batch_id": %d, "data": "sample data for message %d"}`, i, i))
		batchMessage := types.NewMessage("batch-topic", batchPayload).
			WithHeader("batch_index", i).
			WithPartitionKey(fmt.Sprintf("batch-%d", i%3))
		messages = append(messages, batchMessage)
	}

	batch := types.NewMessageBatch(messages)
	fmt.Printf("✅ Created batch with %d messages\n", batch.Len())
	fmt.Printf("   - Total size: %d bytes\n", batch.GetTotalSize())

	// Serialize batch
	start = time.Now()
	batchSerialized, err := codecManager.EncodeBatch(batch)
	if err != nil {
		log.Fatalf("Failed to serialize batch: %v", err)
	}
	batchSerializationTime := time.Since(start)

	// Compress batch
	start = time.Now()
	batchCompressed, err := compressor.CompressBatch([][]byte{batchSerialized})
	if err != nil {
		log.Fatalf("Failed to compress batch: %v", err)
	}
	batchCompressionTime := time.Since(start)

	fmt.Printf("✅ Batch processed in %v (serialize) + %v (compress)\n",
		batchSerializationTime, batchCompressionTime)
	fmt.Printf("   - Original size: %d bytes\n", batch.GetTotalSize())
	fmt.Printf("   - Compressed size: %d bytes\n", len(batchCompressed))
	fmt.Printf("   - Batch compression ratio: %.2f%%\n",
		float64(len(batchCompressed))/float64(batch.GetTotalSize())*100)

	// 9. Performance Summary
	fmt.Println("\n📊 Performance Summary")
	fmt.Println("=====================")
	totalTime := serializationTime + compressionTime + decompressionTime + deserializationTime
	fmt.Printf("Single Message Processing:\n")
	fmt.Printf("  - Serialization: %v\n", serializationTime)
	fmt.Printf("  - Compression: %v\n", compressionTime)
	fmt.Printf("  - Decompression: %v\n", decompressionTime)
	fmt.Printf("  - Deserialization: %v\n", deserializationTime)
	fmt.Printf("  - Total: %v\n", totalTime)
	fmt.Printf("  - Throughput: %.0f messages/sec\n", 1.0/totalTime.Seconds())

	fmt.Printf("\nBatch Processing (10 messages):\n")
	batchTotalTime := batchSerializationTime + batchCompressionTime
	fmt.Printf("  - Batch serialization: %v\n", batchSerializationTime)
	fmt.Printf("  - Batch compression: %v\n", batchCompressionTime)
	fmt.Printf("  - Total: %v\n", batchTotalTime)
	fmt.Printf("  - Throughput: %.0f messages/sec\n", 10.0/batchTotalTime.Seconds())

	// 10. Configuration Switching Demo
	fmt.Println("\n🔄 Configuration Switching Demo...")

	// Switch to JSON for comparison
	if err := codecManager.SwitchCodec(config.SerializationJSON); err != nil {
		log.Printf("Warning: Could not switch to JSON codec: %v", err)
	} else {
		jsonData, err := codecManager.Encode(message)
		if err == nil {
			fmt.Printf("✅ JSON serialization: %d bytes (vs %d CBOR)\n",
				len(jsonData), len(serializedData))
		}
	}

	// Test different compressor
	snappyCompressor, err := compFactory.GetCompressor(config.CompressionSnappy)
	if err == nil {
		snappyData, err := snappyCompressor.Compress(serializedData)
		if err == nil {
			fmt.Printf("✅ Snappy compression: %d bytes (vs %d %s)\n",
				len(snappyData), len(compressedData), compressor.Name())
		}
	}

	fmt.Println("\n🎉 Demo completed successfully!")
	fmt.Println("   See Phase1.md for detailed architecture documentation")
}

// Example function for testing compression ratio estimation
func demonstrateCompressionEstimation() {
	fmt.Println("\n🔍 Compression Ratio Estimation Demo...")

	// Test data with different characteristics
	testData := map[string][]byte{
		"Highly repetitive": []byte(strings.Repeat("ABCD", 100)),
		"JSON data":         []byte(`{"key1":"value1","key2":"value2","key3":"value3"}`),
		"Random data":       make([]byte, 400),
		"Text data":         []byte("The quick brown fox jumps over the lazy dog. " + strings.Repeat("Hello world! ", 20)),
	}

	// Fill random data
	for i := range testData["Random data"] {
		testData["Random data"][i] = byte(i % 256)
	}

	factory := compression.NewCompressorFactory()
	cfg := config.DefaultConfig()
	factory.InitializeDefaultCompressors(cfg)

	compressors := []config.CompressionType{
		config.CompressionZstd,
		config.CompressionLZ4,
		config.CompressionSnappy,
	}

	for dataType, data := range testData {
		fmt.Printf("\n%s (%d bytes):\n", dataType, len(data))

		for _, compType := range compressors {
			if comp, err := factory.GetCompressor(compType); err == nil {
				estimatedRatio := comp.EstimateRatio(data)

				if compressed, err := comp.Compress(data); err == nil {
					actualRatio := float64(len(compressed)) / float64(len(data))
					fmt.Printf("  %s: estimated %.2f, actual %.2f (diff: %.2f)\n",
						comp.Name(), estimatedRatio, actualRatio, actualRatio-estimatedRatio)
				}
			}
		}
	}
}
