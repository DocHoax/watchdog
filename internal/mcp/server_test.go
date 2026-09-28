package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/alerts"
	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/fleet"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

// mockAuditLogger records audit events in memory for test assertions.
type mockAuditLogger struct {
	mu     sync.Mutex
	events []model.AuditEvent
}

func newMockAuditLogger() *mockAuditLogger {
	return &mockAuditLogger{
		events: make([]model.AuditEvent, 0),
	}
}

func (m *mockAuditLogger) Record(ctx context.Context, event model.AuditEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return nil
}

func (m *mockAuditLogger) Close() error {
	return nil
}

func (m *mockAuditLogger) getEvents() []model.AuditEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	cpy := make([]model.AuditEvent, len(m.events))
	copy(cpy, m.events)
	return cpy
}

func setupTestServerWithBurst(t *testing.T, token string, reqPerMin int, burst int) (*Server, *mockAuditLogger, func()) {
	t.Helper()
	store, err := storage.NewSQLiteStorage(storage.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create sqlite: %v", err)
	}

	localID := model.NodeIdentity{
		NodeID:    "test-local-node",
		Hostname:  "test-local-host",
		CreatedAt: time.Now(),
	}

	fleetSvc := fleet.NewFleetService(store, config.FleetConfig{
		RateLimitRate:  100.0,
		RateLimitBurst: 200,
	})

	_, _ = fleetSvc.RegisterNode(context.Background(), &model.NodeRegistrationRequest{
		Identity: localID,
	})

	alertEng := alerts.NewEngine(config.DefaultConfig(), store)
	auditLog := newMockAuditLogger()

	rateRate := 0.0
	if reqPerMin > 0 {
		rateRate = float64(reqPerMin) / 60.0
	}

	cfg := config.MCPConfig{
		Enabled:             true,
		Transport:           "stdio",
		BindAddress:         "127.0.0.1",
		Port:                8444,
		Token:               token,
		RateLimitRate:       rateRate,
		RateLimitBurst:      burst,
		MaxRequestBodyBytes: 1048576,
		ReadTimeout:         5 * time.Second,
		WriteTimeout:        5 * time.Second,
	}

	srv := NewServer(cfg, store, nil, nil, alertEng, fleetSvc, auditLog, localID)

	cleanup := func() {
		_ = store.Close()
	}

	return srv, auditLog, cleanup
}

func setupTestServer(t *testing.T, token string, reqPerMin int) (*Server, *mockAuditLogger, func()) {
	return setupTestServerWithBurst(t, token, reqPerMin, 10)
}

