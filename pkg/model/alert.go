package model

import "time"

// AlertRule defines conditions to trigger an alert.
type AlertRule struct {
	ID        string   `json:"id" yaml:"id"`
	Name      string   `json:"name" yaml:"name"`
	Metric    string   `json:"metric" yaml:"metric"` // cpu_usage, mem_usage, disk_usage, process_cpu
	Operator  string   `json:"operator" yaml:"operator"` // >, >=, <, <=, ==
	Threshold float64  `json:"threshold" yaml:"threshold"`
	Duration  string   `json:"duration" yaml:"duration"` // e.g., 30s, 1m
	Severity  Severity `json:"severity" yaml:"severity"` // INFO, WARNING, CRITICAL
	Cooldown  string   `json:"cooldown" yaml:"cooldown"` // e.g. 5m
	Enabled   bool     `json:"enabled" yaml:"enabled"`
}

// AlertEvent represents a fired alert.
type AlertEvent struct {
	ID          string    `json:"id" yaml:"id"`
	RuleID      string    `json:"rule_id" yaml:"rule_id"`
	RuleName    string    `json:"rule_name" yaml:"rule_name"`
	Severity    Severity  `json:"severity" yaml:"severity"`
	Message     string    `json:"message" yaml:"message"`
	MetricName  string    `json:"metric_name" yaml:"metric_name"`
	ActualValue float64   `json:"actual_value" yaml:"actual_value"`
	Threshold   float64   `json:"threshold" yaml:"threshold"`
	FiredAt     time.Time `json:"fired_at" yaml:"fired_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty" yaml:"resolved_at,omitempty"`
	IsActive    bool      `json:"is_active" yaml:"is_active"`
}
