# 🎯 jcron Usage Examples

Comprehensive examples demonstrating real-world usage of the jcron high-performance cron expression engine.

**Performance Highlights:**
- **152-275ns** for most common operations on Apple M2 Max
- **0.3ns** ultra-fast bit operations  
- **25ns** zero-allocation simple parsing
- **51ns** optimized special character handling
- **Only 64B/op** memory usage for typical schedules

## Table of Contents

- [Basic Scheduling](#basic-scheduling)
- [Business Applications](#business-applications)
- [Advanced Patterns](#advanced-patterns)
- [Timezone Handling](#timezone-handling)
- [Error Handling](#error-handling)
- [Performance Optimization](#performance-optimization)
- [Integration Examples](#integration-examples)

## Basic Scheduling

### Simple Time-based Jobs

```go
package main

import (
    "fmt"
    "log"
    "time"
    "github.com/maple-tech/baseline/jcron"
)

func basicExamples() {
    engine := jcron.New()
    
    // Every minute
    everyMinute := jcron.Schedule{
        Second:   jcron.StrPtr("0"),
        Minute:   jcron.StrPtr("*"),
        Hour:     jcron.StrPtr("*"),
        Timezone: jcron.StrPtr("UTC"),
    }
    
    // Every hour at 30 minutes past
    hourly := jcron.Schedule{
        Second:   jcron.StrPtr("0"),
        Minute:   jcron.StrPtr("30"),
        Hour:     jcron.StrPtr("*"),
        Timezone: jcron.StrPtr("UTC"),
    }
    
    // Daily at midnight
    daily := jcron.Schedule{
        Second:   jcron.StrPtr("0"),
        Minute:   jcron.StrPtr("0"),
        Hour:     jcron.StrPtr("0"),
        Timezone: jcron.StrPtr("UTC"),
    }
    
    // Calculate next execution times
    now := time.Now()
    
    next1, _ := engine.Next(everyMinute, now)
    next2, _ := engine.Next(hourly, now)
    next3, _ := engine.Next(daily, now)
    
    fmt.Printf("Next minute mark: %s\n", next1.Format("15:04:05"))
    fmt.Printf("Next 30-minute mark: %s\n", next2.Format("15:04:05"))
    fmt.Printf("Next midnight: %s\n", next3.Format("2006-01-02 15:04:05"))
}
```

### Using Predefined Schedules

```go
func predefinedExamples() {
    engine := jcron.New()
    
    // Using predefined schedule concepts
    yearly := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("0"),
        Hour:       jcron.StrPtr("0"),
        DayOfMonth: jcron.StrPtr("1"),
        Month:      jcron.StrPtr("1"),
        Timezone:   jcron.StrPtr("UTC"),
    }
    
    monthly := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("0"),
        Hour:       jcron.StrPtr("0"),
        DayOfMonth: jcron.StrPtr("1"),
        Timezone:   jcron.StrPtr("UTC"),
    }
    
    weekly := jcron.Schedule{
        Second:    jcron.StrPtr("0"),
        Minute:    jcron.StrPtr("0"),
        Hour:      jcron.StrPtr("0"),
        DayOfWeek: jcron.StrPtr("0"), // Sunday
        Timezone:  jcron.StrPtr("UTC"),
    }
    
    now := time.Now()
    nextYear, _ := engine.Next(yearly, now)
    nextMonth, _ := engine.Next(monthly, now)
    nextWeek, _ := engine.Next(weekly, now)
    
    fmt.Printf("Next New Year: %s\n", nextYear.Format("2006-01-02"))
    fmt.Printf("Next month start: %s\n", nextMonth.Format("2006-01-02"))
    fmt.Printf("Next Sunday: %s\n", nextWeek.Format("2006-01-02"))
}
```

## Business Applications

### Office Hours Automation

```go
func officeHoursExamples() {
    engine := jcron.New()
    
    // Business hours: 9 AM to 5 PM, Monday to Friday
    businessHours := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("0"),
        Hour:       jcron.StrPtr("9-17"),      // 9 AM to 5 PM
        DayOfMonth: jcron.StrPtr("*"),
        Month:      jcron.StrPtr("*"),
        DayOfWeek:  jcron.StrPtr("1-5"),       // Monday to Friday
        Timezone:   jcron.StrPtr("America/New_York"),
    }
    
    // Every 15 minutes during business hours
    frequentChecks := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("*/15"),      // Every 15 minutes
        Hour:       jcron.StrPtr("9-17"),
        DayOfMonth: jcron.StrPtr("*"),
        Month:      jcron.StrPtr("*"),
        DayOfWeek:  jcron.StrPtr("1-5"),
        Timezone:   jcron.StrPtr("America/New_York"),
    }
    
    // Lunch break reminder: 11:45 AM on weekdays
    lunchReminder := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("45"),
        Hour:       jcron.StrPtr("11"),
        DayOfMonth: jcron.StrPtr("*"),
        Month:      jcron.StrPtr("*"),
        DayOfWeek:  jcron.StrPtr("1-5"),
        Timezone:   jcron.StrPtr("America/New_York"),
    }
    
    now := time.Now()
    
    nextBusiness, _ := engine.Next(businessHours, now)
    nextCheck, _ := engine.Next(frequentChecks, now)
    nextLunch, _ := engine.Next(lunchReminder, now)
    
    fmt.Printf("Next business hour: %s\n", nextBusiness.Format("Mon 2006-01-02 15:04 MST"))
    fmt.Printf("Next 15-min check: %s\n", nextCheck.Format("Mon 15:04 MST"))
    fmt.Printf("Next lunch reminder: %s\n", nextLunch.Format("Mon 2006-01-02 15:04 MST"))
}
```

### Backup and Maintenance

```go
func maintenanceExamples() {
    engine := jcron.New()
    
    // Daily backup at 2 AM
    dailyBackup := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("0"),
        Hour:       jcron.StrPtr("2"),
        DayOfMonth: jcron.StrPtr("*"),
        Month:      jcron.StrPtr("*"),
        DayOfWeek:  jcron.StrPtr("*"),
        Timezone:   jcron.StrPtr("UTC"),
    }
    
    // Weekly full backup: Sunday at 1 AM
    weeklyBackup := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("0"),
        Hour:       jcron.StrPtr("1"),
        DayOfMonth: jcron.StrPtr("*"),
        Month:      jcron.StrPtr("*"),
        DayOfWeek:  jcron.StrPtr("0"),  // Sunday
        Timezone:   jcron.StrPtr("UTC"),
    }
    
    // Monthly maintenance: First Saturday of each month at 3 AM
    monthlyMaintenance := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("0"),
        Hour:       jcron.StrPtr("3"),
        DayOfMonth: jcron.StrPtr("*"),
        Month:      jcron.StrPtr("*"),
        DayOfWeek:  jcron.StrPtr("6#1"),  // First Saturday
        Timezone:   jcron.StrPtr("UTC"),
    }
    
    // System cleanup: Every 6 hours
    systemCleanup := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("0"),
        Hour:       jcron.StrPtr("*/6"),  // Every 6 hours
        DayOfMonth: jcron.StrPtr("*"),
        Month:      jcron.StrPtr("*"),
        DayOfWeek:  jcron.StrPtr("*"),
        Timezone:   jcron.StrPtr("UTC"),
    }
    
    now := time.Now()
    
    nextDaily, _ := engine.Next(dailyBackup, now)
    nextWeekly, _ := engine.Next(weeklyBackup, now)
    nextMaintenance, _ := engine.Next(monthlyMaintenance, now)
    nextCleanup, _ := engine.Next(systemCleanup, now)
    
    fmt.Printf("Next daily backup: %s\n", nextDaily.Format("2006-01-02 15:04 MST"))
    fmt.Printf("Next weekly backup: %s\n", nextWeekly.Format("2006-01-02 15:04 MST"))
    fmt.Printf("Next maintenance: %s\n", nextMaintenance.Format("2006-01-02 15:04 MST"))
    fmt.Printf("Next cleanup: %s\n", nextCleanup.Format("2006-01-02 15:04 MST"))
}
```

### Report Generation

```go
func reportingExamples() {
    engine := jcron.New()
    
    // Daily sales report: 6 PM every weekday
    dailySales := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("0"),
        Hour:       jcron.StrPtr("18"),
        DayOfMonth: jcron.StrPtr("*"),
        Month:      jcron.StrPtr("*"),
        DayOfWeek:  jcron.StrPtr("1-5"),
        Timezone:   jcron.StrPtr("America/New_York"),
    }
    
    // Weekly summary: Friday at 5 PM
    weeklySummary := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("0"),
        Hour:       jcron.StrPtr("17"),
        DayOfMonth: jcron.StrPtr("*"),
        Month:      jcron.StrPtr("*"),
        DayOfWeek:  jcron.StrPtr("5"),  // Friday
        Timezone:   jcron.StrPtr("America/New_York"),
    }
    
    // Monthly report: Last day of month at 11 PM
    monthlyReport := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("0"),
        Hour:       jcron.StrPtr("23"),
        DayOfMonth: jcron.StrPtr("L"),  // Last day of month
        Month:      jcron.StrPtr("*"),
        DayOfWeek:  jcron.StrPtr("*"),
        Timezone:   jcron.StrPtr("UTC"),
    }
    
    // Quarterly report: Last Friday of Mar, Jun, Sep, Dec
    quarterlyReport := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("0"),
        Hour:       jcron.StrPtr("16"),
        DayOfMonth: jcron.StrPtr("*"),
        Month:      jcron.StrPtr("3,6,9,12"),  // Quarterly months
        DayOfWeek:  jcron.StrPtr("5L"),        // Last Friday
        Timezone:   jcron.StrPtr("UTC"),
    }
    
    now := time.Now()
    
    nextDaily, _ := engine.Next(dailySales, now)
    nextWeekly, _ := engine.Next(weeklySummary, now)
    nextMonthly, _ := engine.Next(monthlyReport, now)
    nextQuarterly, _ := engine.Next(quarterlyReport, now)
    
    fmt.Printf("Next daily sales report: %s\n", nextDaily.Format("2006-01-02 15:04 MST"))
    fmt.Printf("Next weekly summary: %s\n", nextWeekly.Format("2006-01-02 15:04 MST"))
    fmt.Printf("Next monthly report: %s\n", nextMonthly.Format("2006-01-02 15:04 MST"))
    fmt.Printf("Next quarterly report: %s\n", nextQuarterly.Format("2006-01-02 15:04 MST"))
}
```

## Advanced Patterns

### Special Character Usage

```go
func specialCharacterExamples() {
    engine := jcron.New()
    
    // Last day of every month at midnight
    monthEnd := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("0"),
        Hour:       jcron.StrPtr("0"),
        DayOfMonth: jcron.StrPtr("L"),  // Last day
        Month:      jcron.StrPtr("*"),
        DayOfWeek:  jcron.StrPtr("*"),
        Timezone:   jcron.StrPtr("UTC"),
    }
    
    // Second Tuesday of every month at 2 PM
    secondTuesday := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("0"),
        Hour:       jcron.StrPtr("14"),
        DayOfMonth: jcron.StrPtr("*"),
        Month:      jcron.StrPtr("*"),
        DayOfWeek:  jcron.StrPtr("2#2"),  // Second Tuesday
        Timezone:   jcron.StrPtr("UTC"),
    }
    
    // Last Friday of every month at 5 PM
    lastFriday := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("0"),
        Hour:       jcron.StrPtr("17"),
        DayOfMonth: jcron.StrPtr("*"),
        Month:      jcron.StrPtr("*"),
        DayOfWeek:  jcron.StrPtr("5L"),  // Last Friday
        Timezone:   jcron.StrPtr("UTC"),
    }
    
    // Multiple nth patterns: 1st and 3rd Monday
    multipleMondays := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("0"),
        Hour:       jcron.StrPtr("9"),
        DayOfMonth: jcron.StrPtr("*"),
        Month:      jcron.StrPtr("*"),
        DayOfWeek:  jcron.StrPtr("1#1,1#3"),  // 1st and 3rd Monday
        Timezone:   jcron.StrPtr("UTC"),
    }
    
    now := time.Now()
    
    nextMonthEnd, _ := engine.Next(monthEnd, now)
    nextSecondTue, _ := engine.Next(secondTuesday, now)
    nextLastFri, _ := engine.Next(lastFriday, now)
    nextMultiMon, _ := engine.Next(multipleMondays, now)
    
    fmt.Printf("Next month end: %s\n", nextMonthEnd.Format("2006-01-02"))
    fmt.Printf("Next second Tuesday: %s\n", nextSecondTue.Format("2006-01-02 15:04"))
    fmt.Printf("Next last Friday: %s\n", nextLastFri.Format("2006-01-02 15:04"))
    fmt.Printf("Next 1st/3rd Monday: %s\n", nextMultiMon.Format("2006-01-02 15:04"))
}
```

### Complex Step Patterns

```go
func stepPatternExamples() {
    engine := jcron.New()
    
    // Every 5 seconds
    everyFiveSeconds := jcron.Schedule{
        Second:   jcron.StrPtr("*/5"),
        Minute:   jcron.StrPtr("*"),
        Hour:     jcron.StrPtr("*"),
        Timezone: jcron.StrPtr("UTC"),
    }
    
    // Every 2 hours during business days
    everyTwoHours := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("0"),
        Hour:       jcron.StrPtr("8-18/2"),  // 8, 10, 12, 14, 16, 18
        DayOfMonth: jcron.StrPtr("*"),
        Month:      jcron.StrPtr("*"),
        DayOfWeek:  jcron.StrPtr("1-5"),
        Timezone:   jcron.StrPtr("UTC"),
    }
    
    // Every 10 minutes during specific hours
    tenMinuteIntervals := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("*/10"),
        Hour:       jcron.StrPtr("9-17"),
        DayOfMonth: jcron.StrPtr("*"),
        Month:      jcron.StrPtr("*"),
        DayOfWeek:  jcron.StrPtr("1-5"),
        Timezone:   jcron.StrPtr("UTC"),
    }
    
    // Every other day at noon
    everyOtherDay := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("0"),
        Hour:       jcron.StrPtr("12"),
        DayOfMonth: jcron.StrPtr("*/2"),  // Every 2 days
        Month:      jcron.StrPtr("*"),
        DayOfWeek:  jcron.StrPtr("*"),
        Timezone:   jcron.StrPtr("UTC"),
    }
    
    now := time.Now()
    
    nextFiveSec, _ := engine.Next(everyFiveSeconds, now)
    nextTwoHour, _ := engine.Next(everyTwoHours, now)
    nextTenMin, _ := engine.Next(tenMinuteIntervals, now)
    nextOtherDay, _ := engine.Next(everyOtherDay, now)
    
    fmt.Printf("Next 5-second mark: %s\n", nextFiveSec.Format("15:04:05"))
    fmt.Printf("Next 2-hour interval: %s\n", nextTwoHour.Format("2006-01-02 15:04"))
    fmt.Printf("Next 10-minute mark: %s\n", nextTenMin.Format("15:04"))
    fmt.Printf("Next other day: %s\n", nextOtherDay.Format("2006-01-02 15:04"))
}
```

### List Combinations

```go
func listCombinationExamples() {
    engine := jcron.New()
    
    // Specific times: 8:30, 12:30, 17:30
    specificTimes := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("30"),
        Hour:       jcron.StrPtr("8,12,17"),
        DayOfMonth: jcron.StrPtr("*"),
        Month:      jcron.StrPtr("*"),
        DayOfWeek:  jcron.StrPtr("1-5"),
        Timezone:   jcron.StrPtr("UTC"),
    }
    
    // Quarter hours: :00, :15, :30, :45
    quarterHours := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("0,15,30,45"),
        Hour:       jcron.StrPtr("9-17"),
        DayOfMonth: jcron.StrPtr("*"),
        Month:      jcron.StrPtr("*"),
        DayOfWeek:  jcron.StrPtr("1-5"),
        Timezone:   jcron.StrPtr("UTC"),
    }
    
    // Specific days: 1st, 15th, last day of month
    specificDays := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("0"),
        Hour:       jcron.StrPtr("9"),
        DayOfMonth: jcron.StrPtr("1,15,L"),
        Month:      jcron.StrPtr("*"),
        DayOfWeek:  jcron.StrPtr("*"),
        Timezone:   jcron.StrPtr("UTC"),
    }
    
    // Seasonal months: Mar, Jun, Sep, Dec
    seasonalMonths := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("0"),
        Hour:       jcron.StrPtr("10"),
        DayOfMonth: jcron.StrPtr("1"),
        Month:      jcron.StrPtr("3,6,9,12"),  // Quarterly
        DayOfWeek:  jcron.StrPtr("*"),
        Timezone:   jcron.StrPtr("UTC"),
    }
    
    now := time.Now()
    
    nextSpecific, _ := engine.Next(specificTimes, now)
    nextQuarter, _ := engine.Next(quarterHours, now)
    nextDays, _ := engine.Next(specificDays, now)
    nextSeasonal, _ := engine.Next(seasonalMonths, now)
    
    fmt.Printf("Next specific time: %s\n", nextSpecific.Format("2006-01-02 15:04"))
    fmt.Printf("Next quarter hour: %s\n", nextQuarter.Format("15:04"))
    fmt.Printf("Next specific day: %s\n", nextDays.Format("2006-01-02"))
    fmt.Printf("Next seasonal month: %s\n", nextSeasonal.Format("2006-01-02"))
}
```

## Timezone Handling

### Multi-Region Scheduling

```go
func timezoneExamples() {
    engine := jcron.New()
    
    // Same time across different timezones
    globalSchedule := func(timezone string) jcron.Schedule {
        return jcron.Schedule{
            Second:     jcron.StrPtr("0"),
            Minute:     jcron.StrPtr("0"),
            Hour:       jcron.StrPtr("9"),  // 9 AM local time
            DayOfMonth: jcron.StrPtr("*"),
            Month:      jcron.StrPtr("*"),
            DayOfWeek:  jcron.StrPtr("1-5"),
            Timezone:   jcron.StrPtr(timezone),
        }
    }
    
    timezones := map[string]string{
        "New York":  "America/New_York",
        "London":    "Europe/London",
        "Tokyo":     "Asia/Tokyo",
        "Sydney":    "Australia/Sydney",
        "Los Angeles": "America/Los_Angeles",
    }
    
    now := time.Now()
    fmt.Printf("Current time: %s\n", now.Format("2006-01-02 15:04:05 MST"))
    fmt.Printf("\nNext 9 AM business day in each timezone:\n")
    
    for city, tz := range timezones {
        schedule := globalSchedule(tz)
        next, err := engine.Next(schedule, now)
        if err != nil {
            fmt.Printf("%s: Error - %v\n", city, err)
            continue
        }
        fmt.Printf("%-12s: %s\n", city, next.Format("2006-01-02 15:04 MST"))
    }
}
```

### DST-Aware Scheduling

```go
func dstAwareExamples() {
    engine := jcron.New()
    
    // Schedule that handles DST transitions
    dstSchedule := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("30"),
        Hour:       jcron.StrPtr("2"),    // 2:30 AM (can be tricky during DST)
        DayOfMonth: jcron.StrPtr("*"),
        Month:      jcron.StrPtr("*"),
        DayOfWeek:  jcron.StrPtr("*"),
        Timezone:   jcron.StrPtr("America/New_York"),
    }
    
    // Test around DST transition dates
    testDates := []string{
        "2024-03-09T00:00:00-05:00", // Day before spring DST
        "2024-03-10T00:00:00-05:00", // Spring DST day (2 AM doesn't exist)
        "2024-11-02T00:00:00-04:00", // Day before fall DST
        "2024-11-03T00:00:00-05:00", // Fall DST day (2 AM occurs twice)
    }
    
    for _, dateStr := range testDates {
        testTime, _ := time.Parse(time.RFC3339, dateStr)
        next, err := engine.Next(dstSchedule, testTime)
        if err != nil {
            fmt.Printf("Error for %s: %v\n", dateStr, err)
            continue
        }
        fmt.Printf("From %s -> %s\n", 
            testTime.Format("2006-01-02 15:04 MST"),
            next.Format("2006-01-02 15:04 MST"))
    }
}
```

## Error Handling

### Robust Schedule Validation

```go
func errorHandlingExamples() {
    engine := jcron.New()
    
    // Function to validate schedule
    validateSchedule := func(name string, schedule jcron.Schedule) {
        _, err := engine.Next(schedule, time.Now())
        if err != nil {
            fmt.Printf("❌ %s: %v\n", name, err)
        } else {
            fmt.Printf("✅ %s: Valid\n", name)
        }
    }
    
    // Valid schedules
    validSchedule := jcron.Schedule{
        Minute:     jcron.StrPtr("0"),
        Hour:       jcron.StrPtr("12"),
        DayOfMonth: jcron.StrPtr("*"),
        Month:      jcron.StrPtr("*"),
        DayOfWeek:  jcron.StrPtr("*"),
        Timezone:   jcron.StrPtr("UTC"),
    }
    
    // Invalid schedules for testing
    invalidHour := jcron.Schedule{
        Minute:   jcron.StrPtr("0"),
        Hour:     jcron.StrPtr("25"), // Invalid: hours are 0-23
        Timezone: jcron.StrPtr("UTC"),
    }
    
    invalidMinute := jcron.Schedule{
        Minute:   jcron.StrPtr("60"), // Invalid: minutes are 0-59
        Hour:     jcron.StrPtr("12"),
        Timezone: jcron.StrPtr("UTC"),
    }
    
    invalidTimezone := jcron.Schedule{
        Minute:   jcron.StrPtr("0"),
        Hour:     jcron.StrPtr("12"),
        Timezone: jcron.StrPtr("Invalid/Zone"),
    }
    
    invalidRange := jcron.Schedule{
        Minute:     jcron.StrPtr("0"),
        Hour:       jcron.StrPtr("12"),
        DayOfWeek:  jcron.StrPtr("8"), // Invalid: weekdays are 0-6
        Timezone:   jcron.StrPtr("UTC"),
    }
    
    fmt.Println("Schedule Validation Results:")
    validateSchedule("Valid Schedule", validSchedule)
    validateSchedule("Invalid Hour", invalidHour)
    validateSchedule("Invalid Minute", invalidMinute)
    validateSchedule("Invalid Timezone", invalidTimezone)
    validateSchedule("Invalid Range", invalidRange)
}
```

### Graceful Error Recovery

```go
func errorRecoveryExample() {
    engine := jcron.New()
    
    // Schedule with potential issues
    schedules := []struct {
        name string
        schedule jcron.Schedule
    }{
        {
            "Normal Schedule",
            jcron.Schedule{
                Minute: jcron.StrPtr("0"),
                Hour:   jcron.StrPtr("12"),
                Timezone: jcron.StrPtr("UTC"),
            },
        },
        {
            "Invalid Timezone",
            jcron.Schedule{
                Minute: jcron.StrPtr("0"),
                Hour:   jcron.StrPtr("12"),
                Timezone: jcron.StrPtr("Invalid/Zone"),
            },
        },
        {
            "February 30th (will skip)",
            jcron.Schedule{
                Minute:     jcron.StrPtr("0"),
                Hour:       jcron.StrPtr("12"),
                DayOfMonth: jcron.StrPtr("30"),
                Month:      jcron.StrPtr("2"), // February
                Timezone:   jcron.StrPtr("UTC"),
            },
        },
    }
    
    now := time.Now()
    
    for _, s := range schedules {
        next, err := engine.Next(s.schedule, now)
        if err != nil {
            fmt.Printf("⚠️  %s failed: %v\n", s.name, err)
            
            // Try fallback schedule
            fallback := jcron.Schedule{
                Minute:   jcron.StrPtr("0"),
                Hour:     jcron.StrPtr("12"),
                Timezone: jcron.StrPtr("UTC"),
            }
            
            fallbackNext, fallbackErr := engine.Next(fallback, now)
            if fallbackErr == nil {
                fmt.Printf("   Using fallback: %s\n", fallbackNext.Format("2006-01-02 15:04"))
            }
        } else {
            fmt.Printf("✅ %s: %s\n", s.name, next.Format("2006-01-02 15:04"))
        }
    }
}
```

## Performance Optimization

### High-Frequency Scheduling

```go
func performanceExamples() {
    engine := jcron.New()
    
    // High-frequency schedule: every second
    highFreq := jcron.Schedule{
        Second:   jcron.StrPtr("*"),
        Minute:   jcron.StrPtr("*"),
        Hour:     jcron.StrPtr("*"),
        Timezone: jcron.StrPtr("UTC"),
    }
    
    // Batch processing: calculate multiple next times
    now := time.Now()
    var nextTimes []time.Time
    
    currentTime := now
    for i := 0; i < 10; i++ {
        next, err := engine.Next(highFreq, currentTime)
        if err != nil {
            break
        }
        nextTimes = append(nextTimes, next)
        currentTime = next
    }
    
    fmt.Printf("Next 10 execution times:\n")
    for i, t := range nextTimes {
        fmt.Printf("%2d: %s\n", i+1, t.Format("15:04:05"))
    }
}
```

### Caching and Reuse

```go
func cachingExamples() {
    // Reuse engine instance for better performance
    engine := jcron.New()
    
    // Common schedule that will be cached
    commonSchedule := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("0"),
        Hour:       jcron.StrPtr("*"),
        DayOfMonth: jcron.StrPtr("*"),
        Month:      jcron.StrPtr("*"),
        DayOfWeek:  jcron.StrPtr("*"),
        Timezone:   jcron.StrPtr("UTC"),
    }
    
    now := time.Now()
    
    // First call will parse and cache
    start := time.Now()
    _, err1 := engine.Next(commonSchedule, now)
    firstCall := time.Since(start)
    
    // Subsequent calls will use cache
    start = time.Now()
    _, err2 := engine.Next(commonSchedule, now)
    secondCall := time.Since(start)
    
    if err1 == nil && err2 == nil {
        fmt.Printf("First call (with parsing): %v\n", firstCall)
        fmt.Printf("Second call (cached): %v\n", secondCall)
        fmt.Printf("Speedup: %.1fx\n", float64(firstCall)/float64(secondCall))
    }
}
```

## Integration Examples

### Built-in Runner Usage

jcron includes a built-in Runner that provides a complete scheduling solution with job management, retry logic, and structured logging.

```go
package main

import (
    "fmt"
    "log"
    "log/slog"
    "time"
    "github.com/maple-tech/baseline/jcron"
)

func runnerBasicExample() {
    // Create a structured logger
    logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
    
    // Create a new runner
    runner := jcron.NewRunner(logger)
    
    // Start the runner
    runner.Start()
    defer runner.Stop()
    
    // Add a simple job using cron syntax
    jobID1, err := runner.AddFuncCron("0 */5 * * * *", func() error {
        fmt.Printf("Job 1 executed at %s\n", time.Now().Format("15:04:05"))
        return nil
    })
    if err != nil {
        log.Fatal(err)
    }
    
    // Add a job with retry options
    jobID2, err := runner.AddFuncCron("0 */10 * * * *", func() error {
        fmt.Printf("Job 2 executed at %s\n", time.Now().Format("15:04:05"))
        // Simulate occasional failure
        if time.Now().Second()%3 == 0 {
            return fmt.Errorf("simulated failure")
        }
        return nil
    }, jcron.WithRetries(3, 2*time.Second))
    if err != nil {
        log.Fatal(err)
    }
    
    // Add a job using Schedule struct
    schedule := jcron.Schedule{
        Second:     jcron.StrPtr("0"),
        Minute:     jcron.StrPtr("*/2"),  // Every 2 minutes
        Hour:       jcron.StrPtr("*"),
        DayOfMonth: jcron.StrPtr("*"),
        Month:      jcron.StrPtr("*"),
        DayOfWeek:  jcron.StrPtr("*"),
        Timezone:   jcron.StrPtr("UTC"),
    }
    
    jobID3, err := runner.AddJob(schedule, jcron.JobFunc(func() error {
        fmt.Printf("Job 3 executed at %s\n", time.Now().Format("15:04:05"))
        return nil
    }))
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Added jobs: %s, %s, %s\n", jobID1, jobID2, jobID3)
    
    // Let it run for a while
    time.Sleep(30 * time.Second)
    
    // Remove a job
    runner.RemoveJob(jobID2)
    fmt.Printf("Removed job: %s\n", jobID2)
    
    // Continue running
    time.Sleep(30 * time.Second)
}
```

### Advanced Runner Features

```go
// Custom Job implementation
type DatabaseCleanupJob struct {
    ConnectionString string
    TableName       string
}

func (j *DatabaseCleanupJob) Run() error {
    fmt.Printf("Cleaning up database table: %s\n", j.TableName)
    // Simulate database cleanup
    time.Sleep(100 * time.Millisecond)
    
    // Simulate occasional database connection error
    if time.Now().Unix()%10 == 0 {
        return fmt.Errorf("database connection failed")
    }
    
    fmt.Printf("Database cleanup completed for: %s\n", j.TableName)
    return nil
}

func runnerAdvancedExample() {
    logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))
    
    runner := jcron.NewRunner(logger)
    runner.Start()
    defer runner.Stop()
    
    // Add custom job with retry logic
    dbJob := &DatabaseCleanupJob{
        ConnectionString: "postgres://user:pass@localhost/db",
        TableName:       "user_sessions",
    }
    
    // Schedule every day at 2 AM with retries
    jobID, err := runner.AddJob(
        jcron.Schedule{
            Second:     jcron.StrPtr("0"),
            Minute:     jcron.StrPtr("0"),
            Hour:       jcron.StrPtr("2"),   // 2 AM
            DayOfMonth: jcron.StrPtr("*"),
            Month:      jcron.StrPtr("*"),
            DayOfWeek:  jcron.StrPtr("*"),
            Timezone:   jcron.StrPtr("UTC"),
        },
        dbJob,
        jcron.WithRetries(5, 30*time.Second), // 5 retries with 30s delay
    )
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Database cleanup job scheduled: %s\n", jobID)
    
    // Add multiple jobs with different schedules
    jobs := []struct {
        name     string
        cronExpr string
        handler  func() error
        retries  int
        delay    time.Duration
    }{
        {
            name:     "Hourly Report",
            cronExpr: "@hourly",
            handler: func() error {
                fmt.Println("Generating hourly report...")
                return nil
            },
        },
        {
            name:     "Weekly Backup",
            cronExpr: "0 0 2 * * 0", // Sunday 2 AM
            handler: func() error {
                fmt.Println("Running weekly backup...")
                // Simulate backup that might fail
                if time.Now().Unix()%7 == 0 {
                    return fmt.Errorf("backup storage unavailable")
                }
                return nil
            },
            retries: 3,
            delay:   5 * time.Minute,
        },
        {
            name:     "Cache Clear",
            cronExpr: "0 */15 * * * *", // Every 15 minutes
            handler: func() error {
                fmt.Println("Clearing cache...")
                return nil
            },
        },
    }
    
    var jobIDs []string
    for _, jobConfig := range jobs {
        var jobID string
        var err error
        
        if jobConfig.retries > 0 {
            jobID, err = runner.AddFuncCron(
                jobConfig.cronExpr,
                jobConfig.handler,
                jcron.WithRetries(jobConfig.retries, jobConfig.delay),
            )
        } else {
            jobID, err = runner.AddFuncCron(jobConfig.cronExpr, jobConfig.handler)
        }
        
        if err != nil {
            log.Printf("Failed to add job %s: %v", jobConfig.name, err)
            continue
        }
        
        jobIDs = append(jobIDs, jobID)
        fmt.Printf("Added %s job: %s\n", jobConfig.name, jobID)
    }
    
    // Let the scheduler run
    time.Sleep(2 * time.Minute)
    
    // Cleanup - remove all jobs
    for _, id := range jobIDs {
        runner.RemoveJob(id)
    }
}
```

### Cron Syntax Parsing Examples

```go
func cronSyntaxExamples() {
    // Test various cron syntax formats
    cronExpressions := []string{
        "@yearly",           // Predefined
        "@monthly",          // Predefined
        "@weekly",           // Predefined
        "@daily",            // Predefined
        "@hourly",           // Predefined
        "@reboot",           // Special: run once at startup
        "0 30 * * * *",      // 6-field: every minute at 30 seconds
        "30 9 * * *",        // 5-field: every day at 9:30 AM
        "0 0 12 * * MON",    // Every Monday at noon
        "0 */15 9-17 * * 1-5", // Every 15 min during business hours, weekdays
        "0 0 0 1 1 *",       // New Year's Day at midnight
        "0 0 0 L * *",       // Last day of every month
        "0 0 14 * * 2#2",    // Second Tuesday of every month at 2 PM
        "0 0 17 * * 5L",     // Last Friday of every month at 5 PM
    }
    
    for _, expr := range cronExpressions {
        schedule, err := jcron.FromCronSyntax(expr)
        if err != nil {
            fmt.Printf("❌ Invalid cron expression '%s': %v\n", expr, err)
            continue
        }
        
        fmt.Printf("✅ Parsed '%s':\n", expr)
        if schedule.Second != nil {
            fmt.Printf("   Second: %s\n", *schedule.Second)
        }
        if schedule.Minute != nil {
            fmt.Printf("   Minute: %s\n", *schedule.Minute)
        }
        if schedule.Hour != nil {
            fmt.Printf("   Hour: %s\n", *schedule.Hour)
        }
        if schedule.DayOfMonth != nil {
            fmt.Printf("   DayOfMonth: %s\n", *schedule.DayOfMonth)
        }
        if schedule.Month != nil {
            fmt.Printf("   Month: %s\n", *schedule.Month)
        }
        if schedule.DayOfWeek != nil {
            fmt.Printf("   DayOfWeek: %s\n", *schedule.DayOfWeek)
        }
        if schedule.Year != nil {
            fmt.Printf("   Year: %s\n", *schedule.Year)
        }
        fmt.Println()
    }
}
```

### Error Handling and Monitoring

```go
func runnerMonitoringExample() {
    // Create logger with JSON output for structured logging
    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelDebug,
    }))
    
    runner := jcron.NewRunner(logger)
    runner.Start()
    defer runner.Stop()
    
    // Job that sometimes fails
    unreliableJobID, err := runner.AddFuncCron("*/10 * * * * *", func() error {
        random := time.Now().Unix() % 4
        switch random {
        case 0:
            return fmt.Errorf("network timeout")
        case 1:
            return fmt.Errorf("service unavailable")
        case 2:
            fmt.Println("Job completed successfully")
            return nil
        default:
            fmt.Println("Job completed with warning")
            return nil
        }
    }, jcron.WithRetries(2, 3*time.Second))
    
    if err != nil {
        log.Fatal(err)
    }
    
    // Job that panics
    panicJobID, err := runner.AddFuncCron("*/15 * * * * *", func() error {
        if time.Now().Unix()%30 == 0 {
            panic("something went terribly wrong!")
        }
        fmt.Println("Panic job completed normally")
        return nil
    })
    
    if err != nil {
        log.Fatal(err)
    }
    
    // Job that takes long time
    longRunningJobID, err := runner.AddFuncCron("*/20 * * * * *", func() error {
        fmt.Println("Starting long-running job...")
        time.Sleep(5 * time.Second)
        fmt.Println("Long-running job completed")
        return nil
    })
    
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Monitoring jobs: %s, %s, %s\n", unreliableJobID, panicJobID, longRunningJobID)
    
    // Let it run and observe the logging
    time.Sleep(2 * time.Minute)
}
```

### Job Scheduler Implementation

```go
package main

import (
    "context"
    "fmt"
    "log"
    "sync"
    "time"
    "github.com/maple-tech/baseline/jcron"
)

type Job struct {
    ID       string
    Name     string
    Schedule jcron.Schedule
    Handler  func() error
}

type Scheduler struct {
    engine *jcron.Engine
    jobs   map[string]*Job
    mu     sync.RWMutex
    ctx    context.Context
    cancel context.CancelFunc
}

func NewScheduler() *Scheduler {
    ctx, cancel := context.WithCancel(context.Background())
    return &Scheduler{
        engine: jcron.New(),
        jobs:   make(map[string]*Job),
        ctx:    ctx,
        cancel: cancel,
    }
}

func (s *Scheduler) AddJob(job *Job) error {
    // Validate schedule
    _, err := s.engine.Next(job.Schedule, time.Now())
    if err != nil {
        return fmt.Errorf("invalid schedule: %w", err)
    }
    
    s.mu.Lock()
    s.jobs[job.ID] = job
    s.mu.Unlock()
    
    // Start job goroutine
    go s.runJob(job)
    return nil
}

func (s *Scheduler) runJob(job *Job) {
    for {
        select {
        case <-s.ctx.Done():
            return
        default:
        }
        
        now := time.Now()
        next, err := s.engine.Next(job.Schedule, now)
        if err != nil {
            log.Printf("Schedule error for job %s: %v", job.ID, err)
            return
        }
        
        // Wait until next execution time
        delay := next.Sub(now)
        timer := time.NewTimer(delay)
        
        select {
        case <-timer.C:
            // Execute job
            if err := job.Handler(); err != nil {
                log.Printf("Job %s failed: %v", job.ID, err)
            } else {
                log.Printf("Job %s completed successfully", job.ID)
            }
        case <-s.ctx.Done():
            timer.Stop()
            return
        }
    }
}

func (s *Scheduler) Stop() {
    s.cancel()
}

// Example usage
func jobSchedulerExample() {
    scheduler := NewScheduler()
    defer scheduler.Stop()
    
    // Add daily backup job
    backupJob := &Job{
        ID:   "daily-backup",
        Name: "Daily Database Backup",
        Schedule: jcron.Schedule{
            Second:     jcron.StrPtr("0"),
            Minute:     jcron.StrPtr("0"),
            Hour:       jcron.StrPtr("2"),
            DayOfMonth: jcron.StrPtr("*"),
            Month:      jcron.StrPtr("*"),
            DayOfWeek:  jcron.StrPtr("*"),
            Timezone:   jcron.StrPtr("UTC"),
        },
        Handler: func() error {
            fmt.Println("Running daily backup...")
            // Simulate backup process
            time.Sleep(1 * time.Second)
            return nil
        },
    }
    
    // Add hourly cleanup job
    cleanupJob := &Job{
        ID:   "hourly-cleanup",
        Name: "Hourly System Cleanup",
        Schedule: jcron.Schedule{
            Second:     jcron.StrPtr("0"),
            Minute:     jcron.StrPtr("0"),
            Hour:       jcron.StrPtr("*"),
            DayOfMonth: jcron.StrPtr("*"),
            Month:      jcron.StrPtr("*"),
            DayOfWeek:  jcron.StrPtr("*"),
            Timezone:   jcron.StrPtr("UTC"),
        },
        Handler: func() error {
            fmt.Println("Running hourly cleanup...")
            return nil
        },
    }
    
    if err := scheduler.AddJob(backupJob); err != nil {
        log.Fatal(err)
    }
    
    if err := scheduler.AddJob(cleanupJob); err != nil {
        log.Fatal(err)
    }
    
    fmt.Println("Scheduler started. Jobs will run according to their schedules.")
    
    // Let it run for a while
    time.Sleep(5 * time.Second)
}
```

### HTTP API Integration

```go
package main

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "time"
    "github.com/maple-tech/baseline/jcron"
)

type ScheduleAPI struct {
    engine *jcron.Engine
}

type ScheduleRequest struct {
    Schedule jcron.Schedule `json:"schedule"`
    FromTime *time.Time     `json:"from_time,omitempty"`
}

type ScheduleResponse struct {
    NextTime     *time.Time `json:"next_time,omitempty"`
    PreviousTime *time.Time `json:"previous_time,omitempty"`
    Error        string     `json:"error,omitempty"`
}

func (api *ScheduleAPI) nextHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    var req ScheduleRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    fromTime := time.Now()
    if req.FromTime != nil {
        fromTime = *req.FromTime
    }
    
    next, err := api.engine.Next(req.Schedule, fromTime)
    
    resp := ScheduleResponse{}
    if err != nil {
        resp.Error = err.Error()
        w.WriteHeader(http.StatusBadRequest)
    } else {
        resp.NextTime = &next
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}

func (api *ScheduleAPI) prevHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    var req ScheduleRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    fromTime := time.Now()
    if req.FromTime != nil {
        fromTime = *req.FromTime
    }
    
    prev, err := api.engine.Prev(req.Schedule, fromTime)
    
    resp := ScheduleResponse{}
    if err != nil {
        resp.Error = err.Error()
        w.WriteHeader(http.StatusBadRequest)
    } else {
        resp.PreviousTime = &prev
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}

func httpAPIExample() {
    api := &ScheduleAPI{
        engine: jcron.New(),
    }
    
    http.HandleFunc("/schedule/next", api.nextHandler)
    http.HandleFunc("/schedule/prev", api.prevHandler)
    
    fmt.Println("Starting HTTP API server on :8080")
    fmt.Println("Endpoints:")
    fmt.Println("  POST /schedule/next - Calculate next execution time")
    fmt.Println("  POST /schedule/prev - Calculate previous execution time")
    
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### Example HTTP Request

```bash
curl -X POST http://localhost:8080/schedule/next \
  -H "Content-Type: application/json" \
  -d '{
    "schedule": {
      "minute": "0",
      "hour": "*/2",
      "day_of_month": "*",
      "month": "*",
      "day_of_week": "1-5",
      "timezone": "UTC"
    }
  }'
```

### Runner with HTTP API Integration

Combining the built-in Runner with HTTP API for dynamic job management:

```go
package main

import (
    "encoding/json"
    "fmt"
    "log"
    "log/slog"
    "net/http"
    "os"
    "time"
    "github.com/maple-tech/baseline/jcron"
)

type JobManagerAPI struct {
    runner *jcron.Runner
    logger *slog.Logger
}

type AddJobRequest struct {
    CronExpression string                 `json:"cron_expression"`
    JobName        string                 `json:"job_name"`
    JobType        string                 `json:"job_type"`
    Parameters     map[string]interface{} `json:"parameters,omitempty"`
    RetryOptions   *RetryConfig          `json:"retry_options,omitempty"`
}

type RetryConfig struct {
    MaxRetries int    `json:"max_retries"`
    DelayMs    int    `json:"delay_ms"`
}

type JobResponse struct {
    JobID   string `json:"job_id,omitempty"`
    Message string `json:"message"`
    Error   string `json:"error,omitempty"`
}

func (api *JobManagerAPI) addJobHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    var req AddJobRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        api.respondWithError(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    // Create job based on type
    var jobFunc func() error
    switch req.JobType {
    case "log":
        message := "Default log message"
        if msg, ok := req.Parameters["message"].(string); ok {
            message = msg
        }
        jobFunc = func() error {
            api.logger.Info("Scheduled log job", "job_name", req.JobName, "message", message)
            return nil
        }
    case "http_ping":
        url := "http://localhost"
        if u, ok := req.Parameters["url"].(string); ok {
            url = u
        }
        jobFunc = func() error {
            resp, err := http.Get(url)
            if err != nil {
                return fmt.Errorf("ping failed: %w", err)
            }
            resp.Body.Close()
            api.logger.Info("HTTP ping successful", "job_name", req.JobName, "url", url, "status", resp.Status)
            return nil
        }
    case "cleanup":
        directory := "/tmp"
        if dir, ok := req.Parameters["directory"].(string); ok {
            directory = dir
        }
        jobFunc = func() error {
            api.logger.Info("Running cleanup job", "job_name", req.JobName, "directory", directory)
            // Simulate cleanup work
            time.Sleep(100 * time.Millisecond)
            return nil
        }
    default:
        api.respondWithError(w, "Unknown job type", http.StatusBadRequest)
        return
    }
    
    // Prepare job options
    var opts []jcron.JobOption
    if req.RetryOptions != nil {
        opts = append(opts, jcron.WithRetries(
            req.RetryOptions.MaxRetries,
            time.Duration(req.RetryOptions.DelayMs)*time.Millisecond,
        ))
    }
    
    // Add job to runner
    jobID, err := api.runner.AddFuncCron(req.CronExpression, jobFunc, opts...)
    if err != nil {
        api.respondWithError(w, fmt.Sprintf("Failed to add job: %v", err), http.StatusBadRequest)
        return
    }
    
    api.logger.Info("Job added via API", "job_id", jobID, "job_name", req.JobName, "cron", req.CronExpression)
    
    response := JobResponse{
        JobID:   jobID,
        Message: fmt.Sprintf("Job '%s' added successfully", req.JobName),
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func (api *JobManagerAPI) removeJobHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodDelete {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    jobID := r.URL.Query().Get("job_id")
    if jobID == "" {
        api.respondWithError(w, "job_id parameter required", http.StatusBadRequest)
        return
    }
    
    api.runner.RemoveJob(jobID)
    api.logger.Info("Job removed via API", "job_id", jobID)
    
    response := JobResponse{
        Message: fmt.Sprintf("Job %s removed successfully", jobID),
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func (api *JobManagerAPI) respondWithError(w http.ResponseWriter, message string, statusCode int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    response := JobResponse{Error: message}
    json.NewEncoder(w).Encode(response)
}

func runnerHTTPExample() {
    // Setup structured logging
    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))
    
    // Create and start runner
    runner := jcron.NewRunner(logger)
    runner.Start()
    defer runner.Stop()
    
    // Create API
    api := &JobManagerAPI{
        runner: runner,
        logger: logger,
    }
    
    // Setup HTTP routes
    http.HandleFunc("/jobs", api.addJobHandler)
    http.HandleFunc("/jobs/remove", api.removeJobHandler)
    
    // Add some initial jobs
    runner.AddFuncCron("@hourly", func() error {
        logger.Info("Hourly health check completed")
        return nil
    })
    
    runner.AddFuncCron("0 0 2 * * *", func() error {
        logger.Info("Daily backup job started")
        time.Sleep(1 * time.Second) // Simulate backup
        logger.Info("Daily backup job completed")
        return nil
    }, jcron.WithRetries(3, 5*time.Minute))
    
    fmt.Println("Job Manager API started on :8080")
    fmt.Println("Endpoints:")
    fmt.Println("  POST /jobs - Add a new scheduled job")
    fmt.Println("  DELETE /jobs/remove?job_id=<id> - Remove a job")
    fmt.Println()
    fmt.Println("Example requests:")
    fmt.Println("Add log job:")
    fmt.Println(`  curl -X POST http://localhost:8080/jobs -H "Content-Type: application/json" -d '{
    "cron_expression": "*/30 * * * * *",
    "job_name": "status_log",
    "job_type": "log",
    "parameters": {"message": "System status OK"},
    "retry_options": {"max_retries": 2, "delay_ms": 1000}
  }'`)
    fmt.Println()
    fmt.Println("Add HTTP ping job:")
    fmt.Println(`  curl -X POST http://localhost:8080/jobs -H "Content-Type: application/json" -d '{
    "cron_expression": "0 */5 * * * *",
    "job_name": "health_check",
    "job_type": "http_ping",
    "parameters": {"url": "https://httpbin.org/status/200"}
  }'`)
    
    log.Fatal(http.ListenAndServe(":8080", nil))
}

func main() {
    // Run different examples based on command line argument
    if len(os.Args) > 1 {
        switch os.Args[1] {
        case "basic":
            runnerBasicExample()
        case "advanced":
            runnerAdvancedExample()
        case "cron":
            cronSyntaxExamples()
        case "monitoring":
            runnerMonitoringExample()
        case "api":
            runnerHTTPExample()
        default:
            fmt.Println("Available examples: basic, advanced, cron, monitoring, api")
        }
    } else {
        fmt.Println("Usage: go run examples.go [basic|advanced|cron|monitoring|api]")
    }
}
```

### Complete Working Example

A production-ready example that combines all features:

```go
package main

import (
    "context"
    "fmt"
    "log/slog"
    "os"
    "os/signal"
    "syscall"
    "time"
    "github.com/maple-tech/baseline/jcron"
)

func productionExample() {
    // Setup structured logging with appropriate level
    logLevel := slog.LevelInfo
    if os.Getenv("DEBUG") == "true" {
        logLevel = slog.LevelDebug
    }
    
    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: logLevel,
    }))
    
    // Create runner
    runner := jcron.NewRunner(logger)
    
    // Setup graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    // Handle shutdown signals
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    
    go func() {
        <-sigChan
        logger.Info("Shutdown signal received, stopping runner...")
        runner.Stop()
        cancel()
    }()
    
    // Start runner
    runner.Start()
    logger.Info("Job runner started")
    
    // Add production jobs
    jobs := []struct {
        name         string
        cron         string
        handler      func() error
        retries      int
        retryDelay   time.Duration
    }{
        {
            name: "Database Health Check",
            cron: "*/30 * * * * *", // Every 30 seconds
            handler: func() error {
                // Simulate DB health check
                logger.Debug("Checking database health...")
                time.Sleep(50 * time.Millisecond)
                return nil
            },
        },
        {
            name: "Cache Cleanup",
            cron: "0 */10 * * * *", // Every 10 minutes
            handler: func() error {
                logger.Info("Running cache cleanup...")
                time.Sleep(200 * time.Millisecond)
                return nil
            },
            retries:    2,
            retryDelay: 30 * time.Second,
        },
        {
            name: "Daily Report Generation",
            cron: "0 0 6 * * *", // Daily at 6 AM
            handler: func() error {
                logger.Info("Generating daily reports...")
                time.Sleep(2 * time.Second)
                logger.Info("Daily reports generated successfully")
                return nil
            },
            retries:    3,
            retryDelay: 10 * time.Minute,
        },
        {
            name: "Weekly Data Archival",
            cron: "0 0 3 * * 0", // Sunday at 3 AM
            handler: func() error {
                logger.Info("Starting weekly data archival...")
                time.Sleep(5 * time.Second)
                logger.Info("Weekly data archival completed")
                return nil
            },
            retries:    5,
            retryDelay: 30 * time.Minute,
        },
    }
    
    // Add all jobs
    var jobIDs []string
    for _, job := range jobs {
        var opts []jcron.JobOption
        if job.retries > 0 {
            opts = append(opts, jcron.WithRetries(job.retries, job.retryDelay))
        }
        
        jobID, err := runner.AddFuncCron(job.cron, job.handler, opts...)
        if err != nil {
            logger.Error("Failed to add job", "job_name", job.name, "error", err)
            continue
        }
        
        jobIDs = append(jobIDs, jobID)
        logger.Info("Job scheduled", "job_name", job.name, "job_id", jobID, "cron", job.cron)
    }
    
    logger.Info("All jobs scheduled successfully", "total_jobs", len(jobIDs))
    
    // Wait for shutdown
    <-ctx.Done()
    logger.Info("Application shutdown complete")
}
```

---

Bu örnekler jcron kütüphanesinin tüm özelliklerini kapsamlı bir şekilde göstermektedir:

- **Built-in Runner**: Tam özellikli job scheduler
- **Cron Syntax Support**: Geleneksel cron ifadeleri ve özel formatlar
- **Retry Logic**: Başarısız işler için yeniden deneme mekanizması
- **Structured Logging**: Üretim ortamı için uygun loglama
- **HTTP API Integration**: Dinamik job yönetimi
- **Graceful Shutdown**: Güvenli uygulama kapatma
- **Production Ready**: Gerçek dünya kullanımı için hazır örnekler
