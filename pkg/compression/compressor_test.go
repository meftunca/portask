package compression

import (
	"bytes"
	"testing"

	"github.com/meftunca/portask/pkg/config"
)

func TestCompressorFactory(t *testing.T) {
	t.Run("NewCompressorFactory", func(t *testing.T) {
		factory := NewCompressorFactory()
		if factory == nil {
			t.Fatal("Expected factory to be created")
		}
		if factory.compressors == nil {
			t.Fatal("Expected compressors map to be initialized")
		}
	})

	t.Run("InitializeDefaultCompressors", func(t *testing.T) {
		factory := NewCompressorFactory()
		cfg := config.DefaultConfig()

		// Override problematic settings for testing
		cfg.Compression.ZstdConfig.WindowSize = 1024   // Use 1024 bytes (minimum)
		cfg.Compression.LZ4Config.CompressionLevel = 0 // Use default level

		err := factory.InitializeDefaultCompressors(cfg)
		if err != nil {
			t.Fatalf("Failed to initialize default compressors: %v", err)
		}

		// Test that compressors are registered
		expectedTypes := []config.CompressionType{
			config.CompressionZstd,
			config.CompressionLZ4,
			config.CompressionSnappy,
			config.CompressionGzip,
			config.CompressionBrotli,
		}

		for _, compType := range expectedTypes {
			compressor, err := factory.GetCompressor(compType)
			if err != nil {
				t.Errorf("Failed to get compressor %s: %v", compType, err)
			}
			if compressor == nil {
				t.Errorf("Expected compressor %s to be registered", compType)
			}
		}
	})

	t.Run("GetCompressor", func(t *testing.T) {
		factory := NewCompressorFactory()
		cfg := config.DefaultConfig()

		// Override problematic settings for testing
		cfg.Compression.ZstdConfig.WindowSize = 1024   // Use 1024 bytes (minimum)
		cfg.Compression.LZ4Config.CompressionLevel = 0 // Use default level

		factory.InitializeDefaultCompressors(cfg)

		// Test getting existing compressor
		compressor, err := factory.GetCompressor(config.CompressionZstd)
		if err != nil {
			t.Fatalf("Failed to get Zstd compressor: %v", err)
		}
		if compressor == nil {
			t.Fatal("Expected compressor to be returned")
		}

		// Test getting none compressor (special case)
		noneCompressor, err := factory.GetCompressor(config.CompressionNone)
		if err != nil {
			t.Fatalf("Failed to get None compressor: %v", err)
		}
		if noneCompressor == nil {
			t.Fatal("Expected none compressor to be returned")
		}

		// Test getting non-existing compressor
		_, err = factory.GetCompressor("nonexistent")
		if err == nil {
			t.Fatal("Expected error for non-existing compressor")
		}
	})
}

func TestNoCompressor(t *testing.T) {
	compressor := &NoCompressor{}
	testData := []byte("Test data for no compression")

	t.Run("Compress", func(t *testing.T) {
		compressed, err := compressor.Compress(testData)
		if err != nil {
			t.Fatalf("Failed to compress: %v", err)
		}
		if !bytes.Equal(compressed, testData) {
			t.Error("No compression should return original data")
		}
	})

	t.Run("Decompress", func(t *testing.T) {
		decompressed, err := compressor.Decompress(testData)
		if err != nil {
			t.Fatalf("Failed to decompress: %v", err)
		}
		if !bytes.Equal(decompressed, testData) {
			t.Error("No decompression should return original data")
		}
	})

	t.Run("Properties", func(t *testing.T) {
		if compressor.Name() != "none" {
			t.Errorf("Expected name 'none', got %s", compressor.Name())
		}
	})
}

