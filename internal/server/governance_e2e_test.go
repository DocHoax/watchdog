package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/governance"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T) (*Server, storage.Storage, *httptest.Server) {
	t.Helper()
	store, err := storage.NewSQLiteStorage(storage.Config{Path: ":memory:"})
	require.NoError(t, err)

	cfg := config.DefaultConfig()
	cfg.Agent.BindAddress = "127.0.0.1"
	cfg.Agent.Token = "" // loopback allows empty token

	srv := NewServer(cfg, nil, store, nil, nil, nil)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/governance", srv.handleGovernanceRoute)
	mux.HandleFunc("/api/v1/governance/", srv.handleGovernanceRoute)

	ts := httptest.NewServer(mux)
	return srv, store, ts
}

func TestE2E_Governance_Organizations(t *testing.T) {
	_, store, ts := setupTestServer(t)
	defer store.Close()
	defer ts.Close()

	client := governance.NewGovernanceClient(governance.ClientConfig{
		Endpoint: ts.URL,
	})
	ctx := context.Background()

	// 1. List organizations (default should be present)
	orgs, err := client.ListOrganizations(ctx)
	require.NoError(t, err)
	assert.Len(t, orgs, 1)
	assert.Equal(t, model.DefaultOrganizationID, orgs[0].ID)

	// 2. Create new organization
	now := time.Now().UTC().Truncate(time.Millisecond)
	newOrg := &model.Organization{
		ID:          "org-finance-01",
		Name:        "Finance Operations",
		DisplayName: "Financial Systems",
		Description: "Finance production tenant",
		Status:      model.OrgStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	created, err := client.CreateOrganization(ctx, newOrg)
	require.NoError(t, err)
	assert.Equal(t, "org-finance-01", created.ID)
	assert.Equal(t, "Finance Operations", created.Name)

	// 3. Retrieve organization by ID
	fetched, err := client.GetOrganization(ctx, "org-finance-01")
	require.NoError(t, err)
	assert.Equal(t, "org-finance-01", fetched.ID)
	assert.Equal(t, model.OrgStatusActive, fetched.Status)

	// 4. Update organization
	fetched.DisplayName = "Global Financial Systems"
	updated, err := client.UpdateOrganization(ctx, fetched.ID, fetched)
	require.NoError(t, err)
	assert.Equal(t, "Global Financial Systems", updated.DisplayName)

	// 5. Delete organization
	err = client.DeleteOrganization(ctx, "org-finance-01")
	require.NoError(t, err)

	// 6. Verify deletion (should return error)
	_, err = client.GetOrganization(ctx, "org-finance-01")
	assert.Error(t, err)
}

func TestE2E_Governance_FleetGroupsAndSubtree(t *testing.T) {
	_, store, ts := setupTestServer(t)
	defer store.Close()
	defer ts.Close()

	client := governance.NewGovernanceClient(governance.ClientConfig{
		Endpoint: ts.URL,
	})
	ctx := context.Background()

	// Create fleet group
	now := time.Now().UTC().Truncate(time.Millisecond)
	group := &model.FleetGroup{
		ID:          "grp-payment-tier",
		OrgID:       model.DefaultOrganizationID,
		Name:        "Payment Tier",
		Description: "Payment processing nodes",
		Type:        model.GroupTypeTier,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	created, err := client.CreateFleetGroup(ctx, group)
	require.NoError(t, err)
	assert.Equal(t, "grp-payment-tier", created.ID)

	// Add member node
	err = client.AddMember(ctx, "grp-payment-tier", "node-pay-01")
	require.NoError(t, err)

	err = client.AddMember(ctx, "grp-payment-tier", "node-pay-02")
	require.NoError(t, err)

	// Get members
	members, err := client.GetGroupMembers(ctx, "grp-payment-tier")
	require.NoError(t, err)
	assert.Len(t, members, 2)

	// Get subtree nodes
	nodes, err := client.GetSubtreeNodes(ctx, "grp-payment-tier")
	require.NoError(t, err)
	assert.Contains(t, nodes, "node-pay-01")
	assert.Contains(t, nodes, "node-pay-02")
}

func TestE2E_Governance_NodeOwnershipAndResolution(t *testing.T) {
	_, store, ts := setupTestServer(t)
	defer store.Close()
	defer ts.Close()

	client := governance.NewGovernanceClient(governance.ClientConfig{
		Endpoint: ts.URL,
	})
	ctx := context.Background()

	nodeID := "node-compute-01"
	own := &model.NodeOwnershipMetadata{
		OwnerTeam:          "sre-platform",
		ContactEmail:       "sre@example.com",
		Environment:        "production",
		Region:             "us-east-1",
		DataClassification: "confidential",
	}

	// Set ownership
	err := client.SetNodeOwnership(ctx, nodeID, own)
	require.NoError(t, err)

	// Get ownership
	fetched, err := client.GetNodeOwnership(ctx, nodeID)
	require.NoError(t, err)
	assert.Equal(t, "sre-platform", fetched.OwnerTeam)
	assert.Equal(t, "production", fetched.Environment)

	// Resolve ownership
	resolved, err := client.ResolveNodeOwnership(ctx, nodeID)
	require.NoError(t, err)
	assert.Equal(t, nodeID, resolved.NodeID)
	assert.Equal(t, "sre-platform", resolved.OwnerTeam)
}

func TestE2E_Governance_PoliciesAndAssignments(t *testing.T) {
	_, store, ts := setupTestServer(t)
	defer store.Close()
	defer ts.Close()

	client := governance.NewGovernanceClient(governance.ClientConfig{
		Endpoint: ts.URL,
	})
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)
	policy := &model.Policy{
		ID:          "pol-ram-limit",
		OrgID:       model.DefaultOrganizationID,
		Name:        "RAM Usage Limit",
		Description: "Memory usage cannot exceed 90%",
		Category:    model.PolicyCategoryResourceThresholds,
		Status:      model.PolicyStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	createdPol, err := client.CreatePolicy(ctx, policy)
	require.NoError(t, err)
	assert.Equal(t, "pol-ram-limit", createdPol.ID)

	// Create assignment
	asgn := &model.PolicyAssignment{
		ID:         "asgn-ram-node01",
		OrgID:      model.DefaultOrganizationID,
		PolicyID:   "pol-ram-limit",
		TargetType: model.TargetTypeNode,
		TargetID:   "node-compute-01",
		Enabled:    true,
		AssignedAt: now,
		AssignedBy: "admin",
	}

	createdAsgn, err := client.CreateAssignment(ctx, asgn)
	require.NoError(t, err)
	assert.Equal(t, "asgn-ram-node01", createdAsgn.ID)

	// List assignments
	asgns, err := client.ListAssignments(ctx, model.PolicyAssignmentFilter{PolicyID: "pol-ram-limit"})
	require.NoError(t, err)
	assert.Len(t, asgns, 1)
	assert.Equal(t, "node-compute-01", asgns[0].TargetID)
}

func TestE2E_Governance_MaintenanceAndSuppression(t *testing.T) {
	_, store, ts := setupTestServer(t)
	defer store.Close()
	defer ts.Close()

	client := governance.NewGovernanceClient(governance.ClientConfig{
		Endpoint: ts.URL,
	})
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)
	start := now.Add(-10 * time.Minute)
	end := now.Add(50 * time.Minute)

	mw := &model.MaintenanceWindow{
		ID:          "mw-database-upgrade",
		OrgID:       model.DefaultOrganizationID,
		Name:        "Database Upgrade Window",
		Status:      model.MaintenanceStatusActive,
		TargetScope: model.MaintenanceTargetScopeNode,
		TargetID:    "node-db-01",
		CreatedAt:   now,
		UpdatedAt:   now,
		Schedule: model.MaintenanceSchedule{
			StartTime: start,
			EndTime:   end,
			TimeZone:  "UTC",
		},
	}

	createdMw, err := client.CreateMaintenanceWindow(ctx, mw)
	require.NoError(t, err)
	assert.Equal(t, "mw-database-upgrade", createdMw.ID)

	// Evaluate suppression for an alert on that node during the active maintenance window
	suppReq := governance.SuppressionEvaluationRequest{
		OrgID:     model.DefaultOrganizationID,
		NodeID:    "node-db-01",
		AlertID:   "alert-high-cpu",
		Category:  "cpu",
		Severity:  model.SeverityWarning,
		Timestamp: now,
	}

	decision, err := client.EvaluateSuppression(ctx, suppReq)
	require.NoError(t, err)
	assert.Equal(t, model.SuppressionOutcomeSuppressed, decision.Outcome)
	assert.Equal(t, model.SuppressionReasonInMaintenanceWindow, decision.Reason)
	assert.Equal(t, "mw-database-upgrade", decision.WindowID)
}

func TestE2E_Governance_AuditTrail(t *testing.T) {
	_, store, ts := setupTestServer(t)
	defer store.Close()
	defer ts.Close()

	client := governance.NewGovernanceClient(governance.ClientConfig{
		Endpoint: ts.URL,
	})
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)
	event := model.AuditEvent{
		ID:        "audit-e2e-001",
		Timestamp: now,
		EventType: model.EventGovernanceOrgCreated,
		Severity:  model.AuditSeverityInfo,
		Outcome:   model.AuditOutcomeSuccess,
		Actor: model.AuditActor{
			Type:     model.ActorTypeCLI,
			Identity: "operator-admin",
		},
		Action:   "create_org",
		Resource: "org:org-finance-01",
		Message:  "Organization created successfully",
	}

	recorded, err := client.RecordAuditEvent(ctx, event)
	require.NoError(t, err)
	assert.Equal(t, "audit-e2e-001", recorded.ID)

	// Query audit events
	events, err := client.QueryAuditEvents(ctx, storage.AuditFilter{
		EventType: model.EventGovernanceOrgCreated,
	})
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, "audit-e2e-001", events[0].ID)
	assert.Equal(t, "operator-admin", events[0].Actor.Identity)
}
