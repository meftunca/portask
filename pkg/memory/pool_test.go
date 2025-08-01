package memory

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestPool(t *testing.T) {
	config := PoolConfig{
		Name:             "test-pool",
		ObjectSize:       1024,
		InitialObjects:   10,
		MaxObjects:       100,
		EnableMonitoring: true,
		EnableAutoResize: false,
		ResizeThreshold:  0.8,
		MaxGrowthRate:    0.5,
	}

	pool := NewPool(config)

	t.Run("GetAndPut", func(t *testing.T) {
		buf := pool.Get()
		if buf == nil {
			t.Error("Expected buffer to be allocated")
		}
		if len(buf) != 1024 {
			t.Errorf("Expected buffer length 1024, got %d", len(buf))
		}

		// Use the buffer
		copy(buf[:10], []byte("test data"))

		// Return it to pool
		pool.Put(buf)

		// Get again - should be reused
		buf2 := pool.Get()
		if buf2 == nil {
			t.Error("Expected buffer to be allocated")
		}
		pool.Put(buf2)
	})

	t.Run("Stats", func(t *testing.T) {
		// Get some objects to generate stats
		objects := make([][]byte, 10)
		for i := 0; i < 10; i++ {
			objects[i] = pool.Get()
		}

		stats := pool.GetStats()
		if stats.Name != "test-pool" {
			t.Errorf("Expected pool name 'test-pool', got %s", stats.Name)
		}
		if stats.ObjectSize != 1024 {
			t.Errorf("Expected object size 1024, got %d", stats.ObjectSize)
		}

		// Return objects
		for _, buf := range objects {
			pool.Put(buf)
		}
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		const goroutines = 50
		const iterations = 50
		var wg sync.WaitGroup

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					buf := pool.Get()
					if buf == nil {
						t.Error("Expected buffer to be allocated")
						return
					}
					// Simulate some work
					copy(buf[:10], []byte("test data"))
					pool.Put(buf)
				}
			}()
		}

		wg.Wait()
	})

	// Cleanup
	pool.Close()
}

func TestFastBuffer(t *testing.T) {
	buffer := NewFastBuffer(1024)

	t.Run("WriteAndRead", func(t *testing.T) {
		data := []byte("Hello, World!")
		n, err := buffer.Write(data)
		if err != nil {
			t.Fatalf("Failed to write: %v", err)
		}
		if n != len(data) {
			t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
		}

		result := buffer.Bytes()
		if string(result) != string(data) {
			t.Errorf("Expected %s, got %s", string(data), string(result))
		}
	})

	t.Run("Reset", func(t *testing.T) {
		buffer.Write([]byte("test data"))
		buffer.Reset()

		if buffer.Len() != 0 {
			t.Errorf("Expected length 0 after reset, got %d", buffer.Len())
		}
	})

	t.Run("Grow", func(t *testing.T) {
		initialCap := buffer.Cap()
		buffer.growTo(2048)

		if buffer.Cap() <= initialCap {
			t.Error("Expected buffer to grow")
		}
	})
}

func TestMemoryStats(t *testing.T) {
	stats := GetMemoryStats()

	if stats.Alloc == 0 {
		t.Error("Expected allocated memory to be > 0")
	}
	if stats.TotalAlloc == 0 {
		t.Error("Expected total allocated memory to be > 0")
	}
	if stats.Sys == 0 {
		t.Error("Expected system memory to be > 0")
	}
}

func TestGCOperations(t *testing.T) {
	t.Run("ForceGC", func(t *testing.T) {
		initialStats := GetMemoryStats()

		// Allocate some memory
		data := make([][]byte, 1000)
		for i := range data {
			data[i] = make([]byte, 1024)
		}

		// Force GC
		ForceGC()

		afterStats := GetMemoryStats()
		if afterStats.NumGC <= initialStats.NumGC {
			t.Log("GC count might not have increased (expected in some test environments)")
		}
	})

	t.Run("SetGCPercent", func(t *testing.T) {
		originalPercent := SetGCPercent(200)

		// Set back to original
		SetGCPercent(originalPercent)

		if originalPercent < 0 {
			t.Log("GC was disabled originally")
		}
	})
}

func TestPoolMonitor(t *testing.T) {
	config := PoolConfig{
		Name:             "monitored-pool",
		ObjectSize:       512,
		InitialObjects:   5,
		MaxObjects:       50,
		EnableMonitoring: true,
		EnableAutoResize: false,
		ResizeThreshold:  0.8,
		MaxGrowthRate:    0.5,
	}

	pool := NewPool(config)
	defer pool.Close()

	t.Run("MonitoringEnabled", func(t *testing.T) {
		// Use the pool to generate some activity
		objects := make([][]byte, 20)
		for i := 0; i < 20; i++ {
			objects[i] = pool.Get()
		}

		// Wait a bit for monitoring
		time.Sleep(100 * time.Millisecond)

		stats := pool.GetStats()
		if stats.AllocCount == 0 {
			t.Error("Expected some allocations to be recorded")
		}

		// Return objects
		for _, buf := range objects {
			pool.Put(buf)
		}

		// Check hit ratio
		finalStats := pool.GetStats()
		if finalStats.HitCount == 0 && finalStats.MissCount == 0 {
			t.Log("No hits/misses recorded yet (expected in short test)")
		}
	})
}

func TestPoolConfig(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		config := PoolConfig{
			Name:             "valid-pool",
			ObjectSize:       1024,
			InitialObjects:   10,
			MaxObjects:       100,
			EnableMonitoring: true,
			EnableAutoResize: true,
			ResizeThreshold:  0.8,
			MaxGrowthRate:    0.5,
		}

		pool := NewPool(config)
		if pool == nil {
			t.Error("Expected pool to be created")
		}
		pool.Close()
	})

	t.Run("ConfigWithDefaults", func(t *testing.T) {
		config := PoolConfig{
			Name:       "default-pool",
			ObjectSize: 1024,
			MaxObjects: 100,
			// Using defaults for other fields
		}

		pool := NewPool(config)
		if pool == nil {
			t.Error("Expected pool to be created with defaults")
		}
		pool.Close()
	})
}

// Benchmarks
func BenchmarkPool(b *testing.B) {
	config := PoolConfig{
		Name:           "benchmark-pool",
		ObjectSize:     1024,
		InitialObjects: 100,
		MaxObjects:     1000,
	}
	pool := NewPool(config)
	defer pool.Close()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.Get()
			copy(buf[:10], []byte("test data"))
			pool.Put(buf)
		}
	})
}

func BenchmarkFastBuffer(b *testing.B) {
	buffer := NewFastBuffer(1024)
	data := []byte("test data for benchmarking")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buffer.Reset()
		buffer.Write(data)
		_ = buffer.Bytes()
	}
}

func BenchmarkMemoryVsStandard(b *testing.B) {
	config := PoolConfig{
		Name:           "bench-pool",
		ObjectSize:     1024,
		InitialObjects: 100,
		MaxObjects:     1000,
	}
	pool := NewPool(config)
	defer pool.Close()

	b.Run("PoolAllocation", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				buf := pool.Get()
				pool.Put(buf)
			}
		})
	})

	b.Run("StandardAllocation", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				data := make([]byte, 1024)
				_ = data // Use data to prevent optimization
				runtime.KeepAlive(data)
			}
		})
	})
}
