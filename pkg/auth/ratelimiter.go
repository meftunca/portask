package auth

import (
	"sync"
	"time"
)

// RateLimiter implements a simple token bucket rate limiter
type RateLimiter struct {
	requestsPerSecond int
	buckets           map[string]*bucket
	mu                sync.RWMutex
	cleanupTicker     *time.Ticker
}

// bucket represents a token bucket for a specific client
type bucket struct {
	tokens     int
	lastRefill time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerSecond int) *RateLimiter {
	rl := &RateLimiter{
		requestsPerSecond: requestsPerSecond,
		buckets:           make(map[string]*bucket),
		cleanupTicker:     time.NewTicker(time.Minute),
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Allow checks if a request from the given client should be allowed
func (rl *RateLimiter) Allow(clientID string) bool {
	rl.mu.RLock()
	b, exists := rl.buckets[clientID]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		// Double-check after acquiring write lock
		if b, exists = rl.buckets[clientID]; !exists {
			b = &bucket{
				tokens:     rl.requestsPerSecond,
				lastRefill: time.Now(),
			}
			rl.buckets[clientID] = b
		}
		rl.mu.Unlock()
	}

	return b.consumeToken(rl.requestsPerSecond)
}

// consumeToken attempts to consume a token from the bucket
func (b *bucket) consumeToken(refillRate int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefill)

	// Refill tokens based on elapsed time
	tokensToAdd := int(elapsed.Seconds()) * refillRate
	if tokensToAdd > 0 {
		b.tokens += tokensToAdd
		if b.tokens > refillRate {
			b.tokens = refillRate // Cap at max capacity
		}
		b.lastRefill = now
	}

	// Try to consume a token
	if b.tokens > 0 {
		b.tokens--
		return true
	}

	return false
}

// cleanup removes old buckets that haven't been used recently
func (rl *RateLimiter) cleanup() {
	for range rl.cleanupTicker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-5 * time.Minute)

		for clientID, bucket := range rl.buckets {
			bucket.mu.Lock()
			if bucket.lastRefill.Before(cutoff) {
				delete(rl.buckets, clientID)
			}
			bucket.mu.Unlock()
		}
		rl.mu.Unlock()
	}
}

// Stop stops the rate limiter and cleans up resources
func (rl *RateLimiter) Stop() {
	if rl.cleanupTicker != nil {
		rl.cleanupTicker.Stop()
	}
}
