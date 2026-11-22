package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/governance"
	"github.com/DocHoax/watchdog/pkg/model"
)

// -----------------------------------------------------------------------------
// Test Fixtures
// -----------------------------------------------------------------------------

func govFixtureTime() time.Time {
	return time.Date(2026, 11, 22, 4, 0, 0, 0, time.UTC)
}

func govFixtureOrg() model.Organization {
	now := govFixtureTime()
	return model.Organization{
		ID:          "org-prod-01",
		Name:        "Production Org",
		DisplayName: "Production Organization",
		Description: "Primary production tenant",
		Status:      model.OrgStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func govFixtureGroup() model.FleetGroup {
	now := govFixtureTime()
	return model.FleetGroup{
		ID:          "grp-prod-db",
		OrgID:       "org-prod-01",
		Name:        "Production Database Tier",
		DisplayName: "Prod DB",
		Type:        model.GroupTypeTier,
		Path:        "/org-prod-01/grp-prod-db",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func govFixturePolicy() model.Policy {
	now := govFixtureTime()
	return model.Policy{
		ID:        "pol-cpu-threshold",
		OrgID:     "org-prod-01",
		Name:      "CPU Threshold Policy",
		Category:  model.PolicyCategoryResourceThresholds,
		Status:    model.PolicyStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func govFixtureAssignment() model.PolicyAssignment {
	now := govFixtureTime()
	return model.PolicyAssignment{
		ID:         "asgn-cpu-prod-01",
		OrgID:      "org-prod-01",
		PolicyID:   "pol-cpu-threshold",
		TargetType: model.TargetTypeNode,
		TargetID:   "node-prod-01",
		Enabled:    true,
		AssignedAt: now,
		AssignedBy: "operator",
	}
}

func govFixtureEvaluation() model.EvaluationExecution {
	now := govFixtureTime()
	return model.EvaluationExecution{
		ID:           "eval-prod-01-001",
		OrgID:        "org-prod-01",
		TargetNodeID: "node-prod-01",
		TriggerType:  model.EvaluationTriggerOnDemand,
		Status:       model.EvaluationStatusCompliant,
		EvaluatedAt:  now,
		Summary: model.EvaluationSummary{
			TotalRules:     5,
			CompliantRules: 4,
			WarningRules:   1,
			ComplianceRatio: 0.8,
			CoverageRatio:   1.0,
		},
	}
}

func govFixtureFinding() model.ComplianceFinding {
	now := govFixtureTime()
	return model.ComplianceFinding{
		ID:            "fnd-cpu-node01",
		OrgID:         "org-prod-01",
		TargetNodeID:  "node-prod-01",
		PolicyID:      "pol-cpu-threshold",
		RuleID:        "rule-cpu-pct",
		RuleName:      "CPU Utilization Threshold",
		Category:      model.PolicyCategoryResourceThresholds,
		Severity:      model.SeverityWarning,
		Status:        model.FindingStatusOpen,
		FirstSeenAt:   now.Add(-24 * time.Hour),
		LastSeenAt:    now,
		Message:       "CPU utilization 87% exceeds 80% threshold",
		ObservedValue: "87",
		ExpectedValue: "<=80",
	}
}

func govFixtureRollup() model.ComplianceRollupSummary {
	now := govFixtureTime()
	return model.ComplianceRollupSummary{
		OrgID:               "org-prod-01",
		EvaluatedAt:         now,
		TotalNodes:          4,
		CompliantNodes:      3,
		NonCompliantNodes:   1,
		TotalRulesEvaluated: 20,
		TotalFindingsOpen:   2,
		ComplianceRatio:     0.75,
		CoverageRatio:       1.0,
	}
}

func govFixtureMaintenanceWindow() model.MaintenanceWindow {
	now := govFixtureTime()
	start := now.Add(1 * time.Hour)
	end := now.Add(3 * time.Hour)
	return model.MaintenanceWindow{
		ID:          "mw-prod-patch-01",
		OrgID:       "org-prod-01",
		Name:        "Monthly Patch Window",
		Status:      model.MaintenanceStatusScheduled,
		TargetScope: model.MaintenanceTargetScopeNode,
		TargetID:    "node-prod-01",
		CreatedAt:   now,
		UpdatedAt:   now,
		Schedule: model.MaintenanceSchedule{
			StartTime: start,
			EndTime:   end,
			TimeZone:  "UTC",
		},
	}
}

func govFixtureEscalationPolicy() model.EscalationPolicy {
	now := govFixtureTime()
	return model.EscalationPolicy{
		ID:             "esc-critical-oncall",
		OrgID:          "org-prod-01",
		Name:           "Critical On-Call Escalation",
		Description:    "Escalates critical incidents to on-call team",
		Enabled:        true,
		SeverityLevels: []model.Severity{model.SeverityCritical},
		Stages: []model.EscalationStage{
			{
				StageNumber:  1,
				DelayMinutes: 0,
				Channel:      model.EscalationChannelSlack,
				Targets: []model.EscalationTarget{
					{TargetType: model.EscalationTargetTeam, TargetID: "oncall-team", Name: "On-Call Team", ContactInfo: "#alerts"},
				},
			},
			{
				StageNumber:  2,
				DelayMinutes: 15,
				Channel:      model.EscalationChannelPagerDuty,
				Targets: []model.EscalationTarget{
					{TargetType: model.EscalationTargetUser, TargetID: "eng-lead", Name: "Engineering Lead", ContactInfo: "pd-svc-01"},
				},
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func govFixtureSuppressionDecision() model.SuppressionDecision {
	now := govFixtureTime()
	return model.SuppressionDecision{
		ID:          "supp-dec-01",
		OrgID:       "org-prod-01",
		NodeID:      "node-prod-01",
		AlertID:     "alert-cpu-01",
		WindowID:    "mw-prod-patch-01",
		RuleName:    "cpu_threshold_rule",
		Category:    "cpu",
		Severity:    model.SeverityWarning,
		Outcome:     model.SuppressionOutcomeSuppressed,
		Reason:      model.SuppressionReasonInMaintenanceWindow,
		Message:     "Alert suppressed: node is in an active maintenance window",
		EvaluatedAt: now,
	}
}

func govFixtureAuditEvent() model.AuditEvent {
	now := govFixtureTime()
	return model.AuditEvent{
		ID:        "audit-gov-001",
		Timestamp: now,
		EventType: model.EventGovernancePolicyCreated,
		Severity:  model.AuditSeverityInfo,
		Outcome:   model.AuditOutcomeSuccess,
		Actor: model.AuditActor{
			Type:     model.ActorTypeCLI,
			Identity: "operator-01",
		},
		Action:   "create_policy",
		Resource: "policy:pol-cpu-threshold",
		Message:  "Policy pol-cpu-threshold created",
	}
}

func govFixtureEscalationResult() model.EscalationEvaluationResult {
	now := govFixtureTime()
	next := 2
	until := 15
	return model.EscalationEvaluationResult{
		PolicyID:              "esc-critical-oncall",
		PolicyName:            "Critical On-Call Escalation",
		CurrentStage:          1,
		ElapsedTimeMinutes:    5,
		EvaluatedAt:           now,
		NextStageNumber:       &next,
		MinutesUntilNextStage: &until,
		Channel:               model.EscalationChannelSlack,
		ActiveTargets: []model.EscalationTarget{
			{TargetType: model.EscalationTargetTeam, TargetID: "oncall-team", Name: "On-Call Team", ContactInfo: "#alerts"},
		},
	}
}

// -----------------------------------------------------------------------------
// Flag Reset
// -----------------------------------------------------------------------------

func resetGovFlags() {
	govServerURL = ""
	govToken = ""
	govTokenFile = ""
	govTokenEnv = ""
	govInsecureTLS = false
	govTimeout = 15 * time.Second
	govFormat = "text"
	govOrgID = ""
	govLimit = 50
	govOffset = 0
	govSearch = ""
	govSeverity = ""
	govStatus = ""
	govCategory = ""
	govNodeID = ""
	govGroupID = ""
	govPolicyID = ""
	govPolicyA = ""
	govPolicyB = ""
	govRevA = 0
	govRevB = 0
	govTargetTyp = ""
	govTargetID = ""
	govStartTime = ""
	govEndTime = ""
	govEnabled = ""
	govAtTime = ""
	govReason = ""
	govName = ""
	govDescription = ""
	govMetadata = ""
	govParentID = ""
	govAssignedBy = ""
	govAuthor = ""
	govChangelog = ""
	govRulesFile = ""
	govStagesFile = ""
	govPoliciesFile = ""
	govSourceFile = ""
	govRevisionNum = 0
	govPriority = 0
	govSelector = ""
	govInheritMode = ""
	govEnforceMode = ""
	govScopeFilter = ""
	govTriggerType = ""
	govRuleID = ""
	govDiffOnly = false
	govTargetScope = ""
	govTargetSelector = ""
	govWindowStart = ""
	govWindowEnd = ""
	govTimezone = ""
	govSeverityThr = ""
	govCatRestrict = nil
	govSuppressAlerts = false
	govSuppressFind = false
	govAllowCritical = false
	govRecurrenceFreq = ""
	govRecurrenceInt = 0
	govRecurDays = nil
	govRecurDayMonth = 0
	govRecurMaxOcc = 0
	govRecurUntil = ""
	govRecurDuration = 0
	govCreatedBy = ""
	govIncludeExpired = false
	govIncludeSubgrp = false
	govIncidentID = ""
	govIncidentTitle = ""
	govIncidentSev = ""
	govIncidentStart = ""
	govIncidentAck = false
	govIncidentNodes = nil
	govAlertID = ""
	govAlertName = ""
	govEventType = ""
	govOutcome = ""
	govActorType = ""
	govActorID = ""
	govAction = ""
	govMessage = ""
	govPolicyIDs = nil
	govNodeIDs = nil
	govGroupIDs = nil
	govDuration = 0
	govAcknowledged = false
	govResourceType = ""
	govResourceID = ""
	govTags = nil
	govSchemaVersion = ""
	govWindowType = ""
	govDailyStart = ""
	govDailyEnd = ""
	govDaysOfWeek = nil
	govDayOfMonth = 0
	govAllowDisrupt = false
	govOwnerService = ""
	govOwnerLead = ""
	govOwnerSlack = ""
	govOwnerPager = ""
	govOwnerEscTier = ""
	govOwnerLifeStg = ""
	govOwnerTeam = ""
	govOwnerContactCh = ""
	govOwnerEmail = ""
	govOwnerRegion = ""
	govOwnerDataClass = ""
	govOwnerCostCtr = ""
	govOwnerCritical = ""
	govOwnerLifecycle = ""
	govOwnerEnv = ""
	govCustomProps = ""
	globalCfg = config.DefaultConfig()
}

// -----------------------------------------------------------------------------
// Mock Server
// -----------------------------------------------------------------------------

func setupGovernanceMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	now := govFixtureTime()

	org := govFixtureOrg()
	group := govFixtureGroup()
	policy := govFixturePolicy()
	assignment := govFixtureAssignment()
	eval := govFixtureEvaluation()
	finding := govFixtureFinding()
	rollup := govFixtureRollup()
	mw := govFixtureMaintenanceWindow()
	escPol := govFixtureEscalationPolicy()
	suppDec := govFixtureSuppressionDecision()
	auditEv := govFixtureAuditEvent()
	escResult := govFixtureEscalationResult()

	member := model.FleetGroupMember{
		GroupID: "grp-prod-db",
		NodeID:  "node-prod-01",
		Role:    model.MembershipRolePrimary,
		AddedAt: now,
	}

	ownership := model.NodeOwnershipMetadata{
		OwnerTeam:    "platform-team",
		ContactEmail: "platform@example.com",
	}

	revision := model.PolicyRevision{
		PolicyID:      "pol-cpu-threshold",
		Revision:      1,
		CreatedBy:     "operator",
		ChangeSummary: "Initial revision",
		CreatedAt:     now,
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		method := r.Method

		encode := func(v any) {
			_ = json.NewEncoder(w).Encode(v)
		}
		notFound := func() {
			w.WriteHeader(http.StatusNotFound)
			encode(model.APIErrorResponse{Error: model.APIErrorDetail{
				Code:    "NOT_FOUND",
				Message: "not found: " + path,
			}})
		}
		serverError := func(msg string) {
			w.WriteHeader(http.StatusInternalServerError)
			encode(model.APIErrorResponse{Error: model.APIErrorDetail{
				Code:    "INTERNAL_ERROR",
				Message: msg,
			}})
		}
		unauthorized := func() {
			w.WriteHeader(http.StatusUnauthorized)
			encode(model.APIErrorResponse{Error: model.APIErrorDetail{
				Code:    "UNAUTHORIZED",
				Message: "authentication required",
			}})
		}

		// ---- orgs ----
		switch {
		case path == "/api/v1/governance/orgs" && method == http.MethodGet:
			encode(map[string]any{"organizations": []model.Organization{org}, "count": 1})

		case path == "/api/v1/governance/orgs" && method == http.MethodPost:
			var o model.Organization
			_ = json.NewDecoder(r.Body).Decode(&o)
			o.ID = "org-new-01"
			o.Status = model.OrgStatusActive
			o.CreatedAt = now
			o.UpdatedAt = now
			w.WriteHeader(http.StatusCreated)
			encode(o)

		case path == "/api/v1/governance/orgs/org-prod-01" && method == http.MethodGet:
			encode(org)

		case path == "/api/v1/governance/orgs/org-prod-01" && method == http.MethodPut:
			var o model.Organization
			_ = json.NewDecoder(r.Body).Decode(&o)
			o.ID = "org-prod-01"
			o.UpdatedAt = now
			encode(o)

		case path == "/api/v1/governance/orgs/org-prod-01" && method == http.MethodDelete:
			encode(map[string]any{"deleted": true, "organization_id": "org-prod-01"})

		case path == "/api/v1/governance/orgs/org-nonexistent" && method == http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
			encode(model.APIErrorResponse{Error: model.APIErrorDetail{
				Code:    "ORGANIZATION_NOT_FOUND",
				Message: "organization not found",
			}})

		case path == "/api/v1/governance/orgs/org-error":
			serverError("internal storage failure")

		// ---- groups ----
		case path == "/api/v1/governance/groups" && method == http.MethodGet:
			encode(map[string]any{"groups": []model.FleetGroup{group}, "count": 1, "org_id": "org-prod-01"})

		case path == "/api/v1/governance/groups" && method == http.MethodPost:
			var g model.FleetGroup
			_ = json.NewDecoder(r.Body).Decode(&g)
			g.ID = "grp-new-01"
			g.CreatedAt = now
			g.UpdatedAt = now
			w.WriteHeader(http.StatusCreated)
			encode(g)

		case path == "/api/v1/governance/groups/grp-prod-db" && method == http.MethodGet:
			encode(group)

		case path == "/api/v1/governance/groups/grp-prod-db" && method == http.MethodPut:
			var g model.FleetGroup
			_ = json.NewDecoder(r.Body).Decode(&g)
			g.ID = "grp-prod-db"
			encode(g)

		case path == "/api/v1/governance/groups/grp-prod-db" && method == http.MethodDelete:
			encode(map[string]any{"deleted": true, "group_id": "grp-prod-db"})

		case path == "/api/v1/governance/groups/grp-prod-db/members" && method == http.MethodGet:
			encode(map[string]any{"group_id": "grp-prod-db", "members": []model.FleetGroupMember{member}, "count": 1})

		case path == "/api/v1/governance/groups/grp-prod-db/members" && method == http.MethodPost:
			var m model.FleetGroupMember
			_ = json.NewDecoder(r.Body).Decode(&m)
			m.GroupID = "grp-prod-db"
			m.AddedAt = now
			w.WriteHeader(http.StatusCreated)
			encode(m)

		case path == "/api/v1/governance/groups/grp-prod-db/members/node-prod-01" && method == http.MethodDelete:
			encode(map[string]any{"removed": true, "group_id": "grp-prod-db", "node_id": "node-prod-01"})

		case path == "/api/v1/governance/groups/grp-prod-db/subtree" && method == http.MethodGet:
			encode(map[string]any{"group_id": "grp-prod-db", "nodes": []string{"node-prod-01", "node-prod-02"}, "count": 2})

		case path == "/api/v1/governance/hierarchy" && method == http.MethodGet:
			encode(map[string]any{
				"org_id": "org-prod-01",
				"hierarchy": []*model.FleetGroupHierarchyNode{
					{Group: group, Children: nil, TotalNodeCount: 1, SubgroupCount: 0},
				},
				"count": 1,
			})

		case path == "/api/v1/governance/groups/grp-nonexistent" && method == http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
			encode(model.APIErrorResponse{Error: model.APIErrorDetail{
				Code:    "FLEET_GROUP_NOT_FOUND",
				Message: "fleet group not found",
			}})

		// ---- ownership ----
		case path == "/api/v1/governance/nodes/node-prod-01/ownership" && method == http.MethodGet:
			encode(ownership)

		case path == "/api/v1/governance/nodes/node-prod-01/ownership" && method == http.MethodPut:
			var o model.NodeOwnershipMetadata
			_ = json.NewDecoder(r.Body).Decode(&o)
			encode(o)

		case path == "/api/v1/governance/nodes/node-prod-01/ownership/resolved" && method == http.MethodGet:
			encode(model.ResolvedOwnership{
				NodeID:       "node-prod-01",
				OrgID:        "org-prod-01",
				OwnerTeam:    "platform-team",
				ContactEmail: "platform@example.com",
			})

		// ---- policies ----
		case path == "/api/v1/governance/policies" && method == http.MethodGet:
			encode(map[string]any{"policies": []model.Policy{policy}, "count": 1})

		case path == "/api/v1/governance/policies" && method == http.MethodPost:
			var p model.Policy
			_ = json.NewDecoder(r.Body).Decode(&p)
			p.ID = "pol-new-01"
			p.CreatedAt = now
			p.UpdatedAt = now
			w.WriteHeader(http.StatusCreated)
			encode(p)

		case path == "/api/v1/governance/policies/pol-cpu-threshold" && method == http.MethodGet:
			encode(policy)

		case path == "/api/v1/governance/policies/pol-cpu-threshold" && method == http.MethodPut:
			var p model.Policy
			_ = json.NewDecoder(r.Body).Decode(&p)
			p.ID = "pol-cpu-threshold"
			encode(p)

		case path == "/api/v1/governance/policies/pol-cpu-threshold" && method == http.MethodDelete:
			encode(map[string]any{"deleted": true, "policy_id": "pol-cpu-threshold"})

		case path == "/api/v1/governance/policies/pol-cpu-threshold/revisions" && method == http.MethodGet:
			encode(map[string]any{"policy_id": "pol-cpu-threshold", "revisions": []model.PolicyRevision{revision}, "count": 1})

		case path == "/api/v1/governance/policies/pol-cpu-threshold/revisions" && method == http.MethodPost:
			var rev model.PolicyRevision
			_ = json.NewDecoder(r.Body).Decode(&rev)
			rev.PolicyID = "pol-cpu-threshold"
			rev.Revision = 2
			rev.CreatedAt = now
			w.WriteHeader(http.StatusCreated)
			encode(rev)

		case path == "/api/v1/governance/policies/pol-cpu-threshold/status" && method == http.MethodPut:
			var req struct {
				Status model.PolicyStatus `json:"status"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			encode(map[string]any{"policy_id": "pol-cpu-threshold", "status": req.Status})

		case path == "/api/v1/governance/policies/pol-cpu-threshold/active-revision" && method == http.MethodPut:
			var req struct {
				Revision int `json:"revision"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			encode(map[string]any{"policy_id": "pol-cpu-threshold", "active_revision": req.Revision})

		case path == "/api/v1/governance/policies/conflicts" && method == http.MethodPost:
			encode(map[string]any{"conflicts": []governance.PolicyConflict{}, "count": 0})

		case path == "/api/v1/governance/policies/pol-nonexistent" && method == http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
			encode(model.APIErrorResponse{Error: model.APIErrorDetail{
				Code:    "POLICY_NOT_FOUND",
				Message: "policy not found",
			}})

		// ---- assignments ----
		case path == "/api/v1/governance/assignments" && method == http.MethodGet:
			encode(map[string]any{"assignments": []model.PolicyAssignment{assignment}, "count": 1})

		case path == "/api/v1/governance/assignments" && method == http.MethodPost:
			var a model.PolicyAssignment
			_ = json.NewDecoder(r.Body).Decode(&a)
			a.ID = "asgn-new-01"
			a.AssignedAt = now
			w.WriteHeader(http.StatusCreated)
			encode(a)

		case path == "/api/v1/governance/assignments/asgn-cpu-prod-01" && method == http.MethodGet:
			encode(assignment)

		case path == "/api/v1/governance/assignments/asgn-cpu-prod-01" && method == http.MethodDelete:
			encode(map[string]any{"deleted": true, "assignment_id": "asgn-cpu-prod-01"})

		case path == "/api/v1/governance/assignments/asgn-cpu-prod-01/enable" && method == http.MethodPut:
			var req struct {
				Enabled bool `json:"enabled"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			encode(map[string]any{"assignment_id": "asgn-cpu-prod-01", "enabled": req.Enabled})

		case path == "/api/v1/governance/assignments/asgn-nonexistent" && method == http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
			encode(model.APIErrorResponse{Error: model.APIErrorDetail{
				Code:    "POLICY_ASSIGNMENT_NOT_FOUND",
				Message: "policy assignment not found",
			}})

		// ---- evaluations ----
		case path == "/api/v1/governance/evaluations" && method == http.MethodGet:
			encode(map[string]any{"evaluations": []model.EvaluationExecution{eval}, "count": 1})

		case path == "/api/v1/governance/evaluations/node/node-prod-01" && method == http.MethodPost:
			encode(eval)

		case path == "/api/v1/governance/evaluations/group/grp-prod-db" && method == http.MethodPost:
			encode(map[string]any{"group_id": "grp-prod-db", "evaluations": []*model.EvaluationExecution{&eval}, "count": 1})

		case path == "/api/v1/governance/evaluations/org/org-prod-01" && method == http.MethodPost:
			encode(map[string]any{"org_id": "org-prod-01", "evaluations": []*model.EvaluationExecution{&eval}, "count": 1})

		case path == "/api/v1/governance/evaluations/eval-prod-01-001" && method == http.MethodGet:
			encode(eval)

		case path == "/api/v1/governance/evaluations/latest/node/node-prod-01" && method == http.MethodGet:
			encode(eval)

		case path == "/api/v1/governance/evaluations/eval-nonexistent" && method == http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
			encode(model.APIErrorResponse{Error: model.APIErrorDetail{
				Code:    "NOT_FOUND",
				Message: "evaluation not found",
			}})

		// ---- findings ----
		case path == "/api/v1/governance/findings" && method == http.MethodGet:
			encode(map[string]any{"findings": []model.ComplianceFinding{finding}, "count": 1})

		case path == "/api/v1/governance/findings/fnd-cpu-node01" && method == http.MethodGet:
			encode(finding)

		case path == "/api/v1/governance/findings/fnd-nonexistent" && method == http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
			encode(model.APIErrorResponse{Error: model.APIErrorDetail{
				Code:    "NOT_FOUND",
				Message: "finding not found",
			}})

		// ---- compliance ----
		case path == "/api/v1/governance/compliance/node/node-prod-01" && method == http.MethodGet:
			encode(rollup)

		case path == "/api/v1/governance/compliance/group/grp-prod-db" && method == http.MethodGet:
			encode(rollup)

		case path == "/api/v1/governance/compliance/org/org-prod-01" && method == http.MethodGet:
			encode(rollup)

		// ---- simulation ----
		case path == "/api/v1/governance/simulation" && method == http.MethodPost:
			encode(model.SimulationResult{
				OrgID:       "org-prod-01",
				SimulatedAt: now,
			})

		// ---- maintenance ----
		case path == "/api/v1/governance/maintenance" && method == http.MethodGet:
			encode(map[string]any{"maintenance_windows": []model.MaintenanceWindow{mw}, "count": 1})

		case path == "/api/v1/governance/maintenance" && method == http.MethodPost:
			var w2 model.MaintenanceWindow
			_ = json.NewDecoder(r.Body).Decode(&w2)
			w2.ID = "mw-new-01"
			w2.CreatedAt = now
			w.WriteHeader(http.StatusCreated)
			encode(w2)

		case path == "/api/v1/governance/maintenance/mw-prod-patch-01" && method == http.MethodGet:
			encode(mw)

		case path == "/api/v1/governance/maintenance/mw-prod-patch-01" && method == http.MethodPut:
			var w2 model.MaintenanceWindow
			_ = json.NewDecoder(r.Body).Decode(&w2)
			w2.ID = "mw-prod-patch-01"
			encode(w2)

		case path == "/api/v1/governance/maintenance/mw-prod-patch-01" && method == http.MethodDelete:
			encode(map[string]any{"deleted": true, "maintenance_window_id": "mw-prod-patch-01"})

		case path == "/api/v1/governance/maintenance/mw-prod-patch-01/cancel" && method == http.MethodPost:
			var req struct {
				Reason string `json:"reason"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			encode(map[string]any{"cancelled": true, "maintenance_window_id": "mw-prod-patch-01", "reason": req.Reason})

		case path == "/api/v1/governance/maintenance/evaluate" && method == http.MethodGet:
			encode(map[string]any{"org_id": "org-prod-01", "active_windows": []model.MaintenanceWindow{mw}, "count": 1, "eval_time": now})

		case path == "/api/v1/governance/maintenance/mw-nonexistent" && method == http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
			encode(model.APIErrorResponse{Error: model.APIErrorDetail{
				Code:    "MAINTENANCE_WINDOW_NOT_FOUND",
				Message: "maintenance window not found",
			}})

		// ---- escalation ----
		case path == "/api/v1/governance/escalation" && method == http.MethodGet:
			encode(map[string]any{"escalation_policies": []model.EscalationPolicy{escPol}, "count": 1})

		case path == "/api/v1/governance/escalation" && method == http.MethodPost:
			var p model.EscalationPolicy
			_ = json.NewDecoder(r.Body).Decode(&p)
			p.ID = "esc-new-01"
			p.CreatedAt = now
			w.WriteHeader(http.StatusCreated)
			encode(p)

		case path == "/api/v1/governance/escalation/esc-critical-oncall" && method == http.MethodGet:
			encode(escPol)

		case path == "/api/v1/governance/escalation/esc-critical-oncall" && method == http.MethodPut:
			var p model.EscalationPolicy
			_ = json.NewDecoder(r.Body).Decode(&p)
			p.ID = "esc-critical-oncall"
			encode(p)

		case path == "/api/v1/governance/escalation/esc-critical-oncall" && method == http.MethodDelete:
			encode(map[string]any{"deleted": true, "escalation_policy_id": "esc-critical-oncall"})

		case path == "/api/v1/governance/escalation/evaluate" && method == http.MethodPost:
			encode(escResult)

		case path == "/api/v1/governance/escalation/esc-nonexistent" && method == http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
			encode(model.APIErrorResponse{Error: model.APIErrorDetail{
				Code:    "ESCALATION_POLICY_NOT_FOUND",
				Message: "escalation policy not found",
			}})

		// ---- suppression ----
		case path == "/api/v1/governance/suppression/evaluate" && method == http.MethodPost:
			var req governance.SuppressionEvaluationRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			dec := suppDec
			if req.NodeID != "" {
				dec.NodeID = req.NodeID
			}
			encode(dec)

		case path == "/api/v1/governance/suppression" && method == http.MethodGet:
			encode(map[string]any{"suppression_decisions": []model.SuppressionDecision{suppDec}, "count": 1})

		case path == "/api/v1/governance/suppression/supp-dec-01" && method == http.MethodGet:
			encode(suppDec)

		case path == "/api/v1/governance/suppression/supp-nonexistent" && method == http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
			encode(model.APIErrorResponse{Error: model.APIErrorDetail{
				Code:    "SUPPRESSION_DECISION_NOT_FOUND",
				Message: "suppression decision not found",
			}})

		// ---- audit ----
		case path == "/api/v1/governance/audit" && method == http.MethodGet:
			et := r.URL.Query().Get("event_type")
			events := []model.AuditEvent{auditEv}
			if et != "" && et != auditEv.EventType {
				events = []model.AuditEvent{}
			}
			encode(map[string]any{"audit_events": events, "count": len(events)})

		case path == "/api/v1/governance/audit" && method == http.MethodPost:
			var ev model.AuditEvent
			_ = json.NewDecoder(r.Body).Decode(&ev)
			ev.ID = "audit-new-001"
			ev.Timestamp = now
			w.WriteHeader(http.StatusCreated)
			encode(ev)

		// ---- auth test ----
		case path == "/api/v1/governance/orgs" && r.Header.Get("Authorization") == "":
			unauthorized()

		default:
			notFound()
		}
	}))

	return ts
}

// -----------------------------------------------------------------------------
// 1. Organisation Tests
// -----------------------------------------------------------------------------

func TestCmd_Gov_Org_List_HumanReadable(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	out, err := captureStdout(func() error {
		return orgListCmd.RunE(orgListCmd, nil)
	})
	if err != nil {
		t.Fatalf("org list failed: %v", err)
	}
	if !strings.Contains(out, "org-prod-01") {
		t.Errorf("expected org ID in output, got: %s", out)
	}
	if !strings.Contains(out, "Production Org") {
		t.Errorf("expected org name in output, got: %s", out)
	}
}

func TestCmd_Gov_Org_List_JSON(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govFormat = "json"
	out, err := captureStdout(func() error {
		return orgListCmd.RunE(orgListCmd, nil)
	})
	if err != nil {
		t.Fatalf("org list JSON failed: %v", err)
	}

	var orgs []model.Organization
	if err := json.Unmarshal([]byte(out), &orgs); err != nil {
		t.Fatalf("failed to decode org list JSON: %v\nOutput: %s", err, out)
	}
	if len(orgs) != 1 {
		t.Errorf("expected 1 org, got len=%d", len(orgs))
	}
	if orgs[0].ID != "org-prod-01" {
		t.Errorf("unexpected org ID: %s", orgs[0].ID)
	}
}

func TestCmd_Gov_Org_Get_HumanReadable(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	out, err := captureStdout(func() error {
		return orgGetCmd.RunE(orgGetCmd, []string{"org-prod-01"})
	})
	if err != nil {
		t.Fatalf("org get failed: %v", err)
	}
	if !strings.Contains(out, "org-prod-01") {
		t.Errorf("expected org ID in output, got: %s", out)
	}
	if !strings.Contains(out, "Production Org") {
		t.Errorf("expected org name in output, got: %s", out)
	}
	if !strings.Contains(out, "active") {
		t.Errorf("expected status 'active' in output, got: %s", out)
	}
}

func TestCmd_Gov_Org_Get_JSON(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govFormat = "json"
	out, err := captureStdout(func() error {
		return orgGetCmd.RunE(orgGetCmd, []string{"org-prod-01"})
	})
	if err != nil {
		t.Fatalf("org get JSON failed: %v", err)
	}
	var o model.Organization
	if err := json.Unmarshal([]byte(out), &o); err != nil {
		t.Fatalf("failed to decode org JSON: %v\nOutput: %s", err, out)
	}
	if o.ID != "org-prod-01" || o.Status != model.OrgStatusActive {
		t.Errorf("unexpected org decoded: %+v", o)
	}
}

func TestCmd_Gov_Org_Get_NotFound(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	_, err := captureStdout(func() error {
		return orgGetCmd.RunE(orgGetCmd, []string{"org-nonexistent"})
	})
	if err == nil {
		t.Fatal("expected error for nonexistent org, got nil")
	}
	if !strings.Contains(err.Error(), "ORGANIZATION_NOT_FOUND") {
		t.Errorf("expected ORGANIZATION_NOT_FOUND error code, got: %v", err)
	}
}

func TestCmd_Gov_Org_ServerError(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	_, err := captureStdout(func() error {
		return orgGetCmd.RunE(orgGetCmd, []string{"org-error"})
	})
	if err == nil {
		t.Fatal("expected error for server error, got nil")
	}
	if !strings.Contains(err.Error(), "500") && !strings.Contains(err.Error(), "INTERNAL_ERROR") {
		t.Errorf("expected HTTP 500 or INTERNAL_ERROR, got: %v", err)
	}
}

func TestCmd_Gov_MissingServerURL(t *testing.T) {
	resetGovFlags()
	globalCfg = nil

	_, err := getGovernanceClient()
	if err == nil {
		t.Fatal("expected error when server URL missing, got nil")
	}
	if !strings.Contains(err.Error(), "server URL is required") {
		t.Errorf("expected 'server URL is required' message, got: %v", err)
	}
}

// -----------------------------------------------------------------------------
// 2. Fleet Group Tests
// -----------------------------------------------------------------------------

func TestCmd_Gov_Group_List(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govOrgID = "org-prod-01"
	out, err := captureStdout(func() error {
		return groupListCmd.RunE(groupListCmd, nil)
	})
	if err != nil {
		t.Fatalf("group list failed: %v", err)
	}
	if !strings.Contains(out, "grp-prod-db") {
		t.Errorf("expected group ID in output, got: %s", out)
	}
	if !strings.Contains(out, "Production Database Tier") {
		t.Errorf("expected group name in output, got: %s", out)
	}
}

func TestCmd_Gov_Group_Get(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	out, err := captureStdout(func() error {
		return groupGetCmd.RunE(groupGetCmd, []string{"grp-prod-db"})
	})
	if err != nil {
		t.Fatalf("group get failed: %v", err)
	}
	if !strings.Contains(out, "grp-prod-db") {
		t.Errorf("expected group ID in output, got: %s", out)
	}
	if !strings.Contains(out, "org-prod-01") {
		t.Errorf("expected org ID in output, got: %s", out)
	}
}

func TestCmd_Gov_Group_Get_NotFound(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	_, err := captureStdout(func() error {
		return groupGetCmd.RunE(groupGetCmd, []string{"grp-nonexistent"})
	})
	if err == nil {
		t.Fatal("expected error for nonexistent group, got nil")
	}
	if !strings.Contains(err.Error(), "FLEET_GROUP_NOT_FOUND") {
		t.Errorf("expected FLEET_GROUP_NOT_FOUND, got: %v", err)
	}
}

func TestCmd_Gov_Group_Members(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	out, err := captureStdout(func() error {
		return groupMembersCmd.RunE(groupMembersCmd, []string{"grp-prod-db"})
	})
	if err != nil {
		t.Fatalf("group members failed: %v", err)
	}
	if !strings.Contains(out, "node-prod-01") {
		t.Errorf("expected node-prod-01 in members output, got: %s", out)
	}
}

func TestCmd_Gov_Group_AddMember(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	out, err := captureStdout(func() error {
		return groupAddMemberCmd.RunE(groupAddMemberCmd, []string{"grp-prod-db", "node-prod-01"})
	})
	if err != nil {
		t.Fatalf("group add-member failed: %v", err)
	}
	if !strings.Contains(out, "node-prod-01") && !strings.Contains(out, "grp-prod-db") {
		t.Errorf("expected confirmation in output, got: %s", out)
	}
}

func TestCmd_Gov_Group_RemoveMember(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	out, err := captureStdout(func() error {
		return groupRemoveMemberCmd.RunE(groupRemoveMemberCmd, []string{"grp-prod-db", "node-prod-01"})
	})
	if err != nil {
		t.Fatalf("group remove-member failed: %v", err)
	}
	if !strings.Contains(out, "node-prod-01") && !strings.Contains(out, "grp-prod-db") {
		t.Errorf("expected removal confirmation, got: %s", out)
	}
}

func TestCmd_Gov_Group_Subtree(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	out, err := captureStdout(func() error {
		return groupSubtreeCmd.RunE(groupSubtreeCmd, []string{"grp-prod-db"})
	})
	if err != nil {
		t.Fatalf("group subtree failed: %v", err)
	}
	if !strings.Contains(out, "node-prod-01") {
		t.Errorf("expected node-prod-01 in subtree, got: %s", out)
	}
	if !strings.Contains(out, "node-prod-02") {
		t.Errorf("expected node-prod-02 in subtree, got: %s", out)
	}
}

func TestCmd_Gov_Group_Hierarchy(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govOrgID = "org-prod-01"
	out, err := captureStdout(func() error {
		return groupHierarchyCmd.RunE(groupHierarchyCmd, nil)
	})
	if err != nil {
		t.Fatalf("group hierarchy failed: %v", err)
	}
	if !strings.Contains(out, "grp-prod-db") {
		t.Errorf("expected group in hierarchy output, got: %s", out)
	}
}

// -----------------------------------------------------------------------------
// 3. Ownership Tests
// -----------------------------------------------------------------------------

func TestCmd_Gov_Ownership_Get(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	out, err := captureStdout(func() error {
		return ownershipGetCmd.RunE(ownershipGetCmd, []string{"node-prod-01"})
	})
	if err != nil {
		t.Fatalf("ownership get failed: %v", err)
	}
	if !strings.Contains(out, "platform-team") {
		t.Errorf("expected team in output, got: %s", out)
	}
}

func TestCmd_Gov_Ownership_Get_JSON(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govFormat = "json"
	out, err := captureStdout(func() error {
		return ownershipGetCmd.RunE(ownershipGetCmd, []string{"node-prod-01"})
	})
	if err != nil {
		t.Fatalf("ownership get JSON failed: %v", err)
	}
	var own model.NodeOwnershipMetadata
	if err := json.Unmarshal([]byte(out), &own); err != nil {
		t.Fatalf("failed to decode ownership JSON: %v\nOutput: %s", err, out)
	}
	if own.OwnerTeam != "platform-team" {
		t.Errorf("unexpected ownership decoded: %+v", own)
	}
}

func TestCmd_Gov_Ownership_Resolve(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	out, err := captureStdout(func() error {
		return ownershipResolveCmd.RunE(ownershipResolveCmd, []string{"node-prod-01"})
	})
	if err != nil {
		t.Fatalf("ownership resolve failed: %v", err)
	}
	if !strings.Contains(out, "node-prod-01") {
		t.Errorf("expected node ID in resolve output, got: %s", out)
	}
}

// -----------------------------------------------------------------------------
// 4. Policy Tests
// -----------------------------------------------------------------------------

func TestCmd_Gov_Policy_List(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govOrgID = "org-prod-01"
	out, err := captureStdout(func() error {
		return policyListCmd.RunE(policyListCmd, nil)
	})
	if err != nil {
		t.Fatalf("policy list failed: %v", err)
	}
	if !strings.Contains(out, "pol-cpu-threshold") {
		t.Errorf("expected policy ID in output, got: %s", out)
	}
	if !strings.Contains(out, "CPU Threshold Policy") {
		t.Errorf("expected policy name in output, got: %s", out)
	}
	if !strings.Contains(out, "active") {
		t.Errorf("expected status active in output, got: %s", out)
	}
}

func TestCmd_Gov_Policy_List_WithFilters(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govOrgID = "org-prod-01"
	govStatus = "active"
	govCategory = "resource_thresholds"
	govSearch = "CPU"
	govLimit = 10
	govOffset = 0
	out, err := captureStdout(func() error {
		return policyListCmd.RunE(policyListCmd, nil)
	})
	if err != nil {
		t.Fatalf("policy list with filters failed: %v", err)
	}
	if !strings.Contains(out, "pol-cpu-threshold") {
		t.Errorf("expected policy in filtered output, got: %s", out)
	}
}

func TestCmd_Gov_Policy_Get(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	out, err := captureStdout(func() error {
		return policyGetCmd.RunE(policyGetCmd, []string{"pol-cpu-threshold"})
	})
	if err != nil {
		t.Fatalf("policy get failed: %v", err)
	}
	if !strings.Contains(out, "pol-cpu-threshold") {
		t.Errorf("expected policy ID, got: %s", out)
	}
}

func TestCmd_Gov_Policy_Get_NotFound(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	_, err := captureStdout(func() error {
		return policyGetCmd.RunE(policyGetCmd, []string{"pol-nonexistent"})
	})
	if err == nil {
		t.Fatal("expected error for nonexistent policy, got nil")
	}
	if !strings.Contains(err.Error(), "POLICY_NOT_FOUND") {
		t.Errorf("expected POLICY_NOT_FOUND, got: %v", err)
	}
}

func TestCmd_Gov_Policy_Revisions(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	out, err := captureStdout(func() error {
		return policyRevisionsCmd.RunE(policyRevisionsCmd, []string{"pol-cpu-threshold"})
	})
	if err != nil {
		t.Fatalf("policy revisions failed: %v", err)
	}
	if !strings.Contains(out, "operator") {
		t.Errorf("expected author in revisions output, got: %s", out)
	}
	if !strings.Contains(out, "Initial revision") {
		t.Errorf("expected change summary in revisions output, got: %s", out)
	}
}

func TestCmd_Gov_Policy_SetStatus(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	out, err := captureStdout(func() error {
		return policySetStatusCmd.RunE(policySetStatusCmd, []string{"pol-cpu-threshold", "disabled"})
	})
	if err != nil {
		t.Fatalf("policy set-status failed: %v", err)
	}
	if !strings.Contains(out, "pol-cpu-threshold") {
		t.Errorf("expected policy ID in output, got: %s", out)
	}
}

// -----------------------------------------------------------------------------
// 5. Assignment Tests
// -----------------------------------------------------------------------------

func TestCmd_Gov_Assignment_List(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govOrgID = "org-prod-01"
	out, err := captureStdout(func() error {
		return assignmentListCmd.RunE(assignmentListCmd, nil)
	})
	if err != nil {
		t.Fatalf("assignment list failed: %v", err)
	}
	if !strings.Contains(out, "asgn-cpu-prod-01") {
		t.Errorf("expected assignment ID in output, got: %s", out)
	}
	if !strings.Contains(out, "pol-cpu-threshold") {
		t.Errorf("expected policy ID in assignment output, got: %s", out)
	}
}

func TestCmd_Gov_Assignment_Get(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	out, err := captureStdout(func() error {
		return assignmentGetCmd.RunE(assignmentGetCmd, []string{"asgn-cpu-prod-01"})
	})
	if err != nil {
		t.Fatalf("assignment get failed: %v", err)
	}
	if !strings.Contains(out, "asgn-cpu-prod-01") {
		t.Errorf("expected assignment ID, got: %s", out)
	}
}

func TestCmd_Gov_Assignment_Get_NotFound(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	_, err := captureStdout(func() error {
		return assignmentGetCmd.RunE(assignmentGetCmd, []string{"asgn-nonexistent"})
	})
	if err == nil {
		t.Fatal("expected error for nonexistent assignment, got nil")
	}
	if !strings.Contains(err.Error(), "POLICY_ASSIGNMENT_NOT_FOUND") {
		t.Errorf("expected POLICY_ASSIGNMENT_NOT_FOUND, got: %v", err)
	}
}

// -----------------------------------------------------------------------------
// 6. Evaluation Tests
// -----------------------------------------------------------------------------

func TestCmd_Gov_Eval_List(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govOrgID = "org-prod-01"
	out, err := captureStdout(func() error {
		return evalListCmd.RunE(evalListCmd, nil)
	})
	if err != nil {
		t.Fatalf("evaluate list failed: %v", err)
	}
	if !strings.Contains(out, "eval-prod-01-001") {
		t.Errorf("expected evaluation ID in output, got: %s", out)
	}
}

func TestCmd_Gov_Eval_Get(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	out, err := captureStdout(func() error {
		return evalGetCmd.RunE(evalGetCmd, []string{"eval-prod-01-001"})
	})
	if err != nil {
		t.Fatalf("evaluate get failed: %v", err)
	}
	if !strings.Contains(out, "eval-prod-01-001") {
		t.Errorf("expected evaluation ID, got: %s", out)
	}
	if !strings.Contains(out, "node-prod-01") {
		t.Errorf("expected node ID in evaluation output, got: %s", out)
	}
}

func TestCmd_Gov_Eval_Get_NotFound(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	_, err := captureStdout(func() error {
		return evalGetCmd.RunE(evalGetCmd, []string{"eval-nonexistent"})
	})
	if err == nil {
		t.Fatal("expected error for nonexistent evaluation, got nil")
	}
}

func TestCmd_Gov_Eval_Latest(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govOrgID = "org-prod-01"
	out, err := captureStdout(func() error {
		return evalLatestCmd.RunE(evalLatestCmd, []string{"node-prod-01"})
	})
	if err != nil {
		t.Fatalf("evaluate latest failed: %v", err)
	}
	if !strings.Contains(out, "eval-prod-01-001") {
		t.Errorf("expected evaluation ID in latest output, got: %s", out)
	}
}

func TestCmd_Gov_Eval_Get_JSON(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govFormat = "json"
	out, err := captureStdout(func() error {
		return evalGetCmd.RunE(evalGetCmd, []string{"eval-prod-01-001"})
	})
	if err != nil {
		t.Fatalf("evaluate get JSON failed: %v", err)
	}
	var exec model.EvaluationExecution
	if err := json.Unmarshal([]byte(out), &exec); err != nil {
		t.Fatalf("failed to decode evaluation JSON: %v\nOutput: %s", err, out)
	}
	if exec.ID != "eval-prod-01-001" {
		t.Errorf("unexpected evaluation ID: %s", exec.ID)
	}
	if exec.Summary.TotalRules != 5 {
		t.Errorf("expected 5 total rules, got: %d", exec.Summary.TotalRules)
	}
}

// -----------------------------------------------------------------------------
// 7. Findings Tests
// -----------------------------------------------------------------------------

func TestCmd_Gov_Findings_List(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govOrgID = "org-prod-01"
	out, err := captureStdout(func() error {
		return findingsListCmd.RunE(findingsListCmd, nil)
	})
	if err != nil {
		t.Fatalf("findings list failed: %v", err)
	}
	if !strings.Contains(out, "fnd-cpu-node01") {
		t.Errorf("expected finding ID in output, got: %s", out)
	}
	if !strings.Contains(out, "CPU Utilization Threshold") {
		t.Errorf("expected rule name in output, got: %s", out)
	}
	if !strings.Contains(out, "WARNING") && !strings.Contains(out, "warning") {
		t.Errorf("expected severity warning in output, got: %s", out)
	}
}

func TestCmd_Gov_Findings_Get(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	out, err := captureStdout(func() error {
		return findingsGetCmd.RunE(findingsGetCmd, []string{"fnd-cpu-node01"})
	})
	if err != nil {
		t.Fatalf("findings get failed: %v", err)
	}
	if !strings.Contains(out, "fnd-cpu-node01") {
		t.Errorf("expected finding ID, got: %s", out)
	}
	if !strings.Contains(out, "87% exceeds 80%") {
		t.Errorf("expected finding message in output, got: %s", out)
	}
}

func TestCmd_Gov_Findings_Get_NotFound(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	_, err := captureStdout(func() error {
		return findingsGetCmd.RunE(findingsGetCmd, []string{"fnd-nonexistent"})
	})
	if err == nil {
		t.Fatal("expected error for nonexistent finding, got nil")
	}
}

func TestCmd_Gov_Findings_List_JSON(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govFormat = "json"
	out, err := captureStdout(func() error {
		return findingsListCmd.RunE(findingsListCmd, nil)
	})
	if err != nil {
		t.Fatalf("findings list JSON failed: %v", err)
	}
	var resp struct {
		Findings []model.ComplianceFinding `json:"findings"`
		Count    int                       `json:"count"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("failed to decode findings JSON: %v\nOutput: %s", err, out)
	}
	if resp.Count != 1 || resp.Findings[0].ID != "fnd-cpu-node01" {
		t.Errorf("unexpected findings decoded: %+v", resp)
	}
}

// -----------------------------------------------------------------------------
// 8. Compliance Tests
// -----------------------------------------------------------------------------

func TestCmd_Gov_Compliance_Node(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govOrgID = "org-prod-01"
	out, err := captureStdout(func() error {
		return compNodeCmd.RunE(compNodeCmd, []string{"node-prod-01"})
	})
	if err != nil {
		t.Fatalf("compliance node failed: %v", err)
	}
	if !strings.Contains(out, "75.0%") {
		t.Errorf("expected compliance ratio 75.0%% in output, got: %s", out)
	}
	if !strings.Contains(out, "org-prod-01") {
		t.Errorf("expected org ID in output, got: %s", out)
	}
}

func TestCmd_Gov_Compliance_Group(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govOrgID = "org-prod-01"
	out, err := captureStdout(func() error {
		return compGroupCmd.RunE(compGroupCmd, []string{"grp-prod-db"})
	})
	if err != nil {
		t.Fatalf("compliance group failed: %v", err)
	}
	if !strings.Contains(out, "75.0%") {
		t.Errorf("expected 75.0%% compliance ratio in group output, got: %s", out)
	}
}

func TestCmd_Gov_Compliance_Org(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govOrgID = "org-prod-01"
	out, err := captureStdout(func() error {
		return compOrgCmd.RunE(compOrgCmd, []string{"org-prod-01"})
	})
	if err != nil {
		t.Fatalf("compliance org failed: %v", err)
	}
	if !strings.Contains(out, "75.0%") {
		t.Errorf("expected 75.0%% compliance ratio in org output, got: %s", out)
	}
}

func TestCmd_Gov_Compliance_Node_JSON(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govFormat = "json"
	govOrgID = "org-prod-01"
	out, err := captureStdout(func() error {
		return compNodeCmd.RunE(compNodeCmd, []string{"node-prod-01"})
	})
	if err != nil {
		t.Fatalf("compliance node JSON failed: %v", err)
	}
	var rollup model.ComplianceRollupSummary
	if err := json.Unmarshal([]byte(out), &rollup); err != nil {
		t.Fatalf("failed to decode compliance rollup JSON: %v\nOutput: %s", err, out)
	}
	if rollup.TotalNodes != 4 || rollup.CompliantNodes != 3 {
		t.Errorf("unexpected rollup: total=%d compliant=%d", rollup.TotalNodes, rollup.CompliantNodes)
	}
}

// -----------------------------------------------------------------------------
// 9. Maintenance Window Tests
// -----------------------------------------------------------------------------

func TestCmd_Gov_Maintenance_List(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govOrgID = "org-prod-01"
	out, err := captureStdout(func() error {
		return maintListCmd.RunE(maintListCmd, nil)
	})
	if err != nil {
		t.Fatalf("maintenance list failed: %v", err)
	}
	if !strings.Contains(out, "mw-prod-patch-01") {
		t.Errorf("expected maintenance window ID in output, got: %s", out)
	}
	if !strings.Contains(out, "Monthly Patch Window") {
		t.Errorf("expected window name in output, got: %s", out)
	}
	if !strings.Contains(out, "scheduled") {
		t.Errorf("expected status 'scheduled' in output, got: %s", out)
	}
}

func TestCmd_Gov_Maintenance_Get(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	out, err := captureStdout(func() error {
		return maintGetCmd.RunE(maintGetCmd, []string{"mw-prod-patch-01"})
	})
	if err != nil {
		t.Fatalf("maintenance get failed: %v", err)
	}
	if !strings.Contains(out, "mw-prod-patch-01") {
		t.Errorf("expected window ID, got: %s", out)
	}
	if !strings.Contains(out, "node-prod-01") {
		t.Errorf("expected target node in output, got: %s", out)
	}
}

func TestCmd_Gov_Maintenance_Get_NotFound(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	_, err := captureStdout(func() error {
		return maintGetCmd.RunE(maintGetCmd, []string{"mw-nonexistent"})
	})
	if err == nil {
		t.Fatal("expected error for nonexistent maintenance window, got nil")
	}
	if !strings.Contains(err.Error(), "MAINTENANCE_WINDOW_NOT_FOUND") {
		t.Errorf("expected MAINTENANCE_WINDOW_NOT_FOUND, got: %v", err)
	}
}

func TestCmd_Gov_Maintenance_Cancel(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govReason = "Emergency deployment cancelled"
	out, err := captureStdout(func() error {
		return maintCancelCmd.RunE(maintCancelCmd, []string{"mw-prod-patch-01"})
	})
	if err != nil {
		t.Fatalf("maintenance cancel failed: %v", err)
	}
	if !strings.Contains(out, "mw-prod-patch-01") {
		t.Errorf("expected window ID in cancel output, got: %s", out)
	}
}

func TestCmd_Gov_Maintenance_Evaluate(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govOrgID = "org-prod-01"
	out, err := captureStdout(func() error {
		return maintEvaluateCmd.RunE(maintEvaluateCmd, nil)
	})
	if err != nil {
		t.Fatalf("maintenance evaluate failed: %v", err)
	}
	if !strings.Contains(out, "mw-prod-patch-01") {
		t.Errorf("expected active window in evaluate output, got: %s", out)
	}
}

func TestCmd_Gov_Maintenance_List_JSON(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govFormat = "json"
	out, err := captureStdout(func() error {
		return maintListCmd.RunE(maintListCmd, nil)
	})
	if err != nil {
		t.Fatalf("maintenance list JSON failed: %v", err)
	}
	var resp struct {
		MaintenanceWindows []model.MaintenanceWindow `json:"maintenance_windows"`
		Count              int                       `json:"count"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("failed to decode maintenance JSON: %v\nOutput: %s", err, out)
	}
	if resp.Count != 1 || resp.MaintenanceWindows[0].ID != "mw-prod-patch-01" {
		t.Errorf("unexpected maintenance windows decoded: %+v", resp)
	}
}

// -----------------------------------------------------------------------------
// 10. Escalation Tests
// -----------------------------------------------------------------------------

func TestCmd_Gov_Escalation_List(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govOrgID = "org-prod-01"
	out, err := captureStdout(func() error {
		return escListCmd.RunE(escListCmd, nil)
	})
	if err != nil {
		t.Fatalf("escalation list failed: %v", err)
	}
	if !strings.Contains(out, "esc-critical-oncall") {
		t.Errorf("expected escalation policy ID in output, got: %s", out)
	}
	if !strings.Contains(out, "Critical On-Call Escalation") {
		t.Errorf("expected policy name in output, got: %s", out)
	}
}

func TestCmd_Gov_Escalation_Get(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	out, err := captureStdout(func() error {
		return escGetCmd.RunE(escGetCmd, []string{"esc-critical-oncall"})
	})
	if err != nil {
		t.Fatalf("escalation get failed: %v", err)
	}
	if !strings.Contains(out, "esc-critical-oncall") {
		t.Errorf("expected escalation policy ID, got: %s", out)
	}
	if !strings.Contains(out, "critical") {
		t.Errorf("expected severity in output, got: %s", out)
	}
}

func TestCmd_Gov_Escalation_Get_NotFound(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	_, err := captureStdout(func() error {
		return escGetCmd.RunE(escGetCmd, []string{"esc-nonexistent"})
	})
	if err == nil {
		t.Fatal("expected error for nonexistent escalation policy, got nil")
	}
	if !strings.Contains(err.Error(), "ESCALATION_POLICY_NOT_FOUND") {
		t.Errorf("expected ESCALATION_POLICY_NOT_FOUND, got: %v", err)
	}
}

func TestCmd_Gov_Escalation_Evaluate(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govOrgID = "org-prod-01"
	govIncidentID = "inc-prod-01"
	govIncidentTitle = "High CPU on node-prod-01"
	out, err := captureStdout(func() error {
		return escEvaluateCmd.RunE(escEvaluateCmd, nil)
	})
	if err != nil {
		t.Fatalf("escalation evaluate failed: %v", err)
	}
	if !strings.Contains(out, "esc-critical-oncall") {
		t.Errorf("expected policy ID in escalation evaluate output, got: %s", out)
	}
	if !strings.Contains(out, "Critical On-Call Escalation") {
		t.Errorf("expected policy name in output, got: %s", out)
	}
	if !strings.Contains(out, "1") {
		t.Errorf("expected stage number 1 in output, got: %s", out)
	}
}

func TestCmd_Gov_Escalation_Evaluate_JSON(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govFormat = "json"
	govOrgID = "org-prod-01"
	govIncidentID = "inc-prod-01"
	out, err := captureStdout(func() error {
		return escEvaluateCmd.RunE(escEvaluateCmd, nil)
	})
	if err != nil {
		t.Fatalf("escalation evaluate JSON failed: %v", err)
	}
	var result model.EscalationEvaluationResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to decode escalation result JSON: %v\nOutput: %s", err, out)
	}
	if result.PolicyID != "esc-critical-oncall" {
		t.Errorf("expected policy ID esc-critical-oncall, got: %s", result.PolicyID)
	}
	if result.CurrentStage != 1 {
		t.Errorf("expected current stage 1, got: %d", result.CurrentStage)
	}
	if result.NextStageNumber == nil || *result.NextStageNumber != 2 {
		t.Errorf("expected next stage 2, got: %v", result.NextStageNumber)
	}
}

// -----------------------------------------------------------------------------
// 11. Suppression Tests
// -----------------------------------------------------------------------------

func TestCmd_Gov_Suppression_Evaluate_Suppressed(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govOrgID = "org-prod-01"
	govNodeID = "node-prod-01"
	govAlertID = "alert-cpu-01"
	govCategory = "cpu"
	govSeverity = "warning"

	out, err := captureStdout(func() error {
		return suppEvaluateCmd.RunE(suppEvaluateCmd, nil)
	})
	if err != nil {
		t.Fatalf("suppression evaluate failed: %v", err)
	}
	if !strings.Contains(out, "true") {
		t.Errorf("expected suppressed=true in output, got: %s", out)
	}
	if !strings.Contains(out, "mw-prod-patch-01") {
		t.Errorf("expected window ID in output, got: %s", out)
	}
	if !strings.Contains(out, "in_maintenance_window") {
		t.Errorf("expected reason in_maintenance_window in output, got: %s", out)
	}
}

func TestCmd_Gov_Suppression_Evaluate_NodeIDForwarded(t *testing.T) {
	resetGovFlags()

	var capturedReq governance.SuppressionEvaluationRequest
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/governance/suppression/evaluate" && r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&capturedReq)
			dec := govFixtureSuppressionDecision()
			dec.NodeID = capturedReq.NodeID
			_ = json.NewEncoder(w).Encode(dec)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	govServerURL = ts.URL
	govOrgID = "org-prod-01"
	govNodeID = "node-special-99"
	govCategory = "disk"
	govSeverity = "warning"

	_, err := captureStdout(func() error {
		return suppEvaluateCmd.RunE(suppEvaluateCmd, nil)
	})
	if err != nil {
		t.Fatalf("suppression evaluate failed: %v", err)
	}
	if capturedReq.NodeID != "node-special-99" {
		t.Errorf("expected node_id node-special-99 in request body, got: %q", capturedReq.NodeID)
	}
	if capturedReq.Category != "disk" {
		t.Errorf("expected category disk in request body, got: %q", capturedReq.Category)
	}
}

func TestCmd_Gov_Suppression_Evaluate_JSON(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govFormat = "json"
	govOrgID = "org-prod-01"
	govNodeID = "node-prod-01"
	govCategory = "cpu"
	govSeverity = "warning"

	out, err := captureStdout(func() error {
		return suppEvaluateCmd.RunE(suppEvaluateCmd, nil)
	})
	if err != nil {
		t.Fatalf("suppression evaluate JSON failed: %v", err)
	}
	var dec model.SuppressionDecision
	if err := json.Unmarshal([]byte(out), &dec); err != nil {
		t.Fatalf("failed to decode suppression decision JSON: %v\nOutput: %s", err, out)
	}
	if dec.Outcome != model.SuppressionOutcomeSuppressed {
		t.Errorf("expected outcome suppressed, got: %s", dec.Outcome)
	}
	if dec.Reason != model.SuppressionReasonInMaintenanceWindow {
		t.Errorf("expected reason in_maintenance_window, got: %s", dec.Reason)
	}
	if dec.WindowID != "mw-prod-patch-01" {
		t.Errorf("expected window ID mw-prod-patch-01, got: %s", dec.WindowID)
	}
}

func TestCmd_Gov_Suppression_List(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govOrgID = "org-prod-01"
	out, err := captureStdout(func() error {
		return suppListCmd.RunE(suppListCmd, nil)
	})
	if err != nil {
		t.Fatalf("suppression list failed: %v", err)
	}
	if !strings.Contains(out, "supp-dec-01") {
		t.Errorf("expected decision ID in output, got: %s", out)
	}
	if !strings.Contains(out, "suppressed") {
		t.Errorf("expected outcome in output, got: %s", out)
	}
	if !strings.Contains(out, "mw-prod-patch-01") {
		t.Errorf("expected window ID in output, got: %s", out)
	}
}

func TestCmd_Gov_Suppression_List_FilterBySuppressed(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govOrgID = "org-prod-01"
	govEnabled = "true"

	out, err := captureStdout(func() error {
		return suppListCmd.RunE(suppListCmd, nil)
	})
	if err != nil {
		t.Fatalf("suppression list with filter failed: %v", err)
	}
	if !strings.Contains(out, "supp-dec-01") {
		t.Errorf("expected filtered decisions in output, got: %s", out)
	}
}

func TestCmd_Gov_Suppression_List_JSON(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govFormat = "json"
	out, err := captureStdout(func() error {
		return suppListCmd.RunE(suppListCmd, nil)
	})
	if err != nil {
		t.Fatalf("suppression list JSON failed: %v", err)
	}
	var resp struct {
		SuppressionDecisions []model.SuppressionDecision `json:"suppression_decisions"`
		Count                int                         `json:"count"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("failed to decode suppression list JSON: %v\nOutput: %s", err, out)
	}
	if resp.Count != 1 || resp.SuppressionDecisions[0].ID != "supp-dec-01" {
		t.Errorf("unexpected suppression decisions decoded: %+v", resp)
	}
}

func TestCmd_Gov_Suppression_Get(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	out, err := captureStdout(func() error {
		return suppGetCmd.RunE(suppGetCmd, []string{"supp-dec-01"})
	})
	if err != nil {
		t.Fatalf("suppression get failed: %v", err)
	}
	if !strings.Contains(out, "supp-dec-01") {
		t.Errorf("expected decision ID, got: %s", out)
	}
	if !strings.Contains(out, "suppressed") {
		t.Errorf("expected outcome in output, got: %s", out)
	}
	if !strings.Contains(out, "in_maintenance_window") {
		t.Errorf("expected reason in output, got: %s", out)
	}
}

func TestCmd_Gov_Suppression_Get_NotFound(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	_, err := captureStdout(func() error {
		return suppGetCmd.RunE(suppGetCmd, []string{"supp-nonexistent"})
	})
	if err == nil {
		t.Fatal("expected error for nonexistent suppression decision, got nil")
	}
	if !strings.Contains(err.Error(), "SUPPRESSION_DECISION_NOT_FOUND") {
		t.Errorf("expected SUPPRESSION_DECISION_NOT_FOUND, got: %v", err)
	}
}

func TestCmd_Gov_Suppression_Guardrail_Rendered(t *testing.T) {
	resetGovFlags()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		dec := model.SuppressionDecision{
			ID:          "supp-guardrail-01",
			OrgID:       "org-prod-01",
			NodeID:      "node-prod-01",
			Outcome:     model.SuppressionOutcomeExempt,
			Reason:      model.SuppressionReasonSeverityExceedsThreshold,
			Severity:    model.SeverityCritical,
			EvaluatedAt: govFixtureTime(),
		}
		_ = json.NewEncoder(w).Encode(dec)
	}))
	defer ts.Close()

	govServerURL = ts.URL
	govNodeID = "node-prod-01"
	govSeverity = "critical"

	out, err := captureStdout(func() error {
		return suppEvaluateCmd.RunE(suppEvaluateCmd, nil)
	})
	if err != nil {
		t.Fatalf("suppression evaluate guardrail test failed: %v", err)
	}
	if !strings.Contains(out, "Guardrail Triggered") {
		t.Errorf("expected guardrail warning in output, got: %s", out)
	}
}

// -----------------------------------------------------------------------------
// 12. Audit Tests
// -----------------------------------------------------------------------------

func TestCmd_Gov_Audit_Query(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	out, err := captureStdout(func() error {
		return auditQueryCmd.RunE(auditQueryCmd, nil)
	})
	if err != nil {
		t.Fatalf("audit query failed: %v", err)
	}
	if !strings.Contains(out, "audit-gov-001") {
		t.Errorf("expected audit event ID in output, got: %s", out)
	}
	if !strings.Contains(out, "operator-01") {
		t.Errorf("expected actor identity in output, got: %s", out)
	}
}

func TestCmd_Gov_Audit_Query_JSON(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govFormat = "json"
	out, err := captureStdout(func() error {
		return auditQueryCmd.RunE(auditQueryCmd, nil)
	})
	if err != nil {
		t.Fatalf("audit query JSON failed: %v", err)
	}
	var resp struct {
		AuditEvents []model.AuditEvent `json:"audit_events"`
		Count       int                `json:"count"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("failed to decode audit JSON: %v\nOutput: %s", err, out)
	}
	if resp.Count != 1 || resp.AuditEvents[0].ID != "audit-gov-001" {
		t.Errorf("unexpected audit events decoded: %+v", resp)
	}
	if resp.AuditEvents[0].Actor.Identity != "operator-01" {
		t.Errorf("expected actor identity operator-01, got: %s", resp.AuditEvents[0].Actor.Identity)
	}
}

func TestCmd_Gov_Audit_Query_EventTypeFilter(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govEventType = "governance.policy.created"
	out, err := captureStdout(func() error {
		return auditQueryCmd.RunE(auditQueryCmd, nil)
	})
	if err != nil {
		t.Fatalf("audit query with event type filter failed: %v", err)
	}
	if !strings.Contains(out, "audit-gov-001") {
		t.Errorf("expected matching event in output, got: %s", out)
	}

	resetGovFlags()
	govServerURL = ts.URL
	govEventType = "auth.failure"
	out, err = captureStdout(func() error {
		return auditQueryCmd.RunE(auditQueryCmd, nil)
	})
	if err != nil {
		t.Fatalf("audit query no-match failed: %v", err)
	}
	if !strings.Contains(out, "No governance audit events found") {
		t.Errorf("expected empty result message, got: %s", out)
	}
}

func TestCmd_Gov_Audit_Record(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govOrgID = "org-prod-01"
	govEventType = "governance.policy.created"
	govSeverity = "info"
	govOutcome = "success"
	govActorType = "cli"
	govActorID = "operator-01"
	govAction = "create_policy"
	govResourceType = "policy"
	govResourceID = "pol-cpu-threshold"
	govMessage = "Policy created by operator"

	out, err := captureStdout(func() error {
		return auditRecordCmd.RunE(auditRecordCmd, nil)
	})
	if err != nil {
		t.Fatalf("audit record failed: %v", err)
	}
	if !strings.Contains(out, "Recorded governance audit event") {
		t.Errorf("expected confirmation message, got: %s", out)
	}
}

func TestCmd_Gov_Audit_Record_DefaultsApplied(t *testing.T) {
	resetGovFlags()

	var capturedEvent model.AuditEvent
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/governance/audit" && r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&capturedEvent)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(capturedEvent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	govServerURL = ts.URL
	govOrgID = "org-prod-01"
	govAction = "test_action"

	_, err := captureStdout(func() error {
		return auditRecordCmd.RunE(auditRecordCmd, nil)
	})
	if err != nil {
		t.Fatalf("audit record defaults test failed: %v", err)
	}
	if capturedEvent.EventType != model.EventGovernancePolicyCreated {
		t.Errorf("expected default event type %q, got %q", model.EventGovernancePolicyCreated, capturedEvent.EventType)
	}
	if capturedEvent.Severity != model.AuditSeverityInfo {
		t.Errorf("expected default severity %q, got %q", model.AuditSeverityInfo, capturedEvent.Severity)
	}
	if capturedEvent.Outcome != model.AuditOutcomeSuccess {
		t.Errorf("expected default outcome %q, got %q", model.AuditOutcomeSuccess, capturedEvent.Outcome)
	}
}

func TestCmd_Gov_Audit_Record_ResourceCombined(t *testing.T) {
	resetGovFlags()

	var capturedEvent model.AuditEvent
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewDecoder(r.Body).Decode(&capturedEvent)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(capturedEvent)
	}))
	defer ts.Close()

	govServerURL = ts.URL
	govOrgID = "org-prod-01"
	govResourceType = "policy"
	govResourceID = "pol-cpu-threshold"
	govAction = "test"

	_, err := captureStdout(func() error {
		return auditRecordCmd.RunE(auditRecordCmd, nil)
	})
	if err != nil {
		t.Fatalf("audit record resource test failed: %v", err)
	}
	if capturedEvent.Resource != "policy:pol-cpu-threshold" {
		t.Errorf("expected resource 'policy:pol-cpu-threshold', got %q", capturedEvent.Resource)
	}
}

func TestCmd_Gov_Audit_Record_ActorForwarded(t *testing.T) {
	resetGovFlags()

	var capturedEvent model.AuditEvent
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewDecoder(r.Body).Decode(&capturedEvent)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(capturedEvent)
	}))
	defer ts.Close()

	govServerURL = ts.URL
	govActorType = model.ActorTypeCLI
	govActorID = "admin-user"
	govAction = "test"

	_, err := captureStdout(func() error {
		return auditRecordCmd.RunE(auditRecordCmd, nil)
	})
	if err != nil {
		t.Fatalf("audit record actor test failed: %v", err)
	}
	if capturedEvent.Actor.Type != model.ActorTypeCLI {
		t.Errorf("expected actor type %q, got %q", model.ActorTypeCLI, capturedEvent.Actor.Type)
	}
	if capturedEvent.Actor.Identity != "admin-user" {
		t.Errorf("expected actor identity 'admin-user', got %q", capturedEvent.Actor.Identity)
	}
}

// -----------------------------------------------------------------------------
// 13. Cross-cutting: auth failure and formatted output
// -----------------------------------------------------------------------------

func TestCmd_Gov_AuthFailure_PropagatedAsError(t *testing.T) {
	resetGovFlags()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(model.APIErrorResponse{Error: model.APIErrorDetail{
			Code:    "UNAUTHORIZED",
			Message: "authentication required",
		}})
	}))
	defer ts.Close()

	govServerURL = ts.URL
	_, err := captureStdout(func() error {
		return orgListCmd.RunE(orgListCmd, nil)
	})
	if err == nil {
		t.Fatal("expected auth error, got nil")
	}
	if !strings.Contains(err.Error(), "401") && !strings.Contains(err.Error(), "UNAUTHORIZED") {
		t.Errorf("expected 401/UNAUTHORIZED error, got: %v", err)
	}
}

func TestCmd_Gov_YAML_Output(t *testing.T) {
	resetGovFlags()
	ts := setupGovernanceMockServer(t)
	defer ts.Close()

	govServerURL = ts.URL
	govFormat = "yaml"
	out, err := captureStdout(func() error {
		return orgListCmd.RunE(orgListCmd, nil)
	})
	if err != nil {
		t.Fatalf("org list YAML failed: %v", err)
	}
	if !strings.Contains(out, "org-prod-01") {
		t.Errorf("expected org ID in YAML output, got: %s", out)
	}
	if strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("expected YAML output, got JSON-looking output: %s", out)
	}
}

func TestCmd_Gov_EmptyList_Message(t *testing.T) {
	resetGovFlags()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]model.Organization{})
	}))
	defer ts.Close()

	govServerURL = ts.URL
	out, err := captureStdout(func() error {
		return orgListCmd.RunE(orgListCmd, nil)
	})
	if err != nil {
		t.Fatalf("empty org list failed: %v", err)
	}
	if !strings.Contains(out, "No organizations found") {
		t.Errorf("expected empty list message 'No organizations found', got: %s", out)
	}
}
