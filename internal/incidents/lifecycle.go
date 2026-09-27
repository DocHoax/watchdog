package incidents

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidStatus indicates an unknown incident status string.
	ErrInvalidStatus = errors.New("invalid incident status")
	// ErrInvalidTransition indicates an illegal transition between incident states.
	ErrInvalidTransition = errors.New("illegal incident status transition")
)

// StatusTransitionError provides detailed context on an invalid state transition.
type StatusTransitionError struct {
	From IncidentStatus
	To   IncidentStatus
}

func (e *StatusTransitionError) Error() string {
	return fmt.Sprintf("cannot transition incident status from '%s' to '%s'", e.From, e.To)
}

func (e *StatusTransitionError) Unwrap() error {
	return ErrInvalidTransition
}

// validTransitions defines the formal state machine transition matrix.
var validTransitions = map[IncidentStatus]map[IncidentStatus]bool{
	IncidentStatusDetected: {
		IncidentStatusAcknowledged:  true,
		IncidentStatusInvestigating: true,
		IncidentStatusResolved:      true,
		IncidentStatusSuppressed:    true,
	},
	IncidentStatusAcknowledged: {
		IncidentStatusInvestigating: true,
		IncidentStatusResolved:      true,
		IncidentStatusSuppressed:    true,
	},
	IncidentStatusInvestigating: {
		IncidentStatusAcknowledged:  true,
		IncidentStatusResolved:      true,
		IncidentStatusSuppressed:    true,
	},
	IncidentStatusResolved: {
		IncidentStatusReopened: true,
	},
	IncidentStatusSuppressed: {
		IncidentStatusReopened:      true,
		IncidentStatusAcknowledged:  true,
		IncidentStatusInvestigating: true,
	},
	IncidentStatusReopened: {
		IncidentStatusAcknowledged:  true,
		IncidentStatusInvestigating: true,
		IncidentStatusResolved:      true,
		IncidentStatusSuppressed:    true,
	},
}

// IsValidStatus validates whether a given status string is a recognized IncidentStatus.
func IsValidStatus(status IncidentStatus) bool {
	switch status {
	case IncidentStatusDetected,
		IncidentStatusAcknowledged,
		IncidentStatusInvestigating,
		IncidentStatusResolved,
		IncidentStatusSuppressed,
		IncidentStatusReopened:
		return true
	default:
		return false
	}
}

// ValidateTransition checks if transitioning from current to next status is permitted.
func ValidateTransition(current, next IncidentStatus) error {
	if !IsValidStatus(current) {
		return fmt.Errorf("%w: '%s'", ErrInvalidStatus, current)
	}
	if !IsValidStatus(next) {
		return fmt.Errorf("%w: '%s'", ErrInvalidStatus, next)
	}

	if current == next {
		return nil // idempotent no-op transition
	}

	allowedNext, exists := validTransitions[current]
	if !exists || !allowedNext[next] {
		return &StatusTransitionError{From: current, To: next}
	}

	return nil
}

// AllowedTransitions returns the list of valid next states for a given status.
func AllowedTransitions(status IncidentStatus) []IncidentStatus {
	allowed, exists := validTransitions[status]
	if !exists {
		return nil
	}

	var results []IncidentStatus
	for next := range allowed {
		results = append(results, next)
	}
	return results
}

// IsActiveStatus returns true if an incident in this state requires operational attention.
func IsActiveStatus(status IncidentStatus) bool {
	switch status {
	case IncidentStatusDetected,
		IncidentStatusAcknowledged,
		IncidentStatusInvestigating,
		IncidentStatusReopened:
		return true
	default:
		return false
	}
}

// IsTerminalStatus returns true if an incident in this state is considered inactive/closed.
func IsTerminalStatus(status IncidentStatus) bool {
	return status == IncidentStatusResolved || status == IncidentStatusSuppressed
}
