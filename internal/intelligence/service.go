package intelligence

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/DocHoax/watchdog/internal/anomaly"
	"github.com/DocHoax/watchdog/internal/fleet"
	"github.com/DocHoax/watchdog/internal/logger"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/internal/topology"
	"github.com/DocHoax/watchdog/pkg/model"
)

var (
	// ErrIncidentNotFound is returned when an incident ID does not match any active or historical incident.
	ErrIncidentNotFound = errors.New("incident not found")
	// ErrNodeNotFound is returned when the specified node is not found in the fleet registry.
	ErrNodeNotFound = errors.New("node not found")
	// ErrPredictionNotFound is returned when a prediction ID does not match any active prediction.
	ErrPredictionNotFound = errors.New("prediction not found")
)

// IntelligenceService defines the interface for fleet-wide analytical intelligence and health scoring.
type IntelligenceService interface {
	EvaluateNodeHealth(ctx context.Context, nodeID string) (*NodeHealthSummary, error)
	EvaluateFleetHealth(ctx context.Context) (*FleetHealthSummary, error)
	GetNodeTrends(ctx context.Context, nodeID string, window time.Duration) ([]HealthTrend, error)
	GetNodeBaselines(ctx context.Context, nodeID string, window time.Duration) ([]HistoricalBaseline, error)
	GetActiveIncidents(ctx context.Context) ([]Incident, error)
	GetIncident(ctx context.Context, incidentID string) (*Incident, error)
	GetCorrelations(ctx context.Context, window time.Duration) ([]Correlation, error)
	GetFindings(ctx context.Context, category FindingCategory, minSeverity model.Severity) ([]IntelligenceFinding, error)

	// Phase 2B Predictive Operations & Capacity Intelligence
	GetNodePredictions(ctx context.Context, nodeID string, horizon time.Duration) ([]Prediction, error)
	GetNodeCapacityForecast(ctx context.Context, nodeID string, horizon time.Duration) (*NodeCapacityReport, error)
	GetFleetPredictions(ctx context.Context, horizon time.Duration) (*FleetCapacitySummary, error)
	GetRecurringIncidents(ctx context.Context, since time.Duration) ([]RecurrencePattern, error)
	GetPrediction(ctx context.Context, predictionID string) (*Prediction, error)

	// Phase 2D Topology & Root Cause Analysis Intelligence
	AnalyzeIncidentRootCause(ctx context.Context, incidentID string, g *topology.Graph) (*RootCauseReport, error)
}

// ServiceConfig holds optional configuration overrides for the intelligence service.
type ServiceConfig struct {
	ScoringConfig    *ScoringConfig
	TrendConfig      *TrendConfig
	TrajectoryConfig *TrajectoryConfig
	CacheTTL         time.Duration
}

type intelligenceService struct {
	store         storage.ReadOnlyStorage
	fleetSvc      fleet.ReadOnlyFleetService
	anomaly       *anomaly.Detector
	scorer        *Scorer
	trendCfg      *TrendConfig
	trajectoryCfg *TrajectoryConfig
	clusterer     *IncidentClusterer
	fleetAnalyzer *FleetPatternAnalyzer
	corrEngine    *CorrelationEngine
	log           *logger.Logger

	mu              sync.RWMutex
	fleetCache      *FleetHealthSummary
	fleetCacheUntil time.Time
	cacheTTL        time.Duration
}

// NewService instantiates a new IntelligenceService with the provided read-only dependencies.
func NewService(store storage.ReadOnlyStorage, fleetSvc fleet.ReadOnlyFleetService,
	anomDetector *anomaly.Detector, log *logger.Logger, cfg *ServiceConfig) IntelligenceService {

	var scoringCfg *ScoringConfig
	var trendCfg *TrendConfig
	var trajCfg *TrajectoryConfig
	cacheTTL := 5 * time.Second

	if cfg != nil {
		scoringCfg = cfg.ScoringConfig
		trendCfg = cfg.TrendConfig
		trajCfg = cfg.TrajectoryConfig
		if cfg.CacheTTL > 0 {
			cacheTTL = cfg.CacheTTL
		}
	}

	return &intelligenceService{
		store:         store,
		fleetSvc:      fleetSvc,
		anomaly:       anomDetector,
		scorer:        NewScorer(scoringCfg),
		trendCfg:      trendCfg,
		trajectoryCfg: trajCfg,
		clusterer:     NewIncidentClusterer(15 * time.Minute),
		fleetAnalyzer: NewFleetPatternAnalyzer(),
		corrEngine:    NewCorrelationEngine(1 * time.Hour),
		log:           log,
		cacheTTL:      cacheTTL,
	}
}

