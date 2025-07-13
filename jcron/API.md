# 📖 jcron API Documentation

Complete API reference for the jcron high-performance cron expression engine.

## Table of Contents

- [Package Overview](#package-overview)
- [Types](#types)
- [Core Functions](#core-functions)
- [Helper Functions](#helper-functions)
- [Error Types](#error-types)
- [Constants](#constants)
- [Examples](#examples)

## Package Overview

```go
package jcron

import "github.com/maple-tech/baseline/jcron"
```

The `jcron` package provides a high-performance cron expression parsing and scheduling engine optimized for enterprise-grade applications. It supports all standard cron features including special characters (L, #), timezones, and Vixie-style OR logic.

### Key Features
- **Sub-300ns performance** for most operations (152-275ns on Apple M2 Max)
- **Ultra-fast bit operations** - 0.3ns for set bit finding
- **Zero-allocation parsing** - 25ns for simple patterns, 0 allocations
- **Memory-optimized** with object pooling - only 64B/op for most operations
- **Thread-safe** with RWMutex caching
- **Complete cron syntax support** including special characters (L, #)
- **Timezone-aware scheduling** with any IANA timezone

## Types

### Schedule

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
```

Represents a cron schedule with all timing fields. All fields are optional pointers to strings, with sensible defaults applied when nil.

**Field Details:**

| Field | Range | Aliases | Special Chars | Default |
|-------|-------|---------|---------------|---------|
| `Second` | 0-59 | - | `*`, `/`, `,`, `-` | `"*"` |
| `Minute` | 0-59 | - | `*`, `/`, `,`, `-` | `"*"` |
| `Hour` | 0-23 | - | `*`, `/`, `,`, `-` | `"*"` |
| `DayOfMonth` | 1-31 | - | `*`, `/`, `,`, `-`, `L` | `"*"` |
| `Month` | 1-12 | JAN-DEC | `*`, `/`, `,`, `-` | `"*"` |
| `DayOfWeek` | 0-6 | SUN-SAT | `*`, `/`, `,`, `-`, `L`, `#` | `"*"` |
| `Year` | 1970-3000 | - | `*`, `/`, `,`, `-` | `"*"` |
| `Timezone` | IANA zones | - | - | `"UTC"` |

**Example:**
```go
schedule := Schedule{
    Second:     StrPtr("0"),           // Top of the minute
    Minute:     StrPtr("30"),          // 30 minutes past
    Hour:       StrPtr("9-17"),        // Business hours
    DayOfMonth: StrPtr("*"),           // Any day
    Month:      StrPtr("*"),           // Any month
    DayOfWeek:  StrPtr("MON-FRI"),     // Weekdays only
    Year:       StrPtr("*"),           // Any year
    Timezone:   StrPtr("America/New_York"),
}
```

### Engine

```go
type Engine struct {
    // Contains filtered or unexported fields
}
```

The core engine that parses cron expressions and calculates execution times. Engines maintain internal caches for performance and are safe for concurrent use.

**Internal Features:**
- RWMutex-based schedule caching
- Optimized bit operations for time matching
- String builder pooling for cache keys
- Fast-path optimizations for common patterns

### ExpandedSchedule

```go
type ExpandedSchedule struct {
    SecondsMask, MinutesMask   uint64  // Bitmasks for seconds/minutes (0-59)
    HoursMask, DaysOfMonthMask uint32  // Bitmasks for hours (0-23), days (1-31)
    MonthsMask                 uint16  // Bitmask for months (1-12)
    DaysOfWeekMask             uint8   // Bitmask for weekdays (0-6)
    Years                      []int   // List of valid years
    DayOfMonth, DayOfWeek      string  // Original expressions for special chars
    Location                   *time.Location // Parsed timezone
    HasSpecialDayPatterns      bool    // Cache flag for L/# optimization
}
```

Internal representation of a parsed cron schedule optimized for fast time matching. This type is used internally by the engine and is not typically accessed directly by users.

## Core Functions

### New

```go
func New() *Engine
```

Creates a new cron engine with initialized internal cache.

**Returns:**
- `*Engine`: A new engine instance ready for use

**Example:**
```go
engine := jcron.New()
```

**Performance Notes:**
- Engine creation is lightweight (~100ns)
- Engines should be reused for best performance
- Thread-safe for concurrent operations

### (*Engine) Next

```go
func (e *Engine) Next(schedule Schedule, fromTime time.Time) (time.Time, error)
```

Calculates the next execution time for the given schedule after the specified time.

**Parameters:**
- `schedule Schedule`: The cron schedule to evaluate
- `fromTime time.Time`: Calculate next time after this moment

**Returns:**
- `time.Time`: The next execution time
- `error`: Error if schedule is invalid or no future time exists

**Example:**
```go
schedule := Schedule{
    Minute: StrPtr("0"),      // Top of hour
    Hour:   StrPtr("*/2"),    // Every 2 hours
}

next, err := engine.Next(schedule, time.Now())
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Next execution: %s\n", next.Format(time.RFC3339))
```

**Performance:**
- Simple schedules: ~160-280 ns/op
- Complex schedules: ~2300 ns/op (with special chars)
- Cache hits: ~280 ns/op
- Cache misses: ~30,000 ns/op (first time only)

**Special Behaviors:**
- Returns time in the schedule's timezone
- Handles DST transitions correctly
- Supports leap years and month boundaries
- Uses Vixie-style OR logic for day-of-month + day-of-week

### (*Engine) Prev

```go
func (e *Engine) Prev(schedule Schedule, fromTime time.Time) (time.Time, error)
```

Calculates the previous execution time for the given schedule before the specified time.

**Parameters:**
- `schedule Schedule`: The cron schedule to evaluate
- `fromTime time.Time`: Calculate previous time before this moment

**Returns:**
- `time.Time`: The previous execution time
- `error`: Error if schedule is invalid or no past time exists

**Example:**
```go
schedule := Schedule{
    Minute:    StrPtr("0"),
    Hour:      StrPtr("12"),     // Noon
    DayOfWeek: StrPtr("MON"),    // Mondays
}

prev, err := engine.Prev(schedule, time.Now())
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Previous execution: %s\n", prev.Format(time.RFC3339))
```

**Performance:**
- Similar to `Next()` performance characteristics
- Optimized reverse bit scanning for efficiency

## Helper Functions

### StrPtr

```go
func StrPtr(s string) *string
```

Utility function to create string pointers for schedule fields. This is a convenience function since Go doesn't have built-in string pointer literals.

**Parameters:**
- `s string`: The string value to convert to a pointer

**Returns:**
- `*string`: Pointer to the string value

**Example:**
```go
schedule := Schedule{
    Minute: StrPtr("30"),
    Hour:   StrPtr("14"),
    // Instead of:
    // Minute: &[]string{"30"}[0],
    // Hour:   &[]string{"14"}[0],
}
```

## Error Types

The jcron package returns standard Go errors with descriptive messages. Common error scenarios:

### Invalid Expression Errors

```go
// Invalid range
schedule.Hour = StrPtr("25")  // Hours only go 0-23
err := "invalid hour '25': out of range [0-23]"

// Invalid step
schedule.Minute = StrPtr("*/61")  // Steps > max value
err := "invalid step '61' for minute: exceeds maximum 59"

// Invalid special character
schedule.DayOfWeek = StrPtr("8L")  // Invalid weekday
err := "invalid weekday '8' in 'L' expression"
```

### Timezone Errors

```go
// Invalid timezone
schedule.Timezone = StrPtr("Invalid/Zone")
err := "invalid timezone 'Invalid/Zone': unknown time zone"
```

### Parsing Errors

```go
// Invalid syntax
schedule.Month = StrPtr("JAN-13")  // Mix of alias and invalid number
err := "invalid month range 'JAN-13': end value out of range"
```

## Constants

### Month Aliases

```go
var monthAbbreviations = map[string]string{
    "JAN": "1", "FEB": "2", "MAR": "3", "APR": "4",
    "MAY": "5", "JUN": "6", "JUL": "7", "AUG": "8", 
    "SEP": "9", "OCT": "10", "NOV": "11", "DEC": "12",
}
```

### Day Aliases

```go
var dayAbbreviations = map[string]string{
    "SUN": "0", "MON": "1", "TUE": "2", "WED": "3",
    "THU": "4", "FRI": "5", "SAT": "6",
}
```

### Predefined Schedules

```go
var predefinedSchedules = map[string]string{
    "@yearly":   "0 0 0 1 1 *",
    "@annually": "0 0 0 1 1 *", 
    "@monthly":  "0 0 0 1 * *",
    "@weekly":   "0 0 0 * * 0",
    "@daily":    "0 0 0 * * *",
    "@midnight": "0 0 0 * * *",
    "@hourly":   "0 0 * * * *",
}
```

## Examples

### Basic Usage

```go
package main

import (
    "fmt"
    "log"
    "time"
    "github.com/maple-tech/baseline/jcron"
)

func main() {
    engine := jcron.New()
    
    // Every weekday at 9:30 AM
    schedule := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("30"),
        Hour:       jcron.StrPtr("9"),
        DayOfMonth: jcron.StrPtr("*"),
        Month:      jcron.StrPtr("*"),
        DayOfWeek:  jcron.StrPtr("1-5"),  // Mon-Fri
        Timezone:   jcron.StrPtr("UTC"),
    }
    
    next, err := engine.Next(schedule, time.Now())
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Next weekday 9:30 AM: %s\n", next.Format("2006-01-02 15:04:05 MST"))
}
```

### Special Characters

```go
// Last Friday of every month at 5 PM
schedule := jcron.Schedule{
    Minute:     jcron.StrPtr("0"),
    Hour:       jcron.StrPtr("17"),
    DayOfMonth: jcron.StrPtr("*"),
    Month:      jcron.StrPtr("*"),
    DayOfWeek:  jcron.StrPtr("5L"),     // Last Friday
    Timezone:   jcron.StrPtr("UTC"),
}

// Second Tuesday of every month at 2 PM
schedule := jcron.Schedule{
    Minute:     jcron.StrPtr("0"),
    Hour:       jcron.StrPtr("14"),
    DayOfMonth: jcron.StrPtr("*"),
    Month:      jcron.StrPtr("*"),
    DayOfWeek:  jcron.StrPtr("2#2"),    // Second Tuesday
    Timezone:   jcron.StrPtr("UTC"),
}
```

### Complex Expressions

```go
// Business hours: Every 15 minutes, 9 AM to 5 PM, weekdays only
schedule := jcron.Schedule{
    Second:     jcron.StrPtr("0"),
    Minute:     jcron.StrPtr("*/15"),           // Every 15 minutes
    Hour:       jcron.StrPtr("9-17"),           // 9 AM to 5 PM
    DayOfMonth: jcron.StrPtr("*"),
    Month:      jcron.StrPtr("*"),
    DayOfWeek:  jcron.StrPtr("MON-FRI"),        // Weekdays
    Timezone:   jcron.StrPtr("America/New_York"),
}

// Quarterly reports: 1st Monday of Mar, Jun, Sep, Dec at 9 AM
schedule := jcron.Schedule{
    Second:     jcron.StrPtr("0"),
    Minute:     jcron.StrPtr("0"),
    Hour:       jcron.StrPtr("9"),
    DayOfMonth: jcron.StrPtr("*"),
    Month:      jcron.StrPtr("3,6,9,12"),       // Quarterly
    DayOfWeek:  jcron.StrPtr("1#1"),            // First Monday
    Timezone:   jcron.StrPtr("UTC"),
}
```

### Error Handling

```go
func validateAndSchedule(engine *jcron.Engine, schedule jcron.Schedule) error {
    // Validate by attempting to calculate next time
    _, err := engine.Next(schedule, time.Now())
    if err != nil {
        return fmt.Errorf("invalid schedule: %w", err)
    }
    
    // Schedule is valid, proceed with job setup
    go runScheduledJob(engine, schedule)
    return nil
}

func runScheduledJob(engine *jcron.Engine, schedule jcron.Schedule) {
    for {
        now := time.Now()
        next, err := engine.Next(schedule, now)
        if err != nil {
            log.Printf("Schedule error: %v", err)
            return
        }
        
        // Wait until next execution time
        time.Sleep(next.Sub(now))
        
        // Execute job
        executeJob()
    }
}
```

### Timezone Handling

```go
// Multiple timezone support
timezones := []string{
    "America/New_York",
    "Europe/London", 
    "Asia/Tokyo",
    "Australia/Sydney",
}

schedule := jcron.Schedule{
    Minute: jcron.StrPtr("0"),
    Hour:   jcron.StrPtr("9"),      // 9 AM local time
}

for _, tz := range timezones {
    schedule.Timezone = jcron.StrPtr(tz)
    
    next, err := engine.Next(schedule, time.Now())
    if err != nil {
        log.Printf("Error for %s: %v", tz, err)
        continue
    }
    
    fmt.Printf("%s: %s\n", tz, next.Format("2006-01-02 15:04:05 MST"))
}
```

## Runner API

The Runner provides a complete job scheduling solution with automatic execution, retry logic, and structured logging.

### NewRunner

```go
func NewRunner(logger *slog.Logger) *Runner
```

Creates a new Runner instance with structured logging support.

**Parameters:**
- `logger *slog.Logger`: Structured logger for job execution events

**Returns:**
- `*Runner`: New Runner instance

**Example:**
```go
logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
runner := jcron.NewRunner(logger)
```

### (*Runner) AddJob

```go
func (r *Runner) AddJob(schedule Schedule, job Job, opts ...JobOption) (string, error)
```

Adds a job to the runner with the specified schedule and options.

**Parameters:**
- `schedule Schedule`: The cron schedule for job execution
- `job Job`: The job to execute (must implement Job interface)
- `opts ...JobOption`: Optional configuration (e.g., retry logic)

**Returns:**
- `string`: Unique job ID for management
- `error`: Error if schedule is invalid

**Example:**
```go
schedule := jcron.Schedule{
    Minute:   jcron.StrPtr("*/5"),
    Hour:     jcron.StrPtr("*"),
    Timezone: jcron.StrPtr("UTC"),
}

jobID, err := runner.AddJob(schedule, jcron.JobFunc(func() error {
    fmt.Println("Job executed!")
    return nil
}), jcron.WithRetries(3, 2*time.Second))
```

### (*Runner) AddFuncCron

```go
func (r *Runner) AddFuncCron(cronString string, cmd func() error, opts ...JobOption) (string, error)
```

Adds a job using traditional cron syntax string.

**Parameters:**
- `cronString string`: Cron expression (5 or 6 fields) or predefined (@hourly, @daily, etc.)
- `cmd func() error`: Function to execute
- `opts ...JobOption`: Optional configuration

**Returns:**
- `string`: Unique job ID
- `error`: Error if cron expression is invalid

**Supported Cron Formats:**
- **5-field**: `minute hour day month weekday`
- **6-field**: `second minute hour day month weekday`
- **Predefined**: `@yearly`, `@monthly`, `@weekly`, `@daily`, `@hourly`
- **Special**: `@reboot` (runs once at startup)

**Example:**
```go
// Traditional 5-field cron
jobID1, err := runner.AddFuncCron("0 9 * * 1-5", func() error {
    fmt.Println("Weekday 9 AM job")
    return nil
})

// 6-field with seconds
jobID2, err := runner.AddFuncCron("30 0 12 * * *", func() error {
    fmt.Println("Daily at 12:00:30")
    return nil
})

// Predefined schedule
jobID3, err := runner.AddFuncCron("@hourly", func() error {
    fmt.Println("Hourly job")
    return nil
})

// With retry options
jobID4, err := runner.AddFuncCron("*/5 * * * *", func() error {
    // Job that might fail
    return someOperation()
}, jcron.WithRetries(3, 10*time.Second))
```

### (*Runner) RemoveJob

```go
func (r *Runner) RemoveJob(id string)
```

Removes a job from the runner by its ID.

**Parameters:**
- `id string`: Job ID returned by AddJob or AddFuncCron

**Example:**
```go
jobID, _ := runner.AddFuncCron("@hourly", someJob)
// Later...
runner.RemoveJob(jobID)
```

### (*Runner) Start

```go
func (r *Runner) Start()
```

Starts the runner's job execution loop. Jobs will begin executing according to their schedules.

**Example:**
```go
runner.Start()
// Jobs are now running in background
```

### (*Runner) Stop

```go
func (r *Runner) Stop()
```

Stops the runner and cancels all pending job executions.

**Example:**
```go
defer runner.Stop() // Ensure cleanup

// Or explicit stop
runner.Stop()
```

### Job Interface

```go
type Job interface {
    Run() error
}
```

Interface that all jobs must implement.

**Example:**
```go
type DatabaseCleanup struct {
    ConnectionString string
}

func (d *DatabaseCleanup) Run() error {
    // Cleanup logic
    fmt.Printf("Cleaning database: %s\n", d.ConnectionString)
    return nil
}

// Usage
job := &DatabaseCleanup{ConnectionString: "postgres://..."}
runner.AddJob(schedule, job)
```

### JobFunc

```go
type JobFunc func() error
```

Adapter to use functions as Jobs.

**Example:**
```go
jobFunc := jcron.JobFunc(func() error {
    fmt.Println("Function-based job")
    return nil
})

runner.AddJob(schedule, jobFunc)
```

### JobOption

```go
type JobOption func(*managedJob)
```

Function type for configuring job behavior.

### WithRetries

```go
func WithRetries(maxRetries int, delay time.Duration) JobOption
```

Configures retry behavior for failed jobs.

**Parameters:**
- `maxRetries int`: Maximum number of retry attempts (0 = no retries)
- `delay time.Duration`: Delay between retry attempts

**Example:**
```go
runner.AddFuncCron("@hourly", unreliableJob, 
    jcron.WithRetries(5, 30*time.Second))
```

### FromCronSyntax

```go
func FromCronSyntax(cronString string) (Schedule, error)
```

Parses a cron syntax string into a Schedule struct.

**Parameters:**
- `cronString string`: Cron expression or predefined schedule

**Returns:**
- `Schedule`: Parsed schedule
- `error`: Error if syntax is invalid

**Supported Formats:**
```go
// Standard 5-field cron
schedule, _ := jcron.FromCronSyntax("0 9 * * 1-5")

// 6-field with seconds  
schedule, _ := jcron.FromCronSyntax("30 0 12 * * *")

// Predefined schedules
schedule, _ := jcron.FromCronSyntax("@daily")
schedule, _ := jcron.FromCronSyntax("@hourly")
schedule, _ := jcron.FromCronSyntax("@reboot")

// Complex expressions
schedule, _ := jcron.FromCronSyntax("0 0 12 L * *")     // Last day of month
schedule, _ := jcron.FromCronSyntax("0 0 14 * * 2#2")   // Second Tuesday
schedule, _ := jcron.FromCronSyntax("0 0 17 * * 5L")    // Last Friday
```

### Complete Runner Example

```go
package main

import (
    "fmt"
    "log/slog"
    "os"
    "time"
    "github.com/maple-tech/baseline/jcron"
)

func main() {
    // Setup logging
    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))
    
    // Create and start runner
    runner := jcron.NewRunner(logger)
    runner.Start()
    defer runner.Stop()
    
    // Add various jobs
    
    // Simple function job
    runner.AddFuncCron("*/30 * * * * *", func() error {
        fmt.Println("30-second job executed")
        return nil
    })
    
    // Job with retries
    runner.AddFuncCron("@hourly", func() error {
        // Simulate unreliable operation
        if time.Now().Hour()%3 == 0 {
            return fmt.Errorf("simulated failure")
        }
        fmt.Println("Hourly job completed")
        return nil
    }, jcron.WithRetries(3, 5*time.Minute))
    
    // Custom job implementation
    type ReportGenerator struct {
        ReportType string
    }
    
    func (r *ReportGenerator) Run() error {
        fmt.Printf("Generating %s report...\n", r.ReportType)
        time.Sleep(100 * time.Millisecond)
        return nil
    }
    
    schedule := jcron.Schedule{
        Minute:   jcron.StrPtr("0"),
        Hour:     jcron.StrPtr("6"),    // 6 AM
        Timezone: jcron.StrPtr("UTC"),
    }
    
    reportJob := &ReportGenerator{ReportType: "Daily Sales"}
    runner.AddJob(schedule, reportJob, jcron.WithRetries(2, 10*time.Minute))
    
    // Let jobs run
    time.Sleep(2 * time.Minute)
}
```

---

This API documentation covers all public interfaces and provides comprehensive examples for effective usage of the jcron library.
