package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/internal/fleet"
	"github.com/DocHoax/watchdog/pkg/model"
)

// handleNodeIdentity handles GET /api/v1/node requests, returning local node identity and capabilities.
func (s *Server) handleNodeIdentity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	var nodeID string
	var tags map[string]string
	if s.cfg != nil {
		nodeID = s.cfg.Fleet.NodeID
		if nodeID == "" && s.cfg.Fleet.NodeIDFile != "" {
			nodeID, _ = fleet.GetOrGenerateNodeID(s.cfg.Fleet.NodeIDFile)
		}
		if len(s.cfg.Fleet.Tags) > 0 {
			tags = s.cfg.Fleet.Tags
		}
	}

	identity, err := fleet.DiscoverNodeIdentity(r.Context(), nodeID, "1.0.0", tags)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, "IDENTITY_ERROR", fmt.Sprintf("Failed to discover node identity: %v", err))
		return
	}

	s.writeJSON(w, http.StatusOK, identity)
}

// handleHeartbeat handles POST /api/v1/heartbeat requests.
func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	if s.fleetService == nil {
		s.writeAPIError(w, r, http.StatusServiceUnavailable, "FLEET_SERVICE_UNAVAILABLE", "Fleet service is not configured")
		return
	}

	var req model.HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST", fmt.Sprintf("Invalid heartbeat payload: %v", err))
		return
	}

	if req.NodeID == "" {
		req.NodeID = r.Header.Get("X-Watchdog-Node-ID")
	}
	if req.NodeID == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, "MISSING_NODE_ID", "Node ID is required")
		return
	}

	resp, err := s.fleetService.ProcessHeartbeat(r.Context(), &req)
	if err != nil {
		if err == fleet.ErrRateLimitExceeded {
			s.writeAPIError(w, r, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Heartbeat rate limit exceeded")
			return
		}
		s.writeAPIError(w, r, http.StatusInternalServerError, "HEARTBEAT_PROCESSING_FAILED", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, resp)
}

// handleTelemetry handles POST /api/v1/telemetry requests.
func (s *Server) handleTelemetry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	if s.fleetService == nil {
		s.writeAPIError(w, r, http.StatusServiceUnavailable, "FLEET_SERVICE_UNAVAILABLE", "Fleet service is not configured")
		return
	}

	var sub model.TelemetrySubmission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST", fmt.Sprintf("Invalid telemetry payload: %v", err))
		return
	}

	if sub.NodeID == "" {
		sub.NodeID = r.Header.Get("X-Watchdog-Node-ID")
	}
	if sub.NodeID == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, "MISSING_NODE_ID", "Node ID is required")
		return
	}

	if err := s.fleetService.IngestTelemetry(r.Context(), &sub); err != nil {
		if err == fleet.ErrRateLimitExceeded {
			s.writeAPIError(w, r, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Telemetry ingestion rate limit exceeded")
			return
		}
		s.writeAPIError(w, r, http.StatusInternalServerError, "INGESTION_FAILED", err.Error())
		return
	}

	if s.topoService != nil && sub.Snapshot != nil {
		s.topoService.IngestSnapshot(sub.Snapshot, sub.NodeID)
	}

	resp := model.TelemetryResponse{
		Accepted:    true,
		IngestedAt:  time.Now().UTC(),
		PointsCount: 1,
		Message:     "Telemetry accepted",
	}
	s.writeJSON(w, http.StatusOK, resp)
}

// handleFleetRoute multiplexes /api/v1/fleet and /api/v1/fleet/* requests.
func (s *Server) handleFleetRoute(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/fleet")
	path = strings.TrimPrefix(path, "/")

	if path == "" {
		// /api/v1/fleet
		switch r.Method {
		case http.MethodGet, http.MethodHead:
			s.handleFleetList(w, r)
		case http.MethodPost:
			s.handleFleetRegister(w, r)
		default:
			w.Header().Set("Allow", "GET, HEAD, POST")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		}
		return
	}

	if path == "register" {
		if r.Method == http.MethodPost {
			s.handleFleetRegister(w, r)
		} else {
			w.Header().Set("Allow", "POST")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		}
		return
	}

	if path == "summary" {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			s.handleFleetSummary(w, r)
		} else {
			w.Header().Set("Allow", "GET, HEAD")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		}
		return
	}

	// Node-specific routes: /api/v1/fleet/{node_id}
	nodeID := path
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		s.handleFleetGetNode(w, r, nodeID)
	case http.MethodDelete:
		s.handleFleetDeleteNode(w, r, nodeID)
	default:
		w.Header().Set("Allow", "GET, HEAD, DELETE")
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
	}
}

