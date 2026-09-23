package collector

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestCollector_GoroutineLeakAndResourceSafety(t *testing.T) {
	mgr := NewManager()
	mgr.Register(NewSystemCollector())
	mgr.Register(NewCPUCollector())
	mgr.Register(NewMemoryCollector())
	mgr.Register(NewDiskCollector())
	mgr.Register(NewNetworkCollector())
	mgr.Register(NewPortCollector())
	mgr.Register(NewDockerCollector())
	mgr.Register(NewKubernetesCollector())

	// Warmup run
	ctxWarmup, cancelWarmup := context.WithTimeout(context.Background(), 5*time.Second)
	_, _ = mgr.CollectAll(ctxWarmup)
	cancelWarmup()

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	initialGoroutines := runtime.NumGoroutine()

	// Execute multiple sampling cycles
	iterations := 15
	for i := 0; i < iterations; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_, err := mgr.CollectAll(ctx)
		cancel()
		if err != nil {
			t.Logf("iteration %d returned partial error: %v", i, err)
		}
	}

	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	finalGoroutines := runtime.NumGoroutine()

	// Verify no unbounded goroutine expansion (allowing small runtime jitter of <= 5)
	delta := finalGoroutines - initialGoroutines
	if delta > 5 {
		t.Fatalf("potential goroutine leak: initial=%d, final=%d, delta=%d", initialGoroutines, finalGoroutines, delta)
	}
}

func TestCollector_RapidContextCancellation(t *testing.T) {
	mgr := NewManager()
	mgr.Register(NewSystemCollector())
	mgr.Register(NewCPUCollector())
	mgr.Register(NewMemoryCollector())
	mgr.Register(NewDiskCollector())
	mgr.Register(NewNetworkCollector())

	// Rapid cancel triggers
	for i := 0; i < 20; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		// Cancel immediately or after 1ms
		go func() {
			time.Sleep(time.Duration(i%5) * time.Millisecond)
			cancel()
		}()
		_, _ = mgr.CollectAll(ctx)
	}
}
