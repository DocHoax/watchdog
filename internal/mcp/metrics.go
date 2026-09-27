package mcp

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Metrics tracks operational metrics for the MCP server.
type Metrics struct {
	mu           sync.RWMutex
	requests     map[string]int64            // method -> count
	errors       map[string]map[string]int64 // method -> error_code -> count
	toolCalls    map[string]map[string]int64 // tool -> status ("success"|"error") -> count
	durations    map[string]time.Duration    // method -> total duration
	durationRuns map[string]int64            // method -> total measured runs
}

// NewMetrics creates an initialized Metrics tracker.
func NewMetrics() *Metrics {
	return &Metrics{
		requests:     make(map[string]int64),
		errors:       make(map[string]map[string]int64),
		toolCalls:    make(map[string]map[string]int64),
		durations:    make(map[string]time.Duration),
		durationRuns: make(map[string]int64),
	}
}

// RecordRequest increments the request counter for the specified JSON-RPC method.
func (m *Metrics) RecordRequest(method string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests[method]++
}

// RecordError increments the error counter for the specified method and error code string.
func (m *Metrics) RecordError(method string, errorCode string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.errors[method] == nil {
		m.errors[method] = make(map[string]int64)
	}
	m.errors[method][errorCode]++
}

// RecordToolCall increments the tool invocation counter by tool name and outcome.
func (m *Metrics) RecordToolCall(tool string, isError bool) {
	if m == nil {
		return
	}
	status := "success"
	if isError {
		status = "error"
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.toolCalls[tool] == nil {
		m.toolCalls[tool] = make(map[string]int64)
	}
	m.toolCalls[tool][status]++
}

// RecordDuration records execution time for an MCP method invocation.
func (m *Metrics) RecordDuration(method string, d time.Duration) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.durations[method] += d
	m.durationRuns[method]++
}

// RenderPrometheus renders the MCP metrics into Prometheus text format.
func (m *Metrics) RenderPrometheus() string {
	if m == nil {
		return ""
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sb strings.Builder

	// 1. watchdog_mcp_requests_total
	sb.WriteString("# HELP watchdog_mcp_requests_total Total number of MCP JSON-RPC requests\n")
	sb.WriteString("# TYPE watchdog_mcp_requests_total counter\n")
	methods := make([]string, 0, len(m.requests))
	for method := range m.requests {
		methods = append(methods, method)
	}
	sort.Strings(methods)
	for _, method := range methods {
		sb.WriteString(fmt.Sprintf("watchdog_mcp_requests_total{method=%q} %d\n", method, m.requests[method]))
	}

	// 2. watchdog_mcp_errors_total
	sb.WriteString("# HELP watchdog_mcp_errors_total Total number of MCP JSON-RPC errors\n")
	sb.WriteString("# TYPE watchdog_mcp_errors_total counter\n")
	errMethods := make([]string, 0, len(m.errors))
	for method := range m.errors {
		errMethods = append(errMethods, method)
	}
	sort.Strings(errMethods)
	for _, method := range errMethods {
		codeMap := m.errors[method]
		codes := make([]string, 0, len(codeMap))
		for code := range codeMap {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		for _, code := range codes {
			sb.WriteString(fmt.Sprintf("watchdog_mcp_errors_total{method=%q,error_code=%q} %d\n", method, code, codeMap[code]))
		}
	}

	// 3. watchdog_mcp_tool_calls_total
	sb.WriteString("# HELP watchdog_mcp_tool_calls_total Total number of MCP tool invocations\n")
	sb.WriteString("# TYPE watchdog_mcp_tool_calls_total counter\n")
	tools := make([]string, 0, len(m.toolCalls))
	for tool := range m.toolCalls {
		tools = append(tools, tool)
	}
	sort.Strings(tools)
	for _, tool := range tools {
		statuses := make([]string, 0, len(m.toolCalls[tool]))
		for status := range m.toolCalls[tool] {
			statuses = append(statuses, status)
		}
		sort.Strings(statuses)
		for _, status := range statuses {
			sb.WriteString(fmt.Sprintf("watchdog_mcp_tool_calls_total{tool=%q,status=%q} %d\n", tool, status, m.toolCalls[tool][status]))
		}
	}

	// 4. watchdog_mcp_request_duration_seconds
	sb.WriteString("# HELP watchdog_mcp_request_duration_seconds_total Total duration of MCP requests in seconds\n")
	sb.WriteString("# TYPE watchdog_mcp_request_duration_seconds_total counter\n")
	durMethods := make([]string, 0, len(m.durations))
	for method := range m.durations {
		durMethods = append(durMethods, method)
	}
	sort.Strings(durMethods)
	for _, method := range durMethods {
		sb.WriteString(fmt.Sprintf("watchdog_mcp_request_duration_seconds_total{method=%q} %.6f\n", method, m.durations[method].Seconds()))
	}

	return sb.String()
}
