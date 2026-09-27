package mcp

import (
	"bufio"
	"context"
	"crypto/subtle"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/DocHoax/watchdog/internal/alerts"
	"github.com/DocHoax/watchdog/internal/audit"
	"github.com/DocHoax/watchdog/internal/collector"
	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/diagnostics"
	"github.com/DocHoax/watchdog/internal/fleet"
	"github.com/DocHoax/watchdog/internal/intelligence"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

// Server implements the Model Context Protocol (MCP) server for Watchdog.
type Server struct {
	cfg           config.MCPConfig
	tools         *ToolRegistry
	resources     *ResourceRegistry
	prompts       *PromptRegistry
	rateLimiter   *ClientRateLimiter
	metrics       *Metrics
	auditLog      audit.AuditLogger
	localIdentity model.NodeIdentity

	mu          sync.RWMutex
	initialized bool
	clientInfo  ClientInfo

	httpServer  *http.Server
	sseMu       sync.RWMutex
	sseSessions map[string]chan []byte
}

// NewServer creates a new MCP Server instance.
func NewServer(
	cfg config.MCPConfig,
	store storage.ReadOnlyStorage,
	col *collector.Manager,
	diagEng *diagnostics.Engine,
	alertEng *alerts.Engine,
	fleetSvc fleet.ReadOnlyFleetService,
	auditLog audit.AuditLogger,
	localID model.NodeIdentity,
	intelSvc ...intelligence.IntelligenceService,
) *Server {
	if auditLog == nil {
		auditLog = audit.NewNopAuditLogger()
	}

	var rateLimiter *ClientRateLimiter
	if cfg.RateLimitRate > 0 {
		rateLimiter = NewClientRateLimiter(int(cfg.RateLimitRate*60), cfg.RateLimitBurst)
	}

	return &Server{
		cfg:           cfg,
		tools:         NewToolRegistry(fleetSvc, store, col, diagEng, alertEng, localID, intelSvc...),
		resources:     NewResourceRegistry(fleetSvc, store, col, diagEng, alertEng, localID, intelSvc...),
		prompts:       NewPromptRegistry(),
		rateLimiter:   rateLimiter,
		metrics:       NewMetrics(),
		auditLog:      auditLog,
		localIdentity: localID,
		sseSessions:   make(map[string]chan []byte),
	}
}

// SetIntelligenceService sets or overrides the intelligence service across tools and resources.
func (s *Server) SetIntelligenceService(svc intelligence.IntelligenceService) {
	if s.tools != nil {
		s.tools.SetIntelligenceService(svc)
	}
	if s.resources != nil {
		s.resources.SetIntelligenceService(svc)
	}
}

// GetMetrics returns the MCP metrics tracker.
func (s *Server) GetMetrics() *Metrics {
	return s.metrics
}

// ServeStdio executes the MCP JSON-RPC protocol loop over standard input/output.
func (s *Server) ServeStdio(ctx context.Context, in io.Reader, out io.Writer) error {
	s.recordAudit(ctx, model.EventMCPServerStart, model.AuditSeverityInfo, model.AuditOutcomeSuccess,
		model.AuditActor{Type: model.ActorTypeCLI, Identity: "stdio-client"},
		model.AuditSource{Transport: "stdio"}, "MCP stdio server started", nil)

	defer func() {
		s.recordAudit(ctx, model.EventMCPServerStop, model.AuditSeverityInfo, model.AuditOutcomeSuccess,
			model.AuditActor{Type: model.ActorTypeCLI, Identity: "stdio-client"},
			model.AuditSource{Transport: "stdio"}, "MCP stdio server stopped", nil)
	}()

	reader := bufio.NewReaderSize(in, 1024*1024) // 1MB buffer for incoming lines
	writer := bufio.NewWriter(out)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.metrics.RecordError("parse", ErrCodeStrInvalidArgument)
			resp := &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      nil,
				Error:   NewParseError(err.Error()),
			}
			data, _ := json.Marshal(resp)
			_, _ = writer.Write(data)
			_, _ = writer.WriteString("\n")
			_ = writer.Flush()
			continue
		}

		mcpCtx := MCPContext{
			RequestID: audit.GenerateEventID(),
			ClientID:  "stdio-client",
			Actor:     "cli",
			Timestamp: time.Now().UTC(),
		}

		// Notifications (requests without an ID) do not yield a JSON-RPC response
		isNotification := req.ID == nil

		resp := s.HandleRequest(ctx, mcpCtx, &req)
		if !isNotification && resp != nil {
			data, err := json.Marshal(resp)
			if err != nil {
				continue
			}
			_, _ = writer.Write(data)
			_, _ = writer.WriteString("\n")
			_ = writer.Flush()
		}
	}
}

