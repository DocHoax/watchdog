package intelligence

import (
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestEvaluateNodeHealthScore_CleanNode(t *testing.T) {
	snap := &model.SystemSnapshot{
		Timestamp: time.Now(),
		CPU: &model.CPUInfo{
			OverallUsage: 25.0,
			LogicalCores: 4,
			LoadAverage: model.LoadAvg{
				Load1: 1.0,
			},
		},
		Memory: &model.MemoryInfo{
			UsedPercent:     40.0,
			SwapUsedPercent: 10.0,
			AvailableBytes:  8 * 1024 * 1024 * 1024,
			TotalBytes:      16 * 1024 * 1024 * 1024,
		},
		Disk: &model.DiskInfo{
			Partitions: []model.PartitionInfo{
				{
					Mountpoint:  "/",
					UsedPercent: 50.0,
					InodesPct:   30.0,
				},
			},
		},
	}

	scorer := NewScorer(nil)
	score := scorer.Calculate(ScoreEvaluationInput{
		NodeID:      "node-clean-01",
		Hostname:    "web-clean-01",
		Status:      model.NodeStatusHealthy,
		Snapshot:    snap,
		EvaluatedAt: time.Now(),
	})

	if score.Score < 95.0 {
		t.Errorf("expected clean node score >= 95.0, got %f", score.Score)
	}
	if score.NormalizedStatus != model.NodeStatusHealthy {
		t.Errorf("expected normalized status %s, got %s", model.NodeStatusHealthy, score.NormalizedStatus)
	}
	if len(score.PrimaryConcerns) > 0 {
		t.Errorf("expected 0 primary concerns for clean node, got %d: %v", len(score.PrimaryConcerns), score.PrimaryConcerns)
	}
}

func TestEvaluateNodeHealthScore_SubsystemDeductions(t *testing.T) {
	tests := []struct {
		name          string
		input         ScoreEvaluationInput
		expectedMax   float64
		expectedMin   float64
		expectedCheck string
	}{
		{
			name: "Severe CPU and Load Pressure",
			input: ScoreEvaluationInput{
				NodeID:   "node-cpu-01",
				Hostname: "web-cpu-01",
				Status:   model.NodeStatusHealthy,
				Snapshot: &model.SystemSnapshot{
					Timestamp: time.Now(),
					CPU: &model.CPUInfo{
						OverallUsage: 98.0, // severe: -25 pts
						LogicalCores: 4,
						LoadAverage: model.LoadAvg{
							Load1: 16.0, // 4x cores: -20 pts
						},
					},
				},
			},
			expectedMax:   75.0,
			expectedMin:   50.0,
			expectedCheck: "cpu",
		},
		{
			name: "Severe Memory and Swap Saturation",
			input: ScoreEvaluationInput{
				NodeID:   "node-mem-01",
				Hostname: "web-mem-01",
				Status:   model.NodeStatusHealthy,
				Snapshot: &model.SystemSnapshot{
					Timestamp: time.Now(),
					Memory: &model.MemoryInfo{
						UsedPercent:     97.0, // severe: -20 pts
						SwapUsedPercent: 88.0, // severe swap: -20 pts
					},
				},
			},
			expectedMax:   75.0,
			expectedMin:   50.0,
			expectedCheck: "memory",
		},
		{
			name: "Critical Disk and Inode Saturation",
			input: ScoreEvaluationInput{
				NodeID:   "node-disk-01",
				Hostname: "web-disk-01",
				Status:   model.NodeStatusHealthy,
				Snapshot: &model.SystemSnapshot{
					Timestamp: time.Now(),
					Disk: &model.DiskInfo{
						Partitions: []model.PartitionInfo{
							{
								Mountpoint:  "/data",
								UsedPercent: 99.0, // severe: -20 pts
								InodesPct:   96.0, // inode pressure: -10 pts
							},
						},
					},
				},
			},
			expectedMax:   80.0,
			expectedMin:   60.0,
			expectedCheck: "storage",
		},
		{
			name: "Active Alerts and Diagnostics Deductions",
			input: ScoreEvaluationInput{
				NodeID:   "node-alt-01",
				Hostname: "web-alt-01",
				Status:   model.NodeStatusHealthy,
				ActiveAlerts: []model.AlertEvent{
					{
						ID:       "alt-01",
						Severity: model.SeverityCritical,
						RuleName: "DiskFull",
						Message:  "Disk space critical",
						FiredAt:  time.Now(),
						IsActive: true,
					},
					{
						ID:       "alt-02",
						Severity: model.SeverityWarning,
						RuleName: "HighMemory",
						Message:  "Memory usage warning",
						FiredAt:  time.Now(),
						IsActive: true,
					},
				},
				Diagnostics: &model.DiagnosticReport{
					Results: []model.DiagnosticResult{
						{
							ID:          "diag-oom",
							Severity:    model.SeverityCritical,
							Status:      model.StatusFail,
							Name:        "OOM Check",
							Description: "OOM killer invoked",
							Timestamp:   time.Now(),
						},
					},
				},
			},
			expectedMax:   75.0,
			expectedMin:   40.0,
			expectedCheck: "alerts",
		},
		{
			name: "Statistical Anomalies Deductions",
			input: ScoreEvaluationInput{
				NodeID:   "node-anom-01",
				Hostname: "web-anom-01",
				Status:   model.NodeStatusHealthy,
				Anomalies: &model.AnomalyReport{
					Scores: []model.AnomalyScore{
						{
							MetricName: "cpu_usage",
							ZScore:     3.5,
							IsAnomaly:  true,
							Severity:   model.SeverityCritical,
							DetectedAt: time.Now(),
						},
						{
							MetricName: "memory_usage",
							ZScore:     4.2,
							IsAnomaly:  true,
							Severity:   model.SeverityWarning,
							DetectedAt: time.Now(),
						},
					},
				},
			},
			expectedMax:   95.0,
			expectedMin:   85.0,
			expectedCheck: "anomaly",
		},
	}

	scorer := NewScorer(nil)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			score := scorer.Calculate(tc.input)
			if score.Score > tc.expectedMax || score.Score < tc.expectedMin {
				t.Errorf("expected score between %.1f and %.1f, got %.1f", tc.expectedMin, tc.expectedMax, score.Score)
			}

			// Verify factor contribution exists
			var found bool
			for _, factor := range score.Breakdown {
				if factor.Category == tc.expectedCheck {
					found = true
					if factor.Deduction <= 0 {
						t.Errorf("expected deduction > 0 for factor %s, got %.2f", factor.Name, factor.Deduction)
					}
					break
				}
			}
			if !found {
				t.Errorf("expected factor contribution for category %q not found in breakdown: %+v", tc.expectedCheck, score.Breakdown)
			}
		})
	}
}

