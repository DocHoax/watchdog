package fleet

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

var (
	// ErrNodeNotFound is returned when the requested fleet node does not exist.
	ErrNodeNotFound = errors.New("fleet node not found")
	// ErrInvalidNodeID is returned when a node ID is empty or improperly formatted.
	ErrInvalidNodeID = errors.New("invalid or empty node_id")
	// ErrRateLimitExceeded is returned when a node exceeds its telemetry ingestion quota.
	ErrRateLimitExceeded = errors.New("telemetry rate limit exceeded")
	// ErrInvalidTelemetry is returned when a telemetry submission lacks mandatory fields.
	ErrInvalidTelemetry = errors.New("invalid telemetry submission: missing node_id")
)

// ReadOnlyFleetService defines the strict read-only query contract for fleet topology and health.
type ReadOnlyFleetService interface {
	GetNode(ctx context.Context, nodeID string) (*model.NodeDetailResponse, error)
	ListNodes(ctx context.Context, filter model.FleetFilter) (*model.FleetListResponse, error)
	GetFleetSummary(ctx context.Context) (*model.FleetSummary, error)
}

// FleetService defines the domain interface for managing fleet nodes and telemetry.
type FleetService interface {
	ReadOnlyFleetService
	RegisterNode(ctx context.Context, req *model.NodeRegistrationRequest) (*model.NodeRegistrationResponse, error)
	ProcessHeartbeat(ctx context.Context, hb *model.HeartbeatRequest) (*model.HeartbeatResponse, error)
	IngestTelemetry(ctx context.Context, sub *model.TelemetrySubmission) error
	DeleteNode(ctx context.Context, nodeID string) error
}

// ServiceConfig holds operational configuration parameters for FleetService.
type ServiceConfig struct {
	HeartbeatInterval time.Duration
	TelemetryInterval time.Duration
	StaleThreshold    time.Duration
	OfflineThreshold  time.Duration
	RateLimitRate     float64
	RateLimitBurst    int
}

type fleetService struct {
	store       storage.Storage
	cfg         ServiceConfig
	rateLimiter *NodeRateLimiter
	mu          sync.RWMutex
}

// NewFleetService instantiates a new FleetService implementation backed by storage.
func NewFleetService(store storage.Storage, fleetCfg config.FleetConfig) FleetService {
	hbInt := fleetCfg.HeartbeatInterval
	if hbInt <= 0 {
		hbInt = 10 * time.Second
	}
	telemInt := fleetCfg.TelemetryInterval
	if telemInt <= 0 {
		telemInt = 30 * time.Second
	}
	staleThresh := fleetCfg.StaleThreshold
	if staleThresh <= 0 {
		staleThresh = 2 * time.Minute
	}
	offlineThresh := fleetCfg.OfflineThreshold
	if offlineThresh <= 0 {
		offlineThresh = 10 * time.Minute
	}

	rate := fleetCfg.RateLimitRate
	if rate <= 0 {
		rate = 5.0
	}
	burst := fleetCfg.RateLimitBurst
	if burst <= 0 {
		burst = 10
	}

	return &fleetService{
		store: store,
		cfg: ServiceConfig{
			HeartbeatInterval: hbInt,
			TelemetryInterval: telemInt,
			StaleThreshold:    staleThresh,
			OfflineThreshold:  offlineThresh,
			RateLimitRate:     rate,
			RateLimitBurst:    burst,
		},
		rateLimiter: NewNodeRateLimiter(rate, burst),
	}
}

// RegisterNode registers or updates a fleet node's identity and metadata.
func (s *fleetService) RegisterNode(ctx context.Context, req *model.NodeRegistrationRequest) (*model.NodeRegistrationResponse, error) {
	if req == nil || req.Identity.NodeID == "" {
		return nil, ErrInvalidNodeID
	}

	now := time.Now().UTC()
	regTime := req.Identity.CreatedAt
	if regTime.IsZero() {
		regTime = now
	}

	node := &model.FleetNode{
		Identity:      req.Identity,
		Status:        model.NodeStatusHealthy,
		StatusMessage: "Node registered successfully",
		RegisteredAt:  regTime,
		LastHeartbeat: now,
		Metadata:      req.Metadata,
	}

	if err := s.store.SaveFleetNode(ctx, node); err != nil {
		return nil, fmt.Errorf("failed to save fleet node registration: %w", err)
	}

	hbSec := int(s.cfg.HeartbeatInterval.Seconds())
	if hbSec <= 0 {
		hbSec = 10
	}
	telemSec := int(s.cfg.TelemetryInterval.Seconds())
	if telemSec <= 0 {
		telemSec = 30
	}

	return &model.NodeRegistrationResponse{
		Registered:               true,
		NodeID:                   req.Identity.NodeID,
		RegisteredAt:             regTime,
		HeartbeatIntervalSeconds: hbSec,
		TelemetryIntervalSeconds: telemSec,
		Message:                  "Node registered successfully",
	}, nil
}

