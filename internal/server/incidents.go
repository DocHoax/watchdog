package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/internal/audit"
	"github.com/DocHoax/watchdog/internal/incidents"
	"github.com/DocHoax/watchdog/pkg/model"
)

// handleIncidentsRoute multiplexes /api/v1/incidents and /api/v1/incidents/* requests.
func (s *Server) handleIncidentsRoute(w http.ResponseWriter, r *http.Request) {
	incSvc := s.IncidentService()
	if incSvc == nil {
		s.writeAPIError(w, r, http.StatusServiceUnavailable, "INCIDENT_SERVICE_UNAVAILABLE", "Incident service is not configured")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v1/incidents")
	path = strings.TrimPrefix(path, "/")

	// 1. Root /api/v1/incidents: List & Filter
	if path == "" {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
			return
		}
		s.handleListIncidents(w, r, incSvc)
		return
	}

	// 2. /api/v1/incidents/summary
	if path == "summary" {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
			return
		}
		s.handleIncidentSummary(w, r, incSvc)
		return
	}

	// 3. /api/v1/incidents/similar
	if path == "similar" {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
			return
		}
		s.handleIncidentSimilar(w, r, incSvc)
		return
	}

	// 4. Parameterized routes by Incident ID: /api/v1/incidents/{id}/...
	parts := strings.Split(path, "/")
	incidentID := parts[0]
	if incidentID == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_INCIDENT_ID", "Incident ID cannot be empty")
		return
	}

	if len(parts) == 1 {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
			return
		}
		s.handleGetIncident(w, r, incSvc, incidentID)
		return
	}

	if len(parts) == 2 {
		subResource := parts[1]
		switch subResource {
		case "timeline":
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				w.Header().Set("Allow", "GET, HEAD")
				s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
				return
			}
			s.handleGetIncidentTimeline(w, r, incSvc, incidentID)
			return

		case "related":
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				w.Header().Set("Allow", "GET, HEAD")
				s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
				return
			}
			s.handleGetIncidentRelated(w, r, incSvc, incidentID)
			return

		case "impact":
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				w.Header().Set("Allow", "GET, HEAD")
				s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
				return
			}
			s.handleGetIncidentImpact(w, r, incSvc, incidentID)
			return

		case "findings":
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				w.Header().Set("Allow", "GET, HEAD")
				s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
				return
			}
			s.handleGetIncidentFindings(w, r, incSvc, incidentID)
			return

		case "investigate":
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				w.Header().Set("Allow", "GET, HEAD")
				s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
				return
			}
			s.handleGetIncidentInvestigate(w, r, incSvc, incidentID)
			return

		case "status":
			if r.Method != http.MethodPost {
				w.Header().Set("Allow", "POST")
				s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
				return
			}
			s.handleUpdateIncidentStatus(w, r, incSvc, incidentID)
			return

		default:
			s.writeAPIError(w, r, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("Unknown incident subresource: %s", subResource))
			return
		}
	}

	s.writeAPIError(w, r, http.StatusNotFound, "NOT_FOUND", "Endpoint not found")
}

