package server

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServerWithAuth(token string) (*Server, *httptest.Server) {
	cfg := config.DefaultConfig()
	cfg.Agent.Token = token

	srv := NewServer(cfg, nil, nil, nil, nil, nil)
	srv.latestSnapshot = sampleSnapshot()
	srv.latestDiag = &model.DiagnosticReport{
		TotalChecks:  10,
		PassedChecks: 10,
	}
	srv.activeAlerts = []model.AlertEvent{
		{RuleName: "HighMemory", Severity: model.SeverityCritical},
	}
	srv.latestAnomaly = &model.AnomalyReport{
		TotalChecked: 5,
	}
	srv.exporter.Update(srv.latestSnapshot, srv.latestDiag, srv.activeAlerts, srv.latestAnomaly)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", srv.handleHealth)
	mux.HandleFunc("/api/v1/health", srv.handleHealth)
	mux.HandleFunc("/metrics", srv.exporter.Handler())
	mux.Handle("/api/v1/snapshot", srv.authMiddleware(http.HandlerFunc(srv.handleSnapshot)))
	mux.Handle("/api/v1/diagnostics", srv.authMiddleware(http.HandlerFunc(srv.handleDiagnostics)))
	mux.Handle("/api/v1/alerts", srv.authMiddleware(http.HandlerFunc(srv.handleAlerts)))
	mux.Handle("/api/v1/anomalies", srv.authMiddleware(http.HandlerFunc(srv.handleAnomalies)))

	ts := httptest.NewServer(mux)
	return srv, ts
}

// ---------------------------------------------------------------------------
// IsLoopback tests
// ---------------------------------------------------------------------------

func TestIsLoopback(t *testing.T) {
	tests := []struct {
		addr     string
		expected bool
	}{
		{"", true},
		{"127.0.0.1", true},
		{"localhost", true},
		{"LOCALHOST", true},
		{"Localhost", true},
		{"::1", true},
		{"0.0.0.0", false},
		{"192.168.1.1", false},
		{"10.0.0.1", false},
		{"::", false},
		{"example.com", false},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("addr=%q", tc.addr), func(t *testing.T) {
			got := IsLoopback(tc.addr)
			assert.Equal(t, tc.expected, got, "IsLoopback(%q)", tc.addr)
		})
	}
}

// ---------------------------------------------------------------------------
// ValidateServerSecurity tests
// ---------------------------------------------------------------------------

func TestValidateServerSecurity_EmptyBindAddress_DefaultsToLoopback(t *testing.T) {
	cfg := &config.AgentConfig{BindAddress: ""}
	err := ValidateServerSecurity(cfg)
	assert.NoError(t, err, "empty bind address should default to loopback and be allowed")
}

func TestValidateServerSecurity_LoopbackAllowedWithoutTokenOrTLS(t *testing.T) {
	loopbackAddrs := []string{"127.0.0.1", "::1", "localhost"}

	for _, addr := range loopbackAddrs {
		t.Run(addr, func(t *testing.T) {
			cfg := &config.AgentConfig{BindAddress: addr}
			err := ValidateServerSecurity(cfg)
			assert.NoError(t, err, "%s should be allowed without token or TLS", addr)
		})
	}
}