// ProcessHeartbeat records a node heartbeat and updates its operational status.
func (s *fleetService) ProcessHeartbeat(ctx context.Context, hb *model.HeartbeatRequest) (*model.HeartbeatResponse, error) {
	if hb == nil || hb.NodeID == "" {
		return nil, ErrInvalidNodeID
	}

	now := time.Now().UTC()
	hbTime := hb.Timestamp
	if hbTime.IsZero() {
		hbTime = now
	}

	// Retrieve existing node or create minimal record
	node, err := s.store.GetFleetNode(ctx, hb.NodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve fleet node: %w", err)
	}

	if node == nil {
		// Auto-register minimal node record if not explicitly registered yet
		node = &model.FleetNode{
			Identity: model.NodeIdentity{
				NodeID:    hb.NodeID,
				Hostname:  hb.NodeID,
				Version:   hb.Version,
				Tags:      hb.Tags,
				CreatedAt: hbTime,
			},
			RegisteredAt: hbTime,
		}
	}

	node.Status = hb.Status
	if node.Status == "" {
		node.Status = model.NodeStatusHealthy
	}
	node.StatusMessage = hb.StatusMessage
	node.LastHeartbeat = hbTime

	if node.Summary == nil {
		node.Summary = &model.NodeSummary{
			NodeID:   hb.NodeID,
			Hostname: node.Identity.Hostname,
		}
	}

	node.Summary.Status = node.Status
	node.Summary.CPUUsagePercent = hb.CPUUsagePercent
	node.Summary.MemoryUsagePercent = hb.MemoryUsagePercent
	node.Summary.DiskUsagePercent = hb.DiskUsagePercent
	node.Summary.Load1 = hb.Load1
	node.Summary.ActiveAlertsCount = hb.ActiveAlertsCount
	node.Summary.DiagnosticStatus = hb.DiagnosticStatus
	node.Summary.LastHeartbeat = hbTime
	if hb.Version != "" {
		node.Summary.Version = hb.Version
	}
	if len(hb.Tags) > 0 {
		node.Summary.Tags = hb.Tags
	}

	if err := s.store.SaveFleetNode(ctx, node); err != nil {
		return nil, fmt.Errorf("failed to update fleet node heartbeat: %w", err)
	}

	hbSec := int(s.cfg.HeartbeatInterval.Seconds())
	if hbSec <= 0 {
		hbSec = 10
	}

	return &model.HeartbeatResponse{
		Acknowledged:          true,
		Timestamp:             now,
		NodeStatus:            node.Status,
		NextHeartbeatInterval: hbSec,
	}, nil
}

// IngestTelemetry validates and persists an incoming telemetry submission.
func (s *fleetService) IngestTelemetry(ctx context.Context, sub *model.TelemetrySubmission) error {
	if sub == nil || sub.NodeID == "" {
		return ErrInvalidTelemetry
	}

	// 1. Rate Limiting Check
	if s.rateLimiter != nil && !s.rateLimiter.Allow(sub.NodeID) {
		return ErrRateLimitExceeded
	}

	if sub.Timestamp.IsZero() {
		sub.Timestamp = time.Now().UTC()
	}

	// 2. Persist to storage
	if err := s.store.SaveTelemetrySubmission(ctx, sub); err != nil {
		return fmt.Errorf("failed to save telemetry submission: %w", err)
	}

	return nil
}

