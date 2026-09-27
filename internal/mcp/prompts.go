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

	default:
		return nil, NewInvalidParamsError(fmt.Sprintf("unknown prompt '%s'", name))
	}
}
