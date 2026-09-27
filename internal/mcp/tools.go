package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/internal/alerts"
	"github.com/DocHoax/watchdog/internal/anomaly"
	"github.com/DocHoax/watchdog/internal/collector"
	"github.com/DocHoax/watchdog/internal/diagnostics"
	"github.com/DocHoax/watchdog/internal/fleet"
	"github.com/DocHoax/watchdog/internal/intelligence"
	"github.com/DocHoax/watchdog/internal/logger"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

// AllowedReadOperations defines the strict, static allowlist of permissible MCP read operations.
var AllowedReadOperations = map[string]bool{
	"list_nodes":                 true,
	"get_node":                   true,
	"get_node_health":            true,
	"get_node_snapshot":          true,
	"get_fleet_health":           true,
	"get_node_metrics":           true,
	"get_recent_diagnostics":     true,
	"get_active_alerts":          true,
	"get_fleet_intelligence":     true,
	"get_node_intelligence":      true,
	"get_fleet_incidents":        true,
	"get_intelligence_findings":  true,
	"get_node_trends":            true,
	"get_node_predictions":       true,
	"get_node_capacity_forecast": true,
	"get_fleet_predictions":      true,
	"get_recurring_incidents":    true,
}

// ToolRegistry manages and executes read-only MCP tools against domain services.
type ToolRegistry struct {
	fleetService  fleet.ReadOnlyFleetService
	storage       storage.ReadOnlyStorage
	collector     *collector.Manager
	diagnostics   *diagnostics.Engine
	alerts        *alerts.Engine
	localIdentity model.NodeIdentity
	intelSvc      intelligence.IntelligenceService
}

// NewToolRegistry creates a new ToolRegistry with injected domain dependencies.
func NewToolRegistry(
	fleetService fleet.ReadOnlyFleetService,
	store storage.ReadOnlyStorage,
	coll *collector.Manager,
	diag *diagnostics.Engine,
	alt *alerts.Engine,
	localID model.NodeIdentity,
	intelSvc ...intelligence.IntelligenceService,
) *ToolRegistry {
	if localID.NodeID == "" {
		localID.NodeID = "local-node"
		localID.Hostname = "localhost"
	}
	var is intelligence.IntelligenceService
	if len(intelSvc) > 0 {
		is = intelSvc[0]
	}
	return &ToolRegistry{
		fleetService:  fleetService,
		storage:       store,
		collector:     coll,
		diagnostics:   diag,
		alerts:        alt,
		localIdentity: localID,
		intelSvc:      is,
	}
}

// SetIntelligenceService sets or overrides the intelligence service instance.
func (r *ToolRegistry) SetIntelligenceService(svc intelligence.IntelligenceService) {
	r.intelSvc = svc
}

func (r *ToolRegistry) getIntelligenceService() intelligence.IntelligenceService {
	if r.intelSvc != nil {
		return r.intelSvc
	}
	return intelligence.NewService(r.storage, r.fleetService, anomaly.NewDetector(nil), logger.GetDefault(), nil)
}