func TestServer_Stdio(t *testing.T) {
	srv, auditLog, cleanup := setupTestServer(t, "", 0)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Prepare input requests
	var inBuf bytes.Buffer
	var outBuf bytes.Buffer

	// 1. Initialize
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"claude-desktop","version":"1.0.0"}}}` + "\n"
	inBuf.WriteString(initReq)

	// 2. Initialized notification
	initializedNotif := `{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n"
	inBuf.WriteString(initializedNotif)

	// 3. Ping
	pingReq := `{"jsonrpc":"2.0","id":2,"method":"ping"}` + "\n"
	inBuf.WriteString(pingReq)

	// 4. List tools
	listToolsReq := `{"jsonrpc":"2.0","id":3,"method":"tools/list"}` + "\n"
	inBuf.WriteString(listToolsReq)

	// 5. Call tool: list_nodes
	callToolReq := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"list_nodes","arguments":{"limit":10}}}` + "\n"
	inBuf.WriteString(callToolReq)

	// 6. List resources
	listResReq := `{"jsonrpc":"2.0","id":5,"method":"resources/list"}` + "\n"
	inBuf.WriteString(listResReq)

	// 7. Read resource
	readResReq := `{"jsonrpc":"2.0","id":6,"method":"resources/read","params":{"uri":"watchdog://fleet"}}` + "\n"
	inBuf.WriteString(readResReq)

	// 8. List prompts
	listPromptsReq := `{"jsonrpc":"2.0","id":7,"method":"prompts/list"}` + "\n"
	inBuf.WriteString(listPromptsReq)

	// 9. Get prompt
	getPromptReq := `{"jsonrpc":"2.0","id":8,"method":"prompts/get","params":{"name":"system_health_audit","arguments":{"severity":"critical"}}}` + "\n"
	inBuf.WriteString(getPromptReq)

	// 10. Invalid JSON-RPC version
	badRPCReq := `{"jsonrpc":"1.0","id":9,"method":"ping"}` + "\n"
	inBuf.WriteString(badRPCReq)

	// 11. Parse error
	parseErrReq := `{"invalid-json` + "\n"
	inBuf.WriteString(parseErrReq)

	// Run stdio loop
	err := srv.ServeStdio(ctx, &inBuf, &outBuf)
	if err != nil && !strings.Contains(err.Error(), "EOF") {
		t.Fatalf("ServeStdio exited with error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(outBuf.String()), "\n")
	if len(lines) < 9 {
		t.Fatalf("expected at least 9 response lines, got %d. Output:\n%s", len(lines), outBuf.String())
	}

	// Verify responses
	// Response 1: Initialize
	var resp1 JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[0]), &resp1); err != nil {
		t.Fatalf("failed to unmarshal resp 1: %v", err)
	}
	if resp1.Error != nil {
		t.Fatalf("init response has error: %v", resp1.Error)
	}
	var initRes InitializeResult
	initResBytes, _ := json.Marshal(resp1.Result)
	_ = json.Unmarshal(initResBytes, &initRes)
	if initRes.ProtocolVersion != ProtocolVersion {
		t.Errorf("expected protocol version %s, got %s", ProtocolVersion, initRes.ProtocolVersion)
	}
	if initRes.ServerInfo.Name != ServerName {
		t.Errorf("expected server name %s, got %s", ServerName, initRes.ServerInfo.Name)
	}

	// Response 2: Ping (response to request ID 2)
	var resp2 JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[1]), &resp2); err != nil {
		t.Fatalf("failed to unmarshal resp 2: %v", err)
	}
	if resp2.Error != nil {
		t.Errorf("ping returned error: %v", resp2.Error)
	}

	// Response 3: List Tools
	var resp3 JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[2]), &resp3); err != nil {
		t.Fatalf("failed to unmarshal resp 3: %v", err)
	}
	var toolsRes ListToolsResult
	toolsBytes, _ := json.Marshal(resp3.Result)
	_ = json.Unmarshal(toolsBytes, &toolsRes)
	if len(toolsRes.Tools) != 23 {
		t.Errorf("expected 23 tools, got %d", len(toolsRes.Tools))
	}

	// Response 4: Call Tool (list_nodes)
	var resp4 JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[3]), &resp4); err != nil {
		t.Fatalf("failed to unmarshal resp 4: %v", err)
	}
	if resp4.Error != nil {
		t.Errorf("call tool returned error: %v", resp4.Error)
	}

	// Response 5: List Resources
	var resp5 JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[4]), &resp5); err != nil {
		t.Fatalf("failed to unmarshal resp 5: %v", err)
	}
	if resp5.Error != nil {
		t.Errorf("list resources returned error: %v", resp5.Error)
	}

	// Response 6: Read Resource
	var resp6 JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[5]), &resp6); err != nil {
		t.Fatalf("failed to unmarshal resp 6: %v", err)
	}
	if resp6.Error != nil {
		t.Errorf("read resource returned error: %v", resp6.Error)
	}

	// Response 7: List Prompts
	var resp7 JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[6]), &resp7); err != nil {
		t.Fatalf("failed to unmarshal resp 7: %v", err)
	}
	if resp7.Error != nil {
		t.Errorf("list prompts returned error: %v", resp7.Error)
	}

	// Response 8: Get Prompt
	var resp8 JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[7]), &resp8); err != nil {
		t.Fatalf("failed to unmarshal resp 8: %v", err)
	}
	if resp8.Error != nil {
		t.Errorf("get prompt returned error: %v", resp8.Error)
	}

	// Response 9: Bad JSON-RPC version
	var resp9 JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[8]), &resp9); err != nil {
		t.Fatalf("failed to unmarshal resp 9: %v", err)
	}
	if resp9.Error == nil || resp9.Error.Code != CodeInvalidRequest {
		t.Errorf("expected CodeInvalidRequest for bad rpc version, got: %v", resp9.Error)
	}

	// Response 10: Parse error
	var resp10 JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[9]), &resp10); err != nil {
		t.Fatalf("failed to unmarshal resp 10: %v", err)
	}
	if resp10.Error == nil || resp10.Error.Code != CodeParseError {
		t.Errorf("expected CodeParseError for bad json, got: %v", resp10.Error)
	}

	// Check audit events recorded
	events := auditLog.getEvents()
	if len(events) < 4 {
		t.Errorf("expected at least 4 audit events, got %d", len(events))
	}
}

