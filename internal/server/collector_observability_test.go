package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/collector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockFailingCollector struct {
	name string
}

func (m *mockFailingCollector) Name() string { return m.name }
func (m *mockFailingCollector) Collect(ctx context.Context) (any, error) {
	time.Sleep(2 * time.Millisecond)
	return nil, errors.New("simulated collection failure")
}

type mockSucceedingCollector struct {
	name string
}

func (m *mockSucceedingCollector) Name() string { return m.name }
func (m *mockSucceedingCollector) Collect(ctx context.Context) (any, error) {
	time.Sleep(1 * time.Millisecond)
	return "success", nil
}

func TestCollectorObservability_Metrics(t *testing.T) {
	mgr := collector.NewManager()
	failCol := &mockFailingCollector{name: "failing_service"}
	succCol := &mockSucceedingCollector{name: "succeeding_service"}
	mgr.Register(failCol)
	mgr.Register(succCol)

	ctx := context.Background()

	// Run multiple collection cycles
	for i := 0; i < 3; i++ {
		_, _ = mgr.CollectAll(ctx)
	}

	statuses := mgr.GetStatuses()
	require.Len(t, statuses, 2)

	var failStatus, succStatus collector.Status
	for _, st := range statuses {
		if st.Name == "failing_service" {
			failStatus = st
		} else if st.Name == "succeeding_service" {
			succStatus = st
		}
	}

	// Verify failure metrics
	assert.Equal(t, int64(3), failStatus.Failures)
	assert.Equal(t, int64(3), failStatus.TotalCount)
	assert.True(t, failStatus.HasRun)
	assert.NotEqual(t, "", failStatus.LastError)

	// Verify success metrics
	assert.Equal(t, int64(0), succStatus.Failures)
	assert.Equal(t, int64(3), succStatus.SuccessCount)
	assert.Equal(t, int64(3), succStatus.TotalCount)
	assert.True(t, succStatus.HasRun)
	assert.Equal(t, "", succStatus.LastError)

	// Test Prometheus exposition rendering
	exporter := NewPrometheusExporter()
	exporter.UpdateCollectorStatuses(statuses)
	metricsOutput := exporter.RenderMetrics()

	assert.Contains(t, metricsOutput, "watchdog_collector_failures_total{collector=\"failing_service\"} 3")
	assert.Contains(t, metricsOutput, "watchdog_collector_failures_total{collector=\"succeeding_service\"} 0")
	assert.Contains(t, metricsOutput, "watchdog_collector_up{collector=\"failing_service\"} 0")
	assert.Contains(t, metricsOutput, "watchdog_collector_up{collector=\"succeeding_service\"} 1")
	assert.Contains(t, metricsOutput, "watchdog_collector_duration_seconds_count{collector=\"failing_service\"} 3")
	assert.Contains(t, metricsOutput, "watchdog_collector_duration_seconds_count{collector=\"succeeding_service\"} 3")
}

func TestCollectorObservability_Concurrency(t *testing.T) {
	mgr := collector.NewManager()
	mgr.Register(&mockSucceedingCollector{name: "concurrent_col"})

	ctx := context.Background()
	concurrency := 10
	iterations := 20
	errChan := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			for j := 0; j < iterations; j++ {
				_, err := mgr.CollectAll(ctx)
				if err != nil {
					errChan <- err
					return
				}
			}
			errChan <- nil
		}()
	}

	for i := 0; i < concurrency; i++ {
		err := <-errChan
		assert.NoError(t, err)
	}

	statuses := mgr.GetStatuses()
	require.Len(t, statuses, 1)
	assert.Equal(t, int64(concurrency*iterations), statuses[0].TotalCount)
}