// ToolDefinitions returns the full list of 17 supported read-only MCP tools with JSON schemas.
func ToolDefinitions() []Tool {
	minLimit := 1.0
	maxLimit100 := 100.0
	maxLimit200 := 200.0
	maxLimit1000 := 1000.0
	minOffset := 0.0

	return []Tool{
		{
			Name:        "list_nodes",
			Description: "Query registered fleet nodes with optional filtering by health status, search query, recent activity, and pagination.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"status": {
						Type:        "string",
						Description: "Filter nodes by operational health status",
						Enum:        []string{"healthy", "warning", "critical", "stale", "offline", "unknown"},
					},
					"search": {
						Type:        "string",
						Description: "Search query matching hostname, node_id, or IP address",
					},
					"since": {
						Type:        "string",
						Description: "Filter nodes active since duration (e.g. '15m', '1h', '24h', '7d')",
					},
					"sort_by": {
						Type:        "string",
						Description: "Field to sort results by",
						Enum:        []string{"hostname", "last_heartbeat", "cpu", "memory", "status"},
						Default:     "hostname",
					},
					"sort_direction": {
						Type:        "string",
						Description: "Sort direction ('asc' or 'desc')",
						Enum:        []string{"asc", "desc"},
						Default:     "asc",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of nodes to return (1-100, default: 50)",
						Minimum:     &minLimit,
						Maximum:     &maxLimit100,
						Default:     50,
					},
					"offset": {
						Type:        "integer",
						Description: "Pagination offset (default: 0)",
						Minimum:     &minOffset,
						Default:     0,
					},
				},
			},
		},
		{
			Name:        "get_node",
			Description: "Inspect a specific node's identity, hardware specifications, operating system, tags, registration time, and status.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"node_id": {
						Type:        "string",
						Description: "Unique identifier of the fleet node (1-128 alphanumeric, dash, dot, underscore)",
					},
				},
				Required: []string{"node_id"},
			},
		},
		{
			Name:        "get_node_health",
			Description: "Retrieve the current health assessment, status, key resource utilization percentages (CPU, RAM, Disk), diagnostic findings summary, and active alert count for a specific node.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"node_id": {
						Type:        "string",
						Description: "Unique identifier of the fleet node",
					},
				},
				Required: []string{"node_id"},
			},
		},
		{
			Name:        "get_node_snapshot",
			Description: "Retrieve a comprehensive point-in-time telemetry snapshot (system, CPU, memory, disks, network, processes, host info) for a specific node.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"node_id": {
						Type:        "string",
						Description: "Unique identifier of the fleet node",
					},
				},
				Required: []string{"node_id"},
			},
		},
		{
			Name:        "get_fleet_health",
			Description: "Retrieve an aggregated health overview of the entire monitored fleet, including node status counts, cluster resource averages, and active alert totals.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		{
			Name:        "get_node_metrics",
			Description: "Retrieve historical time-series metric data points (e.g. cpu_usage_pct, memory_used_pct, disk_used_pct, load1, load5, load15, net_rx_bytes_sec, net_tx_bytes_sec) for a node over a time range.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"node_id": {
						Type:        "string",
						Description: "Unique identifier of the fleet node",
					},
					"metric": {
						Type:        "string",
						Description: "Canonical metric name or alias (e.g. 'cpu', 'memory', 'disk', 'load1', 'load5', 'load15', 'net_rx', 'net_tx', 'processes')",
					},
					"since": {
						Type:        "string",
						Description: "Duration window to query (e.g. '15m', '1h', '6h', '24h', '7d', default: '1h')",
						Default:     "1h",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of historical points to return (1-1000, default: 100)",
						Minimum:     &minLimit,
						Maximum:     &maxLimit1000,
						Default:     100,
					},
				},
				Required: []string{"node_id", "metric"},
			},
		},
		{
			Name:        "get_recent_diagnostics",
			Description: "Retrieve recent heuristic diagnostic evaluation reports, issues, anomalies, and recommended remediation guidelines.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"node_id": {
						Type:        "string",
						Description: "Optional node ID to filter diagnostics for a specific node",
					},
					"severity": {
						Type:        "string",
						Description: "Filter diagnostic findings by severity ('INFO', 'WARNING', 'CRITICAL')",
						Enum:        []string{"INFO", "WARNING", "CRITICAL"},
					},
					"since": {
						Type:        "string",
						Description: "Duration window for historical diagnostic reports (e.g. '1h', '24h', '7d', default: '24h')",
						Default:     "24h",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of diagnostic reports to return (1-200, default: 50)",
						Minimum:     &minLimit,
						Maximum:     &maxLimit200,
						Default:     50,
					},
				},
			},
		},
		{
			Name:        "get_active_alerts",
			Description: "Retrieve currently firing and recently triggered alert incidents across the fleet or for a specific node.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"node_id": {
						Type:        "string",
						Description: "Optional node ID to filter active alerts for a specific node",
					},
					"severity": {
						Type:        "string",
						Description: "Filter alerts by severity level ('INFO', 'WARNING', 'CRITICAL')",
						Enum:        []string{"INFO", "WARNING", "CRITICAL"},
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of active alerts to return (1-100, default: 50)",
						Minimum:     &minLimit,
						Maximum:     &maxLimit100,
						Default:     50,
					},
				},
			},
		},
		{
			Name:        "get_fleet_intelligence",
			Description: "Retrieve comprehensive fleet health intelligence including 0-100 average scores, lowest-scoring nodes, fleet-wide metric trends, active clustered incidents, and correlated findings.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		{
			Name:        "get_node_intelligence",
			Description: "Retrieve deep explainable health intelligence for a specific node, including 0-100 score, factor deduction breakdown, trajectory, active incidents, trends, and findings.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"node_id": {
						Type:        "string",
						Description: "Unique identifier of the fleet node",
					},
				},
				Required: []string{"node_id"},
			},
		},
		{
			Name:        "get_fleet_incidents",
			Description: "Retrieve active clustered incidents across the fleet with root symptoms, affected nodes, and chronological timeline events, or a single incident by ID.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"incident_id": {
						Type:        "string",
						Description: "Optional unique ID of a specific incident to retrieve",
					},
				},
			},
		},
		{
			Name:        "get_intelligence_findings",
			Description: "Retrieve structured fleet and node intelligence findings filtered by category and minimum severity.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"category": {
						Type:        "string",
						Description: "Filter findings by category",
						Enum: []string{
							"resource_exhaustion", "performance_degradation", "fleet_pattern", "stability_risk", "anomaly_cluster",
							"capacity_risk", "predicted_degradation", "threshold_forecast", "recurring_incident", "fleet_capacity_pressure", "accelerating_resource_usage",
						},
					},
					"min_severity": {
						Type:        "string",
						Description: "Filter findings by minimum severity level ('INFO', 'WARNING', 'CRITICAL')",
						Enum:        []string{"INFO", "WARNING", "CRITICAL"},
					},
				},
			},
		},
		{
			Name:        "get_node_trends",
			Description: "Retrieve historical metric rate-of-change trends and statistical baselines (mean, stddev, percentiles) for a specific node.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"node_id": {
						Type:        "string",
						Description: "Unique identifier of the fleet node",
					},
					"window": {
						Type:        "string",
						Description: "Time window for trend evaluation (e.g. '15m', '1h', '6h', '24h', default: '1h')",
						Default:     "1h",
					},
				},
				Required: []string{"node_id"},
			},
		},
		{
			Name:        "get_node_predictions",
			Description: "Retrieve deterministic threshold crossing forecasts and metric trend predictions for a specific node.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"node_id": {
						Type:        "string",
						Description: "Unique identifier of the fleet node",
					},
					"horizon": {
						Type:        "string",
						Description: "Forecast horizon window (e.g. '15m', '1h', '6h', '24h', '7d', default: '24h')",
						Default:     "24h",
					},
				},
				Required: []string{"node_id"},
			},
		},
		{
			Name:        "get_node_capacity_forecast",
			Description: "Retrieve multi-subsystem capacity exhaustion analysis, runway, and projected utilization for CPU, memory, swap, and disk on a specific node.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"node_id": {
						Type:        "string",
						Description: "Unique identifier of the fleet node",
					},
					"horizon": {
						Type:        "string",
						Description: "Forecast horizon window (e.g. '15m', '1h', '6h', '24h', '7d', default: '24h')",
						Default:     "24h",
					},
				},
				Required: []string{"node_id"},
			},
		},
		{
			Name:        "get_fleet_predictions",
			Description: "Retrieve cluster-wide capacity intelligence, aggregated resource pressure percentages, and nodes approaching warning or critical capacity exhaustion thresholds.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"horizon": {
						Type:        "string",
						Description: "Forecast horizon window (e.g. '1h', '6h', '24h', '7d', default: '24h')",
						Default:     "24h",
					},
				},
			},
		},
		{
			Name:        "get_recurring_incidents",
			Description: "Retrieve detected historical incident recurrence patterns across nodes and fleet, including occurrence counts, mean/median intervals, and regularity coefficient of variation.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"since": {
						Type:        "string",
						Description: "Lookback window for recurrence analysis (e.g. '24h', '7d', '30d', default: '24h')",
						Default:     "24h",
					},
				},
			},
		},
	}
}

