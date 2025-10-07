package auth

import (
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// RateLimiterMiddleware provides rate limiting functionality
type RateLimiterMiddleware struct {
	limiter *AdvancedRateLimiter
	config  *RateLimitConfig
}

// RateLimitConfig configures rate limiting
type RateLimitConfig struct {
	RequestsPerSecond int           `json:"requests_per_second"`
	BurstSize         int           `json:"burst_size"`
	Window            time.Duration `json:"window"`
	KeyFunc           KeyFunc       `json:"-"`
	Message           string        `json:"message"`
	StatusCode        int           `json:"status_code"`
}

// KeyFunc generates a rate limit key from the context
type KeyFunc func(*fiber.Ctx) string

// DefaultRateLimitConfig returns default rate limit configuration
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		RequestsPerSecond: 100,
		BurstSize:         200,
		Window:            time.Minute,
		KeyFunc:           defaultKeyFunc,
		Message:           "Rate limit exceeded",
		StatusCode:        fiber.StatusTooManyRequests,
	}
}

// defaultKeyFunc uses IP address as the key
func defaultKeyFunc(c *fiber.Ctx) string {
	return c.IP()
}

// userKeyFunc uses user ID as the key (requires authentication)
func userKeyFunc(c *fiber.Ctx) string {
	userID, err := GetUserIDFromContext(c)
	if err != nil {
		return c.IP() // Fallback to IP if user ID not available
	}
	return fmt.Sprintf("user:%s", userID)
}

// NewRateLimiterMiddleware creates a new rate limiter middleware
func NewRateLimiterMiddleware(config *RateLimitConfig) *RateLimiterMiddleware {
	if config == nil {
		config = DefaultRateLimitConfig()
	}
	
	if config.KeyFunc == nil {
		config.KeyFunc = defaultKeyFunc
	}

	return &RateLimiterMiddleware{
		limiter: newAdvancedRateLimiter(config.RequestsPerSecond, config.BurstSize, config.Window),
		config:  config,
	}
}

// Middleware returns a Fiber middleware handler
func (m *RateLimiterMiddleware) Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		key := m.config.KeyFunc(c)
		
		allowed := m.limiter.Allow(key)
		if !allowed {
			// Add rate limit headers
			c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", m.config.RequestsPerSecond))
			c.Set("X-RateLimit-Remaining", "0")
			c.Set("Retry-After", fmt.Sprintf("%d", int(m.config.Window.Seconds())))
			
			return c.Status(m.config.StatusCode).JSON(fiber.Map{
				"error":   m.config.Message,
				"code":    "RATE_LIMIT_EXCEEDED",
				"limit":   m.config.RequestsPerSecond,
				"window":  m.config.Window.String(),
			})
		}
		
		// Add rate limit info headers
		remaining := m.limiter.Remaining(key)
		c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", m.config.RequestsPerSecond))
		c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		
		return c.Next()
	}
}

// AdvancedRateLimiter implements token bucket algorithm with advanced features
type AdvancedRateLimiter struct {
	rate     int
	burst    int
	window   time.Duration
	buckets  map[string]*advancedTokenBucket
	mutex    sync.RWMutex
	cleanupInterval time.Duration
	stopCleanup chan struct{}
}

// advancedTokenBucket represents a token bucket for rate limiting
type advancedTokenBucket struct {
	tokens    int
	lastCheck time.Time
	mutex     sync.Mutex
}

// newAdvancedRateLimiter creates a new advanced rate limiter
func newAdvancedRateLimiter(rate, burst int, window time.Duration) *AdvancedRateLimiter {
	rl := &AdvancedRateLimiter{
		rate:     rate,
		burst:    burst,
		window:   window,
		buckets:  make(map[string]*advancedTokenBucket),
		cleanupInterval: time.Minute,
		stopCleanup: make(chan struct{}),
	}
	
	// Start cleanup goroutine
	go rl.cleanupRoutine()
	
	return rl
}

// Allow checks if a request is allowed
func (rl *AdvancedRateLimiter) Allow(key string) bool {
	bucket := rl.getAdvancedBucket(key)
	
	bucket.mutex.Lock()
	defer bucket.mutex.Unlock()
	
	now := time.Now()
	elapsed := now.Sub(bucket.lastCheck)
	
	// Add tokens based on elapsed time
	tokensToAdd := int(elapsed.Seconds() * float64(rl.rate))
	bucket.tokens += tokensToAdd
	
	if bucket.tokens > rl.burst {
		bucket.tokens = rl.burst
	}
	
	bucket.lastCheck = now
	
	// Check if we have tokens available
	if bucket.tokens > 0 {
		bucket.tokens--
		return true
	}
	
	return false
}

