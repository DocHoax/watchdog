package governance

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/DocHoax/watchdog/internal/incidents"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

// Clock defines a time provider interface for deterministic evaluation and time testing.
type Clock interface {
	Now() time.Time
	Since(t time.Time) time.Duration
}

// RealClock provides wall-clock time using standard time package.
type RealClock struct{}

// Now returns current UTC time.
func (RealClock) Now() time.Time {
	return time.Now().UTC()
}

// Since returns duration since t based on current wall clock.
func (RealClock) Since(t time.Time) time.Duration {
	return time.Since(t)
}

// MockClock provides a controllable clock for tests and deterministic simulations.
type MockClock struct {
	mu          sync.RWMutex
	currentTime time.Time
}

// NewMockClock creates a mock clock initialized to the given time.
func NewMockClock(t time.Time) *MockClock {
	return &MockClock{
		currentTime: t.UTC(),
	}
}

// Now returns the simulated current time.
func (m *MockClock) Now() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentTime
}

// Since returns the simulated duration elapsed since t.
func (m *MockClock) Since(t time.Time) time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentTime.Sub(t)
}

// SetTime updates the mock clock's current time.
func (m *MockClock) SetTime(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentTime = t.UTC()
}

// Advance moves the mock clock forward by duration d.
func (m *MockClock) Advance(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentTime = m.currentTime.Add(d)
}

// FreshnessConfig configures data freshness tolerance windows for telemetry and health signals.
type FreshnessConfig struct {
	TelemetryTolerance    time.Duration `json:"telemetry_tolerance"`
	HeartbeatTolerance    time.Duration `json:"heartbeat_tolerance"`
	DiagnosticTolerance   time.Duration `json:"diagnostic_tolerance"`
	MetricTolerance       time.Duration `json:"metric_tolerance"`
	TelemetryHistoryLimit int           `json:"telemetry_history_limit"`
	IncidentLookback      time.Duration `json:"incident_lookback"`
}

// DefaultFreshnessConfig returns the recommended default tolerance windows.
func DefaultFreshnessConfig() FreshnessConfig {
	return FreshnessConfig{
		TelemetryTolerance:    5 * time.Minute,
		HeartbeatTolerance:    2 * time.Minute,
		DiagnosticTolerance:   15 * time.Minute,
		MetricTolerance:       5 * time.Minute,
		TelemetryHistoryLimit: 20,
		IncidentLookback:      24 * time.Hour,
	}
}

// EvaluationContext encapsulates all node state, hierarchy provenance, effective policies,
// telemetry, diagnostics, and incidents needed to evaluate policy rules on a target node.
type EvaluationContext struct {
	Node                   *model.FleetNode
	OrgID                  string
	Ownership              *model.NodeOwnershipMetadata
	DirectGroups           []model.FleetGroup
	AllGroupIDs            []string
	GroupPaths             []string
	ResolvedPolicySet      *ResolvedPolicySet
	LatestTelemetry        *model.TelemetrySubmission
	TelemetryHistory       []model.TelemetrySubmission
	MetricAggregates       map[string]*storage.MetricAggregate
	LatestDiagnosticReport *model.DiagnosticReport
	ActiveIncidents        []incidents.Incident
	DataFreshness          map[string]model.DataFreshnessStatus
	Clock                  Clock
	FreshnessConfig        FreshnessConfig
}

// ClassifyFreshness evaluates whether an observation timestamp is fresh, stale, missing, or invalid.
func (ec *EvaluationContext) ClassifyFreshness(observedAt time.Time, maxAge time.Duration) model.DataFreshnessStatus {
	if observedAt.IsZero() {
		return model.DataFreshnessMissing
	}
	age := ec.Clock.Since(observedAt)
	if age < -1*time.Minute {
		// Clock skew: timestamp is more than a minute in the future
		return model.DataFreshnessInvalid
	}
	if maxAge > 0 && age > maxAge {
		return model.DataFreshnessStale
	}
	return model.DataFreshnessFresh
}