// GetTool returns the tool definition matching the specified name.
func GetTool(name string) (Tool, bool) {
	for _, t := range ToolDefinitions() {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}

// Execute handles the invocation of an MCP tool.
func (r *ToolRegistry) Execute(ctx context.Context, mcpCtx MCPContext, name string, args map[string]any) (*CallToolResult, *JSONRPCError) {
	if !AllowedReadOperations[name] {
		return nil, NewMethodNotFoundError(fmt.Sprintf("tool '%s' is not an allowed read-only operation", name))
	}

	if args == nil {
		args = make(map[string]any)
	}

	switch name {
	case "list_nodes":
		return r.handleListNodes(ctx, args)
	case "get_node":
		return r.handleGetNode(ctx, args)
	case "get_node_health":
		return r.handleGetNodeHealth(ctx, args)
	case "get_node_snapshot":
		return r.handleGetNodeSnapshot(ctx, args)
	case "get_fleet_health":
		return r.handleGetFleetHealth(ctx, args)
	case "get_node_metrics":
		return r.handleGetNodeMetrics(ctx, args)
	case "get_recent_diagnostics":
		return r.handleGetRecentDiagnostics(ctx, args)
	case "get_active_alerts":
		return r.handleGetActiveAlerts(ctx, args)
	case "get_fleet_intelligence":
		return r.handleGetFleetIntelligence(ctx, args)
	case "get_node_intelligence":
		return r.handleGetNodeIntelligence(ctx, args)
	case "get_fleet_incidents":
		return r.handleGetFleetIncidents(ctx, args)
	case "get_intelligence_findings":
		return r.handleGetIntelligenceFindings(ctx, args)
	case "get_node_trends":
		return r.handleGetNodeTrends(ctx, args)
	case "get_node_predictions":
		return r.handleGetNodePredictions(ctx, args)
	case "get_node_capacity_forecast":
		return r.handleGetNodeCapacityForecast(ctx, args)
	case "get_fleet_predictions":
		return r.handleGetFleetPredictions(ctx, args)
	case "get_recurring_incidents":
		return r.handleGetRecurringIncidents(ctx, args)
	default:
		return nil, NewMethodNotFoundError(fmt.Sprintf("tool '%s'", name))
	}
}

func (r *ToolRegistry) handleListNodes(ctx context.Context, args map[string]any) (*CallToolResult, *JSONRPCError) {
	statusStr := getStringArg(args, "status")
	status, err := ValidateStatus(statusStr)
	if err != nil {
		return nil, NewInvalidParamsError(err.Error())
	}

	search := getStringArg(args, "search")
	sinceStr := getStringArg(args, "since")
	var sinceTime time.Time
	if sinceStr != "" {
		dur, err := ParseFlexibleDuration(sinceStr, 0)
		if err != nil {
			return nil, NewInvalidParamsError(err.Error())
		}
		if dur > 0 {
			sinceTime = time.Now().Add(-dur)
		}
	}

	sortBy := ValidateSortField(getStringArg(args, "sort_by"))
	sortDir := ValidateSortDirection(getStringArg(args, "sort_direction"))
	limit := ValidateLimit(getIntArg(args, "limit", 50), 50, 100)
	offset := ValidateOffset(getIntArg(args, "offset", 0))

	if r.fleetService != nil {
		filter := model.FleetFilter{
			Status:        status,
			Search:        search,
			Since:         sinceTime,
			SortBy:        sortBy,
			SortDirection: sortDir,
			Limit:         limit,
			Offset:        offset,
		}
		resp, err := r.fleetService.ListNodes(ctx, filter)
		if err != nil {
			return nil, NewInternalError(err.Error())
		}
		return jsonResult(resp)
	}

	// Standalone fallback: Return local node
	node := model.FleetNode{
		Identity:      r.localIdentity,
		Status:        model.NodeStatusHealthy,
		RegisteredAt:  r.localIdentity.CreatedAt,
		LastHeartbeat: time.Now(),
		Summary: &model.NodeSummary{
			NodeID:   r.localIdentity.NodeID,
			Hostname: r.localIdentity.Hostname,
			Status:   model.NodeStatusHealthy,
			Version:  r.localIdentity.Version,
			Tags:     r.localIdentity.Tags,
		},
	}
	resp := model.FleetListResponse{
		Nodes:     []model.FleetNode{node},
		Total:     1,
		Count:     1,
		Limit:     limit,
		Offset:    offset,
		Timestamp: time.Now(),
		Summary: model.FleetSummary{
			TotalNodes:   1,
			HealthyNodes: 1,
			LastUpdated:  time.Now(),
		},
	}
	return jsonResult(resp)
}

func (r *ToolRegistry) handleGetNode(ctx context.Context, args map[string]any) (*CallToolResult, *JSONRPCError) {
	nodeID := getStringArg(args, "node_id")
	if err := ValidateNodeID(nodeID); err != nil {
		return nil, NewInvalidNodeIDError(err.Error())
	}

	if r.fleetService != nil {
		detail, err := r.fleetService.GetNode(ctx, nodeID)
		if err != nil {
			if err == fleet.ErrNodeNotFound {
				return nil, NewNodeNotFoundError(nodeID)
			}
			return nil, NewInternalError(err.Error())
		}
		return jsonResult(detail.Node)
	}

	// Standalone fallback
	if nodeID == r.localIdentity.NodeID || nodeID == "local" || nodeID == "self" || nodeID == "localhost" {
		node := model.FleetNode{
			Identity:      r.localIdentity,
			Status:        model.NodeStatusHealthy,
			RegisteredAt:  r.localIdentity.CreatedAt,
			LastHeartbeat: time.Now(),
		}
		return jsonResult(node)
	}

	return nil, NewNodeNotFoundError(nodeID)
}

func (r *ToolRegistry) handleGetNodeHealth(ctx context.Context, args map[string]any) (*CallToolResult, *JSONRPCError) {
	nodeID := getStringArg(args, "node_id")
	if err := ValidateNodeID(nodeID); err != nil {
		return nil, NewInvalidNodeIDError(err.Error())
	}

	if r.fleetService != nil {
		detail, err := r.fleetService.GetNode(ctx, nodeID)
		if err != nil {
			if err == fleet.ErrNodeNotFound {
				return nil, NewNodeNotFoundError(nodeID)
			}
			return nil, NewInternalError(err.Error())
		}

		healthSummary := map[string]any{
			"node_id":             detail.Node.Identity.NodeID,
			"hostname":            detail.Node.Identity.Hostname,
			"status":              detail.Node.Status,
			"status_message":      detail.Node.StatusMessage,
			"last_heartbeat":      detail.Node.LastHeartbeat,
			"last_telemetry":      detail.Node.LastTelemetry,
			"summary":             detail.Node.Summary,
			"active_alerts_count": len(detail.ActiveAlerts),
			"diagnostic_status":   "",
		}
		if detail.LatestDiagnostics != nil {
			healthSummary["diagnostic_status"] = detail.LatestDiagnostics.OverallStatus
		}
		return jsonResult(healthSummary)
	}

	// Standalone fallback
	var activeAlertsCount int
	if r.alerts != nil {
		activeAlertsCount = len(r.alerts.GetActiveAlerts())
	}

	var diagStatus model.DiagnosticStatus = model.StatusPass
	if r.storage != nil {
		if report, _ := r.storage.GetLatestDiagnosticReport(ctx); report != nil {
			diagStatus = report.OverallStatus
		}
	}

	health := map[string]any{
		"node_id":             r.localIdentity.NodeID,
		"hostname":            r.localIdentity.Hostname,
		"status":              model.NodeStatusHealthy,
		"last_heartbeat":      time.Now(),
		"active_alerts_count": activeAlertsCount,
		"diagnostic_status":   diagStatus,
	}
	return jsonResult(health)
}

func (r *ToolRegistry) handleGetNodeSnapshot(ctx context.Context, args map[string]any) (*CallToolResult, *JSONRPCError) {
	nodeID := getStringArg(args, "node_id")
	if err := ValidateNodeID(nodeID); err != nil {
		return nil, NewInvalidNodeIDError(err.Error())
	}

	if r.fleetService != nil {
		detail, err := r.fleetService.GetNode(ctx, nodeID)
		if err != nil {
			if err == fleet.ErrNodeNotFound {
				return nil, NewNodeNotFoundError(nodeID)
			}
			return nil, NewInternalError(err.Error())
		}
		if detail.LatestSnapshot != nil {
			return jsonResult(detail.LatestSnapshot)
		}
		return jsonResult(map[string]string{"message": "no telemetry snapshot available for node"})
	}

	// Standalone fallback: Collect live snapshot
	if r.collector != nil {
		snap, err := r.collector.CollectAll(ctx)
		if err != nil {
			return nil, NewInternalError(fmt.Sprintf("failed to collect system snapshot: %v", err))
		}
		return jsonResult(snap)
	}

	return nil, NewNodeNotFoundError(nodeID)
}

func (r *ToolRegistry) handleGetFleetHealth(ctx context.Context, _ map[string]any) (*CallToolResult, *JSONRPCError) {
	if r.fleetService != nil {
		summary, err := r.fleetService.GetFleetSummary(ctx)
		if err != nil {
			return nil, NewInternalError(err.Error())
		}
		return jsonResult(summary)
	}

	// Standalone summary
	summary := model.FleetSummary{
		TotalNodes:   1,
		HealthyNodes: 1,
		LastUpdated:  time.Now(),
	}
	return jsonResult(summary)
}

func (r *ToolRegistry) handleGetNodeMetrics(ctx context.Context, args map[string]any) (*CallToolResult, *JSONRPCError) {
	nodeID := getStringArg(args, "node_id")
	if err := ValidateNodeID(nodeID); err != nil {
		return nil, NewInvalidNodeIDError(err.Error())
	}

	metricRaw := getStringArg(args, "metric")
	metricName, err := ValidateMetricName(metricRaw)
	if err != nil {
		return nil, NewInvalidParamsError(err.Error())
	}

	sinceStr := getStringArg(args, "since")
	dur, err := ParseFlexibleDuration(sinceStr, time.Hour)
	if err != nil {
		return nil, NewInvalidParamsError(err.Error())
	}

	limit := ValidateLimit(getIntArg(args, "limit", 100), 100, 1000)

	if r.storage != nil {
		endTime := time.Now()
		startTime := endTime.Add(-dur)

		q := storage.TimeRangeQuery{
			Metric:    metricName,
			StartTime: startTime,
			EndTime:   endTime,
			Limit:     limit,
		}
		points, err := r.storage.QueryMetrics(ctx, q)
		if err != nil {
			return nil, NewInternalError(err.Error())
		}
		return jsonResult(points)
	}

	return jsonResult([]storage.MetricPoint{})
}

func (r *ToolRegistry) handleGetRecentDiagnostics(ctx context.Context, args map[string]any) (*CallToolResult, *JSONRPCError) {
	sevStr := getStringArg(args, "severity")
	sev, err := ValidateSeverity(sevStr)
	if err != nil {
		return nil, NewInvalidParamsError(err.Error())
	}

	limit := ValidateLimit(getIntArg(args, "limit", 50), 50, 200)

	if r.storage != nil {
		reports, err := r.storage.GetDiagnosticHistory(ctx, limit)
		if err != nil {
			return nil, NewInternalError(err.Error())
		}

		if sev != "" {
			var filtered []model.DiagnosticReport
			for _, rep := range reports {
				var matchingResults []model.DiagnosticResult
				for _, res := range rep.Results {
					if strings.EqualFold(string(res.Severity), string(sev)) {
						matchingResults = append(matchingResults, res)
					}
				}
				if len(matchingResults) > 0 {
					rep.Results = matchingResults
					filtered = append(filtered, rep)
				}
			}
			return jsonResult(filtered)
		}
		return jsonResult(reports)
	}

	if r.diagnostics != nil && r.collector != nil {
		snap, err := r.collector.CollectAll(ctx)
		if err == nil {
			report, err := r.diagnostics.Run(ctx, snap)
			if err == nil && report != nil {
				return jsonResult([]model.DiagnosticReport{*report})
			}
		}
	}

	return jsonResult([]model.DiagnosticReport{})
}

func (r *ToolRegistry) handleGetActiveAlerts(ctx context.Context, args map[string]any) (*CallToolResult, *JSONRPCError) {
	sevStr := getStringArg(args, "severity")
	sev, err := ValidateSeverity(sevStr)
	if err != nil {
		return nil, NewInvalidParamsError(err.Error())
	}

	limit := ValidateLimit(getIntArg(args, "limit", 50), 50, 100)

	var active []model.AlertEvent
	if r.alerts != nil {
		active = r.alerts.GetActiveAlerts()
	} else if r.storage != nil {
		var err error
		active, err = r.storage.GetActiveAlerts(ctx)
		if err != nil {
			return nil, NewInternalError(err.Error())
		}
	}

	if sev != "" {
		var filtered []model.AlertEvent
		for _, a := range active {
			if strings.EqualFold(string(a.Severity), string(sev)) {
				filtered = append(filtered, a)
			}
		}
		active = filtered
	}

	if len(active) > limit {
		active = active[:limit]
	}

	return jsonResult(active)
}

func (r *ToolRegistry) handleGetFleetIntelligence(ctx context.Context, _ map[string]any) (*CallToolResult, *JSONRPCError) {
	svc := r.getIntelligenceService()
	summary, err := svc.EvaluateFleetHealth(ctx)
	if err != nil {
		return nil, NewInternalError(fmt.Sprintf("failed to evaluate fleet intelligence: %v", err))
	}
	return jsonResult(summary)
}

func (r *ToolRegistry) handleGetNodeIntelligence(ctx context.Context, args map[string]any) (*CallToolResult, *JSONRPCError) {
	nodeID := getStringArg(args, "node_id")
	if err := ValidateNodeID(nodeID); err != nil {
		return nil, NewInvalidNodeIDError(err.Error())
	}
	svc := r.getIntelligenceService()
	summary, err := svc.EvaluateNodeHealth(ctx, nodeID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			return nil, NewNodeNotFoundError(nodeID)
		}
		return nil, NewInternalError(fmt.Sprintf("failed to evaluate node intelligence for '%s': %v", nodeID, err))
	}
	return jsonResult(summary)
}