// EvaluateNodeHealth evaluates the health, trends, baselines, and incidents for a specific node.
func (s *intelligenceService) EvaluateNodeHealth(ctx context.Context, nodeID string) (*NodeHealthSummary, error) {
	if nodeID == "" {
		return nil, ErrNodeNotFound
	}

	var node *model.FleetNode
	var latestSnap *model.SystemSnapshot
	var latestDiag *model.DiagnosticReport
	var activeAlerts []model.AlertEvent

	if s.fleetSvc != nil {
		detail, err := s.fleetSvc.GetNode(ctx, nodeID)
		if err == nil && detail != nil {
			node = &detail.Node
			latestSnap = detail.LatestSnapshot
			latestDiag = detail.LatestDiagnostics
			activeAlerts = detail.ActiveAlerts
		}
	}

	if node == nil && s.store != nil {
		n, err := s.store.GetFleetNode(ctx, nodeID)
		if err == nil && n != nil {
			node = n
		}
	}

	if node == nil {
		return nil, ErrNodeNotFound
	}

	// If snapshot is missing, create a synthetic snapshot from node metrics if available
	if latestSnap == nil {
		var cpuPct, memPct, diskPct float64
		if node.Summary != nil {
			cpuPct = node.Summary.CPUUsagePercent
			memPct = node.Summary.MemoryUsagePercent
			diskPct = node.Summary.DiskUsagePercent
		}
		latestSnap = &model.SystemSnapshot{
			Timestamp: node.LastHeartbeat,
			CPU: &model.CPUInfo{
				OverallUsage: cpuPct,
			},
			Memory: &model.MemoryInfo{
				UsedPercent: memPct,
			},
			Disk: &model.DiskInfo{
				UsedPercent: diskPct,
			},
		}
	}

	// Feed anomaly detector if present
	var anomReport *model.AnomalyReport
	if s.anomaly != nil && latestSnap != nil {
		anomReport = s.anomaly.FeedSnapshot(latestSnap)
	}

	nodeIdentifier := node.Identity.NodeID
	if nodeIdentifier == "" {
		nodeIdentifier = nodeID
	}
	hostname := node.Identity.Hostname
	if hostname == "" {
		hostname = nodeIdentifier
	}

	// Calculate base health score
	scoreInput := ScoreEvaluationInput{
		NodeID:       nodeIdentifier,
		Hostname:     hostname,
		Status:       node.Status,
		Snapshot:     latestSnap,
		Diagnostics:  latestDiag,
		ActiveAlerts: activeAlerts,
		Anomalies:    anomReport,
		EvaluatedAt:  time.Now(),
	}
	healthScore := s.scorer.Calculate(scoreInput)

	// Fetch historical telemetry submissions for trajectory and trends if storage available
	var historySubmissions []model.TelemetrySubmission
	if s.store != nil {
		since := time.Now().Add(-1 * time.Hour)
		subs, err := s.store.GetNodeTelemetrySubmissions(ctx, nodeIdentifier, since, 60)
		if err == nil {
			historySubmissions = subs
		}
	}

	// Calculate Trajectory from historical submissions
	if len(historySubmissions) >= 2 {
		var scorePoints []ScorePoint
		for _, sub := range historySubmissions {
			subScore := s.scorer.Calculate(ScoreEvaluationInput{
				NodeID:       nodeIdentifier,
				Hostname:     hostname,
				Status:       node.Status,
				Snapshot:     sub.Snapshot,
				Diagnostics:  sub.Diagnostics,
				ActiveAlerts: sub.ActiveAlerts,
				EvaluatedAt:  sub.Timestamp,
			})
			scorePoints = append(scorePoints, ScorePoint{
				Score:     subScore.Score,
				Timestamp: sub.Timestamp,
			})
		}
		healthScore.Trajectory = CalculateTrajectory(scorePoints, s.trajectoryCfg)
	}

	// Calculate trends and baselines over default 1h window
	trends, _ := s.GetNodeTrends(ctx, nodeIdentifier, 1*time.Hour)
	baselines, _ := s.GetNodeBaselines(ctx, nodeIdentifier, 24*time.Hour)

	// Cluster active incidents
	incidents := s.clusterer.ClusterNodeEvents(nodeIdentifier, hostname, activeAlerts, latestDiag, anomReport)

	// Build node-level findings
	var findings []IntelligenceFinding
	for _, f := range healthScore.Breakdown {
		if f.Deduction >= 8.0 {
			findings = append(findings, IntelligenceFinding{
				ID:            fmt.Sprintf("find-node-%s-%s", nodeIdentifier, f.Category),
				Category:      FindingCategoryResourceExhaustion,
				Severity:      model.SeverityWarning,
				Confidence:    FindingConfidenceHigh,
				Title:         fmt.Sprintf("%s on %s", f.Name, hostname),
				Description:   f.Explanation,
				AffectedNodes: []string{nodeIdentifier},
				SupportingEvidence: []string{
					fmt.Sprintf("Deducted %.1f points from health score (Category: %s)", f.Deduction, f.Category),
				},
				NonInvasiveSuggestions: []string{
					"Review resource utilization metrics and top processes on the node",
				},
				DetectedAt: time.Now(),
			})
		}
	}

	return &NodeHealthSummary{
		NodeID:          nodeIdentifier,
		Hostname:        hostname,
		Status:          node.Status,
		HealthScore:     healthScore,
		Trends:          trends,
		Baselines:       baselines,
		ActiveIncidents: incidents,
		Findings:        findings,
		EvaluatedAt:     time.Now(),
	}, nil
}