// GetMetricValue retrieves the latest scalar metric observation for a metric name,
// searching the latest telemetry submission, snapshot metrics, and fallback node summary.
func (ec *EvaluationContext) GetMetricValue(metricName string) (float64, model.DataFreshnessStatus, bool) {
	norm := strings.ToLower(strings.TrimSpace(metricName))
	if norm == "" {
		return 0, model.DataFreshnessInvalid, false
	}

	// 1. Check latest telemetry submission
	if ec.LatestTelemetry != nil {
		freshness := ec.ClassifyFreshness(ec.LatestTelemetry.Timestamp, ec.FreshnessConfig.TelemetryTolerance)

		// Check explicit custom metrics map
		if ec.LatestTelemetry.Metrics != nil {
			if val, ok := ec.LatestTelemetry.Metrics[norm]; ok {
				return val, freshness, true
			}
		}

		// Check snapshot fields
		if snap := ec.LatestTelemetry.Snapshot; snap != nil {
			val, found := extractSnapshotMetric(snap, norm)
			if found {
				return val, freshness, true
			}
		}
	}

	// 2. Fallback to node summary if available
	if ec.Node != nil && ec.Node.Summary != nil {
		var lastTel time.Time
		if ec.Node.Summary.LastTelemetry != nil {
			lastTel = *ec.Node.Summary.LastTelemetry
		} else if ec.Node.LastTelemetry != nil {
			lastTel = *ec.Node.LastTelemetry
		}
		freshness := ec.ClassifyFreshness(lastTel, ec.FreshnessConfig.TelemetryTolerance)
		switch norm {
		case "cpu_usage_pct", "cpu_utilization", "cpu_percent", "cpu_usage":
			return ec.Node.Summary.CPUUsagePercent, freshness, true
		case "memory_used_pct", "memory_percent", "mem_usage_pct", "memory_usage":
			return ec.Node.Summary.MemoryUsagePercent, freshness, true
		case "disk_used_pct", "disk_usage_pct", "disk_percent", "disk_usage":
			return ec.Node.Summary.DiskUsagePercent, freshness, true
		}
	}

	return 0, model.DataFreshnessMissing, false
}

func extractSnapshotMetric(snap *model.SystemSnapshot, norm string) (float64, bool) {
	switch norm {
	case "cpu_usage_pct", "cpu_utilization", "cpu_percent", "cpu_usage":
		if snap.CPU != nil {
			return snap.CPU.OverallUsage, true
		}
	case "temperature_celsius", "cpu_temp", "cpu_temperature":
		if snap.CPU != nil && snap.CPU.TemperatureAvg > 0 {
			return snap.CPU.TemperatureAvg, true
		}
	case "memory_used_pct", "memory_percent", "mem_usage_pct", "memory_usage":
		if snap.Memory != nil {
			return snap.Memory.UsedPercent, true
		}
	case "memory_available_bytes":
		if snap.Memory != nil {
			return float64(snap.Memory.AvailableBytes), true
		}
	case "memory_total_bytes":
		if snap.Memory != nil {
			return float64(snap.Memory.TotalBytes), true
		}
	case "memory_used_bytes":
		if snap.Memory != nil {
			return float64(snap.Memory.UsedBytes), true
		}
	case "swap_used_pct", "swap_percent":
		if snap.Memory != nil {
			return snap.Memory.SwapUsedPercent, true
		}
	case "swap_used_bytes":
		if snap.Memory != nil {
			return float64(snap.Memory.SwapUsedBytes), true
		}
	case "disk_used_pct", "disk_usage_pct", "disk_percent", "disk_usage":
		if snap.Disk != nil {
			if snap.Disk.UsedPercent > 0 {
				return snap.Disk.UsedPercent, true
			}
			if len(snap.Disk.Partitions) > 0 {
				return snap.Disk.Partitions[0].UsedPercent, true
			}
		}
	case "disk_used_bytes":
		if snap.Disk != nil {
			return float64(snap.Disk.UsedBytes), true
		}
	case "disk_total_bytes":
		if snap.Disk != nil {
			return float64(snap.Disk.TotalBytes), true
		}
	case "load_1", "load_1m", "load1":
		if snap.CPU != nil {
			return snap.CPU.LoadAverage.Load1, true
		}
	case "load_5", "load_5m", "load5":
		if snap.CPU != nil {
			return snap.CPU.LoadAverage.Load5, true
		}
	case "load_15", "load_15m", "load15":
		if snap.CPU != nil {
			return snap.CPU.LoadAverage.Load15, true
		}
	case "net_tx_bytes", "network_tx_bytes":
		if snap.Network != nil {
			return float64(snap.Network.TotalBytesSent), true
		}
	case "net_rx_bytes", "network_rx_bytes":
		if snap.Network != nil {
			return float64(snap.Network.TotalBytesRecv), true
		}
	case "net_tx_rate", "network_tx_rate":
		if snap.Network != nil {
			return snap.Network.TotalTxRate, true
		}
	case "net_rx_rate", "network_rx_rate":
		if snap.Network != nil {
			return snap.Network.TotalRxRate, true
		}
	case "uptime_seconds":
		if snap.System != nil {
			return snap.System.Uptime.Seconds(), true
		}
	case "procs_count", "process_count":
		if snap.System != nil {
			return float64(snap.System.ProcsCount), true
		}
	}
	return 0, false
}

