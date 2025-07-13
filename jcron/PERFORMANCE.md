# 🚀 jcron Performance Documentation

Detailed performance analysis and optimization guide for the jcron high-performance cron expression engine.

## 📊 Benchmark Results

All benchmarks performed on **Apple M2 Max** with Go 1.21+:

### Core Engine Performance

| Operation | Ops/sec | ns/op | B/op | allocs/op | Notes |
|-----------|---------|-------|------|-----------|-------|
| **Next() - Simple** | 4,116,897 | 275.5 | 64 | 1 | Basic hourly/daily schedules |
| **Next() - Complex** | 7,550,816 | 160.8 | 64 | 1 | Business hours, ranges, lists |
| **Next() - Frequent** | 6,897,644 | 176.8 | 64 | 1 | High-frequency (every few seconds) |
| **Next() - Timezone** | 4,600,984 | 272.6 | 64 | 1 | Timezone-aware scheduling |
| **Prev() - Simple** | 7,912,470 | 152.1 | 64 | 1 | Previous time calculation |
| **Prev() - Complex** | 7,432,270 | 161.4 | 64 | 1 | Complex previous calculations |

### Special Character Performance

| Operation | Ops/sec | ns/op | B/op | allocs/op | Notes |
|-----------|---------|-------|------|-----------|-------|
| **Special Chars (L/#)** | 509,917 | 2,391 | 64 | 1 | Complex L and # patterns |
| **Optimized Special** | 23,614,957 | 51.16 | 0 | 0 | Fast-path special patterns |
| **Hash Pattern (#)** | 848,288 | 1,453 | 64 | 1 | Nth occurrence patterns |
| **Last Pattern (L)** | 391,533 | 3,093 | 64 | 1 | Last day/weekday patterns |

### Parsing Performance

| Operation | Ops/sec | ns/op | B/op | allocs/op | Notes |
|-----------|---------|-------|------|-----------|-------|
| **Simple Wildcard** | 48,372,141 | 25.15 | 0 | 0 | Asterisk (*) patterns |
| **Range Parsing** | 10,396,274 | 115.2 | 16 | 1 | Range expressions (1-5) |
| **List Parsing** | 5,832,867 | 203.4 | 96 | 1 | Comma-separated lists |
| **Step Parsing** | 16,249,236 | 73.21 | 16 | 1 | Step expressions (*/5) |

### Bit Operations

| Operation | Ops/sec | ns/op | B/op | allocs/op | Notes |
|-----------|---------|-------|------|-----------|-------|
| **Find Next Set Bit** | 1,000,000,000 | 0.3067 | 0 | 0 | CPU instruction level |
| **Find Prev Set Bit** | 1,000,000,000 | 0.3057 | 0 | 0 | CPU instruction level |
| **Bit Pop Count** | 1,000,000,000 | 0.3088 | 0 | 0 | Hardware accelerated |
| **Trailing Zeros** | 1,000,000,000 | 0.3074 | 0 | 0 | Hardware accelerated |

### Caching Performance

| Operation | Ops/sec | ns/op | B/op | allocs/op | Notes |
|-----------|---------|-------|------|-----------|-------|
| **Cache Hit** | 4,458,757 | 274.4 | 64 | 1 | Cached schedule lookup |
| **Cache Miss** | 41,893 | 26,345 | 41,753 | 21 | First-time schedule parsing |
| **Cache Key Generation** | 20,601,344 | 56.46 | 64 | 1 | Optimized key building |

### String Operations

| Operation | Ops/sec | ns/op | B/op | allocs/op | Notes |
|-----------|---------|-------|------|-----------|-------|
| **String Split** | 33,594,271 | 35.88 | 48 | 1 | Comma separation |
| **String Contains** | 386,921,722 | 3.068 | 0 | 0 | Pattern detection |
| **String HasSuffix** | 1,000,000,000 | 0.3064 | 0 | 0 | L pattern detection |

## 🏆 Performance Categories

### 🔥 Ultra-Fast (< 100 ns/op)
- **Bit operations** (0.3ns) - CPU instruction level performance
- **Simple parsing** (25ns) - Zero-allocation wildcard handling
- **Optimized special chars** (51ns) - Fast-path L/# pattern recognition
- **Cache key generation** (56ns) - Pooled string builders

### 🚀 Excellent (100-300 ns/op)
- **Most common operations** (152-275ns) - Typical scheduling calculations
- **Complex parsing** (115-203ns) - Range and list expressions
- **Timezone handling** (272ns) - Location-aware time calculations
- **Cache hits** (274ns) - Optimized schedule reuse

### ✅ Good (300-3000 ns/op)
- **Complex special patterns** (1,453-3,093ns) - Advanced L/# combinations
- **First calculation** (2,391ns) - Initial special character processing

### 📈 Acceptable (> 3000 ns/op)
- **Cache misses** (26,345ns) - One-time parsing overhead per schedule

## 🎯 Key Optimizations

### 1. Zero-Allocation Bit Operations
```go
// Ultra-fast bit finding using CPU instructions
func findNextSetBit(mask uint64, from, max int) (int, bool) {
    if from > max {
        return bits.TrailingZeros64(mask), true
    }
    searchMask := mask >> from
    if searchMask != 0 {
        next := from + bits.TrailingZeros64(searchMask)
        if next <= max {
            return next, false
        }
    }
    return bits.TrailingZeros64(mask), true
}
```

### 2. String Builder Pooling
```go
var builderPool = sync.Pool{
    New: func() interface{} {
        return &strings.Builder{}
    },
}

func (e *Engine) buildCacheKey(schedule Schedule) string {
    builder := builderPool.Get().(*strings.Builder)
    defer builderPool.Put(builder)
    builder.Reset()
    // ... build key efficiently
    return builder.String()
}
```

### 3. Pre-compiled Pattern Maps
```go
var commonSpecialPatterns = map[string]bool{
    "L": true, "1L": true, "2L": true, "3L": true, "4L": true, "5L": true, "6L": true,
    "1#1": true, "1#2": true, "1#3": true, "1#4": true, "1#5": true,
    // ... more patterns
}

// Instant lookup for common patterns
if commonSpecialPatterns[pattern] {
    return e.checkCommonSpecialPattern(pattern, t, currentWeekday, currentDay)
}
```

### 4. Fast-Path Detection
```go
// Skip complex parsing for simple cases
if expr == "*" || expr == "?" {
    // Generate bitmask directly
    var mask uint64
    for i := min; i <= max; i++ {
        mask |= (1 << uint(i))
    }
    return mask, nil
}
```

### 5. Efficient Caching with RWMutex
```go
type Engine struct {
    cache map[string]*ExpandedSchedule
    mu    sync.RWMutex // Allows concurrent reads
}

func (e *Engine) getExpandedSchedule(schedule Schedule) (*ExpandedSchedule, error) {
    key := e.buildCacheKey(schedule)
    
    // Fast read path
    e.mu.RLock()
    if cached, exists := e.cache[key]; exists {
        e.mu.RUnlock()
        return cached, nil
    }
    e.mu.RUnlock()
    
    // Slow write path (only for cache misses)
    e.mu.Lock()
    defer e.mu.Unlock()
    // ... parse and cache
}
```

## 📈 Performance Best Practices

### 1. Reuse Engine Instances
```go
// ✅ Good: Single engine with caching
engine := jcron.New()
for _, schedule := range schedules {
    next, _ := engine.Next(schedule, time.Now())
}

// ❌ Bad: Multiple engines, no caching benefit
for _, schedule := range schedules {
    engine := jcron.New()
    next, _ := engine.Next(schedule, time.Now())
}
```

### 2. Use Simple Expressions When Possible
```go
// ✅ Fast: Simple patterns (25ns)
minute := "*"
hour := "*/2"

// ⚠️ Slower: Complex special chars (2,391ns)
dayOfWeek := "1#2,3#4,5L"
```

### 3. Prefer UTC for Timezone-Agnostic Schedules
```go
// ✅ Fastest: UTC timezone
schedule.Timezone = jcron.StrPtr("UTC")

// ⚠️ Slightly slower: Named timezones
schedule.Timezone = jcron.StrPtr("America/New_York")
```

### 4. Batch Process Multiple Times
```go
// ✅ Efficient: Calculate multiple future times
currentTime := time.Now()
for i := 0; i < 10; i++ {
    next, _ := engine.Next(schedule, currentTime)
    futureExecTimes = append(futureExecTimes, next)
    currentTime = next
}
```

### 5. Pre-validate Schedules at Startup
```go
// ✅ Good: Validate during initialization
func validateSchedules(schedules []Schedule) error {
    engine := jcron.New()
    for _, schedule := range schedules {
        if _, err := engine.Next(schedule, time.Now()); err != nil {
            return fmt.Errorf("invalid schedule: %w", err)
        }
    }
    return nil
}
```

## 🔍 Profiling and Monitoring

### Memory Usage Patterns
- **Typical operation**: 64 bytes, 1 allocation
- **Cache miss**: 41,753 bytes, 21 allocations (one-time per schedule)
- **Zero allocations**: Bit operations, simple parsing, optimized special chars

### CPU Usage Characteristics
- **Dominated by**: Bit manipulation operations
- **Minimal string processing**: Only during parsing phase
- **Cache-friendly**: Hot paths use CPU registers and L1 cache

### Scaling Characteristics
- **Linear scaling** with number of schedules (with caching)
- **Constant time** for cached schedule lookups
- **Sub-linear scaling** for batch operations on same schedule

## 🚀 Future Optimizations

### Potential Improvements
1. **SIMD instructions** for parallel bit operations
2. **Compile-time schedule optimization** for static schedules
3. **Memory-mapped caching** for persistent schedule storage
4. **GPU acceleration** for massive batch processing

### Monitoring Recommendations
1. **Track cache hit ratios** - Should be > 95% in production
2. **Monitor allocation rates** - Should be minimal during steady state
3. **Profile hot paths** - Focus optimization on frequently used schedules
4. **Benchmark regressions** - Run benchmarks in CI/CD pipeline

---

This performance documentation demonstrates jcron's enterprise-grade optimization and provides guidance for achieving optimal performance in production environments.
