package model

import (
	"testing"
	"time"
)

func TestFindingStatus_IsValid(t *testing.T) {
	statuses := []FindingStatus{
		FindingStatusOpen,
		FindingStatusRecurring,
		FindingStatusResolved,
		FindingStatusIndeterminate,
	}

	for _, s := range statuses {
		if !s.IsValid() {
			t.Errorf("expected finding status %s to be valid", s)
		}
	}

	if FindingStatus("invalid").IsValid() {
		t.Errorf("expected 'invalid' finding status to be invalid")
	}
}

func TestComputeFindingID(t *testing.T) {
	id1 := ComputeFindingID("org-1", "node-1", "pol-1", "rule-1")
	id2 := ComputeFindingID("org-1", "node-1", "pol-1", "rule-1")
	id3 := ComputeFindingID("org-1", "node-1", "pol-1", "rule-2")

	if id1 != id2 {
		t.Errorf("expected deterministic IDs to match: %s != %s", id1, id2)
	}
	if id1 == id3 {
		t.Errorf("expected different IDs for different rules: %s == %s", id1, id3)
	}
	if len(id1) == 0 || id1[:4] != "fnd_" {
		t.Errorf("expected finding ID prefix 'fnd_', got: %s", id1)
	}
}

func TestComplianceFinding_Validate(t *testing.T) {
	now := time.Now().UTC()
	finding := &ComplianceFinding{
		ID:              ComputeFindingID("org-acme", "node-01", "pol-01", "rule-01"),
		OrgID:           "org-acme",
		TargetNodeID:    "node-01",
		PolicyID:        "pol-01",
		PolicyRevision:  1,
		RuleID:          "rule-01",
		RuleName:        "Test Rule",
		Category:        PolicyCategoryResourceThresholds,
		Severity:        SeverityCritical,
		EnforcementMode: EnforcementModeEnforce,
		Status:          FindingStatusOpen,
		FirstSeenAt:     now,
		LastSeenAt:      now,
		OccurrenceCount: 1,
		Message:         "CPU breach",
	}

	if err := finding.Validate(); err != nil {
		t.Fatalf("expected valid finding, got: %v", err)
	}

	finding.OccurrenceCount = 0
	if err := finding.Validate(); err == nil {
		t.Errorf("expected error for non-positive occurrence count")
	}
}

func TestComplianceRollupLevel_IsValid(t *testing.T) {
	levels := []ComplianceRollupLevel{
		ComplianceRollupLevelNode,
		ComplianceRollupLevelFleetGroup,
		ComplianceRollupLevelOrg,
		ComplianceRollupLevelFleet,
	}

	for _, l := range levels {
		if !l.IsValid() {
			t.Errorf("expected level %s to be valid", l)
		}
	}

	if ComplianceRollupLevel("invalid").IsValid() {
		t.Errorf("expected 'invalid' level to be invalid")
	}
}
