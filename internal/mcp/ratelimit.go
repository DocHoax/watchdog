package mcp

import (
	"sync"
	"time"
)

// clientBucket represents the token bucket rate limit state for a single MCP client.
type clientBucket struct {
	tokens     float64
	lastRefill time.Time
}

// ClientRateLimiter provides per-client token-bucket rate limiting to prevent AI tool loop exhaustion.
type ClientRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*clientBucket
	rate    float64 // tokens per second
	burst   int     // max burst capacity
	enabled bool
}

// NewClientRateLimiter creates a new ClientRateLimiter from requests-per-minute and burst capacity.
// If reqPerMin <= 0, rate limiting is disabled (Allow will always return true).
func NewClientRateLimiter(reqPerMin int, burst int) *ClientRateLimiter {
	if reqPerMin <= 0 {
		return &ClientRateLimiter{
			enabled: false,
		}
	}

	rate := float64(reqPerMin) / 60.0
	if burst <= 0 {
		burst = 20
	}

	return &ClientRateLimiter{
		buckets: make(map[string]*clientBucket),
		rate:    rate,
		burst:   burst,
		enabled: true,
	}
}

// Allow reports whether a request from the given clientID is permitted under the rate limit.
func (rl *ClientRateLimiter) Allow(clientID string) bool {
	if rl == nil || !rl.enabled {
		return true
	}

	if clientID == "" {
		clientID = "anonymous"
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, exists := rl.buckets[clientID]
	if !exists {
		// Initialize bucket at full burst capacity minus 1 for current request
		rl.buckets[clientID] = &clientBucket{
			tokens:     float64(rl.burst) - 1.0,
			lastRefill: now,
		}
		return true
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.lastRefill = now
	b.tokens += elapsed * rl.rate
	if b.tokens > float64(rl.burst) {
		b.tokens = float64(rl.burst)
	}

	// Check if token available
	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		return true
	}

	return false
}

// Cleanup removes tracking buckets for clients that have not sent requests within the specified TTL.
func (rl *ClientRateLimiter) Cleanup(ttl time.Duration) int {
	if rl == nil || !rl.enabled {
		return 0
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	removed := 0
	for id, b := range rl.buckets {
		if now.Sub(b.lastRefill) >= ttl {
			delete(rl.buckets, id)
			removed++
		}
	}
	return removed
}

// Size returns the count of tracked client buckets.
func (rl *ClientRateLimiter) Size() int {
	if rl == nil || !rl.enabled {
		return 0
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()
	return len(rl.buckets)
}
