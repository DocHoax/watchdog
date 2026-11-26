package collector

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProcessCollector_ProductionContract exercises Watchdog's actual ProcessCollector methods
// against gopsutil v4.25.1 to define and test production behavior.

func TestProductionContract_GetProcessesAndSummary(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewProcessCollector()
	assert.Equal(t, "process", c.Name())

	// Exercise production GetProcesses method
	summary, err := c.GetProcesses(ctx, 10, "cpu", "")
	require.NoError(t, err)
	require.NotNil(t, summary)

	assert.GreaterOrEqual(t, summary.TotalCount, 0)
	assert.NotNil(t, summary.CollectedAt)

	if summary.TotalCount > 0 {
		require.NotEmpty(t, summary.Processes)
		p := summary.Processes[0]
		assert.NotZero(t, p.PID)
		assert.NotEmpty(t, p.Status)
	}
}

func TestProductionContract_GetProcessDetails(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c := NewProcessCollector()
	currentPID := int32(os.Getpid())

	// Exercise production GetProcessDetails method (covering Nice and I/O counters in production)
	details, err := c.GetProcessDetails(ctx, currentPID)
	require.NoError(t, err)
	require.NotNil(t, details)

	assert.Equal(t, currentPID, details.PID)
	assert.NotEmpty(t, details.Name)
}

func TestProductionContract_GetProcessDetails_NotFound(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c := NewProcessCollector()
	// Use an extremely high unlikely PID
	badPID := int32(2147483647)

	details, err := c.GetProcessDetails(ctx, badPID)
	assert.Error(t, err)
	assert.Nil(t, details)
}

// Limitation Note:
// Under the strict test-only authorization (no production changes, no addition of unexported test hooks/seams
// like ProcfsReader interfaces in production), deterministic unit testing of low-level procfs helper functions
// (such as internal parseStat with spaces/parentheses, exact bitwise CPU tick accumulation without gopsutil,
// or injectable mock process readers) is currently blocked because production ProcessCollector tightly couples
// directly to gopsutil/v4/process global functions without an injectable reader abstraction.
// Therefore, low-level parser and error-policy unit tests require production refactoring (such as Phase 2F.2.6's reader abstraction)
// which is strictly outside the current test-only authorization.
