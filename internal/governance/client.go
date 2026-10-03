package governance

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/internal/incidents"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

// ClientConfig holds configuration settings for the GovernanceClient.
type ClientConfig struct {
	Endpoint  string
	Token     string
	Timeout   time.Duration
	TLSConfig *tls.Config
}

// GovernanceClient provides a typed HTTP client for Watchdog's Governance REST API.
type GovernanceClient struct {
	endpoint   string
	token      string
	httpClient *http.Client
}

// NewGovernanceClient initializes a new GovernanceClient.
func NewGovernanceClient(cfg ClientConfig) *GovernanceClient {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	transport := &http.Transport{
		TLSClientConfig:     cfg.TLSConfig,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	return &GovernanceClient{
		endpoint: strings.TrimRight(cfg.Endpoint, "/"),
		token:    cfg.Token,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
	}
}

// Endpoint returns the configured base endpoint URL.
func (c *GovernanceClient) Endpoint() string {
	return c.endpoint
}

// ============================================================================
// Organization Management
// ============================================================================

// ListOrganizations queries all registered organizations.
func (c *GovernanceClient) ListOrganizations(ctx context.Context) ([]model.Organization, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/orgs", c.endpoint)
	var resp struct {
		Organizations []model.Organization `json:"organizations"`
		Count         int                  `json:"count"`
	}
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Organizations, nil
}

// CreateOrganization registers a new organization.
func (c *GovernanceClient) CreateOrganization(ctx context.Context, org *model.Organization) (*model.Organization, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/orgs", c.endpoint)
	var resp model.Organization
	if err := c.doJSON(ctx, http.MethodPost, reqURL, org, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetOrganization retrieves an organization by ID.
func (c *GovernanceClient) GetOrganization(ctx context.Context, id string) (*model.Organization, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/orgs/%s", c.endpoint, url.PathEscape(id))
	var resp model.Organization
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpdateOrganization updates an existing organization.
func (c *GovernanceClient) UpdateOrganization(ctx context.Context, id string, org *model.Organization) (*model.Organization, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/orgs/%s", c.endpoint, url.PathEscape(id))
	var resp model.Organization
	if err := c.doJSON(ctx, http.MethodPut, reqURL, org, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteOrganization removes an organization by ID.
func (c *GovernanceClient) DeleteOrganization(ctx context.Context, id string) error {
	reqURL := fmt.Sprintf("%s/api/v1/governance/orgs/%s", c.endpoint, url.PathEscape(id))
	return c.doJSON(ctx, http.MethodDelete, reqURL, nil, nil)
}

// ============================================================================
// Fleet Groups & Hierarchy
// ============================================================================

// ListFleetGroups retrieves fleet groups, optionally filtered by orgID.
func (c *GovernanceClient) ListFleetGroups(ctx context.Context, orgID string) ([]model.FleetGroup, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/groups", c.endpoint)
	if orgID != "" {
		reqURL += "?org_id=" + url.QueryEscape(orgID)
	}
	var resp struct {
		Groups []model.FleetGroup `json:"groups"`
		Count  int                `json:"count"`
	}
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Groups, nil
}

// CreateFleetGroup creates a new fleet group.
func (c *GovernanceClient) CreateFleetGroup(ctx context.Context, group *model.FleetGroup) (*model.FleetGroup, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/groups", c.endpoint)
	var resp model.FleetGroup
	if err := c.doJSON(ctx, http.MethodPost, reqURL, group, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetFleetGroup retrieves a fleet group by ID.
func (c *GovernanceClient) GetFleetGroup(ctx context.Context, id string) (*model.FleetGroup, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/groups/%s", c.endpoint, url.PathEscape(id))
	var resp model.FleetGroup
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpdateFleetGroup updates an existing fleet group.
func (c *GovernanceClient) UpdateFleetGroup(ctx context.Context, id string, group *model.FleetGroup) (*model.FleetGroup, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/groups/%s", c.endpoint, url.PathEscape(id))
	var resp model.FleetGroup
	if err := c.doJSON(ctx, http.MethodPut, reqURL, group, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteFleetGroup deletes a fleet group by ID.
func (c *GovernanceClient) DeleteFleetGroup(ctx context.Context, id string) error {
	reqURL := fmt.Sprintf("%s/api/v1/governance/groups/%s", c.endpoint, url.PathEscape(id))
	return c.doJSON(ctx, http.MethodDelete, reqURL, nil, nil)
}

// GetGroupMembers retrieves members of a fleet group.
func (c *GovernanceClient) GetGroupMembers(ctx context.Context, groupID string) ([]model.FleetGroupMember, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/groups/%s/members", c.endpoint, url.PathEscape(groupID))
	var resp struct {
		Members []model.FleetGroupMember `json:"members"`
		Count   int                      `json:"count"`
	}
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Members, nil
}

// AddMember adds a node member to a fleet group.
func (c *GovernanceClient) AddMember(ctx context.Context, groupID string, nodeID string) error {
	reqURL := fmt.Sprintf("%s/api/v1/governance/groups/%s/members", c.endpoint, url.PathEscape(groupID))
	payload := model.FleetGroupMember{
		GroupID: groupID,
		NodeID:  nodeID,
	}
	return c.doJSON(ctx, http.MethodPost, reqURL, payload, nil)
}

// AddGroupMember adds a node member to a fleet group.
func (c *GovernanceClient) AddGroupMember(ctx context.Context, groupID string, member *model.FleetGroupMember) error {
	reqURL := fmt.Sprintf("%s/api/v1/governance/groups/%s/members", c.endpoint, url.PathEscape(groupID))
	return c.doJSON(ctx, http.MethodPost, reqURL, member, nil)
}

// RemoveMember removes a node from a fleet group.
func (c *GovernanceClient) RemoveMember(ctx context.Context, groupID string, nodeID string) error {
	reqURL := fmt.Sprintf("%s/api/v1/governance/groups/%s/members/%s", c.endpoint, url.PathEscape(groupID), url.PathEscape(nodeID))
	return c.doJSON(ctx, http.MethodDelete, reqURL, nil, nil)
}

// RemoveGroupMember removes a node from a fleet group.
func (c *GovernanceClient) RemoveGroupMember(ctx context.Context, groupID string, nodeID string) error {
	return c.RemoveMember(ctx, groupID, nodeID)
}

// GetSubtreeNodes returns all node IDs in a group's entire hierarchy subtree.
func (c *GovernanceClient) GetSubtreeNodes(ctx context.Context, groupID string) ([]string, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/groups/%s/subtree", c.endpoint, url.PathEscape(groupID))
	var resp struct {
		Nodes   []string `json:"nodes"`
		NodeIDs []string `json:"node_ids"`
	}
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	if len(resp.Nodes) > 0 {
		return resp.Nodes, nil
	}
	return resp.NodeIDs, nil
}

// GetHierarchy retrieves the hierarchical tree of fleet groups.
func (c *GovernanceClient) GetHierarchy(ctx context.Context, orgID string) ([]*model.FleetGroupHierarchyNode, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/hierarchy", c.endpoint)
	if orgID != "" {
		reqURL += "?org_id=" + url.QueryEscape(orgID)
	}
	var resp struct {
		Hierarchy []*model.FleetGroupHierarchyNode `json:"hierarchy"`
		Count     int                              `json:"count"`
	}
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Hierarchy, nil
}

// ============================================================================
// Node Governance & Ownership
// ============================================================================

// GetNodeOwnership retrieves explicit node ownership metadata.
func (c *GovernanceClient) GetNodeOwnership(ctx context.Context, nodeID string) (*model.NodeOwnershipMetadata, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/nodes/%s/ownership", c.endpoint, url.PathEscape(nodeID))
	var resp model.NodeOwnershipMetadata
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SetNodeOwnership sets explicit ownership metadata for a node.
func (c *GovernanceClient) SetNodeOwnership(ctx context.Context, nodeID string, meta *model.NodeOwnershipMetadata) error {
	reqURL := fmt.Sprintf("%s/api/v1/governance/nodes/%s/ownership", c.endpoint, url.PathEscape(nodeID))
	return c.doJSON(ctx, http.MethodPut, reqURL, meta, nil)
}

// ResolveNodeOwnership resolves cascading ownership for a node.
func (c *GovernanceClient) ResolveNodeOwnership(ctx context.Context, nodeID string) (*model.ResolvedOwnership, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/nodes/%s/ownership/resolved", c.endpoint, url.PathEscape(nodeID))
	var resp model.ResolvedOwnership
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetNodeGroups retrieves all fleet groups a node belongs to.
func (c *GovernanceClient) GetNodeGroups(ctx context.Context, nodeID string) ([]model.FleetGroup, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/nodes/%s/groups", c.endpoint, url.PathEscape(nodeID))
	var resp struct {
		Groups []model.FleetGroup `json:"groups"`
		Count  int                `json:"count"`
	}
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Groups, nil
}

// ResolveNodePolicies resolves all effective policies assigned to a node.
func (c *GovernanceClient) ResolveNodePolicies(ctx context.Context, nodeID string) (*ResolvedPolicySet, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/nodes/%s/policies/resolved", c.endpoint, url.PathEscape(nodeID))
	var resp ResolvedPolicySet
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// EvaluateNodeCompliance evaluates node compliance without persisting findings.
func (c *GovernanceClient) EvaluateNodeCompliance(ctx context.Context, nodeID string) (*NodeComplianceReport, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/nodes/%s/compliance", c.endpoint, url.PathEscape(nodeID))
	var resp NodeComplianceReport
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ExplainResolution provides a step-by-step audit explanation of policy resolution for a node.
func (c *GovernanceClient) ExplainResolution(ctx context.Context, nodeID string) (*ResolutionExplanation, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/nodes/%s/resolution-explanation", c.endpoint, url.PathEscape(nodeID))
	var resp ResolutionExplanation
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ============================================================================
// Policy Lifecycle & Revisions
// ============================================================================

// ListPolicies queries policies matching the given filter.
func (c *GovernanceClient) ListPolicies(ctx context.Context, filter model.PolicyFilter) ([]model.Policy, error) {
	params := url.Values{}
	if filter.OrgID != "" {
		params.Set("org_id", filter.OrgID)
	}
	if filter.Status != "" {
		params.Set("status", string(filter.Status))
	}
	if filter.Category != "" {
		params.Set("category", string(filter.Category))
	}
	if filter.Search != "" {
		params.Set("search", filter.Search)
	}
	if filter.Limit > 0 {
		params.Set("limit", strconv.Itoa(filter.Limit))
	}
	if filter.Offset > 0 {
		params.Set("offset", strconv.Itoa(filter.Offset))
	}

	reqURL := fmt.Sprintf("%s/api/v1/governance/policies", c.endpoint)
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	var resp struct {
		Policies []model.Policy `json:"policies"`
		Count    int            `json:"count"`
	}
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Policies, nil
}

// CreatePolicy creates a new governance policy.
func (c *GovernanceClient) CreatePolicy(ctx context.Context, policy *model.Policy) (*model.Policy, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/policies", c.endpoint)
	var resp model.Policy
	if err := c.doJSON(ctx, http.MethodPost, reqURL, policy, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetPolicy retrieves a policy by ID.
func (c *GovernanceClient) GetPolicy(ctx context.Context, id string) (*model.Policy, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/policies/%s", c.endpoint, url.PathEscape(id))
	var resp model.Policy
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpdatePolicy updates a policy's metadata.
func (c *GovernanceClient) UpdatePolicy(ctx context.Context, id string, policy *model.Policy) (*model.Policy, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/policies/%s", c.endpoint, url.PathEscape(id))
	var resp model.Policy
	if err := c.doJSON(ctx, http.MethodPut, reqURL, policy, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeletePolicy deletes a policy by ID.
func (c *GovernanceClient) DeletePolicy(ctx context.Context, id string) error {
	reqURL := fmt.Sprintf("%s/api/v1/governance/policies/%s", c.endpoint, url.PathEscape(id))
	return c.doJSON(ctx, http.MethodDelete, reqURL, nil, nil)
}

// ListPolicyRevisions lists all revisions of a policy.
func (c *GovernanceClient) ListPolicyRevisions(ctx context.Context, policyID string) ([]model.PolicyRevision, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/policies/%s/revisions", c.endpoint, url.PathEscape(policyID))
	var resp struct {
		Revisions []model.PolicyRevision `json:"revisions"`
		Count     int                    `json:"count"`
	}
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Revisions, nil
}

// PublishRevision publishes an immutable revision of a policy.
func (c *GovernanceClient) PublishRevision(ctx context.Context, policyID string, rev *model.PolicyRevision) (*model.PolicyRevision, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/policies/%s/revisions", c.endpoint, url.PathEscape(policyID))
	var resp model.PolicyRevision
	if err := c.doJSON(ctx, http.MethodPost, reqURL, rev, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetPolicyRevision retrieves a specific policy revision.
func (c *GovernanceClient) GetPolicyRevision(ctx context.Context, policyID string, revision int) (*model.PolicyRevision, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/policies/%s/revisions/%d", c.endpoint, url.PathEscape(policyID), revision)
	var resp model.PolicyRevision
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SetPolicyStatus sets the lifecycle status of a policy (draft, active, disabled, archived).
func (c *GovernanceClient) SetPolicyStatus(ctx context.Context, policyID string, status model.PolicyStatus) error {
	reqURL := fmt.Sprintf("%s/api/v1/governance/policies/%s/status", c.endpoint, url.PathEscape(policyID))
	payload := map[string]string{"status": string(status)}
	return c.doJSON(ctx, http.MethodPut, reqURL, payload, nil)
}

// SetActiveRevision sets the active revision for a policy.
func (c *GovernanceClient) SetActiveRevision(ctx context.Context, policyID string, revision int) error {
	reqURL := fmt.Sprintf("%s/api/v1/governance/policies/%s/active-revision", c.endpoint, url.PathEscape(policyID))
	payload := map[string]int{"revision": revision}
	return c.doJSON(ctx, http.MethodPut, reqURL, payload, nil)
}

// DetectPolicyConflicts checks for overlapping or conflicting rules between two policy revisions.
func (c *GovernanceClient) DetectPolicyConflicts(ctx context.Context, policyIDA string, revA int, policyIDB string, revB int) ([]PolicyConflict, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/policies/conflicts", c.endpoint)
	payload := map[string]any{
		"policy_id_a": policyIDA,
		"rev_a":       revA,
		"policy_id_b": policyIDB,
		"rev_b":       revB,
	}
	var resp struct {
		Conflicts []PolicyConflict `json:"conflicts"`
		Count     int              `json:"count"`
	}
	if err := c.doJSON(ctx, http.MethodPost, reqURL, payload, &resp); err != nil {
		return nil, err
	}
	return resp.Conflicts, nil
}

// ============================================================================
// Policy Assignments
// ============================================================================

// ListAssignments queries policy assignments based on the filter.
func (c *GovernanceClient) ListAssignments(ctx context.Context, filter model.PolicyAssignmentFilter) ([]model.PolicyAssignment, error) {
	params := url.Values{}
	if filter.OrgID != "" {
		params.Set("org_id", filter.OrgID)
	}
	if filter.PolicyID != "" {
		params.Set("policy_id", filter.PolicyID)
	}
	if filter.TargetType != "" {
		params.Set("target_type", string(filter.TargetType))
	}
	if filter.TargetID != "" {
		params.Set("target_id", filter.TargetID)
	}
	if filter.EnabledOnly {
		params.Set("enabled", "true")
	}

	reqURL := fmt.Sprintf("%s/api/v1/governance/assignments", c.endpoint)
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	var resp struct {
		Assignments []model.PolicyAssignment `json:"assignments"`
		Count       int                      `json:"count"`
	}
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Assignments, nil
}

// CreateAssignment assigns a policy to an organization, fleet group, or node.
func (c *GovernanceClient) CreateAssignment(ctx context.Context, asgn *model.PolicyAssignment) (*model.PolicyAssignment, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/assignments", c.endpoint)
	var resp model.PolicyAssignment
	if err := c.doJSON(ctx, http.MethodPost, reqURL, asgn, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetAssignment retrieves a policy assignment by ID.
func (c *GovernanceClient) GetAssignment(ctx context.Context, id string) (*model.PolicyAssignment, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/assignments/%s", c.endpoint, url.PathEscape(id))
	var resp model.PolicyAssignment
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteAssignment deletes a policy assignment by ID.
func (c *GovernanceClient) DeleteAssignment(ctx context.Context, id string) error {
	reqURL := fmt.Sprintf("%s/api/v1/governance/assignments/%s", c.endpoint, url.PathEscape(id))
	return c.doJSON(ctx, http.MethodDelete, reqURL, nil, nil)
}

// SetAssignmentEnabled enables or disables a policy assignment.
func (c *GovernanceClient) SetAssignmentEnabled(ctx context.Context, id string, enabled bool) error {
	reqURL := fmt.Sprintf("%s/api/v1/governance/assignments/%s/enable", c.endpoint, url.PathEscape(id))
	payload := map[string]bool{"enabled": enabled}
	return c.doJSON(ctx, http.MethodPut, reqURL, payload, nil)
}

// ============================================================================
// Runtime Evaluations & Findings
// ============================================================================

// EvaluateNode executes a full policy evaluation run on a node.
func (c *GovernanceClient) EvaluateNode(ctx context.Context, nodeID string, trigger model.EvaluationTriggerType) (*model.EvaluationExecution, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/evaluations/node/%s", c.endpoint, url.PathEscape(nodeID))
	payload := map[string]string{"trigger": string(trigger)}
	var resp model.EvaluationExecution
	if err := c.doJSON(ctx, http.MethodPost, reqURL, payload, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// EvaluateFleetGroup executes policy evaluation across all nodes in a fleet group.
func (c *GovernanceClient) EvaluateFleetGroup(ctx context.Context, groupID string, orgID string, includeSubgroups bool, trigger model.EvaluationTriggerType) ([]*model.EvaluationExecution, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/evaluations/group/%s", c.endpoint, url.PathEscape(groupID))
	payload := map[string]any{
		"org_id":            orgID,
		"include_subgroups": includeSubgroups,
		"trigger":           string(trigger),
	}
	var resp struct {
		GroupID     string                     `json:"group_id"`
		Evaluations []*model.EvaluationExecution `json:"evaluations"`
		Count       int                        `json:"count"`
	}
	if err := c.doJSON(ctx, http.MethodPost, reqURL, payload, &resp); err != nil {
		return nil, err
	}
	return resp.Evaluations, nil
}

// EvaluateOrganization executes policy evaluation across all nodes in an entire organization.
func (c *GovernanceClient) EvaluateOrganization(ctx context.Context, orgID string, trigger model.EvaluationTriggerType) ([]*model.EvaluationExecution, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/evaluations/org/%s", c.endpoint, url.PathEscape(orgID))
	payload := map[string]string{"trigger": string(trigger)}
	var resp struct {
		OrgID       string                     `json:"org_id"`
		Evaluations []*model.EvaluationExecution `json:"evaluations"`
		Count       int                        `json:"count"`
	}
	if err := c.doJSON(ctx, http.MethodPost, reqURL, payload, &resp); err != nil {
		return nil, err
	}
	return resp.Evaluations, nil
}

// ListEvaluationExecutions queries evaluation history matching the filter.
func (c *GovernanceClient) ListEvaluationExecutions(ctx context.Context, filter model.EvaluationFilter) ([]model.EvaluationExecution, error) {
	params := url.Values{}
	if filter.OrgID != "" {
		params.Set("org_id", filter.OrgID)
	}
	if filter.TargetNodeID != "" {
		params.Set("target_node_id", filter.TargetNodeID)
	}
	if filter.TriggerType != "" {
		params.Set("trigger", string(filter.TriggerType))
	}
	if filter.Status != "" {
		params.Set("status", string(filter.Status))
	}
	if !filter.Since.IsZero() {
		params.Set("since", filter.Since.Format(time.RFC3339))
	}
	if !filter.Until.IsZero() {
		params.Set("until", filter.Until.Format(time.RFC3339))
	}
	if filter.Limit > 0 {
		params.Set("limit", strconv.Itoa(filter.Limit))
	}
	if filter.Offset > 0 {
		params.Set("offset", strconv.Itoa(filter.Offset))
	}

	reqURL := fmt.Sprintf("%s/api/v1/governance/evaluations", c.endpoint)
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	var resp struct {
		Evaluations []model.EvaluationExecution `json:"evaluations"`
		Count       int                         `json:"count"`
	}
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Evaluations, nil
}

// GetEvaluationExecution retrieves an evaluation execution record by ID.
func (c *GovernanceClient) GetEvaluationExecution(ctx context.Context, id string) (*model.EvaluationExecution, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/evaluations/%s", c.endpoint, url.PathEscape(id))
	var resp model.EvaluationExecution
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetLatestNodeEvaluation retrieves the most recent evaluation execution for a node.
func (c *GovernanceClient) GetLatestNodeEvaluation(ctx context.Context, nodeID string, orgID string) (*model.EvaluationExecution, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/evaluations/latest/node/%s", c.endpoint, url.PathEscape(nodeID))
	if orgID != "" {
		reqURL += "?org_id=" + url.QueryEscape(orgID)
	}
	var resp model.EvaluationExecution
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListComplianceFindings queries compliance findings based on the filter.
func (c *GovernanceClient) ListComplianceFindings(ctx context.Context, filter model.FindingFilter) ([]model.ComplianceFinding, error) {
	params := url.Values{}
	if filter.OrgID != "" {
		params.Set("org_id", filter.OrgID)
	}
	if filter.TargetNodeID != "" {
		params.Set("target_node_id", filter.TargetNodeID)
	}
	if filter.PolicyID != "" {
		params.Set("policy_id", filter.PolicyID)
	}
	if filter.Status != "" {
		params.Set("status", string(filter.Status))
	}
	if filter.Severity != "" {
		params.Set("severity", string(filter.Severity))
	}
	if filter.Category != "" {
		params.Set("category", string(filter.Category))
	}
	if filter.Limit > 0 {
		params.Set("limit", strconv.Itoa(filter.Limit))
	}

	reqURL := fmt.Sprintf("%s/api/v1/governance/findings", c.endpoint)
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	var resp struct {
		Findings []model.ComplianceFinding `json:"findings"`
		Count    int                       `json:"count"`
	}
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Findings, nil
}

// GetComplianceFinding retrieves a compliance finding by ID.
func (c *GovernanceClient) GetComplianceFinding(ctx context.Context, id string) (*model.ComplianceFinding, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/findings/%s", c.endpoint, url.PathEscape(id))
	var resp model.ComplianceFinding
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ============================================================================
// Compliance Summaries
// ============================================================================

// GetNodeComplianceSummary retrieves aggregated compliance rollup for a node.
func (c *GovernanceClient) GetNodeComplianceSummary(ctx context.Context, nodeID string, orgID string) (*model.ComplianceRollupSummary, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/compliance/node/%s", c.endpoint, url.PathEscape(nodeID))
	if orgID != "" {
		reqURL += "?org_id=" + url.QueryEscape(orgID)
	}
	var resp model.ComplianceRollupSummary
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetGroupComplianceSummary retrieves aggregated compliance rollup for a fleet group.
func (c *GovernanceClient) GetGroupComplianceSummary(ctx context.Context, groupID string, orgID string, includeSubgroups bool) (*model.ComplianceRollupSummary, error) {
	params := url.Values{}
	if orgID != "" {
		params.Set("org_id", orgID)
	}
	if includeSubgroups {
		params.Set("include_subgroups", "true")
	}

	reqURL := fmt.Sprintf("%s/api/v1/governance/compliance/group/%s", c.endpoint, url.PathEscape(groupID))
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	var resp model.ComplianceRollupSummary
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetOrgComplianceSummary retrieves aggregated compliance rollup for an entire organization.
func (c *GovernanceClient) GetOrgComplianceSummary(ctx context.Context, orgID string) (*model.ComplianceRollupSummary, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/compliance/org/%s", c.endpoint, url.PathEscape(orgID))
	var resp model.ComplianceRollupSummary
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ============================================================================
// Ephemeral Policy Simulation
// ============================================================================

// SimulatePolicyChanges runs isolated, ephemeral what-if policy evaluations with zero side effects.
func (c *GovernanceClient) SimulatePolicyChanges(ctx context.Context, req *model.SimulationRequest) (*model.SimulationResult, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/simulation", c.endpoint)
	var resp model.SimulationResult
	if err := c.doJSON(ctx, http.MethodPost, reqURL, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ============================================================================
// Maintenance Windows
// ============================================================================

// ListMaintenanceWindows queries maintenance windows based on the filter.
func (c *GovernanceClient) ListMaintenanceWindows(ctx context.Context, filter model.MaintenanceWindowFilter) ([]model.MaintenanceWindow, error) {
	params := url.Values{}
	if filter.OrgID != "" {
		params.Set("org_id", filter.OrgID)
	}
	if filter.Status != "" {
		params.Set("status", string(filter.Status))
	}
	if filter.TargetScope != "" {
		params.Set("target_scope", string(filter.TargetScope))
	}
	if filter.TargetID != "" {
		params.Set("target_id", filter.TargetID)
	}
	if filter.ActiveAt != nil {
		params.Set("active_at", filter.ActiveAt.Format(time.RFC3339))
	}
	if filter.From != nil {
		params.Set("from", filter.From.Format(time.RFC3339))
	}
	if filter.To != nil {
		params.Set("to", filter.To.Format(time.RFC3339))
	}
	if filter.IncludeExpired {
		params.Set("include_expired", "true")
	}
	if filter.Limit > 0 {
		params.Set("limit", strconv.Itoa(filter.Limit))
	}
	if filter.Offset > 0 {
		params.Set("offset", strconv.Itoa(filter.Offset))
	}

	reqURL := fmt.Sprintf("%s/api/v1/governance/maintenance", c.endpoint)
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	var resp struct {
		MaintenanceWindows []model.MaintenanceWindow `json:"maintenance_windows"`
		Count              int                      `json:"count"`
	}
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return resp.MaintenanceWindows, nil
}

// CreateMaintenanceWindow schedules a new maintenance window.
func (c *GovernanceClient) CreateMaintenanceWindow(ctx context.Context, window *model.MaintenanceWindow) (*model.MaintenanceWindow, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/maintenance", c.endpoint)
	var resp model.MaintenanceWindow
	if err := c.doJSON(ctx, http.MethodPost, reqURL, window, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetMaintenanceWindow retrieves a maintenance window by ID.
func (c *GovernanceClient) GetMaintenanceWindow(ctx context.Context, id string) (*model.MaintenanceWindow, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/maintenance/%s", c.endpoint, url.PathEscape(id))
	var resp model.MaintenanceWindow
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpdateMaintenanceWindow updates an existing maintenance window.
func (c *GovernanceClient) UpdateMaintenanceWindow(ctx context.Context, id string, window *model.MaintenanceWindow) (*model.MaintenanceWindow, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/maintenance/%s", c.endpoint, url.PathEscape(id))
	var resp model.MaintenanceWindow
	if err := c.doJSON(ctx, http.MethodPut, reqURL, window, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteMaintenanceWindow deletes a maintenance window by ID.
func (c *GovernanceClient) DeleteMaintenanceWindow(ctx context.Context, id string) error {
	reqURL := fmt.Sprintf("%s/api/v1/governance/maintenance/%s", c.endpoint, url.PathEscape(id))
	return c.doJSON(ctx, http.MethodDelete, reqURL, nil, nil)
}

// CancelMaintenanceWindow cancels an active or scheduled maintenance window.
func (c *GovernanceClient) CancelMaintenanceWindow(ctx context.Context, id string, reason string) error {
	reqURL := fmt.Sprintf("%s/api/v1/governance/maintenance/%s/cancel", c.endpoint, url.PathEscape(id))
	payload := map[string]string{"reason": reason}
	return c.doJSON(ctx, http.MethodPost, reqURL, payload, nil)
}

// EvaluateMaintenanceWindows checks active maintenance windows in an organization at a given timestamp.
func (c *GovernanceClient) EvaluateMaintenanceWindows(ctx context.Context, orgID string, evalTime time.Time) ([]model.MaintenanceWindow, error) {
	params := url.Values{}
	if orgID != "" {
		params.Set("org_id", orgID)
	}
	if !evalTime.IsZero() {
		params.Set("time", evalTime.Format(time.RFC3339))
	}

	reqURL := fmt.Sprintf("%s/api/v1/governance/maintenance/evaluate", c.endpoint)
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	var resp struct {
		OrgID         string                   `json:"org_id"`
		ActiveWindows []model.MaintenanceWindow `json:"active_windows"`
		Count         int                      `json:"count"`
		EvalTime      time.Time                `json:"eval_time"`
	}
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return resp.ActiveWindows, nil
}

// ============================================================================
// Escalation Policies
// ============================================================================

// ListEscalationPolicies queries escalation policies matching the filter.
func (c *GovernanceClient) ListEscalationPolicies(ctx context.Context, filter model.EscalationPolicyFilter) ([]model.EscalationPolicy, error) {
	params := url.Values{}
	if filter.OrgID != "" {
		params.Set("org_id", filter.OrgID)
	}
	if filter.Severity != "" {
		params.Set("severity", string(filter.Severity))
	}
	if filter.Enabled != nil {
		params.Set("enabled", strconv.FormatBool(*filter.Enabled))
	}
	if filter.Limit > 0 {
		params.Set("limit", strconv.Itoa(filter.Limit))
	}
	if filter.Offset > 0 {
		params.Set("offset", strconv.Itoa(filter.Offset))
	}

	reqURL := fmt.Sprintf("%s/api/v1/governance/escalation", c.endpoint)
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	var resp struct {
		EscalationPolicies []model.EscalationPolicy `json:"escalation_policies"`
		Count              int                      `json:"count"`
	}
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return resp.EscalationPolicies, nil
}

// CreateEscalationPolicy creates a new incident escalation policy.
func (c *GovernanceClient) CreateEscalationPolicy(ctx context.Context, policy *model.EscalationPolicy) (*model.EscalationPolicy, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/escalation", c.endpoint)
	var resp model.EscalationPolicy
	if err := c.doJSON(ctx, http.MethodPost, reqURL, policy, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetEscalationPolicy retrieves an escalation policy by ID.
func (c *GovernanceClient) GetEscalationPolicy(ctx context.Context, id string) (*model.EscalationPolicy, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/escalation/%s", c.endpoint, url.PathEscape(id))
	var resp model.EscalationPolicy
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpdateEscalationPolicy updates an existing escalation policy.
func (c *GovernanceClient) UpdateEscalationPolicy(ctx context.Context, id string, policy *model.EscalationPolicy) (*model.EscalationPolicy, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/escalation/%s", c.endpoint, url.PathEscape(id))
	var resp model.EscalationPolicy
	if err := c.doJSON(ctx, http.MethodPut, reqURL, policy, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteEscalationPolicy deletes an escalation policy by ID.
func (c *GovernanceClient) DeleteEscalationPolicy(ctx context.Context, id string) error {
	reqURL := fmt.Sprintf("%s/api/v1/governance/escalation/%s", c.endpoint, url.PathEscape(id))
	return c.doJSON(ctx, http.MethodDelete, reqURL, nil, nil)
}

// EvaluateIncidentEscalation evaluates incident escalation stage progression.
func (c *GovernanceClient) EvaluateIncidentEscalation(ctx context.Context, orgID string, incident *incidents.Incident, evalTime time.Time) (*model.EscalationEvaluationResult, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/escalation/evaluate", c.endpoint)
	payload := map[string]any{
		"org_id":    orgID,
		"incident":  incident,
		"eval_time": evalTime,
	}
	var resp model.EscalationEvaluationResult
	if err := c.doJSON(ctx, http.MethodPost, reqURL, payload, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ============================================================================
// Alert Suppression
// ============================================================================

// EvaluateSuppression evaluates whether an alert should be suppressed by active maintenance windows.
func (c *GovernanceClient) EvaluateSuppression(ctx context.Context, req SuppressionEvaluationRequest) (*model.SuppressionDecision, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/suppression/evaluate", c.endpoint)
	var resp model.SuppressionDecision
	if err := c.doJSON(ctx, http.MethodPost, reqURL, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListSuppressionDecisions queries historical suppression decisions matching the filter.
func (c *GovernanceClient) ListSuppressionDecisions(ctx context.Context, filter model.SuppressionFilter) ([]model.SuppressionDecision, error) {
	params := url.Values{}
	if filter.OrgID != "" {
		params.Set("org_id", filter.OrgID)
	}
	if filter.Outcome != "" {
		params.Set("outcome", string(filter.Outcome))
	}
	if filter.WindowID != "" {
		params.Set("window_id", filter.WindowID)
	}
	if filter.NodeID != "" {
		params.Set("node_id", filter.NodeID)
	}
	if filter.AlertID != "" {
		params.Set("alert_id", filter.AlertID)
	}
	if filter.Limit > 0 {
		params.Set("limit", strconv.Itoa(filter.Limit))
	}

	reqURL := fmt.Sprintf("%s/api/v1/governance/suppression", c.endpoint)
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	var resp struct {
		SuppressionDecisions []model.SuppressionDecision `json:"suppression_decisions"`
		Count                int                         `json:"count"`
	}
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return resp.SuppressionDecisions, nil
}

// GetSuppressionDecision retrieves a specific suppression decision by ID.
func (c *GovernanceClient) GetSuppressionDecision(ctx context.Context, id string) (*model.SuppressionDecision, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/suppression/%s", c.endpoint, url.PathEscape(id))
	var resp model.SuppressionDecision
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ============================================================================
// Audit Trail
// ============================================================================

// QueryAuditEvents queries sanitized governance audit logs matching the filter.
func (c *GovernanceClient) QueryAuditEvents(ctx context.Context, filter storage.AuditFilter) ([]model.AuditEvent, error) {
	params := url.Values{}
	if filter.EventType != "" {
		params.Set("event_type", filter.EventType)
	}
	if filter.Severity != "" {
		params.Set("severity", filter.Severity)
	}
	if filter.Outcome != "" {
		params.Set("outcome", filter.Outcome)
	}
	if filter.ActorType != "" {
		params.Set("actor_type", filter.ActorType)
	}
	if filter.ActorIdentity != "" {
		params.Set("actor_identity", filter.ActorIdentity)
	}
	if filter.SourceAddress != "" {
		params.Set("source_address", filter.SourceAddress)
	}
	if filter.RequestID != "" {
		params.Set("request_id", filter.RequestID)
	}
	if !filter.StartTime.IsZero() {
		params.Set("start_time", filter.StartTime.Format(time.RFC3339))
	}
	if !filter.EndTime.IsZero() {
		params.Set("end_time", filter.EndTime.Format(time.RFC3339))
	}
	if filter.Limit > 0 {
		params.Set("limit", strconv.Itoa(filter.Limit))
	}
	if filter.Offset > 0 {
		params.Set("offset", strconv.Itoa(filter.Offset))
	}

	reqURL := fmt.Sprintf("%s/api/v1/governance/audit", c.endpoint)
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	var resp struct {
		AuditEvents []model.AuditEvent `json:"audit_events"`
		Count       int                `json:"count"`
	}
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &resp); err != nil {
		return nil, err
	}
	return resp.AuditEvents, nil
}

// RecordAuditEvent writes a new sanitized audit event.
func (c *GovernanceClient) RecordAuditEvent(ctx context.Context, event model.AuditEvent) (*model.AuditEvent, error) {
	reqURL := fmt.Sprintf("%s/api/v1/governance/audit", c.endpoint)
	var resp model.AuditEvent
	if err := c.doJSON(ctx, http.MethodPost, reqURL, event, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ============================================================================
// Internal HTTP Helpers
// ============================================================================

func (c *GovernanceClient) doJSON(ctx context.Context, method, reqURL string, payload any, result any) error {
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal request payload: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr model.APIErrorResponse
		if jsonErr := json.Unmarshal(respBody, &apiErr); jsonErr == nil && apiErr.Error.Message != "" {
			return fmt.Errorf("server error (%d %s): %s", resp.StatusCode, apiErr.Error.Code, apiErr.Error.Message)
		}
		return fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to decode response payload: %w", err)
		}
	}

	return nil
}