// handleListIncidents handles GET /api/v1/incidents with rich filtering and pagination.
func (s *Server) handleListIncidents(w http.ResponseWriter, r *http.Request, svc incidents.Service) {
	q := r.URL.Query()
	var filter incidents.IncidentFilter

	// Status filter (comma-separated or multiple keys)
	if rawStatuses := q["status"]; len(rawStatuses) > 0 {
		for _, raw := range rawStatuses {
			for _, st := range strings.Split(raw, ",") {
				st = strings.TrimSpace(st)
				if st != "" {
					statusVal := incidents.IncidentStatus(st)
					if !incidents.IsValidStatus(statusVal) {
						s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_STATUS_FILTER", fmt.Sprintf("Invalid status: %s", st))
						return
					}
					filter.Status = append(filter.Status, statusVal)
				}
			}
		}
	}

	// Severity filter (comma-separated or multiple keys)
	if rawSeverities := q["severity"]; len(rawSeverities) > 0 {
		for _, raw := range rawSeverities {
			for _, sv := range strings.Split(raw, ",") {
				sv = strings.TrimSpace(sv)
				if sv != "" {
					filter.Severity = append(filter.Severity, model.Severity(strings.ToUpper(sv)))
				}
			}
		}
	}

	// Scope filter (comma-separated or multiple keys)
	if rawScopes := q["scope"]; len(rawScopes) > 0 {
		for _, raw := range rawScopes {
			for _, sc := range strings.Split(raw, ",") {
				sc = strings.TrimSpace(sc)
				if sc != "" {
					filter.Scope = append(filter.Scope, incidents.IncidentScope(sc))
				}
			}
		}
	}

	// NodeID and Search filters
	filter.NodeID = strings.TrimSpace(q.Get("node_id"))
	filter.Search = strings.TrimSpace(q.Get("search"))

	// Time filters (RFC3339 or relative duration)
	if startStr := strings.TrimSpace(q.Get("start_time")); startStr != "" {
		if t, err := parseTimeOrDuration(startStr); err == nil {
			filter.StartTime = t
		} else {
			s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_START_TIME", fmt.Sprintf("Invalid start_time: %v", err))
			return
		}
	}
	if endStr := strings.TrimSpace(q.Get("end_time")); endStr != "" {
		if t, err := parseTimeOrDuration(endStr); err == nil {
			filter.EndTime = t
		} else {
			s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_END_TIME", fmt.Sprintf("Invalid end_time: %v", err))
			return
		}
	}

	// Pagination
	limit := 50
	if limitStr := strings.TrimSpace(q.Get("limit")); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		} else {
			s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_LIMIT", "limit must be a positive integer")
			return
		}
	}
	if limit > 500 {
		limit = 500
	}
	filter.Limit = limit

	offset := 0
	if offsetStr := strings.TrimSpace(q.Get("offset")); offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		} else {
			s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_OFFSET", "offset must be a non-negative integer")
			return
		}
	}
	filter.Offset = offset

	// Sorting
	filter.SortBy = strings.TrimSpace(q.Get("sort_by"))
	filter.SortOrder = strings.TrimSpace(q.Get("sort_order"))

	list, total, err := svc.ListIncidents(r.Context(), filter)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, "LIST_INCIDENTS_FAILED", err.Error())
		return
	}

	if list == nil {
		list = []incidents.Incident{}
	}

	resp := map[string]any{
		"incidents": list,
		"total":     total,
		"limit":     filter.Limit,
		"offset":    filter.Offset,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	s.writeJSON(w, http.StatusOK, resp)
}

// handleIncidentSummary handles GET /api/v1/incidents/summary.
func (s *Server) handleIncidentSummary(w http.ResponseWriter, r *http.Request, svc incidents.Service) {
	summary, err := svc.GetSummary(r.Context())
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, "GET_SUMMARY_FAILED", err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, summary)
}

// handleIncidentSimilar handles GET /api/v1/incidents/similar.
func (s *Server) handleIncidentSimilar(w http.ResponseWriter, r *http.Request, svc incidents.Service) {
	q := r.URL.Query()
	incidentID := strings.TrimSpace(q.Get("id"))
	if incidentID == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, "MISSING_INCIDENT_ID", "Query parameter 'id' is required")
		return
	}

	minSimilarity := 0.3
	if minSimStr := strings.TrimSpace(q.Get("min_similarity")); minSimStr != "" {
		if val, err := strconv.ParseFloat(minSimStr, 64); err == nil && val >= 0.0 && val <= 1.0 {
			minSimilarity = val
		} else {
			s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_MIN_SIMILARITY", "min_similarity must be between 0.0 and 1.0")
			return
		}
	}

	limit := 10
	if limitStr := strings.TrimSpace(q.Get("limit")); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	similar, err := svc.GetSimilar(r.Context(), incidentID, minSimilarity, limit)
	if err != nil {
		if errors.Is(err, incidents.ErrIncidentNotFound) {
			s.writeAPIError(w, r, http.StatusNotFound, "INCIDENT_NOT_FOUND", fmt.Sprintf("Incident '%s' not found", incidentID))
			return
		}
		s.writeAPIError(w, r, http.StatusInternalServerError, "GET_SIMILAR_FAILED", err.Error())
		return
	}

	if similar == nil {
		similar = []incidents.SimilarIncidentResult{}
	}

	resp := map[string]any{
		"incident_id":        incidentID,
		"similar_incidents":  similar,
		"count":              len(similar),
		"min_similarity_cut": minSimilarity,
		"timestamp":          time.Now().UTC().Format(time.RFC3339),
	}
	s.writeJSON(w, http.StatusOK, resp)
}

