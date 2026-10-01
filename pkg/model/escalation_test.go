package model

import (
	"testing"
	"time"
)

func TestEscalationPolicy_Validation(t *testing.T) {
	now := time.Now().UTC()

	valid := &EscalationPolicy{
		ID:          "esc-001",
		OrgID:       "org-main",
		Name:        "Production Incident Escalation",
		Description: "Escalates production critical incidents",
		Enabled:     true,
		SeverityLevels: []Severity{
			SeverityCritical,
			SeverityWarning,
		},
		Stages: []EscalationStage{
			{
				StageNumber:  1,
				DelayMinutes: 0,
				Channel:      EscalationChannelSlack,
				Targets: []EscalationTarget{
					{
						TargetType:  EscalationTargetTeam,
						TargetID:    "team-sre",
						Name:        "SRE Primary On-Call",
						ContactInfo: "#sre-alerts",
					},
				},
			},
			{
				StageNumber:  2,
				DelayMinutes: 15,
				Channel:      EscalationChannelPagerDuty,
				Targets: []EscalationTarget{
					{
						TargetType:  EscalationTargetSchedule,
						TargetID:    "pd-schedule-tier2",
						Name:        "Tier 2 Escalation Schedule",
						ContactInfo: "P12345",
					},
				},
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid escalation policy, got error: %v", err)
	}

	// Invalid out-of-order stage numbers
	badStages := *valid
	badStages.Stages = []EscalationStage{
		{
			StageNumber:  2,
			DelayMinutes: 10,
			Targets: []EscalationTarget{
				{TargetType: EscalationTargetTeam, TargetID: "team-sre", Name: "SRE"},
			},
		},
	}
	if err := badStages.Validate(); err == nil {
		t.Errorf("expected error for non-sequential stage numbers")
	}

	// Invalid delay decreasing
	badDelay := *valid
	badDelay.Stages = []EscalationStage{
		{
			StageNumber:  1,
			DelayMinutes: 30,
			Targets: []EscalationTarget{
				{TargetType: EscalationTargetTeam, TargetID: "team-sre", Name: "SRE"},
			},
		},
		{
			StageNumber:  2,
			DelayMinutes: 10,
			Targets: []EscalationTarget{
				{TargetType: EscalationTargetTeam, TargetID: "team-sre", Name: "SRE"},
			},
		},
	}
	if err := badDelay.Validate(); err == nil {
		t.Errorf("expected error for decreasing stage delay")
	}
}
