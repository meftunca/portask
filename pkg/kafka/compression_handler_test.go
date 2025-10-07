package kafka

import (
	"bytes"
	"testing"
)

func TestCompressionHandler_Gzip(t *testing.T) {
	ch, err := NewCompressionHandler()
	if err != nil {
		t.Fatalf("Failed to create compression handler: %v", err)
	}
	defer ch.Close()

	testData := []byte("Hello, Kafka! This is a test message for compression.")

	// Compress
	compressed, err := ch.Compress(testData, CompressionGzip)
	if err != nil {
		t.Fatalf("Failed to compress: %v", err)
	}

	if len(compressed) >= len(testData) {
		t.Logf("Warning: Compressed data (%d bytes) is not smaller than original (%d bytes)", len(compressed), len(testData))
	}

	// Decompress
	decompressed, err := ch.Decompress(compressed, CompressionGzip)
	if err != nil {
		t.Fatalf("Failed to decompress: %v", err)
	}

	if !bytes.Equal(testData, decompressed) {
		t.Error("Decompressed data does not match original")
	}
}

func TestCompressionHandler_Snappy(t *testing.T) {
	ch, err := NewCompressionHandler()
	if err != nil {
		t.Fatalf("Failed to create compression handler: %v", err)
	}
	defer ch.Close()

	testData := []byte("Hello, Kafka! This is a test message for Snappy compression.")

	// Compress
	compressed, err := ch.Compress(testData, CompressionSnappy)
	if err != nil {
		t.Fatalf("Failed to compress: %v", err)
	}

	// Decompress
	decompressed, err := ch.Decompress(compressed, CompressionSnappy)
	if err != nil {
		t.Fatalf("Failed to decompress: %v", err)
	}

	if !bytes.Equal(testData, decompressed) {
		t.Error("Decompressed data does not match original")
	}
}

func TestCompressionHandler_LZ4(t *testing.T) {
	ch, err := NewCompressionHandler()
	if err != nil {
		t.Fatalf("Failed to create compression handler: %v", err)
	}
	defer ch.Close()

	testData := []byte("Hello, Kafka! This is a test message for LZ4 compression.")

	// Compress
	compressed, err := ch.Compress(testData, CompressionLZ4)
	if err != nil {
		t.Fatalf("Failed to compress: %v", err)
	}

	// Decompress
	decompressed, err := ch.Decompress(compressed, CompressionLZ4)
	if err != nil {
		t.Fatalf("Failed to decompress: %v", err)
	}

	if !bytes.Equal(testData, decompressed) {
		t.Error("Decompressed data does not match original")
	}
}

func TestCompressionHandler_Zstd(t *testing.T) {
	ch, err := NewCompressionHandler()
	if err != nil {
		t.Fatalf("Failed to create compression handler: %v", err)
	}
	defer ch.Close()

	testData := []byte("Hello, Kafka! This is a test message for Zstd compression.")

	// Compress
	compressed, err := ch.Compress(testData, CompressionZstd)
	if err != nil {
		t.Fatalf("Failed to compress: %v", err)
	}

	// Decompress
	decompressed, err := ch.Decompress(compressed, CompressionZstd)
	if err != nil {
		t.Fatalf("Failed to decompress: %v", err)
	}

	if !bytes.Equal(testData, decompressed) {
		t.Error("Decompressed data does not match original")
	}
}

func TestCompressionHandler_None(t *testing.T) {
	ch, err := NewCompressionHandler()
	if err != nil {
		t.Fatalf("Failed to create compression handler: %v", err)
	}
	defer ch.Close()

	testData := []byte("No compression test")

	// Compress (should return original)
	compressed, err := ch.Compress(testData, CompressionNone)
	if err != nil {
		t.Fatalf("Failed to compress: %v", err)
	}

	if !bytes.Equal(testData, compressed) {
		t.Error("No compression should return original data")
	}

	// Decompress (should return original)
	decompressed, err := ch.Decompress(compressed, CompressionNone)
	if err != nil {
		t.Fatalf("Failed to decompress: %v", err)
	}

	if !bytes.Equal(testData, decompressed) {
		t.Error("Decompressed data does not match original")
	}
}

func TestCompressionHandler_UnsupportedCodec(t *testing.T) {
	ch, err := NewCompressionHandler()
	if err != nil {
		t.Fatalf("Failed to create compression handler: %v", err)
	}
	defer ch.Close()

	testData := []byte("Test data")

	// Try to compress with unsupported codec
	_, err = ch.Compress(testData, CompressionType(99))
	if err == nil {
		t.Error("Expected error for unsupported codec")
	}
}

func TestCompressionHandler_DetectBestCodec(t *testing.T) {
	ch, err := NewCompressionHandler()
	if err != nil {
		t.Fatalf("Failed to create compression handler: %v", err)
	}
	defer ch.Close()

	// Highly compressible data
	testData := bytes.Repeat([]byte("AAAAAAAA"), 100)

	bestCodec, err := ch.DetectBestCodec(testData)
	if err != nil {
		t.Fatalf("Failed to detect best codec: %v", err)
	}

	if bestCodec == CompressionNone {
		t.Error("Expected compression codec to be chosen for compressible data")
	}

	t.Logf("Best codec for compressible data: %s", GetCodecName(bestCodec))
}

