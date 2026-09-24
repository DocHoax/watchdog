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