// handleGetIncident handles GET /api/v1/incidents/{id}.
func (s *Server) handleGetIncident(w http.ResponseWriter, r *http.Request, svc incidents.Service, id string) {
	inc, err := svc.GetIncident(r.Context(), id)
	if err != nil {
		if errors.Is(err, incidents.ErrIncidentNotFound) {
			s.writeAPIError(w, r, http.StatusNotFound, "INCIDENT_NOT_FOUND", fmt.Sprintf("Incident '%s' not found", id))
			return
		}
		s.writeAPIError(w, r, http.StatusInternalServerError, "GET_INCIDENT_FAILED", err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, inc)
}

// handleGetIncidentTimeline handles GET /api/v1/incidents/{id}/timeline.
func (s *Server) handleGetIncidentTimeline(w http.ResponseWriter, r *http.Request, svc incidents.Service, id string) {
	q := r.URL.Query()
	var filter incidents.TimelineFilter

	filter.NodeID = strings.TrimSpace(q.Get("node_id"))
	if sev := strings.TrimSpace(q.Get("min_severity")); sev != "" {
		filter.MinSeverity = model.Severity(strings.ToUpper(sev))
	}

	if startStr := strings.TrimSpace(q.Get("start_time")); startStr != "" {
		if t, err := parseTimeOrDuration(startStr); err == nil {
			filter.StartTime = t
		} else {
			s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_START_TIME", fmt.Sprintf("Invalid start_time: %v", err))
			return
		}
	}
	if endStr := strings.TrimSpace(q.Get("end_time")); endStr != "" {
		if t, err := parseTimeOrDuration(endStr); err == nil {
			filter.EndTime = t
		} else {
			s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_END_TIME", fmt.Sprintf("Invalid end_time: %v", err))
			return
		}
	}

	if limitStr := strings.TrimSpace(q.Get("limit")); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			filter.Limit = parsed
		}
	}

	entries, err := svc.GetTimeline(r.Context(), id, filter)
	if err != nil {
		if errors.Is(err, incidents.ErrIncidentNotFound) {
			s.writeAPIError(w, r, http.StatusNotFound, "INCIDENT_NOT_FOUND", fmt.Sprintf("Incident '%s' not found", id))
			return
		}
		s.writeAPIError(w, r, http.StatusInternalServerError, "GET_TIMELINE_FAILED", err.Error())
		return
	}

	if entries == nil {
		entries = []incidents.IncidentTimelineEntry{}
	}

	resp := map[string]any{
		"incident_id": id,
		"timeline":    entries,
		"count":       len(entries),
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	}
	s.writeJSON(w, http.StatusOK, resp)
}

// handleGetIncidentRelated handles GET /api/v1/incidents/{id}/related.
func (s *Server) handleGetIncidentRelated(w http.ResponseWriter, r *http.Request, svc incidents.Service, id string) {
	report, err := svc.Investigate(r.Context(), id)
	if err != nil {
		if errors.Is(err, incidents.ErrIncidentNotFound) {
			s.writeAPIError(w, r, http.StatusNotFound, "INCIDENT_NOT_FOUND", fmt.Sprintf("Incident '%s' not found", id))
			return
		}
		s.writeAPIError(w, r, http.StatusInternalServerError, "GET_RELATED_FAILED", err.Error())
		return
	}

	resp := map[string]any{
		"incident_id":       id,
		"signals":           report.Incident.RootSignals,
		"recurrence":        report.Recurrence,
		"similar_incidents": report.SimilarIncidents,
		"timestamp":         time.Now().UTC().Format(time.RFC3339),
	}
	s.writeJSON(w, http.StatusOK, resp)
}

