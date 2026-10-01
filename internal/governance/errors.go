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
)