// GetMetricHistory returns a slice of historical metric observations extracted from TelemetryHistory.
func (ec *EvaluationContext) GetMetricHistory(metricName string) []storage.MetricPoint {
	norm := strings.ToLower(strings.TrimSpace(metricName))
	var points []storage.MetricPoint

	for _, sub := range ec.TelemetryHistory {
		// Try custom metrics map
		if sub.Metrics != nil {
			if val, ok := sub.Metrics[norm]; ok {
				points = append(points, storage.MetricPoint{
					Timestamp: sub.Timestamp,
					Metric:    norm,
					Value:     val,
				})
				continue
			}
		}

		// Try snapshot
		if sub.Snapshot != nil {
			if val, found := extractSnapshotMetric(sub.Snapshot, norm); found {
				points = append(points, storage.MetricPoint{
					Timestamp: sub.Timestamp,
					Metric:    norm,
					Value:     val,
				})
			}
		}
	}

	// Sort chronologically ascending
	slices.SortFunc(points, func(a, b storage.MetricPoint) int {
		if a.Timestamp.Before(b.Timestamp) {
			return -1
		}
		if a.Timestamp.After(b.Timestamp) {
			return 1
		}
		return 0
	})

	return points
}

// GetMetricAggregate returns a precalculated statistical aggregate for metricName if available.
func (ec *EvaluationContext) GetMetricAggregate(metricName string) (*storage.MetricAggregate, bool) {
	if ec.MetricAggregates == nil {
		return nil, false
	}
	agg, ok := ec.MetricAggregates[strings.ToLower(strings.TrimSpace(metricName))]
	return agg, ok
}

// GetHeartbeatFreshness returns the elapsed duration since the node's last heartbeat and its freshness status.
func (ec *EvaluationContext) GetHeartbeatFreshness() (time.Duration, model.DataFreshnessStatus) {
	if ec.Node == nil || ec.Node.LastHeartbeat.IsZero() {
		return 0, model.DataFreshnessMissing
	}
	age := ec.Clock.Since(ec.Node.LastHeartbeat)
	freshness := ec.ClassifyFreshness(ec.Node.LastHeartbeat, ec.FreshnessConfig.HeartbeatTolerance)
	return age, freshness
}

// GetDiagnosticReport returns the latest diagnostic report and its freshness status.
func (ec *EvaluationContext) GetDiagnosticReport() (*model.DiagnosticReport, model.DataFreshnessStatus) {
	if ec.LatestDiagnosticReport == nil {
		return nil, model.DataFreshnessMissing
	}
	freshness := ec.ClassifyFreshness(ec.LatestDiagnosticReport.GeneratedAt, ec.FreshnessConfig.DiagnosticTolerance)
	return ec.LatestDiagnosticReport, freshness
}

// DataAcquisitionProvider coordinates the retrieval and assembly of evaluation contexts for target nodes.
type DataAcquisitionProvider struct {
	store     storage.ReadOnlyStorage
	resolver  *PolicyResolver
	clock     Clock
	freshness FreshnessConfig
}

// NewDataAcquisitionProvider constructs a new DataAcquisitionProvider.
func NewDataAcquisitionProvider(
	store storage.ReadOnlyStorage,
	resolver *PolicyResolver,
	clock Clock,
	cfg *FreshnessConfig,
) *DataAcquisitionProvider {
	if clock == nil {
		clock = RealClock{}
	}
	fc := DefaultFreshnessConfig()
	if cfg != nil {
		fc = *cfg
	}
	return &DataAcquisitionProvider{
		store:     store,
		resolver:  resolver,
		clock:     clock,
		freshness: fc,
	}
}

