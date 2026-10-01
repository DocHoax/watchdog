package intelligence

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/incidents"
	"github.com/DocHoax/watchdog/internal/logger"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

type mockFleetService struct {
	nodes map[string]*model.NodeDetailResponse
	list  []model.FleetNode
}

func (m *mockFleetService) GetNode(_ context.Context, nodeID string) (*model.NodeDetailResponse, error) {
	if n, ok := m.nodes[nodeID]; ok {
		return n, nil
	}
	return nil, fmt.Errorf("node not found")
}

func (m *mockFleetService) ListNodes(_ context.Context, _ model.FleetFilter) (*model.FleetListResponse, error) {
	return &model.FleetListResponse{
		Nodes: m.list,
		Total: len(m.list),
	}, nil
}

func (m *mockFleetService) GetFleetSummary(_ context.Context) (*model.FleetSummary, error) {
	return &model.FleetSummary{
		TotalNodes: len(m.list),
	}, nil
}

type mockStorage struct {
	metrics     map[string][]storage.MetricPoint
	submissions map[string][]model.TelemetrySubmission
}

func (m *mockStorage) Ping(_ context.Context) error { return nil }
func (m *mockStorage) QueryMetrics(_ context.Context, q storage.TimeRangeQuery) ([]storage.MetricPoint, error) {
	if pts, ok := m.metrics[q.Metric]; ok {
		return pts, nil
	}
	return nil, nil
}
func (m *mockStorage) GetMetricAggregate(_ context.Context, _ string, _, _ time.Time) (*storage.MetricAggregate, error) {
	return nil, nil
}
func (m *mockStorage) GetAvailableMetrics(_ context.Context) ([]string, error) {
	var names []string
	for k := range m.metrics {
		names = append(names, k)
	}
	return names, nil
}
func (m *mockStorage) GetAlertHistory(_ context.Context, _, _ int) ([]model.AlertEvent, error) {
	return nil, nil
}
func (m *mockStorage) GetActiveAlerts(_ context.Context) ([]model.AlertEvent, error) {
	return nil, nil
}
func (m *mockStorage) GetLatestDiagnosticReport(_ context.Context) (*model.DiagnosticReport, error) {
	return nil, nil
}
func (m *mockStorage) GetDiagnosticHistory(_ context.Context, _ int) ([]model.DiagnosticReport, error) {
	return nil, nil
}
func (m *mockStorage) QueryAuditEvents(_ context.Context, _ storage.AuditFilter) ([]model.AuditEvent, error) {
	return nil, nil
}
func (m *mockStorage) CountAuditEvents(_ context.Context, _ storage.AuditFilter) (int64, error) {
	return 0, nil
}
func (m *mockStorage) GetFleetNode(_ context.Context, _ string) (*model.FleetNode, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockStorage) ListFleetNodes(_ context.Context, _ model.FleetFilter) ([]model.FleetNode, int, error) {
	return nil, 0, nil
}
func (m *mockStorage) GetNodeTelemetrySubmissions(_ context.Context, nodeID string, _ time.Time, _ int) ([]model.TelemetrySubmission, error) {
	if subs, ok := m.submissions[nodeID]; ok {
		return subs, nil
	}
	return nil, nil
}
func (m *mockStorage) GetDatabaseSize() (int64, error) {
	return 1024, nil
}
func (m *mockStorage) GetIncident(_ context.Context, _ string) (*incidents.Incident, error) {
	return nil, nil
}
func (m *mockStorage) ListIncidents(_ context.Context, _ incidents.IncidentFilter) ([]incidents.Incident, int, error) {
	return nil, 0, nil
}
func (m *mockStorage) GetTimeline(_ context.Context, _ string, _ incidents.TimelineFilter) ([]incidents.IncidentTimelineEntry, error) {
	return nil, nil
}
func (m *mockStorage) GetIncidentHistory(_ context.Context, _ time.Duration) ([]incidents.Incident, error) {
	return nil, nil
}
func (m *mockStorage) GetOrganization(_ context.Context, _ string) (*model.Organization, error) {
	return nil, nil
}
func (m *mockStorage) ListOrganizations(_ context.Context) ([]model.Organization, error) {
	return nil, nil
}
func (m *mockStorage) GetFleetGroup(_ context.Context, _ string) (*model.FleetGroup, error) {
	return nil, nil
}
func (m *mockStorage) ListFleetGroups(_ context.Context, _ string) ([]model.FleetGroup, error) {
	return nil, nil
}
func (m *mockStorage) GetGroupMembers(_ context.Context, _ string) ([]model.FleetGroupMember, error) {
	return nil, nil
}
func (m *mockStorage) GetNodeGroups(_ context.Context, _ string) ([]model.FleetGroup, error) {
	return nil, nil
}
func (m *mockStorage) GetPolicy(_ context.Context, _ string) (*model.Policy, error) {
	return nil, nil
}
func (m *mockStorage) ListPolicies(_ context.Context, _ model.PolicyFilter) ([]model.Policy, error) {
	return nil, nil
}
func (m *mockStorage) GetPolicyRevision(_ context.Context, _ string, _ int) (*model.PolicyRevision, error) {
	return nil, nil
}
func (m *mockStorage) ListPolicyRevisions(_ context.Context, _ string) ([]model.PolicyRevision, error) {
	return nil, nil
}
func (m *mockStorage) GetPolicyAssignment(_ context.Context, _ string) (*model.PolicyAssignment, error) {
	return nil, nil
}
func (m *mockStorage) ListPolicyAssignments(_ context.Context, _ model.PolicyAssignmentFilter) ([]model.PolicyAssignment, error) {
	return nil, nil
}
func (m *mockStorage) GetAssignmentsForTargets(_ context.Context, _ string, _ model.PolicyTargetType, _ []string) ([]model.PolicyAssignment, error) {
	return nil, nil
}
func (m *mockStorage) GetEvaluationExecution(_ context.Context, _ string) (*model.EvaluationExecution, error) {
	return nil, nil
}
func (m *mockStorage) GetLatestNodeEvaluation(_ context.Context, _, _ string) (*model.EvaluationExecution, error) {
	return nil, nil
}
func (m *mockStorage) ListEvaluationExecutions(_ context.Context, _ model.EvaluationFilter) ([]model.EvaluationExecution, error) {
	return nil, nil
}
func (m *mockStorage) GetComplianceFinding(_ context.Context, _ string) (*model.ComplianceFinding, error) {
	return nil, nil
}
func (m *mockStorage) ListComplianceFindings(_ context.Context, _ model.FindingFilter) ([]model.ComplianceFinding, error) {
	return nil, nil
}