// HandleRequest processes a single JSON-RPC request and returns the corresponding response.
func (s *Server) HandleRequest(ctx context.Context, mcpCtx MCPContext, req *JSONRPCRequest) *JSONRPCResponse {
	start := time.Now()
	s.metrics.RecordRequest(req.Method)
	defer func() {
		s.metrics.RecordDuration(req.Method, time.Since(start))
	}()

	// Validate JSON-RPC version
	if req.JSONRPC != "2.0" && req.JSONRPC != "" {
		s.metrics.RecordError(req.Method, ErrCodeStrInvalidArgument)
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   NewInvalidRequestError("jsonrpc must be '2.0'"),
		}
	}

	// Rate limiting check
	if s.rateLimiter != nil && !s.rateLimiter.Allow(mcpCtx.ClientID) {
		s.metrics.RecordError(req.Method, ErrCodeStrRateLimited)
		s.recordAudit(ctx, model.EventMCPRateLimited, model.AuditSeverityWarning, model.AuditOutcomeDenied,
			model.AuditActor{Type: model.ActorTypeAuthenticatedClient, Identity: mcpCtx.ClientID},
			model.AuditSource{RequestID: mcpCtx.RequestID, Transport: "network"},
			fmt.Sprintf("Rate limit exceeded for client '%s'", mcpCtx.ClientID),
			map[string]string{"method": req.Method})

		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   NewRateLimitedError("client rate limit exceeded"),
		}
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(ctx, mcpCtx, req)
	case "notifications/initialized", "initialized":
		return s.handleInitialized(ctx, mcpCtx, req)
	case "ping":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{},
		}
	case "tools/list":
		return s.handleListTools(ctx, mcpCtx, req)
	case "tools/call":
		return s.handleCallTool(ctx, mcpCtx, req)
	case "resources/list":
		return s.handleListResources(ctx, mcpCtx, req)
	case "resources/read":
		return s.handleReadResource(ctx, mcpCtx, req)
	case "prompts/list":
		return s.handleListPrompts(ctx, mcpCtx, req)
	case "prompts/get":
		return s.handleGetPrompt(ctx, mcpCtx, req)
	default:
		s.metrics.RecordError(req.Method, ErrCodeStrInvalidArgument)
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   NewMethodNotFoundError(req.Method),
		}
	}
}