func (r *ToolRegistry) handleGetFleetIncidents(ctx context.Context, args map[string]any) (*CallToolResult, *JSONRPCError) {
	svc := r.getIntelligenceService()
	incidentID := getStringArg(args, "incident_id")
	if incidentID != "" {
		inc, err := svc.GetIncident(ctx, incidentID)
		if err != nil {
			return nil, NewInternalError(fmt.Sprintf("failed to retrieve incident '%s': %v", incidentID, err))
		}
		if inc == nil {
			return nil, NewInvalidParamsError(fmt.Sprintf("incident '%s' not found", incidentID))
		}
		return jsonResult(inc)
	}

	incidents, err := svc.GetActiveIncidents(ctx)
	if err != nil {
		return nil, NewInternalError(fmt.Sprintf("failed to retrieve active incidents: %v", err))
	}
	return jsonResult(incidents)
}

func (r *ToolRegistry) handleGetIntelligenceFindings(ctx context.Context, args map[string]any) (*CallToolResult, *JSONRPCError) {
	svc := r.getIntelligenceService()
	catStr := getStringArg(args, "category")
	category := intelligence.FindingCategory(catStr)

	sevStr := getStringArg(args, "min_severity")
	var minSev model.Severity
	if sevStr != "" {
		s, err := ValidateSeverity(sevStr)
		if err != nil {
			return nil, NewInvalidParamsError(err.Error())
		}
		minSev = model.Severity(s)
	}

	findings, err := svc.GetFindings(ctx, category, minSev)
	if err != nil {
		return nil, NewInternalError(fmt.Sprintf("failed to retrieve intelligence findings: %v", err))
	}
	return jsonResult(findings)
}

