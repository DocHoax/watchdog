package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleSnapshot() *model.SystemSnapshot {
	return &model.SystemSnapshot{
		Timestamp: time.Now(),
		System: &model.SystemInfo{
			Hostname: "test-node",
			OS:       "linux",
		},
		CPU: &model.CPUInfo{
			OverallUsage: 45.5,
			LogicalCores: 8,
			LoadAverage:  model.LoadAvg{Load1: 1.2, Load5: 1.0, Load15: 0.8},
		},
		Memory: &model.MemoryInfo{
			TotalBytes:     16 * 1024 * 1024 * 1024,
			UsedBytes:      8 * 1024 * 1024 * 1024,
			AvailableBytes: 8 * 1024 * 1024 * 1024,
			UsedPercent:    50.0,
		},
		Disk: &model.DiskInfo{
			TotalBytes:  500 * 1024 * 1024 * 1024,
			UsedBytes:   250 * 1024 * 1024 * 1024,
			FreeBytes:   250 * 1024 * 1024 * 1024,
			UsedPercent: 50.0,
		},
		Network: &model.NetworkInfo{
			TotalRxRate: 1024 * 1024,
			TotalTxRate: 512 * 1024,
		},
		Processes: &model.ProcessSummary{
			TotalCount:   150,
			RunningCount: 5,
		},
	}
}

func TestPrometheusExporter(t *testing.T) {
	exporter := NewPrometheusExporter()
	snap := sampleSnapshot()
	alerts := []model.AlertEvent{
		{
			RuleName: "HighCPU",
			Severity: model.SeverityCritical,
		},
	}
	anom := &model.AnomalyReport{
		Scores: []model.AnomalyScore{
			{
				MetricName: "cpu_usage",
				IsAnomaly:  true,
				ZScore:     3.2,
			},
		},
	}

	exporter.Update(snap, nil, alerts, anom)
	rendered := exporter.RenderMetrics()

	assert.Contains(t, rendered, "watchdog_cpu_usage_percent 45.50")
	assert.Contains(t, rendered, "watchdog_memory_used_percent 50.00")
	assert.Contains(t, rendered, "watchdog_disk_used_percent 50.00")
	assert.Contains(t, rendered, "watchdog_active_alerts_total{severity=\"critical\"} 1")
	assert.Contains(t, rendered, "watchdog_anomaly_detected{metric=\"cpu_usage\"} 1")
}

func TestServer_APIEndpointsAndAuth(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agent.Token = "secret-token-123"

	srv := NewServer(cfg, nil, nil, nil, nil, nil)
	srv.latestSnapshot = sampleSnapshot()
	srv.latestDiag = &model.DiagnosticReport{
		TotalChecks:  5,
		PassedChecks: 5,
	}
	srv.activeAlerts = []model.AlertEvent{
		{RuleName: "HighCPU", Severity: model.SeverityWarning},
	}
	srv.latestAnomaly = &model.AnomalyReport{}
	srv.exporter.Update(srv.latestSnapshot, srv.latestDiag, srv.activeAlerts, srv.latestAnomaly)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", srv.handleHealth)
	mux.HandleFunc("/api/v1/health", srv.handleHealth)
	mux.HandleFunc("/metrics", srv.exporter.Handler())
	mux.Handle("/api/v1/snapshot", srv.authMiddleware(http.HandlerFunc(srv.handleSnapshot)))
	mux.Handle("/api/v1/diagnostics", srv.authMiddleware(http.HandlerFunc(srv.handleDiagnostics)))
	mux.Handle("/api/v1/alerts", srv.authMiddleware(http.HandlerFunc(srv.handleAlerts)))

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// 1. Unauthenticated Health Check (Should pass)
	resp, err := http.Get(ts.URL + "/health")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// 2. Metrics (Should pass)
	resp, err = http.Get(ts.URL + "/metrics")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// 3. Snapshot without Token (Should fail with 401)
	resp, err = http.Get(ts.URL + "/api/v1/snapshot")
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()

	// 4. Snapshot with Valid Token via Client
	client := NewClient(ts.URL, "secret-token-123", true)
	snap, err := client.FetchSnapshot(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Equal(t, "test-node", snap.System.Hostname)

	// 5. Diagnostics via Client
	diag, err := client.FetchDiagnostics(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, diag)
	assert.Equal(t, 5, diag.TotalChecks)

	// 6. Alerts via Client
	activeAlerts, err := client.FetchAlerts(context.Background())
	require.NoError(t, err)
	assert.Len(t, activeAlerts, 1)
	assert.Equal(t, "HighCPU", activeAlerts[0].RuleName)

	// 7. Health via Client
	health, err := client.GetHealth(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ok", health["status"])
}

func TestPanicRecoveryMiddleware(t *testing.T) {
	panickingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("simulated fatal nil pointer or divide by zero")
	})

	recoveredHandler := PanicRecoveryMiddleware(panickingHandler, nil)

	req := httptest.NewRequest(http.MethodGet, "/panicking", nil)
	rr := httptest.NewRecorder()

	assert.NotPanics(t, func() {
		recoveredHandler.ServeHTTP(rr, req)
	})

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "Internal server error")
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
}

