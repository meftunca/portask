package compression

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"runtime"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/config"
)

func createTestCompressor() (*ZstdCompressor, error) {
	cfg := config.ZstdConfig{
		WindowSize:      17,
		UseDict:         false,
		DictSize:        64 * 1024,
		ConcurrencyMode: 1,
	}
	return NewZstdCompressor(cfg, 3)
}

// BenchmarkCompression tests compression performance with different scenarios
func BenchmarkCompression(b *testing.B) {
	compressor, err := createTestCompressor()
	if err != nil {
		b.Fatal(err)
	}

	b.Run("SmallMessages", func(b *testing.B) {
		data := generateRandomData(100) // 100 bytes
		benchmarkCompressionData(b, compressor, data)
	})

	b.Run("MediumMessages", func(b *testing.B) {
		data := generateRandomData(1024) // 1KB
		benchmarkCompressionData(b, compressor, data)
	})

	b.Run("LargeMessages", func(b *testing.B) {
		data := generateRandomData(10240) // 10KB
		benchmarkCompressionData(b, compressor, data)
	})

	b.Run("VeryLargeMessages", func(b *testing.B) {
		data := generateRandomData(102400) // 100KB
		benchmarkCompressionData(b, compressor, data)
	})

	b.Run("HighlyCompressibleData", func(b *testing.B) {
		data := generateCompressibleData(1024) // Repeating patterns
		benchmarkCompressionData(b, compressor, data)
	})

	b.Run("RandomData", func(b *testing.B) {
		data := generateRandomData(1024) // Random, less compressible
		benchmarkCompressionData(b, compressor, data)
	})
}

func benchmarkCompressionData(b *testing.B, compressor *ZstdCompressor, data []byte) {
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			compressed, err := compressor.Compress(data)
			if err != nil {
				b.Fatal(err)
			}

			decompressed, err := compressor.Decompress(compressed)
			if err != nil {
				b.Fatal(err)
			}

			if len(decompressed) != len(data) {
				b.Fatal("Decompressed data size mismatch")
			}
		}
	})

	// Report compression ratio
	compressed, _ := compressor.Compress(data)
	ratio := float64(len(data)) / float64(len(compressed))
	b.ReportMetric(ratio, "compression_ratio")
	b.ReportMetric(float64(len(compressed)), "compressed_bytes")
	b.ReportMetric(float64(len(data)), "original_bytes")
}

func BenchmarkCompressionLevels(b *testing.B) {
	data := generateRandomData(1024)
	cfg := config.ZstdConfig{
		WindowSize:      17,
		UseDict:         false,
		DictSize:        64 * 1024,
		ConcurrencyMode: 1,
	}

	levels := []int{1, 3, 6, 9, 12, 15, 19}

	for _, level := range levels {
		b.Run(fmt.Sprintf("Level_%d", level), func(b *testing.B) {
			compressor, err := NewZstdCompressor(cfg, level)
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				compressed, err := compressor.Compress(data)
				if err != nil {
					b.Fatal(err)
				}

				_, err = compressor.Decompress(compressed)
				if err != nil {
					b.Fatal(err)
				}
			}

			// Report compression metrics for this level
			compressed, _ := compressor.Compress(data)
			ratio := float64(len(data)) / float64(len(compressed))
			b.ReportMetric(ratio, "compression_ratio")
		})
	}
}

func BenchmarkBatchCompression(b *testing.B) {
	compressor, err := createTestCompressor()
	if err != nil {
		b.Fatal(err)
	}

	batchSizes := []int{1, 10, 50, 100, 500, 1000}
	messageSize := 256

	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("BatchSize_%d", batchSize), func(b *testing.B) {
			// Create batch of messages
			messages := make([][]byte, batchSize)
			for i := 0; i < batchSize; i++ {
				messages[i] = generateRandomData(messageSize)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Compress individual messages vs batch
				var totalCompressed int

				// Individual compression
				for _, msg := range messages {
					compressed, _ := compressor.Compress(msg)
					totalCompressed += len(compressed)
				}

				// Batch compression using CompressBatch
				batchCompressed, _ := compressor.CompressBatch(messages)

				_ = totalCompressed
				_ = batchCompressed
			}
		})
	}
}

