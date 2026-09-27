package mcp

import (
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestValidateNodeID(t *testing.T) {
	tests := []struct {
		name    string
		nodeID  string
		wantErr bool
	}{
		{"valid alphanumeric", "node-01", false},
		{"valid uuid", "a1b2c3d4-e5f6-7890-abcd-ef1234567890", false},
		{"valid with dots and underscores", "worker_01.us-east.cluster", false},
		{"empty string", "", true},
		{"whitespace only", "   ", true},
		{"path traversal attempt", "../node-01", true},
		{"slash attempt", "node/01", true},
		{"backslash attempt", "node\\01", true},
		{"null byte", "node\x0001", true},
		{"special characters", "node@01!", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNodeID(tt.nodeID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNodeID(%q) error = %v, wantErr %v", tt.nodeID, err, tt.wantErr)
			}
		})
	}
}

func TestValidateLimit(t *testing.T) {
	if got := ValidateLimit(0, 50, 100); got != 50 {
		t.Errorf("ValidateLimit(0, 50, 100) = %d, want 50", got)
	}
	if got := ValidateLimit(-5, 50, 100); got != 50 {
		t.Errorf("ValidateLimit(-5, 50, 100) = %d, want 50", got)
	}
	if got := ValidateLimit(25, 50, 100); got != 25 {
		t.Errorf("ValidateLimit(25, 50, 100) = %d, want 25", got)
	}
	if got := ValidateLimit(200, 50, 100); got != 100 {
		t.Errorf("ValidateLimit(200, 50, 100) = %d, want 100", got)
	}
}

func TestValidateOffset(t *testing.T) {
	if got := ValidateOffset(-10); got != 0 {
		t.Errorf("ValidateOffset(-10) = %d, want 0", got)
	}
	if got := ValidateOffset(15); got != 15 {
		t.Errorf("ValidateOffset(15) = %d, want 15", got)
	}
}

func TestParseFlexibleDuration(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		defaultDur time.Duration
		want       time.Duration
		wantErr    bool
	}{
		{"empty string returns default", "", 1 * time.Hour, 1 * time.Hour, false},
		{"standard minutes", "15m", 1 * time.Hour, 15 * time.Minute, false},
		{"standard hours", "2h", 1 * time.Hour, 2 * time.Hour, false},
		{"days notation lowercase", "7d", 1 * time.Hour, 7 * 24 * time.Hour, false},
		{"days notation uppercase", "30D", 1 * time.Hour, 30 * 24 * time.Hour, false},
		{"invalid format", "invalid", 1 * time.Hour, 0, true},
		{"negative duration", "-5m", 1 * time.Hour, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFlexibleDuration(tt.input, tt.defaultDur)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFlexibleDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseFlexibleDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateStatus(t *testing.T) {
	tests := []struct {
		input   string
		want    model.NodeStatus
		wantErr bool
	}{
		{"", "", false},
		{"healthy", model.NodeStatusHealthy, false},
		{"ok", model.NodeStatusHealthy, false},
		{"warning", model.NodeStatusWarning, false},
		{"warn", model.NodeStatusWarning, false},
		{"critical", model.NodeStatusCritical, false},
		{"crit", model.NodeStatusCritical, false},
		{"stale", model.NodeStatusStale, false},
		{"offline", model.NodeStatusOffline, false},
		{"unknown", model.NodeStatusUnknown, false},
		{"invalid_status", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ValidateStatus(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateStatus(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ValidateStatus(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateSeverity(t *testing.T) {
	tests := []struct {
		input   string
		want    model.Severity
		wantErr bool
	}{
		{"", "", false},
		{"INFO", model.SeverityInfo, false},
		{"warning", model.SeverityWarning, false},
		{"CRITICAL", model.SeverityCritical, false},
		{"crit", model.SeverityCritical, false},
		{"invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ValidateSeverity(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSeverity(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ValidateSeverity(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateSortFieldAndDirection(t *testing.T) {
	if got := ValidateSortField("hostname"); got != "hostname" {
		t.Errorf("ValidateSortField(hostname) = %s, want hostname", got)
	}
	if got := ValidateSortField("cpu_usage"); got != "cpu" {
		t.Errorf("ValidateSortField(cpu_usage) = %s, want cpu", got)
	}
	if got := ValidateSortField("heartbeat"); got != "last_heartbeat" {
		t.Errorf("ValidateSortField(heartbeat) = %s, want last_heartbeat", got)
	}

	if got := ValidateSortDirection("DESC"); got != "desc" {
		t.Errorf("ValidateSortDirection(DESC) = %s, want desc", got)
	}
	if got := ValidateSortDirection("asc"); got != "asc" {
		t.Errorf("ValidateSortDirection(asc) = %s, want asc", got)
	}
}

func TestValidateMetricName(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"cpu", "cpu_usage_pct", false},
		{"memory", "memory_used_pct", false},
		{"disk", "disk_used_pct", false},
		{"load1", "load1", false},
		{"net_rx", "net_rx_bytes_sec", false},
		{"custom_metric", "custom_metric", false},
		{"", "", true},
		{"bad/metric/name!", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ValidateMetricName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMetricName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ValidateMetricName(%q) = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}
