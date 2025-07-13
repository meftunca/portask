# 🚀 jcron - High-Performance Cron Expression Engine

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![Performance](https://img.shields.io/badge/Performance-Enterprise%20Grade-00D26A?style=flat&logo=speedtest)](https://github.com/maple-tech/baseline/tree/main/jcron)
[![Test Coverage](https://img.shields.io/badge/Coverage-100%25-brightgreen?style=flat&logo=codecov)](https://github.com/maple-tech/baseline/tree/main/jcron)
[![License](https://img.shields.io/badge/License-MIT-blue?style=flat&logo=mit)](LICENSE)

Enterprise-grade cron expression parsing and scheduling engine optimized for high-throughput production environments. Supports all standard cron features including special characters (L, #), timezones, and Vixie-style OR logic.

## ⚡ Performance Highlights

- **Sub-300ns** operations for most cron expressions (152-275ns on Apple M2 Max)
- **Ultra-fast bit operations** - 0.3ns for set bit finding
- **Zero-allocation parsing** - 25ns for simple patterns, 0 allocations
- **Optimized special characters** - 51ns for complex L/# patterns
- **Memory-efficient caching** - Only 64B/op for most operations
- **Thread-safe** with optimized RWMutex caching

## 📋 Table of Contents

- [Features](#-features)
- [Installation](#-installation)
- [Quick Start](#-quick-start)
- [Cron Syntax](#-cron-syntax)
- [API Reference](#-api-reference)
- [Performance](#-performance)
- [Examples](#-examples)
- [Advanced Usage](#-advanced-usage)
- [Contributing](#-contributing)

## ✨ Features

### Core Functionality
- ✅ **Complete cron syntax support** - 6 or 7 field expressions
- ✅ **Special characters** - L (last), # (nth occurrence) 
- ✅ **Timezone support** - Any IANA timezone
- ✅ **Vixie-style OR logic** - Day-of-month OR day-of-week
- ✅ **Predefined schedules** - @yearly, @monthly, @weekly, @daily, @hourly
- ✅ **Next/Previous calculations** - Find next or previous execution times
- ✅ **Range expressions** - MON-FRI, 9-17, etc.
- ✅ **Step values** - */5, 10-50/2, etc.
- ✅ **List values** - 1,15,30 or MON,WED,FRI

### Performance Features
- 🚀 **High-speed parsing** - Optimized bit operations
- 💾 **Memory efficient** - Minimal allocations with object pooling
- 🔄 **Smart caching** - RWMutex-based schedule caching
- ⚡ **Fast-path optimizations** - Special handling for common patterns
- 🎯 **Zero-allocation** bit operations for time matching

## 📦 Installation

```bash
go get github.com/maple-tech/baseline/jcron
```

## 🚀 Quick Start

```go
package main

import (
    "fmt"
    "time"
    "github.com/maple-tech/baseline/jcron"
)

func main() {
    // Create a new engine
    engine := jcron.New()
    
    // Define a cron schedule (every day at 9:30 AM)
    schedule := jcron.Schedule{
        Second:     jcron.StrPtr("0"),     // 0 seconds
        Minute:     jcron.StrPtr("30"),    // 30 minutes
        Hour:       jcron.StrPtr("9"),     // 9 AM
        DayOfMonth: jcron.StrPtr("*"),     // every day
        Month:      jcron.StrPtr("*"),     // every month
        DayOfWeek:  jcron.StrPtr("*"),     // any day of week
        Year:       jcron.StrPtr("*"),     // every year
        Timezone:   jcron.StrPtr("UTC"),   // UTC timezone
    }
    
    // Get next execution time
    now := time.Now()
    next, err := engine.Next(schedule, now)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Next execution: %s\n", next.Format(time.RFC3339))
    
    // Get previous execution time
    prev, err := engine.Prev(schedule, now)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Previous execution: %s\n", prev.Format(time.RFC3339))
}
```

## 📝 Cron Syntax

jcron supports both 6-field and 7-field cron expressions:

### 6-Field Format (Classic)
```
 ┌─────────── minute (0-59)
 │ ┌───────── hour (0-23)
 │ │ ┌─────── day of month (1-31)
 │ │ │ ┌───── month (1-12 or JAN-DEC)
 │ │ │ │ ┌─── day of week (0-6 or SUN-SAT)
 │ │ │ │ │ ┌─ year (optional)
 │ │ │ │ │ │
 * * * * * *
```

### 7-Field Format (With Seconds)
```
 ┌──────────── second (0-59)
 │ ┌─────────── minute (0-59)
 │ │ ┌───────── hour (0-23)
 │ │ │ ┌─────── day of month (1-31)
 │ │ │ │ ┌───── month (1-12 or JAN-DEC)
 │ │ │ │ │ ┌─── day of week (0-6 or SUN-SAT)
 │ │ │ │ │ │ ┌─ year (optional)
 │ │ │ │ │ │ │
 * * * * * * *
```

### Special Characters

| Character | Description | Example | Meaning |
|-----------|-------------|---------|---------|
| `*` | Any value | `* * * * *` | Every minute |
| `?` | Any value (alias for *) | `0 0 ? * MON` | Every Monday at midnight |
| `-` | Range | `0 9-17 * * *` | Every hour from 9 AM to 5 PM |
| `,` | List | `0 0 1,15 * *` | 1st and 15th of every month |
| `/` | Step | `*/5 * * * *` | Every 5 minutes |
| `L` | Last | `0 0 L * *` | Last day of every month |
| `#` | Nth occurrence | `0 0 * * MON#2` | Second Monday of every month |

### Special "L" (Last) Patterns

```go
// Last day of month
"0 0 L * *"

// Last Friday of month  
"0 22 * * 5L"

// Last weekday (Monday-Friday) of month
"0 18 * * 1-5L"
```

### Special "#" (Nth Occurrence) Patterns

```go
// Second Tuesday of every month
"0 14 * * 2#2"

// First and third Monday of every month
"0 9 * * 1#1,1#3"

// Fourth Friday of every month
"0 17 * * 5#4"
```

### Predefined Schedules

| Predefined | Equivalent | Description |
|------------|------------|-------------|
| `@yearly` | `0 0 1 1 *` | Once a year at midnight on January 1st |
| `@annually` | `0 0 1 1 *` | Same as @yearly |
| `@monthly` | `0 0 1 * *` | Once a month at midnight on the 1st |
| `@weekly` | `0 0 * * 0` | Once a week at midnight on Sunday |
| `@daily` | `0 0 * * *` | Once a day at midnight |
| `@midnight` | `0 0 * * *` | Same as @daily |
| `@hourly` | `0 * * * *` | Once an hour at the beginning of the hour |

## 🔧 API Reference

### Types

```go
type Schedule struct {
    Second     *string  // 0-59 (optional, defaults to "*")
    Minute     *string  // 0-59
    Hour       *string  // 0-23  
    DayOfMonth *string  // 1-31
    Month      *string  // 1-12 or JAN-DEC
    DayOfWeek  *string  // 0-6 or SUN-SAT (0=Sunday)
    Year       *string  // Year (optional, defaults to "*")
    Timezone   *string  // IANA timezone (optional, defaults to "UTC")
}

type Engine struct {
    // Internal caching and optimization
}
```

### Core Functions

#### `New() *Engine`
Creates a new cron engine with initialized cache.

```go
engine := jcron.New()
```

#### `(*Engine) Next(schedule Schedule, fromTime time.Time) (time.Time, error)`
Calculates the next execution time after the given time.

```go
schedule := jcron.Schedule{
    Minute: jcron.StrPtr("30"),
    Hour:   jcron.StrPtr("14"), 
    // ... other fields
}

next, err := engine.Next(schedule, time.Now())
```

#### `(*Engine) Prev(schedule Schedule, fromTime time.Time) (time.Time, error)`
Calculates the previous execution time before the given time.

```go
prev, err := engine.Prev(schedule, time.Now())
```

### Helper Functions

#### `StrPtr(s string) *string`
Utility function to create string pointers for schedule fields.

```go
schedule := jcron.Schedule{
    Minute: jcron.StrPtr("0"),
    Hour:   jcron.StrPtr("12"),
}
```

## 📊 Performance

jcron is optimized for production environments with enterprise-grade performance:

### Benchmark Results (Apple M2 Max)

```
BenchmarkEngineNext_Simple-12                    4,116,897     275.5 ns/op      64 B/op    1 allocs/op
BenchmarkEngineNext_Complex-12                   7,550,816     160.8 ns/op      64 B/op    1 allocs/op
BenchmarkEngineNext_SpecialChars-12                509,917    2391   ns/op      64 B/op    1 allocs/op
BenchmarkEngineNext_Timezone-12                  4,600,984     272.6 ns/op      64 B/op    1 allocs/op
BenchmarkEngineNext_Frequent-12                  6,897,644     176.8 ns/op      64 B/op    1 allocs/op
BenchmarkEnginePrev_Simple-12                    7,912,470     152.1 ns/op      64 B/op    1 allocs/op
BenchmarkEnginePrev_Complex-12                   7,432,270     161.4 ns/op      64 B/op    1 allocs/op

BenchmarkExpandPart_Simple-12                   48,372,141      25.15 ns/op       0 B/op    0 allocs/op
BenchmarkExpandPart_Range-12                    10,396,274     115.2  ns/op      16 B/op    1 allocs/op
BenchmarkExpandPart_List-12                      5,832,867     203.4  ns/op      96 B/op    1 allocs/op
BenchmarkExpandPart_Step-12                     16,249,236      73.21 ns/op      16 B/op    1 allocs/op

BenchmarkFindNextSetBit-12                    1,000,000,000     0.3067 ns/op       0 B/op    0 allocs/op
BenchmarkFindPrevSetBit-12                    1,000,000,000     0.3057 ns/op       0 B/op    0 allocs/op

BenchmarkSpecialCharsOptimized-12               23,614,957      51.16 ns/op       0 B/op    0 allocs/op
BenchmarkCacheKeyOptimized-12                   20,601,344      56.46 ns/op      64 B/op    1 allocs/op

BenchmarkCacheHit-12                             4,458,757     274.4  ns/op      64 B/op    1 allocs/op
BenchmarkCacheMiss-12                               41,893   26,345   ns/op   41,753 B/op   21 allocs/op
```

### Performance Categories

- **🔥 Ultra-Fast (< 100 ns/op)**: Bit operations (0.3ns), simple parsing (25ns), optimized special chars (51ns)
- **🚀 Excellent (100-300 ns/op)**: Most common operations (152-275ns), timezone handling (272ns)
- **✅ Good (300-3000 ns/op)**: Complex special character patterns (2391ns)
- **📈 Acceptable (> 3000 ns/op)**: Cache misses (26,345ns - happens only once per schedule)

### Key Optimizations

- **Zero-allocation bit operations** - Core time calculations use CPU instructions
- **String builder pooling** - Reused objects for cache key generation
- **Fast-path detection** - Common patterns bypass complex parsing
- **Pre-compiled pattern maps** - Instant lookup for frequent special chars
- **Efficient caching** - RWMutex allows concurrent reads, single allocation per schedule

## 💡 Examples

### Business Hours Schedule

```go
// Monday to Friday, 9 AM to 5 PM, every 15 minutes
schedule := jcron.Schedule{
    Second:     jcron.StrPtr("0"),
    Minute:     jcron.StrPtr("*/15"),        // Every 15 minutes
    Hour:       jcron.StrPtr("9-17"),        // 9 AM to 5 PM  
    DayOfMonth: jcron.StrPtr("*"),           // Any day
    Month:      jcron.StrPtr("*"),           // Any month
    DayOfWeek:  jcron.StrPtr("MON-FRI"),     // Weekdays only
    Year:       jcron.StrPtr("*"),           // Any year
    Timezone:   jcron.StrPtr("America/New_York"),
}
```

### Monthly Reports

```go
// Last day of every month at 11:30 PM
schedule := jcron.Schedule{
    Second:     jcron.StrPtr("0"),
    Minute:     jcron.StrPtr("30"),
    Hour:       jcron.StrPtr("23"),
    DayOfMonth: jcron.StrPtr("L"),           // Last day of month
    Month:      jcron.StrPtr("*"),
    DayOfWeek:  jcron.StrPtr("*"),
    Timezone:   jcron.StrPtr("UTC"),
}
```

### Quarterly Meetings

```go
// First Monday of March, June, September, December at 2 PM
schedule := jcron.Schedule{
    Second:     jcron.StrPtr("0"),
    Minute:     jcron.StrPtr("0"),
    Hour:       jcron.StrPtr("14"),
    DayOfMonth: jcron.StrPtr("*"),
    Month:      jcron.StrPtr("3,6,9,12"),    // Quarterly months
    DayOfWeek:  jcron.StrPtr("1#1"),         // First Monday (#1)
    Timezone:   jcron.StrPtr("UTC"),
}
```

### Weekend Maintenance

```go
// Every Saturday at 3 AM for maintenance
schedule := jcron.Schedule{
    Second:     jcron.StrPtr("0"),
    Minute:     jcron.StrPtr("0"),
    Hour:       jcron.StrPtr("3"),
    DayOfMonth: jcron.StrPtr("*"),
    Month:      jcron.StrPtr("*"),
    DayOfWeek:  jcron.StrPtr("SAT"),         // Saturday only
    Timezone:   jcron.StrPtr("UTC"),
}
```

### High-Frequency Processing

```go
// Every 5 seconds during business hours
schedule := jcron.Schedule{
    Second:     jcron.StrPtr("*/5"),         // Every 5 seconds
    Minute:     jcron.StrPtr("*"),
    Hour:       jcron.StrPtr("9-17"),
    DayOfMonth: jcron.StrPtr("*"),
    Month:      jcron.StrPtr("*"),
    DayOfWeek:  jcron.StrPtr("1-5"),         // Weekdays
    Timezone:   jcron.StrPtr("UTC"),
}
```

## 🔬 Advanced Usage

### Custom Engine with Specific Cache Size

```go
// Create engine with pre-allocated cache for performance
engine := &jcron.Engine{
    // Note: Internal cache is automatically managed
}
```

### Multiple Timezone Handling

```go
// New York business hours
nySchedule := jcron.Schedule{
    Hour:     jcron.StrPtr("9-17"),
    DayOfWeek: jcron.StrPtr("1-5"),
    Timezone: jcron.StrPtr("America/New_York"),
}

// London business hours  
londonSchedule := jcron.Schedule{
    Hour:     jcron.StrPtr("9-17"),
    DayOfWeek: jcron.StrPtr("1-5"),
    Timezone: jcron.StrPtr("Europe/London"),
}

// Calculate next execution for both
nyNext, _ := engine.Next(nySchedule, time.Now())
londonNext, _ := engine.Next(londonSchedule, time.Now())
```

### Combining Multiple Patterns

```go
// Complex schedule: Every 30 minutes on weekdays, 
// plus specific times on weekends
weekdaySchedule := jcron.Schedule{
    Minute:    jcron.StrPtr("*/30"),
    Hour:      jcron.StrPtr("8-18"),
    DayOfWeek: jcron.StrPtr("1-5"),
    Timezone:  jcron.StrPtr("UTC"),
}

weekendSchedule := jcron.Schedule{
    Minute:    jcron.StrPtr("0"),
    Hour:      jcron.StrPtr("10,14,18"),
    DayOfWeek: jcron.StrPtr("0,6"),     // Sunday, Saturday
    Timezone:  jcron.StrPtr("UTC"),
}
```

### Error Handling Best Practices

```go
func scheduleJob(engine *jcron.Engine, schedule jcron.Schedule) error {
    // Validate schedule by trying to get next time
    _, err := engine.Next(schedule, time.Now())
    if err != nil {
        return fmt.Errorf("invalid schedule: %w", err)
    }
    
    // Schedule is valid, proceed with job scheduling
    return nil
}
```

## 🧪 Testing

Run all tests:
```bash
go test -v
```

Run performance benchmarks:
```bash
go test -bench=. -benchmem
```

Run specific benchmark:
```bash
go test -bench=BenchmarkEngineNext_SpecialChars -benchmem
```

### Test Coverage

The jcron engine includes comprehensive test coverage:

- ✅ **40+ test scenarios** covering all cron features
- ✅ **Edge cases**: Leap years, timezone transitions, month boundaries
- ✅ **Special characters**: All L and # pattern combinations  
- ✅ **Performance tests**: Benchmarks for all major operations
- ✅ **Error handling**: Invalid expressions and edge cases

## 🔧 Troubleshooting

### Common Issues

#### 1. Invalid Timezone
```go
// ❌ Invalid
schedule.Timezone = jcron.StrPtr("PST")

// ✅ Correct
schedule.Timezone = jcron.StrPtr("America/Los_Angeles")
```

#### 2. Invalid Day Combination
```go
// ❌ Invalid: 31st of February
schedule.DayOfMonth = jcron.StrPtr("31")
schedule.Month = jcron.StrPtr("2")

// ✅ Valid: Last day of February
schedule.DayOfMonth = jcron.StrPtr("L")
schedule.Month = jcron.StrPtr("2")
```

#### 3. Mixing Day-of-Month and Day-of-Week
```go
// ⚠️ Uses OR logic (Vixie-style): 15th OR Monday
schedule.DayOfMonth = jcron.StrPtr("15")
schedule.DayOfWeek = jcron.StrPtr("MON")

// ✅ Specific: 15th of month only if it's Monday
schedule.DayOfMonth = jcron.StrPtr("15") 
schedule.DayOfWeek = jcron.StrPtr("*")
// Then check if result.Weekday() == time.Monday
```

### Performance Tips

1. **Reuse Engine instances** - Engines maintain internal caches
2. **Use simple expressions when possible** - Avoid special characters if not needed
3. **Pre-validate schedules** - Check for errors during setup, not runtime
4. **Consider timezone impact** - UTC is fastest for timezone-agnostic schedules

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md) for details.

### Development Setup

```bash
# Clone the repository
git clone https://github.com/maple-tech/baseline.git
cd baseline/jcron

# Run tests
go test -v

# Run benchmarks
go test -bench=. -benchmem

# Run with race detection
go test -race -v
```

### Code Style

- Follow standard Go formatting (`go fmt`)
- Include comprehensive tests for new features
- Benchmark performance-critical changes
- Update documentation for API changes

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Inspired by traditional Unix cron and modern scheduling libraries
- Performance optimizations based on production usage patterns
- Special thanks to the Go community for excellent tooling and libraries

---

**🚀 Ready to schedule like a pro? Get started with jcron today!**

```bash
go get github.com/maple-tech/baseline/jcron
```
