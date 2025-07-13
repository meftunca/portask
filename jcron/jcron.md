# JCRON - A High-Performance Go Job Scheduler

JCRON is a modern, high-performance job scheduling library for Go. It is designed to be a flexible and efficient alternative to standard cron libraries, incorporating advanced scheduling features inspired by the Quartz Scheduler while maintaining a simple and developer-friendly API.

Its core philosophy is built on **performance**, **readability**, and **robustness**.

## Features

- **Standard & Advanced Cron Syntax:** Fully compatible with the 5-field (`* * * * *`) and 6-field (`* * * * * *`) Vixie-cron formats.
- **Enhanced Scheduling Rules:** Supports advanced, Quartz-like specifiers for complex schedules:
    - `L`: For the "last" day of the month or week (e.g., `L` for the last day of the month, `5L` for the last Friday).
    - `#`: For the "Nth" day of the week in a month (e.g., `1#3` for the third Monday).
- **High-Performance "Next Jump" Algorithm:** Instead of tick-by-tick time checking, JCRON mathematically calculates the next valid run time, providing significant performance gains for long-interval jobs.
- **Aggressive Caching:** Cron expressions are parsed only once and their expanded integer-based representations are cached, making subsequent schedule calculations extremely fast.
- **Built-in Error Handling & Retries:** Jobs can return errors, and you can configure automatic retry policies with delays for each job.
- **Panic Recovery:** A job that panics will not crash the runner. The panic is recovered, logged, and the runner continues to operate smoothly.
- **Structured Logging:** Uses the standard `log/slog` library. Inject your own configured logger to integrate JCRON's logs seamlessly into your application's logging infrastructure.
- **Thread-Safe:** Designed from the ground up to be safe for concurrent use. You can add, remove, and manage jobs from multiple goroutines without data races.

## Installation

Bash

```javascript
go get github.com/meftunca/jcron

```


## Quick Start

Here is a simple example to get you up and running in minutes.

Go

```javascript
package main

import (
	"fmt"
	"github.com/meftunca/jcron"
	"log/slog"
	"os"
	"time"
)

func main() {
	// 1. Create a structured logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// 2. Initialize the runner with the logger
	runner := jcron.NewRunner(logger)

	// 3. Add jobs using the familiar cron syntax
	// This job runs every 5 seconds.
	runner.AddFuncCron("*/5 * * * * *", func() error {
		fmt.Println("-> Simple job running every 5 seconds!")
		return nil
	})

	// This job runs once at the 30th second of every minute.
	runner.AddFuncCron("30 * * * * *", func() error {
		fmt.Printf("--> It's the 30th second of the minute. Time is %s\n", time.Now().Format("15:04:05"))
		return nil
	})

	// 4. Start the runner (non-blocking)
	runner.Start()
	logger.Info("JCRON runner has been started. Press CTRL+C to exit.")

	// 5. Wait for the application to be terminated
	// In a real application, this would be your main application loop.
	time.Sleep(1 * time.Minute)

	// 6. Stop the runner gracefully
	runner.Stop()
}

```

## Core Concepts


### The `Runner`

The `Runner` is the heart of the scheduler. It manages the entire lifecycle of all jobs. You create a single `Runner` instance for your application.

Go

```javascript
// The runner requires a *slog.Logger for structured logging.
logger := slog.New(...)
runner := jcron.NewRunner(logger)

// Start and Stop the runner's background goroutine.
runner.Start()
defer runner.Stop()

```

### The `Job` Interface

Any task you want to schedule must implement the `Job` interface. The `Run` method must return an `error`. If the job is successful, return `nil`.

Go

```javascript
type Job interface {
    Run() error
}

```

For convenience, you can use the `JobFunc` type to adapt any function with the signature `func() error` into a `Job`.

### Scheduling a Job

The easiest way to schedule a job is with `AddFuncCron`, which accepts the standard cron string format.

Go

```javascript
id, err := runner.AddFuncCron(
    "0 17 * * 5", // Run at 5:00 PM every Friday
    func() error {
        fmt.Println("It's Friday at 5 PM! Time to go home.")
        return nil
    },
)

```

### Advanced Options: Retry Policies

You can pass additional options when scheduling a job. The most common is setting a retry policy. The `WithRetries` option uses the "Functional Options Pattern".

Go

```javascript
import "time"

// This job will be retried up to 3 times, with a 5-second delay between each attempt.
id, err := runner.AddFuncCron(
    "*/15 * * * * *", // Trigger every 15 seconds
    myFailingJob,
    jcron.WithRetries(3, 5*time.Second),
)

func myFailingJob() error {
    // ... logic that might fail ...
    return errors.New("something went wrong")
}

```

## JCRON Format Specification (v1.1)

For advanced scheduling that is not covered by the standard cron syntax, you can build a `jcron.Schedule` struct manually.

| Key | Description | Allowed Values | Example |
| `s` | Second | `0-59` and `* , - /` | `"30"`, `"0/15"` |
| `m` | Minute | `0-59` and `* , - /` | `"5"`, `"0-29"` |
| `h` | Hour | `0-23` and `* , - /` | `"9"`, `"9-17"` |
| `D` | Day of Month | `1-31` and `* , - / ? L` | `"15"`, `"L"` |
| `M` | Month | `1-12` and `* , - /` | `"10"`, `"6-8"` |
| `dow` | Day of Week | `0-7` (0 or 7 is Sun) and `* , - / ? L #` | `"1"`, `"1-5"`, `"5L"` |
| `Y` | Year | `YYYY` and `* , -` | `"2025"`, `"2025-2030"` |
| `tz` | Timezone | IANA Format | `"Europe/Istanbul"`, `"America/New_York"` |

E-Tablolar'a aktar

**Defaults:** If a key is not specified, it defaults to `*` (every), except for `s` (second), which defaults to `0`.

## Logging

JCRON uses the standard library's structured logger, `log/slog`. You are required to pass a `*slog.Logger` instance to the `NewRunner`. This gives you full control over the log level, format (Text or JSON), and output destination, ensuring JCRON's logs integrate perfectly with your application's.

Go

```javascript
// Example: Create a JSON logger that logs to stderr.
logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
runner := jcron.NewRunner(logger)

```

## Thread Safety

The JCRON `Runner` and `Engine` are designed to be fully thread-safe. All access to shared internal state (the job list and the schedule cache) is protected by mutexes. You can safely call `AddJob`, `RemoveJob`, etc., from multiple goroutines concurrently.

## License

JCRON is released under the MIT License. See the `LICENSE` file for details.