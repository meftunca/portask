package auth

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewRateLimiterMiddleware(t *testing.T) {
	t.Run("with default config", func(t *testing.T) {
		middleware := NewRateLimiterMiddleware(nil)
		
		assert.NotNil(t, middleware)
		assert.NotNil(t, middleware.config)
		assert.Equal(t, 100, middleware.config.RequestsPerSecond)
	})
	
	t.Run("with custom config", func(t *testing.T) {
		config := &RateLimitConfig{
			RequestsPerSecond: 50,
			BurstSize:         100,
			Window:            time.Minute,
		}
		middleware := NewRateLimiterMiddleware(config)
		
		assert.NotNil(t, middleware)
		assert.Equal(t, 50, middleware.config.RequestsPerSecond)
		assert.Equal(t, 100, middleware.config.BurstSize)
	})
}

func TestAdvancedRateLimiter_Allow(t *testing.T) {
	t.Run("allows requests under limit", func(t *testing.T) {
		limiter := newAdvancedRateLimiter(10, 20, time.Second)
		defer limiter.Stop()
		
		// Should allow first 20 requests (burst)
		for i := 0; i < 20; i++ {
			allowed := limiter.Allow("test-key")
			assert.True(t, allowed, "Request %d should be allowed", i)
		}
	})
	
	t.Run("blocks requests over limit", func(t *testing.T) {
		limiter := newAdvancedRateLimiter(10, 20, time.Second)
		defer limiter.Stop()
		
		// Exhaust burst
		for i := 0; i < 20; i++ {
			limiter.Allow("test-key")
		}
		
		// Next request should be blocked
		allowed := limiter.Allow("test-key")
		assert.False(t, allowed, "Request over burst should be blocked")
	})
	
	t.Run("refills tokens over time", func(t *testing.T) {
		limiter := newAdvancedRateLimiter(100, 100, time.Second)
		defer limiter.Stop()
		
		// Exhaust tokens
		for i := 0; i < 100; i++ {
			limiter.Allow("test-key")
		}
		
		// Wait for refill
		time.Sleep(100 * time.Millisecond)
		
		// Should have ~10 tokens refilled (100 tokens/sec * 0.1 sec)
		allowed := limiter.Allow("test-key")
		assert.True(t, allowed, "Should allow after token refill")
	})
	
	t.Run("handles multiple keys independently", func(t *testing.T) {
		limiter := newAdvancedRateLimiter(10, 10, time.Second)
		defer limiter.Stop()
		
		// Exhaust key1
		for i := 0; i < 10; i++ {
			limiter.Allow("key1")
		}
		
		// key1 should be blocked
		assert.False(t, limiter.Allow("key1"))
		
		// key2 should still be allowed
		assert.True(t, limiter.Allow("key2"))
	})
}

func TestAdvancedRateLimiter_Remaining(t *testing.T) {
	limiter := newAdvancedRateLimiter(10, 100, time.Second)
	defer limiter.Stop()
	
	t.Run("returns initial burst size", func(t *testing.T) {
		remaining := limiter.Remaining("test-key")
		assert.Equal(t, 100, remaining)
	})
	
	t.Run("decreases after consumption", func(t *testing.T) {
		limiter.Allow("test-key-2")
		limiter.Allow("test-key-2")
		limiter.Allow("test-key-2")
		
		remaining := limiter.Remaining("test-key-2")
		assert.True(t, remaining < 100, "Remaining should decrease")
	})
}

func TestAdvancedRateLimiter_Reset(t *testing.T) {
	limiter := newAdvancedRateLimiter(10, 10, time.Second)
	defer limiter.Stop()
	
	// Exhaust tokens
	for i := 0; i < 10; i++ {
		limiter.Allow("test-key")
	}
	
	// Should be blocked
	assert.False(t, limiter.Allow("test-key"))
	
	// Reset
	limiter.Reset("test-key")
	
	// Should be allowed again
	assert.True(t, limiter.Allow("test-key"))
}

func TestAdvancedRateLimiter_Stats(t *testing.T) {
	limiter := newAdvancedRateLimiter(100, 200, time.Minute)
	defer limiter.Stop()
	
	// Create some keys
	limiter.Allow("key1")
	limiter.Allow("key2")
	limiter.Allow("key3")
	
	stats := limiter.Stats()
	
	assert.Equal(t, 100, stats["rate"])
	assert.Equal(t, 200, stats["burst"])
	assert.Equal(t, "1m0s", stats["window"])
	assert.Equal(t, 3, stats["active_keys"])
}

func TestAdvancedRateLimiter_Cleanup(t *testing.T) {
	limiter := newAdvancedRateLimiter(10, 10, time.Millisecond)
	defer limiter.Stop()
	
	// Create some keys
	limiter.Allow("key1")
	limiter.Allow("key2")
	
	// Manually trigger cleanup
	limiter.cleanupStaleBuckets()
	
	// Note: This test is basic, full cleanup test would need time manipulation
	stats := limiter.Stats()
	assert.NotNil(t, stats)
}

func TestAdvancedRateLimiter_Concurrent(t *testing.T) {
	limiter := newAdvancedRateLimiter(1000, 2000, time.Second)
	defer limiter.Stop()
	
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	blocked := 0
	
	// Spawn 100 goroutines making requests
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			localAllowed := 0
			localBlocked := 0
			
			for j := 0; j < 30; j++ {
				if limiter.Allow("concurrent-test") {
					localAllowed++
				} else {
					localBlocked++
				}
			}
			
			mu.Lock()
			allowed += localAllowed
			blocked += localBlocked
			mu.Unlock()
		}(i)
	}
	
	wg.Wait()
	
	// Total requests = 100 * 30 = 3000
	// Should have processed all requests
	assert.Equal(t, 3000, allowed+blocked, "Should process all requests")
	// At least some should be blocked (exact number varies with timing)
	assert.True(t, blocked > 0, "Should block some excess requests")
}

func TestRateLimitByIP(t *testing.T) {
	middleware := RateLimitByIP(50)
	
	assert.NotNil(t, middleware)
	assert.Equal(t, 50, middleware.config.RequestsPerSecond)
	assert.Equal(t, 100, middleware.config.BurstSize) // 2x rate
}

func TestRateLimitByUser(t *testing.T) {
	middleware := RateLimitByUser(100)
	
	assert.NotNil(t, middleware)
	assert.Equal(t, 100, middleware.config.RequestsPerSecond)
	assert.Equal(t, 200, middleware.config.BurstSize) // 2x rate
}

func TestRateLimitByEndpoint(t *testing.T) {
	middleware := RateLimitByEndpoint(25)
	
	assert.NotNil(t, middleware)
	assert.Equal(t, 25, middleware.config.RequestsPerSecond)
	assert.Equal(t, 50, middleware.config.BurstSize) // 2x rate
}

func BenchmarkAdvancedRateLimiter_Allow(b *testing.B) {
	limiter := newAdvancedRateLimiter(1000000, 2000000, time.Second)
	defer limiter.Stop()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow("bench-key")
		}
	})
}

func BenchmarkAdvancedRateLimiter_Remaining(b *testing.B) {
	limiter := newAdvancedRateLimiter(1000000, 2000000, time.Second)
	defer limiter.Stop()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Remaining("bench-key")
	}
}

func BenchmarkAdvancedRateLimiter_MultipleKeys(b *testing.B) {
	limiter := newAdvancedRateLimiter(1000000, 2000000, time.Second)
	defer limiter.Stop()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := "bench-key-" + string(rune(i%10))
			limiter.Allow(key)
			i++
		}
	})
}

