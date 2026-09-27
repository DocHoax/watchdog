package intelligence

import (
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

// Trajectory describes the directional change of a node's health score over time.
type Trajectory string

const (
	TrajectoryImproving Trajectory = "improving"
	TrajectoryDegrading Trajectory = "degrading"
	TrajectoryStable    Trajectory = "stable"
	TrajectoryVolatile  Trajectory = "volatile"
	TrajectoryUnknown   Trajectory = "unknown"
)

// Impact indicates whether a factor positively, negatively, or neutrally affects health.
type Impact string

const (
	ImpactPositive Impact = "positive"
	ImpactNegative Impact = "negative"
	ImpactNeutral  Impact = "neutral"
)

// FactorContribution represents an explainable component of the health score calculation.
type FactorContribution struct {
	Name        string  `json:"name"`
	Category    string  `json:"category"`
	Weight      float64 `json:"weight"`
	Score       float64 `json:"score"`
	Deduction   float64 `json:"deduction"`
	Impact      Impact  `json:"impact"`
	Explanation string  `json:"explanation"`
}

// HealthScore provides a normalized 0-100 score and an explainable breakdown.
type HealthScore struct {
	Score            float64              `json:"score"`
	NormalizedStatus model.NodeStatus     `json:"normalized_status"`
	Trajectory       Trajectory           `json:"trajectory"`
	Breakdown        []FactorContribution `json:"breakdown"`
	PrimaryConcerns  []string             `json:"primary_concerns"`
	EvaluatedAt      time.Time            `json:"evaluated_at"`
}

// TrendDirection indicates the direction of a metric's trajectory.
type TrendDirection string

const (
	TrendDirectionIncreasing       TrendDirection = "increasing"
	TrendDirectionDecreasing       TrendDirection = "decreasing"
	TrendDirectionStable           TrendDirection = "stable"
	TrendDirectionInsufficientData TrendDirection = "insufficient_data"
)

// HealthTrend represents a statistical trend for a single metric over a time window.
type HealthTrend struct {
	Metric       string         `json:"metric"`
	Direction    TrendDirection `json:"direction"`
	RateOfChange float64        `json:"rate_of_change"`
	Unit         string         `json:"unit"`
	StartValue   float64        `json:"start_value"`
	EndValue     float64        `json:"end_value"`
	Window       time.Duration  `json:"window"`
	Confidence   float64        `json:"confidence"`
}

// HistoricalBaseline represents precomputed statistical properties for a metric over a window.
type HistoricalBaseline struct {
	Metric      string        `json:"metric"`
	Window      time.Duration `json:"window"`
	SampleCount int           `json:"sample_count"`
	Min         float64       `json:"min"`
	Max         float64       `json:"max"`
	Mean        float64       `json:"mean"`
	StdDev      float64       `json:"std_dev"`
	P50         float64       `json:"p50"`
	P90         float64       `json:"p90"`
	P95         float64       `json:"p95"`
	P99         float64       `json:"p99"`
	ComputedAt  time.Time     `json:"computed_at"`
}

// CorrelationConfidence rates confidence in a temporal correlation.
type CorrelationConfidence string

const (
	CorrelationConfidenceHigh   CorrelationConfidence = "high"
	CorrelationConfidenceMedium CorrelationConfidence = "medium"
	CorrelationConfidenceLow    CorrelationConfidence = "low"
)

// Correlation represents a non-causal statistical or temporal co-occurrence between signals.
type Correlation struct {
	PrimarySignal     string                `json:"primary_signal"`
	SecondarySignal   string                `json:"secondary_signal"`
	Coefficient       float64               `json:"coefficient"`
	TimeOffsetSeconds int                   `json:"time_offset_seconds"`
	CoOccurrenceCount int                   `json:"co_occurrence_count"`
	Confidence        CorrelationConfidence `json:"confidence"`
	Description       string                `json:"description"`
}

// IncidentStatus represents the state of a detected incident cluster.
type IncidentStatus string

const (
	IncidentStatusOpen      IncidentStatus = "open"
	IncidentStatusMitigated IncidentStatus = "mitigated"
	IncidentStatusResolved  IncidentStatus = "resolved"
)

// IncidentTimelineEvent represents a discrete event in an incident's chronological lifecycle.
type IncidentTimelineEvent struct {
	Timestamp   time.Time      `json:"timestamp"`
	NodeID      string         `json:"node_id,omitempty"`
	EventType   string         `json:"event_type"`
	Description string         `json:"description"`
	Severity    model.Severity `json:"severity"`
}

// Incident groups related multi-signal correlations, alerts, and anomalies.
type Incident struct {
	ID               string                  `json:"id"`
	Title            string                  `json:"title"`
	Status           IncidentStatus          `json:"status"`
	Severity         model.Severity          `json:"severity"`
	StartTime        time.Time               `json:"start_time"`
	EndTime          *time.Time              `json:"end_time,omitempty"`
	AffectedNodes    []string                `json:"affected_nodes"`
	PrimarySymptoms  []string                `json:"primary_symptoms"`
	RelatedAlerts    []model.AlertEvent      `json:"related_alerts"`
	RelatedAnomalies []string                `json:"related_anomalies"`
	Findings         []IntelligenceFinding   `json:"findings"`
	Timeline         []IncidentTimelineEvent `json:"timeline"`
}

// FindingCategory identifies the operational domain of an intelligence finding.
type FindingCategory string

const (
	FindingCategoryResourceExhaustion     FindingCategory = "resource_exhaustion"
	FindingCategoryPerformanceDegradation FindingCategory = "performance_degradation"
	FindingCategoryFleetPattern           FindingCategory = "fleet_pattern"
	FindingCategoryStabilityRisk          FindingCategory = "stability_risk"
	FindingCategoryAnomalyCluster         FindingCategory = "anomaly_cluster"

	// Phase 2B Predictive Finding Categories
	FindingCategoryCapacityRisk          FindingCategory = "capacity_risk"
	FindingCategoryPredictedDegradation  FindingCategory = "predicted_degradation"
	FindingCategoryThresholdForecast     FindingCategory = "threshold_forecast"
	FindingCategoryRecurringIncident     FindingCategory = "recurring_incident"
	FindingCategoryFleetCapacityPressure FindingCategory = "fleet_capacity_pressure"
	FindingCategoryAcceleratingResource  FindingCategory = "accelerating_resource_usage"
)

// PredictionConfidence rates confidence in a predictive threshold or capacity forecast.
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

// CapacityResource identifies the hardware subsystem being evaluated for capacity exhaustion.
type CapacityResource string

const (
	CapacityResourceCPU    CapacityResource = "cpu"
	CapacityResourceMemory CapacityResource = "memory"
	CapacityResourceSwap   CapacityResource = "swap"
	CapacityResourceDisk   CapacityResource = "disk"
)

// CapacityForecast represents a capacity projection for a single resource subsystem.
type CapacityForecast struct {
	Resource                         CapacityResource     `json:"resource"`
	Unit                             string               `json:"unit"`
	CurrentUtilization               float64              `json:"current_utilization"`
	BaselineUtilization              float64              `json:"baseline_utilization"`
	TrendSlopePerMinute              float64              `json:"trend_slope_per_minute"`
	WarningThreshold                 float64              `json:"warning_threshold"`
	CriticalThreshold                float64              `json:"critical_threshold"`
	TimeToWarning                    *time.Duration       `json:"time_to_warning,omitempty"`
	TimeToCritical                   *time.Duration       `json:"time_to_critical,omitempty"`
	Confidence                       PredictionConfidence `json:"confidence"`
	ProjectedUtilizationAfterHorizon float64              `json:"projected_utilization_after_horizon"`
	Horizon                          time.Duration        `json:"horizon"`
	Evidence                         []string             `json:"evidence"`
}

// NodeCapacityReport aggregates multi-resource capacity forecasts for a single node.
type NodeCapacityReport struct {
	NodeID      string             `json:"node_id"`
	Hostname    string             `json:"hostname"`
	Status      model.NodeStatus   `json:"status"`
	Forecasts   []CapacityForecast `json:"forecasts"`
	Predictions []Prediction       `json:"predictions"`
	EvaluatedAt time.Time          `json:"evaluated_at"`
}

// FleetCapacitySummary aggregates cluster-wide capacity risks, pressures, and forecasts.
type FleetCapacitySummary struct {
	EvaluatedAt              time.Time             `json:"evaluated_at"`
	TotalNodes               int                   `json:"total_nodes"`
	CPUPressurePercent       float64               `json:"cpu_pressure_percent"`
	MemoryPressurePercent    float64               `json:"memory_pressure_percent"`
	DiskPressurePercent      float64               `json:"disk_pressure_percent"`
	NodesApproachingWarning  int                   `json:"nodes_approaching_warning"`
	NodesApproachingCritical int                   `json:"nodes_approaching_critical"`
	TopCapacityRisks         []NodeCapacityReport  `json:"top_capacity_risks"`
	FleetPredictions         []Prediction          `json:"fleet_predictions"`
	Findings                 []IntelligenceFinding `json:"findings"`
}

// RecurrenceScope defines the scope at which recurrence is evaluated.
type RecurrenceScope string

const (
	RecurrenceScopeNode  RecurrenceScope = "node"
	RecurrenceScopeFleet RecurrenceScope = "fleet"
	RecurrenceScopeTag   RecurrenceScope = "tag"
)

// RecurrencePattern represents statistical recurrence of an incident or event type over time.
type RecurrencePattern struct {
	ID                     string               `json:"id"`
	Scope                  RecurrenceScope      `json:"scope"`
	TargetID               string               `json:"target_id"`
	EventType              string               `json:"event_type"`
	OccurrenceCount        int                  `json:"occurrence_count"`
	FirstOccurrence        time.Time            `json:"first_occurrence"`
	MostRecentOccurrence   time.Time            `json:"most_recent_occurrence"`
	AverageInterval        time.Duration        `json:"average_interval"`
	MedianInterval         time.Duration        `json:"median_interval"`
	CoefficientOfVariation float64              `json:"coefficient_of_variation"`
	Confidence             PredictionConfidence `json:"confidence"`
	RelatedIncidentIDs     []string             `json:"related_incident_ids"`
	Summary                string               `json:"summary"`
}

// FindingConfidence rates the confidence of an intelligence finding.
type FindingConfidence string

const (
	FindingConfidenceHigh   FindingConfidence = "high"
	FindingConfidenceMedium FindingConfidence = "medium"
	FindingConfidenceLow    FindingConfidence = "low"
)

// IntelligenceFinding represents an explainable advisory insight.
type IntelligenceFinding struct {
	ID                     string            `json:"id"`
	Category               FindingCategory   `json:"category"`
	Severity               model.Severity    `json:"severity"`
	Confidence             FindingConfidence `json:"confidence"`
	Title                  string            `json:"title"`
	Description            string            `json:"description"`
	AffectedNodes          []string          `json:"affected_nodes"`
	SupportingEvidence     []string          `json:"supporting_evidence"`
	NonInvasiveSuggestions []string          `json:"non_invasive_suggestions"`
	DetectedAt             time.Time         `json:"detected_at"`
}

// NodeHealthSummary aggregates intelligence findings, score, and trends for a single node.
type NodeHealthSummary struct {
	NodeID          string                `json:"node_id"`
	Hostname        string                `json:"hostname"`
	Status          model.NodeStatus      `json:"status"`
	HealthScore     HealthScore           `json:"health_score"`
	Trends          []HealthTrend         `json:"trends"`
	Baselines       []HistoricalBaseline  `json:"baselines"`
	ActiveIncidents []Incident            `json:"active_incidents"`
	Findings        []IntelligenceFinding `json:"findings"`
	EvaluatedAt     time.Time             `json:"evaluated_at"`
}

// FleetHealthSummary aggregates high-level fleet-wide health scores, trends, and incidents.
type FleetHealthSummary struct {
	EvaluatedAt        time.Time             `json:"evaluated_at"`
	TotalNodes         int                   `json:"total_nodes"`
	HealthyCount       int                   `json:"healthy_count"`
	WarningCount       int                   `json:"warning_count"`
	CriticalCount      int                   `json:"critical_count"`
	StaleCount         int                   `json:"stale_count"`
	OfflineCount       int                   `json:"offline_count"`
	AverageScore       float64               `json:"average_score"`
	LowestScoringNodes []NodeHealthSummary   `json:"lowest_scoring_nodes"`
	FleetTrends        []HealthTrend         `json:"fleet_trends"`
	ActiveIncidents    []Incident            `json:"active_incidents"`
	FleetFindings      []IntelligenceFinding `json:"fleet_findings"`
}