func TestCompressionHandler_GetCompressionRatio(t *testing.T) {
	ch, err := NewCompressionHandler()
	if err != nil {
		t.Fatalf("Failed to create compression handler: %v", err)
	}
	defer ch.Close()

	testData := bytes.Repeat([]byte("AAAAAAAA"), 100)

	compressed, err := ch.Compress(testData, CompressionGzip)
	if err != nil {
		t.Fatalf("Failed to compress: %v", err)
	}

	ratio := ch.GetCompressionRatio(testData, compressed)

	if ratio <= 0 || ratio > 1 {
		t.Errorf("Expected compression ratio between 0 and 1, got %f", ratio)
	}

	t.Logf("Compression ratio: %.2f%%", ratio*100)
}

func TestGetCodecName(t *testing.T) {
	tests := []struct {
		codec    CompressionType
		expected string
	}{
		{CompressionNone, "none"},
		{CompressionGzip, "gzip"},
		{CompressionSnappy, "snappy"},
		{CompressionLZ4, "lz4"},
		{CompressionZstd, "zstd"},
		{CompressionType(99), "unknown"},
	}

	for _, tt := range tests {
		name := GetCodecName(tt.codec)
		if name != tt.expected {
			t.Errorf("Expected codec name '%s', got '%s'", tt.expected, name)
		}
	}
}

func TestParseCodecName(t *testing.T) {
	tests := []struct {
		name     string
		expected CompressionType
		wantErr  bool
	}{
		{"none", CompressionNone, false},
		{"gzip", CompressionGzip, false},
		{"snappy", CompressionSnappy, false},
		{"lz4", CompressionLZ4, false},
		{"zstd", CompressionZstd, false},
		{"invalid", CompressionNone, true},
	}

	for _, tt := range tests {
		codec, err := ParseCodecName(tt.name)
		if tt.wantErr {
			if err == nil {
				t.Errorf("Expected error for codec name '%s'", tt.name)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for codec name '%s': %v", tt.name, err)
			}
			if codec != tt.expected {
				t.Errorf("Expected codec %d, got %d", tt.expected, codec)
			}
		}
	}
}

func TestCompressionHandler_LargeData(t *testing.T) {
	ch, err := NewCompressionHandler()
	if err != nil {
		t.Fatalf("Failed to create compression handler: %v", err)
	}
	defer ch.Close()

	// 1MB of data
	testData := bytes.Repeat([]byte("This is a test message for large data compression. "), 20000)

	codecs := []CompressionType{
		CompressionGzip,
		CompressionSnappy,
		CompressionLZ4,
		CompressionZstd,
	}

	for _, codec := range codecs {
		t.Run(GetCodecName(codec), func(t *testing.T) {
			compressed, err := ch.Compress(testData, codec)
			if err != nil {
				t.Fatalf("Failed to compress: %v", err)
			}

			decompressed, err := ch.Decompress(compressed, codec)
			if err != nil {
				t.Fatalf("Failed to decompress: %v", err)
			}

			if !bytes.Equal(testData, decompressed) {
				t.Error("Decompressed data does not match original")
			}

			ratio := ch.GetCompressionRatio(testData, compressed)
			t.Logf("Compression ratio for %s: %.2f%% (original: %d bytes, compressed: %d bytes)",
				GetCodecName(codec), ratio*100, len(testData), len(compressed))
		})
	}
}

// Benchmarks

func BenchmarkCompression_Gzip(b *testing.B) {
	ch, _ := NewCompressionHandler()
	defer ch.Close()

	testData := bytes.Repeat([]byte("Hello, Kafka! "), 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch.Compress(testData, CompressionGzip)
	}
}

func BenchmarkCompression_Snappy(b *testing.B) {
	ch, _ := NewCompressionHandler()
	defer ch.Close()

	testData := bytes.Repeat([]byte("Hello, Kafka! "), 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch.Compress(testData, CompressionSnappy)
	}
}

func BenchmarkCompression_LZ4(b *testing.B) {
	ch, _ := NewCompressionHandler()
	defer ch.Close()

	testData := bytes.Repeat([]byte("Hello, Kafka! "), 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch.Compress(testData, CompressionLZ4)
	}
}

func BenchmarkCompression_Zstd(b *testing.B) {
	ch, _ := NewCompressionHandler()
	defer ch.Close()

	testData := bytes.Repeat([]byte("Hello, Kafka! "), 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch.Compress(testData, CompressionZstd)
	}
}

func BenchmarkDecompression_Gzip(b *testing.B) {
	ch, _ := NewCompressionHandler()
	defer ch.Close()

	testData := bytes.Repeat([]byte("Hello, Kafka! "), 100)
	compressed, _ := ch.Compress(testData, CompressionGzip)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch.Decompress(compressed, CompressionGzip)
	}
}

func BenchmarkDecompression_Snappy(b *testing.B) {
	ch, _ := NewCompressionHandler()
	defer ch.Close()

	testData := bytes.Repeat([]byte("Hello, Kafka! "), 100)
	compressed, _ := ch.Compress(testData, CompressionSnappy)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch.Decompress(compressed, CompressionSnappy)
	}
}

func BenchmarkDecompression_Zstd(b *testing.B) {
	ch, _ := NewCompressionHandler()
	defer ch.Close()

	testData := bytes.Repeat([]byte("Hello, Kafka! "), 100)
	compressed, _ := ch.Compress(testData, CompressionZstd)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch.Decompress(compressed, CompressionZstd)
	}
}

