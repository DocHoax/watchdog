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

// ResourceRegistry manages readable MCP resources.
type ResourceRegistry struct {
	fleetService  fleet.ReadOnlyFleetService
	storage       storage.ReadOnlyStorage
	collector     *collector.Manager
	diagnostics   *diagnostics.Engine
	alerts        *alerts.Engine
	localIdentity model.NodeIdentity
	intelSvc      intelligence.IntelligenceService
}

// NewResourceRegistry creates a new ResourceRegistry.
func NewResourceRegistry(
	fleetService fleet.ReadOnlyFleetService,
	store storage.ReadOnlyStorage,
	coll *collector.Manager,
	diag *diagnostics.Engine,
	alt *alerts.Engine,
	localID model.NodeIdentity,
	intelSvc ...intelligence.IntelligenceService,
) *ResourceRegistry {
	if localID.NodeID == "" {
		localID.NodeID = "local-node"
		localID.Hostname = "localhost"
	}
	var is intelligence.IntelligenceService
	if len(intelSvc) > 0 {
		is = intelSvc[0]
	}
	return &ResourceRegistry{
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
func (r *ResourceRegistry) SetIntelligenceService(svc intelligence.IntelligenceService) {
	r.intelSvc = svc
}

func (r *ResourceRegistry) getIntelligenceService() intelligence.IntelligenceService {
	if r.intelSvc != nil {
		return r.intelSvc
	}
	return intelligence.NewService(r.storage, r.fleetService, anomaly.NewDetector(nil), logger.GetDefault(), nil)
}

// ListResources returns the static list and dynamic template resources available to the client.
func (r *ResourceRegistry) ListResources(ctx context.Context) ([]Resource, error) {
	resources := []Resource{
		{
			URI:         "watchdog://fleet",
			Name:        "Fleet Overview",
			Description: "Aggregated health and summary status of all managed fleet nodes",
			MIMEType:    "application/json",
		},
		{
			URI:         fmt.Sprintf("watchdog://fleet/%s", r.localIdentity.NodeID),
			Name:        "Local Node Details",
			Description: "Identity and hardware configuration of the local host",
			MIMEType:    "application/json",
		},
		{
			URI:         fmt.Sprintf("watchdog://fleet/%s/health", r.localIdentity.NodeID),
			Name:        "Local Node Health",
			Description: "Health metrics and diagnostic status of the local host",
			MIMEType:    "application/json",
		},
		{
			URI:         fmt.Sprintf("watchdog://fleet/%s/snapshot", r.localIdentity.NodeID),
			Name:        "Local Node Snapshot",
			Description: "Latest telemetry snapshot of the local host",
			MIMEType:    "application/json",
		},
		{
			URI:         fmt.Sprintf("watchdog://fleet/%s/alerts", r.localIdentity.NodeID),
			Name:        "Local Node Alerts",
			Description: "Active firing alerts for the local host",
			MIMEType:    "application/json",
		},
		{
			URI:         "intelligence://fleet/summary",
			Name:        "Fleet Health Intelligence Summary",
			Description: "Aggregated 0-100 fleet health score, trends, incidents, and findings",
			MIMEType:    "application/json",
		},
		{
			URI:         "intelligence://incidents/active",
			Name:        "Active Fleet Incidents",
			Description: "Active clustered incidents with root symptoms and timelines across the fleet",
			MIMEType:    "application/json",
		},
		{
			URI:         fmt.Sprintf("intelligence://nodes/%s/summary", r.localIdentity.NodeID),
			Name:        "Local Node Intelligence Summary",
			Description: "Explainable health score breakdown and trend assessment for the local host",
			MIMEType:    "application/json",
		},
	}

	// If fleetService is available, add discovered nodes
	if r.fleetService != nil {
		listResp, err := r.fleetService.ListNodes(ctx, model.FleetFilter{Limit: 20})
		if err == nil && listResp != nil {
			for _, n := range listResp.Nodes {
				if n.Identity.NodeID == r.localIdentity.NodeID {
					continue
				}
				resources = append(resources, Resource{
					URI:         fmt.Sprintf("watchdog://fleet/%s", n.Identity.NodeID),
					Name:        fmt.Sprintf("Node %s (%s)", n.Identity.Hostname, n.Identity.NodeID),
					Description: fmt.Sprintf("Identity and configuration for %s", n.Identity.Hostname),
					MIMEType:    "application/json",
				})
				resources = append(resources, Resource{
					URI:         fmt.Sprintf("intelligence://nodes/%s/summary", n.Identity.NodeID),
					Name:        fmt.Sprintf("Intelligence Summary for %s", n.Identity.Hostname),
					Description: fmt.Sprintf("Explainable health score and trends for %s", n.Identity.Hostname),
					MIMEType:    "application/json",
				})
			}
		}
	}

	return resources, nil
}

// ReadResource reads and resolves a resource by URI.
func (r *ResourceRegistry) ReadResource(ctx context.Context, mcpCtx MCPContext, uri string) (*ReadResourceResult, *JSONRPCError) {
	trimmed := strings.TrimSpace(uri)
	if strings.HasPrefix(trimmed, "intelligence://") {
		return r.readIntelligenceResource(ctx, uri, trimmed)
	}

	if !strings.HasPrefix(trimmed, "watchdog://fleet") {
		return nil, NewResourceNotFoundError(uri)
	}

	path := strings.TrimPrefix(trimmed, "watchdog://fleet")
	path = strings.TrimPrefix(path, "/")

	if path == "" {
		// watchdog://fleet
		return r.readFleetOverview(ctx, uri)
	}

	parts := strings.Split(path, "/")
	nodeID := parts[0]
	if err := ValidateNodeID(nodeID); err != nil {
		return nil, NewInvalidNodeIDError(err.Error())
	}

	if len(parts) == 1 {
		// watchdog://fleet/{node_id}
		return r.readNodeDetail(ctx, uri, nodeID)
	}

	subResource := parts[1]
	switch subResource {
	case "health":
		return r.readNodeHealth(ctx, uri, nodeID)
	case "snapshot":
		return r.readNodeSnapshot(ctx, uri, nodeID)
	case "alerts":
		return r.readNodeAlerts(ctx, uri, nodeID)
	default:
		return nil, NewResourceNotFoundError(uri)
	}
}

func (r *ResourceRegistry) readIntelligenceResource(ctx context.Context, origURI string, trimmedURI string) (*ReadResourceResult, *JSONRPCError) {
	svc := r.getIntelligenceService()
	path := strings.TrimPrefix(trimmedURI, "intelligence://")
	path = strings.TrimPrefix(path, "/")

	if path == "fleet/summary" || path == "fleet" {
		summary, err := svc.EvaluateFleetHealth(ctx)
		if err != nil {
			return nil, NewInternalError(fmt.Sprintf("failed to evaluate fleet intelligence: %v", err))
		}
		return resourceResult(origURI, summary)
	}

	if path == "incidents/active" || path == "incidents" {
		incidents, err := svc.GetActiveIncidents(ctx)
		if err != nil {
			return nil, NewInternalError(fmt.Sprintf("failed to retrieve active incidents: %v", err))
		}
		return resourceResult(origURI, incidents)
	}

	if strings.HasPrefix(path, "nodes/") {
		nodePath := strings.TrimPrefix(path, "nodes/")
		parts := strings.Split(nodePath, "/")
		nodeID := parts[0]
		if err := ValidateNodeID(nodeID); err != nil {
			return nil, NewInvalidNodeIDError(err.Error())
		}
		summary, err := svc.EvaluateNodeHealth(ctx, nodeID)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "not found") {
				return nil, NewNodeNotFoundError(nodeID)
			}
			return nil, NewInternalError(fmt.Sprintf("failed to evaluate node health for '%s': %v", nodeID, err))
		}
		return resourceResult(origURI, summary)
	}

	return nil, NewResourceNotFoundError(origURI)
}

