package incidents

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

// IncidentStore defines persistence operations for incident management.
type IncidentStore interface {
	SaveIncident(ctx context.Context, inc *Incident) error
	GetIncident(ctx context.Context, id string) (*Incident, error)
	ListIncidents(ctx context.Context, filter IncidentFilter) ([]Incident, int, error)
	UpdateIncidentStatus(ctx context.Context, id string, status IncidentStatus, reason string, resolvedAt *time.Time) error
	SaveTimelineEntries(ctx context.Context, entries []IncidentTimelineEntry) error
	GetTimeline(ctx context.Context, incidentID string, filter TimelineFilter) ([]IncidentTimelineEntry, error)
	GetIncidentHistory(ctx context.Context, lookback time.Duration) ([]Incident, error)
}

// Service provides high-level business logic for incident detection, querying, and lifecycle management.
type Service interface {
	ListIncidents(ctx context.Context, filter IncidentFilter) ([]Incident, int, error)
	GetIncident(ctx context.Context, id string) (*Incident, error)
	GetTimeline(ctx context.Context, id string, filter TimelineFilter) ([]IncidentTimelineEntry, error)
	GetImpact(ctx context.Context, id string) (*ImpactAnalysis, error)
	GetFindings(ctx context.Context, id string) ([]IntelligenceFinding, error)
	GetSimilar(ctx context.Context, id string, minSimilarity float64, limit int) ([]SimilarIncidentResult, error)
	GetSummary(ctx context.Context) (*IncidentSummary, error)
	Investigate(ctx context.Context, id string) (*IncidentInvestigationReport, error)
	UpdateStatus(ctx context.Context, id string, newStatus IncidentStatus, reason string) (*Incident, error)
	EvaluateFleetSignals(ctx context.Context, nodes []NodeSignalsBundle, totalFleetNodes int) ([]Incident, error)
}

// DefaultService is the concrete implementation of Service.
type DefaultService struct {
	store     IncidentStore
	clusterer *IncidentClusterer
	mu        sync.RWMutex
}

// NewService instantiates an incident management service.
func NewService(store IncidentStore, window time.Duration) *DefaultService {
	if window <= 0 {
		window = 15 * time.Minute
	}
	return &DefaultService{
		store:     store,
		clusterer: NewIncidentClusterer(window),
	}
}

// ListIncidents retrieves incidents matching the specified filter.
func (s *DefaultService) ListIncidents(ctx context.Context, filter IncidentFilter) ([]Incident, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	return s.store.ListIncidents(ctx, filter)
}

// GetIncident retrieves a single incident by ID.
func (s *DefaultService) GetIncident(ctx context.Context, id string) (*Incident, error) {
	inc, err := s.store.GetIncident(ctx, id)
	if err != nil {
		return nil, err
	}
	if inc == nil {
		return nil, fmt.Errorf("%w: %s", ErrIncidentNotFound, id)
	}
	return inc, nil
}

// GetTimeline retrieves the filtered and sorted timeline for an incident.
func (s *DefaultService) GetTimeline(ctx context.Context, id string, filter TimelineFilter) ([]IncidentTimelineEntry, error) {
	inc, err := s.GetIncident(ctx, id)
	if err != nil {
		return nil, err
	}

	entries, err := s.store.GetTimeline(ctx, id, filter)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 && len(inc.Timeline) > 0 {
		entries = FilterTimeline(inc.Timeline, filter.NodeID, filter.MinSeverity, filter.StartTime, filter.EndTime)
	}

	if filter.Limit > 0 && len(entries) > filter.Limit {
		entries = entries[:filter.Limit]
	}

	return entries, nil
}

// GetImpact returns the blast-radius impact analysis for an incident.
func (s *DefaultService) GetImpact(ctx context.Context, id string) (*ImpactAnalysis, error) {
	inc, err := s.GetIncident(ctx, id)
	if err != nil {
		return nil, err
	}

	analysis := CalculateImpact(inc.ID, inc.AffectedNodes, nil, nil, inc.RootSignals, inc.Impact.TotalFleetNodes)
	return &analysis, nil
}

// GetFindings returns non-invasive advisory findings for an incident.
func (s *DefaultService) GetFindings(ctx context.Context, id string) ([]IntelligenceFinding, error) {
	inc, err := s.GetIncident(ctx, id)
	if err != nil {
		return nil, err
	}

	if len(inc.Findings) > 0 {
		return inc.Findings, nil
	}

	return GenerateIncidentFindings(inc), nil
}

// GetSimilar finds similar historical or active incidents.
func (s *DefaultService) GetSimilar(ctx context.Context, id string, minSimilarity float64, limit int) ([]SimilarIncidentResult, error) {
	target, err := s.GetIncident(ctx, id)
	if err != nil {
		return nil, err
	}

	candidates, _, err := s.store.ListIncidents(ctx, IncidentFilter{Limit: 200})
	if err != nil {
		return nil, err
	}

	return FindSimilarIncidents(*target, candidates, minSimilarity, limit), nil
}