func TestZstdCompressor(t *testing.T) {
	compressor, err := NewZstdCompressor(config.ZstdConfig{
		WindowSize:      1024, // Use 1024 bytes (minimum)
		UseDict:         false,
		DictSize:        16384,
		ConcurrencyMode: 1,
	}, 3)
	if err != nil {
		t.Fatalf("Failed to create Zstd compressor: %v", err)
	}

	testData := []byte("This is test data for Zstd compression. It should compress well due to repetitive nature. This is test data for Zstd compression.")

	t.Run("Compress", func(t *testing.T) {
		compressed, err := compressor.Compress(testData)
		if err != nil {
			t.Fatalf("Failed to compress: %v", err)
		}
		if len(compressed) == 0 {
			t.Fatal("Expected compressed data to have content")
		}
		// Zstd should compress repetitive data
		if len(compressed) >= len(testData) {
			t.Logf("Warning: Compressed size (%d) >= original size (%d)", len(compressed), len(testData))
		}
	})

	t.Run("Decompress", func(t *testing.T) {
		compressed, err := compressor.Compress(testData)
		if err != nil {
			t.Fatalf("Failed to compress: %v", err)
		}

		decompressed, err := compressor.Decompress(compressed)
		if err != nil {
			t.Fatalf("Failed to decompress: %v", err)
		}

		if !bytes.Equal(decompressed, testData) {
			t.Error("Decompressed data doesn't match original")
		}
	})

	t.Run("Properties", func(t *testing.T) {
		if compressor.Name() != "zstd" {
			t.Errorf("Expected name 'zstd', got %s", compressor.Name())
		}
	})
}

func TestLZ4Compressor(t *testing.T) {
	compressor := NewLZ4Compressor(config.LZ4Config{
		CompressionLevel: 0, // Use default level
		FastAcceleration: 1,
		BlockMode:        true,
	})

	testData := []byte("This is test data for LZ4 compression. LZ4 is fast. This is test data for LZ4 compression.")

	t.Run("Compress", func(t *testing.T) {
		compressed, err := compressor.Compress(testData)
		if err != nil {
			t.Fatalf("Failed to compress: %v", err)
		}
		if len(compressed) == 0 {
			t.Fatal("Expected compressed data to have content")
		}
	})

	t.Run("Decompress", func(t *testing.T) {
		compressed, err := compressor.Compress(testData)
		if err != nil {
			t.Fatalf("Failed to compress: %v", err)
		}

		decompressed, err := compressor.Decompress(compressed)
		if err != nil {
			t.Fatalf("Failed to decompress: %v", err)
		}

		if !bytes.Equal(decompressed, testData) {
			t.Error("Decompressed data doesn't match original")
		}
	})

	t.Run("Properties", func(t *testing.T) {
		if compressor.Name() != "lz4" {
			t.Errorf("Expected name 'lz4', got %s", compressor.Name())
		}
	})
}

func TestSnappyCompressor(t *testing.T) {
	compressor := NewSnappyCompressor()
	testData := []byte("This is test data for Snappy compression. Snappy is fast. This is test data for Snappy compression.")

	t.Run("Compress", func(t *testing.T) {
		compressed, err := compressor.Compress(testData)
		if err != nil {
			t.Fatalf("Failed to compress: %v", err)
		}
		if len(compressed) == 0 {
			t.Fatal("Expected compressed data to have content")
		}
	})

	t.Run("Decompress", func(t *testing.T) {
		compressed, err := compressor.Compress(testData)
		if err != nil {
			t.Fatalf("Failed to compress: %v", err)
		}

		decompressed, err := compressor.Decompress(compressed)
		if err != nil {
			t.Fatalf("Failed to decompress: %v", err)
		}

		if !bytes.Equal(decompressed, testData) {
			t.Error("Decompressed data doesn't match original")
		}
	})

	t.Run("Properties", func(t *testing.T) {
		if compressor.Name() != "snappy" {
			t.Errorf("Expected name 'snappy', got %s", compressor.Name())
		}
	})
}

func TestGzipCompressor(t *testing.T) {
	compressor := NewGzipCompressor(6)
	testData := []byte("This is test data for Gzip compression. Gzip has good compression ratio. This is test data for Gzip compression.")

	t.Run("Compress", func(t *testing.T) {
		compressed, err := compressor.Compress(testData)
		if err != nil {
			t.Fatalf("Failed to compress: %v", err)
		}
		if len(compressed) == 0 {
			t.Fatal("Expected compressed data to have content")
		}
	})

	t.Run("Decompress", func(t *testing.T) {
		compressed, err := compressor.Compress(testData)
		if err != nil {
			t.Fatalf("Failed to compress: %v", err)
		}

		decompressed, err := compressor.Decompress(compressed)
		if err != nil {
			t.Fatalf("Failed to decompress: %v", err)
		}

		if !bytes.Equal(decompressed, testData) {
			t.Error("Decompressed data doesn't match original")
		}
	})

	t.Run("Properties", func(t *testing.T) {
		if compressor.Name() != "gzip" {
			t.Errorf("Expected name 'gzip', got %s", compressor.Name())
		}
	})
}