func TestServer_HTTP_Auth_And_Limits(t *testing.T) {
	testToken := "secret-auth-token-12345"
	srv, auditLog, cleanup := setupTestServer(t, testToken, 100)
	defer cleanup()

	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", srv.handleHTTPPost)
	mux.HandleFunc("/health", srv.handleHealth)
	mux.HandleFunc("/healthz", srv.handleHealth)
	mux.HandleFunc("/metrics", srv.handleMetrics)

	handler := srv.authAndLimitMiddleware(mux)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	t.Run("health and metrics are unauthenticated", func(t *testing.T) {
		res, err := http.Get(ts.URL + "/health")
		if err != nil {
			t.Fatalf("GET /health failed: %v", err)
		}
		if res.StatusCode != http.StatusOK {
			t.Errorf("expected 200 OK for /health, got %d", res.StatusCode)
		}

		resMetrics, err := http.Get(ts.URL + "/metrics")
		if err != nil {
			t.Fatalf("GET /metrics failed: %v", err)
		}
		if resMetrics.StatusCode != http.StatusOK {
			t.Errorf("expected 200 OK for /metrics, got %d", resMetrics.StatusCode)
		}
	})

	t.Run("missing bearer token returns 401", func(t *testing.T) {
		reqBody := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
		res, err := http.Post(ts.URL+"/mcp", "application/json", strings.NewReader(reqBody))
		if err != nil {
			t.Fatalf("POST /mcp failed: %v", err)
		}
		if res.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", res.StatusCode)
		}
	})

	t.Run("invalid bearer token returns 403", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
		req.Header.Set("Authorization", "Bearer wrong-token")
		req.Header.Set("Content-Type", "application/json")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /mcp failed: %v", err)
		}
		if res.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden, got %d", res.StatusCode)
		}
	})

	t.Run("valid Authorization Bearer token returns 200", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
		req.Header.Set("Authorization", "Bearer "+testToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Request-ID", "req-test-auth")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /mcp failed: %v", err)
		}
		if res.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", res.StatusCode)
		}

		var rpcResp JSONRPCResponse
		if err := json.NewDecoder(res.Body).Decode(&rpcResp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if rpcResp.Error != nil {
			t.Errorf("expected no error in rpc response, got %v", rpcResp.Error)
		}
		if res.Header.Get("X-Request-ID") != "req-test-auth" {
			t.Errorf("expected X-Request-ID echo req-test-auth, got %s", res.Header.Get("X-Request-ID"))
		}
	})

	t.Run("valid X-Watchdog-Token header returns 200", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"ping"}`))
		req.Header.Set("X-Watchdog-Token", testToken)
		req.Header.Set("Content-Type", "application/json")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /mcp failed: %v", err)
		}
		if res.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", res.StatusCode)
		}
	})

	t.Run("method not allowed for GET on /mcp", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/mcp", nil)
		req.Header.Set("Authorization", "Bearer "+testToken)

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /mcp failed: %v", err)
		}
		if res.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("expected 405 Method Not Allowed, got %d", res.StatusCode)
		}
	})

	t.Run("audit log records auth failure without secret leakage", func(t *testing.T) {
		events := auditLog.getEvents()
		var foundFailure bool
		for _, e := range events {
			if e.EventType == model.EventMCPAuthFailure {
				foundFailure = true
				if strings.Contains(e.Actor.Identity, "wrong-token") {
					t.Errorf("audit actor identity contains unmasked token: %s", e.Actor.Identity)
				}
			}
		}
		if !foundFailure {
			t.Errorf("expected at least one MCP auth failure event in audit log")
		}
	})
}

func TestServer_SSE(t *testing.T) {
	srv, _, cleanup := setupTestServer(t, "", 0)
	defer cleanup()

	mux := http.NewServeMux()
	mux.HandleFunc("/sse", srv.handleSSE)
	mux.HandleFunc("/mcp/message", srv.handleSSEMessage)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect to /sse
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/sse", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /sse: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for /sse, got %d", resp.StatusCode)
	}

	// Read initial endpoint event
	buf := make([]byte, 1024)
	n, err := resp.Body.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("failed to read from SSE stream: %v", err)
	}
	sseOutput := string(buf[:n])
	if !strings.Contains(sseOutput, "event: endpoint") || !strings.Contains(sseOutput, "sessionId=") {
		t.Fatalf("expected endpoint event in SSE output, got: %s", sseOutput)
	}

	// Extract session ID
	_, after, found := strings.Cut(sseOutput, "sessionId=")
	if !found {
		t.Fatalf("could not find sessionId in SSE output: %s", sseOutput)
	}
	sessionID := strings.TrimSpace(after)
	if end := strings.IndexAny(sessionID, "\r\n "); end != -1 {
		sessionID = sessionID[:end]
	}

	// Post message to /mcp/message with extracted session ID
	msgURL := fmt.Sprintf("%s/mcp/message?sessionId=%s", ts.URL, sessionID)
	msgBody := `{"jsonrpc":"2.0","id":42,"method":"ping"}`
	postResp, err := http.Post(msgURL, "application/json", strings.NewReader(msgBody))
	if err != nil {
		t.Fatalf("failed to post message to %s: %v", msgURL, err)
	}
	if postResp.StatusCode != http.StatusAccepted {
		t.Errorf("expected 202 Accepted, got %d", postResp.StatusCode)
	}

	// Read response message event from SSE stream
	n2, err := resp.Body.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("failed to read message response from SSE: %v", err)
	}
	msgOutput := string(buf[:n2])
	if !strings.Contains(msgOutput, "event: message") || !strings.Contains(msgOutput, `"id":42`) {
		t.Errorf("expected message response with id 42 on SSE stream, got: %s", msgOutput)
	}
}

func TestServer_RateLimiter(t *testing.T) {
	// Create server with 2 requests per minute limit, burst 2
	srv, _, cleanup := setupTestServerWithBurst(t, "", 2, 2)
	defer cleanup()

	ctx := context.Background()
	mcpCtx := MCPContext{RequestID: "req-rl", ClientID: "flooder-client"}

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "ping",
	}

	// First 2 requests should succeed (burst capacity = 2)
	res1 := srv.HandleRequest(ctx, mcpCtx, req)
	if res1.Error != nil {
		t.Errorf("request 1 unexpectedly failed: %v", res1.Error)
	}

	res2 := srv.HandleRequest(ctx, mcpCtx, req)
	if res2.Error != nil {
		t.Errorf("request 2 unexpectedly failed: %v", res2.Error)
	}

	// Third request should be rate limited
	res3 := srv.HandleRequest(ctx, mcpCtx, req)
	if res3.Error == nil {
		t.Fatalf("expected rate limit error on request 3, got nil error")
	}
	if res3.Error.Code != CodeRateLimited {
		t.Errorf("expected CodeRateLimited (%d), got %d", CodeRateLimited, res3.Error.Code)
	}
}
