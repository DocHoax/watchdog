package model

import (
	"testing"
	"time"
)

func TestSuppressionDecision_Validation(t *testing.T) {
	now := time.Now().UTC()

	valid := &SuppressionDecision{
		ID:          "sup-001",
		OrgID:       "org-main",
		AlertID:     "alt-001",
		NodeID:      "node-001",
		WindowID:    "win-001",
		RuleID:      "rule-high-cpu",
		RuleName:    "High CPU",
		Category:    string(PolicyCategoryResourceThresholds),
		Severity:    SeverityWarning,
		Outcome:     SuppressionOutcomeSuppressed,
		Reason:      SuppressionReasonInMaintenanceWindow,
		Message:     "Alert suppressed due to active maintenance window win-001",
		EvaluatedAt: now,
	}

	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid suppression decision, got error: %v", err)
	}

	// Invalid outcome
	badOutcome := *valid
	badOutcome.Outcome = "unknown_outcome"
	if err := badOutcome.Validate(); err == nil {
		t.Errorf("expected error for invalid outcome")
	}

	// Invalid reason
	badReason := *valid
	badReason.Reason = "invalid_reason"
	if err := badReason.Validate(); err == nil {
		t.Errorf("expected error for invalid reason")
	}

	// Invalid severity
	badSev := *valid
	badSev.Severity = "unknown_severity"
	if err := badSev.Validate(); err == nil {
		t.Errorf("expected error for invalid severity")
	}
}
