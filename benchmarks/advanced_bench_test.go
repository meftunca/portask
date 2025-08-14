package benchmarks

import (
	"testing"
	"time"
)

func BenchmarkBatchProcessing(b *testing.B) {
	for i := 0; i < b.N; i++ {
		time.Sleep(1 * time.Millisecond) // stub
	}
}