// handleGetIncidentImpact handles GET /api/v1/incidents/{id}/impact.
func (s *Server) handleGetIncidentImpact(w http.ResponseWriter, r *http.Request, svc incidents.Service, id string) {
	impact, err := svc.GetImpact(r.Context(), id)
	if err != nil {
		if errors.Is(err, incidents.ErrIncidentNotFound) {
			s.writeAPIError(w, r, http.StatusNotFound, "INCIDENT_NOT_FOUND", fmt.Sprintf("Incident '%s' not found", id))
			return
		}
		s.writeAPIError(w, r, http.StatusInternalServerError, "GET_IMPACT_FAILED", err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, impact)
}

// handleGetIncidentFindings handles GET /api/v1/incidents/{id}/findings.
func (s *Server) handleGetIncidentFindings(w http.ResponseWriter, r *http.Request, svc incidents.Service, id string) {
	findings, err := svc.GetFindings(r.Context(), id)
	if err != nil {
		if errors.Is(err, incidents.ErrIncidentNotFound) {
			s.writeAPIError(w, r, http.StatusNotFound, "INCIDENT_NOT_FOUND", fmt.Sprintf("Incident '%s' not found", id))
			return
		}
		s.writeAPIError(w, r, http.StatusInternalServerError, "GET_FINDINGS_FAILED", err.Error())
		return
	}

	if findings == nil {
		findings = []incidents.IntelligenceFinding{}
	}

	resp := map[string]any{
		"incident_id": id,
		"findings":    findings,
		"count":       len(findings),
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	}
	s.writeJSON(w, http.StatusOK, resp)
}

// handleGetIncidentInvestigate handles GET /api/v1/incidents/{id}/investigate.
func (s *Server) handleGetIncidentInvestigate(w http.ResponseWriter, r *http.Request, svc incidents.Service, id string) {
	report, err := svc.Investigate(r.Context(), id)
	if err != nil {
		if errors.Is(err, incidents.ErrIncidentNotFound) {
			s.writeAPIError(w, r, http.StatusNotFound, "INCIDENT_NOT_FOUND", fmt.Sprintf("Incident '%s' not found", id))
			return
		}
		s.writeAPIError(w, r, http.StatusInternalServerError, "INVESTIGATE_FAILED", err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, report)
}

// UpdateIncidentStatusRequest defines the request body for changing incident status.
type UpdateIncidentStatusRequest struct {
	Status incidents.IncidentStatus `json:"status"`
	Reason string                 `json:"reason"`
}

// handleUpdateIncidentStatus handles POST /api/v1/incidents/{id}/status.
func (s *Server) handleUpdateIncidentStatus(w http.ResponseWriter, r *http.Request, svc incidents.Service, id string) {
	var req UpdateIncidentStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", fmt.Sprintf("Invalid JSON body: %v", err))
		return
	}

	if !incidents.IsValidStatus(req.Status) {
		s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_STATUS", fmt.Sprintf("Invalid incident status '%s'", req.Status))
		return
	}

	updated, err := svc.UpdateStatus(r.Context(), id, req.Status, req.Reason)
	if err != nil {
		if errors.Is(err, incidents.ErrIncidentNotFound) {
			s.writeAPIError(w, r, http.StatusNotFound, "INCIDENT_NOT_FOUND", fmt.Sprintf("Incident '%s' not found", id))
			return
		}
		if errors.Is(err, incidents.ErrInvalidTransition) || errors.Is(err, incidents.ErrInvalidStatus) {
			s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_STATUS_TRANSITION", err.Error())
			return
		}
		s.writeAPIError(w, r, http.StatusInternalServerError, "UPDATE_STATUS_FAILED", err.Error())
		return
	}

	// Audit the state change if audit logging is enabled
	if s.auditLog != nil {
		reqID := GetRequestID(r.Context())
		_ = s.auditLog.Record(r.Context(), model.AuditEvent{
			ID:        audit.GenerateEventID(),
			Timestamp: time.Now().UTC(),
			EventType: model.EventIncidentStatusChange,
			Severity:  model.AuditSeverityInfo,
			Outcome:   model.AuditOutcomeSuccess,
			Actor: model.AuditActor{
				Type:     model.ActorTypeAuthenticatedClient,
				Identity: "api-client",
			},
			Source: model.AuditSource{
				Address:   r.RemoteAddr,
				Endpoint:  r.URL.Path,
				Method:    r.Method,
				RequestID: reqID,
				UserAgent: r.UserAgent(),
			},
			Message: fmt.Sprintf("Incident '%s' status transitioned to '%s' (Reason: %s)", id, req.Status, req.Reason),
			Metadata: map[string]string{
				"incident_id": id,
				"new_status":  string(req.Status),
				"reason":      req.Reason,
			},
		})
	}

	s.writeJSON(w, http.StatusOK, updated)
}