func TestValidateServerSecurity_ExternalWithoutToken_Rejected(t *testing.T) {
	cfg := &config.AgentConfig{
		BindAddress: "0.0.0.0",
		Token:       "",
	}
	err := ValidateServerSecurity(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication")
	assert.Contains(t, err.Error(), "0.0.0.0")
}

func TestValidateServerSecurity_ExternalWithTokenButNoTLS_Rejected(t *testing.T) {
	cfg := &config.AgentConfig{
		BindAddress: "0.0.0.0",
		Token:       "test-token-value",
	}
	err := ValidateServerSecurity(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TLS")
	assert.Contains(t, err.Error(), "0.0.0.0")
}

func TestValidateServerSecurity_ExternalWithTLSButNoToken_Rejected(t *testing.T) {
	cfg := &config.AgentConfig{
		BindAddress: "0.0.0.0",
		Token:       "",
		TLSCert:     "/path/to/cert.pem",
		TLSKey:      "/path/to/key.pem",
	}
	err := ValidateServerSecurity(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication")
}

func TestValidateServerSecurity_ExternalWithTokenAndTLS_Accepted(t *testing.T) {
	cfg := &config.AgentConfig{
		BindAddress: "0.0.0.0",
		Token:       "test-token-value",
		TLSCert:     "/path/to/cert.pem",
		TLSKey:      "/path/to/key.pem",
	}
	err := ValidateServerSecurity(cfg)
	assert.NoError(t, err, "external with token + TLS should be accepted")
}

func TestValidateServerSecurity_IncompleteTLS_CertOnly_Rejected(t *testing.T) {
	cfg := &config.AgentConfig{
		BindAddress: "127.0.0.1",
		TLSCert:     "/path/to/cert.pem",
		TLSKey:      "",
	}
	err := ValidateServerSecurity(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tls_key")
	assert.Contains(t, err.Error(), "incomplete")
}

func TestValidateServerSecurity_IncompleteTLS_KeyOnly_Rejected(t *testing.T) {
	cfg := &config.AgentConfig{
		BindAddress: "127.0.0.1",
		TLSCert:     "",
		TLSKey:      "/path/to/key.pem",
	}
	err := ValidateServerSecurity(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tls_cert")
	assert.Contains(t, err.Error(), "incomplete")
}

func TestValidateServerSecurity_IncompleteTLS_ExternalBind_Rejected(t *testing.T) {
	cfg := &config.AgentConfig{
		BindAddress: "0.0.0.0",
		Token:       "test-token-value",
		TLSCert:     "/path/to/cert.pem",
		TLSKey:      "",
	}
	err := ValidateServerSecurity(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "incomplete")
}

func TestValidateServerSecurity_LoopbackWithTokenAndTLS_Accepted(t *testing.T) {
	cfg := &config.AgentConfig{
		BindAddress: "127.0.0.1",
		Token:       "some-token",
		TLSCert:     "/path/to/cert.pem",
		TLSKey:      "/path/to/key.pem",
	}
	err := ValidateServerSecurity(cfg)
	assert.NoError(t, err, "loopback with optional token+TLS should be accepted")
}

func TestValidateServerSecurity_PrivateIPWithoutToken_Rejected(t *testing.T) {
	cfg := &config.AgentConfig{
		BindAddress: "192.168.1.100",
		Token:       "",
	}
	err := ValidateServerSecurity(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication")
}

// ---------------------------------------------------------------------------
// Server.Start security integration tests
// ---------------------------------------------------------------------------

func TestServer_Start_EmptyBindFallsBackToLoopback(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agent.BindAddress = ""
	cfg.Agent.Port = 0 // will use 8443 default

	srv := NewServer(cfg, nil, nil, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())

	errChan := make(chan error, 1)
	go func() {
		errChan <- srv.Start(ctx)
	}()

	// Let the server attempt to start
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-errChan:
		// Should succeed (port may be in use, but should not fail on security check)
		if err != nil {
			assert.NotContains(t, err.Error(), "security check failed",
				"empty bind address should fall back to 127.0.0.1, not fail security")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server failed to respond to shutdown within timeout")
	}
}

func TestServer_Start_ExternalWithoutAuth_FailsFast(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agent.BindAddress = "0.0.0.0"
	cfg.Agent.Port = 19876
	cfg.Agent.Token = ""

	srv := NewServer(cfg, nil, nil, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := srv.Start(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "security check failed")
	assert.Contains(t, err.Error(), "authentication")
}

func TestServer_Start_ExternalTokenNoTLS_FailsFast(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agent.BindAddress = "0.0.0.0"
	cfg.Agent.Port = 19877
	cfg.Agent.Token = "test-only-token"

	srv := NewServer(cfg, nil, nil, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := srv.Start(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "security check failed")
	assert.Contains(t, err.Error(), "TLS")
}

// ---------------------------------------------------------------------------
// HTTP-level auth enforcement tests
// ---------------------------------------------------------------------------

func TestSecurity_UnauthenticatedRequests(t *testing.T) {
	_, ts := setupTestServerWithAuth("test-secure-token-9988")
	defer ts.Close()

	endpoints := []string{
		"/api/v1/snapshot",
		"/api/v1/diagnostics",
		"/api/v1/alerts",
		"/api/v1/anomalies",
	}

	for _, ep := range endpoints {
		t.Run("Unauthenticated_"+ep, func(t *testing.T) {
			resp, err := http.Get(ts.URL + ep)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}
}

func TestSecurity_AuthenticatedRequests_Succeed(t *testing.T) {
	token := "test-auth-token-for-api"
	_, ts := setupTestServerWithAuth(token)
	defer ts.Close()

	endpoints := []string{
		"/api/v1/snapshot",
		"/api/v1/diagnostics",
		"/api/v1/alerts",
		"/api/v1/anomalies",
	}

	for _, ep := range endpoints {
		t.Run("Authenticated_"+ep, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, ts.URL+ep, nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer "+token)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}

func TestSecurity_InvalidTokens(t *testing.T) {
	_, ts := setupTestServerWithAuth("production-super-secret-key-321")
	defer ts.Close()

	badTokens := []string{
		"production-super-secret-key-320", // 1 char diff
		"Bearer ",
		"admin",
		"null",
		"production-super-secret-key-321-extra",
		"",
	}

	for _, token := range badTokens {
		t.Run("BadToken_"+token, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/snapshot", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer "+token)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}
}

func TestSecurity_CustomHeaderAuth(t *testing.T) {
	_, ts := setupTestServerWithAuth("auth-via-header-xyz")
	defer ts.Close()

	// 1. Using X-Watchdog-Token header
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/snapshot", nil)
	require.NoError(t, err)
	req.Header.Set("X-Watchdog-Token", "auth-via-header-xyz")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestSecurity_UnsupportedMethods(t *testing.T) {
	_, ts := setupTestServerWithAuth("valid-token")
	defer ts.Close()

	endpoints := []string{
		"/health",
		"/api/v1/snapshot",
		"/api/v1/diagnostics",
		"/api/v1/alerts",
		"/api/v1/anomalies",
	}

	disallowedMethods := []string{
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
	}

	for _, ep := range endpoints {
		for _, method := range disallowedMethods {
			t.Run(fmt.Sprintf("%s_%s", method, ep), func(t *testing.T) {
				req, err := http.NewRequest(method, ts.URL+ep, bytes.NewBufferString(`{"payload":"test"}`))
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer valid-token")

				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
				assert.NotEmpty(t, resp.Header.Get("Allow"))
			})
		}
	}
}

func TestSecurity_UnknownEndpoints(t *testing.T) {
	_, ts := setupTestServerWithAuth("valid-token")
	defer ts.Close()

	unknownPaths := []string{
		"/unknown",
		"/api/v2/snapshot",
		"/admin",
		"/etc/passwd",
		"/../../etc/passwd",
	}

	for _, path := range unknownPaths {
		t.Run("404_"+path, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, ts.URL+path, nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer valid-token")

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})
	}
}

func TestSecurity_ConcurrentRequests(t *testing.T) {
	_, ts := setupTestServerWithAuth("concurrent-secret-token")
	defer ts.Close()

	var wg sync.WaitGroup
	concurrentClients := 30
	requestsPerClient := 10

	for i := 0; i < concurrentClients; i++ {
		wg.Add(1)
		go func(clientIdx int) {
			defer wg.Done()
			for j := 0; j < requestsPerClient; j++ {
				// Mix valid and invalid calls
				token := "concurrent-secret-token"
				expectedStatus := http.StatusOK
				if (clientIdx+j)%3 == 0 {
					token = "bad-token"
					expectedStatus = http.StatusUnauthorized
				}

				req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/snapshot", nil)
				if err != nil {
					t.Errorf("request creation error: %v", err)
					return
				}
				req.Header.Set("Authorization", "Bearer "+token)

				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Errorf("request error: %v", err)
					return
				}
				resp.Body.Close()

				if resp.StatusCode != expectedStatus {
					t.Errorf("expected status %d, got %d", expectedStatus, resp.StatusCode)
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestSecurity_ServerGracefulShutdown(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agent.Port = 18999
	cfg.Agent.BindAddress = "127.0.0.1"

	srv := NewServer(cfg, nil, nil, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())

	errChan := make(chan error, 1)
	go func() {
		errChan <- srv.Start(ctx)
	}()

	// Allow server to spin up
	time.Sleep(100 * time.Millisecond)

	// Initiate graceful shutdown
	cancel()

	select {
	case err := <-errChan:
		assert.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("server failed to shutdown within timeout")
	}
}

func TestSecurity_HealthEndpoint_Unauthenticated(t *testing.T) {
	_, ts := setupTestServerWithAuth("some-token")
	defer ts.Close()

	// /health should be accessible without authentication
	resp, err := http.Get(ts.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestSecurity_MetricsEndpoint_Unauthenticated(t *testing.T) {
	_, ts := setupTestServerWithAuth("some-token")
	defer ts.Close()

	// /metrics (Prometheus) should be accessible without authentication
	resp, err := http.Get(ts.URL + "/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/plain")
}
