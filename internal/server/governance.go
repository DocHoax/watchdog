package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/internal/governance"
	"github.com/DocHoax/watchdog/internal/incidents"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

// handleGovernanceRoute multiplexes /api/v1/governance and /api/v1/governance/* requests.
func (s *Server) handleGovernanceRoute(w http.ResponseWriter, r *http.Request) {
	govSvc := s.GovernanceService()
	if govSvc == nil {
		s.writeAPIError(w, r, http.StatusServiceUnavailable, "GOVERNANCE_SERVICE_UNAVAILABLE", "Governance service is not configured")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v1/governance")
	path = strings.TrimPrefix(path, "/")

	if path == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_GOVERNANCE_PATH", "Governance sub-resource required")
		return
	}

	parts := strings.Split(path, "/")
	resource := parts[0]

	switch resource {
	case "orgs":
		s.handleGovernanceOrgs(w, r, govSvc, parts[1:])
	case "groups":
		s.handleGovernanceGroups(w, r, govSvc, parts[1:])
	case "hierarchy":
		s.handleGovernanceHierarchy(w, r, govSvc)
	case "nodes":
		s.handleGovernanceNodes(w, r, govSvc, parts[1:])
	case "policies":
		s.handleGovernancePolicies(w, r, govSvc, parts[1:])
	case "assignments":
		s.handleGovernanceAssignments(w, r, govSvc, parts[1:])
	case "evaluations":
		s.handleGovernanceEvaluations(w, r, govSvc, parts[1:])
	case "findings":
		s.handleGovernanceFindings(w, r, govSvc, parts[1:])
	case "compliance":
		s.handleGovernanceCompliance(w, r, govSvc, parts[1:])
	case "simulation":
		s.handleGovernanceSimulation(w, r, govSvc)
	case "maintenance":
		s.handleGovernanceMaintenance(w, r, govSvc, parts[1:])
	case "escalation":
		s.handleGovernanceEscalation(w, r, govSvc, parts[1:])
	case "suppression":
		s.handleGovernanceSuppression(w, r, govSvc, parts[1:])
	case "audit":
		s.handleGovernanceAudit(w, r, govSvc)
	default:
		s.writeAPIError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", fmt.Sprintf("Governance resource '%s' not found", resource))
	}
}

// ---------------------------------------------------------------------------
// 1. Organizations
// ---------------------------------------------------------------------------

