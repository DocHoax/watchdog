package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/pkg/model"
)

func createBenchmarkServer() (*Server, http.Handler) {
	cfg := config.DefaultConfig()
	cfg.Agent.Token = "bench-secret-token"

	srv := NewServer(cfg, nil, nil, nil, nil, nil)
	srv.latestSnapshot = &model.SystemSnapshot{
		Timestamp: time.Now(),
		System: &model.SystemInfo{
			Hostname:   "bench-api-host",
			OS:         "linux",
			KernelArch: "x86_64",
			Uptime:     123456,
		},
		CPU: &model.CPUInfo{
			OverallUsage:  42.5,
			PhysicalCores: 8,
			LogicalCores:  16,
			LoadAverage:   model.LoadAvg{Load1: 1.5, Load5: 1.2, Load15: 0.9},
		},
		Memory: &model.MemoryInfo{
			TotalBytes:      16 * 1024 * 1024 * 1024,
			AvailableBytes:  8 * 1024 * 1024 * 1024,
			UsedBytes:       8 * 1024 * 1024 * 1024,
			UsedPercent:     50.0,
			SwapTotalBytes:  4 * 1024 * 1024 * 1024,
			SwapUsedBytes:   256 * 1024 * 1024,
			SwapUsedPercent: 6.25,
		},
		Disk: &model.DiskInfo{
			TotalBytes:  500 * 1024 * 1024 * 1024,
			UsedBytes:   250 * 1024 * 1024 * 1024,
			UsedPercent: 50.0,
			Partitions: []model.PartitionInfo{
				{Mountpoint: "/", TotalBytes: 500 * 1024 * 1024 * 1024, UsedBytes: 250 * 1024 * 1024 * 1024, UsedPercent: 50.0},
			},
		},
		Network: &model.NetworkInfo{
			TotalRxRate: 1024 * 1024,
			TotalTxRate: 512 * 1024,
		},
		Processes: &model.ProcessSummary{
			TotalCount:   150,
			RunningCount: 4,
		},
	}
	srv.latestDiag = &model.DiagnosticReport{
		TotalChecks:  10,
		PassedChecks: 10,
	}
	srv.activeAlerts = []model.AlertEvent{}
	srv.latestAnomaly = &model.AnomalyReport{}
	srv.exporter.Update(srv.latestSnapshot, srv.latestDiag, srv.activeAlerts, srv.latestAnomaly)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", srv.handleHealth)
	mux.HandleFunc("/metrics", srv.exporter.Handler())
	mux.Handle("/api/v1/snapshot", srv.authMiddleware(http.HandlerFunc(srv.handleSnapshot)))

	return srv, mux
}

// BenchmarkServer_SnapshotEndpoint benchmarks requests/sec for /api/v1/snapshot with Bearer auth.
func BenchmarkServer_SnapshotEndpoint(b *testing.B) {
	_, handler := createBenchmarkServer()

	req, err := http.NewRequest("GET", "/api/v1/snapshot", nil)
	if err != nil {
		b.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer bench-secret-token")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("expected status 200, got %d", rec.Code)
		}
	}
}

// BenchmarkServer_PrometheusEndpoint benchmarks requests/sec for /metrics scraping endpoint.
func BenchmarkServer_PrometheusEndpoint(b *testing.B) {
	_, handler := createBenchmarkServer()

	req, err := http.NewRequest("GET", "/metrics", nil)
	if err != nil {
		b.Fatalf("failed to create request: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("expected status 200, got %d", rec.Code)
		}
	}
}