// GetNode retrieves a specific fleet node along with its dynamic status and recent telemetry.
func (s *fleetService) GetNode(ctx context.Context, nodeID string) (*model.NodeDetailResponse, error) {
	if nodeID == "" {
		return nil, ErrInvalidNodeID
	}

	node, err := s.store.GetFleetNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to query fleet node: %w", err)
	}
	if node == nil {
		return nil, ErrNodeNotFound
	}

	// Dynamic status evaluation
	node.Status = s.evaluateNodeStatus(node, time.Now().UTC())

	// Fetch recent telemetry submissions
	recentSubs, err := s.store.GetNodeTelemetrySubmissions(ctx, nodeID, time.Time{}, 10)
	if err != nil {
		recentSubs = nil
	}

	var latestSnap *model.SystemSnapshot
	var latestDiag *model.DiagnosticReport
	var activeAlerts []model.AlertEvent

	if len(recentSubs) > 0 {
		latest := recentSubs[0]
		latestSnap = latest.Snapshot
		latestDiag = latest.Diagnostics
		activeAlerts = latest.ActiveAlerts
	}

	return &model.NodeDetailResponse{
		Node:              *node,
		LatestSnapshot:    latestSnap,
		LatestDiagnostics: latestDiag,
		ActiveAlerts:      activeAlerts,
		RecentTelemetry:   recentSubs,
	}, nil
}

// ListNodes queries fleet nodes with filtering and computes dynamic summary metrics.
func (s *fleetService) ListNodes(ctx context.Context, filter model.FleetFilter) (*model.FleetListResponse, error) {
	nodes, total, err := s.store.ListFleetNodes(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list fleet nodes: %w", err)
	}

	now := time.Now().UTC()
	for i := range nodes {
		nodes[i].Status = s.evaluateNodeStatus(&nodes[i], now)
	}

	summary := s.calculateFleetSummary(nodes)

	return &model.FleetListResponse{
		Nodes:     nodes,
		Total:     total,
		Count:     len(nodes),
		Limit:     filter.Limit,
		Offset:    filter.Offset,
		Summary:   summary,
		Timestamp: now,
	}, nil
}

// DeleteNode deregisters a node and deletes its telemetry records.
func (s *fleetService) DeleteNode(ctx context.Context, nodeID string) error {
	if nodeID == "" {
		return ErrInvalidNodeID
	}

	node, err := s.store.GetFleetNode(ctx, nodeID)
	if err != nil {
		return err
	}
	if node == nil {
		return ErrNodeNotFound
	}

	return s.store.DeleteFleetNode(ctx, nodeID)
}

// GetFleetSummary aggregates real-time health and resource statistics across all nodes.
func (s *fleetService) GetFleetSummary(ctx context.Context) (*model.FleetSummary, error) {
	nodes, _, err := s.store.ListFleetNodes(ctx, model.FleetFilter{Limit: 1000})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve nodes for fleet summary: %w", err)
	}

	now := time.Now().UTC()
	for i := range nodes {
		nodes[i].Status = s.evaluateNodeStatus(&nodes[i], now)
	}

	sum := s.calculateFleetSummary(nodes)
	return &sum, nil
}

func (s *fleetService) evaluateNodeStatus(node *model.FleetNode, now time.Time) model.NodeStatus {
	if node == nil {
		return model.NodeStatusUnknown
	}

	timeSinceHB := now.Sub(node.LastHeartbeat)
	if s.cfg.OfflineThreshold > 0 && timeSinceHB >= s.cfg.OfflineThreshold {
		return model.NodeStatusOffline
	}
	if s.cfg.StaleThreshold > 0 && timeSinceHB >= s.cfg.StaleThreshold {
		return model.NodeStatusStale
	}

	if node.Status != "" {
		return node.Status
	}

	return model.NodeStatusHealthy
}

func (s *fleetService) calculateFleetSummary(nodes []model.FleetNode) model.FleetSummary {
	var summary model.FleetSummary
	summary.TotalNodes = len(nodes)
	summary.LastUpdated = time.Now().UTC()

	var totalCPU, totalMem float64
	var nodesWithMetrics int

	for _, n := range nodes {
		switch n.Status {
		case model.NodeStatusHealthy:
			summary.HealthyNodes++
		case model.NodeStatusWarning:
			summary.WarningNodes++
		case model.NodeStatusCritical:
			summary.CriticalNodes++
		case model.NodeStatusStale:
			summary.StaleNodes++
		case model.NodeStatusOffline:
			summary.OfflineNodes++
		default:
			summary.UnknownNodes++
		}

		if n.Summary != nil {
			totalCPU += n.Summary.CPUUsagePercent
			totalMem += n.Summary.MemoryUsagePercent
			summary.TotalAlerts += n.Summary.ActiveAlertsCount
			nodesWithMetrics++
		}
	}

	if nodesWithMetrics > 0 {
		summary.AvgCPUPercent = totalCPU / float64(nodesWithMetrics)
		summary.AvgMemoryPct = totalMem / float64(nodesWithMetrics)
	}

	return summary
}
