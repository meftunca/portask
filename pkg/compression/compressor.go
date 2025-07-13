package compression

import (
	"bytes"
	"compress/gzip"
	"io"
	"sync"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/snappy"
	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"

	"github.com/meftunca/portask/pkg/config"
	"github.com/meftunca/portask/pkg/types"
)

// Compressor defines the interface for compression algorithms
type Compressor interface {
	// Compress compresses data
	Compress(data []byte) ([]byte, error)

	// Decompress decompresses data
	Decompress(data []byte) ([]byte, error)

	// CompressBatch compresses multiple data chunks together for better compression
	CompressBatch(data [][]byte) ([]byte, error)

	// DecompressBatch decompresses batch data back to individual chunks
	DecompressBatch(data []byte) ([][]byte, error)

	// Name returns the compressor name
	Name() string

	// Ratio estimates compression ratio for given data
	EstimateRatio(data []byte) float64

	// MinSize returns minimum size threshold for compression efficiency
	MinSize() int
}

// CompressorFactory creates compressors based on configuration
type CompressorFactory struct {
	compressors map[config.CompressionType]Compressor
	mutex       sync.RWMutex
}

// NewCompressorFactory creates a new compressor factory
func NewCompressorFactory() *CompressorFactory {
	return &CompressorFactory{
		compressors: make(map[config.CompressionType]Compressor),
	}
}

// RegisterCompressor registers a compressor for a compression type
func (f *CompressorFactory) RegisterCompressor(compType config.CompressionType, compressor Compressor) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.compressors[compType] = compressor
}

// GetCompressor returns a compressor for the specified compression type
func (f *CompressorFactory) GetCompressor(compType config.CompressionType) (Compressor, error) {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	if compType == config.CompressionNone {
		return &NoCompressor{}, nil
	}

	compressor, exists := f.compressors[compType]
	if !exists {
		return nil, types.NewPortaskError(types.ErrCodeCompressionError, "unsupported compression type").
			WithDetail("type", compType)
	}

	return compressor, nil
}

// InitializeDefaultCompressors initializes all default compressors
func (f *CompressorFactory) InitializeDefaultCompressors(cfg *config.Config) error {
	// Zstd Compressor
	zstdCompressor, err := NewZstdCompressor(cfg.Compression.ZstdConfig, cfg.Compression.Level)
	if err != nil {
		return err
	}
	f.RegisterCompressor(config.CompressionZstd, zstdCompressor)

	// LZ4 Compressor
	lz4Compressor := NewLZ4Compressor(cfg.Compression.LZ4Config)
	f.RegisterCompressor(config.CompressionLZ4, lz4Compressor)

	// Snappy Compressor
	snappyCompressor := NewSnappyCompressor()
	f.RegisterCompressor(config.CompressionSnappy, snappyCompressor)

	// Gzip Compressor
	gzipCompressor := NewGzipCompressor(cfg.Compression.Level)
	f.RegisterCompressor(config.CompressionGzip, gzipCompressor)

	// Brotli Compressor
	brotliCompressor := NewBrotliCompressor(cfg.Compression.Level)
	f.RegisterCompressor(config.CompressionBrotli, brotliCompressor)

	return nil
}

// NoCompressor implements a no-op compressor
type NoCompressor struct{}

func (n *NoCompressor) Compress(data []byte) ([]byte, error) {
	return data, nil
}

func (n *NoCompressor) Decompress(data []byte) ([]byte, error) {
	return data, nil
}

func (n *NoCompressor) CompressBatch(data [][]byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	if len(data) == 1 {
		return data[0], nil
	}

	// Concatenate all data with length prefixes
	var buf bytes.Buffer
	for _, chunk := range data {
		// Write length prefix (4 bytes)
		length := uint32(len(chunk))
		buf.WriteByte(byte(length >> 24))
		buf.WriteByte(byte(length >> 16))
		buf.WriteByte(byte(length >> 8))
		buf.WriteByte(byte(length))
		buf.Write(chunk)
	}

	return buf.Bytes(), nil
}

func (n *NoCompressor) DecompressBatch(data []byte) ([][]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}

	var result [][]byte
	buf := bytes.NewReader(data)

	for buf.Len() > 0 {
		// Read length prefix
		var lengthBytes [4]byte
		if _, err := buf.Read(lengthBytes[:]); err != nil {
			if err == io.EOF {
				break
			}
			return nil, types.ErrDecompressionError("none", err)
		}

		length := uint32(lengthBytes[0])<<24 | uint32(lengthBytes[1])<<16 |
			uint32(lengthBytes[2])<<8 | uint32(lengthBytes[3])

		// Read chunk
		chunk := make([]byte, length)
		if _, err := buf.Read(chunk); err != nil {
			return nil, types.ErrDecompressionError("none", err)
		}

		result = append(result, chunk)
	}

	return result, nil
}

