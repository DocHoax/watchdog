package model

import "time"

// ReportData consolidates all system snapshot metrics for reporting.
type ReportData struct {
	Title        string            `json:"title" yaml:"title"`
	GeneratedAt  time.Time         `json:"generated_at" yaml:"generated_at"`
	Host         SystemInfo        `json:"host" yaml:"host"`
	CPU          CPUInfo           `json:"cpu" yaml:"cpu"`
	Memory       MemoryInfo        `json:"memory" yaml:"memory"`
	Disk         DiskInfo          `json:"disk" yaml:"disk"`
	Network      NetworkInfo       `json:"network" yaml:"network"`
	TopProcesses []ProcessInfo     `json:"top_processes" yaml:"top_processes"`
	Diagnostics  *DiagnosticReport `json:"diagnostics,omitempty" yaml:"diagnostics,omitempty"`
	ActiveAlerts []AlertEvent      `json:"active_alerts,omitempty" yaml:"active_alerts,omitempty"`
	Anomalies    *AnomalyReport    `json:"anomalies,omitempty" yaml:"anomalies,omitempty"`
	Docker       *DockerSummary    `json:"docker,omitempty" yaml:"docker,omitempty"`
	Kubernetes   *K8sSummary       `json:"kubernetes,omitempty" yaml:"kubernetes,omitempty"`
}