// EvaluateFleetHealth aggregates health scoring, incidents, and cross-node findings across the fleet.
func (s *intelligenceService) EvaluateFleetHealth(ctx context.Context) (*FleetHealthSummary, error) {
	s.mu.RLock()
	if s.fleetCache != nil && time.Now().Before(s.fleetCacheUntil) {
		cached := *s.fleetCache
		s.mu.RUnlock()
		return &cached, nil
	}
	s.mu.RUnlock()

	var nodes []model.FleetNode
	if s.fleetSvc != nil {
		res, err := s.fleetSvc.ListNodes(ctx, model.FleetFilter{Limit: 1000})
		if err == nil && res != nil {
			nodes = res.Nodes
		}
	}

	if len(nodes) == 0 && s.store != nil {
		ns, _, err := s.store.ListFleetNodes(ctx, model.FleetFilter{Limit: 1000})
		if err == nil {
			nodes = ns
		}
	}

	now := time.Now()
	summary := &FleetHealthSummary{
		EvaluatedAt: now,
		TotalNodes:  len(nodes),
	}

	if len(nodes) == 0 {
		return summary, nil
	}

	var nodeSummaries []NodeHealthSummary
	totalScore := 0.0

	for _, n := range nodes {
		switch n.Status {
		case model.NodeStatusHealthy:
			summary.HealthyCount++
		case model.NodeStatusWarning:
			summary.WarningCount++
		case model.NodeStatusCritical:
			summary.CriticalCount++
		case model.NodeStatusStale:
			summary.StaleCount++
		case model.NodeStatusOffline:
			summary.OfflineCount++
		}

		nodeSum, err := s.EvaluateNodeHealth(ctx, n.Identity.NodeID)
		if err == nil && nodeSum != nil {
			nodeSummaries = append(nodeSummaries, *nodeSum)
			totalScore += nodeSum.HealthScore.Score
		}
	}

	if len(nodeSummaries) > 0 {
		summary.AverageScore = math.Round((totalScore/float64(len(nodeSummaries)))*10) / 10
	}

	// Sort lowest scoring nodes
	sort.Slice(nodeSummaries, func(i, j int) bool {
		return nodeSummaries[i].HealthScore.Score < nodeSummaries[j].HealthScore.Score
	})

	limitLowest := 5
	if len(nodeSummaries) < limitLowest {
		limitLowest = len(nodeSummaries)
	}
	summary.LowestScoringNodes = append([]NodeHealthSummary{}, nodeSummaries[:limitLowest]...)

	// Cross-node fleet pattern analysis
	summary.FleetFindings = s.fleetAnalyzer.AnalyzeFleet(nodeSummaries)

	// Collect active incidents from all nodes
	var allIncidents []Incident
	for _, ns := range nodeSummaries {
		allIncidents = append(allIncidents, ns.ActiveIncidents...)
	}
	summary.ActiveIncidents = allIncidents

	// Populate fleet trends if storage is available
	summary.FleetTrends = s.computeFleetTrends(ctx)

	// Update cache
	s.mu.Lock()
	s.fleetCache = summary
	s.fleetCacheUntil = now.Add(s.cacheTTL)
	s.mu.Unlock()

	return summary, nil
}