func (s *Server) handleInitialize(ctx context.Context, mcpCtx MCPContext, req *JSONRPCRequest) *JSONRPCResponse {
	var params InitializeParams
	if len(req.Params) > 0 {
		_ = json.Unmarshal(req.Params, &params)
	}

	s.mu.Lock()
	s.initialized = true
	s.clientInfo = params.ClientInfo
	s.mu.Unlock()

	s.recordAudit(ctx, model.EventMCPInitialize, model.AuditSeverityInfo, model.AuditOutcomeSuccess,
		model.AuditActor{Type: model.ActorTypeAuthenticatedClient, Identity: params.ClientInfo.Name},
		model.AuditSource{RequestID: mcpCtx.RequestID},
		fmt.Sprintf("MCP client initialized: %s (%s)", params.ClientInfo.Name, params.ClientInfo.Version),
		map[string]string{
			"client_name":      params.ClientInfo.Name,
			"client_version":   params.ClientInfo.Version,
			"protocol_version": params.ProtocolVersion,
		})

	result := InitializeResult{
		ProtocolVersion: ProtocolVersion,
		Capabilities: ServerCapabilities{
			Tools:     &ToolsCapability{ListChanged: false},
			Resources: &ResourcesCapability{Subscribe: false, ListChanged: false},
			Prompts:   &PromptsCapability{ListChanged: false},
			Logging:   &LoggingCapability{},
		},
		ServerInfo: ServerInfo{
			Name:    ServerName,
			Version: ServerVersion,
		},
		Instructions: "Watchdog is a strictly read-only, high-performance systems observability and diagnostics server.\n" +
			"You can query fleet nodes, inspect health metrics, retrieve telemetry snapshots, and triage active alerts.\n" +
			"All operations are read-only; no state modification, configuration change, or code execution is permitted.",
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (s *Server) handleInitialized(_ context.Context, _ MCPContext, _ *JSONRPCRequest) *JSONRPCResponse {
	s.mu.Lock()
	s.initialized = true
	s.mu.Unlock()
	return nil // Notification, no response needed
}

func (s *Server) handleListTools(_ context.Context, _ MCPContext, req *JSONRPCRequest) *JSONRPCResponse {
	tools := ToolDefinitions()
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  ListToolsResult{Tools: tools},
	}
}

func (s *Server) handleCallTool(ctx context.Context, mcpCtx MCPContext, req *JSONRPCRequest) *JSONRPCResponse {
	var params CallToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.metrics.RecordError("tools/call", ErrCodeStrInvalidArgument)
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   NewInvalidParamsError(fmt.Sprintf("invalid call parameters: %v", err)),
		}
	}

	toolStart := time.Now()
	res, rpcErr := s.tools.Execute(ctx, mcpCtx, params.Name, params.Arguments)
	isErr := rpcErr != nil || (res != nil && res.IsError)
	s.metrics.RecordToolCall(params.Name, isErr)

	outcome := model.AuditOutcomeSuccess
	if isErr {
		outcome = model.AuditOutcomeFailure
	}

	s.recordAudit(ctx, model.EventMCPToolCall, model.AuditSeverityInfo, outcome,
		model.AuditActor{Type: model.ActorTypeAuthenticatedClient, Identity: mcpCtx.ClientID},
		model.AuditSource{RequestID: mcpCtx.RequestID},
		fmt.Sprintf("MCP tool invoked: %s", params.Name),
		map[string]string{
			"tool_name":   params.Name,
			"duration_ms": fmt.Sprintf("%d", time.Since(toolStart).Milliseconds()),
			"is_error":    fmt.Sprintf("%t", isErr),
		})

	if rpcErr != nil {
		s.metrics.RecordError("tools/call", ErrCodeStrInternalError)
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   rpcErr,
		}
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  res,
	}
}

func (s *Server) handleListResources(ctx context.Context, _ MCPContext, req *JSONRPCRequest) *JSONRPCResponse {
	resList, err := s.resources.ListResources(ctx)
	if err != nil {
		s.metrics.RecordError("resources/list", ErrCodeStrInternalError)
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   NewInternalError(err.Error()),
		}
	}
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  ListResourcesResult{Resources: resList},
	}
}

func (s *Server) handleReadResource(ctx context.Context, mcpCtx MCPContext, req *JSONRPCRequest) *JSONRPCResponse {
	var params ReadResourceParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.metrics.RecordError("resources/read", ErrCodeStrInvalidArgument)
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   NewInvalidParamsError(fmt.Sprintf("invalid resource parameters: %v", err)),
		}
	}

	res, rpcErr := s.resources.ReadResource(ctx, mcpCtx, params.URI)
	outcome := model.AuditOutcomeSuccess
	if rpcErr != nil {
		outcome = model.AuditOutcomeFailure
	}

	s.recordAudit(ctx, model.EventMCPResourceRead, model.AuditSeverityInfo, outcome,
		model.AuditActor{Type: model.ActorTypeAuthenticatedClient, Identity: mcpCtx.ClientID},
		model.AuditSource{RequestID: mcpCtx.RequestID},
		fmt.Sprintf("MCP resource read: %s", params.URI),
		map[string]string{"uri": params.URI})

	if rpcErr != nil {
		s.metrics.RecordError("resources/read", ErrCodeStrResourceNotFound)
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   rpcErr,
		}
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  res,
	}
}

func (s *Server) handleListPrompts(_ context.Context, _ MCPContext, req *JSONRPCRequest) *JSONRPCResponse {
	prompts := s.prompts.ListPrompts()
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  ListPromptsResult{Prompts: prompts},
	}
}

