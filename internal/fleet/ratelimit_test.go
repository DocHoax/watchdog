package fleet

import (
	"sync"
	"testing"
	"time"
)

func TestNodeRateLimiter_Allow(t *testing.T) {
	// 1 token per second, burst 2
	rl := NewNodeRateLimiter(1.0, 2)

	nodeID := "node-1"

	// 1st request should be allowed (burst has 2, consumed 1, tokens=1)
	if !rl.Allow(nodeID) {
		t.Errorf("Expected 1st request to be allowed")
	}

	// 2nd request should be allowed (consumed 1, tokens=0)
	if !rl.Allow(nodeID) {
		t.Errorf("Expected 2nd request to be allowed")
	}

	// 3rd request should be rejected (tokens < 1.0)
	if rl.Allow(nodeID) {
		t.Errorf("Expected 3rd immediate request to be rejected")
	}

	// Wait for token refill (1.1s -> ~1.1 tokens)
	time.Sleep(1100 * time.Millisecond)

	// 4th request should now be allowed
	if !rl.Allow(nodeID) {
		t.Errorf("Expected request after refill to be allowed")
	}

	// Empty node ID should be rejected
	if rl.Allow("") {
		t.Errorf("Expected empty node ID to be rejected")
	}
}

func TestNodeRateLimiter_Cleanup(t *testing.T) {
	rl := NewNodeRateLimiter(5.0, 10)

	rl.Allow("node-active")
	rl.Allow("node-idle")

	if rl.Size() != 2 {
		t.Errorf("Expected 2 tracked buckets, got %d", rl.Size())
	}

	// Sleep 50ms and cleanup with TTL of 20ms
	time.Sleep(50 * time.Millisecond)
	removed := rl.Cleanup(20 * time.Millisecond)

	if removed != 2 {
		t.Errorf("Expected 2 removed buckets, got %d", removed)
	}
	if rl.Size() != 0 {
		t.Errorf("Expected 0 buckets after cleanup, got %d", rl.Size())
	}
}

func TestNodeRateLimiter_Concurrent(t *testing.T) {
	rl := NewNodeRateLimiter(100.0, 50)
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			nodeID := "node-concurrent"
			for j := 0; j < 50; j++ {
				_ = rl.Allow(nodeID)
			}
		}(i)
	}

	wg.Wait()
}