func (n *NoCompressor) Name() string {
	return "none"
}

func (n *NoCompressor) EstimateRatio(data []byte) float64 {
	return 1.0 // No compression
}

func (n *NoCompressor) MinSize() int {
	return 0
}

// ZstdCompressor implements Zstandard compression
type ZstdCompressor struct {
	encoder *zstd.Encoder
	decoder *zstd.Decoder
	level   zstd.EncoderLevel

	// Pools for encoder/decoder reuse
	encoderPool sync.Pool
	decoderPool sync.Pool
}

// NewZstdCompressor creates a new Zstd compressor
func NewZstdCompressor(cfg config.ZstdConfig, level int) (*ZstdCompressor, error) {
	encoderLevel := zstd.EncoderLevel(level)

	// Create encoder with optimized settings
	encoder, err := zstd.NewWriter(nil,
		zstd.WithEncoderLevel(encoderLevel),
		zstd.WithEncoderConcurrency(cfg.ConcurrencyMode),
		zstd.WithWindowSize(cfg.WindowSize),
		zstd.WithZeroFrames(true), // Reduce overhead
	)
	if err != nil {
		return nil, types.ErrCompressionError("zstd", err)
	}

	// Create decoder
	decoder, err := zstd.NewReader(nil,
		zstd.WithDecoderConcurrency(cfg.ConcurrencyMode),
		zstd.WithDecoderMaxMemory(64<<20), // 64MB max memory
	)
	if err != nil {
		encoder.Close()
		return nil, types.ErrCompressionError("zstd", err)
	}

	comp := &ZstdCompressor{
		encoder: encoder,
		decoder: decoder,
		level:   encoderLevel,
	}

	// Initialize pools
	comp.encoderPool.New = func() interface{} {
		enc, _ := zstd.NewWriter(nil,
			zstd.WithEncoderLevel(encoderLevel),
			zstd.WithEncoderConcurrency(cfg.ConcurrencyMode),
			zstd.WithWindowSize(cfg.WindowSize),
			zstd.WithZeroFrames(true),
		)
		return enc
	}

	comp.decoderPool.New = func() interface{} {
		dec, _ := zstd.NewReader(nil,
			zstd.WithDecoderConcurrency(cfg.ConcurrencyMode),
			zstd.WithDecoderMaxMemory(64<<20),
		)
		return dec
	}

	return comp, nil
}

func (z *ZstdCompressor) Compress(data []byte) ([]byte, error) {
	encoder := z.encoderPool.Get().(*zstd.Encoder)
	defer z.encoderPool.Put(encoder)

	return encoder.EncodeAll(data, nil), nil
}

func (z *ZstdCompressor) Decompress(data []byte) ([]byte, error) {
	decoder := z.decoderPool.Get().(*zstd.Decoder)
	defer z.decoderPool.Put(decoder)

	result, err := decoder.DecodeAll(data, nil)
	if err != nil {
		return nil, types.ErrDecompressionError("zstd", err)
	}

	return result, nil
}

func (z *ZstdCompressor) CompressBatch(data [][]byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}

	encoder := z.encoderPool.Get().(*zstd.Encoder)
	defer z.encoderPool.Put(encoder)

	// Estimate total size and create buffer
	totalSize := 0
	for _, chunk := range data {
		totalSize += len(chunk) + 4 // +4 for length prefix
	}

	var buf bytes.Buffer
	buf.Grow(totalSize)

	// Write all chunks with length prefixes
	for _, chunk := range data {
		length := uint32(len(chunk))
		buf.WriteByte(byte(length >> 24))
		buf.WriteByte(byte(length >> 16))
		buf.WriteByte(byte(length >> 8))
		buf.WriteByte(byte(length))
		buf.Write(chunk)
	}

	// Compress the concatenated data
	return encoder.EncodeAll(buf.Bytes(), nil), nil
}

func (z *ZstdCompressor) DecompressBatch(data []byte) ([][]byte, error) {
	// First decompress
	decompressed, err := z.Decompress(data)
	if err != nil {
		return nil, err
	}

	// Then split into chunks
	return (&NoCompressor{}).DecompressBatch(decompressed)
}

func (z *ZstdCompressor) Name() string {
	return "zstd"
}

