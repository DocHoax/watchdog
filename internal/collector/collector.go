package collector

import (
	"context"
	"time"
)

// Status represents the operational status of a collector.
type Status struct {
	Name        string        `json:"name"`
	Enabled     bool          `json:"enabled"`
	LastSuccess time.Time     `json:"last_success"`
	LastError   string        `json:"last_error,omitempty"`
	Duration    time.Duration `json:"duration"`
}

// Collector is the base interface that all metric collectors must implement.
type Collector interface {
	Name() string
	Collect(ctx context.Context) (any, error)
}
