package model

import "time"

// AnomalyScore describes statistical deviation.
type AnomalyScore struct {
	MetricName   string    `json:"metric_name" yaml:"metric_name"`
	CurrentValue float64   `json:"current_value" yaml:"current_value"`
	Mean         float64   `json:"mean" yaml:"mean"`
	StdDev       float64   `json:"std_dev" yaml:"std_dev"`
	ZScore       float64   `json:"z_score" yaml:"z_score"`
	EWMA         float64   `json:"ewma" yaml:"ewma"`
	DeviationPct float64   `json:"deviation_pct" yaml:"deviation_pct"`
	IsAnomaly    bool      `json:"is_anomaly" yaml:"is_anomaly"`
	Severity     Severity  `json:"severity" yaml:"severity"`
	Explanation  string    `json:"explanation" yaml:"explanation"`
	DetectedAt   time.Time `json:"detected_at" yaml:"detected_at"`
}

// AnomalyReport contains anomalies across tracked metrics.
type AnomalyReport struct {
	TotalChecked   int            `json:"total_checked" yaml:"total_checked"`
	AnomaliesCount int            `json:"anomalies_count" yaml:"anomalies_count"`
	Scores         []AnomalyScore `json:"scores" yaml:"scores"`
	GeneratedAt    time.Time      `json:"generated_at" yaml:"generated_at"`
}
