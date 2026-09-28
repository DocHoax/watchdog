package mcp

import (
	"fmt"
	"strings"
)

// PromptRegistry manages available prompt templates for MCP clients.
type PromptRegistry struct{}

// NewPromptRegistry creates a new PromptRegistry.
func NewPromptRegistry() *PromptRegistry {
	return &PromptRegistry{}
}

// ListPrompts returns all available prompt templates.
func (p *PromptRegistry) ListPrompts() []Prompt {
	return []Prompt{
		{
			Name:        "system_health_audit",
			Description: "Comprehensive system diagnostic and health inspection across CPU, memory, disk, network, and active alerts.",
			Arguments: []PromptArgument{
				{
					Name:        "severity",
					Description: "Minimum severity filter for diagnostics (INFO, WARNING, CRITICAL)",
					Required:    false,
				},
			},
		},
		{
			Name:        "diagnose_node",
			Description: "Investigate a specific fleet node's telemetry, health status, and active anomalies.",
			Arguments: []PromptArgument{
				{
					Name:        "node_id",
					Description: "Identifier of the target fleet node to diagnose",
					Required:    true,
				},
			},
		},
		{
			Name:        "incident_triage",
			Description: "Triage active alerts and anomalies across the fleet for incident response.",
			Arguments: []PromptArgument{
				{
					Name:        "time_window",
					Description: "Time window for triage history (e.g. 1h, 24h, 7d)",
					Required:    false,
				},
			},
		},
		{
			Name:        "fleet_status_report",
			Description: "Generate an executive fleet health and resource capacity summary.",
			Arguments: []PromptArgument{
				{
					Name:        "tag",
					Description: "Filter fleet nodes by specific tag key=value",
					Required:    false,
				},
			},
		},
		{
			Name:        "analyze_fleet_health",
			Description: "Assess fleet-wide health intelligence, score trajectories, active incidents, and systemic patterns.",
			Arguments:   []PromptArgument{},
		},
		{
			Name:        "investigate_incident",
			Description: "Conduct a deep-dive investigation into a clustered fleet incident and its chronological timeline.",
			Arguments: []PromptArgument{
				{
					Name:        "incident_id",
					Description: "Identifier of the active clustered incident to investigate",
					Required:    true,
				},
			},
		},
		{
			Name:        "triage_node_degradation",
			Description: "Triage a degrading node using explainable factor deductions, metric rate-of-change trends, and baselines.",
			Arguments: []PromptArgument{
				{
					Name:        "node_id",
					Description: "Identifier of the target node to triage",
					Required:    true,
				},
			},
		},
		{
			Name:        "forecast_node_capacity",
			Description: "Forecast multi-subsystem resource runway and threshold exhaustion for a specific fleet node.",
			Arguments: []PromptArgument{
				{
					Name:        "node_id",
					Description: "Identifier of the target fleet node to forecast",
					Required:    true,
				},
				{
					Name:        "horizon",
					Description: "Forecast horizon window (e.g. 15m, 1h, 6h, 24h, 7d)",
					Required:    false,
				},
			},
		},
		{
			Name:        "analyze_fleet_capacity",
			Description: "Evaluate fleet-wide capacity pressure, top capacity risks, and multi-node threshold predictions.",
			Arguments: []PromptArgument{
				{
					Name:        "horizon",
					Description: "Forecast horizon window across the fleet (e.g. 1h, 6h, 24h, 7d)",
					Required:    false,
				},
			},
		},
		{
			Name:        "investigate_recurring_incidents",
			Description: "Analyze statistical recurrence patterns, inter-arrival intervals, and periodic flapping incidents across the fleet.",
			Arguments: []PromptArgument{
				{
					Name:        "since",
					Description: "Lookback window for recurrence analysis (e.g. 24h, 7d)",
					Required:    false,
				},
			},
		},
		{
			Name:        "topology_spof_analysis",
			Description: "Identify single points of failure (SPOFs) and evaluate component blast radius across the service topology.",
			Arguments: []PromptArgument{
				{
					Name:        "min_criticality",
					Description: "Minimum criticality threshold (0-100) for SPOF filtering",
					Required:    false,
				},
			},
		},
		{
			Name:        "root_cause_analysis",
			Description: "Perform topological and temporal root cause analysis for an incident with causal chain discovery and factor contributions.",
			Arguments: []PromptArgument{
				{
					Name:        "incident_id",
					Description: "Identifier of the incident to analyze root cause for",
					Required:    true,
				},
			},
		},
	}
}