// computeFleetTrends computes high-level average trends across the fleet.
func (s *intelligenceService) computeFleetTrends(ctx context.Context) []HealthTrend {
	if s.store == nil {
		return nil
	}

	now := time.Now()
	startTime := now.Add(-1 * time.Hour)

	var trends []HealthTrend
	metricsToTrack := []struct {
		name string
		unit string
	}{
		{"cpu_usage_pct", "%/hr"},
		{"memory_used_pct", "%/hr"},
		{"disk_used_pct", "%/hr"},
	}

	for _, m := range metricsToTrack {
		pts, err := s.store.QueryMetrics(ctx, storage.TimeRangeQuery{
			Metric:    m.name,
			StartTime: startTime,
			EndTime:   now,
			Limit:     100,
		})
		if err == nil && len(pts) >= 3 {
			t := CalculateTrend(m.name, pts, 1*time.Hour, m.unit, s.trendCfg)
			trends = append(trends, t)
		}
	}

	return trends
}

// GetNodeTrends returns statistical trends for a node over a sliding window.
func (s *intelligenceService) GetNodeTrends(ctx context.Context, nodeID string, window time.Duration) ([]HealthTrend, error) {
	if window <= 0 {
		window = 1 * time.Hour
	}

	if s.store == nil {
		return nil, nil
	}

	now := time.Now()
	startTime := now.Add(-window)

	metrics := []struct {
		name string
		unit string
	}{
		{"cpu_usage_pct", "%/hr"},
		{"memory_used_pct", "%/hr"},
		{"disk_used_pct", "%/hr"},
		{"net_rx_rate", "B/s/hr"},
		{"net_tx_rate", "B/s/hr"},
	}

	var results []HealthTrend
	for _, m := range metrics {
		pts, err := s.store.QueryMetrics(ctx, storage.TimeRangeQuery{
			Metric:    m.name,
			StartTime: startTime,
			EndTime:   now,
			Limit:     200,
		})
		if err == nil {
			t := CalculateTrend(m.name, pts, window, m.unit, s.trendCfg)
			results = append(results, t)
		}
	}

	return results, nil
}

// GetNodeBaselines returns historical baseline distributions for a node over a window.
func (s *intelligenceService) GetNodeBaselines(ctx context.Context, nodeID string, window time.Duration) ([]HistoricalBaseline, error) {
	if window <= 0 {
		window = 24 * time.Hour
	}

	if s.store == nil {
		return nil, nil
	}

	now := time.Now()
	startTime := now.Add(-window)

	metrics := []string{
		"cpu_usage_pct",
		"memory_used_pct",
		"disk_used_pct",
		"net_rx_rate",
		"net_tx_rate",
	}

	var results []HistoricalBaseline
	for _, m := range metrics {
		pts, err := s.store.QueryMetrics(ctx, storage.TimeRangeQuery{
			Metric:    m,
			StartTime: startTime,
			EndTime:   now,
			Limit:     1000,
		})
		if err == nil && len(pts) > 0 {
			b := CalculateBaseline(m, pts, window)
			results = append(results, b)
		}
	}

	return results, nil
}

