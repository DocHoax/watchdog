package intelligence

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

// IncidentClusterer aggregates and correlates multi-signal anomalies, alerts, and diagnostics into incidents.
type IncidentClusterer struct {
	window time.Duration
}

// NewIncidentClusterer instantiates an incident clustering engine.
func NewIncidentClusterer(window time.Duration) *IncidentClusterer {
	if window <= 0 {
		window = 15 * time.Minute
	}
	return &IncidentClusterer{window: window}
}

// ClusterNodeEvents groups active alerts, diagnostic failures, and anomalies for a node into incidents.
func (ic *IncidentClusterer) ClusterNodeEvents(nodeID string, hostname string, alerts []model.AlertEvent,
	diag *model.DiagnosticReport, anom *model.AnomalyReport) []Incident {

	var timeline []IncidentTimelineEvent
	var activeAlerts []model.AlertEvent
	var anomalyDescriptions []string
	var symptoms []string
	highestSeverity := model.SeverityInfo

	var earliestTime time.Time

	// 1. Process active alerts
	for _, a := range alerts {
		if !a.IsActive {
			continue
		}
		activeAlerts = append(activeAlerts, a)
		symptoms = append(symptoms, fmt.Sprintf("Alert [%s]: %s", a.RuleName, a.Message))

		if earliestTime.IsZero() || a.FiredAt.Before(earliestTime) {
			earliestTime = a.FiredAt
		}

		if a.Severity == model.SeverityCritical {
			highestSeverity = model.SeverityCritical
		} else if a.Severity == model.SeverityWarning && highestSeverity != model.SeverityCritical {
			highestSeverity = model.SeverityWarning
		}

		timeline = append(timeline, IncidentTimelineEvent{
			Timestamp:   a.FiredAt,
			NodeID:      nodeID,
			EventType:   "alert_fired",
			Description: fmt.Sprintf("Alert '%s' triggered: %s", a.RuleName, a.Message),
			Severity:    a.Severity,
		})
	}

	// 2. Process diagnostic failures
	if diag != nil {
		for _, r := range diag.Results {
			if r.Status == model.StatusFail || r.Status == model.StatusWarning {
				symptoms = append(symptoms, fmt.Sprintf("Diagnostic [%s]: %s", r.Name, r.Description))
				t := r.Timestamp
				if t.IsZero() {
					t = diag.GeneratedAt
				}
				if earliestTime.IsZero() || t.Before(earliestTime) {
					earliestTime = t
				}

				sev := r.Severity
				if sev == model.SeverityCritical {
					highestSeverity = model.SeverityCritical
				} else if sev == model.SeverityWarning && highestSeverity != model.SeverityCritical {
					highestSeverity = model.SeverityWarning
				}

				timeline = append(timeline, IncidentTimelineEvent{
					Timestamp:   t,
					NodeID:      nodeID,
					EventType:   "diagnostic_check",
					Description: fmt.Sprintf("Diagnostic check '%s' flagged %s: %s", r.Name, r.Status, r.Description),
					Severity:    sev,
				})
			}
		}
	}

	// 3. Process statistical anomalies
	if anom != nil {
		for _, s := range anom.Scores {
			if s.IsAnomaly {
				anomalyDescriptions = append(anomalyDescriptions, fmt.Sprintf("%s (%s)", s.MetricName, s.Explanation))
				symptoms = append(symptoms, fmt.Sprintf("Anomaly [%s]: %s", s.MetricName, s.Explanation))
				t := s.DetectedAt
				if t.IsZero() {
					t = anom.GeneratedAt
				}
				if earliestTime.IsZero() || t.Before(earliestTime) {
					earliestTime = t
				}

				if s.Severity == model.SeverityCritical {
					highestSeverity = model.SeverityCritical
				} else if s.Severity == model.SeverityWarning && highestSeverity != model.SeverityCritical {
					highestSeverity = model.SeverityWarning
				}

				timeline = append(timeline, IncidentTimelineEvent{
					Timestamp:   t,
					NodeID:      nodeID,
					EventType:   "statistical_anomaly",
					Description: fmt.Sprintf("Anomaly detected on %s: %s", s.MetricName, s.Explanation),
					Severity:    s.Severity,
				})
			}
		}
	}

	if len(timeline) == 0 {
		return nil
	}

	// Sort timeline chronologically
	sort.Slice(timeline, func(i, j int) bool {
		return timeline[i].Timestamp.Before(timeline[j].Timestamp)
	})

	// Generate deterministic Incident ID
	hashInput := fmt.Sprintf("%s:%d:%s", nodeID, earliestTime.Unix(), symptoms[0])
	hasher := sha256.New()
	hasher.Write([]byte(hashInput))
	incidentID := fmt.Sprintf("inc-%s", hex.EncodeToString(hasher.Sum(nil))[:12])

	// Synthesize Title
	title := fmt.Sprintf("Degradation cluster on %s (%d co-occurring signals)", hostname, len(symptoms))
	if len(activeAlerts) > 0 {
		title = fmt.Sprintf("Active alert and degradation on %s: %s", hostname, activeAlerts[0].RuleName)
	}

	inc := Incident{
		ID:               incidentID,
		Title:            title,
		Status:           IncidentStatusOpen,
		Severity:         highestSeverity,
		StartTime:        earliestTime,
		AffectedNodes:    []string{nodeID},
		PrimarySymptoms:  symptoms,
		RelatedAlerts:    activeAlerts,
		RelatedAnomalies: anomalyDescriptions,
		Timeline:         timeline,
	}

	return []Incident{inc}
}
