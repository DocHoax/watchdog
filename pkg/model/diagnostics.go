package model

import "time"

// DiagnosticStatus represents the health check outcome.
type DiagnosticStatus string

const (
	StatusPass    DiagnosticStatus = "PASS"
	StatusWarning DiagnosticStatus = "WARNING"
	StatusFail    DiagnosticStatus = "CRITICAL"
	StatusInfo    DiagnosticStatus = "INFO"
	StatusSkip    DiagnosticStatus = "SKIPPED"
)

// Severity indicates urgency.
type Severity string

const (
	SeverityInfo     Severity = "INFO"
	SeverityWarning  Severity = "WARNING"
	SeverityCritical Severity = "CRITICAL"
)

// DiagnosticResult represents an individual health check evaluation.
type DiagnosticResult struct {
	ID             string           `json:"id" yaml:"id"`
	Category       string           `json:"category" yaml:"category"` // CPU, Memory, Disk, Network, DNS, Process, Docker, Kubernetes
	Name           string           `json:"name" yaml:"name"`
	Status         DiagnosticStatus `json:"status" yaml:"status"`
	Severity       Severity         `json:"severity" yaml:"severity"`
	Description    string           `json:"description" yaml:"description"`
	MetricValue    string           `json:"metric_value" yaml:"metric_value"`
	Threshold      string           `json:"threshold,omitempty" yaml:"threshold,omitempty"`
	Recommendation string           `json:"recommendation,omitempty" yaml:"recommendation,omitempty"`
	Timestamp      time.Time        `json:"timestamp" yaml:"timestamp"`
}

// DiagnosticReport contains all evaluation results.
type DiagnosticReport struct {
	OverallStatus  DiagnosticStatus   `json:"overall_status" yaml:"overall_status"`
	TotalChecks    int                `json:"total_checks" yaml:"total_checks"`
	PassedChecks   int                `json:"passed_checks" yaml:"passed_checks"`
	WarningChecks  int                `json:"warning_checks" yaml:"warning_checks"`
	CriticalChecks int                `json:"critical_checks" yaml:"critical_checks"`
	Results        []DiagnosticResult `json:"results" yaml:"results"`
	GeneratedAt    time.Time          `json:"generated_at" yaml:"generated_at"`
}