func (z *ZstdCompressor) EstimateRatio(data []byte) float64 {
	// Rough estimation based on data characteristics
	if len(data) < 64 {
		return 1.0 // Don't compress very small data
	}

	// Simple entropy estimation
	freq := make(map[byte]int)
	for _, b := range data {
		freq[b]++
	}

	if len(freq) > 200 {
		return 0.9 // Low compression for high entropy data
	}
	if len(freq) < 20 {
		return 0.3 // High compression for low entropy data
	}

	return 0.6 // Average case
}

func (z *ZstdCompressor) MinSize() int {
	return 64 // Don't compress data smaller than 64 bytes
}

// LZ4Compressor implements LZ4 compression
type LZ4Compressor struct {
	cfg config.LZ4Config
}

func NewLZ4Compressor(cfg config.LZ4Config) *LZ4Compressor {
	return &LZ4Compressor{cfg: cfg}
}

func (l *LZ4Compressor) Compress(data []byte) ([]byte, error) {
	if l.cfg.BlockMode {
		// Use block mode for better performance
		var buf bytes.Buffer
		writer := lz4.NewWriter(&buf)

		if l.cfg.CompressionLevel > 0 {
			writer.Apply(lz4.CompressionLevelOption(lz4.CompressionLevel(l.cfg.CompressionLevel)))
		}

		if _, err := writer.Write(data); err != nil {
			return nil, types.ErrCompressionError("lz4", err)
		}

		if err := writer.Close(); err != nil {
			return nil, types.ErrCompressionError("lz4", err)
		}

		return buf.Bytes(), nil
	}

	// Use raw compression for maximum speed
	dst := make([]byte, lz4.CompressBlockBound(len(data)))
	compressedSize, err := lz4.CompressBlock(data, dst, nil)
	if err != nil {
		return nil, types.ErrCompressionError("lz4", err)
	}

	return dst[:compressedSize], nil
}

func (l *LZ4Compressor) Decompress(data []byte) ([]byte, error) {
	if l.cfg.BlockMode {
		reader := lz4.NewReader(bytes.NewReader(data))

		var buf bytes.Buffer
		if _, err := io.Copy(&buf, reader); err != nil {
			return nil, types.ErrDecompressionError("lz4", err)
		}

		return buf.Bytes(), nil
	}

	// Raw decompression - we need to know the original size
	// For simplicity, we'll use a reasonable buffer size
	dst := make([]byte, len(data)*4) // Assume 4x expansion
	decompressedSize, err := lz4.UncompressBlock(data, dst)
	if err != nil {
		return nil, types.ErrDecompressionError("lz4", err)
	}

	return dst[:decompressedSize], nil
}

func (l *LZ4Compressor) CompressBatch(data [][]byte) ([]byte, error) {
	// Concatenate and compress
	var buf bytes.Buffer
	for _, chunk := range data {
		length := uint32(len(chunk))
		buf.WriteByte(byte(length >> 24))
		buf.WriteByte(byte(length >> 16))
		buf.WriteByte(byte(length >> 8))
		buf.WriteByte(byte(length))
		buf.Write(chunk)
	}

	return l.Compress(buf.Bytes())
}

func (l *LZ4Compressor) DecompressBatch(data []byte) ([][]byte, error) {
	decompressed, err := l.Decompress(data)
	if err != nil {
		return nil, err
	}

	return (&NoCompressor{}).DecompressBatch(decompressed)
}

func (l *LZ4Compressor) Name() string {
	return "lz4"
}

func (l *LZ4Compressor) EstimateRatio(data []byte) float64 {
	if len(data) < 32 {
		return 1.0
	}
	return 0.7 // LZ4 typically achieves ~70% of original size
}

func (l *LZ4Compressor) MinSize() int {
	return 32
}

// SnappyCompressor implements Snappy compression
type SnappyCompressor struct{}

func NewSnappyCompressor() *SnappyCompressor {
	return &SnappyCompressor{}
}

func (s *SnappyCompressor) Compress(data []byte) ([]byte, error) {
	return snappy.Encode(nil, data), nil
}

func (s *SnappyCompressor) Decompress(data []byte) ([]byte, error) {
	result, err := snappy.Decode(nil, data)
	if err != nil {
		return nil, types.ErrDecompressionError("snappy", err)
	}
	return result, nil
}

func (s *SnappyCompressor) CompressBatch(data [][]byte) ([]byte, error) {
	var buf bytes.Buffer
	for _, chunk := range data {
		length := uint32(len(chunk))
		buf.WriteByte(byte(length >> 24))
		buf.WriteByte(byte(length >> 16))
		buf.WriteByte(byte(length >> 8))
		buf.WriteByte(byte(length))
		buf.Write(chunk)
	}

	return s.Compress(buf.Bytes())
}