// GetActiveIncidents returns all open incidents across the fleet.
func (s *intelligenceService) GetActiveIncidents(ctx context.Context) ([]Incident, error) {
	fleetSum, err := s.EvaluateFleetHealth(ctx)
	if err != nil {
		return nil, err
	}
	return fleetSum.ActiveIncidents, nil
}

// GetIncident retrieves a single incident by its unique ID.
func (s *intelligenceService) GetIncident(ctx context.Context, incidentID string) (*Incident, error) {
	incidents, err := s.GetActiveIncidents(ctx)
	if err != nil {
		return nil, err
	}

	for _, inc := range incidents {
		if inc.ID == incidentID {
			return &inc, nil
		}
	}

	return nil, ErrIncidentNotFound
}

// GetCorrelations evaluates temporal associations between key metric streams.
func (s *intelligenceService) GetCorrelations(ctx context.Context, window time.Duration) ([]Correlation, error) {
	if s.store == nil {
		return nil, nil
	}

	if window <= 0 {
		window = 1 * time.Hour
	}

	now := time.Now()
	startTime := now.Add(-window)

	// Fetch primary metric points
	cpuPts, _ := s.store.QueryMetrics(ctx, storage.TimeRangeQuery{Metric: "cpu_usage_pct", StartTime: startTime, EndTime: now, Limit: 500})
	memPts, _ := s.store.QueryMetrics(ctx, storage.TimeRangeQuery{Metric: "memory_used_pct", StartTime: startTime, EndTime: now, Limit: 500})
	txPts, _ := s.store.QueryMetrics(ctx, storage.TimeRangeQuery{Metric: "net_tx_rate", StartTime: startTime, EndTime: now, Limit: 500})
	swapPts, _ := s.store.QueryMetrics(ctx, storage.TimeRangeQuery{Metric: "swap_used_pct", StartTime: startTime, EndTime: now, Limit: 500})

	var correlations []Correlation

	// CPU vs Network TX
	if c := s.corrEngine.CorrelateSignals("cpu_usage_pct", cpuPts, "net_tx_rate", txPts); c != nil {
		correlations = append(correlations, *c)
	}

	// Memory vs Swap
	if c := s.corrEngine.CorrelateSignals("memory_used_pct", memPts, "swap_used_pct", swapPts); c != nil {
		correlations = append(correlations, *c)
	}

	// CPU vs Memory
	if c := s.corrEngine.CorrelateSignals("cpu_usage_pct", cpuPts, "memory_used_pct", memPts); c != nil {
		correlations = append(correlations, *c)
	}

	return correlations, nil
}

// GetFindings returns intelligence findings filtered by category and minimum severity.
func (s *intelligenceService) GetFindings(ctx context.Context, category FindingCategory, minSeverity model.Severity) ([]IntelligenceFinding, error) {
	fleetSum, err := s.EvaluateFleetHealth(ctx)
	if err != nil {
		return nil, err
	}

	var matched []IntelligenceFinding
	for _, f := range fleetSum.FleetFindings {
		if category != "" && f.Category != category {
			continue
		}
		if minSeverity != "" && !isSeverityAtLeast(f.Severity, minSeverity) {
			continue
		}
		matched = append(matched, f)
	}

	return matched, nil
}

// GetNodePredictions returns deterministic metric threshold predictions for a node.
func (s *intelligenceService) GetNodePredictions(ctx context.Context, nodeID string, horizon time.Duration) ([]Prediction, error) {
	rep, err := s.GetNodeCapacityForecast(ctx, nodeID, horizon)
	if err != nil {
		return nil, err
	}
	return rep.Predictions, nil
}

