package storage

import (
	"context"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

// MetricPoint represents a single scalar time-series metric data point.
type MetricPoint struct {
	Timestamp time.Time         `json:"timestamp"`
	Metric    string            `json:"metric"`
	Value     float64           `json:"value"`
	Hostname  string            `json:"hostname,omitempty"`
	Tags      map[string]string `json:"tags,omitempty"`
}

// MetricAggregate represents aggregated statistics over a time window.
type MetricAggregate struct {
	Metric    string    `json:"metric"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Count     int64     `json:"count"`
	Min       float64   `json:"min"`
	Max       float64   `json:"max"`
	Avg       float64   `json:"avg"`
	Sum       float64   `json:"sum"`
	P50       float64   `json:"p50"`
	P90       float64   `json:"p90"`
	P99       float64   `json:"p99"`
	StdDev    float64   `json:"std_dev"`
}

// TimeRangeQuery defines parameters for querying metric time-series.
type TimeRangeQuery struct {
	Metric    string        `json:"metric"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Step      time.Duration `json:"step,omitempty"`
	Limit     int           `json:"limit,omitempty"`
}

// AuditFilter defines parameters for querying audit events.
type AuditFilter struct {
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
	EventType     string    `json:"event_type"`
	Severity      string    `json:"severity"`
	Outcome       string    `json:"outcome"`
	ActorType     string    `json:"actor_type"`
	ActorIdentity string    `json:"actor_identity"`
	SourceAddress string    `json:"source_address"`
	RequestID     string    `json:"request_id"`
	Limit         int       `json:"limit"`
	Offset        int       `json:"offset"`
}

// Storage defines the contract for storing and querying system metrics, alerts, diagnostics, and audit events.
type Storage interface {
	// Lifecycle
	Close() error
	Ping(ctx context.Context) error

	// Metrics
	SaveSnapshot(ctx context.Context, snapshot *model.SystemSnapshot) error
	SaveMetricPoint(ctx context.Context, pt MetricPoint) error
	SaveMetricPoints(ctx context.Context, pts []MetricPoint) error
	QueryMetrics(ctx context.Context, q TimeRangeQuery) ([]MetricPoint, error)
	GetMetricAggregate(ctx context.Context, metric string, start, end time.Time) (*MetricAggregate, error)
	GetAvailableMetrics(ctx context.Context) ([]string, error)

	// Alerts
	SaveAlertEvent(ctx context.Context, alert model.AlertEvent) error
	UpdateAlertStatus(ctx context.Context, id string, status model.AlertStatus, resolvedAt *time.Time) error
	GetAlertHistory(ctx context.Context, limit int, offset int) ([]model.AlertEvent, error)
	GetActiveAlerts(ctx context.Context) ([]model.AlertEvent, error)

	// Diagnostics
	SaveDiagnosticReport(ctx context.Context, report *model.DiagnosticReport) error
	GetLatestDiagnosticReport(ctx context.Context) (*model.DiagnosticReport, error)
	GetDiagnosticHistory(ctx context.Context, limit int) ([]model.DiagnosticReport, error)

	// Audit Events
	SaveAuditEvent(ctx context.Context, event model.AuditEvent) error
	SaveAuditEvents(ctx context.Context, events []model.AuditEvent) error
	QueryAuditEvents(ctx context.Context, filter AuditFilter) ([]model.AuditEvent, error)
	CountAuditEvents(ctx context.Context, filter AuditFilter) (int64, error)
	PruneAuditEvents(ctx context.Context, retention time.Duration) (int64, error)
	PurgeAuditEvents(ctx context.Context, before time.Time) (int64, error)

	// Retention & Maintenance
	PruneOlderThan(ctx context.Context, retention time.Duration) (int64, error)
	GetDatabaseSize() (int64, error)
	Vacuum(ctx context.Context) error
}