func (s *SnappyCompressor) DecompressBatch(data []byte) ([][]byte, error) {
	decompressed, err := s.Decompress(data)
	if err != nil {
		return nil, err
	}

	return (&NoCompressor{}).DecompressBatch(decompressed)
}

func (s *SnappyCompressor) Name() string {
	return "snappy"
}

func (s *SnappyCompressor) EstimateRatio(data []byte) float64 {
	if len(data) < 32 {
		return 1.0
	}
	return 0.75 // Snappy typically achieves ~75% of original size
}

func (s *SnappyCompressor) MinSize() int {
	return 32
}

// GzipCompressor implements Gzip compression
type GzipCompressor struct {
	level int
}

func NewGzipCompressor(level int) *GzipCompressor {
	return &GzipCompressor{level: level}
}

func (g *GzipCompressor) Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buf, g.level)
	if err != nil {
		return nil, types.ErrCompressionError("gzip", err)
	}

	if _, err := writer.Write(data); err != nil {
		return nil, types.ErrCompressionError("gzip", err)
	}

	if err := writer.Close(); err != nil {
		return nil, types.ErrCompressionError("gzip", err)
	}

	return buf.Bytes(), nil
}

func (g *GzipCompressor) Decompress(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, types.ErrDecompressionError("gzip", err)
	}
	defer reader.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		return nil, types.ErrDecompressionError("gzip", err)
	}

	return buf.Bytes(), nil
}

func (g *GzipCompressor) CompressBatch(data [][]byte) ([]byte, error) {
	var buf bytes.Buffer
	for _, chunk := range data {
		length := uint32(len(chunk))
		buf.WriteByte(byte(length >> 24))
		buf.WriteByte(byte(length >> 16))
		buf.WriteByte(byte(length >> 8))
		buf.WriteByte(byte(length))
		buf.Write(chunk)
	}

	return g.Compress(buf.Bytes())
}

func (g *GzipCompressor) DecompressBatch(data []byte) ([][]byte, error) {
	decompressed, err := g.Decompress(data)
	if err != nil {
		return nil, err
	}

	return (&NoCompressor{}).DecompressBatch(decompressed)
}

func (g *GzipCompressor) Name() string {
	return "gzip"
}

func (g *GzipCompressor) EstimateRatio(data []byte) float64 {
	if len(data) < 64 {
		return 1.0
	}
	return 0.5 // Gzip typically achieves ~50% of original size
}

func (g *GzipCompressor) MinSize() int {
	return 64
}

// BrotliCompressor implements Brotli compression
type BrotliCompressor struct {
	level int
}

func NewBrotliCompressor(level int) *BrotliCompressor {
	return &BrotliCompressor{level: level}
}

func (b *BrotliCompressor) Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := brotli.NewWriterLevel(&buf, b.level)

	if _, err := writer.Write(data); err != nil {
		return nil, types.ErrCompressionError("brotli", err)
	}

	if err := writer.Close(); err != nil {
		return nil, types.ErrCompressionError("brotli", err)
	}

	return buf.Bytes(), nil
}

func (b *BrotliCompressor) Decompress(data []byte) ([]byte, error) {
	reader := brotli.NewReader(bytes.NewReader(data))

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		return nil, types.ErrDecompressionError("brotli", err)
	}

	return buf.Bytes(), nil
}

func (b *BrotliCompressor) CompressBatch(data [][]byte) ([]byte, error) {
	var buf bytes.Buffer
	for _, chunk := range data {
		length := uint32(len(chunk))
		buf.WriteByte(byte(length >> 24))
		buf.WriteByte(byte(length >> 16))
		buf.WriteByte(byte(length >> 8))
		buf.WriteByte(byte(length))
		buf.Write(chunk)
	}

	return b.Compress(buf.Bytes())
}

func (b *BrotliCompressor) DecompressBatch(data []byte) ([][]byte, error) {
	decompressed, err := b.Decompress(data)
	if err != nil {
		return nil, err
	}

	return (&NoCompressor{}).DecompressBatch(decompressed)
}

func (b *BrotliCompressor) Name() string {
	return "brotli"
}

func (b *BrotliCompressor) EstimateRatio(data []byte) float64 {
	if len(data) < 64 {
		return 1.0
	}
	return 0.45 // Brotli typically achieves ~45% of original size
}

func (b *BrotliCompressor) MinSize() int {
	return 64
}
