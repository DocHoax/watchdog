package model

import (
	"testing"
	"time"
)

func TestEvaluationStatus_IsValid(t *testing.T) {
	validStatuses := []EvaluationStatus{
		EvaluationStatusCompliant,
		EvaluationStatusNonCompliant,
		EvaluationStatusWarning,
		EvaluationStatusNotApplicable,
		EvaluationStatusInsufficientData,
		EvaluationStatusError,
	}

	for _, s := range validStatuses {
		if !s.IsValid() {
			t.Errorf("expected status %s to be valid", s)
		}
	}

	if EvaluationStatus("unknown").IsValid() {
		t.Errorf("expected 'unknown' status to be invalid")
	}
}

func TestDataFreshnessStatus_IsValid(t *testing.T) {
	valid := []DataFreshnessStatus{
		DataFreshnessFresh,
		DataFreshnessStale,
		DataFreshnessMissing,
		DataFreshnessInvalid,
		DataFreshnessUnsupported,
	}

	for _, f := range valid {
		if !f.IsValid() {
			t.Errorf("expected freshness %s to be valid", f)
		}
	}

	if DataFreshnessStatus("garbage").IsValid() {
		t.Errorf("expected 'garbage' freshness to be invalid")
	}
}

func TestEvaluationExecution_Validate(t *testing.T) {
	now := time.Now().UTC()
	exec := &EvaluationExecution{
		ID:           "exec-1234",
		OrgID:        "org-acme",
		TargetNodeID: "node-001",
		TriggerType:  EvaluationTriggerOnDemand,
		EvaluatedAt:  now,
		Status:       EvaluationStatusCompliant,
		Results: []EvaluationResult{
			{
				RuleID:          "rule-cpu",
				RuleName:        "CPU Rule",
				PolicyID:        "pol-res",
				PolicyRevision:  1,
				Category:        PolicyCategoryResourceThresholds,
				RuleType:        RuleTypeResourceThreshold,
				Status:          EvaluationStatusCompliant,
				Severity:        SeverityCritical,
				EnforcementMode: EnforcementModeEnforce,
				DataFreshness:   DataFreshnessFresh,
				EvaluatedAt:     now,
			},
		},
		Summary: EvaluationSummary{
			TotalRules:      1,
			CompliantRules:  1,
			ComplianceRatio: 1.0,
			CoverageRatio:   1.0,
		},
	}

	if err := exec.Validate(); err != nil {
		t.Fatalf("expected valid execution, got: %v", err)
	}

	// Test invalid ID
	exec.ID = ""
	if err := exec.Validate(); err == nil {
		t.Errorf("expected error for empty execution ID")
	}
	exec.ID = "exec-1234"

	// Test invalid target node
	exec.TargetNodeID = ""
	if err := exec.Validate(); err == nil {
		t.Errorf("expected error for empty target node ID")
	}
	exec.TargetNodeID = "node-001"

	// Test invalid result in slice
	exec.Results[0].RuleID = ""
	if err := exec.Validate(); err == nil {
		t.Errorf("expected error for invalid result rule ID")
	}
}

func TestCalculateSummary(t *testing.T) {
	results := []EvaluationResult{
		{Status: EvaluationStatusCompliant},
		{Status: EvaluationStatusCompliant},
		{Status: EvaluationStatusNonCompliant},
		{Status: EvaluationStatusWarning},
		{Status: EvaluationStatusNotApplicable},
		{Status: EvaluationStatusInsufficientData},
		{Status: EvaluationStatusError},
	}

	summary := CalculateSummary(results)

	if summary.TotalRules != 7 {
		t.Errorf("expected TotalRules=7, got %d", summary.TotalRules)
	}
	if summary.CompliantRules != 2 {
		t.Errorf("expected CompliantRules=2, got %d", summary.CompliantRules)
	}
	if summary.NonCompliantRules != 1 {
		t.Errorf("expected NonCompliantRules=1, got %d", summary.NonCompliantRules)
	}
	if summary.WarningRules != 1 {
		t.Errorf("expected WarningRules=1, got %d", summary.WarningRules)
	}
	if summary.NotApplicableRules != 1 {
		t.Errorf("expected NotApplicableRules=1, got %d", summary.NotApplicableRules)
	}
	if summary.InsufficientDataRules != 1 {
		t.Errorf("expected InsufficientDataRules=1, got %d", summary.InsufficientDataRules)
	}
	if summary.ErrorRules != 1 {
		t.Errorf("expected ErrorRules=1, got %d", summary.ErrorRules)
	}

	// evaluatedCount = 2 + 1 + 1 = 4
	// complianceRatio = 2 / 4 = 0.5
	if summary.ComplianceRatio != 0.5 {
		t.Errorf("expected ComplianceRatio=0.5, got %f", summary.ComplianceRatio)
	}

	// applicableCount = 7 - 1 = 6
	// coverageRatio = 4 / 6 = 0.6666...
	expectedCoverage := 4.0 / 6.0
	if summary.CoverageRatio != expectedCoverage {
		t.Errorf("expected CoverageRatio=%f, got %f", expectedCoverage, summary.CoverageRatio)
	}
}

func TestSimulationRequest_Validate(t *testing.T) {
	req := &SimulationRequest{
		OrgID: "org-acme",
		ProposedPolicies: []Policy{
			{
				ID:             "pol-01",
				Name:           "Test Policy",
				OrgID:          "org-acme",
				Category:       PolicyCategoryResourceThresholds,
				Status:         PolicyStatusActive,
				ActiveRevision: 1,
			},
		},
		ProposedRevisions: []PolicyRevision{
			{
				PolicyID:        "pol-01",
				Revision:        1,
				InheritanceMode: InheritanceModeInheritAndOverride,
				EnforcementMode: EnforcementModeEnforce,
				Rules: []PolicyRule{
					{
						ID:       "rule-01",
						Name:     "CPU Limit",
						Type:     RuleTypeResourceThreshold,
						Severity: SeverityCritical,
						Enabled:  true,
						ResourceThreshold: &ResourceThresholdRuleConfig{
							Metric:            "cpu_usage_pct",
							WarningThreshold:  70,
							CriticalThreshold: 90,
						},
					},
				},
			},
		},
		ProposedAssignments: []PolicyAssignment{
			{
				ID:         "asgn-01",
				OrgID:      "org-acme",
				PolicyID:   "pol-01",
				TargetType: TargetTypeOrganization,
				TargetID:   "org-acme",
				Enabled:    true,
			},
		},
	}

	if err := req.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	if !RuleDiffTypeAdded.IsValid() || !RuleDiffTypeRemoved.IsValid() || !RuleDiffTypeModified.IsValid() || !RuleDiffTypeUnchanged.IsValid() {
		t.Errorf("expected valid rule diff types")
	}
	if RuleDiffType("unknown").IsValid() {
		t.Errorf("expected invalid rule diff type")
	}
}