func TestIntelligenceService_EvaluateNodeHealth(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	mockFleet := &mockFleetService{
		nodes: map[string]*model.NodeDetailResponse{
			"node-01": {
				Node: model.FleetNode{
					Identity: model.NodeIdentity{
						NodeID:   "node-01",
						Hostname: "srv-01",
					},
					Status:        model.NodeStatusHealthy,
					LastHeartbeat: now,
				},
				LatestSnapshot: &model.SystemSnapshot{
					Timestamp: now,
					CPU: &model.CPUInfo{
						OverallUsage: 25.0,
						LogicalCores: 8,
					},
					Memory: &model.MemoryInfo{
						UsedPercent:    45.0,
						TotalBytes:     16 * 1024 * 1024 * 1024,
						AvailableBytes: 9 * 1024 * 1024 * 1024,
					},
					Disk: &model.DiskInfo{
						Partitions: []model.PartitionInfo{
							{Mountpoint: "/", UsedPercent: 50.0},
						},
					},
				},
			},
		},
	}

	mockStore := &mockStorage{
		submissions: map[string][]model.TelemetrySubmission{
			"node-01": {
				{
					Timestamp: now.Add(-30 * time.Minute),
					Snapshot: &model.SystemSnapshot{
						Timestamp: now.Add(-30 * time.Minute),
						CPU:       &model.CPUInfo{OverallUsage: 20.0},
					},
				},
				{
					Timestamp: now.Add(-15 * time.Minute),
					Snapshot: &model.SystemSnapshot{
						Timestamp: now.Add(-15 * time.Minute),
						CPU:       &model.CPUInfo{OverallUsage: 22.0},
					},
				},
			},
		},
	}

	log := logger.New(io.Discard, logger.LevelError, false, false)
	svc := NewService(mockStore, mockFleet, nil, log, &ServiceConfig{
		CacheTTL: 100 * time.Millisecond,
	})

	t.Run("Evaluate Existing Clean Node", func(t *testing.T) {
		summary, err := svc.EvaluateNodeHealth(ctx, "node-01")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if summary.NodeID != "node-01" {
			t.Errorf("expected node-01, got %s", summary.NodeID)
		}
		if summary.HealthScore.Score < 95.0 {
			t.Errorf("expected score >= 95.0, got %.2f", summary.HealthScore.Score)
		}
		if summary.HealthScore.Trajectory != TrajectoryStable {
			t.Errorf("expected TrajectoryStable, got %s", summary.HealthScore.Trajectory)
		}
	})

	t.Run("Evaluate Non-Existent Node", func(t *testing.T) {
		_, err := svc.EvaluateNodeHealth(ctx, "node-999")
		if err != ErrNodeNotFound {
			t.Errorf("expected ErrNodeNotFound, got %v", err)
		}
	})

	t.Run("Evaluate Empty NodeID", func(t *testing.T) {
		_, err := svc.EvaluateNodeHealth(ctx, "")
		if err != ErrNodeNotFound {
			t.Errorf("expected ErrNodeNotFound, got %v", err)
		}
	})
}