// handleFleetRegister handles node registration requests.
func (s *Server) handleFleetRegister(w http.ResponseWriter, r *http.Request) {
	if s.fleetService == nil {
		s.writeAPIError(w, r, http.StatusServiceUnavailable, "FLEET_SERVICE_UNAVAILABLE", "Fleet service is not configured")
		return
	}

	var req model.NodeRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST", fmt.Sprintf("Invalid registration payload: %v", err))
		return
	}

	if req.Identity.NodeID == "" {
		req.Identity.NodeID = r.Header.Get("X-Watchdog-Node-ID")
	}

	resp, err := s.fleetService.RegisterNode(r.Context(), &req)
	if err != nil {
		if err == fleet.ErrRateLimitExceeded {
			s.writeAPIError(w, r, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Registration rate limit exceeded")
			return
		}
		s.writeAPIError(w, r, http.StatusBadRequest, "REGISTRATION_FAILED", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, resp)
}

// handleFleetList handles listing fleet nodes with filters.
func (s *Server) handleFleetList(w http.ResponseWriter, r *http.Request) {
	if s.fleetService == nil {
		s.writeAPIError(w, r, http.StatusServiceUnavailable, "FLEET_SERVICE_UNAVAILABLE", "Fleet service is not configured")
		return
	}

	q := r.URL.Query()
	var filter model.FleetFilter

	if statusStr := strings.TrimSpace(q.Get("status")); statusStr != "" {
		filter.Status = model.NodeStatus(statusStr)
	}
	filter.Search = strings.TrimSpace(q.Get("search"))
	filter.SortBy = strings.TrimSpace(q.Get("sort_by"))
	filter.SortDirection = strings.TrimSpace(q.Get("sort_direction"))

	if limitStr := strings.TrimSpace(q.Get("limit")); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			filter.Limit = limit
		}
	}
	if offsetStr := strings.TrimSpace(q.Get("offset")); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}

	if sinceStr := strings.TrimSpace(q.Get("since")); sinceStr != "" {
		if t, err := parseTimeOrDuration(sinceStr); err == nil {
			filter.Since = t
		}
	}

	resp, err := s.fleetService.ListNodes(r.Context(), filter)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, "LIST_NODES_FAILED", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, resp)
}

// handleFleetSummary handles GET /api/v1/fleet/summary.
func (s *Server) handleFleetSummary(w http.ResponseWriter, r *http.Request) {
	if s.fleetService == nil {
		s.writeAPIError(w, r, http.StatusServiceUnavailable, "FLEET_SERVICE_UNAVAILABLE", "Fleet service is not configured")
		return
	}

	summary, err := s.fleetService.GetFleetSummary(r.Context())
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, "FLEET_SUMMARY_FAILED", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, summary)
}

// handleFleetGetNode handles GET /api/v1/fleet/{node_id}.
func (s *Server) handleFleetGetNode(w http.ResponseWriter, r *http.Request, nodeID string) {
	if s.fleetService == nil {
		s.writeAPIError(w, r, http.StatusServiceUnavailable, "FLEET_SERVICE_UNAVAILABLE", "Fleet service is not configured")
		return
	}

	nodeDetail, err := s.fleetService.GetNode(r.Context(), nodeID)
	if err != nil {
		if err == fleet.ErrNodeNotFound {
			s.writeAPIError(w, r, http.StatusNotFound, "NODE_NOT_FOUND", fmt.Sprintf("Node '%s' not found", nodeID))
			return
		}
		s.writeAPIError(w, r, http.StatusInternalServerError, "GET_NODE_FAILED", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, nodeDetail)
}

// handleFleetDeleteNode handles DELETE /api/v1/fleet/{node_id}.
func (s *Server) handleFleetDeleteNode(w http.ResponseWriter, r *http.Request, nodeID string) {
	if s.fleetService == nil {
		s.writeAPIError(w, r, http.StatusServiceUnavailable, "FLEET_SERVICE_UNAVAILABLE", "Fleet service is not configured")
		return
	}

	err := s.fleetService.DeleteNode(r.Context(), nodeID)
	if err != nil {
		if err == fleet.ErrNodeNotFound {
			s.writeAPIError(w, r, http.StatusNotFound, "NODE_NOT_FOUND", fmt.Sprintf("Node '%s' not found", nodeID))
			return
		}
		s.writeAPIError(w, r, http.StatusInternalServerError, "DELETE_NODE_FAILED", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"deleted":   true,
		"node_id":   nodeID,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// writeAPIError writes a standardized APIErrorResponse payload.
func (s *Server) writeAPIError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	reqID := GetRequestID(r.Context())
	resp := model.APIErrorResponse{
		Error: model.APIErrorDetail{
			Code:      code,
			Message:   message,
			RequestID: reqID,
			Timestamp: time.Now().UTC(),
		},
	}
	s.writeJSON(w, status, resp)
}