func (s *Server) handleGetPrompt(ctx context.Context, mcpCtx MCPContext, req *JSONRPCRequest) *JSONRPCResponse {
	var params GetPromptParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.metrics.RecordError("prompts/get", ErrCodeStrInvalidArgument)
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   NewInvalidParamsError(fmt.Sprintf("invalid prompt parameters: %v", err)),
		}
	}

	res, rpcErr := s.prompts.GetPrompt(params.Name, params.Arguments)
	outcome := model.AuditOutcomeSuccess
	if rpcErr != nil {
		outcome = model.AuditOutcomeFailure
	}

	s.recordAudit(ctx, model.EventMCPPromptGet, model.AuditSeverityInfo, outcome,
		model.AuditActor{Type: model.ActorTypeAuthenticatedClient, Identity: mcpCtx.ClientID},
		model.AuditSource{RequestID: mcpCtx.RequestID},
		fmt.Sprintf("MCP prompt requested: %s", params.Name),
		map[string]string{"prompt_name": params.Name})

	if rpcErr != nil {
		s.metrics.RecordError("prompts/get", ErrCodeStrInvalidArgument)
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   rpcErr,
		}
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  res,
	}
}

// Handler returns the configured http.Handler for the MCP server.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", s.handleHTTPPost)
	mux.HandleFunc("/sse", s.handleSSE)
	mux.HandleFunc("/mcp/message", s.handleSSEMessage)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/metrics", s.handleMetrics)
	return s.authAndLimitMiddleware(mux)
}

// ServeHTTP implements http.Handler for the MCP server.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.Handler().ServeHTTP(w, r)
}

// StartHTTP starts the network transport HTTP/SSE listener.
func (s *Server) StartHTTP(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.BindAddress, s.cfg.Port)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.Handler(),
		ReadTimeout:  s.cfg.ReadTimeout,
		WriteTimeout: s.cfg.WriteTimeout,
	}

	if s.httpServer.ReadTimeout <= 0 {
		s.httpServer.ReadTimeout = 15 * time.Second
	}
	if s.httpServer.WriteTimeout <= 0 {
		s.httpServer.WriteTimeout = 15 * time.Second
	}

	s.recordAudit(ctx, model.EventMCPServerStart, model.AuditSeverityInfo, model.AuditOutcomeSuccess,
		model.AuditActor{Type: model.ActorTypeSystem, Identity: "watchdog-mcp"},
		model.AuditSource{Address: addr, Transport: "http"},
		fmt.Sprintf("MCP HTTP server starting on %s", addr),
		map[string]string{
			"bind_address": s.cfg.BindAddress,
			"port":         fmt.Sprintf("%d", s.cfg.Port),
			"transport":    s.cfg.Transport,
			"tls_enabled":  fmt.Sprintf("%t", s.cfg.TLSCert != ""),
		})

	errChan := make(chan error, 1)
	go func() {
		if s.cfg.TLSCert != "" && s.cfg.TLSKey != "" {
			cert, err := tls.X509KeyPair([]byte(s.cfg.TLSCert), []byte(s.cfg.TLSKey))
			if err != nil {
				errChan <- fmt.Errorf("failed to parse MCP TLS certificates: %w", err)
				return
			}
			s.httpServer.TLSConfig = &tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion:   tls.VersionTLS12,
			}
			errChan <- s.httpServer.ListenAndServeTLS("", "")
		} else {
			errChan <- s.httpServer.ListenAndServe()
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.httpServer.Shutdown(shutdownCtx)
		s.recordAudit(context.Background(), model.EventMCPServerStop, model.AuditSeverityInfo, model.AuditOutcomeSuccess,
			model.AuditActor{Type: model.ActorTypeSystem, Identity: "watchdog-mcp"},
			model.AuditSource{Address: addr, Transport: "http"},
			"MCP HTTP server shutdown cleanly", nil)
		return nil
	case err := <-errChan:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}

