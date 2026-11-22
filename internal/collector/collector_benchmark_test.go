package collector

import (
	"context"
	"testing"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/stretchr/testify/require"
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

// BenchmarkCollector_Process benchmarks process collection.
func BenchmarkCollector_Process(b *testing.B) {
	c := NewProcessCollector()
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Collect(ctx)
	}
}

// BenchmarkCollector_Port benchmarks port collection.
func BenchmarkCollector_Port(b *testing.B) {
	c := NewPortCollector()
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Collect(ctx)
	}
}
