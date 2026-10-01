package model

import (
	"testing"
	"time"
)

func TestPolicy_ValidationAndLifecycle(t *testing.T) {
	p := &Policy{
		ID:          "pol-cpu-guard",
		OrgID:       DefaultOrganizationID,
		Name:        "Global CPU Guard",
		Description: "Enforces CPU usage limits across all standard nodes",
		Category:    PolicyCategoryResourceThresholds,
		Status:      PolicyStatusDraft,
	}

	if err := p.Validate(); err != nil {
		t.Fatalf("expected valid policy, got: %v", err)
	}

	// Test lifecycle transitions
	if !p.CanTransitionTo(PolicyStatusActive) {
		t.Errorf("expected draft -> active to be valid")
	}
	if !p.CanTransitionTo(PolicyStatusArchived) {
		t.Errorf("expected draft -> archived to be valid")
	}
	if p.CanTransitionTo(PolicyStatusDisabled) {
		t.Errorf("expected draft -> disabled to be invalid")
	}

	// Transition to Active
	p.Status = PolicyStatusActive
	if !p.CanTransitionTo(PolicyStatusDisabled) {
		t.Errorf("expected active -> disabled to be valid")
	}
	if !p.CanTransitionTo(PolicyStatusArchived) {
		t.Errorf("expected active -> archived to be valid")
	}

	// Transition to Disabled
	p.Status = PolicyStatusDisabled
	if !p.CanTransitionTo(PolicyStatusActive) {
		t.Errorf("expected disabled -> active to be valid")
	}

	// Transition to Archived (terminal)
	p.Status = PolicyStatusArchived
	if p.CanTransitionTo(PolicyStatusActive) {
		t.Errorf("expected archived -> active to be invalid (terminal state)")
	}
	if p.CanTransitionTo(PolicyStatusDraft) {
		t.Errorf("expected archived -> draft to be invalid")
	}
}

func TestPolicy_InvalidCases(t *testing.T) {
	var nilP *Policy
	if nilP.Validate() == nil {
		t.Errorf("expected error on nil policy")
	}

	p := &Policy{
		ID:       "pol-1",
		Name:     "Test",
		Category: "invalid_category",
		Status:   PolicyStatusActive,
	}
	if err := p.Validate(); err == nil {
		t.Errorf("expected error for invalid category")
	}

	p.Category = PolicyCategoryResourceThresholds
	p.Status = "invalid_status"
	if err := p.Validate(); err == nil {
		t.Errorf("expected error for invalid status")
	}

	p.Status = PolicyStatusActive
	p.ActiveRevision = -1
	if err := p.Validate(); err == nil {
		t.Errorf("expected error for negative active revision")
	}
}

func TestPolicyRevision_DigestAndRules(t *testing.T) {
	now := time.Now().UTC()
	rev := &PolicyRevision{
		PolicyID:        "pol-resource-01",
		Revision:        1,
		CreatedAt:       now,
		CreatedBy:       "admin@company.internal",
		ChangeSummary:   "Initial revision with CPU & Memory thresholds",
		Priority:        100,
		InheritanceMode: InheritanceModeInheritAndOverride,
		EnforcementMode: EnforcementModeEnforce,
		Rules: []PolicyRule{
			{
				ID:       "rule-cpu-thresh",
				Name:     "High CPU Threshold",
				Type:     RuleTypeResourceThreshold,
				Severity: SeverityCritical,
				Enabled:  true,
				ResourceThreshold: &ResourceThresholdRuleConfig{
					Metric:            "cpu_usage_pct",
					WarningThreshold:  80.0,
					CriticalThreshold: 90.0,
					Unit:              "%",
					DurationWindow:    "5m",
				},
			},
			{
				ID:       "rule-mem-thresh",
				Name:     "High Memory Threshold",
				Type:     RuleTypeResourceThreshold,
				Severity: SeverityWarning,
				Enabled:  true,
				ResourceThreshold: &ResourceThresholdRuleConfig{
					Metric:            "memory_used_pct",
					WarningThreshold:  85.0,
					CriticalThreshold: 95.0,
					Unit:              "%",
				},
			},
		},
	}

	digest := rev.ComputeDigest()
	if digest == "" {
		t.Fatalf("expected non-empty digest")
	}

	// Validate populates ContentDigest if empty
	if err := rev.Validate(); err != nil {
		t.Fatalf("expected valid revision, got: %v", err)
	}
	if rev.ContentDigest != digest {
		t.Errorf("expected digest %s, got %s", digest, rev.ContentDigest)
	}

	// Test deterministic digest regardless of rule slice ordering
	revReordered := &PolicyRevision{
		PolicyID:        "pol-resource-01",
		Revision:        1,
		CreatedAt:       now,
		CreatedBy:       "admin@company.internal",
		ChangeSummary:   "Initial revision with CPU & Memory thresholds",
		Priority:        100,
		InheritanceMode: InheritanceModeInheritAndOverride,
		EnforcementMode: EnforcementModeEnforce,
		Rules: []PolicyRule{
			rev.Rules[1],
			rev.Rules[0],
		},
	}
	if revReordered.ComputeDigest() != digest {
		t.Errorf("expected deterministic digest regardless of rule order")
	}

	// Test digest mismatch detection
	rev.ContentDigest = "tampered_digest_value"
	if err := rev.Validate(); err == nil {
		t.Errorf("expected validation failure on tampered content digest")
	}
}

