package mcp

import (
	"testing"
	"time"
)

func TestClientRateLimiter(t *testing.T) {
	t.Run("disabled when reqPerMin <= 0", func(t *testing.T) {
		rl := NewClientRateLimiter(0, 10)
		for i := 0; i < 100; i++ {
			if !rl.Allow("client-1") {
				t.Fatalf("expected request %d to be allowed when rate limiting is disabled", i)
			}
		}
	})

	t.Run("burst limit enforcement", func(t *testing.T) {
		// 60 req/min = 1 req/sec, burst = 3
		rl := NewClientRateLimiter(60, 3)

		// First 3 requests should succeed (burst capacity)
		for i := 0; i < 3; i++ {
			if !rl.Allow("client-a") {
				t.Fatalf("expected request %d within burst to be allowed", i)
			}
		}

		// 4th request immediately should fail
		if rl.Allow("client-a") {
			t.Fatalf("expected 4th request to exceed burst and be denied")
		}

		// Different client should still have full burst
		if !rl.Allow("client-b") {
			t.Fatalf("expected separate client to be allowed")
		}
	})

	t.Run("cleanup stale buckets", func(t *testing.T) {
		rl := NewClientRateLimiter(60, 5)
		rl.Allow("client-stale")
		if rl.Size() != 1 {
			t.Fatalf("expected size 1, got %d", rl.Size())
		}

		// With zero TTL, it should clean up
		removed := rl.Cleanup(0 * time.Millisecond)
		if removed != 1 {
			t.Fatalf("expected 1 bucket removed, got %d", removed)
		}
		if rl.Size() != 0 {
			t.Fatalf("expected size 0, got %d", rl.Size())
		}
	})
}