// GetNodeCapacityForecast evaluates multi-resource capacity forecasts and runway projections for a node.
func (s *intelligenceService) GetNodeCapacityForecast(ctx context.Context, nodeID string, horizon time.Duration) (*NodeCapacityReport, error) {
	if nodeID == "" {
		return nil, ErrNodeNotFound
	}

	var node *model.FleetNode
	if s.fleetSvc != nil {
		detail, err := s.fleetSvc.GetNode(ctx, nodeID)
		if err == nil && detail != nil {
			node = &detail.Node
		}
	}
	if node == nil && s.store != nil {
		n, err := s.store.GetFleetNode(ctx, nodeID)
		if err == nil && n != nil {
			node = n
		}
	}
	if node == nil {
		return nil, ErrNodeNotFound
	}

	nodeIdentifier := node.Identity.NodeID
	if nodeIdentifier == "" {
		nodeIdentifier = nodeID
	}
	hostname := node.Identity.Hostname
	if hostname == "" {
		hostname = nodeIdentifier
	}

	if horizon <= 0 {
		horizon = 24 * time.Hour
	}
	obsWindow := 1 * time.Hour
	if horizon > 24*time.Hour {
		obsWindow = 24 * time.Hour
	}

	metricPoints := make(map[string][]storage.MetricPoint)

	if s.store != nil {
		now := time.Now()
		startTime := now.Add(-obsWindow)

		// 1. Check direct metric store
		for _, m := range []string{"cpu.usage_percent", "memory.usage_percent", "swap.usage_percent", "disk.usage_percent"} {
			pts, err := s.store.QueryMetrics(ctx, storage.TimeRangeQuery{
				Metric:    m,
				StartTime: startTime,
				EndTime:   now,
				Limit:     500,
			})
			if err == nil && len(pts) > 0 {
				metricPoints[m] = pts
			}
		}

		// Check alternative names if empty
		altMap := map[string]string{
			"cpu.usage_percent":    "cpu_usage_pct",
			"memory.usage_percent": "memory_used_pct",
			"swap.usage_percent":   "swap_used_pct",
			"disk.usage_percent":   "disk_used_pct",
		}
		for canonical, alt := range altMap {
			if len(metricPoints[canonical]) == 0 {
				pts, err := s.store.QueryMetrics(ctx, storage.TimeRangeQuery{
					Metric:    alt,
					StartTime: startTime,
					EndTime:   now,
					Limit:     500,
				})
				if err == nil && len(pts) > 0 {
					metricPoints[canonical] = pts
				}
			}
		}

		// 2. Also extract from historical telemetry submissions if needed
		if len(metricPoints["cpu.usage_percent"]) < 10 {
			subs, err := s.store.GetNodeTelemetrySubmissions(ctx, nodeIdentifier, startTime, 500)
			if err == nil && len(subs) > 0 {
				var cpuPts, memPts, swapPts, diskPts []storage.MetricPoint
				for _, sub := range subs {
					t := sub.Timestamp
					if sub.Snapshot != nil {
						if sub.Snapshot.CPU != nil {
							cpuPts = append(cpuPts, storage.MetricPoint{Metric: "cpu.usage_percent", Timestamp: t, Value: sub.Snapshot.CPU.OverallUsage})
						}
						if sub.Snapshot.Memory != nil {
							memPts = append(memPts, storage.MetricPoint{Metric: "memory.usage_percent", Timestamp: t, Value: sub.Snapshot.Memory.UsedPercent})
							swapPts = append(swapPts, storage.MetricPoint{Metric: "swap.usage_percent", Timestamp: t, Value: sub.Snapshot.Memory.SwapUsedPercent})
						}
						if sub.Snapshot.Disk != nil {
							diskPts = append(diskPts, storage.MetricPoint{Metric: "disk.usage_percent", Timestamp: t, Value: sub.Snapshot.Disk.UsedPercent})
						}
					}
				}
				if len(metricPoints["cpu.usage_percent"]) < len(cpuPts) {
					metricPoints["cpu.usage_percent"] = cpuPts
				}
				if len(metricPoints["memory.usage_percent"]) < len(memPts) {
					metricPoints["memory.usage_percent"] = memPts
				}
				if len(metricPoints["swap.usage_percent"]) < len(swapPts) {
					metricPoints["swap.usage_percent"] = swapPts
				}
				if len(metricPoints["disk.usage_percent"]) < len(diskPts) {
					metricPoints["disk.usage_percent"] = diskPts
				}
			}
		}
	}

	report := EvaluateNodeCapacity(nodeIdentifier, hostname, node.Status, metricPoints, horizon, obsWindow, nil)
	return report, nil
}

