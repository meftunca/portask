# compression/ (Multiple Compression Algorithms)

This package provides multiple compression algorithms for Portask, including Zstd, Gzip, LZ4, Snappy, and Brotli. It supports batch and single compression, extensible via a factory pattern.

## Main Files
- `compressor.go`: Compression algorithms and factory
- `compressor_test.go`: Tests for compression logic

## Key Types, Interfaces, and Methods

### Interfaces
- `Compressor` — Interface for compression algorithms (Compress, Decompress, CompressBatch, DecompressBatch, Name, EstimateRatio, MinSize)

### Structs & Types
- `CompressorFactory` — Factory for registering and retrieving compressors
- `NoCompressor`, `ZstdCompressor`, `LZ4Compressor`, `SnappyCompressor`, `GzipCompressor`, `BrotliCompressor` — Compression implementations

### Main Methods (selected)
- `NewCompressorFactory() *CompressorFactory`, `RegisterCompressor()`, `GetCompressor()`, `InitializeDefaultCompressors()` (CompressorFactory)
- `Compress`, `Decompress`, `CompressBatch`, `DecompressBatch`, `Name`, `EstimateRatio`, `MinSize` (all Compressor implementations)

## Usage Example
```go
import "github.com/meftunca/portask/pkg/compression"

factory := compression.NewCompressorFactory()
comp, _ := factory.GetCompressor("zstd")
```

## TODO / Missing Features
- [ ] Add new algorithms
- [ ] Performance benchmarks
- [ ] Streaming compression support

---

For details, see the code and comments in each file.