func (s *Server) handleGovernanceOrgs(w http.ResponseWriter, r *http.Request, svc governance.GovernanceService, parts []string) {
	if len(parts) == 0 {
		switch r.Method {
		case http.MethodGet:
			orgs, err := svc.ListOrganizations(r.Context())
			if err != nil {
				s.writeGovError(w, r, err, "LIST_ORGS_FAILED")
				return
			}
			if orgs == nil {
				orgs = []model.Organization{}
			}
			s.writeJSON(w, http.StatusOK, map[string]any{"organizations": orgs, "count": len(orgs)})
		case http.MethodPost:
			var org model.Organization
			if err := json.NewDecoder(r.Body).Decode(&org); err != nil {
				s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Failed to parse organization JSON")
				return
			}
			if err := svc.CreateOrganization(r.Context(), &org); err != nil {
				s.writeGovError(w, r, err, "CREATE_ORG_FAILED")
				return
			}
			s.writeJSON(w, http.StatusCreated, org)
		default:
			w.Header().Set("Allow", "GET, POST")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		}
		return
	}

	id := parts[0]
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			org, err := svc.GetOrganization(r.Context(), id)
			if err != nil {
				s.writeGovError(w, r, err, "GET_ORG_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, org)
		case http.MethodPut:
			var org model.Organization
			if err := json.NewDecoder(r.Body).Decode(&org); err != nil {
				s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Failed to parse organization JSON")
				return
			}
			org.ID = id
			if err := svc.UpdateOrganization(r.Context(), &org); err != nil {
				s.writeGovError(w, r, err, "UPDATE_ORG_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, org)
		case http.MethodDelete:
			if err := svc.DeleteOrganization(r.Context(), id); err != nil {
				s.writeGovError(w, r, err, "DELETE_ORG_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "organization_id": id})
		default:
			w.Header().Set("Allow", "GET, PUT, DELETE")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		}
		return
	}

	s.writeAPIError(w, r, http.StatusNotFound, "NOT_FOUND", "Endpoint not found")
}

// ---------------------------------------------------------------------------
// 2. Fleet Groups & Hierarchy
// ---------------------------------------------------------------------------

func (s *Server) handleGovernanceGroups(w http.ResponseWriter, r *http.Request, svc governance.GovernanceService, parts []string) {
	if len(parts) == 0 {
		switch r.Method {
		case http.MethodGet:
			orgID := r.URL.Query().Get("org_id")
			if orgID == "" {
				orgID = model.DefaultOrganizationID
			}
			groups, err := svc.ListFleetGroups(r.Context(), orgID)
			if err != nil {
				s.writeGovError(w, r, err, "LIST_GROUPS_FAILED")
				return
			}
			if groups == nil {
				groups = []model.FleetGroup{}
			}
			s.writeJSON(w, http.StatusOK, map[string]any{"groups": groups, "count": len(groups), "org_id": orgID})
		case http.MethodPost:
			var group model.FleetGroup
			if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
				s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Failed to parse fleet group JSON")
				return
			}
			if group.OrgID == "" {
				group.OrgID = model.DefaultOrganizationID
			}
			if err := svc.CreateFleetGroup(r.Context(), &group); err != nil {
				s.writeGovError(w, r, err, "CREATE_GROUP_FAILED")
				return
			}
			s.writeJSON(w, http.StatusCreated, group)
		default:
			w.Header().Set("Allow", "GET, POST")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		}
		return
	}

	id := parts[0]
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			group, err := svc.GetFleetGroup(r.Context(), id)
			if err != nil {
				s.writeGovError(w, r, err, "GET_GROUP_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, group)
		case http.MethodPut:
			var group model.FleetGroup
			if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
				s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Failed to parse fleet group JSON")
				return
			}
			group.ID = id
			if err := svc.UpdateFleetGroup(r.Context(), &group); err != nil {
				s.writeGovError(w, r, err, "UPDATE_GROUP_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, group)
		case http.MethodDelete:
			if err := svc.DeleteFleetGroup(r.Context(), id); err != nil {
				s.writeGovError(w, r, err, "DELETE_GROUP_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "group_id": id})
		default:
			w.Header().Set("Allow", "GET, PUT, DELETE")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		}
		return
	}

	sub := parts[1]
	switch sub {
	case "members":
		if len(parts) == 2 {
			switch r.Method {
			case http.MethodGet:
				members, err := svc.GetGroupMembers(r.Context(), id)
				if err != nil {
					s.writeGovError(w, r, err, "GET_GROUP_MEMBERS_FAILED")
					return
				}
				if members == nil {
					members = []model.FleetGroupMember{}
				}
				s.writeJSON(w, http.StatusOK, map[string]any{"group_id": id, "members": members, "count": len(members)})
			case http.MethodPost:
				var member model.FleetGroupMember
				if err := json.NewDecoder(r.Body).Decode(&member); err != nil {
					s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Failed to parse member JSON")
					return
				}
				member.GroupID = id
				if err := svc.AddMember(r.Context(), &member); err != nil {
					s.writeGovError(w, r, err, "ADD_MEMBER_FAILED")
					return
				}
				s.writeJSON(w, http.StatusCreated, member)
			case http.MethodPut:
				var req struct {
					NodeIDs []string `json:"node_ids"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Failed to parse node_ids JSON")
					return
				}
				if err := svc.SetMembers(r.Context(), id, req.NodeIDs); err != nil {
					s.writeGovError(w, r, err, "SET_MEMBERS_FAILED")
					return
				}
				s.writeJSON(w, http.StatusOK, map[string]any{"group_id": id, "node_ids": req.NodeIDs})
			default:
				w.Header().Set("Allow", "GET, POST, PUT")
				s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
			}
			return
		}
		if len(parts) == 3 && r.Method == http.MethodDelete {
			nodeID := parts[2]
			if err := svc.RemoveMember(r.Context(), id, nodeID); err != nil {
				s.writeGovError(w, r, err, "REMOVE_MEMBER_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, map[string]any{"removed": true, "group_id": id, "node_id": nodeID})
			return
		}
	case "subtree":
		if r.Method == http.MethodGet {
			nodes, err := svc.GetSubtreeNodes(r.Context(), id)
			if err != nil {
				s.writeGovError(w, r, err, "GET_SUBTREE_FAILED")
				return
			}
			if nodes == nil {
				nodes = []string{}
			}
			s.writeJSON(w, http.StatusOK, map[string]any{"group_id": id, "nodes": nodes, "count": len(nodes)})
			return
		}
	}

	s.writeAPIError(w, r, http.StatusNotFound, "NOT_FOUND", "Endpoint not found")
}

func (s *Server) handleGovernanceHierarchy(w http.ResponseWriter, r *http.Request, svc governance.GovernanceService) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}
	orgID := r.URL.Query().Get("org_id")
	if orgID == "" {
		orgID = model.DefaultOrganizationID
	}
	roots, err := svc.GetHierarchy(r.Context(), orgID)
	if err != nil {
		s.writeGovError(w, r, err, "GET_HIERARCHY_FAILED")
		return
	}
	if roots == nil {
		roots = []*model.FleetGroupHierarchyNode{}
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"org_id": orgID, "hierarchy": roots, "count": len(roots)})
}

// ---------------------------------------------------------------------------
// 3. Node Governance & Ownership
// ---------------------------------------------------------------------------

func (s *Server) handleGovernanceNodes(w http.ResponseWriter, r *http.Request, svc governance.GovernanceService, parts []string) {
	if len(parts) < 2 {
		s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_NODE_ROUTE", "Node ID and sub-resource required")
		return
	}
	nodeID := parts[0]
	sub := parts[1]

	switch sub {
	case "ownership":
		if len(parts) == 2 {
			switch r.Method {
			case http.MethodGet:
				own, err := svc.GetNodeOwnership(r.Context(), nodeID)
				if err != nil {
					s.writeGovError(w, r, err, "GET_OWNERSHIP_FAILED")
					return
				}
				s.writeJSON(w, http.StatusOK, own)
			case http.MethodPut:
				var own model.NodeOwnershipMetadata
				if err := json.NewDecoder(r.Body).Decode(&own); err != nil {
					s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Failed to parse ownership metadata JSON")
					return
				}
				if err := svc.SetNodeOwnership(r.Context(), nodeID, &own); err != nil {
					s.writeGovError(w, r, err, "SET_OWNERSHIP_FAILED")
					return
				}
				s.writeJSON(w, http.StatusOK, own)
			default:
				w.Header().Set("Allow", "GET, PUT")
				s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
			}
			return
		}
		if len(parts) == 3 && parts[2] == "resolved" && r.Method == http.MethodGet {
			resolved, err := svc.ResolveNodeOwnership(r.Context(), nodeID)
			if err != nil {
				s.writeGovError(w, r, err, "RESOLVE_OWNERSHIP_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, resolved)
			return
		}
	case "groups":
		if r.Method == http.MethodGet {
			groups, err := svc.GetNodeGroups(r.Context(), nodeID)
			if err != nil {
				s.writeGovError(w, r, err, "GET_NODE_GROUPS_FAILED")
				return
			}
			if groups == nil {
				groups = []model.FleetGroup{}
			}
			s.writeJSON(w, http.StatusOK, map[string]any{"node_id": nodeID, "groups": groups, "count": len(groups)})
			return
		}
	case "policies":
		if len(parts) == 3 && parts[2] == "resolved" && r.Method == http.MethodGet {
			resolved, err := svc.ResolveNodePolicies(r.Context(), nodeID)
			if err != nil {
				s.writeGovError(w, r, err, "RESOLVE_POLICIES_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, resolved)
			return
		}
	case "compliance":
		if r.Method == http.MethodGet {
			report, err := svc.EvaluateNodeCompliance(r.Context(), nodeID)
			if err != nil {
				s.writeGovError(w, r, err, "EVALUATE_COMPLIANCE_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, report)
			return
		}
	case "resolution-explanation":
		if r.Method == http.MethodGet {
			explanation, err := svc.ExplainResolution(r.Context(), nodeID)
			if err != nil {
				s.writeGovError(w, r, err, "EXPLAIN_RESOLUTION_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, explanation)
			return
		}
	}

	s.writeAPIError(w, r, http.StatusNotFound, "NOT_FOUND", "Endpoint not found")
}

// ---------------------------------------------------------------------------
// 4. Policies & Revisions
// ---------------------------------------------------------------------------

func (s *Server) handleGovernancePolicies(w http.ResponseWriter, r *http.Request, svc governance.GovernanceService, parts []string) {
	if len(parts) == 0 {
		switch r.Method {
		case http.MethodGet:
			q := r.URL.Query()
			filter := model.PolicyFilter{
				OrgID:    q.Get("org_id"),
				Category: model.PolicyCategory(q.Get("category")),
				Status:   model.PolicyStatus(q.Get("status")),
				Search:   q.Get("search"),
			}
			policies, err := svc.ListPolicies(r.Context(), filter)
			if err != nil {
				s.writeGovError(w, r, err, "LIST_POLICIES_FAILED")
				return
			}
			if policies == nil {
				policies = []model.Policy{}
			}
			s.writeJSON(w, http.StatusOK, map[string]any{"policies": policies, "count": len(policies)})
		case http.MethodPost:
			var policy model.Policy
			if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
				s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Failed to parse policy JSON")
				return
			}
			if policy.OrgID == "" {
				policy.OrgID = model.DefaultOrganizationID
			}
			if err := svc.CreatePolicy(r.Context(), &policy); err != nil {
				s.writeGovError(w, r, err, "CREATE_POLICY_FAILED")
				return
			}
			s.writeJSON(w, http.StatusCreated, policy)
		default:
			w.Header().Set("Allow", "GET, POST")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		}
		return
	}

	// Conflict detection endpoint: POST /api/v1/governance/policies/conflicts
	if parts[0] == "conflicts" && r.Method == http.MethodPost {
		var req struct {
			PolicyIDA string `json:"policy_id_a"`
			RevA      int    `json:"rev_a"`
			PolicyIDB string `json:"policy_id_b"`
			RevB      int    `json:"rev_b"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Failed to parse conflict check JSON")
			return
		}
		conflicts, err := svc.DetectPolicyConflicts(r.Context(), req.PolicyIDA, req.RevA, req.PolicyIDB, req.RevB)
		if err != nil {
			s.writeGovError(w, r, err, "DETECT_CONFLICTS_FAILED")
			return
		}
		if conflicts == nil {
			conflicts = []governance.PolicyConflict{}
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"conflicts": conflicts, "count": len(conflicts)})
		return
	}

	id := parts[0]
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			policy, err := svc.GetPolicy(r.Context(), id)
			if err != nil {
				s.writeGovError(w, r, err, "GET_POLICY_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, policy)
		case http.MethodPut:
			var policy model.Policy
			if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
				s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Failed to parse policy JSON")
				return
			}
			policy.ID = id
			if err := svc.UpdatePolicy(r.Context(), &policy); err != nil {
				s.writeGovError(w, r, err, "UPDATE_POLICY_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, policy)
		case http.MethodDelete:
			if err := svc.DeletePolicy(r.Context(), id); err != nil {
				s.writeGovError(w, r, err, "DELETE_POLICY_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "policy_id": id})
		default:
			w.Header().Set("Allow", "GET, PUT, DELETE")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		}
		return
	}

	sub := parts[1]
	switch sub {
	case "status":
		if r.Method == http.MethodPut {
			var req struct {
				Status model.PolicyStatus `json:"status"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Failed to parse status JSON")
				return
			}
			if err := svc.SetPolicyStatus(r.Context(), id, req.Status); err != nil {
				s.writeGovError(w, r, err, "SET_STATUS_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, map[string]any{"policy_id": id, "status": req.Status})
			return
		}
	case "active-revision":
		if r.Method == http.MethodPut {
			var req struct {
				Revision int `json:"revision"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Failed to parse active revision JSON")
				return
			}
			if err := svc.SetActiveRevision(r.Context(), id, req.Revision); err != nil {
				s.writeGovError(w, r, err, "SET_ACTIVE_REVISION_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, map[string]any{"policy_id": id, "active_revision": req.Revision})
			return
		}
	case "revisions":
		if len(parts) == 2 {
			switch r.Method {
			case http.MethodGet:
				revs, err := svc.ListPolicyRevisions(r.Context(), id)
				if err != nil {
					s.writeGovError(w, r, err, "LIST_REVISIONS_FAILED")
					return
				}
				if revs == nil {
					revs = []model.PolicyRevision{}
				}
				s.writeJSON(w, http.StatusOK, map[string]any{"policy_id": id, "revisions": revs, "count": len(revs)})
			case http.MethodPost:
				var rev model.PolicyRevision
				if err := json.NewDecoder(r.Body).Decode(&rev); err != nil {
					s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Failed to parse revision JSON")
					return
				}
				rev.PolicyID = id
				if err := svc.PublishRevision(r.Context(), &rev); err != nil {
					s.writeGovError(w, r, err, "PUBLISH_REVISION_FAILED")
					return
				}
				s.writeJSON(w, http.StatusCreated, rev)
			default:
				w.Header().Set("Allow", "GET, POST")
				s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
			}
			return
		}
		if len(parts) == 3 && r.Method == http.MethodGet {
			revNum, err := strconv.Atoi(parts[2])
			if err != nil {
				s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REVISION_NUMBER", "Revision must be an integer")
				return
			}
			rev, err := svc.GetPolicyRevision(r.Context(), id, revNum)
			if err != nil {
				s.writeGovError(w, r, err, "GET_REVISION_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, rev)
			return
		}
	}

	s.writeAPIError(w, r, http.StatusNotFound, "NOT_FOUND", "Endpoint not found")
}

// ---------------------------------------------------------------------------
// 5. Policy Assignments
// ---------------------------------------------------------------------------

func (s *Server) handleGovernanceAssignments(w http.ResponseWriter, r *http.Request, svc governance.GovernanceService, parts []string) {
	if len(parts) == 0 {
		switch r.Method {
		case http.MethodGet:
			q := r.URL.Query()
			enabledOnly := false
			if enStr := q.Get("enabled_only"); enStr != "" {
				enabledOnly = strings.EqualFold(enStr, "true") || enStr == "1"
			} else if enStr := q.Get("enabled"); enStr != "" {
				enabledOnly = strings.EqualFold(enStr, "true") || enStr == "1"
			}
			filter := model.PolicyAssignmentFilter{
				OrgID:       q.Get("org_id"),
				PolicyID:    q.Get("policy_id"),
				TargetType:  model.PolicyTargetType(q.Get("target_type")),
				TargetID:    q.Get("target_id"),
				EnabledOnly: enabledOnly,
			}
			asgns, err := svc.ListAssignments(r.Context(), filter)
			if err != nil {
				s.writeGovError(w, r, err, "LIST_ASSIGNMENTS_FAILED")
				return
			}
			if asgns == nil {
				asgns = []model.PolicyAssignment{}
			}
			s.writeJSON(w, http.StatusOK, map[string]any{"assignments": asgns, "count": len(asgns)})
		case http.MethodPost:
			var asgn model.PolicyAssignment
			if err := json.NewDecoder(r.Body).Decode(&asgn); err != nil {
				s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Failed to parse assignment JSON")
				return
			}
			if asgn.OrgID == "" {
				asgn.OrgID = model.DefaultOrganizationID
			}
			if err := svc.CreateAssignment(r.Context(), &asgn); err != nil {
				s.writeGovError(w, r, err, "CREATE_ASSIGNMENT_FAILED")
				return
			}
			s.writeJSON(w, http.StatusCreated, asgn)
		default:
			w.Header().Set("Allow", "GET, POST")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		}
		return
	}

	id := parts[0]
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			asgn, err := svc.GetAssignment(r.Context(), id)
			if err != nil {
				s.writeGovError(w, r, err, "GET_ASSIGNMENT_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, asgn)
		case http.MethodDelete:
			if err := svc.DeleteAssignment(r.Context(), id); err != nil {
				s.writeGovError(w, r, err, "DELETE_ASSIGNMENT_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "assignment_id": id})
		default:
			w.Header().Set("Allow", "GET, DELETE")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		}
		return
	}

	if len(parts) == 2 && parts[1] == "enable" && r.Method == http.MethodPut {
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Failed to parse enable JSON")
			return
		}
		if err := svc.SetAssignmentEnabled(r.Context(), id, req.Enabled); err != nil {
			s.writeGovError(w, r, err, "SET_ASSIGNMENT_ENABLED_FAILED")
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"assignment_id": id, "enabled": req.Enabled})
		return
	}

	s.writeAPIError(w, r, http.StatusNotFound, "NOT_FOUND", "Endpoint not found")
}

// ---------------------------------------------------------------------------
// 6. Runtime Evaluations & Findings
// ---------------------------------------------------------------------------

func (s *Server) handleGovernanceEvaluations(w http.ResponseWriter, r *http.Request, svc governance.GovernanceService, parts []string) {
	if len(parts) == 0 {
		if r.Method == http.MethodGet {
			q := r.URL.Query()
			limit, _ := strconv.Atoi(q.Get("limit"))
			var since time.Time
			if sStr := q.Get("since"); sStr != "" {
				if t, err := parseTimeOrDuration(sStr); err == nil {
					since = t
				}
			}
			filter := model.EvaluationFilter{
				OrgID:        q.Get("org_id"),
				TargetNodeID: q.Get("node_id"),
				TriggerType:  model.EvaluationTriggerType(q.Get("trigger_type")),
				Status:       model.EvaluationStatus(q.Get("status")),
				Limit:        limit,
				Since:        since,
			}
			if filter.TargetNodeID == "" {
				filter.TargetNodeID = q.Get("target_node_id")
			}
			execs, err := svc.ListEvaluationExecutions(r.Context(), filter)
			if err != nil {
				s.writeGovError(w, r, err, "LIST_EVALUATIONS_FAILED")
				return
			}
			if execs == nil {
				execs = []model.EvaluationExecution{}
			}
			s.writeJSON(w, http.StatusOK, map[string]any{"evaluations": execs, "count": len(execs)})
			return
		}
		w.Header().Set("Allow", "GET")
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	sub := parts[0]
	switch sub {
	case "node":
		if len(parts) == 2 && r.Method == http.MethodPost {
			nodeID := parts[1]
			var req struct {
				Trigger model.EvaluationTriggerType `json:"trigger"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.Trigger == "" {
				req.Trigger = model.EvaluationTriggerOnDemand
			}
			exec, err := svc.EvaluateNode(r.Context(), nodeID, req.Trigger)
			if err != nil {
				s.writeGovError(w, r, err, "EVALUATE_NODE_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, exec)
			return
		}
	case "group":
		if len(parts) == 2 && r.Method == http.MethodPost {
			groupID := parts[1]
			var req struct {
				OrgID            string                      `json:"org_id"`
				IncludeSubgroups bool                        `json:"include_subgroups"`
				Trigger          model.EvaluationTriggerType `json:"trigger"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.OrgID == "" {
				req.OrgID = r.URL.Query().Get("org_id")
			}
			if req.OrgID == "" {
				req.OrgID = model.DefaultOrganizationID
			}
			if req.Trigger == "" {
				req.Trigger = model.EvaluationTriggerOnDemand
			}
			execs, err := svc.EvaluateFleetGroup(r.Context(), req.OrgID, groupID, req.IncludeSubgroups, req.Trigger)
			if err != nil {
				s.writeGovError(w, r, err, "EVALUATE_GROUP_FAILED")
				return
			}
			if execs == nil {
				execs = []*model.EvaluationExecution{}
			}
			s.writeJSON(w, http.StatusOK, map[string]any{"group_id": groupID, "evaluations": execs, "count": len(execs)})
			return
		}
	case "org":
		if len(parts) == 2 && r.Method == http.MethodPost {
			orgID := parts[1]
			var req struct {
				Trigger model.EvaluationTriggerType `json:"trigger"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.Trigger == "" {
				req.Trigger = model.EvaluationTriggerOnDemand
			}
			execs, err := svc.EvaluateOrganization(r.Context(), orgID, req.Trigger)
			if err != nil {
				s.writeGovError(w, r, err, "EVALUATE_ORG_FAILED")
				return
			}
			if execs == nil {
				execs = []*model.EvaluationExecution{}
			}
			s.writeJSON(w, http.StatusOK, map[string]any{"org_id": orgID, "evaluations": execs, "count": len(execs)})
			return
		}
	case "latest":
		if len(parts) == 3 && parts[1] == "node" && r.Method == http.MethodGet {
			targetNodeID := parts[2]
			orgID := r.URL.Query().Get("org_id")
			if orgID == "" {
				orgID = model.DefaultOrganizationID
			}
			exec, err := svc.GetLatestNodeEvaluation(r.Context(), orgID, targetNodeID)
			if err != nil {
				s.writeGovError(w, r, err, "GET_LATEST_EVALUATION_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, exec)
			return
		}
	default:
		if len(parts) == 1 && r.Method == http.MethodGet {
			id := parts[0]
			exec, err := svc.GetEvaluationExecution(r.Context(), id)
			if err != nil {
				s.writeGovError(w, r, err, "GET_EVALUATION_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, exec)
			return
		}
	}

	s.writeAPIError(w, r, http.StatusNotFound, "NOT_FOUND", "Endpoint not found")
}

func (s *Server) handleGovernanceFindings(w http.ResponseWriter, r *http.Request, svc governance.GovernanceService, parts []string) {
	if len(parts) == 0 {
		if r.Method == http.MethodGet {
			q := r.URL.Query()
			limit, _ := strconv.Atoi(q.Get("limit"))
			var since time.Time
			if sStr := q.Get("since"); sStr != "" {
				if t, err := parseTimeOrDuration(sStr); err == nil {
					since = t
				}
			}
			filter := model.FindingFilter{
				OrgID:        q.Get("org_id"),
				TargetNodeID: q.Get("node_id"),
				PolicyID:     q.Get("policy_id"),
				RuleID:       q.Get("rule_id"),
				Status:       model.FindingStatus(q.Get("status")),
				Severity:     model.Severity(q.Get("severity")),
				Category:     model.PolicyCategory(q.Get("category")),
				Limit:        limit,
				Since:        since,
			}
			if filter.TargetNodeID == "" {
				filter.TargetNodeID = q.Get("target_node_id")
			}
			findings, err := svc.ListComplianceFindings(r.Context(), filter)
			if err != nil {
				s.writeGovError(w, r, err, "LIST_FINDINGS_FAILED")
				return
			}
			if findings == nil {
				findings = []model.ComplianceFinding{}
			}
			s.writeJSON(w, http.StatusOK, map[string]any{"findings": findings, "count": len(findings)})
			return
		}
		w.Header().Set("Allow", "GET")
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	if len(parts) == 1 && r.Method == http.MethodGet {
		id := parts[0]
		finding, err := svc.GetComplianceFinding(r.Context(), id)
		if err != nil {
			s.writeGovError(w, r, err, "GET_FINDING_FAILED")
			return
		}
		s.writeJSON(w, http.StatusOK, finding)
		return
	}

	s.writeAPIError(w, r, http.StatusNotFound, "NOT_FOUND", "Endpoint not found")
}

// ---------------------------------------------------------------------------
// 7. Compliance Summaries & Rollups
// ---------------------------------------------------------------------------

func (s *Server) handleGovernanceCompliance(w http.ResponseWriter, r *http.Request, svc governance.GovernanceService, parts []string) {
	if len(parts) < 2 || r.Method != http.MethodGet {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
			return
		}
		s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_COMPLIANCE_ROUTE", "Scope and target ID required")
		return
	}

	scope := parts[0]
	targetID := parts[1]
	orgID := r.URL.Query().Get("org_id")
	if orgID == "" {
		orgID = model.DefaultOrganizationID
	}

	switch scope {
	case "node":
		summary, err := svc.GetNodeComplianceSummary(r.Context(), orgID, targetID)
		if err != nil {
			s.writeGovError(w, r, err, "GET_NODE_COMPLIANCE_FAILED")
			return
		}
		s.writeJSON(w, http.StatusOK, summary)
	case "group":
		incSub := r.URL.Query().Get("include_subgroups") == "true" || r.URL.Query().Get("include_subgroups") == "1"
		summary, err := svc.GetGroupComplianceSummary(r.Context(), orgID, targetID, incSub)
		if err != nil {
			s.writeGovError(w, r, err, "GET_GROUP_COMPLIANCE_FAILED")
			return
		}
		s.writeJSON(w, http.StatusOK, summary)
	case "org":
		summary, err := svc.GetOrgComplianceSummary(r.Context(), targetID)
		if err != nil {
			s.writeGovError(w, r, err, "GET_ORG_COMPLIANCE_FAILED")
			return
		}
		s.writeJSON(w, http.StatusOK, summary)
	default:
		s.writeAPIError(w, r, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("Compliance scope '%s' not supported", scope))
	}
}

// ---------------------------------------------------------------------------
// 8. Ephemeral Simulation
// ---------------------------------------------------------------------------

func (s *Server) handleGovernanceSimulation(w http.ResponseWriter, r *http.Request, svc governance.GovernanceService) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}
	var req model.SimulationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Failed to parse simulation request JSON")
		return
	}
	if req.OrgID == "" {
		req.OrgID = model.DefaultOrganizationID
	}
	res, err := svc.SimulatePolicyChanges(r.Context(), &req)
	if err != nil {
		s.writeGovError(w, r, err, "SIMULATION_FAILED")
		return
	}
	s.writeJSON(w, http.StatusOK, res)
}

// ---------------------------------------------------------------------------
// 9. Maintenance Windows
// ---------------------------------------------------------------------------

func (s *Server) handleGovernanceMaintenance(w http.ResponseWriter, r *http.Request, svc governance.GovernanceService, parts []string) {
	if len(parts) == 0 {
		switch r.Method {
		case http.MethodGet:
			q := r.URL.Query()
			limit, _ := strconv.Atoi(q.Get("limit"))
			filter := model.MaintenanceWindowFilter{
				OrgID:       q.Get("org_id"),
				TargetScope: model.MaintenanceTargetScope(q.Get("target_scope")),
				TargetID:    q.Get("target_id"),
				Status:      model.MaintenanceStatus(q.Get("status")),
				Limit:       limit,
			}
			windows, err := svc.ListMaintenanceWindows(r.Context(), filter)
			if err != nil {
				s.writeGovError(w, r, err, "LIST_MAINTENANCE_FAILED")
				return
			}
			if windows == nil {
				windows = []model.MaintenanceWindow{}
			}
			s.writeJSON(w, http.StatusOK, map[string]any{"maintenance_windows": windows, "count": len(windows)})
		case http.MethodPost:
			var win model.MaintenanceWindow
			if err := json.NewDecoder(r.Body).Decode(&win); err != nil {
				s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Failed to parse maintenance window JSON")
				return
			}
			if win.OrgID == "" {
				win.OrgID = model.DefaultOrganizationID
			}
			if err := svc.CreateMaintenanceWindow(r.Context(), &win); err != nil {
				s.writeGovError(w, r, err, "CREATE_MAINTENANCE_FAILED")
				return
			}
			s.writeJSON(w, http.StatusCreated, win)
		default:
			w.Header().Set("Allow", "GET, POST")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		}
		return
	}

	if parts[0] == "evaluate" && r.Method == http.MethodGet {
		orgID := r.URL.Query().Get("org_id")
		if orgID == "" {
			orgID = model.DefaultOrganizationID
		}
		evalTime := time.Now().UTC()
		if tStr := r.URL.Query().Get("time"); tStr != "" {
			if t, err := parseTimeOrDuration(tStr); err == nil {
				evalTime = t
			}
		}
		activeWins, err := svc.EvaluateMaintenanceWindows(r.Context(), orgID, evalTime)
		if err != nil {
			s.writeGovError(w, r, err, "EVALUATE_MAINTENANCE_FAILED")
			return
		}
		if activeWins == nil {
			activeWins = []model.MaintenanceWindow{}
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"org_id": orgID, "active_windows": activeWins, "count": len(activeWins), "eval_time": evalTime})
		return
	}

	id := parts[0]
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			win, err := svc.GetMaintenanceWindow(r.Context(), id)
			if err != nil {
				s.writeGovError(w, r, err, "GET_MAINTENANCE_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, win)
		case http.MethodPut:
			var win model.MaintenanceWindow
			if err := json.NewDecoder(r.Body).Decode(&win); err != nil {
				s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Failed to parse maintenance window JSON")
				return
			}
			win.ID = id
			if err := svc.UpdateMaintenanceWindow(r.Context(), &win); err != nil {
				s.writeGovError(w, r, err, "UPDATE_MAINTENANCE_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, win)
		case http.MethodDelete:
			if err := svc.DeleteMaintenanceWindow(r.Context(), id); err != nil {
				s.writeGovError(w, r, err, "DELETE_MAINTENANCE_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "maintenance_window_id": id})
		default:
			w.Header().Set("Allow", "GET, PUT, DELETE")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		}
		return
	}

	if len(parts) == 2 && parts[1] == "cancel" && r.Method == http.MethodPost {
		var req struct {
			Reason string `json:"reason"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if err := svc.CancelMaintenanceWindow(r.Context(), id, req.Reason); err != nil {
			s.writeGovError(w, r, err, "CANCEL_MAINTENANCE_FAILED")
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"cancelled": true, "maintenance_window_id": id, "reason": req.Reason})
		return
	}

	s.writeAPIError(w, r, http.StatusNotFound, "NOT_FOUND", "Endpoint not found")
}

// ---------------------------------------------------------------------------
// 10. Escalation Policies
// ---------------------------------------------------------------------------

func (s *Server) handleGovernanceEscalation(w http.ResponseWriter, r *http.Request, svc governance.GovernanceService, parts []string) {
	if len(parts) == 0 {
		switch r.Method {
		case http.MethodGet:
			q := r.URL.Query()
			var enabled *bool
			if enStr := q.Get("enabled"); enStr != "" {
				val := strings.EqualFold(enStr, "true") || enStr == "1"
				enabled = &val
			}
			filter := model.EscalationPolicyFilter{
				OrgID:   q.Get("org_id"),
				Enabled: enabled,
			}
			policies, err := svc.ListEscalationPolicies(r.Context(), filter)
			if err != nil {
				s.writeGovError(w, r, err, "LIST_ESCALATION_FAILED")
				return
			}
			if policies == nil {
				policies = []model.EscalationPolicy{}
			}
			s.writeJSON(w, http.StatusOK, map[string]any{"escalation_policies": policies, "count": len(policies)})
		case http.MethodPost:
			var policy model.EscalationPolicy
			if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
				s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Failed to parse escalation policy JSON")
				return
			}
			if policy.OrgID == "" {
				policy.OrgID = model.DefaultOrganizationID
			}
			if err := svc.CreateEscalationPolicy(r.Context(), &policy); err != nil {
				s.writeGovError(w, r, err, "CREATE_ESCALATION_FAILED")
				return
			}
			s.writeJSON(w, http.StatusCreated, policy)
		default:
			w.Header().Set("Allow", "GET, POST")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		}
		return
	}

	if parts[0] == "evaluate" && r.Method == http.MethodPost {
		var req struct {
			OrgID    string              `json:"org_id"`
			Incident *incidents.Incident `json:"incident"`
			EvalTime *time.Time          `json:"eval_time"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Failed to parse escalation eval JSON")
			return
		}
		if req.OrgID == "" {
			req.OrgID = model.DefaultOrganizationID
		}
		evalTime := time.Now().UTC()
		if req.EvalTime != nil {
			evalTime = *req.EvalTime
		}
		res, err := svc.EvaluateIncidentEscalation(r.Context(), req.OrgID, req.Incident, evalTime)
		if err != nil {
			s.writeGovError(w, r, err, "EVALUATE_ESCALATION_FAILED")
			return
		}
		s.writeJSON(w, http.StatusOK, res)
		return
	}

	id := parts[0]
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			policy, err := svc.GetEscalationPolicy(r.Context(), id)
			if err != nil {
				s.writeGovError(w, r, err, "GET_ESCALATION_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, policy)
		case http.MethodPut:
			var policy model.EscalationPolicy
			if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
				s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Failed to parse escalation policy JSON")
				return
			}
			policy.ID = id
			if err := svc.UpdateEscalationPolicy(r.Context(), &policy); err != nil {
				s.writeGovError(w, r, err, "UPDATE_ESCALATION_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, policy)
		case http.MethodDelete:
			if err := svc.DeleteEscalationPolicy(r.Context(), id); err != nil {
				s.writeGovError(w, r, err, "DELETE_ESCALATION_FAILED")
				return
			}
			s.writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "escalation_policy_id": id})
		default:
			w.Header().Set("Allow", "GET, PUT, DELETE")
			s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		}
		return
	}

	s.writeAPIError(w, r, http.StatusNotFound, "NOT_FOUND", "Endpoint not found")
}

// ---------------------------------------------------------------------------
// 11. Alert & Finding Suppression
// ---------------------------------------------------------------------------

func (s *Server) handleGovernanceSuppression(w http.ResponseWriter, r *http.Request, svc governance.GovernanceService, parts []string) {
	if len(parts) == 0 {
		if r.Method == http.MethodGet {
			q := r.URL.Query()
			limit, _ := strconv.Atoi(q.Get("limit"))
			var since *time.Time
			if sStr := q.Get("since"); sStr != "" {
				if t, err := parseTimeOrDuration(sStr); err == nil {
					since = &t
				}
			}
			filter := model.SuppressionFilter{
				OrgID:   q.Get("org_id"),
				NodeID:  q.Get("node_id"),
				AlertID: q.Get("alert_id"),
				Outcome: model.SuppressionOutcome(q.Get("outcome")),
				Limit:   limit,
				Since:   since,
			}
			decisions, err := svc.ListSuppressionDecisions(r.Context(), filter)
			if err != nil {
				s.writeGovError(w, r, err, "LIST_SUPPRESSIONS_FAILED")
				return
			}
			if decisions == nil {
				decisions = []model.SuppressionDecision{}
			}
			s.writeJSON(w, http.StatusOK, map[string]any{"suppression_decisions": decisions, "count": len(decisions)})
			return
		}
		w.Header().Set("Allow", "GET")
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	if parts[0] == "evaluate" && r.Method == http.MethodPost {
		var req governance.SuppressionEvaluationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Failed to parse suppression request JSON")
			return
		}
		if req.OrgID == "" {
			req.OrgID = model.DefaultOrganizationID
		}
		decision, err := svc.EvaluateSuppression(r.Context(), req)
		if err != nil {
			s.writeGovError(w, r, err, "EVALUATE_SUPPRESSION_FAILED")
			return
		}
		s.writeJSON(w, http.StatusOK, decision)
		return
	}

	if len(parts) == 1 && r.Method == http.MethodGet {
		id := parts[0]
		decision, err := svc.GetSuppressionDecision(r.Context(), id)
		if err != nil {
			s.writeGovError(w, r, err, "GET_SUPPRESSION_FAILED")
			return
		}
		s.writeJSON(w, http.StatusOK, decision)
		return
	}

	s.writeAPIError(w, r, http.StatusNotFound, "NOT_FOUND", "Endpoint not found")
}

// ---------------------------------------------------------------------------
// 12. Audit Trail
// ---------------------------------------------------------------------------

func (s *Server) handleGovernanceAudit(w http.ResponseWriter, r *http.Request, svc governance.GovernanceService) {
	switch r.Method {
	case http.MethodGet:
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		offset, _ := strconv.Atoi(q.Get("offset"))
		var startTime time.Time
		var endTime time.Time
		if st := q.Get("start_time"); st != "" {
			if t, err := parseTimeOrDuration(st); err == nil {
				startTime = t
			}
		}
		if et := q.Get("end_time"); et != "" {
			if t, err := parseTimeOrDuration(et); err == nil {
				endTime = t
			}
		}
		filter := storage.AuditFilter{
			EventType:     q.Get("event_type"),
			Severity:      q.Get("severity"),
			Outcome:       q.Get("outcome"),
			ActorType:     q.Get("actor_type"),
			ActorIdentity: q.Get("actor_identity"),
			SourceAddress: q.Get("source_address"),
			RequestID:     q.Get("request_id"),
			StartTime:     startTime,
			EndTime:       endTime,
			Limit:         limit,
			Offset:        offset,
		}
		if filter.ActorIdentity == "" {
			filter.ActorIdentity = q.Get("actor")
		}
		events, err := svc.QueryAuditEvents(r.Context(), filter)
		if err != nil {
			s.writeGovError(w, r, err, "QUERY_AUDIT_FAILED")
			return
		}
		if events == nil {
			events = []model.AuditEvent{}
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"audit_events": events, "count": len(events)})
	case http.MethodPost:
		var event model.AuditEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Failed to parse audit event JSON")
			return
		}
		if event.Metadata == nil {
			event.Metadata = make(map[string]string)
		}
		if event.Metadata["org_id"] == "" {
			event.Metadata["org_id"] = model.DefaultOrganizationID
		}
		if err := svc.RecordAuditEvent(r.Context(), event); err != nil {
			s.writeGovError(w, r, err, "RECORD_AUDIT_FAILED")
			return
		}
		s.writeJSON(w, http.StatusCreated, event)
	default:
		w.Header().Set("Allow", "GET, POST")
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
	}
}

// ---------------------------------------------------------------------------
// Error Mapping Helper
// ---------------------------------------------------------------------------

func (s *Server) writeGovError(w http.ResponseWriter, r *http.Request, err error, defaultCode string) {
	if err == nil {
		return
	}
	status, code := mapGovernanceError(err)
	if code == "INTERNAL_ERROR" && defaultCode != "" {
		code = defaultCode
	}
	s.writeAPIError(w, r, status, code, err.Error())
}

func mapGovernanceError(err error) (int, string) {
	if err == nil {
		return http.StatusOK, ""
	}
	switch {
	case errors.Is(err, governance.ErrOrganizationNotFound):
		return http.StatusNotFound, "ORGANIZATION_NOT_FOUND"
	case errors.Is(err, governance.ErrOrganizationExists):
		return http.StatusConflict, "ORGANIZATION_ALREADY_EXISTS"
	case errors.Is(err, governance.ErrDefaultOrgImmutable):
		return http.StatusForbidden, "DEFAULT_ORG_IMMUTABLE"
	case errors.Is(err, governance.ErrFleetGroupNotFound):
		return http.StatusNotFound, "FLEET_GROUP_NOT_FOUND"
	case errors.Is(err, governance.ErrFleetGroupExists):
		return http.StatusConflict, "FLEET_GROUP_ALREADY_EXISTS"
	case errors.Is(err, governance.ErrCycleDetected):
		return http.StatusBadRequest, "CYCLE_DETECTED"
	case errors.Is(err, governance.ErrParentNotFound):
		return http.StatusBadRequest, "PARENT_NOT_FOUND"
	case errors.Is(err, governance.ErrCrossOrgParent):
		return http.StatusForbidden, "CROSS_ORG_PARENT_FORBIDDEN"
	case errors.Is(err, governance.ErrNodeNotFound):
		return http.StatusNotFound, "NODE_NOT_FOUND"
	case errors.Is(err, governance.ErrInvalidIdentifier):
		return http.StatusBadRequest, "INVALID_IDENTIFIER"
	case errors.Is(err, governance.ErrInvalidInput):
		return http.StatusBadRequest, "INVALID_INPUT"
	case errors.Is(err, governance.ErrPolicyNotFound):
		return http.StatusNotFound, "POLICY_NOT_FOUND"
	case errors.Is(err, governance.ErrPolicyExists):
		return http.StatusConflict, "POLICY_ALREADY_EXISTS"
	case errors.Is(err, governance.ErrPolicyRevisionNotFound):
		return http.StatusNotFound, "POLICY_REVISION_NOT_FOUND"
	case errors.Is(err, governance.ErrPolicyRevisionDigestMismatch):
		return http.StatusBadRequest, "POLICY_REVISION_DIGEST_MISMATCH"
	case errors.Is(err, governance.ErrPolicyAssignmentNotFound):
		return http.StatusNotFound, "POLICY_ASSIGNMENT_NOT_FOUND"
	case errors.Is(err, governance.ErrPolicyAssignmentExists):
		return http.StatusConflict, "POLICY_ASSIGNMENT_ALREADY_EXISTS"
	case errors.Is(err, governance.ErrCrossOrgAssignment):
		return http.StatusForbidden, "CROSS_ORG_ASSIGNMENT_FORBIDDEN"
	case errors.Is(err, governance.ErrInvalidLifecycleTransition):
		return http.StatusBadRequest, "INVALID_LIFECYCLE_TRANSITION"
	case errors.Is(err, governance.ErrInvalidSelector),
		errors.Is(err, governance.ErrSelectorMaxDepthExceeded),
		errors.Is(err, governance.ErrSelectorTooLong):
		return http.StatusBadRequest, "INVALID_SELECTOR"
	case errors.Is(err, governance.ErrMaintenanceWindowNotFound):
		return http.StatusNotFound, "MAINTENANCE_WINDOW_NOT_FOUND"
	case errors.Is(err, governance.ErrMaintenanceWindowExists):
		return http.StatusConflict, "MAINTENANCE_WINDOW_ALREADY_EXISTS"
	case errors.Is(err, governance.ErrInvalidMaintenanceTransition):
		return http.StatusBadRequest, "INVALID_MAINTENANCE_TRANSITION"
	case errors.Is(err, governance.ErrCrossOrgMaintenance):
		return http.StatusForbidden, "CROSS_ORG_MAINTENANCE_FORBIDDEN"
	case errors.Is(err, governance.ErrEscalationPolicyNotFound):
		return http.StatusNotFound, "ESCALATION_POLICY_NOT_FOUND"
	case errors.Is(err, governance.ErrEscalationPolicyExists):
		return http.StatusConflict, "ESCALATION_POLICY_ALREADY_EXISTS"
	case errors.Is(err, governance.ErrCrossOrgEscalation):
		return http.StatusForbidden, "CROSS_ORG_ESCALATION_FORBIDDEN"
	case errors.Is(err, governance.ErrSuppressionDecisionNotFound):
		return http.StatusNotFound, "SUPPRESSION_DECISION_NOT_FOUND"
	default:
		return http.StatusInternalServerError, "INTERNAL_ERROR"
	}
}
