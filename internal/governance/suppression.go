package governance

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

// SuppressionEngine coordinates policy-driven alert and finding suppression evaluations
// against active maintenance windows, scope matching, category restrictions, and severity thresholds.
type SuppressionEngine struct {
	store             storage.ReadOnlyStorage
	maintenanceEngine *MaintenanceEngine
	clock             Clock
}

// NewSuppressionEngine constructs a new SuppressionEngine.
func NewSuppressionEngine(store storage.ReadOnlyStorage, maintenanceEngine *MaintenanceEngine, clock Clock) *SuppressionEngine {
	if clock == nil {
		clock = RealClock{}
	}
	if maintenanceEngine == nil {
		maintenanceEngine = NewMaintenanceEngine(store, clock)
	}
	return &SuppressionEngine{
		store:             store,
		maintenanceEngine: maintenanceEngine,
		clock:             clock,
	}
}

// SuppressionEvaluationRequest wraps the contextual inputs required to evaluate alert suppression.
type SuppressionEvaluationRequest struct {
	OrgID         string
	NodeID        string
	Node          *model.FleetNode
	Ownership     *model.NodeOwnershipMetadata
	GroupIDs      []string
	GroupPaths    []string
	AlertID       string
	IncidentID    string
	RuleID        string
	RuleName      string
	Category      string
	Severity      model.Severity
	IsFinding     bool
	Message       string
	Timestamp     time.Time
	CustomDetails map[string]string
}

