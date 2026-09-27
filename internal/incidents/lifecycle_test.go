package incidents

import (
	"errors"
	"testing"
)

func TestValidateTransition(t *testing.T) {
	tests := []struct {
		name      string
		current   IncidentStatus
		target    IncidentStatus
		wantError bool
	}{
		// Valid transitions from Detected
		{"Detected to Acknowledged", IncidentStatusDetected, IncidentStatusAcknowledged, false},
		{"Detected to Investigating", IncidentStatusDetected, IncidentStatusInvestigating, false},
		{"Detected to Resolved", IncidentStatusDetected, IncidentStatusResolved, false},
		{"Detected to Suppressed", IncidentStatusDetected, IncidentStatusSuppressed, false},
		{"Detected to Reopened (invalid)", IncidentStatusDetected, IncidentStatusReopened, true},
		{"Detected to Detected (invalid self)", IncidentStatusDetected, IncidentStatusDetected, true},

		// Valid transitions from Acknowledged
		{"Acknowledged to Investigating", IncidentStatusAcknowledged, IncidentStatusInvestigating, false},
		{"Acknowledged to Resolved", IncidentStatusAcknowledged, IncidentStatusResolved, false},
		{"Acknowledged to Suppressed", IncidentStatusAcknowledged, IncidentStatusSuppressed, false},
		{"Acknowledged to Detected (invalid)", IncidentStatusAcknowledged, IncidentStatusDetected, true},
		{"Acknowledged to Reopened (invalid)", IncidentStatusAcknowledged, IncidentStatusReopened, true},

		// Valid transitions from Investigating
		{"Investigating to Resolved", IncidentStatusInvestigating, IncidentStatusResolved, false},
		{"Investigating to Suppressed", IncidentStatusInvestigating, IncidentStatusSuppressed, false},
		{"Investigating to Acknowledged (invalid)", IncidentStatusInvestigating, IncidentStatusAcknowledged, true},
		{"Investigating to Detected (invalid)", IncidentStatusInvestigating, IncidentStatusDetected, true},

		// Valid transitions from Resolved
		{"Resolved to Reopened", IncidentStatusResolved, IncidentStatusReopened, false},
		{"Resolved to Investigating (invalid)", IncidentStatusResolved, IncidentStatusInvestigating, true},
		{"Resolved to Detected (invalid)", IncidentStatusResolved, IncidentStatusDetected, true},

		// Valid transitions from Suppressed
		{"Suppressed to Reopened", IncidentStatusSuppressed, IncidentStatusReopened, false},
		{"Suppressed to Acknowledged", IncidentStatusSuppressed, IncidentStatusAcknowledged, false},
		{"Suppressed to Investigating", IncidentStatusSuppressed, IncidentStatusInvestigating, false},
		{"Suppressed to Detected (invalid)", IncidentStatusSuppressed, IncidentStatusDetected, true},
		{"Suppressed to Resolved (invalid)", IncidentStatusSuppressed, IncidentStatusResolved, true},

		// Valid transitions from Reopened
		{"Reopened to Investigating", IncidentStatusReopened, IncidentStatusInvestigating, false},
		{"Reopened to Resolved", IncidentStatusReopened, IncidentStatusResolved, false},
		{"Reopened to Suppressed", IncidentStatusReopened, IncidentStatusSuppressed, false},
		{"Reopened to Detected (invalid)", IncidentStatusReopened, IncidentStatusDetected, true},

		// Unknown/Invalid status
		{"Invalid current status", IncidentStatus("foo"), IncidentStatusResolved, true},
		{"Invalid target status", IncidentStatusDetected, IncidentStatus("bar"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTransition(tt.current, tt.target)
			if (err != nil) != tt.wantError {
				t.Fatalf("ValidateTransition(%q, %q) error = %v, wantError %v", tt.current, tt.target, err, tt.wantError)
			}
			if tt.wantError && err != nil {
				var transErr *StatusTransitionError
				if errors.As(err, &transErr) {
					if !errors.Is(err, ErrInvalidTransition) {
						t.Errorf("expected StatusTransitionError to unwrap to ErrInvalidTransition")
					}
					if transErr.From != tt.current || transErr.To != tt.target {
						t.Errorf("StatusTransitionError fields mismatch: got (%v, %v), want (%v, %v)", transErr.From, transErr.To, tt.current, tt.target)
					}
					if transErr.Error() == "" {
						t.Errorf("StatusTransitionError.Error() should not be empty")
					}
				}
			}
		})
	}
}

func TestIsValidStatus(t *testing.T) {
	validStatuses := []IncidentStatus{
		IncidentStatusDetected,
		IncidentStatusAcknowledged,
		IncidentStatusInvestigating,
		IncidentStatusResolved,
		IncidentStatusSuppressed,
		IncidentStatusReopened,
	}

	for _, s := range validStatuses {
		if !IsValidStatus(s) {
			t.Errorf("expected %q to be a valid status", s)
		}
	}

	invalidStatuses := []IncidentStatus{
		"",
		"active",
		"closed",
		"pending",
		"unknown",
	}

	for _, s := range invalidStatuses {
		if IsValidStatus(s) {
			t.Errorf("expected %q to be an invalid status", s)
		}
	}
}