// Remaining returns the number of remaining requests
func (rl *AdvancedRateLimiter) Remaining(key string) int {
	bucket := rl.getAdvancedBucket(key)
	
	bucket.mutex.Lock()
	defer bucket.mutex.Unlock()
	
	now := time.Now()
	elapsed := now.Sub(bucket.lastCheck)
	tokensToAdd := int(elapsed.Seconds() * float64(rl.rate))
	
	tokens := bucket.tokens + tokensToAdd
	if tokens > rl.burst {
		tokens = rl.burst
	}
	
	return tokens
}

// getAdvancedBucket gets or creates a token bucket for a key
func (rl *AdvancedRateLimiter) getAdvancedBucket(key string) *advancedTokenBucket {
	rl.mutex.RLock()
	bucket, exists := rl.buckets[key]
	rl.mutex.RUnlock()
	
	if exists {
		return bucket
	}
	
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	
	// Double-check after acquiring write lock
	bucket, exists = rl.buckets[key]
	if exists {
		return bucket
	}
	
	bucket = &advancedTokenBucket{
		tokens:    rl.burst,
		lastCheck: time.Now(),
	}
	rl.buckets[key] = bucket
	
	return bucket
}

// cleanupRoutine removes stale buckets
func (rl *AdvancedRateLimiter) cleanupRoutine() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			rl.cleanupStaleBuckets()
		case <-rl.stopCleanup:
			return
		}
	}
}

// cleanupStaleBuckets removes buckets that haven't been used recently
func (rl *AdvancedRateLimiter) cleanupStaleBuckets() {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	
	now := time.Now()
	staleThreshold := now.Add(-10 * time.Minute)
	
	for key, bucket := range rl.buckets {
		bucket.mutex.Lock()
		if bucket.lastCheck.Before(staleThreshold) {
			delete(rl.buckets, key)
		}
		bucket.mutex.Unlock()
	}
}

// Stop stops the rate limiter cleanup goroutine
func (rl *AdvancedRateLimiter) Stop() {
	close(rl.stopCleanup)
}

// Reset resets the rate limit for a specific key
func (rl *AdvancedRateLimiter) Reset(key string) {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	delete(rl.buckets, key)
}

// Stats returns rate limiter statistics
func (rl *AdvancedRateLimiter) Stats() map[string]interface{} {
	rl.mutex.RLock()
	defer rl.mutex.RUnlock()
	
	return map[string]interface{}{
		"rate":          rl.rate,
		"burst":         rl.burst,
		"window":        rl.window.String(),
		"active_keys":   len(rl.buckets),
	}
}

// RateLimitByUser returns a rate limiter that uses user ID
func RateLimitByUser(rps int) *RateLimiterMiddleware {
	config := &RateLimitConfig{
		RequestsPerSecond: rps,
		BurstSize:         rps * 2,
		Window:            time.Minute,
		KeyFunc:           userKeyFunc,
		Message:           "User rate limit exceeded",
		StatusCode:        fiber.StatusTooManyRequests,
	}
	return NewRateLimiterMiddleware(config)
}

// RateLimitByIP returns a rate limiter that uses IP address
func RateLimitByIP(rps int) *RateLimiterMiddleware {
	config := &RateLimitConfig{
		RequestsPerSecond: rps,
		BurstSize:         rps * 2,
		Window:            time.Minute,
		KeyFunc:           defaultKeyFunc,
		Message:           "IP rate limit exceeded",
		StatusCode:        fiber.StatusTooManyRequests,
	}
	return NewRateLimiterMiddleware(config)
}

// RateLimitByEndpoint returns a rate limiter that uses endpoint + IP
func RateLimitByEndpoint(rps int) *RateLimiterMiddleware {
	config := &RateLimitConfig{
		RequestsPerSecond: rps,
		BurstSize:         rps * 2,
		Window:            time.Minute,
		KeyFunc: func(c *fiber.Ctx) string {
			return fmt.Sprintf("%s:%s", c.Path(), c.IP())
		},
		Message:    "Endpoint rate limit exceeded",
		StatusCode: fiber.StatusTooManyRequests,
	}
	return NewRateLimiterMiddleware(config)
}