func TestBrotliCompressor(t *testing.T) {
	compressor := NewBrotliCompressor(6)
	testData := []byte("This is test data for Brotli compression. Brotli has excellent compression ratio. This is test data for Brotli compression.")

	t.Run("Compress", func(t *testing.T) {
		compressed, err := compressor.Compress(testData)
		if err != nil {
			t.Fatalf("Failed to compress: %v", err)
		}
		if len(compressed) == 0 {
			t.Fatal("Expected compressed data to have content")
		}
	})

	t.Run("Decompress", func(t *testing.T) {
		compressed, err := compressor.Compress(testData)
		if err != nil {
			t.Fatalf("Failed to compress: %v", err)
		}

		decompressed, err := compressor.Decompress(compressed)
		if err != nil {
			t.Fatalf("Failed to decompress: %v", err)
		}

		if !bytes.Equal(decompressed, testData) {
			t.Error("Decompressed data doesn't match original")
		}
	})

	t.Run("Properties", func(t *testing.T) {
		if compressor.Name() != "brotli" {
			t.Errorf("Expected name 'brotli', got %s", compressor.Name())
		}
	})
}

// Benchmark tests
func BenchmarkCompressionAlgorithms(b *testing.B) {
	// Create test data with different characteristics
	testCases := []struct {
		name string
		data []byte
	}{
		{
			name: "Small",
			data: []byte("Small test data for compression benchmarking"),
		},
		{
			name: "Medium",
			data: bytes.Repeat([]byte("This is medium test data for compression benchmarking. "), 10),
		},
		{
			name: "Large",
			data: bytes.Repeat([]byte("This is large test data for compression benchmarking with lots of repetitive content. "), 100),
		},
	}

	// Create compressors
	cfg := config.DefaultConfig()
	factory := NewCompressorFactory()
	factory.InitializeDefaultCompressors(cfg)

	compressors := []struct {
		name string
		comp Compressor
	}{
		{"None", &NoCompressor{}},
	}

	// Add other compressors if they're available
	if zstd, err := factory.GetCompressor(config.CompressionZstd); err == nil {
		compressors = append(compressors, struct {
			name string
			comp Compressor
		}{"Zstd", zstd})
	}

	if lz4, err := factory.GetCompressor(config.CompressionLZ4); err == nil {
		compressors = append(compressors, struct {
			name string
			comp Compressor
		}{"LZ4", lz4})
	}

	if snappy, err := factory.GetCompressor(config.CompressionSnappy); err == nil {
		compressors = append(compressors, struct {
			name string
			comp Compressor
		}{"Snappy", snappy})
	}

	for _, tc := range testCases {
		for _, comp := range compressors {
			b.Run(tc.name+"_"+comp.name+"_Compress", func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_, err := comp.comp.Compress(tc.data)
					if err != nil {
						b.Fatal(err)
					}
				}
			})

			// Pre-compress for decompress benchmarks
			compressed, err := comp.comp.Compress(tc.data)
			if err != nil {
				b.Fatalf("Failed to compress test data: %v", err)
			}

			b.Run(tc.name+"_"+comp.name+"_Decompress", func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_, err := comp.comp.Decompress(compressed)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func BenchmarkCompressionRatios(b *testing.B) {
	testData := bytes.Repeat([]byte("This is test data with repetitive content for measuring compression ratios. "), 50)

	cfg := config.DefaultConfig()
	factory := NewCompressorFactory()
	factory.InitializeDefaultCompressors(cfg)

	compTypes := []config.CompressionType{
		config.CompressionNone,
		config.CompressionZstd,
		config.CompressionLZ4,
		config.CompressionSnappy,
		config.CompressionGzip,
		config.CompressionBrotli,
	}

	b.Logf("Original data size: %d bytes", len(testData))

	for _, compType := range compTypes {
		comp, err := factory.GetCompressor(compType)
		if err != nil {
			b.Logf("Skipping %s: %v", compType, err)
			continue
		}

		compressed, err := comp.Compress(testData)
		if err != nil {
			b.Logf("Failed to compress with %s: %v", compType, err)
			continue
		}

		ratio := float64(len(testData)) / float64(len(compressed))
		b.Logf("%s: %d bytes, ratio: %.2fx", compType, len(compressed), ratio)
	}
}