// GetSummary returns fleet-wide incident aggregates.
func (s *DefaultService) GetSummary(ctx context.Context) (*IncidentSummary, error) {
	incidents, _, err := s.store.ListIncidents(ctx, IncidentFilter{Limit: 1000})
	if err != nil {
		return nil, err
	}

	summary := &IncidentSummary{
		TotalCount:        len(incidents),
		ScopeDistribution: make(map[string]int),
		EvaluatedAt:       time.Now().UTC(),
	}

	nodeMap := make(map[string]*NodeIncidentCount)
	var totalResolutionSec float64
	var resolvedCount int

	for _, inc := range incidents {
		switch inc.Status {
		case IncidentStatusDetected:
			summary.DetectedCount++
		case IncidentStatusAcknowledged:
			summary.AcknowledgedCount++
		case IncidentStatusInvestigating:
			summary.InvestigatingCount++
		case IncidentStatusResolved:
			summary.ResolvedCount++
			if inc.ResolvedAt != nil && !inc.StartTime.IsZero() {
				diff := inc.ResolvedAt.Sub(inc.StartTime).Seconds()
				if diff > 0 {
					totalResolutionSec += diff
					resolvedCount++
				}
			}
		case IncidentStatusSuppressed:
			summary.SuppressedCount++
		case IncidentStatusReopened:
			summary.ReopenedCount++
		}

		switch inc.Severity {
		case model.SeverityCritical:
			summary.CriticalCount++
		case model.SeverityWarning:
			summary.WarningCount++
		case model.SeverityInfo:
			summary.InfoCount++
		}

		summary.ScopeDistribution[string(inc.Scope)]++

		for _, n := range inc.AffectedNodes {
			if nodeMap[n] == nil {
				nodeMap[n] = &NodeIncidentCount{NodeID: n}
			}
			nodeMap[n].IncidentCount++
			if inc.Severity == model.SeverityCritical {
				nodeMap[n].CriticalCount++
			} else if inc.Severity == model.SeverityWarning {
				nodeMap[n].WarningCount++
			}
		}
	}

	if resolvedCount > 0 {
		summary.AverageResolutionTime = time.Duration((totalResolutionSec / float64(resolvedCount)) * float64(time.Second))
	}

	// Sort top affected nodes
	var nodeCounts []NodeIncidentCount
	for _, v := range nodeMap {
		nodeCounts = append(nodeCounts, *v)
	}
	sort.Slice(nodeCounts, func(i, j int) bool {
		if nodeCounts[i].IncidentCount != nodeCounts[j].IncidentCount {
			return nodeCounts[i].IncidentCount > nodeCounts[j].IncidentCount
		}
		return nodeCounts[i].CriticalCount > nodeCounts[j].CriticalCount
	})
	if len(nodeCounts) > 5 {
		nodeCounts = nodeCounts[:5]
	}
	summary.TopAffectedNodes = nodeCounts

	if len(incidents) > 10 {
		summary.RecentIncidents = incidents[:10]
	} else {
		summary.RecentIncidents = incidents
	}

	return summary, nil
}

// Investigate compiles a comprehensive dossier for an incident.
func (s *DefaultService) Investigate(ctx context.Context, id string) (*IncidentInvestigationReport, error) {
	target, err := s.GetIncident(ctx, id)
	if err != nil {
		return nil, err
	}

	history, err := s.store.GetIncidentHistory(ctx, 30*24*time.Hour)
	if err != nil {
		// Fallback to recent list
		history, _, _ = s.store.ListIncidents(ctx, IncidentFilter{Limit: 200})
	}

	report := BuildInvestigationReport(*target, history, nil, nil, target.Impact.TotalFleetNodes)
	return &report, nil
}

// UpdateStatus performs a validated state machine transition on an incident.
func (s *DefaultService) UpdateStatus(ctx context.Context, id string, newStatus IncidentStatus, reason string) (*Incident, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	inc, err := s.store.GetIncident(ctx, id)
	if err != nil {
		return nil, err
	}
	if inc == nil {
		return nil, fmt.Errorf("%w: %s", ErrIncidentNotFound, id)
	}

	if err := ValidateTransition(inc.Status, newStatus); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	var resolvedAt *time.Time
	if newStatus == IncidentStatusResolved {
		resolvedAt = &now
	}

	if err := s.store.UpdateIncidentStatus(ctx, id, newStatus, reason, resolvedAt); err != nil {
		return nil, err
	}

	// Add timeline entry recording the status transition
	tb := NewTimelineBuilder(id)
	tb.AddEntry(IncidentTimelineEntry{
		IncidentID:  id,
		Timestamp:   now,
		EventType:   TimelineEventStatusChanged,
		Source:      "lifecycle",
		Severity:    inc.Severity,
		Title:       fmt.Sprintf("Status transitioned to %s", newStatus),
		Description: fmt.Sprintf("Incident status changed from %s to %s. Reason: %s", inc.Status, newStatus, reason),
		Payload: map[string]any{
			"from_status": string(inc.Status),
			"to_status":   string(newStatus),
			"reason":      reason,
		},
	})
	_ = s.store.SaveTimelineEntries(ctx, tb.Build())

	return s.store.GetIncident(ctx, id)
}

// EvaluateFleetSignals detects, clusters, and persists incidents from current fleet signals.
func (s *DefaultService) EvaluateFleetSignals(ctx context.Context, nodes []NodeSignalsBundle, totalFleetNodes int) ([]Incident, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	detected := s.clusterer.ClusterFleetSignals(nodes, totalFleetNodes)
	for i := range detected {
		inc := &detected[i]
		if err := s.store.SaveIncident(ctx, inc); err != nil {
			return nil, fmt.Errorf("failed to save incident %s: %w", inc.ID, err)
		}
		if len(inc.Timeline) > 0 {
			if err := s.store.SaveTimelineEntries(ctx, inc.Timeline); err != nil {
				return nil, fmt.Errorf("failed to save timeline for incident %s: %w", inc.ID, err)
			}
		}
	}

	return detected, nil
}