// GetPrompt builds and returns the prompt messages for a given prompt name and arguments.
func (p *PromptRegistry) GetPrompt(name string, args map[string]string) (*GetPromptResult, *JSONRPCError) {
	trimmed := strings.TrimSpace(name)
	switch trimmed {
	case "system_health_audit":
		severity := "WARNING"
		if s, ok := args["severity"]; ok && strings.TrimSpace(s) != "" {
			severity = strings.ToUpper(strings.TrimSpace(s))
		}
		return &GetPromptResult{
			Description: "Perform a comprehensive system health audit",
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: PromptContent{
						Type: "text",
						Text: fmt.Sprintf(
							"Please perform a comprehensive system health audit on the Watchdog host.\n"+
								"1. Call the `get_node_health` tool to inspect current CPU, memory, disk, and load metrics.\n"+
								"2. Call the `get_recent_diagnostics` tool with severity=%q to inspect diagnostic rule evaluations.\n"+
								"3. Call the `get_active_alerts` tool to see any active firing alerts.\n"+
								"4. Synthesize your findings into a structured health report with risk level, identified bottlenecks, and recommended remediations.",
							severity,
						),
					},
				},
			},
		}, nil

	case "diagnose_node":
		nodeID, ok := args["node_id"]
		if !ok || strings.TrimSpace(nodeID) == "" {
			return nil, NewInvalidParamsError("missing required argument 'node_id'")
		}
		nodeID = strings.TrimSpace(nodeID)
		if err := ValidateNodeID(nodeID); err != nil {
			return nil, NewInvalidNodeIDError(err.Error())
		}
		return &GetPromptResult{
			Description: fmt.Sprintf("Diagnose fleet node %s", nodeID),
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: PromptContent{
						Type: "text",
						Text: fmt.Sprintf(
							"Please diagnose fleet node %q:\n"+
								"1. Call `get_node` with node_id=%q to retrieve node identity and specs.\n"+
								"2. Call `get_node_health` with node_id=%q to check current telemetry and resource utilization.\n"+
								"3. Call `get_node_snapshot` with node_id=%q to inspect detailed process, filesystem, and network state.\n"+
								"4. Call `get_node_metrics` for node_id=%q with metric=\"cpu\" and metric=\"memory\" over the last 1 hour.\n"+
								"5. Provide a root-cause analysis of any anomalies or degradation observed.",
							nodeID, nodeID, nodeID, nodeID, nodeID,
						),
					},
				},
			},
		}, nil

	case "incident_triage":
		timeWindow := "1h"
		if tw, ok := args["time_window"]; ok && strings.TrimSpace(tw) != "" {
			timeWindow = strings.TrimSpace(tw)
		}
		return &GetPromptResult{
			Description: "Triage active alerts and anomalies across the fleet",
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: PromptContent{
						Type: "text",
						Text: fmt.Sprintf(
							"Please triage active incidents across the fleet:\n"+
								"1. Call `get_active_alerts` to retrieve all currently firing alerts across nodes.\n"+
								"2. Call `get_fleet_health` to evaluate overall cluster degradation.\n"+
								"3. Call `get_recent_diagnostics` with severity=\"CRITICAL\" and since=%q.\n"+
								"4. Prioritize incidents by blast radius and severity, providing immediate mitigation steps.",
							timeWindow,
						),
					},
				},
			},
		}, nil

	case "fleet_status_report":
		tagFilter := ""
		if t, ok := args["tag"]; ok && strings.TrimSpace(t) != "" {
			tagFilter = fmt.Sprintf(" filtering by tag %q", strings.TrimSpace(t))
		}
		return &GetPromptResult{
			Description: "Generate an executive fleet health and resource capacity summary",
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: PromptContent{
						Type: "text",
						Text: fmt.Sprintf(
							"Please generate an executive fleet status report%s:\n"+
								"1. Call `get_fleet_health` to gather aggregate node counts and fleet averages.\n"+
								"2. Call `list_nodes` with limit=100 to review individual node statuses.\n"+
								"3. Identify any nodes in warning, critical, stale, or offline states.\n"+
								"4. Produce an executive summary with capacity utilization, fleet availability percentage, and operational recommendations.",
							tagFilter,
						),
					},
				},
			},
		}, nil

	case "analyze_fleet_health":
		return &GetPromptResult{
			Description: "Assess fleet-wide health intelligence and prioritize degraded nodes",
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: PromptContent{
						Type: "text",
						Text: "Please perform an intelligence-driven fleet health analysis:\n" +
							"1. Call `get_fleet_intelligence` to evaluate aggregate health score (0-100), trajectories, lowest scoring nodes, and fleet trends.\n" +
							"2. Call `get_fleet_incidents` to review active clustered incidents across nodes.\n" +
							"3. Call `get_intelligence_findings` to check for multi-node systemic patterns, resource exhaustion risks, and anomaly clusters.\n" +
							"4. Synthesize an explainable health overview highlighting degraded nodes, primary factor deductions, and observational recommendations.",
					},
				},
			},
		}, nil

	case "investigate_incident":
		incidentID, ok := args["incident_id"]
		if !ok || strings.TrimSpace(incidentID) == "" {
			return nil, NewInvalidParamsError("missing required argument 'incident_id'")
		}
		incidentID = strings.TrimSpace(incidentID)
		return &GetPromptResult{
			Description: fmt.Sprintf("Investigate clustered fleet incident %s", incidentID),
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: PromptContent{
						Type: "text",
						Text: fmt.Sprintf(
							"Please investigate incident %q:\n"+
								"1. Call `get_fleet_incidents` to locate incident %q and review its affected nodes, primary symptoms, and chronological timeline.\n"+
								"2. For each affected node, call `get_node_intelligence` to analyze factor deductions and active score trajectory.\n"+
								"3. Call `get_node_trends` for the affected nodes to check metric rates of change and baseline deviations.\n"+
								"4. Provide a non-causal chronological timeline analysis and observational findings.",
							incidentID, incidentID,
						),
					},
				},
			},
		}, nil

	case "triage_node_degradation":
		nodeID, ok := args["node_id"]
		if !ok || strings.TrimSpace(nodeID) == "" {
			return nil, NewInvalidParamsError("missing required argument 'node_id'")
		}
		nodeID = strings.TrimSpace(nodeID)
		if err := ValidateNodeID(nodeID); err != nil {
			return nil, NewInvalidNodeIDError(err.Error())
		}
		return &GetPromptResult{
			Description: fmt.Sprintf("Triage degradation on node %s", nodeID),
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: PromptContent{
						Type: "text",
						Text: fmt.Sprintf(
							"Please triage health degradation on node %q:\n"+
								"1. Call `get_node_intelligence` with node_id=%q to inspect the 0-100 health score, score trajectory, and granular factor deductions.\n"+
								"2. Call `get_node_trends` with node_id=%q to evaluate metric slope directions, rates of change, and baseline percentiles.\n"+
								"3. Call `get_node_snapshot` with node_id=%q to inspect current process table and disk partition states.\n"+
								"4. Explain the primary contributors to health score deduction and suggest non-invasive verification steps.",
							nodeID, nodeID, nodeID, nodeID,
						),
					},
				},
			},
		}, nil

	case "forecast_node_capacity":
		nodeID, ok := args["node_id"]
		if !ok || strings.TrimSpace(nodeID) == "" {
			return nil, NewInvalidParamsError("missing required argument 'node_id'")
		}
		nodeID = strings.TrimSpace(nodeID)
		if err := ValidateNodeID(nodeID); err != nil {
			return nil, NewInvalidNodeIDError(err.Error())
		}
		horizon := "24h"
		if h, ok := args["horizon"]; ok && strings.TrimSpace(h) != "" {
			horizon = strings.TrimSpace(h)
		}
		return &GetPromptResult{
			Description: fmt.Sprintf("Forecast multi-subsystem capacity for node %s", nodeID),
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: PromptContent{
						Type: "text",
						Text: fmt.Sprintf(
							"Please forecast multi-subsystem capacity runway for node %q over horizon %q:\n"+
								"1. Call `get_node_capacity_forecast` with node_id=%q and horizon=%q to evaluate CPU, memory, swap, and disk exhaustion runways.\n"+
								"2. Call `get_node_predictions` with node_id=%q and horizon=%q to evaluate threshold crossing predictions and regression quality (R²).\n"+
								"3. Call `get_node_trends` with node_id=%q to inspect short-term rate of change.\n"+
								"4. Summarize critical resources approaching exhaustion, confidence tiers, and recommend non-invasive capacity planning actions.",
							nodeID, horizon, nodeID, horizon, nodeID, horizon, nodeID,
						),
					},
				},
			},
		}, nil

	case "analyze_fleet_capacity":
		horizon := "24h"
		if h, ok := args["horizon"]; ok && strings.TrimSpace(h) != "" {
			horizon = strings.TrimSpace(h)
		}
		return &GetPromptResult{
			Description: "Evaluate fleet-wide capacity pressure and top capacity risks",
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: PromptContent{
						Type: "text",
						Text: fmt.Sprintf(
							"Please analyze fleet-wide capacity pressure and threshold risks across horizon %q:\n"+
								"1. Call `get_fleet_predictions` with horizon=%q to assess cluster CPU, memory, and disk pressure percentages.\n"+
								"2. Review top capacity risk nodes and their projected exhaustion timelines.\n"+
								"3. Call `get_intelligence_findings` with category=\"capacity_risk\" to review flagged multi-node capacity findings.\n"+
								"4. Provide an executive capacity forecast highlighting at-risk nodes and operational recommendations.",
							horizon, horizon,
						),
					},
				},
			},
		}, nil

	case "investigate_recurring_incidents":
		since := "24h"
		if s, ok := args["since"]; ok && strings.TrimSpace(s) != "" {
			since = strings.TrimSpace(s)
		}
		return &GetPromptResult{
			Description: "Analyze recurring incidents and flapping patterns across the fleet",
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: PromptContent{
						Type: "text",
						Text: fmt.Sprintf(
							"Please investigate recurring incident patterns across the fleet over lookback window %q:\n"+
								"1. Call `get_recurring_incidents` with since=%q to inspect statistical recurrence patterns and inter-arrival intervals.\n"+
								"2. Review regularity metrics including Coefficient of Variation (CV) and median intervals for flapping incidents.\n"+
								"3. For flapping nodes, call `get_node_intelligence` and `get_node_trends` to identify underlying trigger factors.\n"+
								"4. Synthesize observational recurrence patterns and non-invasive investigation guidance.",
							since, since,
						),
					},
				},
			},
		}, nil

	case "topology_spof_analysis":
		minCrit := "0"
		if mc, ok := args["min_criticality"]; ok && strings.TrimSpace(mc) != "" {
			minCrit = strings.TrimSpace(mc)
		}
		return &GetPromptResult{
			Description: "Analyze single points of failure and blast radius across the topology",
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: PromptContent{
						Type: "text",
						Text: fmt.Sprintf(
							"Please analyze single points of failure and topological blast radius:\n"+
								"1. Call `get_spofs` with min_criticality=%s to identify high-risk single points of failure.\n"+
								"2. Call `get_topology_summary` to understand overall graph density, node distribution, and edge relationships.\n"+
								"3. For identified SPOF nodes, call `get_node_impact` to assess downstream dependencies, blast radius, and affected components.\n"+
								"4. Synthesize structural architectural risks and propose non-invasive redundancy enhancements.",
							minCrit,
						),
					},
				},
			},
		}, nil

	case "root_cause_analysis":
		incidentID, ok := args["incident_id"]
		if !ok || strings.TrimSpace(incidentID) == "" {
			return nil, NewInvalidParamsError("missing required argument 'incident_id'")
		}
		incidentID = strings.TrimSpace(incidentID)
		return &GetPromptResult{
			Description: fmt.Sprintf("Perform topological root cause analysis for incident %s", incidentID),
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: PromptContent{
						Type: "text",
						Text: fmt.Sprintf(
							"Please perform topological and temporal root cause analysis for incident %q:\n"+
								"1. Call `analyze_root_cause` with incident_id=%q to compute causal ranking, factor contributions, and propagation paths.\n"+
								"2. Call `get_topology_path` between the suspected root cause and victim nodes to trace failure propagation.\n"+
								"3. Call `get_node_impact` on the root-cause node to evaluate blast radius and dependent services.\n"+
								"4. Synthesize the findings into an explainable root cause report with non-invasive operator recommendations.",
							incidentID, incidentID,
						),
					},
				},
			},
		}, nil

	default:
		return nil, NewInvalidParamsError(fmt.Sprintf("unknown prompt '%s'", name))
	}
}