// Evaluate evaluates whether an alert, incident, or finding should be suppressed.
func (e *SuppressionEngine) Evaluate(ctx context.Context, req SuppressionEvaluationRequest) (*model.SuppressionDecision, error) {
	evalTime := req.Timestamp
	if evalTime.IsZero() {
		evalTime = e.clock.Now().UTC()
	}

	orgID := req.OrgID
	if orgID == "" {
		orgID = model.DefaultOrganizationID
	}

	// 1. Validate Severity
	if req.Severity == "" {
		req.Severity = model.SeverityInfo
	}
	if !req.Severity.IsValid() {
		return e.newDecision(orgID, req, model.SuppressionOutcomeIndeterminate, model.SuppressionReasonEvaluationError,
			"", fmt.Sprintf("invalid severity: %s", req.Severity), evalTime), fmt.Errorf("%w: invalid severity %s", ErrInvalidInput, req.Severity)
	}

	// 2. Safety Rule: Critical Security and Integrity Events are strictly exempt from suppression
	if isExempt, reason, msg := isSecurityOrIntegrityExempt(req.Category, req.RuleName, req.RuleID, req.Message, req.CustomDetails); isExempt {
		return e.newDecision(orgID, req, model.SuppressionOutcomeExempt, reason, "", msg, evalTime), nil
	}

	// 3. Resolve Node and Hierarchy context if node object is missing but NodeID is present
	node := req.Node
	ownership := req.Ownership
	groupIDs := req.GroupIDs
	groupPaths := req.GroupPaths

	if node == nil && req.NodeID != "" && e.store != nil {
		if storedNode, err := e.store.GetFleetNode(ctx, req.NodeID); err == nil && storedNode != nil {
			node = storedNode
		}
	}

	if ownership == nil && node != nil && node.Metadata != nil {
		ownership = model.NodeOwnershipFromMetadata(node.Metadata)
	}

	if len(groupIDs) == 0 && req.NodeID != "" && e.store != nil {
		if groups, err := e.store.GetNodeGroups(ctx, req.NodeID); err == nil {
			for _, g := range groups {
				groupIDs = append(groupIDs, g.ID)
				if g.Path != "" {
					groupPaths = append(groupPaths, g.Path)
				} else if g.Metadata != nil && g.Metadata["path"] != "" {
					groupPaths = append(groupPaths, g.Metadata["path"])
				}
			}
		}
	}

	// 4. Retrieve Active Maintenance Windows for this scope
	var activeWindows []model.MaintenanceWindow
	if node != nil {
		windows, err := e.maintenanceEngine.GetActiveWindowsForNode(ctx, orgID, node, ownership, groupIDs, groupPaths, evalTime)
		if err != nil {
			return e.newDecision(orgID, req, model.SuppressionOutcomeIndeterminate, model.SuppressionReasonEvaluationError,
				"", fmt.Sprintf("failed to query maintenance windows: %v", err), evalTime), err
		}
		activeWindows = windows
	} else if e.store != nil {
		// Target is not a specific node (e.g. Org-level or Incident-level without node), query org-level active windows
		windows, err := e.store.ListMaintenanceWindows(ctx, model.MaintenanceWindowFilter{
			OrgID:    orgID,
			ActiveAt: &evalTime,
		})
		if err != nil {
			return e.newDecision(orgID, req, model.SuppressionOutcomeIndeterminate, model.SuppressionReasonEvaluationError,
				"", fmt.Sprintf("failed to query maintenance windows: %v", err), evalTime), err
		}
		for _, w := range windows {
			active, err := e.maintenanceEngine.IsWindowActive(&w, evalTime)
			if err == nil && active {
				activeWindows = append(activeWindows, w)
			}
		}
	}

	// If no active maintenance window covers this target
	if len(activeWindows) == 0 {
		return e.newDecision(orgID, req, model.SuppressionOutcomeNotSuppressed, model.SuppressionReasonNoActiveWindow,
			"", "no active maintenance window matched", evalTime), nil
	}

	// 5. Evaluate matching windows
	var lastNonSuppressReason model.SuppressionReason = model.SuppressionReasonNoActiveWindow
	var lastNonSuppressMsg string

	for _, w := range activeWindows {
		// A. Check if the window is configured to suppress this event type
		if req.IsFinding {
			if !w.SuppressFindings {
				lastNonSuppressReason = model.SuppressionReasonWindowAlertsNotSuppressed
				lastNonSuppressMsg = fmt.Sprintf("window %q does not suppress compliance findings", w.Name)
				continue
			}
		} else {
			if !w.SuppressAlerts {
				lastNonSuppressReason = model.SuppressionReasonWindowAlertsNotSuppressed
				lastNonSuppressMsg = fmt.Sprintf("window %q does not suppress alerts", w.Name)
				continue
			}
		}

		// B. Check Hard Safety Guardrail: AllowCriticalAlerts on Critical severity
		if req.Severity == model.SeverityCritical && w.AllowCriticalAlerts {
			lastNonSuppressReason = model.SuppressionReasonSeverityExceedsThreshold
			lastNonSuppressMsg = fmt.Sprintf("critical alert allowed by safety rule in window %q", w.Name)
			continue
		}

		// C. Check Severity Threshold: if event severity exceeds window's threshold, it is not suppressed
		if w.SeverityThreshold != "" {
			if severityRank(req.Severity) > severityRank(w.SeverityThreshold) {
				lastNonSuppressReason = model.SuppressionReasonSeverityExceedsThreshold
				lastNonSuppressMsg = fmt.Sprintf("event severity %s exceeds window threshold %s in window %q", req.Severity, w.SeverityThreshold, w.Name)
				continue
			}
		}

		// D. Check Category Restrictions
		if len(w.CategoryRestrictions) > 0 {
			normalizedCat := model.PolicyCategory(req.Category)
			categoryMatched := false
			reqCatLower := strings.ToLower(strings.TrimSpace(req.Category))
			for _, cat := range w.CategoryRestrictions {
				catLower := strings.ToLower(strings.TrimSpace(string(cat)))
				if cat == normalizedCat || strings.EqualFold(string(cat), req.Category) ||
					(reqCatLower != "" && (strings.HasPrefix(catLower, reqCatLower) || strings.HasPrefix(reqCatLower, catLower))) {
					categoryMatched = true
					break
				}
			}
			if !categoryMatched {
				lastNonSuppressReason = model.SuppressionReasonCategoryNotCovered
				lastNonSuppressMsg = fmt.Sprintf("event category %q is not covered by window %q restrictions", req.Category, w.Name)
				continue
			}
		}

		// All conditions passed for this window -> Suppress!
		msg := fmt.Sprintf("Suppressed by active maintenance window %q (%s)", w.Name, w.ID)
		decision := e.newDecision(orgID, req, model.SuppressionOutcomeSuppressed, model.SuppressionReasonInMaintenanceWindow, w.ID, msg, evalTime)
		if decision.Details == nil {
			decision.Details = make(map[string]string)
		}
		decision.Details["window_name"] = w.Name
		decision.Details["window_scope"] = string(w.TargetScope)
		return decision, nil
	}

	// If no window passed all checks to suppress
	if lastNonSuppressMsg == "" {
		lastNonSuppressMsg = "no active maintenance window satisfied all suppression criteria"
	}
	return e.newDecision(orgID, req, model.SuppressionOutcomeNotSuppressed, lastNonSuppressReason, "", lastNonSuppressMsg, evalTime), nil
}

