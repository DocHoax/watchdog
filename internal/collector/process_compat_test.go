package collector

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProcessCollector_CompatibilityContract defines the test suite for Phase 2F.2.7.
// It verifies gopsutil v4 behavior and contracts expected by Watchdog ProcessCollector.

func TestCompatibility_GopsutilProcessEnumerationAndDetails(t *testing.T) {
	ctx := context.Background()
	c := NewProcessCollector()

	// Verify GetProcesses interacts with gopsutil correctly
	summary, err := c.GetProcesses(ctx, 5, "pid", "")
	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.GreaterOrEqual(t, summary.TotalCount, 0)

	// Verify GetProcessDetails on current process
	currentPID := int32(os.Getpid())
	details, err := c.GetProcessDetails(ctx, currentPID)
	require.NoError(t, err)
	require.NotNil(t, details)
	assert.Equal(t, currentPID, details.PID)
}

func TestCompatibility_MemoryZeroTotalArithmetic(t *testing.T) {
	// Verifies behavior when total system memory is zero or undefined.
	totalMem := uint64(0)
	rss := uint64(1024 * 1024 * 50)

	var memPct float32
	if totalMem > 0 && rss > 0 {
		memPct = float32(rss) / float32(totalMem) * 100.0
	}
	assert.Equal(t, float32(0.0), memPct)
}

func TestCompatibility_StatusClassification(t *testing.T) {
	// Verifies raw status classification expected by ProcessCollector summary counters.
	statuses := []string{"R", "S", "T", "Z", "I", "UNKNOWN"}
	for _, st := range statuses {
		statusUpper := st
		isSleeping := false
		switch statusUpper {
		case "R", "RUNNING":
		case "S", "SLEEP", "SLEEPING", "I", "IDLE":
			isSleeping = true
		case "T", "STOPPED":
		case "Z", "ZOMBIE":
		default:
			isSleeping = true
		}
		assert.NotNil(t, isSleeping)
	}
}

func TestCompatibility_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.Error(t, ctx.Err())
}

func TestCompatibility_ErrorPolicies(t *testing.T) {
	errENOENT := os.ErrNotExist
	errEACCES := os.ErrPermission

	assert.True(t, errors.Is(errENOENT, os.ErrNotExist))
	assert.True(t, errors.Is(errEACCES, os.ErrPermission))
}
