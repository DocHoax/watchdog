package incidents

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/DocHoax/watchdog/internal/intelligence"
	"github.com/DocHoax/watchdog/pkg/model"
)

// CorrelationConfig defines parameters for multi-signal correlation and temporal grouping.
type CorrelationConfig struct {
	Window              time.Duration
	CrossNodeClustering bool
}

// DefaultCorrelationConfig returns standard parameters for correlation.
func DefaultCorrelationConfig() CorrelationConfig {
	return CorrelationConfig{
		Window:              15 * time.Minute,
		CrossNodeClustering: true,
	}
}

// SignalCorrelator correlates raw observability signals into correlated signal clusters.
type SignalCorrelator struct {
	cfg CorrelationConfig
}

// NewSignalCorrelator creates a signal correlator with the specified configuration.
func NewSignalCorrelator(cfg ...CorrelationConfig) *SignalCorrelator {
	c := DefaultCorrelationConfig()
	if len(cfg) > 0 {
		c = cfg[0]
	}
	if c.Window <= 0 {
		c.Window = 15 * time.Minute
	}
	return &SignalCorrelator{cfg: c}
}

// CorrelateSignals transforms alerts, diagnostics, anomalies, and predictions into normalized IncidentSignals.
func (sc *SignalCorrelator) CorrelateSignals(
	nodeID string,
	hostname string,
	alerts []model.AlertEvent,
	diag *model.DiagnosticReport,
	anom *model.AnomalyReport,
	preds []intelligence.Prediction,
) []IncidentSignal {
	var signals []IncidentSignal

	// 1. Process active alerts
	for _, a := range alerts {
		if !a.IsActive {
			continue
		}
		sigID := fmt.Sprintf("sig-alt-%s", a.ID)
		signals = append(signals, IncidentSignal{
			ID:          sigID,
			Type:        SignalTypeAlert,
			Source:      a.RuleName,
			NodeID:      nodeID,
			Hostname:    hostname,
			Severity:    a.Severity,
			Timestamp:   a.FiredAt,
			Description: a.Message,
			Value:       a.ActualValue,
			Threshold:   a.Threshold,
			Metadata: map[string]string{
				"rule_id":     a.RuleID,
				"metric_name": a.MetricName,
			},
		})
	}

	// 2. Process diagnostic checks
	if diag != nil {
		for _, r := range diag.Results {
			if r.Status == model.StatusFail || r.Status == model.StatusWarning {
				t := r.Timestamp
				if t.IsZero() {
					t = diag.GeneratedAt
				}
				sigID := generateSignalID(nodeID, "diag", r.Name, t)
				signals = append(signals, IncidentSignal{
					ID:          sigID,
					Type:        SignalTypeDiagnostic,
					Source:      r.Name,
					NodeID:      nodeID,
					Hostname:    hostname,
					Severity:    r.Severity,
					Timestamp:   t,
					Description: fmt.Sprintf("%s: %s", r.Status, r.Description),
					Metadata: map[string]string{
						"category": r.Category,
						"status":   string(r.Status),
					},
				})
			}
		}
	}

	// 3. Process statistical anomalies
	if anom != nil {
		for _, s := range anom.Scores {
			if s.IsAnomaly {
				t := s.DetectedAt
				if t.IsZero() {
					t = anom.GeneratedAt
				}
				sigID := generateSignalID(nodeID, "anom", s.MetricName, t)
				signals = append(signals, IncidentSignal{
					ID:          sigID,
					Type:        SignalTypeAnomaly,
					Source:      s.MetricName,
					NodeID:      nodeID,
					Hostname:    hostname,
					Severity:    s.Severity,
					Timestamp:   t,
					Description: s.Explanation,
					Value:       s.CurrentValue,
					Threshold:   s.Mean + 3*s.StdDev,
					Metadata: map[string]string{
						"z_score":       fmt.Sprintf("%.2f", s.ZScore),
						"deviation_pct": fmt.Sprintf("%.2f", s.DeviationPct),
					},
				})
			}
		}
	}

	// 4. Process predictions / capacity risks
	for _, p := range preds {
		if p.Direction == intelligence.PredictionDirectionApproaching || p.Direction == intelligence.PredictionDirectionAlreadyExceeded {
			sev := model.SeverityWarning
			if p.EstimatedTimeToThreshold != nil && *p.EstimatedTimeToThreshold < 1*time.Hour {
				sev = model.SeverityCritical
			}
			sigID := generateSignalID(nodeID, "pred", p.Metric, p.GeneratedAt)
			signals = append(signals, IncidentSignal{
				ID:          sigID,
				Type:        SignalTypePrediction,
				Source:      p.Metric,
				NodeID:      nodeID,
				Hostname:    hostname,
				Severity:    sev,
				Timestamp:   p.GeneratedAt,
				Description: fmt.Sprintf("Metric %s projected to cross threshold %.2f (slope: %.4f/min, R²: %.2f)", p.Metric, p.TargetThreshold, p.SlopePerMinute, p.RSquared),
				Value:       p.CurrentValue,
				Threshold:   p.TargetThreshold,
				Metadata: map[string]string{
					"confidence": string(p.Confidence),
				},
			})
		}
	}

	return signals
}

func generateSignalID(nodeID, sigType, source string, t time.Time) string {
	raw := fmt.Sprintf("%s:%s:%s:%d", nodeID, sigType, source, t.UnixMilli())
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("sig-%s-%s", sigType, hex.EncodeToString(hash[:4]))
}

// GenerateIncidentID produces a deterministic identifier for an incident.
func GenerateIncidentID(scope IncidentScope, nodeStr, symptom string, t time.Time) string {
	hashInput := fmt.Sprintf("%s:%s:%d:%s", scope, nodeStr, t.Unix(), symptom)
	hasher := sha256.New()
	hasher.Write([]byte(hashInput))
	return fmt.Sprintf("inc-%s-%s", scope, hex.EncodeToString(hasher.Sum(nil))[:10])
}