func TestIntelligenceService_EvaluateFleetHealth(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	mockFleet := &mockFleetService{
		list: []model.FleetNode{
			{
				Identity: model.NodeIdentity{NodeID: "node-01", Hostname: "srv-01"},
				Status:   model.NodeStatusHealthy,
				Summary: &model.NodeSummary{
					CPUUsagePercent:    20.0,
					MemoryUsagePercent: 30.0,
					DiskUsagePercent:   40.0,
				},
			},
			{
				Identity: model.NodeIdentity{NodeID: "node-02", Hostname: "srv-02"},
				Status:   model.NodeStatusWarning,
				Summary: &model.NodeSummary{
					CPUUsagePercent:    90.0,
					MemoryUsagePercent: 75.0,
					DiskUsagePercent:   80.0,
				},
			},
			{
				Identity: model.NodeIdentity{NodeID: "node-03", Hostname: "srv-03"},
				Status:   model.NodeStatusCritical,
				Summary: &model.NodeSummary{
					CPUUsagePercent:    99.0,
					MemoryUsagePercent: 95.0,
					DiskUsagePercent:   98.0,
				},
			},
		},
		nodes: map[string]*model.NodeDetailResponse{},
	}

	for _, n := range mockFleet.list {
		mockFleet.nodes[n.Identity.NodeID] = &model.NodeDetailResponse{
			Node: n,
			LatestSnapshot: &model.SystemSnapshot{
				Timestamp: now,
				CPU:       &model.CPUInfo{OverallUsage: n.Summary.CPUUsagePercent, LogicalCores: 4},
				Memory:    &model.MemoryInfo{UsedPercent: n.Summary.MemoryUsagePercent},
				Disk:      &model.DiskInfo{UsedPercent: n.Summary.DiskUsagePercent},
			},
		}
	}

	log := logger.New(io.Discard, logger.LevelError, false, false)
	svc := NewService(nil, mockFleet, nil, log, &ServiceConfig{
		CacheTTL: 200 * time.Millisecond,
	})

	t.Run("Evaluate Fleet Health Aggregates", func(t *testing.T) {
		summary, err := svc.EvaluateFleetHealth(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if summary.TotalNodes != 3 {
			t.Errorf("expected 3 total nodes, got %d", summary.TotalNodes)
		}
		if summary.HealthyCount != 1 || summary.WarningCount != 1 || summary.CriticalCount != 1 {
			t.Errorf("unexpected status counts: healthy=%d warn=%d crit=%d", summary.HealthyCount, summary.WarningCount, summary.CriticalCount)
		}
		if len(summary.LowestScoringNodes) != 3 {
			t.Fatalf("expected 3 lowest scoring nodes, got %d", len(summary.LowestScoringNodes))
		}
		// First node should be the critical node with lowest score
		if summary.LowestScoringNodes[0].NodeID != "node-03" {
			t.Errorf("expected lowest node to be node-03, got %s", summary.LowestScoringNodes[0].NodeID)
		}

		// Verify caching: calling immediately should return cached summary
		cachedSummary, err := svc.EvaluateFleetHealth(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cachedSummary.EvaluatedAt != summary.EvaluatedAt {
			t.Errorf("expected cached summary timestamp to match")
		}
	})
}

func TestIntelligenceService_IncidentsAndCorrelations(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	ptsA := make([]storage.MetricPoint, 20)
	ptsB := make([]storage.MetricPoint, 20)
	for i := range 20 {
		ts := now.Add(time.Duration(i*10) * time.Second)
		ptsA[i] = storage.MetricPoint{Timestamp: ts, Value: float64(10 + i*2)}
		ptsB[i] = storage.MetricPoint{Timestamp: ts, Value: float64(50 + i*4)}
	}

	mockStore := &mockStorage{
		metrics: map[string][]storage.MetricPoint{
			"cpu_usage_pct":   ptsA,
			"net_tx_rate":     ptsB,
			"memory_used_pct": ptsA,
			"swap_used_pct":   ptsA,
		},
	}

	mockFleet := &mockFleetService{
		list: []model.FleetNode{
			{
				Identity: model.NodeIdentity{NodeID: "node-inc-01", Hostname: "srv-inc-01"},
				Status:   model.NodeStatusCritical,
			},
		},
		nodes: map[string]*model.NodeDetailResponse{
			"node-inc-01": {
				Node: model.FleetNode{
					Identity: model.NodeIdentity{NodeID: "node-inc-01", Hostname: "srv-inc-01"},
					Status:   model.NodeStatusCritical,
				},
				ActiveAlerts: []model.AlertEvent{
					{
						ID:       "alt-crit-1",
						RuleName: "HighCPU",
						Severity: model.SeverityCritical,
						Message:  "CPU sustained above 95%",
						FiredAt:  now.Add(-10 * time.Minute),
						IsActive: true,
					},
				},
			},
		},
	}

	log := logger.New(io.Discard, logger.LevelError, false, false)
	svc := NewService(mockStore, mockFleet, nil, log, nil)

	t.Run("GetActiveIncidents and GetIncident", func(t *testing.T) {
		incidents, err := svc.GetActiveIncidents(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(incidents) != 1 {
			t.Fatalf("expected 1 incident, got %d", len(incidents))
		}

		incID := incidents[0].ID
		singleInc, err := svc.GetIncident(ctx, incID)
		if err != nil {
			t.Fatalf("failed to get incident %s: %v", incID, err)
		}
		if singleInc.ID != incID {
			t.Errorf("expected incident ID %s, got %s", incID, singleInc.ID)
		}

		// Non-existent incident
		_, err = svc.GetIncident(ctx, "inc-non-existent")
		if err != ErrIncidentNotFound {
			t.Errorf("expected ErrIncidentNotFound, got %v", err)
		}
	})

	t.Run("GetCorrelations", func(t *testing.T) {
		corrs, err := svc.GetCorrelations(ctx, 1*time.Hour)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(corrs) == 0 {
			t.Fatalf("expected correlations between cpu, net_tx, and memory, got 0")
		}
	})

	t.Run("GetFindings Filtered", func(t *testing.T) {
		findings, err := svc.GetFindings(ctx, "", model.SeverityInfo)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_ = findings
	})
}

func BenchmarkEvaluateFleetHealth_100Nodes(b *testing.B) {
	ctx := context.Background()
	now := time.Now()

	nodes := make([]model.FleetNode, 100)
	nodeMap := make(map[string]*model.NodeDetailResponse, 100)

	for i := range 100 {
		nodeID := fmt.Sprintf("node-%03d", i)
		hostname := fmt.Sprintf("srv-%03d", i)

		var status model.NodeStatus
		switch i % 5 {
		case 0:
			status = model.NodeStatusWarning
		case 1:
			status = model.NodeStatusCritical
		default:
			status = model.NodeStatusHealthy
		}

		node := model.FleetNode{
			Identity:      model.NodeIdentity{NodeID: nodeID, Hostname: hostname},
			Status:        status,
			LastHeartbeat: now,
			Summary: &model.NodeSummary{
				CPUUsagePercent:    float64(20 + (i % 80)),
				MemoryUsagePercent: float64(30 + (i % 65)),
				DiskUsagePercent:   float64(40 + (i % 55)),
			},
		}
		nodes[i] = node

		nodeMap[nodeID] = &model.NodeDetailResponse{
			Node: node,
			LatestSnapshot: &model.SystemSnapshot{
				Timestamp: now,
				CPU: &model.CPUInfo{
					OverallUsage: node.Summary.CPUUsagePercent,
					LogicalCores: 8,
				},
				Memory: &model.MemoryInfo{
					UsedPercent: node.Summary.MemoryUsagePercent,
					TotalBytes:  16 * 1024 * 1024 * 1024,
				},
				Disk: &model.DiskInfo{
					Partitions: []model.PartitionInfo{
						{Mountpoint: "/", UsedPercent: node.Summary.DiskUsagePercent},
					},
				},
			},
		}
	}

	mockFleet := &mockFleetService{
		list:  nodes,
		nodes: nodeMap,
	}

	log := logger.New(io.Discard, logger.LevelError, false, false)
	// Disable caching for the benchmark to measure raw evaluation throughput
	svc := NewService(nil, mockFleet, nil, log, &ServiceConfig{
		CacheTTL: 0,
	})

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := svc.EvaluateFleetHealth(ctx)
		if err != nil {
			b.Fatalf("benchmark failed: %v", err)
		}
	}
}

func BenchmarkCalculateBaselines_10KPoints(b *testing.B) {
	now := time.Now()
	points := make([]storage.MetricPoint, 10000)

	rnd := rand.New(rand.NewSource(42))
	for i := range 10000 {
		points[i] = storage.MetricPoint{
			Timestamp: now.Add(time.Duration(i) * time.Second),
			Value:     10.0 + rnd.Float64()*90.0,
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = CalculateBaseline("cpu_usage_pct", points, 24*time.Hour)
	}
}
