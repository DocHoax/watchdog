package mcp

import (
	"strings"
	"testing"
	"time"
)

func TestMetrics(t *testing.T) {
	m := NewMetrics()

	m.RecordRequest("tools/call")
	m.RecordRequest("tools/call")
	m.RecordRequest("resources/read")

	m.RecordError("tools/call", "INVALID_ARGUMENT")
	m.RecordError("resources/read", "RESOURCE_NOT_FOUND")

	m.RecordToolCall("get_node_health", false)
	m.RecordToolCall("get_node_health", true)

	m.RecordDuration("tools/call", 125*time.Millisecond)

	prom := m.RenderPrometheus()

	expectedSubstrings := []string{
		`watchdog_mcp_requests_total{method="tools/call"} 2`,
		`watchdog_mcp_requests_total{method="resources/read"} 1`,
		`watchdog_mcp_errors_total{method="tools/call",error_code="INVALID_ARGUMENT"} 1`,
		`watchdog_mcp_errors_total{method="resources/read",error_code="RESOURCE_NOT_FOUND"} 1`,
		`watchdog_mcp_tool_calls_total{tool="get_node_health",status="success"} 1`,
		`watchdog_mcp_tool_calls_total{tool="get_node_health",status="error"} 1`,
		`watchdog_mcp_request_duration_seconds_total{method="tools/call"} 0.125000`,
	}

	for _, sub := range expectedSubstrings {
		if !strings.Contains(prom, sub) {
			t.Errorf("expected prometheus output to contain %q, but got:\n%s", sub, prom)
		}
	}
}
