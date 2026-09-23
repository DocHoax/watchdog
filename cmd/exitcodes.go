package cmd

import (
	"errors"
	"fmt"
	"strings"
)

// Standard CLI Exit Codes
const (
	// ExitSuccess indicates clean execution and healthy system status.
	ExitSuccess = 0
	// ExitGeneralError indicates a general runtime failure, critical diagnostic defect, or anomaly.
	ExitGeneralError = 1
	// ExitUsageError indicates invalid CLI arguments, flags, or syntax.
	ExitUsageError = 2
	// ExitConfigError indicates invalid or unparseable configuration settings.
	ExitConfigError = 3
	// ExitAuthError indicates permission denied or authentication failure.
	ExitAuthError = 4
	// ExitNetworkError indicates network unreachable, DNS failure, or remote agent connection error.
	ExitNetworkError = 5
)

// ExitCodeError wraps an error with an explicit numeric exit code.
type ExitCodeError struct {
	Code int
	Err  error
}

// Error implements the standard error interface.
func (e *ExitCodeError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("exit status %d", e.Code)
}

// Unwrap returns the underlying error.
func (e *ExitCodeError) Unwrap() error {
	return e.Err
}

// NewExitError constructs an ExitCodeError with a specific exit code and message.
func NewExitError(code int, format string, a ...interface{}) error {
	return &ExitCodeError{
		Code: code,
		Err:  fmt.Errorf(format, a...),
	}
}

// WrapExitError wraps an existing error with a specific exit code.
func WrapExitError(code int, err error) error {
	if err == nil {
		return nil
	}
	return &ExitCodeError{
		Code: code,
		Err:  err,
	}
}

// GetExitCode extracts the exit code from an error or returns ExitGeneralError (1) by default.
// It also intelligently categorizes standard Cobra/pflag CLI usage errors as ExitUsageError (2).
func GetExitCode(err error) int {
	if err == nil {
		return ExitSuccess
	}
	var exitErr *ExitCodeError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}

	msg := err.Error()
	// Standard cobra and pflag error detection
	if strings.Contains(msg, "unknown flag") ||
		strings.Contains(msg, "unknown shorthand flag") ||
		strings.Contains(msg, "flag needs an argument") ||
		strings.Contains(msg, "invalid argument") ||
		strings.Contains(msg, "unknown command") ||
		strings.Contains(msg, "accepts ") ||
		strings.Contains(msg, "requires at least") ||
		strings.Contains(msg, "requires at most") ||
		strings.Contains(msg, "exact ") ||
		strings.Contains(msg, "only valid args") {
		return ExitUsageError
	}

	return ExitGeneralError
}