func TestPolicyRule_AllTypes(t *testing.T) {
	tests := []struct {
		name    string
		rule    PolicyRule
		wantErr bool
	}{
		{
			name: "valid anomaly detection rule",
			rule: PolicyRule{
				ID:       "rule-anom-01",
				Name:     "Network Anomaly Detection",
				Type:     RuleTypeAnomalyDetection,
				Severity: SeverityWarning,
				Enabled:  true,
				AnomalyDetection: &AnomalyDetectionRuleConfig{
					Metric:          "net_tx_rate",
					Sensitivity:     "high",
					ZScoreThreshold: 3.5,
					ExcludedHours:   []int{0, 1, 2},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid anomaly detection - negative zscore",
			rule: PolicyRule{
				ID:       "rule-anom-02",
				Name:     "Network Anomaly",
				Type:     RuleTypeAnomalyDetection,
				AnomalyDetection: &AnomalyDetectionRuleConfig{
					Metric:          "net_tx_rate",
					ZScoreThreshold: -1.0,
				},
			},
			wantErr: true,
		},
		{
			name: "valid capacity planning rule",
			rule: PolicyRule{
				ID:       "rule-cap-01",
				Name:     "Disk Exhaustion Forecast",
				Type:     RuleTypeCapacityPlanning,
				Severity: SeverityCritical,
				CapacityPlanning: &CapacityPlanningRuleConfig{
					Metric:                   "disk_used_pct",
					HorizonDays:              90,
					WarningDaysToExhaustion:  30,
					CriticalDaysToExhaustion: 14,
					GrowthModel:              "linear",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid capacity planning - critical > warning",
			rule: PolicyRule{
				ID:       "rule-cap-02",
				Name:     "Disk Exhaustion",
				Type:     RuleTypeCapacityPlanning,
				CapacityPlanning: &CapacityPlanningRuleConfig{
					Metric:                   "disk_used_pct",
					HorizonDays:              90,
					WarningDaysToExhaustion:  10,
					CriticalDaysToExhaustion: 20,
				},
			},
			wantErr: true,
		},
		{
			name: "valid incident severity rule",
			rule: PolicyRule{
				ID:       "rule-inc-01",
				Name:     "Auto Escalation on Critical CPU",
				Type:     RuleTypeIncidentSeverity,
				IncidentSeverity: &IncidentSeverityRuleConfig{
					Condition:           "unhealthy_duration > 15m",
					EscalateToSeverity:  SeverityCritical,
					NotificationChannel: "#ops-pager",
					AutoEscalationAfter: "15m",
				},
			},
			wantErr: false,
		},
		{
			name: "valid operational compliance rule",
			rule: PolicyRule{
				ID:       "rule-comp-01",
				Name:     "Heartbeat Freshness Check",
				Type:     RuleTypeOperationalCompliance,
				OperationalCompliance: &OperationalComplianceRuleConfig{
					CheckType:         "heartbeat_freshness",
					MaxAgeSeconds:     300,
					ViolationSeverity: SeverityCritical,
				},
			},
			wantErr: false,
		},
		{
			name: "missing config for rule type",
			rule: PolicyRule{
				ID:   "rule-empty-01",
				Name: "Empty Rule",
				Type: RuleTypeResourceThreshold,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPolicyAssignment_Validation(t *testing.T) {
	assignment := &PolicyAssignment{
		ID:         "asgn-01",
		PolicyID:   "pol-01",
		OrgID:      DefaultOrganizationID,
		TargetType: TargetTypeFleetGroup,
		TargetID:   "grp-prod",
		AssignedAt: time.Now().UTC(),
		Enabled:    true,
	}

	if err := assignment.Validate(); err != nil {
		t.Fatalf("expected valid assignment, got %v", err)
	}

	// Invalid target type
	assignment.TargetType = "cluster"
	if err := assignment.Validate(); err == nil {
		t.Errorf("expected error for invalid target type")
	}

	// Invalid TargetID
	assignment.TargetType = TargetTypeNode
	assignment.TargetID = ""
	if err := assignment.Validate(); err == nil {
		t.Errorf("expected error for empty target id")
	}
}
