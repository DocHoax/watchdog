package collector

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockPanicCollector struct {
	name     string
	panicVal any
}

func (m *mockPanicCollector) Name() string { return m.name }
func (m *mockPanicCollector) Collect(ctx context.Context) (any, error) {
	panic(m.panicVal)
}

type mockNilResultCollector struct {
	name string
}

func (m *mockNilResultCollector) Name() string { return m.name }
func (m *mockNilResultCollector) Collect(ctx context.Context) (any, error) {
	return nil, nil
}

func TestSystemCollector(t *testing.T) {
	c := NewSystemCollector()
	if c.Name() != "system" {
		t.Fatalf("expected collector name 'system', got '%s'", c.Name())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sys, err := c.GetSystemInfo(ctx)
	if err != nil {
		t.Fatalf("unexpected error collecting system info: %v", err)
	}

	if sys.Hostname == "" {
		t.Errorf("expected non-empty hostname")
	}
	if sys.OS == "" {
		t.Errorf("expected non-empty OS")
	}
	if sys.KernelArch == "" {
		t.Errorf("expected non-empty KernelArch")
	}

	res, err := c.Collect(ctx)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}
	if _, ok := res.(*model.SystemInfo); !ok {
		t.Errorf("expected *model.SystemInfo from Collect(), got %T", res)
	}
}