func (r *ResourceRegistry) readFleetOverview(ctx context.Context, uri string) (*ReadResourceResult, *JSONRPCError) {
	if r.fleetService != nil {
		summary, err := r.fleetService.GetFleetSummary(ctx)
		if err != nil {
			return nil, NewInternalError(err.Error())
		}
		return resourceResult(uri, summary)
	}

	summary := model.FleetSummary{
		TotalNodes:   1,
		HealthyNodes: 1,
		LastUpdated:  time.Now(),
	}
	return resourceResult(uri, summary)
}

func (r *ResourceRegistry) readNodeDetail(ctx context.Context, uri string, nodeID string) (*ReadResourceResult, *JSONRPCError) {
	if r.fleetService != nil {
		detail, err := r.fleetService.GetNode(ctx, nodeID)
		if err != nil {
			if err == fleet.ErrNodeNotFound {
				return nil, NewNodeNotFoundError(nodeID)
			}
			return nil, NewInternalError(err.Error())
		}
		return resourceResult(uri, detail.Node)
	}

	if nodeID == r.localIdentity.NodeID || nodeID == "local" || nodeID == "self" || nodeID == "localhost" {
		node := model.FleetNode{
			Identity:      r.localIdentity,
			Status:        model.NodeStatusHealthy,
			RegisteredAt:  r.localIdentity.CreatedAt,
			LastHeartbeat: time.Now(),
		}
		return resourceResult(uri, node)
	}

	return nil, NewNodeNotFoundError(nodeID)
}

