package fleet

import (
	"sync"
	"time"
)

// tokenBucket represents the rate limit state for a single node.
type tokenBucket struct {
	tokens     float64
	lastRefill time.Time
}

// NodeRateLimiter provides per-node token bucket rate limiting to prevent telemetry storms.
type NodeRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	rate    float64 // tokens per second
	burst   int     // max burst capacity
}

// NewNodeRateLimiter creates a new NodeRateLimiter.
func NewNodeRateLimiter(rate float64, burst int) *NodeRateLimiter {
	if rate <= 0 {
		rate = 5.0 // default 5 requests per second
	}
	if burst <= 0 {
		burst = 10 // default burst of 10
	}

	return &NodeRateLimiter{
		buckets: make(map[string]*tokenBucket),
		rate:    rate,
		burst:   burst,
	}
}

// Allow reports whether a request from the given nodeID is permitted under the rate limit.
func (rl *NodeRateLimiter) Allow(nodeID string) bool {
	if nodeID == "" {
		return false
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, exists := rl.buckets[nodeID]
	if !exists {
		// Initialize bucket at full burst capacity minus 1 for the current request
		rl.buckets[nodeID] = &tokenBucket{
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

// Cleanup removes tracking buckets for nodes that haven't sent requests within the specified TTL.
func (rl *NodeRateLimiter) Cleanup(ttl time.Duration) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	removed := 0
	for id, b := range rl.buckets {
		if now.Sub(b.lastRefill) > ttl {
			delete(rl.buckets, id)
			removed++
		}
	}
	return removed
}

// Size returns the count of tracked node buckets.
func (rl *NodeRateLimiter) Size() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return len(rl.buckets)
}