func TestCPUCollector(t *testing.T) {
	c := NewCPUCollector()
	if c.Name() != "cpu" {
		t.Fatalf("expected collector name 'cpu', got '%s'", c.Name())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cpuInfo, err := c.GetCPUInfo(ctx)
	if err != nil {
		t.Fatalf("unexpected error collecting CPU info: %v", err)
	}

	if cpuInfo.PhysicalCores <= 0 && cpuInfo.LogicalCores <= 0 {
		t.Errorf("expected positive core count, got phys=%d, log=%d", cpuInfo.PhysicalCores, cpuInfo.LogicalCores)
	}

	// Verify load average is calculated and non-negative
	if cpuInfo.LoadAverage.Load1 < 0 || cpuInfo.LoadAverage.Load5 < 0 || cpuInfo.LoadAverage.Load15 < 0 {
		t.Errorf("expected non-negative load average, got: %+v", cpuInfo.LoadAverage)
	}
}

func TestMemoryCollector(t *testing.T) {
	c := NewMemoryCollector()
	if c.Name() != "memory" {
		t.Fatalf("expected collector name 'memory', got '%s'", c.Name())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	memInfo, err := c.GetMemoryInfo(ctx)
	if err != nil {
		t.Fatalf("unexpected error collecting memory info: %v", err)
	}

	if memInfo.TotalBytes == 0 {
		t.Errorf("expected non-zero total memory")
	}
	if memInfo.UsedPercent < 0 || memInfo.UsedPercent > 100 {
		t.Errorf("invalid memory percent: %f", memInfo.UsedPercent)
	}
}

func TestDiskCollector(t *testing.T) {
	c := NewDiskCollector()
	if c.Name() != "disk" {
		t.Fatalf("expected collector name 'disk', got '%s'", c.Name())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	diskInfo, err := c.GetDiskInfo(ctx)
	if err != nil {
		t.Fatalf("unexpected error collecting disk info: %v", err)
	}

	if len(diskInfo.Partitions) == 0 {
		t.Logf("no partitions returned (could happen in restricted environments)")
	}
}

func TestNetworkCollector(t *testing.T) {
	c := NewNetworkCollector()
	if c.Name() != "network" {
		t.Fatalf("expected collector name 'network', got '%s'", c.Name())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	netInfo, err := c.GetNetworkInfo(ctx)
	if err != nil {
		t.Fatalf("unexpected error collecting network info: %v", err)
	}

	if len(netInfo.Interfaces) == 0 {
		t.Logf("no interfaces returned")
	}
}

func TestProcessCollector(t *testing.T) {
	c := NewProcessCollector()
	if c.Name() != "process" {
		t.Fatalf("expected collector name 'process', got '%s'", c.Name())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	procSummary, err := c.GetProcesses(ctx, 10, "cpu", "")
	if err != nil {
		t.Fatalf("unexpected error collecting processes: %v", err)
	}

	if procSummary.TotalCount <= 0 {
		t.Errorf("expected positive process count, got %d", procSummary.TotalCount)
	}

	// Test getting current process details
	currentPID := int32(os.Getpid())
	procDetail, err := c.GetProcessDetails(ctx, currentPID)
	if err != nil {
		t.Fatalf("failed to get details for current process: %v", err)
	}
	if procDetail.PID != currentPID {
		t.Errorf("expected PID %d, got %d", currentPID, procDetail.PID)
	}

	// Test process tree
	tree, err := c.GetProcessTree(ctx)
	if err != nil {
		t.Fatalf("failed to build process tree: %v", err)
	}
	if tree == nil {
		t.Errorf("expected non-nil process tree")
	}
}

func TestPortCollector(t *testing.T) {
	c := NewPortCollector()
	if c.Name() != "port" {
		t.Fatalf("expected collector name 'port', got '%s'", c.Name())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ports, err := c.GetListeningPorts(ctx)
	if err != nil {
		t.Fatalf("unexpected error collecting ports: %v", err)
	}
	t.Logf("collected %d listening ports", len(ports))
}

func TestDockerCollectorGracefulFallback(t *testing.T) {
	c := NewDockerCollector()
	if c.Name() != "docker" {
		t.Fatalf("expected collector name 'docker', got '%s'", c.Name())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dockerInfo, err := c.GetDockerSummary(ctx)
	if err != nil {
		t.Fatalf("unexpected fatal error from docker collector: %v", err)
	}
	if dockerInfo == nil {
		t.Fatalf("expected non-nil dockerInfo")
	}
	t.Logf("docker available: %v, total containers: %d", dockerInfo.Available, dockerInfo.ContainersTotal)
}

func TestKubernetesCollectorGracefulFallback(t *testing.T) {
	c := NewKubernetesCollector()
	if c.Name() != "kubernetes" {
		t.Fatalf("expected collector name 'kubernetes', got '%s'", c.Name())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	k8sInfo, err := c.GetClusterSummary(ctx)
	if err != nil {
		t.Fatalf("unexpected fatal error from k8s collector: %v", err)
	}
	if k8sInfo == nil {
		t.Fatalf("expected non-nil k8sInfo")
	}
	t.Logf("k8s available: %v, total nodes: %d", k8sInfo.Available, k8sInfo.TotalNodes)
}

func TestCollectorManager(t *testing.T) {
	mgr := NewManager()

	mgr.Register(NewSystemCollector())
	mgr.Register(NewCPUCollector())
	mgr.Register(NewMemoryCollector())
	mgr.Register(NewDiskCollector())
	mgr.Register(NewNetworkCollector())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	snapshot, err := mgr.CollectAll(ctx)
	if err != nil {
		t.Fatalf("CollectAll failed: %v", err)
	}

	if snapshot.System == nil {
		t.Errorf("expected System in snapshot")
	}
	if snapshot.CPU == nil {
		t.Errorf("expected CPU in snapshot")
	}
	if snapshot.Memory == nil {
		t.Errorf("expected Memory in snapshot")
	}
	if snapshot.Disk == nil {
		t.Errorf("expected Disk in snapshot")
	}
	if snapshot.Network == nil {
		t.Errorf("expected Network in snapshot")
	}
}

// Additional comprehensive test coverage for Phase 2F.3.2 Requirements

type mockFailingCollector struct {
	name string
	err  error
}

func (m *mockFailingCollector) Name() string { return m.name }
func (m *mockFailingCollector) Collect(ctx context.Context) (any, error) {
	return nil, m.err
}

type mockSuccessCollector struct {
	name string
	res  any
}

func (m *mockSuccessCollector) Name() string { return m.name }
func (m *mockSuccessCollector) Collect(ctx context.Context) (any, error) {
	return m.res, nil
}

type mockSequenceCollector struct {
	name    string
	calls   int
	panicOn int
	res     any
}

func (m *mockSequenceCollector) Name() string { return m.name }
func (m *mockSequenceCollector) Collect(ctx context.Context) (any, error) {
	m.calls++
	if m.calls == m.panicOn {
		panic("sequential boom")
	}
	return m.res, nil
}

func TestCollectorManager_MultipleIndependentPanicsAndSuccess(t *testing.T) {
	mgr := NewManager()

	p1 := &mockPanicCollector{name: "panic_1", panicVal: "boom1"}
	p2 := &mockPanicCollector{name: "panic_2", panicVal: "boom2"}
	succ := &mockSuccessCollector{name: "succ", res: &model.SystemInfo{Hostname: "multi-ok"}}

	mgr.Register(p1)
	mgr.Register(p2)
	mgr.Register(succ)

	ctx := context.Background()
	snapshot, err := mgr.CollectAll(ctx)
	require.NoError(t, err)
	require.NotNil(t, snapshot)

	// Verify successful collector data contributed to snapshot
	require.NotNil(t, snapshot.System)
	assert.Equal(t, "multi-ok", snapshot.System.Hostname)

	statuses := mgr.GetStatuses()
	require.Len(t, statuses, 3)

	var st1, st2, stSucc Status
	for _, st := range statuses {
		switch st.Name {
		case "panic_1":
			st1 = st
		case "panic_2":
			st2 = st
		case "succ":
			stSucc = st
		}
	}

	assert.Equal(t, int64(1), st1.Failures)
	assert.Equal(t, "collector panic (details available in internal logs)", st1.LastError)
	assert.NotContains(t, st1.LastError, "boom1")
	assert.NotContains(t, st1.LastError, "goroutine ")

	assert.Equal(t, int64(1), st2.Failures)
	assert.Equal(t, "collector panic (details available in internal logs)", st2.LastError)
	assert.NotContains(t, st2.LastError, "boom2")
	assert.NotContains(t, st2.LastError, "goroutine ")

	assert.Equal(t, int64(0), stSucc.Failures)
	assert.Equal(t, int64(1), stSucc.SuccessCount)
}

func TestCollectorManager_LastSuccessPreservationAfterPanic(t *testing.T) {
	mgr := NewManager()

	seq := &mockSequenceCollector{
		name:    "seq_col",
		panicOn: 2,
		res:     &model.SystemInfo{Hostname: "seq-host"},
	}
	mgr.Register(seq)

	ctx := context.Background()

	// 1st invocation: Success
	_, err := mgr.CollectAll(ctx)
	require.NoError(t, err)

	statuses1 := mgr.GetStatuses()
	require.Len(t, statuses1, 1)
	firstSuccessTime := statuses1[0].LastSuccess
	assert.False(t, firstSuccessTime.IsZero())
	assert.Equal(t, int64(1), statuses1[0].SuccessCount)
	assert.Equal(t, int64(0), statuses1[0].Failures)

	// Sleep briefly to ensure time advances if LastSuccess were incorrectly updated
	time.Sleep(10 * time.Millisecond)

	// 2nd invocation: Panic
	_, err = mgr.CollectAll(ctx)
	require.NoError(t, err)

	statuses2 := mgr.GetStatuses()
	require.Len(t, statuses2, 1)
	assert.Equal(t, firstSuccessTime, statuses2[0].LastSuccess, "LastSuccess must remain unchanged after a panic")
	assert.Equal(t, int64(1), statuses2[0].Failures)
	assert.Equal(t, int64(1), statuses2[0].SuccessCount)
	assert.Equal(t, "collector panic (details available in internal logs)", statuses2[0].LastError)
}

func TestCollectorManager_HistogramBucketAndDurationTelemetry(t *testing.T) {
	mgr := NewManager()
	mgr.Register(&mockSuccessCollector{name: "timed_succ", res: &model.SystemInfo{Hostname: "time-host"}})

	ctx := context.Background()
	_, err := mgr.CollectAll(ctx)
	require.NoError(t, err)

	statuses := mgr.GetStatuses()
	require.Len(t, statuses, 1)
	st := statuses[0]

	assert.True(t, st.HasRun)
	assert.Equal(t, int64(1), st.TotalCount)
	assert.Equal(t, int64(0), st.Failures)
	assert.True(t, st.Duration > 0)
	assert.Equal(t, st.Duration, st.TotalDuration)

	// Verify exact cumulative histogram bucket semantics based on actual duration and defined bounds:
	// Bounds: [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0]
	// Cumulative histogram: every bucket <= duration is incremented, plus index 11 (+Inf bucket).
	durSec := st.Duration.Seconds()
	bounds := [11]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0}

	expectedFiniteMatches := 0
	for _, b := range bounds {
		if durSec <= b {
			expectedFiniteMatches++
		}
	}

	// Check each individual cumulative bucket explicitly
	var totalBuckets int64
	for i, b := range bounds {
		if durSec <= b {
			assert.Equal(t, int64(1), st.Buckets[i], "expected bucket index %d (bound %f) to be incremented", i, b)
		} else {
			assert.Equal(t, int64(0), st.Buckets[i], "expected bucket index %d (bound %f) NOT to be incremented", i, b)
		}
		totalBuckets += st.Buckets[i]
	}
	// Index 11 is the +Inf bucket, which always increments.
	assert.Equal(t, int64(1), st.Buckets[11], "expected +Inf bucket (index 11) to be incremented")
	totalBuckets += st.Buckets[11]

	assert.Equal(t, int64(expectedFiniteMatches+1), totalBuckets, "total bucket increments must equal matching finite buckets + Inf bucket")
}

func TestServer_DiagnosticStatusSerializationDoesNotLeakStackTracesOrRawValues(t *testing.T) {
	// Verify that the status serialization containing collector status
	// includes sanitized safe message LastError but never contains raw sensitive panic values or stack traces.
	mgr := NewManager()
	mgr.Register(&mockPanicCollector{name: "http_panic", panicVal: "secret sensitive credential 0xdeadbeef"})

	ctx := context.Background()
	_, err := mgr.CollectAll(ctx)
	require.NoError(t, err)

	statuses := mgr.GetStatuses()
	require.Len(t, statuses, 1)

	// Simulate serialization of statuses (such as via JSON endpoint / GetStatuses)
	serialized, err := json.Marshal(statuses)
	require.NoError(t, err)
	jsonStr := string(serialized)

	// Assert sanitized safe message is present
	assert.Contains(t, jsonStr, "collector panic (details available in internal logs)")
	// Assert raw sensitive panic value is strictly absent
	assert.NotContains(t, jsonStr, "secret sensitive credential 0xdeadbeef")
	// Assert stack trace headers / file paths from debug.Stack() are completely absent from serialized output
	assert.NotContains(t, jsonStr, "goroutine ")
	assert.NotContains(t, jsonStr, "runtime/debug.Stack")
	assert.NotContains(t, jsonStr, "collector.go")
}

func TestCollectorManager_SnapshotIntegrityWithMixedCollectors(t *testing.T) {
	mgr := NewManager()
	mgr.Register(&mockSuccessCollector{name: "sys", res: &model.SystemInfo{Hostname: "int-host"}})
	mgr.Register(&mockPanicCollector{name: "panic_cpu", panicVal: "cpu panic"})
	mgr.Register(&mockSuccessCollector{name: "mem", res: &model.MemoryInfo{TotalBytes: 1024}})

	ctx := context.Background()
	snapshot, err := mgr.CollectAll(ctx)
	require.NoError(t, err)
	require.NotNil(t, snapshot)

	// Verify successful collectors populated their respective fields
	require.NotNil(t, snapshot.System)
	assert.Equal(t, "int-host", snapshot.System.Hostname)
	require.NotNil(t, snapshot.Memory)
	assert.Equal(t, uint64(1024), snapshot.Memory.TotalBytes)

	// Verify panicking collector left its field (CPU) strictly nil (distinguishable from zero value / unpopulated)
	assert.Nil(t, snapshot.CPU)
}

func TestCollectorManager_PanicIsolationAndTelemetry(t *testing.T) {
	mgr := NewManager()

	panicCol := &mockPanicCollector{name: "panic_col", panicVal: "boom"}
	succCol := &mockSuccessCollector{name: "succ_col", res: &model.SystemInfo{Hostname: "ok-host"}}
	errCol := &mockFailingCollector{name: "err_col", err: errors.New("standard error")}
	nilCol := &mockNilResultCollector{name: "nil_col"}

	mgr.Register(panicCol)
	mgr.Register(succCol)
	mgr.Register(errCol)
	mgr.Register(nilCol)

	ctx := context.Background()
	snapshot, err := mgr.CollectAll(ctx)
	require.NoError(t, err)
	require.NotNil(t, snapshot)

	// Verify successful collector data survived
	require.NotNil(t, snapshot.System)
	assert.Equal(t, "ok-host", snapshot.System.Hostname)

	statuses := mgr.GetStatuses()
	require.Len(t, statuses, 4)

	var panicStatus Status
	for _, st := range statuses {
		if st.Name == "panic_col" {
			panicStatus = st
		}
	}

	assert.True(t, panicStatus.HasRun)
	assert.Equal(t, int64(1), panicStatus.Failures)
	assert.Equal(t, int64(0), panicStatus.SuccessCount)
	assert.Equal(t, int64(1), panicStatus.TotalCount)
	assert.Equal(t, "collector panic (details available in internal logs)", panicStatus.LastError)
	assert.NotContains(t, panicStatus.LastError, "boom")
	assert.NotContains(t, panicStatus.LastError, "goroutine ")
}

func TestCollectorManager_PreCancelledContext(t *testing.T) {
	mgr := NewManager()
	mgr.Register(&mockSuccessCollector{name: "succ", res: &model.SystemInfo{}})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	snapshot, err := mgr.CollectAll(ctx)
	require.NoError(t, err)
	require.NotNil(t, snapshot)
}

func TestCollectorManager_ConcurrentCollectAllWithPanics(t *testing.T) {
	mgr := NewManager()
	mgr.Register(&mockPanicCollector{name: "panic", panicVal: "concurrent boom"})
	mgr.Register(&mockSuccessCollector{name: "succ", res: &model.SystemInfo{Hostname: "concurrent-ok"}})

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
	var panicSt Status
	for _, st := range statuses {
		if st.Name == "panic" {
			panicSt = st
		}
	}
	assert.Equal(t, int64(concurrency*iterations), panicSt.Failures)
	assert.Equal(t, int64(concurrency*iterations), panicSt.TotalCount)
}

func TestCollectorManager_PanicNil(t *testing.T) {
	mgr := NewManager()
	mgr.Register(&mockPanicCollector{name: "panic_nil", panicVal: nil})

	ctx := context.Background()
	snapshot, err := mgr.CollectAll(ctx)
	require.NoError(t, err)
	require.NotNil(t, snapshot)

	statuses := mgr.GetStatuses()
	require.Len(t, statuses, 1)
	assert.Equal(t, int64(1), statuses[0].Failures)
	assert.Equal(t, "collector panic (details available in internal logs)", statuses[0].LastError)
}
