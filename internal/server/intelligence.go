package server

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/internal/intelligence"
	"github.com/DocHoax/watchdog/pkg/model"
)

// handleIntelligenceRoute multiplexes /api/v1/intelligence and /api/v1/intelligence/* requests.
func (s *Server) handleIntelligenceRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	intelSvc := s.IntelligenceService()
	if intelSvc == nil {
		s.writeAPIError(w, r, http.StatusServiceUnavailable, "INTELLIGENCE_SERVICE_UNAVAILABLE", "Intelligence service is not configured")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v1/intelligence")
	path = strings.TrimPrefix(path, "/")

	// 1. /api/v1/intelligence or /api/v1/intelligence/fleet
	if path == "" || path == "fleet" {
		s.handleIntelligenceFleet(w, r, intelSvc)
		return
	}

	// 2. /api/v1/intelligence/incidents or /api/v1/intelligence/incidents/{id}
	if strings.HasPrefix(path, "incidents") {
		subPath := strings.TrimPrefix(path, "incidents")
		subPath = strings.TrimPrefix(subPath, "/")
		if subPath == "" {
			s.handleIntelligenceIncidents(w, r, intelSvc)
		} else {
			s.handleIntelligenceIncidentByID(w, r, intelSvc, subPath)
		}
		return
	}

	// 3. /api/v1/intelligence/correlations
	if path == "correlations" {
		s.handleIntelligenceCorrelations(w, r, intelSvc)
		return
	}

	// 4. /api/v1/intelligence/findings
	if path == "findings" {
		s.handleIntelligenceFindings(w, r, intelSvc)
		return
	}

	// 5. /api/v1/intelligence/nodes/{id}, /api/v1/intelligence/nodes/{id}/trends, /api/v1/intelligence/nodes/{id}/baselines
	if strings.HasPrefix(path, "nodes/") {
		nodeParts := strings.Split(strings.TrimPrefix(path, "nodes/"), "/")
		nodeID := nodeParts[0]
		if nodeID == "" {
			s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_NODE_ID", "Node ID cannot be empty")
			return
		}

		if len(nodeParts) == 1 {
			s.handleIntelligenceNode(w, r, intelSvc, nodeID)
			return
		}

		if len(nodeParts) == 2 {
			switch nodeParts[1] {
			case "trends":
				s.handleIntelligenceNodeTrends(w, r, intelSvc, nodeID)
				return
			case "baselines":
				s.handleIntelligenceNodeBaselines(w, r, intelSvc, nodeID)
				return
			}
		}
	}

	s.writeAPIError(w, r, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("Path /api/v1/intelligence/%s not found", path))
}