func TestMaxBodySizeMiddleware(t *testing.T) {
	echoHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		_, err := r.Body.Read(buf)
		if err != nil && err.Error() == "http: request body too large" {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// 100 byte limit
	limited := MaxBodySizeMiddleware(100)(echoHandler)

	// Small payload (within limit)
	smallBody := strings.NewReader("small payload")
	reqSmall := httptest.NewRequest(http.MethodPost, "/upload", smallBody)
	rrSmall := httptest.NewRecorder()
	limited.ServeHTTP(rrSmall, reqSmall)
	assert.Equal(t, http.StatusOK, rrSmall.Code)

	// Oversized payload (> 100 bytes)
	largeBody := strings.NewReader(strings.Repeat("A", 200))
	reqLarge := httptest.NewRequest(http.MethodPost, "/upload", largeBody)
	rrLarge := httptest.NewRecorder()
	limited.ServeHTTP(rrLarge, reqLarge)
	assert.Equal(t, http.StatusRequestEntityTooLarge, rrLarge.Code)
}

func TestServer_HealthAndReadiness(t *testing.T) {
	cfg := config.DefaultConfig()
	srv := NewServer(cfg, nil, nil, nil, nil, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", srv.handleHealth)
	mux.HandleFunc("/healthz", srv.handleHealth)
	mux.HandleFunc("/ready", srv.handleReady)
	mux.HandleFunc("/readyz", srv.handleReady)
	mux.HandleFunc("/api/v1/ready", srv.handleReady)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// 1. Healthz should return 200 OK immediately
	resp, err := http.Get(ts.URL + "/healthz")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// 2. Readyz when no snapshot is available should return 503 Service Unavailable
	resp, err = http.Get(ts.URL + "/readyz")
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	resp.Body.Close()

	// 3. Update with snapshot -> Readyz should now return 200 OK
	srv.mu.Lock()
	srv.latestSnapshot = sampleSnapshot()
	srv.mu.Unlock()

	resp, err = http.Get(ts.URL + "/ready")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	resp, err = http.Get(ts.URL + "/api/v1/ready")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestPrometheusExporter_OperationalMetrics(t *testing.T) {
	exporter := NewPrometheusExporter()
	snap := sampleSnapshot()
	diag := &model.DiagnosticReport{
		CriticalChecks: 1,
		WarningChecks:  0,
	}

	exporter.Update(snap, diag, nil, nil)
	rendered := exporter.RenderMetrics()

	assert.Contains(t, rendered, "watchdog_build_info{version=\"1.0.0\"")
	assert.Contains(t, rendered, "watchdog_up 1")
	assert.Contains(t, rendered, "watchdog_health_status{status=\"critical\"} 1")
}
