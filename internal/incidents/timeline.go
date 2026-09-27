package incidents

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

// TimelineBuilder aggregates and normalizes chronological timeline entries.
type TimelineBuilder struct {
	incidentID string
	entries    []IncidentTimelineEntry
}

// NewTimelineBuilder instantiates a timeline builder for an incident.
func NewTimelineBuilder(incidentID string) *TimelineBuilder {
	return &TimelineBuilder{
		incidentID: incidentID,
		entries:    make([]IncidentTimelineEntry, 0),
	}
}

// AddEntry appends a new timeline entry with a deterministic ID if missing.
func (tb *TimelineBuilder) AddEntry(entry IncidentTimelineEntry) {
	if entry.IncidentID == "" {
		entry.IncidentID = tb.incidentID
	}
	if entry.ID == "" {
		entry.ID = generateTimelineID(entry.IncidentID, entry.NodeID, string(entry.EventType), entry.Timestamp)
	}
	tb.entries = append(tb.entries, entry)
}

// AddSignal maps an incident signal to a timeline entry.
func (tb *TimelineBuilder) AddSignal(sig IncidentSignal) {
	var evtType TimelineEventType
	switch sig.Type {
	case SignalTypeAlert:
		evtType = TimelineEventAlertFired
	case SignalTypeDiagnostic:
		evtType = TimelineEventDiagnosticFailed
	case SignalTypeAnomaly:
		evtType = TimelineEventAnomalyDetected
	case SignalTypeCapacityRisk, SignalTypePrediction:
		evtType = TimelineEventThresholdApproached
	default:
		evtType = TimelineEventSignalDetected
	}

	title := fmt.Sprintf("%s on %s", sig.Type, sig.Source)
	if sig.NodeID != "" {
		title = fmt.Sprintf("%s on node %s: %s", sig.Type, sig.NodeID, sig.Source)
	}

	payload := make(map[string]any)
	if sig.Value > 0 || sig.Threshold > 0 {
		payload["value"] = sig.Value
		payload["threshold"] = sig.Threshold
	}
	for k, v := range sig.Metadata {
		payload[k] = v
	}

	tb.AddEntry(IncidentTimelineEntry{
		IncidentID:  tb.incidentID,
		Timestamp:   sig.Timestamp,
		NodeID:      sig.NodeID,
		EventType:   evtType,
		Source:      sig.Source,
		Severity:    sig.Severity,
		Title:       title,
		Description: sig.Description,
		Payload:     payload,
	})
}

// Build returns the deduplicated and chronologically sorted timeline entries.
func (tb *TimelineBuilder) Build() []IncidentTimelineEntry {
	seen := make(map[string]bool)
	var deduped []IncidentTimelineEntry

	for _, e := range tb.entries {
		if seen[e.ID] {
			continue
		}
		seen[e.ID] = true
		deduped = append(deduped, e)
	}

	// Sort chronologically ascending, tie-break by severity descending, then ID
	sort.Slice(deduped, func(i, j int) bool {
		if !deduped[i].Timestamp.Equal(deduped[j].Timestamp) {
			return deduped[i].Timestamp.Before(deduped[j].Timestamp)
		}
		if deduped[i].Severity != deduped[j].Severity {
			return severityRank(deduped[i].Severity) > severityRank(deduped[j].Severity)
		}
		return deduped[i].ID < deduped[j].ID
	})

	return deduped
}

// FilterTimeline returns a sub-slice of timeline entries matching specific criteria.
func FilterTimeline(entries []IncidentTimelineEntry, nodeID string, minSev model.Severity, startTime, endTime time.Time) []IncidentTimelineEntry {
	var filtered []IncidentTimelineEntry
	for _, e := range entries {
		if nodeID != "" && e.NodeID != nodeID && e.NodeID != "" {
			continue
		}
		if minSev != "" && severityRank(e.Severity) < severityRank(minSev) {
			continue
		}
		if !startTime.IsZero() && e.Timestamp.Before(startTime) {
			continue
		}
		if !endTime.IsZero() && e.Timestamp.After(endTime) {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered
}

func severityRank(sev model.Severity) int {
	switch sev {
	case model.SeverityCritical:
		return 3
	case model.SeverityWarning:
		return 2
	case model.SeverityInfo:
		return 1
	default:
		return 0
	}
}

func generateTimelineID(incidentID, nodeID, eventType string, t time.Time) string {
	raw := fmt.Sprintf("%s:%s:%s:%d", incidentID, nodeID, eventType, t.UnixMilli())
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("tl-%s", hex.EncodeToString(hash[:6]))
}
