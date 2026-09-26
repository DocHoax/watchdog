package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

// handleAuditEvents handles GET /api/v1/audit/events requests.
func (s *Server) handleAuditEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		s.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if s.storage == nil {
		s.writeJSONError(w, http.StatusServiceUnavailable, "Storage engine is not available")
		return
	}

	q := r.URL.Query()

	var filter storage.AuditFilter

	// Parse 'since' parameter (e.g., "24h", "30m", or RFC3339 timestamp)
	if sinceStr := strings.TrimSpace(q.Get("since")); sinceStr != "" {
		if startTime, err := parseTimeOrDuration(sinceStr); err == nil {
			filter.StartTime = startTime
		} else {
			s.writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid 'since' parameter: %v", err))
			return
		}
	}

	// Parse 'until' parameter (e.g., "1h", or RFC3339 timestamp)
	if untilStr := strings.TrimSpace(q.Get("until")); untilStr != "" {
		if endTime, err := parseTimeOrDuration(untilStr); err == nil {
			filter.EndTime = endTime
		} else {
			s.writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid 'until' parameter: %v", err))
			return
		}
	}

	// Direct string filters
	filter.EventType = strings.TrimSpace(q.Get("event_type"))
	if filter.EventType == "" {
		filter.EventType = strings.TrimSpace(q.Get("type"))
	}
	filter.Severity = strings.TrimSpace(q.Get("severity"))
	filter.Outcome = strings.TrimSpace(q.Get("outcome"))
	filter.ActorType = strings.TrimSpace(q.Get("actor_type"))
	filter.ActorIdentity = strings.TrimSpace(q.Get("actor_identity"))
	if filter.ActorIdentity == "" {
		filter.ActorIdentity = strings.TrimSpace(q.Get("actor"))
	}
	filter.SourceAddress = strings.TrimSpace(q.Get("source_address"))
	if filter.SourceAddress == "" {
		filter.SourceAddress = strings.TrimSpace(q.Get("source"))
	}
	filter.RequestID = strings.TrimSpace(q.Get("request_id"))

	// Pagination: limit
	maxLimit := 1000
	if s.cfg != nil && s.cfg.Audit.MaxQueryLimit > 0 {
		maxLimit = s.cfg.Audit.MaxQueryLimit
	}

	limit := 100
	if limitStr := strings.TrimSpace(q.Get("limit")); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		} else if err != nil {
			s.writeJSONError(w, http.StatusBadRequest, "invalid 'limit' parameter: must be a positive integer")
			return
		}
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	filter.Limit = limit

	// Pagination: offset
	offset := 0
	if offsetStr := strings.TrimSpace(q.Get("offset")); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		} else if err != nil {
			s.writeJSONError(w, http.StatusBadRequest, "invalid 'offset' parameter: must be a non-negative integer")
			return
		}
	}
	filter.Offset = offset

	ctx := r.Context()

	events, err := s.storage.QueryAuditEvents(ctx, filter)
	if err != nil {
		s.writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to query audit events: %v", err))
		return
	}
	if events == nil {
		events = []model.AuditEvent{}
	}

	total, err := s.storage.CountAuditEvents(ctx, filter)
	if err != nil {
		s.writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to count audit events: %v", err))
		return
	}

	resp := map[string]interface{}{
		"events":    events,
		"count":     len(events),
		"total":     total,
		"limit":     filter.Limit,
		"offset":    filter.Offset,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	s.writeJSON(w, http.StatusOK, resp)
}

// parseTimeOrDuration parses a string as either an RFC3339 timestamp or a relative duration string.
func parseTimeOrDuration(s string) (time.Time, error) {
	// 1. Try RFC3339
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}

	// 2. Try RFC3339Nano
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC(), nil
	}

	// 3. Try Date format "2006-01-02"
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC(), nil
	}

	// 4. Try Day duration like "7d", "30d"
	if strings.HasSuffix(s, "d") || strings.HasSuffix(s, "D") {
		daysStr := s[:len(s)-1]
		if days, err := strconv.Atoi(daysStr); err == nil && days >= 0 {
			dur := time.Duration(days) * 24 * time.Hour
			return time.Now().UTC().Add(-dur), nil
		}
	}

	// 5. Try standard Go duration (e.g. "24h", "30m", "1h30m")
	if dur, err := time.ParseDuration(s); err == nil {
		return time.Now().UTC().Add(-dur), nil
	}

	return time.Time{}, fmt.Errorf("unable to parse '%s' as timestamp (RFC3339) or duration (e.g. 24h, 7d)", s)
}