// authAndLimitMiddleware validates bearer token authentication and body limits.
func (s *Server) authAndLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow unauthenticated health & metrics
		if r.URL.Path == "/health" || r.URL.Path == "/healthz" || r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = audit.GenerateEventID()
		}
		w.Header().Set("X-Request-ID", reqID)

		// Token verification
		if s.cfg.Token != "" {
			authHeader := r.Header.Get("Authorization")
			customHeader := r.Header.Get("X-Watchdog-Token")

			provided := ""
			if len(authHeader) > 7 && strings.EqualFold(authHeader[:7], "Bearer ") {
				provided = strings.TrimSpace(authHeader[7:])
			} else if customHeader != "" {
				provided = strings.TrimSpace(customHeader)
			}

			if provided == "" {
				s.recordAudit(r.Context(), model.EventMCPAuthFailure, model.AuditSeverityWarning, model.AuditOutcomeDenied,
					model.AuditActor{Type: model.ActorTypeAnonymousClient, Identity: "anonymous"},
					model.AuditSource{Address: r.RemoteAddr, RequestID: reqID, Endpoint: r.URL.Path, Method: r.Method},
					"MCP authentication missing token", nil)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(NewJSONRPCError(CodeUnauthorized, "Unauthorized: missing bearer token", nil))
				return
			}

			if subtle.ConstantTimeCompare([]byte(provided), []byte(s.cfg.Token)) != 1 {
				s.recordAudit(r.Context(), model.EventMCPAuthFailure, model.AuditSeverityWarning, model.AuditOutcomeDenied,
					model.AuditActor{Type: model.ActorTypeAnonymousClient, Identity: audit.MaskToken(provided)},
					model.AuditSource{Address: r.RemoteAddr, RequestID: reqID, Endpoint: r.URL.Path, Method: r.Method},
					"MCP authentication invalid token", nil)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(NewJSONRPCError(CodeForbidden, "Forbidden: invalid authentication token", nil))
				return
			}
		}

		// Enforce body size limit
		maxBytes := s.cfg.MaxRequestBodyBytes
		if maxBytes <= 0 {
			maxBytes = 1048576 // 1MB default
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHTTPPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(&JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   NewParseError("failed to read request body"),
		})
		return
	}

	var req JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(&JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   NewParseError(err.Error()),
		})
		return
	}

	clientID := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		clientID = host
	}

	mcpCtx := MCPContext{
		RequestID: r.Header.Get("X-Request-ID"),
		ClientID:  clientID,
		Actor:     "network_client",
		Timestamp: time.Now().UTC(),
	}

	resp := s.HandleRequest(r.Context(), mcpCtx, &req)
	w.Header().Set("Content-Type", "application/json")
	if resp == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	sessionID := audit.GenerateEventID()
	msgChan := make(chan []byte, 32)

	s.sseMu.Lock()
	s.sseSessions[sessionID] = msgChan
	s.sseMu.Unlock()

	defer func() {
		s.sseMu.Lock()
		delete(s.sseSessions, sessionID)
		close(msgChan)
		s.sseMu.Unlock()
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Send endpoint event with session ID per MCP SSE specification
	endpointURL := fmt.Sprintf("/mcp/message?sessionId=%s", sessionID)
	_, _ = fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpointURL)
	flusher.Flush()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case msg, ok := <-msgChan:
			if !ok {
				return
			}
			_, _ = fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(msg))
			flusher.Flush()
		}
	}
}

func (s *Server) handleSSEMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		http.Error(w, "Missing sessionId query parameter", http.StatusBadRequest)
		return
	}

	s.sseMu.RLock()
	msgChan, exists := s.sseSessions[sessionID]
	s.sseMu.RUnlock()

	if !exists {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	var req JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON-RPC payload", http.StatusBadRequest)
		return
	}

	mcpCtx := MCPContext{
		RequestID: r.Header.Get("X-Request-ID"),
		ClientID:  sessionID,
		Actor:     "sse_client",
		Timestamp: time.Now().UTC(),
	}

	resp := s.HandleRequest(r.Context(), mcpCtx, &req)
	if resp != nil {
		respBytes, err := json.Marshal(resp)
		if err == nil {
			select {
			case msgChan <- respBytes:
			default:
			}
		}
	}

	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":    "ok",
		"service":   "mcp",
		"version":   ServerVersion,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(s.metrics.RenderPrometheus()))
}

func (s *Server) recordAudit(
	ctx context.Context,
	eventType string,
	severity string,
	outcome string,
	actor model.AuditActor,
	src model.AuditSource,
	msg string,
	metadata map[string]string,
) {
	if s.auditLog == nil {
		return
	}
	_ = s.auditLog.Record(ctx, model.AuditEvent{
		ID:        audit.GenerateEventID(),
		Timestamp: time.Now().UTC(),
		EventType: eventType,
		Severity:  severity,
		Outcome:   outcome,
		Actor:     actor,
		Source:    src,
		Message:   msg,
		Metadata:  metadata,
	})
}
