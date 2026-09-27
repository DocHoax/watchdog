package mcp

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

var (
	nodeIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]{1,128}$`)
)

// ValidateNodeID verifies that a node ID conforms to security and format requirements.
func ValidateNodeID(nodeID string) error {
	trimmed := strings.TrimSpace(nodeID)
	if trimmed == "" {
		return errors.New("node_id cannot be empty")
	}
	if strings.Contains(trimmed, "..") || strings.ContainsAny(trimmed, "/\\ \t\r\n\x00") {
		return errors.New("node_id contains invalid characters")
	}
	if !nodeIDRegex.MatchString(trimmed) {
		return fmt.Errorf("node_id '%s' does not match allowed format (1-128 alphanumeric, dash, dot, or underscore)", trimmed)
	}
	return nil
}

// ValidateLimit clamps limit between 1 and maxLimit, applying defaultLimit if input <= 0.
func ValidateLimit(limit int, defaultLimit, maxLimit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

// ValidateOffset ensures offset is non-negative.
func ValidateOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

// ParseFlexibleDuration parses Go standard durations as well as days notation (e.g. "7d", "30d").
func ParseFlexibleDuration(dStr string, defaultDur time.Duration) (time.Duration, error) {
	trimmed := strings.TrimSpace(dStr)
	if trimmed == "" {
		return defaultDur, nil
	}

	// Handle 'd' or 'D' suffix for days
	if strings.HasSuffix(trimmed, "d") || strings.HasSuffix(trimmed, "D") {
		daysStr := trimmed[:len(trimmed)-1]
		days, err := strconv.ParseFloat(daysStr, 64)
		if err != nil || days <= 0 {
			return 0, fmt.Errorf("invalid days duration: %s", dStr)
		}
		return time.Duration(days * float64(24*time.Hour)), nil
	}

	d, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid duration format '%s': %w", dStr, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("duration must be positive: %s", dStr)
	}
	return d, nil
}

// ValidateStatus validates and normalizes node health status.
func ValidateStatus(status string) (model.NodeStatus, error) {
	trimmed := strings.ToLower(strings.TrimSpace(status))
	if trimmed == "" {
		return "", nil
	}

	switch trimmed {
	case "healthy", "ok", "pass":
		return model.NodeStatusHealthy, nil
	case "warning", "warn":
		return model.NodeStatusWarning, nil
	case "critical", "crit", "fail", "error":
		return model.NodeStatusCritical, nil
	case "stale":
		return model.NodeStatusStale, nil
	case "offline", "down":
		return model.NodeStatusOffline, nil
	case "unknown":
		return model.NodeStatusUnknown, nil
	default:
		return "", fmt.Errorf("invalid status '%s', expected one of: healthy, warning, critical, stale, offline, unknown", status)
	}
}

// ValidateSeverity validates and normalizes diagnostic or alert severity.
func ValidateSeverity(sev string) (model.Severity, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(sev))
	if trimmed == "" {
		return "", nil
	}

	switch trimmed {
	case "INFO":
		return model.SeverityInfo, nil
	case "WARNING", "WARN":
		return model.SeverityWarning, nil
	case "CRITICAL", "CRIT", "ERROR", "FATAL":
		return model.SeverityCritical, nil
	default:
		return "", fmt.Errorf("invalid severity '%s', expected one of: INFO, WARNING, CRITICAL", sev)
	}
}

// ValidateSortField normalizes and validates the sort field.
func ValidateSortField(field string) string {
	trimmed := strings.ToLower(strings.TrimSpace(field))
	switch trimmed {
	case "hostname", "host":
		return "hostname"
	case "last_heartbeat", "heartbeat", "time", "timestamp":
		return "last_heartbeat"
	case "cpu", "cpu_usage", "cpu_usage_pct":
		return "cpu"
	case "memory", "mem", "memory_used_pct":
		return "memory"
	case "status":
		return "status"
	default:
		return "hostname"
	}
}

// ValidateSortDirection normalizes sort direction to "asc" or "desc".
func ValidateSortDirection(dir string) string {
	trimmed := strings.ToLower(strings.TrimSpace(dir))
	if trimmed == "desc" || trimmed == "descending" {
		return "desc"
	}
	return "asc"
}

// ValidateMetricName validates and maps common metric aliases to canonical metric names.
func ValidateMetricName(metric string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(metric))
	if trimmed == "" {
		return "", errors.New("metric name cannot be empty")
	}

	switch trimmed {
	case "cpu", "cpu_usage", "cpu_usage_pct", "cpu_percent":
		return "cpu_usage_pct", nil
	case "memory", "mem", "memory_used_pct", "mem_percent", "memory_percent":
		return "memory_used_pct", nil
	case "memory_used_bytes", "mem_bytes":
		return "memory_used_bytes", nil
	case "swap_used_pct", "swap":
		return "swap_used_pct", nil
	case "disk", "disk_used_pct", "disk_percent":
		return "disk_used_pct", nil
	case "disk_read_bytes_sec", "disk_read":
		return "disk_read_bytes_sec", nil
	case "disk_write_bytes_sec", "disk_write":
		return "disk_write_bytes_sec", nil
	case "load1":
		return "load1", nil
	case "load5":
		return "load5", nil
	case "load15":
		return "load15", nil
	case "net_rx_bytes_sec", "net_rx", "network_rx":
		return "net_rx_bytes_sec", nil
	case "net_tx_bytes_sec", "net_tx", "network_tx":
		return "net_tx_bytes_sec", nil
	case "process_count", "processes":
		return "process_count", nil
	default:
		// Return trimmed if it matches safe alphanumeric/underscore format
		if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_\-]{1,64}$`, trimmed); matched {
			return trimmed, nil
		}
		return "", fmt.Errorf("invalid metric name '%s'", metric)
	}
}