// GetFleetPredictions evaluates fleet-wide capacity forecasting and aggregated threshold crossings.
func (s *intelligenceService) GetFleetPredictions(ctx context.Context, horizon time.Duration) (*FleetCapacitySummary, error) {
	if horizon <= 0 {
		horizon = 24 * time.Hour
	}

	var nodes []model.FleetNode
	if s.fleetSvc != nil {
		res, err := s.fleetSvc.ListNodes(ctx, model.FleetFilter{Limit: 1000})
		if err == nil && res != nil {
			nodes = res.Nodes
		}
	}
	if len(nodes) == 0 && s.store != nil {
		ns, _, err := s.store.ListFleetNodes(ctx, model.FleetFilter{Limit: 1000})
		if err == nil {
			nodes = ns
		}
	}

	var reports []NodeCapacityReport
	for _, n := range nodes {
		rep, err := s.GetNodeCapacityForecast(ctx, n.Identity.NodeID, horizon)
		if err == nil && rep != nil {
			reports = append(reports, *rep)
		}
	}

	summary := AggregateFleetCapacity(reports, horizon)
	return summary, nil
}

// GetRecurringIncidents detects periodic or recurring operational incident patterns across a lookback window.
func (s *intelligenceService) GetRecurringIncidents(ctx context.Context, since time.Duration) ([]RecurrencePattern, error) {
	if since <= 0 {
		since = 24 * time.Hour
	}

	// Fetch active and recent incidents
	activeIncidents, err := s.GetActiveIncidents(ctx)
	if err != nil {
		return nil, err
	}

	// Also fetch historical alerts if available from store to synthesize historical incident clusters
	var historicalIncidents []Incident
	historicalIncidents = append(historicalIncidents, activeIncidents...)

	if s.store != nil {
		history, err := s.store.GetAlertHistory(ctx, 500, 0)
		if err == nil && len(history) > 0 {
			clustered := s.clusterer.ClusterNodeEvents("fleet", "fleet", history, nil, nil)
			historicalIncidents = append(historicalIncidents, clustered...)
		}
	}

	patterns := DetectRecurrencePatterns(historicalIncidents, since, nil)
	return patterns, nil
}

// GetPrediction retrieves a single deterministic prediction by its unique ID across the fleet.
func (s *intelligenceService) GetPrediction(ctx context.Context, predictionID string) (*Prediction, error) {
	if predictionID == "" {
		return nil, ErrPredictionNotFound
	}

	fleetPreds, err := s.GetFleetPredictions(ctx, 7*24*time.Hour)
	if err != nil {
		return nil, err
	}

	for _, p := range fleetPreds.FleetPredictions {
		if p.ID == predictionID {
			return &p, nil
		}
	}

	for _, rep := range fleetPreds.TopCapacityRisks {
		for _, p := range rep.Predictions {
			if p.ID == predictionID {
				return &p, nil
			}
		}
	}

	return nil, ErrPredictionNotFound
}

// AnalyzeIncidentRootCause evaluates deterministic root-cause candidates for an incident cluster against the topology graph.
func (s *intelligenceService) AnalyzeIncidentRootCause(ctx context.Context, incidentID string, g *topology.Graph) (*RootCauseReport, error) {
	if incidentID == "" {
		return nil, ErrIncidentNotFound
	}

	inc, err := s.GetIncident(ctx, incidentID)
	if err != nil {
		return nil, err
	}

	// Fetch predictions and recurrence patterns for context
	var preds []Prediction
	capReport, err := s.GetFleetPredictions(ctx, 24*time.Hour)
	if err == nil && capReport != nil {
		preds = capReport.FleetPredictions
	}

	recurrence, _ := s.GetRecurringIncidents(ctx, 24*time.Hour)

	rcaEngine := NewRootCauseEngine()
	report := rcaEngine.AnalyzeIncident(inc, g, preds, recurrence)
	return report, nil
}

func isSeverityAtLeast(sev model.Severity, minSev model.Severity) bool {
	rank := func(s model.Severity) int {
		switch s {
		case model.SeverityCritical:
			return 3
		case model.SeverityWarning:
			return 2
		case model.SeverityInfo:
			return 1
		default:
			return 0
		}
	}
	return rank(sev) >= rank(minSev)
}