func (r *ToolRegistry) handleGetNodeTrends(ctx context.Context, args map[string]any) (*CallToolResult, *JSONRPCError) {
	nodeID := getStringArg(args, "node_id")
	if err := ValidateNodeID(nodeID); err != nil {
		return nil, NewInvalidNodeIDError(err.Error())
	}

	windowStr := getStringArg(args, "window")
	window, err := ParseFlexibleDuration(windowStr, time.Hour)
	if err != nil {
		return nil, NewInvalidParamsError(err.Error())
	}

	svc := r.getIntelligenceService()
	trends, err := svc.GetNodeTrends(ctx, nodeID, window)
	if err != nil {
		return nil, NewInternalError(fmt.Sprintf("failed to evaluate trends for node '%s': %v", nodeID, err))
	}

	baselines, err := svc.GetNodeBaselines(ctx, nodeID, window)
	if err != nil {
		return nil, NewInternalError(fmt.Sprintf("failed to evaluate baselines for node '%s': %v", nodeID, err))
	}

	result := map[string]any{
		"node_id":   nodeID,
		"window":    window.String(),
		"trends":    trends,
		"baselines": baselines,
	}
	return jsonResult(result)
}

func (r *ToolRegistry) handleGetNodePredictions(ctx context.Context, args map[string]any) (*CallToolResult, *JSONRPCError) {
	nodeID := getStringArg(args, "node_id")
	if err := ValidateNodeID(nodeID); err != nil {
		return nil, NewInvalidNodeIDError(err.Error())
	}

	horizonStr := getStringArg(args, "horizon")
	horizon, err := ParseFlexibleDuration(horizonStr, 24*time.Hour)
	if err != nil {
		return nil, NewInvalidParamsError(err.Error())
	}

	svc := r.getIntelligenceService()
	predictions, err := svc.GetNodePredictions(ctx, nodeID, horizon)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			return nil, NewNodeNotFoundError(nodeID)
		}
		return nil, NewInternalError(fmt.Sprintf("failed to get predictions for node '%s': %v", nodeID, err))
	}

	return jsonResult(predictions)
}

