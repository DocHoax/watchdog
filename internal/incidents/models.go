package incidents

import (
	"errors"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

var (
	// ErrIncidentNotFound indicates that the requested incident ID does not exist.
	ErrIncidentNotFound = errors.New("incident not found")
)

// IncidentStatus represents the formal lifecycle state of an incident.
type IncidentStatus string

const (
	IncidentStatusDetected      IncidentStatus = "detected"
	IncidentStatusAcknowledged  IncidentStatus = "acknowledged"
	IncidentStatusInvestigating IncidentStatus = "investigating"
	IncidentStatusResolved      IncidentStatus = "resolved"
	IncidentStatusSuppressed    IncidentStatus = "suppressed"
	IncidentStatusReopened      IncidentStatus = "reopened"
)

// IncidentScope defines the topological extent of an incident.
type IncidentScope string

const (
	IncidentScopeNode      IncidentScope = "node"
	IncidentScopeMultiNode IncidentScope = "multi_node"
	IncidentScopeFleet     IncidentScope = "fleet"
)

// SignalType categorizes the underlying observability signal contributing to an incident.
type SignalType string

const (
	SignalTypeAlert             SignalType = "alert"
	SignalTypeDiagnostic        SignalType = "diagnostic"
	SignalTypeAnomaly           SignalType = "anomaly"
	SignalTypeHealthDegradation SignalType = "health_degradation"
	SignalTypeCapacityRisk      SignalType = "capacity_risk"
	SignalTypeRecurrence        SignalType = "recurrence"
	SignalTypePrediction        SignalType = "prediction"
)

// IncidentSignal represents a discrete contributing telemetry or diagnostic signal.
type IncidentSignal struct {
	ID          string            `json:"id"`
	Type        SignalType        `json:"type"`
	Source      string            `json:"source"`
	NodeID      string            `json:"node_id,omitempty"`
	Hostname    string            `json:"hostname,omitempty"`
	Severity    model.Severity    `json:"severity"`
	Timestamp   time.Time         `json:"timestamp"`
	Description string            `json:"description"`
	Value       float64           `json:"value,omitempty"`
	Threshold   float64           `json:"threshold,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// TimelineEventType categorizes events in an incident's chronological lifecycle.
type TimelineEventType string

const (
	TimelineEventSignalDetected       TimelineEventType = "signal_detected"
	TimelineEventAlertFired           TimelineEventType = "alert_fired"
	TimelineEventAlertResolved        TimelineEventType = "alert_resolved"
	TimelineEventDiagnosticFailed     TimelineEventType = "diagnostic_failed"
	TimelineEventAnomalyDetected      TimelineEventType = "anomaly_detected"
	TimelineEventThresholdApproached  TimelineEventType = "threshold_approached"
	TimelineEventStatusChanged        TimelineEventType = "status_changed"
	TimelineEventCommentAdded         TimelineEventType = "comment_added"
	TimelineEventFindingGenerated     TimelineEventType = "finding_generated"
	TimelineEventMitigationObserved   TimelineEventType = "mitigation_observed"
)

// IncidentTimelineEntry represents a normalized, chronological event tied to an incident.
type IncidentTimelineEntry struct {
	ID          string            `json:"id"`
	IncidentID  string            `json:"incident_id"`
	Timestamp   time.Time         `json:"timestamp"`
	NodeID      string            `json:"node_id,omitempty"`
	EventType   TimelineEventType `json:"event_type"`
	Source      string            `json:"source"`
	Severity    model.Severity    `json:"severity"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Payload     map[string]any    `json:"payload,omitempty"`
}

// TimelineFilter specifies criteria for filtering incident timeline entries.
type TimelineFilter struct {
	NodeID      string         `json:"node_id,omitempty"`
	MinSeverity model.Severity `json:"min_severity,omitempty"`
	StartTime   time.Time      `json:"start_time,omitempty"`
	EndTime     time.Time      `json:"end_time,omitempty"`
	Limit       int            `json:"limit,omitempty"`
}

// ImpactScope details the blast radius, impacted subsystems, and fleet coverage.
type ImpactScope struct {
	Subsystems           []string            `json:"subsystems"`
	Resources            []string            `json:"resources"`
	AffectedNodeIDs      []string            `json:"affected_node_ids"`
	AffectedHostnames    []string            `json:"affected_hostnames"`
	TotalFleetNodes      int                 `json:"total_fleet_nodes"`
	FleetPercentage      float64             `json:"fleet_percentage"`
	SeverityDistribution map[string]int      `json:"severity_distribution"`
	CommonTags           map[string]string   `json:"common_tags,omitempty"`
	TagsDistribution     map[string][]string `json:"tags_distribution,omitempty"`
}

// ImpactAnalysis provides an explainable blast-radius assessment for an incident.
type ImpactAnalysis struct {
	IncidentID           string        `json:"incident_id"`
	Scope                IncidentScope `json:"scope"`
	Impact               ImpactScope   `json:"impact"`
	EstimatedBlastRadius string        `json:"estimated_blast_radius"`
	CriticalNodesCount   int           `json:"critical_nodes_count"`
	WarningNodesCount    int           `json:"warning_nodes_count"`
	AnalyzedAt           time.Time     `json:"analyzed_at"`
	Summary              string        `json:"summary"`
}

// SeverityFactorContribution documents an explainable component in the severity calculation.
type SeverityFactorContribution struct {
	Name        string  `json:"name"`
	Category    string  `json:"category"`
	Weight      float64 `json:"weight"`
	Points      float64 `json:"points"`
	Description string  `json:"description"`
}

// SeverityExplanation provides full transparency and explainability for calculated severity.
type SeverityExplanation struct {
	CalculatedSeverity model.Severity               `json:"calculated_severity"`
	BaseScore          float64                      `json:"base_score"`
	Confidence         string                       `json:"confidence"`
	Factors            []SeverityFactorContribution `json:"factors"`
	Reasoning          []string                     `json:"reasoning"`
}

// FindingCategory identifies the operational domain of an advisory finding.
type FindingCategory string

const (
	FindingCategoryResourceExhaustion     FindingCategory = "resource_exhaustion"
	FindingCategoryPerformanceDegradation FindingCategory = "performance_degradation"
	FindingCategoryFleetPattern           FindingCategory = "fleet_pattern"
	FindingCategoryStabilityRisk          FindingCategory = "stability_risk"
	FindingCategoryAnomalyCluster         FindingCategory = "anomaly_cluster"
	FindingCategoryCapacityRisk           FindingCategory = "capacity_risk"
	FindingCategoryPredictedDegradation   FindingCategory = "predicted_degradation"
	FindingCategoryThresholdForecast      FindingCategory = "threshold_forecast"
	FindingCategoryRecurringIncident      FindingCategory = "recurring_incident"
	FindingCategoryFleetCapacityPressure  FindingCategory = "fleet_capacity_pressure"
	FindingCategoryAcceleratingResource   FindingCategory = "accelerating_resource_usage"
)

// FindingConfidence rates confidence in an advisory finding.
type FindingConfidence string

const (
	FindingConfidenceHigh   FindingConfidence = "high"
	FindingConfidenceMedium FindingConfidence = "medium"
	FindingConfidenceLow    FindingConfidence = "low"
)

// IntelligenceFinding provides an explainable non-invasive advisory finding.
type IntelligenceFinding struct {
	ID                     string            `json:"id"`
	Category               FindingCategory   `json:"category"`
	Severity               model.Severity    `json:"severity"`
	Confidence             FindingConfidence `json:"confidence"`
	Title                  string            `json:"title"`
	Description            string            `json:"description"`
	AffectedNodes          []string          `json:"affected_nodes,omitempty"`
	SupportingEvidence     []string          `json:"supporting_evidence,omitempty"`
	NonInvasiveSuggestions []string          `json:"non_invasive_suggestions,omitempty"`
	DetectedAt             time.Time         `json:"detected_at"`
}

// PredictionConfidence rates confidence in a predictive threshold projection.
type PredictionConfidence string

const (
	PredictionConfidenceHigh             PredictionConfidence = "high"
	PredictionConfidenceMedium           PredictionConfidence = "medium"
	PredictionConfidenceLow              PredictionConfidence = "low"
	PredictionConfidenceInsufficientData PredictionConfidence = "insufficient_data"
)

// PredictionDirection indicates the directional movement of a metric relative to a threshold.
type PredictionDirection string

const (
	PredictionDirectionApproaching     PredictionDirection = "approaching"
	PredictionDirectionReceding        PredictionDirection = "receding"
	PredictionDirectionStable          PredictionDirection = "stable"
	PredictionDirectionAlreadyExceeded PredictionDirection = "already_exceeded"
	PredictionDirectionUnknown         PredictionDirection = "unknown"
)

// Prediction represents a deterministic linear threshold projection for a metric on a node.
type Prediction struct {
	ID                       string               `json:"id"`
	NodeID                   string               `json:"node_id"`
	Metric                   string               `json:"metric"`
	CurrentValue             float64              `json:"current_value"`
	TargetThreshold          float64              `json:"target_threshold"`
	Direction                PredictionDirection  `json:"direction"`
	SlopePerMinute           float64              `json:"slope_per_minute"`
	RSquared                 float64              `json:"r_squared"`
	Variance                 float64              `json:"variance"`
	Confidence               PredictionConfidence `json:"confidence"`
	EstimatedTimeToThreshold *time.Duration       `json:"estimated_time_to_threshold,omitempty"`
	PredictedCrossingTime    *time.Time           `json:"predicted_crossing_time,omitempty"`
	Horizon                  time.Duration        `json:"horizon"`
	ObservationWindow        time.Duration        `json:"observation_window"`
	SampleCount              int                  `json:"sample_count"`
	Method                   string               `json:"method"`
	Evidence                 []string             `json:"evidence"`
	GeneratedAt              time.Time            `json:"generated_at"`
}

// Incident is the comprehensive domain entity representing a correlated operational incident.
type Incident struct {
	ID                  string                  `json:"id"`
	Title               string                  `json:"title"`
	Summary             string                  `json:"summary"`
	Status              IncidentStatus          `json:"status"`
	Severity            model.Severity          `json:"severity"`
	Scope               IncidentScope           `json:"scope"`
	Confidence          string                  `json:"confidence"`
	StartTime           time.Time               `json:"start_time"`
	EndTime             *time.Time              `json:"end_time,omitempty"`
	AcknowledgedAt      *time.Time              `json:"acknowledged_at,omitempty"`
	ResolvedAt          *time.Time              `json:"resolved_at,omitempty"`
	AffectedNodes       []string                `json:"affected_nodes"`
	PrimarySymptoms     []string                `json:"primary_symptoms"`
	RootSignals         []IncidentSignal        `json:"root_signals"`
	SeverityScore       float64                 `json:"severity_score"`
	SeverityExplanation SeverityExplanation     `json:"severity_explanation"`
	Impact              ImpactScope             `json:"impact"`
	Timeline            []IncidentTimelineEntry `json:"timeline,omitempty"`
	Findings            []IntelligenceFinding   `json:"findings,omitempty"`
	Tags                map[string]string       `json:"tags,omitempty"`
	Metadata            map[string]string       `json:"metadata,omitempty"`
	CreatedAt           time.Time               `json:"created_at"`
	UpdatedAt           time.Time               `json:"updated_at"`
}

// IncidentEvent records a lifecycle or administrative event associated with an incident.
type IncidentEvent struct {
	ID             string            `json:"id"`
	IncidentID     string            `json:"incident_id"`
	Timestamp      time.Time         `json:"timestamp"`
	EventType      string            `json:"event_type"`
	Actor          string            `json:"actor,omitempty"`
	Message        string            `json:"message"`
	PreviousStatus IncidentStatus    `json:"previous_status,omitempty"`
	NewStatus      IncidentStatus    `json:"new_status,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// SimilarIncident represents a historical incident with high symptom/signal similarity.
type SimilarIncident struct {
	Incident        Incident      `json:"incident"`
	SimilarityScore float64       `json:"similarity_score"`
	SharedSymptoms  []string      `json:"shared_symptoms"`
	SharedNodes     []string      `json:"shared_nodes"`
	SharedSignals   []string      `json:"shared_signals"`
	TimeDifference  time.Duration `json:"time_difference"`
	Summary         string        `json:"summary"`
}

// NodeIncidentCount aggregates incident counts per node.
type NodeIncidentCount struct {
	NodeID        string `json:"node_id"`
	Hostname      string `json:"hostname,omitempty"`
	IncidentCount int    `json:"incident_count"`
	CriticalCount int    `json:"critical_count"`
	WarningCount  int    `json:"warning_count"`
}

// IncidentSummary provides aggregate cluster-wide metrics on active and historical incidents.
type IncidentSummary struct {
	TotalCount            int                 `json:"total_count"`
	DetectedCount         int                 `json:"detected_count"`
	AcknowledgedCount     int                 `json:"acknowledged_count"`
	InvestigatingCount    int                 `json:"investigating_count"`
	ResolvedCount         int                 `json:"resolved_count"`
	SuppressedCount       int                 `json:"suppressed_count"`
	ReopenedCount         int                 `json:"reopened_count"`
	CriticalCount         int                 `json:"critical_count"`
	WarningCount          int                 `json:"warning_count"`
	InfoCount             int                 `json:"info_count"`
	ScopeDistribution     map[string]int      `json:"scope_distribution"`
	AverageResolutionTime time.Duration       `json:"average_resolution_time"`
	RecentIncidents       []Incident          `json:"recent_incidents"`
	TopAffectedNodes      []NodeIncidentCount `json:"top_affected_nodes"`
	EvaluatedAt           time.Time           `json:"evaluated_at"`
}

// IncidentFilter defines query parameters for filtering and listing incidents.
type IncidentFilter struct {
	Status    []IncidentStatus `json:"status,omitempty"`
	Severity  []model.Severity `json:"severity,omitempty"`
	Scope     []IncidentScope  `json:"scope,omitempty"`
	NodeID    string           `json:"node_id,omitempty"`
	Search    string           `json:"search,omitempty"`
	StartTime time.Time        `json:"start_time,omitempty"`
	EndTime   time.Time        `json:"end_time,omitempty"`
	Limit     int              `json:"limit,omitempty"`
	Offset    int              `json:"offset,omitempty"`
	SortBy    string           `json:"sort_by,omitempty"`
	SortOrder string           `json:"sort_order,omitempty"`
}

// CreateIncidentRequest defines the payload for creating or registering a correlated incident.
type CreateIncidentRequest struct {
	Title           string            `json:"title"`
	Summary         string            `json:"summary,omitempty"`
	Severity        model.Severity    `json:"severity,omitempty"`
	Scope           IncidentScope     `json:"scope,omitempty"`
	AffectedNodes   []string          `json:"affected_nodes"`
	PrimarySymptoms []string          `json:"primary_symptoms"`
	RootSignals     []IncidentSignal  `json:"root_signals,omitempty"`
	Tags            map[string]string `json:"tags,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	StartTime       time.Time         `json:"start_time,omitempty"`
}

// UpdateStatusRequest defines the payload for transitioning an incident status.
type UpdateStatusRequest struct {
	Status   IncidentStatus    `json:"status"`
	Message  string            `json:"message,omitempty"`
	Actor    string            `json:"actor,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}
