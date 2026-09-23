package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/watchdog-cli/watchdog/internal/alerts"
	"github.com/watchdog-cli/watchdog/internal/anomaly"
	"github.com/watchdog-cli/watchdog/internal/collector"
	"github.com/watchdog-cli/watchdog/internal/config"
	"github.com/watchdog-cli/watchdog/internal/diagnostics"
	"github.com/watchdog-cli/watchdog/internal/logger"
	"github.com/watchdog-cli/watchdog/internal/storage"
	"github.com/watchdog-cli/watchdog/pkg/model"
)

// Server represents the Watchdog HTTP API server / remote agent.
type Server struct {
	cfg       *config.Config
	collector *collector.Manager
	storage   storage.Storage
	diagEng   *diagnostics.Engine
	alertEng  *alerts.Engine
	anomDet   *anomaly.Detector
	exporter  *PrometheusExporter

	httpServer *http.Server
	startTime  time.Time

	mu             sync.RWMutex
	latestSnapshot *model.SystemSnapshot
	latestDiag     *model.DiagnosticReport
	activeAlerts   []model.AlertEvent
	latestAnomaly  *model.AnomalyReport
}

// NewServer initializes a new Server.
func NewServer(
	cfg *config.Config,
	col *collector.Manager,
	store storage.Storage,
	diagEng *diagnostics.Engine,
	alertEng *alerts.Engine,
	anomDet *anomaly.Detector,
) *Server {
	return &Server{
		cfg:       cfg,
		collector: col,
		storage:   store,
		diagEng:   diagEng,
		alertEng:  alertEng,
		anomDet:   anomDet,
		exporter:  NewPrometheusExporter(),
		startTime: time.Now(),
	}
}

// Start runs the HTTP server and background collection worker until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// Register routes
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/v1/health", s.handleHealth)
	mux.HandleFunc("/metrics", s.exporter.Handler())

	// Secured API routes
	mux.Handle("/api/v1/snapshot", s.authMiddleware(http.HandlerFunc(s.handleSnapshot)))
	mux.Handle("/api/v1/diagnostics", s.authMiddleware(http.HandlerFunc(s.handleDiagnostics)))
	mux.Handle("/api/v1/alerts", s.authMiddleware(http.HandlerFunc(s.handleAlerts)))
	mux.Handle("/api/v1/anomalies", s.authMiddleware(http.HandlerFunc(s.handleAnomalies)))

	bindAddr := s.cfg.Agent.BindAddress
	if bindAddr == "" {
		bindAddr = "0.0.0.0"
	}
	port := s.cfg.Agent.Port
	if port == 0 {
		port = 8443
	}

	addr := fmt.Sprintf("%s:%d", bindAddr, port)
	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Start background collection loop
	go s.runCollectionLoop(ctx)

	// Channel for server errors
	errChan := make(chan error, 1)

	go func() {
		logger.Infof("Watchdog API / Agent server listening on %s (TLS: %v)", addr, s.cfg.Agent.TLSCert != "")
		if s.cfg.Agent.TLSCert != "" && s.cfg.Agent.TLSKey != "" {
			if err := s.httpServer.ListenAndServeTLS(s.cfg.Agent.TLSCert, s.cfg.Agent.TLSKey); err != nil && err != http.ErrServerClosed {
				errChan <- err
			}
		} else {
			if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				errChan <- err
			}
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		logger.Infof("Shutting down Watchdog server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(shutdownCtx)
	case err := <-errChan:
		return err
	}
}

// runCollectionLoop periodically collects system metrics, evaluates alerts, and updates the exporter.
func (s *Server) runCollectionLoop(ctx context.Context) {
	interval := s.cfg.RefreshInterval
	if interval < 500*time.Millisecond {
		interval = 1 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial collection
	s.collectAndEvaluate(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.collectAndEvaluate(ctx)
		}
	}
}

func (s *Server) collectAndEvaluate(ctx context.Context) {
	if s.collector == nil {
		return
	}

	snap, err := s.collector.CollectAll(ctx)
	if err != nil {
		logger.Warnf("Collector error in server loop: %v", err)
		return
	}

	var diag *model.DiagnosticReport
	if s.diagEng != nil {
		diag, _ = s.diagEng.Run(ctx, snap)
	}

	var activeAlerts []model.AlertEvent
	if s.alertEng != nil {
		_, _, _ = s.alertEng.Evaluate(ctx, snap)
		activeAlerts = s.alertEng.GetActiveAlerts()
	}

	var anom *model.AnomalyReport
	if s.anomDet != nil {
		anom = s.anomDet.FeedSnapshot(snap)
	}

	// Update storage if configured
	if s.storage != nil {
		_ = s.storage.SaveSnapshot(ctx, snap)
	}

	// Update in-memory state
	s.mu.Lock()
	s.latestSnapshot = snap
	s.latestDiag = diag
	s.activeAlerts = activeAlerts
	s.latestAnomaly = anom
	s.mu.Unlock()

	// Update Prometheus exporter
	s.exporter.Update(snap, diag, activeAlerts, anom)
}

// authMiddleware enforces bearer token authentication if configured.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := s.cfg.Agent.Token
		if token != "" {
			authHeader := r.Header.Get("Authorization")
			customHeader := r.Header.Get("X-Watchdog-Token")

			provided := ""
			if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
				provided = authHeader[7:]
			} else if customHeader != "" {
				provided = customHeader
			}

			if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
				s.writeJSONError(w, http.StatusUnauthorized, "Unauthorized: invalid or missing authentication token")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		s.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	uptime := time.Since(s.startTime).Seconds()
	resp := map[string]interface{}{
		"status":         "ok",
		"uptime_seconds": uptime,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"version":        "1.0.0",
	}
	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		s.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	s.mu.RLock()
	snap := s.latestSnapshot
	s.mu.RUnlock()

	if snap == nil {
		s.writeJSONError(w, http.StatusServiceUnavailable, "System metrics snapshot not yet available")
		return
	}

	s.writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		s.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	s.mu.RLock()
	diag := s.latestDiag
	s.mu.RUnlock()

	if diag == nil {
		s.writeJSONError(w, http.StatusServiceUnavailable, "Diagnostic report not yet available")
		return
	}

	s.writeJSON(w, http.StatusOK, diag)
}

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		s.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	s.mu.RLock()
	alerts := s.activeAlerts
	s.mu.RUnlock()

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"active_alerts": alerts,
		"count":         len(alerts),
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleAnomalies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		s.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	s.mu.RLock()
	anom := s.latestAnomaly
	s.mu.RUnlock()

	if anom == nil {
		s.writeJSONError(w, http.StatusServiceUnavailable, "Anomaly detection report not yet available")
		return
	}

	s.writeJSON(w, http.StatusOK, anom)
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) writeJSONError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, map[string]string{
		"error": message,
	})
}