func (s *Server) handleIntelligenceFleet(w http.ResponseWriter, r *http.Request, intelSvc intelligence.IntelligenceService) {
	summary, err := intelSvc.EvaluateFleetHealth(r.Context())
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, "INTELLIGENCE_EVAL_FAILED", fmt.Sprintf("Failed to evaluate fleet health: %v", err))
		return
	}
	s.writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleIntelligenceNode(w http.ResponseWriter, r *http.Request, intelSvc intelligence.IntelligenceService, nodeID string) {
	summary, err := intelSvc.EvaluateNodeHealth(r.Context(), nodeID)
	if err != nil {
		if errors.Is(err, intelligence.ErrNodeNotFound) {
			s.writeAPIError(w, r, http.StatusNotFound, "NODE_NOT_FOUND", fmt.Sprintf("Node '%s' not found", nodeID))
			return
		}
		s.writeAPIError(w, r, http.StatusInternalServerError, "INTELLIGENCE_EVAL_FAILED", fmt.Sprintf("Failed to evaluate node health: %v", err))
		return
	}
	s.writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleIntelligenceNodeTrends(w http.ResponseWriter, r *http.Request, intelSvc intelligence.IntelligenceService, nodeID string) {
	window := parseWindowDuration(r.URL.Query().Get("window"), 1*time.Hour)
	trends, err := intelSvc.GetNodeTrends(r.Context(), nodeID, window)
	if err != nil {
		if errors.Is(err, intelligence.ErrNodeNotFound) {
			s.writeAPIError(w, r, http.StatusNotFound, "NODE_NOT_FOUND", fmt.Sprintf("Node '%s' not found", nodeID))
			return
		}
		s.writeAPIError(w, r, http.StatusInternalServerError, "INTELLIGENCE_TRENDS_FAILED", fmt.Sprintf("Failed to get node trends: %v", err))
		return
	}
	s.writeJSON(w, http.StatusOK, trends)
}

func (s *Server) handleIntelligenceNodeBaselines(w http.ResponseWriter, r *http.Request, intelSvc intelligence.IntelligenceService, nodeID string) {
	window := parseWindowDuration(r.URL.Query().Get("window"), 24*time.Hour)
	baselines, err := intelSvc.GetNodeBaselines(r.Context(), nodeID, window)
	if err != nil {
		if errors.Is(err, intelligence.ErrNodeNotFound) {
			s.writeAPIError(w, r, http.StatusNotFound, "NODE_NOT_FOUND", fmt.Sprintf("Node '%s' not found", nodeID))
			return
		}
		s.writeAPIError(w, r, http.StatusInternalServerError, "INTELLIGENCE_BASELINES_FAILED", fmt.Sprintf("Failed to get node baselines: %v", err))
		return
	}
	s.writeJSON(w, http.StatusOK, baselines)
}

func (s *Server) handleIntelligenceIncidents(w http.ResponseWriter, r *http.Request, intelSvc intelligence.IntelligenceService) {
	incidents, err := intelSvc.GetActiveIncidents(r.Context())
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, "INTELLIGENCE_INCIDENTS_FAILED", fmt.Sprintf("Failed to get active incidents: %v", err))
		return
	}
	s.writeJSON(w, http.StatusOK, incidents)
}

func (s *Server) handleIntelligenceIncidentByID(w http.ResponseWriter, r *http.Request, intelSvc intelligence.IntelligenceService, incidentID string) {
	incident, err := intelSvc.GetIncident(r.Context(), incidentID)
	if err != nil {
		if errors.Is(err, intelligence.ErrIncidentNotFound) {
			s.writeAPIError(w, r, http.StatusNotFound, "INCIDENT_NOT_FOUND", fmt.Sprintf("Incident '%s' not found", incidentID))
			return
		}
		s.writeAPIError(w, r, http.StatusInternalServerError, "INTELLIGENCE_INCIDENT_FAILED", fmt.Sprintf("Failed to get incident: %v", err))
		return
	}
	s.writeJSON(w, http.StatusOK, incident)
}

func (s *Server) handleIntelligenceCorrelations(w http.ResponseWriter, r *http.Request, intelSvc intelligence.IntelligenceService) {
	window := parseWindowDuration(r.URL.Query().Get("window"), 1*time.Hour)
	correlations, err := intelSvc.GetCorrelations(r.Context(), window)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, "INTELLIGENCE_CORRELATIONS_FAILED", fmt.Sprintf("Failed to get correlations: %v", err))
		return
	}
	s.writeJSON(w, http.StatusOK, correlations)
}

func (s *Server) handleIntelligenceFindings(w http.ResponseWriter, r *http.Request, intelSvc intelligence.IntelligenceService) {
	category := intelligence.FindingCategory(r.URL.Query().Get("category"))
	minSeverity := model.Severity(r.URL.Query().Get("severity"))

	findings, err := intelSvc.GetFindings(r.Context(), category, minSeverity)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, "INTELLIGENCE_FINDINGS_FAILED", fmt.Sprintf("Failed to get findings: %v", err))
		return
	}
	s.writeJSON(w, http.StatusOK, findings)
}

func parseWindowDuration(raw string, defaultVal time.Duration) time.Duration {
	if raw == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return defaultVal
	}
	return d
}
