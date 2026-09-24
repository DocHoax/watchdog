package collector

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

type failingMockCollector struct {
	name string
	err  error
}

func (f *failingMockCollector) Name() string {
	return f.name
}

func (f *failingMockCollector) Collect(ctx context.Context) (any, error) {
	return nil, f.err
}

type healthyMockCollector struct {
	name string
	res  *model.CPUInfo
}

func (h *healthyMockCollector) Name() string {
	return h.name
}

func (h *healthyMockCollector) Collect(ctx context.Context) (any, error) {
	return h.res, nil
}

type hangingCollector struct {
	name string
}

func (h *hangingCollector) Name() string {
	return h.name
}

func (h *hangingCollector) Collect(ctx context.Context) (any, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(10 * time.Second):
		return nil, nil
	}
}

// TestCollectorGracefulDegradation verifies that manager handles individual collector failures gracefully.
func TestCollectorGracefulDegradation(t *testing.T) {
	mgr := NewManager()

	// Register 1 healthy collector and 2 failing collectors
	healthy := &healthyMockCollector{
		name: "cpu",
		res: &model.CPUInfo{
			OverallUsage: 42.5,
			LogicalCores: 8,
		},
	}
	failing1 := &failingMockCollector{
		name: "disk",
		err:  errors.New("disk permission denied (EACCES)"),
	}
	failing2 := &failingMockCollector{
		name: "docker",
		err:  errors.New("docker daemon socket not found: connection refused"),
	}

	mgr.Register(healthy)
	mgr.Register(failing1)
	mgr.Register(failing2)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	snap, err := mgr.CollectAll(ctx)
	if err != nil {
		t.Fatalf("CollectAll should not fail when some collectors error: %v", err)
	}

	if snap == nil {
		t.Fatalf("expected non-nil snapshot")
	}

	if snap.CPU == nil || snap.CPU.OverallUsage != 42.5 {
		t.Errorf("expected healthy CPU collector result to be populated in snapshot")
	}

	if snap.Disk != nil {
		t.Errorf("expected failed Disk collector result to be nil")
	}

	// Verify statuses
	statuses := mgr.GetStatuses()
	statusMap := make(map[string]Status)
	for _, s := range statuses {
		statusMap[s.Name] = s
	}

	if statusMap["disk"].LastError == "" {
		t.Errorf("expected disk collector to have LastError recorded")
	}
	if statusMap["docker"].LastError == "" {
		t.Errorf("expected docker collector to have LastError recorded")
	}
	if statusMap["cpu"].LastError != "" {
		t.Errorf("expected cpu collector to have no error, got: %s", statusMap["cpu"].LastError)
	}
}

// TestCollectorContextCancellation verifies that slow or hanging collectors terminate on context cancellation.
func TestCollectorContextCancellation(t *testing.T) {
	mgr := NewManager()
	mgr.Register(&hangingCollector{name: "hanging-probe"})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, _ = mgr.CollectAll(ctx)
	elapsed := time.Since(start)

	if elapsed > 1*time.Second {
		t.Errorf("CollectAll took %v, context cancellation did not terminate in-flight collector promptly", elapsed)
	}
}