func BenchmarkCompressionMemoryUsage(b *testing.B) {
	compressor, err := createTestCompressor()
	if err != nil {
		b.Fatal(err)
	}

	data := generateRandomData(1024)

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compressed, _ := compressor.Compress(data)
		_, _ = compressor.Decompress(compressed)
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	b.ReportMetric(float64(m2.Alloc-m1.Alloc)/float64(b.N), "bytes_per_operation")
	b.ReportMetric(float64(m2.TotalAlloc-m1.TotalAlloc)/float64(b.N), "total_bytes_per_operation")
}

func BenchmarkCompressionConcurrency(b *testing.B) {
	compressor, err := createTestCompressor()
	if err != nil {
		b.Fatal(err)
	}

	data := generateRandomData(1024)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			compressed, err := compressor.Compress(data)
			if err != nil {
				b.Fatal(err)
			}

			_, err = compressor.Decompress(compressed)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkCompressionRealWorldData(b *testing.B) {
	compressor, err := createTestCompressor()
	if err != nil {
		b.Fatal(err)
	}

	b.Run("JSONData", func(b *testing.B) {
		jsonData := generateJSONData(1024)
		benchmarkCompressionData(b, compressor, jsonData)
	})

	b.Run("LogData", func(b *testing.B) {
		logData := generateLogData(1024)
		benchmarkCompressionData(b, compressor, logData)
	})

	b.Run("BinaryData", func(b *testing.B) {
		binaryData := generateBinaryData(1024)
		benchmarkCompressionData(b, compressor, binaryData)
	})

	b.Run("TextData", func(b *testing.B) {
		textData := generateTextData(1024)
		benchmarkCompressionData(b, compressor, textData)
	})
}

// Helper functions for generating test data

func generateRandomData(size int) []byte {
	data := make([]byte, size)
	rand.Read(data)
	return data
}

func generateCompressibleData(size int) []byte {
	pattern := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	data := make([]byte, 0, size)

	for len(data) < size {
		remaining := size - len(data)
		if remaining >= len(pattern) {
			data = append(data, pattern...)
		} else {
			data = append(data, pattern[:remaining]...)
		}
	}
	return data
}

func generateJSONData(size int) []byte {
	type TestData struct {
		ID        string                 `json:"id"`
		Name      string                 `json:"name"`
		Email     string                 `json:"email"`
		Timestamp time.Time              `json:"timestamp"`
		Value     float64                `json:"value"`
		Active    bool                   `json:"active"`
		Tags      []string               `json:"tags"`
		Data      map[string]interface{} `json:"data"`
	}

	testObj := TestData{
		ID:        "test-id-12345",
		Name:      "Test User Name",
		Email:     "test@example.com",
		Timestamp: time.Now(),
		Value:     123.456,
		Active:    true,
		Tags:      []string{"tag1", "tag2", "tag3"},
		Data: map[string]interface{}{
			"field1": "value1",
			"field2": 42,
			"field3": true,
		},
	}

	data, _ := json.Marshal(testObj)

	// Repeat to reach desired size
	result := make([]byte, 0, size)
	for len(result) < size {
		remaining := size - len(result)
		if remaining >= len(data) {
			result = append(result, data...)
		} else {
			result = append(result, data[:remaining]...)
		}
	}
	return result
}

func generateLogData(size int) []byte {
	logEntry := fmt.Sprintf("%s [INFO] User login successful for user_id=12345 ip=192.168.1.100 session_id=abc123def456 duration=245ms status=200 bytes_sent=1024 user_agent=\"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36\"", time.Now().Format(time.RFC3339))

	result := make([]byte, 0, size)
	logBytes := []byte(logEntry)

	for len(result) < size {
		remaining := size - len(result)
		if remaining >= len(logBytes) {
			result = append(result, logBytes...)
		} else {
			result = append(result, logBytes[:remaining]...)
		}
	}
	return result
}

func generateBinaryData(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 256)
	}
	return data
}

func generateTextData(size int) []byte {
	text := "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum."

	result := make([]byte, 0, size)
	textBytes := []byte(text)

	for len(result) < size {
		remaining := size - len(result)
		if remaining >= len(textBytes) {
			result = append(result, textBytes...)
		} else {
			result = append(result, textBytes[:remaining]...)
		}
	}
	return result
}
