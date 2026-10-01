package governance

import (
	"context"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

func TestGovernanceService_E2E_EvaluationLifecycle(t *testing.T) {
	ctx := context.Background()
	store, err := storage.NewSQLiteStorage(storage.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to initialize sqlite storage: %v", err)
	}
	defer store.Close()

	baseTime := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	clock := NewMockClock(baseTime)
	svc := NewGovernanceServiceWithClock(store, clock)

	orgID := "org-enterprise-corp"

	// 1. Setup Organization & Fleet Hierarchy
	if err := svc.CreateOrganization(ctx, &model.Organization{
		ID:          orgID,
		Name:        "Enterprise Corp",
		DisplayName: "Enterprise Corp Global",
	}); err != nil {
		t.Fatalf("failed to create organization: %v", err)
	}

	rootGrp := &model.FleetGroup{
		ID:    "grp-root",
		OrgID: orgID,
		Name:  "Global Infrastructure",
		Type:  model.GroupTypeCustom,
	}
	if err := svc.CreateFleetGroup(ctx, rootGrp); err != nil {
		t.Fatalf("failed to create root group: %v", err)
	}

	prodGrp := &model.FleetGroup{
		ID:            "grp-prod",
		OrgID:         orgID,
		ParentGroupID: "grp-root",
		Name:          "Production Tier",
		Type:          model.GroupTypeEnvironment,
	}
	if err := svc.CreateFleetGroup(ctx, prodGrp); err != nil {
		t.Fatalf("failed to create prod group: %v", err)
	}

	dbGrp := &model.FleetGroup{
		ID:            "grp-db-cluster",
		OrgID:         orgID,
		ParentGroupID: "grp-prod",
		Name:          "Database Cluster",
		Type:          model.GroupTypeTier,
	}
	if err := svc.CreateFleetGroup(ctx, dbGrp); err != nil {
		t.Fatalf("failed to create db group: %v", err)
	}

	// 2. Setup Fleet Nodes
	nodeDB := &model.FleetNode{
		Identity: model.NodeIdentity{
			NodeID:      "node-db-01",
			Hostname:    "db-01.prod.corp",
			Platform:    "linux",
			CPUCores:    16,
			TotalMemory: 64 * 1024 * 1024 * 1024,
			Tags: map[string]string{
				"env":  "production",
				"tier": "backend",
				"role": "database",
			},
		},
		Status:        model.NodeStatusHealthy,
		LastHeartbeat: clock.Now(),
		Metadata: map[string]string{
			"org_id": orgID,
			"env":    "production",
			"tier":   "backend",
			"role":   "database",
		},
	}
	if err := store.SaveFleetNode(ctx, nodeDB); err != nil {
		t.Fatalf("failed to save nodeDB: %v", err)
	}

	nodeWeb := &model.FleetNode{
		Identity: model.NodeIdentity{
			NodeID:      "node-web-01",
			Hostname:    "web-01.prod.corp",
			Platform:    "linux",
			CPUCores:    8,
			TotalMemory: 16 * 1024 * 1024 * 1024,
			Tags: map[string]string{
				"env":  "production",
				"tier": "frontend",
				"role": "web",
			},
		},
		Status:        model.NodeStatusHealthy,
		LastHeartbeat: clock.Now(),
		Metadata: map[string]string{
			"org_id": orgID,
			"env":    "production",
			"tier":   "frontend",
			"role":   "web",
		},
	}
	if err := store.SaveFleetNode(ctx, nodeWeb); err != nil {
		t.Fatalf("failed to save nodeWeb: %v", err)
	}

	nodeDev := &model.FleetNode{
		Identity: model.NodeIdentity{
			NodeID:      "node-dev-01",
			Hostname:    "dev-01.dev.corp",
			Platform:    "linux",
			CPUCores:    4,
			TotalMemory: 8 * 1024 * 1024 * 1024,
			Tags: map[string]string{
				"env":  "development",
				"tier": "backend",
				"role": "worker",
			},
		},
		Status:        model.NodeStatusHealthy,
		LastHeartbeat: clock.Now().Add(-15 * time.Minute), // stale heartbeat for 5m limit
		Metadata: map[string]string{
			"org_id": orgID,
			"env":    "development",
			"tier":   "backend",
			"role":   "worker",
		},
	}
	if err := store.SaveFleetNode(ctx, nodeDev); err != nil {
		t.Fatalf("failed to save nodeDev: %v", err)
	}

	// Assign group memberships
	if err := svc.AddMember(ctx, &model.FleetGroupMember{GroupID: "grp-db-cluster", NodeID: "node-db-01"}); err != nil {
		t.Fatalf("failed to add member: %v", err)
	}
	if err := svc.AddMember(ctx, &model.FleetGroupMember{GroupID: "grp-prod", NodeID: "node-web-01"}); err != nil {
		t.Fatalf("failed to add member: %v", err)
	}
	if err := svc.AddMember(ctx, &model.FleetGroupMember{GroupID: "grp-root", NodeID: "node-dev-01"}); err != nil {
		t.Fatalf("failed to add member: %v", err)
	}

	// 3. Ingest Telemetry Submissions
	// nodeDB: high CPU (92%), memory (88%), disk (94%) -> will breach thresholds
	subDB := &model.TelemetrySubmission{
		NodeID:    "node-db-01",
		Timestamp: clock.Now(),
		Metrics: map[string]float64{
			"cpu_usage_pct":    92.0,
			"memory_usage_pct": 88.0,
			"disk_usage_pct":   94.0,
		},
	}
	if err := store.SaveTelemetrySubmission(ctx, subDB); err != nil {
		t.Fatalf("failed to save telemetry DB: %v", err)
	}

	// nodeWeb: normal metrics
	subWeb := &model.TelemetrySubmission{
		NodeID:    "node-web-01",
		Timestamp: clock.Now(),
		Metrics: map[string]float64{
			"cpu_usage_pct":    45.0,
			"memory_usage_pct": 50.0,
			"disk_usage_pct":   40.0,
		},
	}
	if err := store.SaveTelemetrySubmission(ctx, subWeb); err != nil {
		t.Fatalf("failed to save telemetry Web: %v", err)
	}

	// nodeDev: low metrics, but telemetry 15m ago (stale heartbeat for a 5m check)
	subDev := &model.TelemetrySubmission{
		NodeID:    "node-dev-01",
		Timestamp: clock.Now().Add(-15 * time.Minute),
		Metrics: map[string]float64{
			"cpu_usage_pct":    10.0,
			"memory_usage_pct": 20.0,
			"disk_usage_pct":   15.0,
		},
	}
	if err := store.SaveTelemetrySubmission(ctx, subDev); err != nil {
		t.Fatalf("failed to save telemetry Dev: %v", err)
	}

	// 4. Create Policies, Revisions, and Assignments
	// A. Org Guardrails (Heartbeat check + mandatory tag 'tier')
	polGuard := &model.Policy{
		ID:             "pol-guardrails",
		OrgID:          orgID,
		Name:           "Org Security Guardrails",
		Category:       model.PolicyCategoryOperationalCompliance,
		Status:         model.PolicyStatusActive,
		ActiveRevision: 1,
	}
	if err := svc.CreatePolicy(ctx, polGuard); err != nil {
		t.Fatalf("failed to create guardrail policy: %v", err)
	}
	revGuard := &model.PolicyRevision{
		PolicyID:        "pol-guardrails",
		Revision:        1,
		Priority:        50,
		InheritanceMode: model.InheritanceModeAdditive,
		EnforcementMode: model.EnforcementModeEnforce,
		Rules: []model.PolicyRule{
			{
				ID:       "rule-heartbeat",
				Name:     "Heartbeat Freshness",
				Type:     model.RuleTypeOperationalCompliance,
				Severity: model.SeverityCritical,
				Enabled:  true,
				OperationalCompliance: &model.OperationalComplianceRuleConfig{
					CheckType:         "heartbeat_freshness",
					MaxAgeSeconds:     300,
					ViolationSeverity: model.SeverityCritical,
				},
			},
			{
				ID:       "rule-mandatory-tag",
				Name:     "Mandatory Tier Tag",
				Type:     model.RuleTypeOperationalCompliance,
				Severity: model.SeverityWarning,
				Enabled:  true,
				OperationalCompliance: &model.OperationalComplianceRuleConfig{
					CheckType:         "mandatory_tags",
					ExpectedValues:    []string{"tier"},
					ViolationSeverity: model.SeverityWarning,
				},
			},
		},
	}
	if err := svc.PublishRevision(ctx, revGuard); err != nil {
		t.Fatalf("failed to publish guardrail revision: %v", err)
	}
	if err := svc.CreateAssignment(ctx, &model.PolicyAssignment{
		ID:         "asgn-guard-org",
		OrgID:      orgID,
		PolicyID:   "pol-guardrails",
		TargetType: model.TargetTypeOrganization,
		TargetID:   orgID,
		Enabled:    true,
	}); err != nil {
		t.Fatalf("failed to create guardrail assignment: %v", err)
	}

	// B. Prod Resource Thresholds (CPU < 90, Disk < 90)
	polProd := &model.Policy{
		ID:             "pol-prod-thresholds",
		OrgID:          orgID,
		Name:           "Production Resource Thresholds",
		Category:       model.PolicyCategoryResourceThresholds,
		Status:         model.PolicyStatusActive,
		ActiveRevision: 1,
	}
	if err := svc.CreatePolicy(ctx, polProd); err != nil {
		t.Fatalf("failed to create prod policy: %v", err)
	}
	revProd := &model.PolicyRevision{
		PolicyID:        "pol-prod-thresholds",
		Revision:        1,
		Priority:        100,
		Selector:        "tags.env == 'production'",
		InheritanceMode: model.InheritanceModeInheritAndOverride,
		EnforcementMode: model.EnforcementModeEnforce,
		Rules: []model.PolicyRule{
			{
				ID:       "rule-cpu-threshold",
				Name:     "CPU Threshold",
				Type:     model.RuleTypeResourceThreshold,
				Severity: model.SeverityCritical,
				Enabled:  true,
				ResourceThreshold: &model.ResourceThresholdRuleConfig{
					Metric:            "cpu_usage_pct",
					WarningThreshold:  80,
					CriticalThreshold: 90,
				},
			},
			{
				ID:       "rule-disk-threshold",
				Name:     "Disk Threshold",
				Type:     model.RuleTypeResourceThreshold,
				Severity: model.SeverityCritical,
				Enabled:  true,
				ResourceThreshold: &model.ResourceThresholdRuleConfig{
					Metric:            "disk_usage_pct",
					WarningThreshold:  85,
					CriticalThreshold: 90,
				},
			},
		},
	}
	if err := svc.PublishRevision(ctx, revProd); err != nil {
		t.Fatalf("failed to publish prod revision: %v", err)
	}
	if err := svc.CreateAssignment(ctx, &model.PolicyAssignment{
		ID:         "asgn-prod-grp",
		OrgID:      orgID,
		PolicyID:   "pol-prod-thresholds",
		TargetType: model.TargetTypeFleetGroup,
		TargetID:   "grp-prod",
		Enabled:    true,
	}); err != nil {
		t.Fatalf("failed to create prod assignment: %v", err)
	}

	// C. DB Cluster Memory Policy (Memory < 85)
	polDB := &model.Policy{
		ID:             "pol-db-mem",
		OrgID:          orgID,
		Name:           "Database Memory Threshold",
		Category:       model.PolicyCategoryResourceThresholds,
		Status:         model.PolicyStatusActive,
		ActiveRevision: 1,
	}
	if err := svc.CreatePolicy(ctx, polDB); err != nil {
		t.Fatalf("failed to create db policy: %v", err)
	}
	revDB := &model.PolicyRevision{
		PolicyID:        "pol-db-mem",
		Revision:        1,
		Priority:        200,
		InheritanceMode: model.InheritanceModeAdditive,
		EnforcementMode: model.EnforcementModeEnforce,
		Rules: []model.PolicyRule{
			{
				ID:       "rule-db-memory",
				Name:     "DB Memory Limit",
				Type:     model.RuleTypeResourceThreshold,
				Severity: model.SeverityCritical,
				Enabled:  true,
				ResourceThreshold: &model.ResourceThresholdRuleConfig{
					Metric:            "memory_usage_pct",
					WarningThreshold:  80,
					CriticalThreshold: 85,
				},
			},
		},
	}
	if err := svc.PublishRevision(ctx, revDB); err != nil {
		t.Fatalf("failed to publish db revision: %v", err)
	}
	if err := svc.CreateAssignment(ctx, &model.PolicyAssignment{
		ID:         "asgn-db-grp",
		OrgID:      orgID,
		PolicyID:   "pol-db-mem",
		TargetType: model.TargetTypeFleetGroup,
		TargetID:   "grp-db-cluster",
		Enabled:    true,
	}); err != nil {
		t.Fatalf("failed to create db assignment: %v", err)
	}

	// 5. Evaluate Organization
	execs, err := svc.EvaluateOrganization(ctx, orgID, model.EvaluationTriggerScheduled)
	if err != nil {
		t.Fatalf("EvaluateOrganization failed: %v", err)
	}
	if len(execs) != 3 {
		t.Fatalf("expected 3 executions, got %d", len(execs))
	}

	// Map executions by target node
	execMap := make(map[string]*model.EvaluationExecution)
	for _, e := range execs {
		execMap[e.TargetNodeID] = e
	}

	execDB := execMap["node-db-01"]
	execWeb := execMap["node-web-01"]
	execDev := execMap["node-dev-01"]

	if execDB == nil || execDB.Status != model.EvaluationStatusNonCompliant {
		t.Errorf("expected node-db-01 status non_compliant, got %+v", execDB)
	}
	if execWeb == nil || execWeb.Status != model.EvaluationStatusCompliant {
		t.Errorf("expected node-web-01 status compliant, got %+v", execWeb)
	}
	if execDev == nil || execDev.Status != model.EvaluationStatusNonCompliant {
		t.Errorf("expected node-dev-01 status non_compliant due to stale heartbeat, got %+v", execDev)
	}

	// 6. Verify Findings Lifecycle - Pass 1: Initial Open Findings
	findings, err := svc.ListComplianceFindings(ctx, model.FindingFilter{OrgID: orgID, Status: model.FindingStatusOpen})
	if err != nil {
		t.Fatalf("failed to list open findings: %v", err)
	}
	// Expected open findings:
	// node-db-01: rule-cpu-threshold, rule-disk-threshold, rule-db-memory = 3 findings
	// node-dev-01: rule-heartbeat = 1 finding
	// Total = 4 open findings
	if len(findings) != 4 {
		t.Fatalf("expected 4 open findings, got %d", len(findings))
	}

	for _, f := range findings {
		if f.Status != model.FindingStatusOpen || f.OccurrenceCount != 1 {
			t.Errorf("expected finding %s to be open with count 1, got status=%s count=%d", f.ID, f.Status, f.OccurrenceCount)
		}
	}

	// 7. Verify Findings Lifecycle - Pass 2: Recurring Findings
	clock.Advance(10 * time.Minute)
	nodeDB.LastHeartbeat = clock.Now()
	if err := store.SaveFleetNode(ctx, nodeDB); err != nil {
		t.Fatalf("failed to update nodeDB heartbeat: %v", err)
	}
	subDB2 := &model.TelemetrySubmission{
		NodeID:    "node-db-01",
		Timestamp: clock.Now(),
		Metrics: map[string]float64{
			"cpu_usage_pct":    92.0,
			"memory_usage_pct": 88.0,
			"disk_usage_pct":   94.0,
		},
	}
	if err := store.SaveTelemetrySubmission(ctx, subDB2); err != nil {
		t.Fatalf("failed to save telemetry DB pass 2: %v", err)
	}

	execDB2, err := svc.EvaluateNode(ctx, "node-db-01", model.EvaluationTriggerScheduled)
	if err != nil {
		t.Fatalf("failed to re-evaluate node-db-01: %v", err)
	}
	if execDB2.Status != model.EvaluationStatusNonCompliant {
		t.Errorf("expected node-db-01 to remain non_compliant")
	}

	findingsDB, err := svc.ListComplianceFindings(ctx, model.FindingFilter{OrgID: orgID, TargetNodeID: "node-db-01"})
	if err != nil {
		t.Fatalf("failed to list node-db-01 findings: %v", err)
	}
	if len(findingsDB) != 3 {
		t.Fatalf("expected 3 findings for node-db-01, got %d", len(findingsDB))
	}
	for _, f := range findingsDB {
		if f.Status != model.FindingStatusRecurring || f.OccurrenceCount != 2 {
			t.Errorf("expected finding %s to be recurring with count 2, got status=%s count=%d", f.ID, f.Status, f.OccurrenceCount)
		}
	}

	// 8. Verify Findings Lifecycle - Pass 3: Remediation & Finding Resolution
	// Remediate node-db-01: reduce CPU to 40%, Memory to 50%, Disk to 45%
	clock.Advance(5 * time.Minute)
	nodeDB.LastHeartbeat = clock.Now()
	if err := store.SaveFleetNode(ctx, nodeDB); err != nil {
		t.Fatalf("failed to update nodeDB heartbeat: %v", err)
	}
	subDBRemediated := &model.TelemetrySubmission{
		NodeID:    "node-db-01",
		Timestamp: clock.Now(),
		Metrics: map[string]float64{
			"cpu_usage_pct":    40.0,
			"memory_usage_pct": 50.0,
			"disk_usage_pct":   45.0,
		},
	}
	if err := store.SaveTelemetrySubmission(ctx, subDBRemediated); err != nil {
		t.Fatalf("failed to save remediated telemetry: %v", err)
	}

	execDB3, err := svc.EvaluateNode(ctx, "node-db-01", model.EvaluationTriggerScheduled)
	if err != nil {
		t.Fatalf("failed to evaluate remediated node: %v", err)
	}
	if execDB3.Status != model.EvaluationStatusCompliant {
		t.Errorf("expected node-db-01 to become compliant, got %s", execDB3.Status)
	}

	// Check resolved findings for node-db-01
	resolvedFindings, err := svc.ListComplianceFindings(ctx, model.FindingFilter{
		OrgID:        orgID,
		TargetNodeID: "node-db-01",
		Status:       model.FindingStatusResolved,
	})
	if err != nil {
		t.Fatalf("failed to list resolved findings: %v", err)
	}
	if len(resolvedFindings) != 3 {
		t.Errorf("expected 3 resolved findings for node-db-01, got %d", len(resolvedFindings))
	}
	for _, f := range resolvedFindings {
		if f.ResolvedAt == nil || f.ResolvedAt.IsZero() {
			t.Errorf("expected resolved_at to be set on finding %s", f.ID)
		}
	}

	// 9. Compliance Rollup Summaries
	// Node rollup
	nodeSum, err := svc.GetNodeComplianceSummary(ctx, orgID, "node-db-01")
	if err != nil {
		t.Fatalf("failed to get node compliance summary: %v", err)
	}
	if nodeSum.ComplianceRatio != 1.0 {
		t.Errorf("expected node-db-01 compliance ratio 1.0, got %f", nodeSum.ComplianceRatio)
	}
	if nodeSum.TotalFindingsOpen != 0 {
		t.Errorf("expected 0 active findings on remediated node-db-01, got %d", nodeSum.TotalFindingsOpen)
	}

	// Group rollup for prod (db-cluster + web)
	grpSum, err := svc.GetGroupComplianceSummary(ctx, orgID, "grp-prod", true)
	if err != nil {
		t.Fatalf("failed to get group compliance summary: %v", err)
	}
	if grpSum.TotalNodes != 2 || grpSum.CompliantNodes != 2 {
		t.Errorf("expected 2 compliant nodes in prod group, got total=%d compliant=%d", grpSum.TotalNodes, grpSum.CompliantNodes)
	}

	// Org rollup
	orgSum, err := svc.GetOrgComplianceSummary(ctx, orgID)
	if err != nil {
		t.Fatalf("failed to get org compliance summary: %v", err)
	}
	if orgSum.TotalNodes != 3 || orgSum.CompliantNodes != 2 {
		t.Errorf("expected 3 total nodes and 2 compliant nodes in org, got total=%d compliant=%d", orgSum.TotalNodes, orgSum.CompliantNodes)
	}

	// 10. Simulation Testing
	// Propose a stricter memory limit on DB: Warning 40%, Critical 45% (currently 50% -> should flag new finding)
	simReq := &model.SimulationRequest{
		OrgID:         orgID,
		TargetNodeIDs: []string{"node-db-01"},
		ProposedPolicies: []model.Policy{
			{
				ID:             "pol-db-mem",
				OrgID:          orgID,
				Name:           "Database Memory Threshold",
				Category:       model.PolicyCategoryResourceThresholds,
				Status:         model.PolicyStatusActive,
				ActiveRevision: 2,
			},
		},
		ProposedRevisions: []model.PolicyRevision{
			{
				PolicyID:        "pol-db-mem",
				Revision:        2,
				Priority:        200,
				InheritanceMode: model.InheritanceModeAdditive,
				EnforcementMode: model.EnforcementModeEnforce,
				Rules: []model.PolicyRule{
					{
						ID:       "rule-db-memory",
						Name:     "DB Strict Memory Limit",
						Type:     model.RuleTypeResourceThreshold,
						Severity: model.SeverityCritical,
						Enabled:  true,
						ResourceThreshold: &model.ResourceThresholdRuleConfig{
							Metric:            "memory_usage_pct",
							WarningThreshold:  40,
							CriticalThreshold: 45,
						},
					},
				},
			},
		},
	}

	simRes, err := svc.SimulatePolicyChanges(ctx, simReq)
	if err != nil {
		t.Fatalf("simulation failed: %v", err)
	}

	if simRes.EvaluatedNodes != 1 {
		t.Errorf("expected 1 evaluated node in simulation, got %d", simRes.EvaluatedNodes)
	}
	if len(simRes.NewFindings) != 1 {
		t.Errorf("expected 1 new finding in simulation, got %d", len(simRes.NewFindings))
	}
	if simRes.ProposedComplianceRatio >= simRes.BaselineComplianceRatio {
		t.Errorf("expected proposed compliance ratio to drop under stricter policy, got base=%f proposed=%f",
			simRes.BaselineComplianceRatio, simRes.ProposedComplianceRatio)
	}

	// Verify simulation had ZERO storage mutation side-effects
	activeFindingsAfterSim, err := svc.ListComplianceFindings(ctx, model.FindingFilter{
		OrgID:        orgID,
		TargetNodeID: "node-db-01",
		Status:       model.FindingStatusOpen,
	})
	if err != nil || len(activeFindingsAfterSim) != 0 {
		t.Errorf("expected 0 open findings in store after simulation, got %d, err: %v", len(activeFindingsAfterSim), err)
	}
}
