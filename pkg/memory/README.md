# memory/ — Memory Pooling, Buffer Management, and Fast Buffer Utilities

This package provides high-performance memory pooling, buffer management, and fast buffer utilities for Portask. It is designed to reduce GC pressure, optimize memory usage, and enable efficient message and buffer reuse across the system.

---

## Key Files
- `pool.go`: Main memory pool, buffer pool, fast buffer, and message pool implementations
- `pool_test.go`: Unit tests and benchmarks for all memory pool and buffer types

---

## Main Types & Interfaces

### Memory Pooling
- `type Pool` — General-purpose byte slice or custom object pool with configurable size, stats, and diagnostics
- `type PoolStats` — Statistics for a pool (gets, puts, hits, misses, size, etc)
- `type PoolConfig` — Configuration for pool (size, object size, factory, monitor interval, etc)
- `type PoolMonitor` — Monitors and auto-resizes pools based on usage
- `type PoolEvent` — Diagnostics event (grow, shrink, reset, custom)
- `type PoolManager` — Manages multiple pools (global, per-type, etc)

### Buffer Management
- `type BufferPool` — Pool for variable-sized byte slices, with usage stats
- `type FastBuffer` — High-performance, growable buffer for fast writes and reuse

### Message Pooling
- `type MessagePool` — Pool for Portask message objects
- `type Message` — Reusable message struct for high-throughput scenarios

### Utilities & Stats
- `type MemoryStats` — System memory statistics (heap, alloc, GC, etc)

---

## Core Methods (Selection)
- `Pool.Get/Put/Resize/Clear/Close/GetStats/GetEvents/ResetAndDrain` — Pool operations and diagnostics
- `BufferPool.Get/Put/GetStats` — Get and return variable-sized buffers, usage stats
- `FastBuffer.Write/WriteByte/WriteString/Bytes/Reset/Len/Cap` — Fast buffer operations
- `MessagePool.Get/Put/Stats` — Message object pooling
- `GetMemoryStats/ForceGC/SetGCPercent` — System memory utilities

---

## Usage Examples

### Adaptive Pool Resizing & Diagnostics
```go
pool := memory.NewPool(memory.PoolConfig{...})
// ... use the pool ...
for _, event := range pool.GetEvents() {
    fmt.Println(event.Time, event.Type, event.Message)
}
```

### Custom Object Pool
```go
type MyStruct struct{ X int }
pool := memory.NewPool(memory.PoolConfig{
    Name: "custom",
    Factory: func() interface{} { return &MyStruct{} },
})
obj := pool.Get().(*MyStruct)
pool.Put(obj)
```

### Pool Reset/Drain
```go
pool.ResetAndDrain()
```

### BufferPool Usage Stats
```go
bp := memory.NewBufferPool()
bp.Get(64)
stats := bp.GetStats()
fmt.Println(stats[64]) // Usage count for 64-byte buffers
```

---

## Best Practices
- Use diagnostics to monitor pool health and resizing.
- For custom object pools, implement your own reset logic if needed.
- Use `ResetAndDrain` before/after benchmarks or tests for clean state.
- Monitor BufferPool stats to tune buffer sizes for your workload.

## Anti-Patterns
- Avoid using pools for objects with complex lifecycles or external dependencies.
- Do not rely on pool events for critical application logic (use for diagnostics/monitoring only).

---

## API Reference
- `NewPool(PoolConfig) *Pool`
- `Pool.Get() interface{}` / `Pool.Put(interface{})`
- `Pool.GetBytes() []byte` (for []byte pools)
- `Pool.GetEvents() []PoolEvent`
- `Pool.ResetAndDrain()`
- `BufferPool.Get(size int) []byte`
- `BufferPool.Put([]byte)`
- `BufferPool.GetStats() map[int]int64`

---

## See Also
- See `pool_test.go` for comprehensive usage and feature tests.

---

_Last updated: 2025-07-21_