func (r *ResourceRegistry) readNodeHealth(ctx context.Context, uri string, nodeID string) (*ReadResourceResult, *JSONRPCError) {
	if r.fleetService != nil {
		detail, err := r.fleetService.GetNode(ctx, nodeID)
		if err != nil {
			if err == fleet.ErrNodeNotFound {
				return nil, NewNodeNotFoundError(nodeID)
			}
			return nil, NewInternalError(err.Error())
		}
		health := map[string]any{
			"node_id":             detail.Node.Identity.NodeID,
			"hostname":            detail.Node.Identity.Hostname,
			"status":              detail.Node.Status,
			"status_message":      detail.Node.StatusMessage,
			"last_heartbeat":      detail.Node.LastHeartbeat,
			"summary":             detail.Node.Summary,
			"active_alerts_count": len(detail.ActiveAlerts),
		}
		return resourceResult(uri, health)
	}

	if nodeID == r.localIdentity.NodeID || nodeID == "local" || nodeID == "self" || nodeID == "localhost" {
		var activeAlertsCount int
		if r.alerts != nil {
			activeAlertsCount = len(r.alerts.GetActiveAlerts())
		}
		health := map[string]any{
			"node_id":             r.localIdentity.NodeID,
			"hostname":            r.localIdentity.Hostname,
			"status":              model.NodeStatusHealthy,
			"last_heartbeat":      time.Now(),
			"active_alerts_count": activeAlertsCount,
		}
		return resourceResult(uri, health)
	}

	return nil, NewNodeNotFoundError(nodeID)
}

func (r *ResourceRegistry) readNodeSnapshot(ctx context.Context, uri string, nodeID string) (*ReadResourceResult, *JSONRPCError) {
	if r.fleetService != nil {
		detail, err := r.fleetService.GetNode(ctx, nodeID)
		if err != nil {
			if err == fleet.ErrNodeNotFound {
				return nil, NewNodeNotFoundError(nodeID)
			}
			return nil, NewInternalError(err.Error())
		}
		if detail.LatestSnapshot != nil {
			return resourceResult(uri, detail.LatestSnapshot)
		}
		return resourceResult(uri, map[string]string{"message": "no snapshot available"})
	}

	if (nodeID == r.localIdentity.NodeID || nodeID == "local" || nodeID == "self" || nodeID == "localhost") && r.collector != nil {
		snap, err := r.collector.CollectAll(ctx)
		if err != nil {
			return nil, NewInternalError(fmt.Sprintf("failed to collect snapshot: %v", err))
		}
		return resourceResult(uri, snap)
	}

	return nil, NewNodeNotFoundError(nodeID)
}

func (r *ResourceRegistry) readNodeAlerts(ctx context.Context, uri string, nodeID string) (*ReadResourceResult, *JSONRPCError) {
	if r.fleetService != nil {
		detail, err := r.fleetService.GetNode(ctx, nodeID)
		if err != nil {
			if err == fleet.ErrNodeNotFound {
				return nil, NewNodeNotFoundError(nodeID)
			}
			return nil, NewInternalError(err.Error())
		}
		return resourceResult(uri, detail.ActiveAlerts)
	}

	if nodeID == r.localIdentity.NodeID || nodeID == "local" || nodeID == "self" || nodeID == "localhost" {
		var active []model.AlertEvent
		if r.alerts != nil {
			active = r.alerts.GetActiveAlerts()
		} else if r.storage != nil {
			active, _ = r.storage.GetActiveAlerts(ctx)
		}
		return resourceResult(uri, active)
	}

	return nil, NewNodeNotFoundError(nodeID)
}

func resourceResult(uri string, v any) (*ReadResourceResult, *JSONRPCError) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, NewInternalError(fmt.Sprintf("failed to marshal resource payload: %v", err))
	}
	return &ReadResourceResult{
		Contents: []ResourceContent{
			{
				URI:      uri,
				MIMEType: "application/json",
				Text:     string(b),
			},
		},
	}, nil
}