// BuildEvaluationContext queries storage and assembles an EvaluationContext for the specified node.
func (p *DataAcquisitionProvider) BuildEvaluationContext(ctx context.Context, nodeID string) (*EvaluationContext, error) {
	if strings.TrimSpace(nodeID) == "" {
		return nil, fmt.Errorf("%w: empty node id", ErrInvalidInput)
	}

	// 1. Fetch Fleet Node
	node, err := p.store.GetFleetNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNodeNotFound, err)
	}

	// 2. Determine OrgID and ownership metadata
	orgID := model.DefaultOrganizationID
	if node.Metadata != nil {
		if val, ok := node.Metadata["org_id"]; ok && val != "" {
			orgID = val
		} else if val, ok := node.Metadata["organization_id"]; ok && val != "" {
			orgID = val
		}
	}
	ownership := model.NodeOwnershipFromMetadata(node.Metadata)

	// 3. Discover groups and ancestor hierarchy paths
	directGroups, err := p.store.GetNodeGroups(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node groups: %w", err)
	}

	var allGroupIDs []string
	var groupPaths []string
	visited := make(map[string]bool)

	for _, dg := range directGroups {
		curr := dg
		for {
			if visited[curr.ID] {
				break
			}
			visited[curr.ID] = true
			allGroupIDs = append(allGroupIDs, curr.ID)
			if curr.Path != "" && !slices.Contains(groupPaths, curr.Path) {
				groupPaths = append(groupPaths, curr.Path)
			}

			if curr.ParentGroupID == "" {
				break
			}
			parent, err := p.store.GetFleetGroup(ctx, curr.ParentGroupID)
			if err != nil || parent == nil {
				break
			}
			curr = *parent
		}
	}

	// 4. Resolve effective policy rules for node
	var resolvedPolicies *ResolvedPolicySet
	if p.resolver != nil {
		resolved, err := p.resolver.ResolveNodePolicies(ctx, nodeID)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve policies for node %s: %w", nodeID, err)
		}
		resolvedPolicies = resolved
	}

	// 5. Fetch recent telemetry submissions
	var latestTelemetry *model.TelemetrySubmission
	var telemetryHistory []model.TelemetrySubmission

	since := p.clock.Now().Add(-24 * time.Hour)
	submissions, err := p.store.GetNodeTelemetrySubmissions(ctx, nodeID, since, p.freshness.TelemetryHistoryLimit)
	if err == nil && len(submissions) > 0 {
		telemetryHistory = submissions
		latestTelemetry = &submissions[0]
	}

	// 6. Fetch diagnostic report
	diagReport, _ := p.store.GetLatestDiagnosticReport(ctx)

	// 7. Fetch active incidents impacting this node
	var activeIncidents []incidents.Incident
	incList, _, err := p.store.ListIncidents(ctx, incidents.IncidentFilter{
		NodeID: nodeID,
		Status: []incidents.IncidentStatus{
			incidents.IncidentStatusDetected,
			incidents.IncidentStatusAcknowledged,
			incidents.IncidentStatusInvestigating,
			incidents.IncidentStatusReopened,
		},
		Limit: 50,
	})
	if err == nil {
		activeIncidents = incList
	}

	// 8. Classify signal freshness
	dataFreshness := make(map[string]model.DataFreshnessStatus)
	if latestTelemetry != nil {
		dataFreshness["telemetry"] = p.classify(latestTelemetry.Timestamp, p.freshness.TelemetryTolerance)
	} else {
		dataFreshness["telemetry"] = model.DataFreshnessMissing
	}

	if !node.LastHeartbeat.IsZero() {
		dataFreshness["heartbeat"] = p.classify(node.LastHeartbeat, p.freshness.HeartbeatTolerance)
	} else {
		dataFreshness["heartbeat"] = model.DataFreshnessMissing
	}

	if diagReport != nil {
		dataFreshness["diagnostics"] = p.classify(diagReport.GeneratedAt, p.freshness.DiagnosticTolerance)
	} else {
		dataFreshness["diagnostics"] = model.DataFreshnessMissing
	}

	return &EvaluationContext{
		Node:                   node,
		OrgID:                  orgID,
		Ownership:              ownership,
		DirectGroups:           directGroups,
		AllGroupIDs:            allGroupIDs,
		GroupPaths:             groupPaths,
		ResolvedPolicySet:      resolvedPolicies,
		LatestTelemetry:        latestTelemetry,
		TelemetryHistory:       telemetryHistory,
		MetricAggregates:       make(map[string]*storage.MetricAggregate),
		LatestDiagnosticReport: diagReport,
		ActiveIncidents:        activeIncidents,
		DataFreshness:          dataFreshness,
		Clock:                  p.clock,
		FreshnessConfig:        p.freshness,
	}, nil
}

func (p *DataAcquisitionProvider) classify(observedAt time.Time, maxAge time.Duration) model.DataFreshnessStatus {
	if observedAt.IsZero() {
		return model.DataFreshnessMissing
	}
	age := p.clock.Since(observedAt)
	if age < -1*time.Minute {
		return model.DataFreshnessInvalid
	}
	if maxAge > 0 && age > maxAge {
		return model.DataFreshnessStale
	}
	return model.DataFreshnessFresh
}
