package jcron

import (
	"testing"
	"time"
)

// Quick benchmark test to measure direct algorithm performance
func BenchmarkDirectQuick_L(b *testing.B) {
	engine := New()
	schedule := strPtrSchedule("0", "0", "12", "L", "*", "*", "*", "UTC")
	testTime := mustParseTime(time.RFC3339, "2025-01-15T10:00:00Z")

	// Pre-warm cache
	engine.Next(schedule, testTime)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = engine.Next(schedule, testTime)
	}
}

func BenchmarkDirectQuick_Hash(b *testing.B) {
	engine := New()
	schedule := strPtrSchedule("0", "0", "12", "*", "*", "1#2", "*", "UTC")
	testTime := mustParseTime(time.RFC3339, "2025-01-01T10:00:00Z")

	// Pre-warm cache
	engine.Next(schedule, testTime)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = engine.Next(schedule, testTime)
	}
}

func BenchmarkDirectQuick_LastWeekday(b *testing.B) {
	engine := New()
	schedule := strPtrSchedule("0", "0", "12", "*", "*", "5L", "*", "UTC")
	testTime := mustParseTime(time.RFC3339, "2025-01-01T10:00:00Z")

	// Pre-warm cache
	engine.Next(schedule, testTime)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = engine.Next(schedule, testTime)
	}
}
