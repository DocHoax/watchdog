package collector

import (
	"context"
	"testing"
)

// BenchmarkProcessCollector_GetProcesses measures live Linux process inspection.
// Each iteration reads the host's /proc process state through gopsutil and
// builds the complete ProcessSummary. Setup is intentionally outside timing.
func BenchmarkProcessCollector_GetProcesses(b *testing.B) {
	collector := NewProcessCollector()
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		summary, err := collector.GetProcesses(ctx, 0, "cpu", "")
		if err != nil {
			b.Fatalf("process collection failed: %v", err)
		}
		if summary == nil {
			b.Fatal("expected non-nil process summary")
		}
	}
}

// BenchmarkPortCollector_GetListeningPorts measures live Linux socket
// inspection, including listening-port discovery and process-name lookup.
func BenchmarkPortCollector_GetListeningPorts(b *testing.B) {
	collector := NewPortCollector()
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ports, err := collector.GetListeningPorts(ctx)
		if err != nil {
			b.Fatalf("port collection failed: %v", err)
		}
		if ports == nil {
			b.Fatal("expected non-nil port list")
		}
	}
}