func TestEvaluateNodeHealthScore_AuthoritativeStatusCapping(t *testing.T) {
	scorer := NewScorer(nil)

	// Critical status must be capped at 49.0
	critInput := ScoreEvaluationInput{
		NodeID:   "node-crit-01",
		Hostname: "web-crit-01",
		Status:   model.NodeStatusCritical,
		Snapshot: &model.SystemSnapshot{
			Timestamp: time.Now(),
			CPU: &model.CPUInfo{
				OverallUsage: 10.0,
				LogicalCores: 4,
			},
		},
	}
	critScore := scorer.Calculate(critInput)
	if critScore.Score > 49.0 {
		t.Errorf("expected critical node score <= 49.0, got %.2f", critScore.Score)
	}

	// Warning status must be capped at 79.0
	warnInput := ScoreEvaluationInput{
		NodeID:   "node-warn-01",
		Hostname: "web-warn-01",
		Status:   model.NodeStatusWarning,
		Snapshot: &model.SystemSnapshot{
			Timestamp: time.Now(),
			CPU: &model.CPUInfo{
				OverallUsage: 10.0,
				LogicalCores: 4,
			},
		},
	}
	warnScore := scorer.Calculate(warnInput)
	if warnScore.Score > 79.0 {
		t.Errorf("expected warning node score <= 79.0, got %.2f", warnScore.Score)
	}

	// Offline status must be set to 0.0
	offlineInput := ScoreEvaluationInput{
		NodeID:   "node-off-01",
		Hostname: "web-off-01",
		Status:   model.NodeStatusOffline,
	}
	offScore := scorer.Calculate(offlineInput)
	if offScore.Score != 0.0 {
		t.Errorf("expected offline node score == 0.0, got %.2f", offScore.Score)
	}

	// Stale status must be set to 0.0
	staleInput := ScoreEvaluationInput{
		NodeID:   "node-stale-01",
		Hostname: "web-stale-01",
		Status:   model.NodeStatusStale,
	}
	staleScore := scorer.Calculate(staleInput)
	if staleScore.Score != 0.0 {
		t.Errorf("expected stale node score == 0.0, got %.2f", staleScore.Score)
	}
}

func TestTrajectoryClassification(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		points   []ScorePoint
		expected Trajectory
	}{
		{
			name:     "Insufficient Data",
			points:   []ScorePoint{},
			expected: TrajectoryUnknown,
		},
		{
			name: "Improving Score Trajectory",
			points: []ScorePoint{
				{Timestamp: now.Add(-30 * time.Minute), Score: 60.0},
				{Timestamp: now.Add(-15 * time.Minute), Score: 70.0},
				{Timestamp: now, Score: 85.0},
			},
			expected: TrajectoryImproving,
		},
		{
			name: "Degrading Score Trajectory",
			points: []ScorePoint{
				{Timestamp: now.Add(-30 * time.Minute), Score: 95.0},
				{Timestamp: now.Add(-15 * time.Minute), Score: 85.0},
				{Timestamp: now, Score: 70.0},
			},
			expected: TrajectoryDegrading,
		},
		{
			name: "Stable Score Trajectory",
			points: []ScorePoint{
				{Timestamp: now.Add(-30 * time.Minute), Score: 88.0},
				{Timestamp: now.Add(-15 * time.Minute), Score: 89.0},
				{Timestamp: now, Score: 87.0},
			},
			expected: TrajectoryStable,
		},
		{
			name: "Volatile Score Trajectory",
			points: []ScorePoint{
				{Timestamp: now.Add(-40 * time.Minute), Score: 95.0},
				{Timestamp: now.Add(-30 * time.Minute), Score: 50.0},
				{Timestamp: now.Add(-20 * time.Minute), Score: 92.0},
				{Timestamp: now.Add(-10 * time.Minute), Score: 48.0},
				{Timestamp: now, Score: 90.0},
			},
			expected: TrajectoryVolatile,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			traj := CalculateTrajectory(tc.points, nil)
			if traj != tc.expected {
				t.Errorf("expected trajectory %s, got %s", tc.expected, traj)
			}
		})
	}
}
