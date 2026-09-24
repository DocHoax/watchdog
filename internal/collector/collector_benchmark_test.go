package collector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/DocHoax/watchdog/internal/config"
)

// BenchmarkCollector_FullCycle benchmarks the full snapshot collection cycle.
func BenchmarkCollector_FullCycle(b *testing.B) {
	cfg := config.DefaultConfig()
	mgr := NewDefaultManager(cfg)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		snap, err := mgr.CollectAll(ctx)
		if err != nil {
			b.Fatalf("collection cycle failed: %v", err)
		}
		if snap == nil {
			b.Fatalf("expected non-nil snapshot")
		}
	}
}

// BenchmarkCollector_CoreMetrics benchmarks core system collectors (CPU, Memory, System, Disk).
func BenchmarkCollector_CoreMetrics(b *testing.B) {
	mgr := NewManager()
	mgr.Register(NewSystemCollector())
	mgr.Register(NewCPUCollector())
	mgr.Register(NewMemoryCollector())
	mgr.Register(NewDiskCollector())

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := mgr.CollectAll(ctx)
		require.NoError(b, err)
	}
}