func (r *ToolRegistry) handleGetNodeCapacityForecast(ctx context.Context, args map[string]any) (*CallToolResult, *JSONRPCError) {
	nodeID := getStringArg(args, "node_id")
	if err := ValidateNodeID(nodeID); err != nil {
		return nil, NewInvalidNodeIDError(err.Error())
	}

	horizonStr := getStringArg(args, "horizon")
	horizon, err := ParseFlexibleDuration(horizonStr, 24*time.Hour)
	if err != nil {
		return nil, NewInvalidParamsError(err.Error())
	}

	svc := r.getIntelligenceService()
	forecast, err := svc.GetNodeCapacityForecast(ctx, nodeID, horizon)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			return nil, NewNodeNotFoundError(nodeID)
		}
		return nil, NewInternalError(fmt.Sprintf("failed to get capacity forecast for node '%s': %v", nodeID, err))
	}

	return jsonResult(forecast)
}

func (r *ToolRegistry) handleGetFleetPredictions(ctx context.Context, args map[string]any) (*CallToolResult, *JSONRPCError) {
	horizonStr := getStringArg(args, "horizon")
	horizon, err := ParseFlexibleDuration(horizonStr, 24*time.Hour)
	if err != nil {
		return nil, NewInvalidParamsError(err.Error())
	}

	svc := r.getIntelligenceService()
	fleetCap, err := svc.GetFleetPredictions(ctx, horizon)
	if err != nil {
		return nil, NewInternalError(fmt.Sprintf("failed to get fleet predictions: %v", err))
	}

	return jsonResult(fleetCap)
}

func (r *ToolRegistry) handleGetRecurringIncidents(ctx context.Context, args map[string]any) (*CallToolResult, *JSONRPCError) {
	sinceStr := getStringArg(args, "since")
	since, err := ParseFlexibleDuration(sinceStr, 24*time.Hour)
	if err != nil {
		return nil, NewInvalidParamsError(err.Error())
	}

	svc := r.getIntelligenceService()
	recurring, err := svc.GetRecurringIncidents(ctx, since)
	if err != nil {
		return nil, NewInternalError(fmt.Sprintf("failed to get recurring incidents: %v", err))
	}

	return jsonResult(recurring)
}

func jsonResult(v any) (*CallToolResult, *JSONRPCError) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, NewInternalError(fmt.Sprintf("failed to marshal tool output: %v", err))
	}
	return &CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: string(b),
			},
		},
	}, nil
}

func getStringArg(args map[string]any, key string) string {
	if val, ok := args[key]; ok {
		if s, ok := val.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func getIntArg(args map[string]any, key string, def int) int {
	if val, ok := args[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		case json.Number:
			if i, err := v.Int64(); err == nil {
				return int(i)
			}
		}
	}
	return def
}
