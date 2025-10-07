package kafka

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"

	"github.com/golang/snappy"
	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
)

// CompressionType represents Kafka compression codec
type CompressionType int8

const (
	CompressionNone   CompressionType = 0
	CompressionGzip   CompressionType = 1
	CompressionSnappy CompressionType = 2
	CompressionLZ4    CompressionType = 3
	CompressionZstd   CompressionType = 4
)

// CompressionHandler handles message compression/decompression
type CompressionHandler struct {
	gzipLevel int
	zstdEnc   *zstd.Encoder
	zstdDec   *zstd.Decoder
}

// NewCompressionHandler creates a new compression handler
func NewCompressionHandler() (*CompressionHandler, error) {
	// Initialize Zstd encoder/decoder
	zstdEnc, err := zstd.NewWriter(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd encoder: %w", err)
	}

	zstdDec, err := zstd.NewReader(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd decoder: %w", err)
	}

	return &CompressionHandler{
		gzipLevel: gzip.DefaultCompression,
		zstdEnc:   zstdEnc,
		zstdDec:   zstdDec,
	}, nil
}

// Compress compresses data using the specified codec
func (ch *CompressionHandler) Compress(data []byte, codec CompressionType) ([]byte, error) {
	switch codec {
	case CompressionNone:
		return data, nil

	case CompressionGzip:
		return ch.compressGzip(data)

	case CompressionSnappy:
		return ch.compressSnappy(data)

	case CompressionLZ4:
		return ch.compressLZ4(data)

	case CompressionZstd:
		return ch.compressZstd(data)

	default:
		return nil, fmt.Errorf("unsupported compression codec: %d", codec)
	}
}

// Decompress decompresses data using the specified codec
func (ch *CompressionHandler) Decompress(data []byte, codec CompressionType) ([]byte, error) {
	switch codec {
	case CompressionNone:
		return data, nil

	case CompressionGzip:
		return ch.decompressGzip(data)

	case CompressionSnappy:
		return ch.decompressSnappy(data)

	case CompressionLZ4:
		return ch.decompressLZ4(data)

	case CompressionZstd:
		return ch.decompressZstd(data)

	default:
		return nil, fmt.Errorf("unsupported compression codec: %d", codec)
	}
}

// Gzip compression/decompression

func (ch *CompressionHandler) compressGzip(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buf, ch.gzipLevel)
	if err != nil {
		return nil, err
	}

	if _, err := writer.Write(data); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (ch *CompressionHandler) decompressGzip(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

// Snappy compression/decompression

func (ch *CompressionHandler) compressSnappy(data []byte) ([]byte, error) {
	return snappy.Encode(nil, data), nil
}

func (ch *CompressionHandler) decompressSnappy(data []byte) ([]byte, error) {
	return snappy.Decode(nil, data)
}

// LZ4 compression/decompression

func (ch *CompressionHandler) compressLZ4(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := lz4.NewWriter(&buf)

	if _, err := writer.Write(data); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (ch *CompressionHandler) decompressLZ4(data []byte) ([]byte, error) {
	reader := lz4.NewReader(bytes.NewReader(data))
	return io.ReadAll(reader)
}

// Zstd compression/decompression

func (ch *CompressionHandler) compressZstd(data []byte) ([]byte, error) {
	return ch.zstdEnc.EncodeAll(data, nil), nil
}

func (ch *CompressionHandler) decompressZstd(data []byte) ([]byte, error) {
	return ch.zstdDec.DecodeAll(data, nil)
}

// GetCompressionRatio returns the compression ratio
func (ch *CompressionHandler) GetCompressionRatio(original, compressed []byte) float64 {
	if len(original) == 0 {
		return 0
	}
	return float64(len(compressed)) / float64(len(original))
}

// DetectBestCodec tries all codecs and returns the one with best compression
func (ch *CompressionHandler) DetectBestCodec(data []byte) (CompressionType, error) {
	codecs := []CompressionType{
		CompressionGzip,
		CompressionSnappy,
		CompressionLZ4,
		CompressionZstd,
	}

	bestCodec := CompressionNone
	bestSize := len(data)

	for _, codec := range codecs {
		compressed, err := ch.Compress(data, codec)
		if err != nil {
			continue
		}

		if len(compressed) < bestSize {
			bestSize = len(compressed)
			bestCodec = codec
		}
	}

	return bestCodec, nil
}

// GetCodecName returns the name of a compression codec
func GetCodecName(codec CompressionType) string {
	switch codec {
	case CompressionNone:
		return "none"
	case CompressionGzip:
		return "gzip"
	case CompressionSnappy:
		return "snappy"
	case CompressionLZ4:
		return "lz4"
	case CompressionZstd:
		return "zstd"
	default:
		return "unknown"
	}
}

// ParseCodecName parses a codec name string
func ParseCodecName(name string) (CompressionType, error) {
	switch name {
	case "none":
		return CompressionNone, nil
	case "gzip":
		return CompressionGzip, nil
	case "snappy":
		return CompressionSnappy, nil
	case "lz4":
		return CompressionLZ4, nil
	case "zstd":
		return CompressionZstd, nil
	default:
		return CompressionNone, fmt.Errorf("unknown codec: %s", name)
	}
}

// Close releases resources
func (ch *CompressionHandler) Close() error {
	if ch.zstdEnc != nil {
		ch.zstdEnc.Close()
	}
	if ch.zstdDec != nil {
		ch.zstdDec.Close()
	}
	return nil
}