// EvaluateAlert evaluates suppression for an alert event.
func (e *SuppressionEngine) EvaluateAlert(
	ctx context.Context,
	orgID string,
	node *model.FleetNode,
	ownership *model.NodeOwnershipMetadata,
	groupIDs []string,
	groupPaths []string,
	alert *model.AlertEvent,
	category string,
	t time.Time,
) (*model.SuppressionDecision, error) {
	if alert == nil {
		return nil, fmt.Errorf("%w: nil alert event", ErrInvalidInput)
	}

	nodeID := ""
	if node != nil {
		nodeID = node.Identity.NodeID
	}

	req := SuppressionEvaluationRequest{
		OrgID:      orgID,
		NodeID:     nodeID,
		Node:       node,
		Ownership:  ownership,
		GroupIDs:   groupIDs,
		GroupPaths: groupPaths,
		AlertID:    alert.ID,
		RuleID:     alert.RuleID,
		RuleName:   alert.RuleName,
		Category:   category,
		Severity:   alert.Severity,
		IsFinding:  false,
		Message:    alert.Message,
		Timestamp:  t,
	}

	return e.Evaluate(ctx, req)
}

// EvaluateIncident evaluates suppression for an incident event.
func (e *SuppressionEngine) EvaluateIncident(
	ctx context.Context,
	orgID string,
	nodeID string,
	incidentID string,
	severity model.Severity,
	category string,
	title string,
	t time.Time,
) (*model.SuppressionDecision, error) {
	req := SuppressionEvaluationRequest{
		OrgID:      orgID,
		NodeID:     nodeID,
		IncidentID: incidentID,
		Category:   category,
		Severity:   severity,
		IsFinding:  false,
		Message:    title,
		Timestamp:  t,
	}

	return e.Evaluate(ctx, req)
}

// EvaluateFinding evaluates suppression for a compliance finding.
func (e *SuppressionEngine) EvaluateFinding(
	ctx context.Context,
	orgID string,
	node *model.FleetNode,
	ownership *model.NodeOwnershipMetadata,
	groupIDs []string,
	groupPaths []string,
	finding *model.ComplianceFinding,
	t time.Time,
) (*model.SuppressionDecision, error) {
	if finding == nil {
		return nil, fmt.Errorf("%w: nil compliance finding", ErrInvalidInput)
	}

	nodeID := finding.TargetNodeID
	if node != nil {
		nodeID = node.Identity.NodeID
	}

	req := SuppressionEvaluationRequest{
		OrgID:      orgID,
		NodeID:     nodeID,
		Node:       node,
		Ownership:  ownership,
		GroupIDs:   groupIDs,
		GroupPaths: groupPaths,
		RuleID:     finding.RuleID,
		Category:   string(finding.Category),
		Severity:   finding.Severity,
		IsFinding:  true,
		Message:    finding.Message,
		Timestamp:  t,
	}

	return e.Evaluate(ctx, req)
}

