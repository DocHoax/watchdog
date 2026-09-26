package audit

import (
	"sync"
	"time"
)

// FloodLimiter provides in-memory rate limiting for high-frequency audit failure events
// (e.g. brute-force authentication attempts) to prevent database write exhaustion
// while maintaining accurate security monitoring.
type FloodLimiter struct {
	mu          sync.Mutex
	maxBurst    int
	window      time.Duration
	records     map[string]*floodRecord
	lastCleanup time.Time
}

type floodRecord struct {
	count           int
	windowStart     time.Time
	suppressedCount int
}

// NewFloodLimiter creates a new FloodLimiter with specified burst limit and window duration.
func NewFloodLimiter(maxBurst int, window time.Duration) *FloodLimiter {
	if maxBurst <= 0 {
		maxBurst = 50 // Default max 50 failure events per window per IP
	}
	if window <= 0 {
		window = 10 * time.Second
	}
	return &FloodLimiter{
		maxBurst:    maxBurst,
		window:      window,
		records:     make(map[string]*floodRecord),
		lastCleanup: time.Now(),
	}
}

// Allow checks if the given key (e.g. source IP + event type) is permitted within rate limits.
// Returns (allowed bool, suppressedTotal int). When allowed is false, the caller should suppress storage.
func (fl *FloodLimiter) Allow(key string) (bool, int) {
	if fl == nil {
		return true, 0
	}

	fl.mu.Lock()
	defer fl.mu.Unlock()

	now := time.Now()

	// Periodic cleanup of stale entries every 5 minutes
	if now.Sub(fl.lastCleanup) > 5*time.Minute {
		fl.cleanup(now)
	}

	rec, exists := fl.records[key]
	if !exists || now.Sub(rec.windowStart) > fl.window {
		fl.records[key] = &floodRecord{
			count:           1,
			windowStart:     now,
			suppressedCount: 0,
		}
		return true, 0
	}

	rec.count++
	if rec.count <= fl.maxBurst {
		return true, 0
	}

	rec.suppressedCount++
	return false, rec.suppressedCount
}

func (fl *FloodLimiter) cleanup(now time.Time) {
	for k, rec := range fl.records {
		if now.Sub(rec.windowStart) > 2*fl.window {
			delete(fl.records, k)
		}
	}
	fl.lastCleanup = now
}
