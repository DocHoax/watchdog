package governance

import "errors"

var (
	// ErrOrganizationNotFound is returned when an organization is not found.
	ErrOrganizationNotFound = errors.New("organization not found")
	// ErrOrganizationExists is returned when an organization with the same ID already exists.
	ErrOrganizationExists = errors.New("organization already exists")
	// ErrDefaultOrgImmutable is returned when attempting to delete the default organization.
	ErrDefaultOrgImmutable = errors.New("cannot delete default organization")
	// ErrFleetGroupNotFound is returned when a fleet group is not found.
	ErrFleetGroupNotFound = errors.New("fleet group not found")
	// ErrFleetGroupExists is returned when a fleet group with the same ID already exists.
	ErrFleetGroupExists = errors.New("fleet group already exists")
	// ErrCycleDetected is returned when a group hierarchy relationship would produce a cycle.
	ErrCycleDetected = errors.New("cycle detected in group hierarchy")
	// ErrParentNotFound is returned when a group's parent group does not exist.
	ErrParentNotFound = errors.New("parent group not found")
	// ErrCrossOrgParent is returned when a group specifies a parent belonging to a different organization.
	ErrCrossOrgParent = errors.New("parent group belongs to a different organization")
	// ErrNodeNotFound is returned when a requested fleet node is not found.
	ErrNodeNotFound = errors.New("fleet node not found")
	// ErrInvalidIdentifier is returned when an ID fails character/length validation.
	ErrInvalidIdentifier = errors.New("invalid identifier")
	// ErrInvalidInput is returned when input data fails semantic or structural validation.
	ErrInvalidInput = errors.New("invalid input data")

	// Policy & Compliance Errors
	// ErrPolicyNotFound is returned when a policy is not found.
	ErrPolicyNotFound = errors.New("policy not found")
	// ErrPolicyExists is returned when a policy with the same ID already exists.
	ErrPolicyExists = errors.New("policy already exists")
	// ErrPolicyRevisionNotFound is returned when a policy revision is not found.
	ErrPolicyRevisionNotFound = errors.New("policy revision not found")
	// ErrPolicyRevisionDigestMismatch is returned when a revision's computed digest does not match its record.
	ErrPolicyRevisionDigestMismatch = errors.New("policy revision digest mismatch")
	// ErrPolicyAssignmentNotFound is returned when a policy assignment is not found.
	ErrPolicyAssignmentNotFound = errors.New("policy assignment not found")
	// ErrPolicyAssignmentExists is returned when an assignment already exists.
	ErrPolicyAssignmentExists = errors.New("policy assignment already exists")
	// ErrCrossOrgAssignment is returned when attempting to assign a policy across organization boundaries.
	ErrCrossOrgAssignment = errors.New("cross-organization policy assignment forbidden")
	// ErrInvalidLifecycleTransition is returned when an illegal policy status transition is attempted.
	ErrInvalidLifecycleTransition = errors.New("invalid policy lifecycle transition")
	// ErrInvalidSelector is returned when a selector query expression is invalid.
	ErrInvalidSelector = errors.New("invalid selector expression")
	// ErrSelectorMaxDepthExceeded is returned when a selector AST exceeds the maximum nesting depth.
	ErrSelectorMaxDepthExceeded = errors.New("selector expression exceeds maximum depth")
	// ErrSelectorTooLong is returned when a selector string exceeds the maximum allowable length.
	ErrSelectorTooLong = errors.New("selector expression exceeds maximum length")
)