// isSecurityOrIntegrityExempt checks whether an event represents a critical security or integrity issue
// that must NEVER be suppressed under any maintenance window.
func isSecurityOrIntegrityExempt(category string, ruleName string, ruleID string, message string, details map[string]string) (bool, model.SuppressionReason, string) {
	catLower := strings.ToLower(category)
	ruleLower := strings.ToLower(ruleName)
	ruleIDLower := strings.ToLower(ruleID)
	msgLower := strings.ToLower(message)

	// Explicit override/tag exemptions
	if details != nil {
		if strings.EqualFold(details["exemption"], "security") || strings.EqualFold(details["security"], "true") {
			return true, model.SuppressionReasonSecurityExemption, "security exemption flagged in event metadata"
		}
		if strings.EqualFold(details["exemption"], "integrity") || strings.EqualFold(details["integrity"], "true") {
			return true, model.SuppressionReasonIntegrityExemption, "integrity exemption flagged in event metadata"
		}
	}

	// Category check
	if strings.Contains(catLower, "security") || strings.Contains(catLower, "auth") || strings.Contains(catLower, "vulnerability") {
		return true, model.SuppressionReasonSecurityExemption, "security category events are exempt from suppression"
	}
	if strings.Contains(catLower, "integrity") || strings.Contains(catLower, "tamper") || strings.Contains(catLower, "corruption") {
		return true, model.SuppressionReasonIntegrityExemption, "integrity category events are exempt from suppression"
	}

	// Rule / Title semantic check
	securityKeywords := []string{"unauthorized", "privilege_escalation", "cve-", "tamper", "certificate_expired", "crypto_failure"}
	for _, kw := range securityKeywords {
		if strings.Contains(ruleLower, kw) || strings.Contains(ruleIDLower, kw) || strings.Contains(msgLower, kw) {
			return true, model.SuppressionReasonSecurityExemption, fmt.Sprintf("security keyword %q matches exemption policy", kw)
		}
	}

	integrityKeywords := []string{"data_corruption", "filesystem_corrupt", "checksum_mismatch", "database_corrupt"}
	for _, kw := range integrityKeywords {
		if strings.Contains(ruleLower, kw) || strings.Contains(ruleIDLower, kw) || strings.Contains(msgLower, kw) {
			return true, model.SuppressionReasonIntegrityExemption, fmt.Sprintf("integrity keyword %q matches exemption policy", kw)
		}
	}

	return false, "", ""
}

// severityRank maps a Severity to an ordinal rank for comparison.
func severityRank(s model.Severity) int {
	switch strings.ToUpper(string(s)) {
	case string(model.SeverityCritical):
		return 3
	case string(model.SeverityWarning):
		return 2
	case string(model.SeverityInfo):
		return 1
	default:
		return 0
	}
}

// generateDecisionID produces a unique deterministic or pseudo-random suppression decision identifier.
func generateDecisionID(t time.Time) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("sd-%d-%s", t.UnixNano(), hex.EncodeToString(b))
}

// newDecision creates and populates a SuppressionDecision model.
func (e *SuppressionEngine) newDecision(
	orgID string,
	req SuppressionEvaluationRequest,
	outcome model.SuppressionOutcome,
	reason model.SuppressionReason,
	windowID string,
	message string,
	evalTime time.Time,
) *model.SuppressionDecision {
	details := make(map[string]string)
	if req.CustomDetails != nil {
		for k, v := range req.CustomDetails {
			details[k] = v
		}
	}
	if req.Category != "" {
		details["category"] = req.Category
	}

	return &model.SuppressionDecision{
		ID:          generateDecisionID(evalTime),
		OrgID:       orgID,
		AlertID:     req.AlertID,
		IncidentID:  req.IncidentID,
		NodeID:      req.NodeID,
		WindowID:    windowID,
		RuleID:      req.RuleID,
		RuleName:    req.RuleName,
		Category:    req.Category,
		Severity:    req.Severity,
		Outcome:     outcome,
		Reason:      reason,
		Message:     message,
		EvaluatedAt: evalTime,
		Details:     details,
	}
}

// RecordDecision persists a suppression decision using the provided storage backend.
func (e *SuppressionEngine) RecordDecision(ctx context.Context, store storage.Storage, decision *model.SuppressionDecision) error {
	if store == nil {
		return fmt.Errorf("%w: nil storage", ErrInvalidInput)
	}
	if decision == nil {
		return fmt.Errorf("%w: nil suppression decision", ErrInvalidInput)
	}
	return store.SaveSuppressionDecision(ctx, decision)
}
