# json/ (Standard and Alternative JSON Handlers)

This package provides standard and alternative JSON encoding/decoding implementations for Portask, supporting pluggable backends and streaming.

## Main Files
- `json.go`: Standard JSON logic and interfaces
- `sonic.go`: Sonic JSON implementation
- `standard.go`: Alternative JSON implementation

## Key Types, Interfaces, and Methods

### Interfaces
- `Encoder` — Interface for JSON encoding (Marshal, Unmarshal, NewEncoder, NewDecoder)
- `StreamEncoder` — Streaming JSON encoder
- `StreamDecoder` — Streaming JSON decoder

### Structs & Types
- `JSONLibrary` — Enum for JSON backend
- `Config` — JSON configuration
- `StandardEncoder`, `StandardStreamEncoder`, `StandardStreamDecoder` — Standard JSON
- `SonicEncoder`, `SonicStreamEncoder`, `SonicStreamDecoder` — Sonic JSON

### Main Methods (selected)
- `DefaultConfig() Config`, `SetEncoder()`, `GetEncoder()`, `Marshal()`, `Unmarshal()`, `NewEncoder()`, `NewDecoder()`, `InitializeFromConfig()`
- `NewStandardEncoder()`, `Marshal()`, `Unmarshal()`, `NewEncoder()`, `NewDecoder()` (StandardEncoder)
- `NewSonicEncoder()`, `Marshal()`, `Unmarshal()`, `NewEncoder()`, `NewDecoder()` (SonicEncoder)

## Usage Example
```go
import "github.com/meftunca/portask/pkg/json"

b, err := json.Marshal(map[string]string{"hello": "world"})
```

## TODO / Missing Features
- [ ] Performance benchmarks
- [ ] New JSON parser integrations
- [ ] Streaming optimizations

---

For details, see the code and comments in each file.
