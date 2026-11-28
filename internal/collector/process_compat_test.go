package collector

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock processProvider and processHandle for deterministic unit testing.

type mockProcessProvider struct {
	processes []processHandle
	listErr   error
	procMap   map[int32]processHandle
	getErr    error
}

func (m *mockProcessProvider) ProcessesWithContext(ctx context.Context) ([]processHandle, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.processes, nil
}

func (m *mockProcessProvider) NewProcessWithContext(ctx context.Context, pid int32) (processHandle, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if p, ok := m.procMap[pid]; ok {
		return p, nil
	}
	return nil, os.ErrNotExist
}

type mockProcessHandle struct {
	pid        int32
	ppid       int32
	name       string
	cpuPct     float64
	memPct     float32
	memInfo    *process.MemoryInfoStat
	numThreads int32
	username   string
	status     []string
	createTime int64
	cmdline    string
	exe        string
	nice       int32
	ioCounters *process.IOCountersStat

	nameErr    error
	statusErr  error
	memInfoErr error
	ioCountErr error
	cpuErr     error
	memPctErr  error

	nameCallCount int
	onName        func()
}

func (h *mockProcessHandle) PID() int32 { return h.pid }

func (h *mockProcessHandle) NameWithContext(ctx context.Context) (string, error) {
	h.nameCallCount++
	if h.onName != nil {
		h.onName()
	}
	if h.nameErr != nil {
		return "", h.nameErr
	}
	return h.name, nil
}

func (h *mockProcessHandle) PpidWithContext(ctx context.Context) (int32, error) {
	return h.ppid, nil
}

func (h *mockProcessHandle) CPUPercentWithContext(ctx context.Context) (float64, error) {
	if h.cpuErr != nil {
		return 0, h.cpuErr
	}
	return h.cpuPct, nil
}

func (h *mockProcessHandle) MemoryPercentWithContext(ctx context.Context) (float32, error) {
	if h.memPctErr != nil {
		return 0, h.memPctErr
	}
	return h.memPct, nil
}

func (h *mockProcessHandle) MemoryInfoWithContext(ctx context.Context) (*process.MemoryInfoStat, error) {
	if h.memInfoErr != nil {
		return nil, h.memInfoErr
	}
	return h.memInfo, nil
}

func (h *mockProcessHandle) NumThreadsWithContext(ctx context.Context) (int32, error) {
	return h.numThreads, nil
}

func (h *mockProcessHandle) UsernameWithContext(ctx context.Context) (string, error) {
	return h.username, nil
}

func (h *mockProcessHandle) StatusWithContext(ctx context.Context) ([]string, error) {
	if h.statusErr != nil {
		return nil, h.statusErr
	}
	return h.status, nil
}

func (h *mockProcessHandle) CreateTimeWithContext(ctx context.Context) (int64, error) {
	return h.createTime, nil
}

func (h *mockProcessHandle) CmdlineWithContext(ctx context.Context) (string, error) {
	return h.cmdline, nil
}

func (h *mockProcessHandle) ExeWithContext(ctx context.Context) (string, error) {
	return h.exe, nil
}

func (h *mockProcessHandle) NiceWithContext(ctx context.Context) (int32, error) {
	return h.nice, nil
}

func (h *mockProcessHandle) IOCountersWithContext(ctx context.Context) (*process.IOCountersStat, error) {
	if h.ioCountErr != nil {
		return nil, h.ioCountErr
	}
	return h.ioCounters, nil
}

// Unit Tests exercising production methods with explicit assertions

func TestMock_GetProcesses_EnumerationAndSummary(t *testing.T) {
	// 1. Timestamp precision: using 1700000000123 ms (1700000000 seconds and 123 milliseconds)
	h1 := &mockProcessHandle{
		pid:        100,
		ppid:       1,
		name:       "test-proc-1",
		cpuPct:     45.5,
		memPct:     12.3,
		memInfo:    &process.MemoryInfoStat{RSS: 102400, VMS: 204800},
		numThreads: 4,
		username:   "muhammad",
		status:     []string{"R"},
		createTime: 1700000000123,
		cmdline:    "/bin/test1 --arg",
		exe:        "/bin/test1",
	}
	h2 := &mockProcessHandle{
		pid:        101,
		ppid:       1,
		name:       "", // name fallback to PID-%d
		cpuPct:     10.0,
		memPct:     5.0,
		memInfo:    &process.MemoryInfoStat{RSS: 51200, VMS: 102400},
		numThreads: 2,
		username:   "root",
		status:     []string{"S"},
		createTime: 1700000000000,
		cmdline:    "/bin/test2",
		exe:        "/bin/test2",
	}

	provider := &mockProcessProvider{
		processes: []processHandle{h1, h2},
	}
	c := newProcessCollector(provider)
	ctx := context.Background()

	summary, err := c.GetProcesses(ctx, 10, "cpu", "")
	require.NoError(t, err)
	require.NotNil(t, summary)

	assert.Equal(t, 2, summary.TotalCount)
	assert.Equal(t, 1, summary.RunningCount)
	assert.Equal(t, 1, summary.SleepingCount)
	assert.Equal(t, 0, summary.StoppedCount)
	assert.Equal(t, 0, summary.ZombieCount)

	// Assert CPU and Memory percentage propagation
	assert.Equal(t, 45.5, summary.Processes[0].CPUPercent)
	assert.InDelta(t, 12.3, summary.Processes[0].MemoryPercent, 0.001)

	// Assert RSS and VMS propagation
	assert.Equal(t, uint64(102400), summary.Processes[0].MemoryRSS)
	assert.Equal(t, uint64(204800), summary.Processes[0].MemoryVMS)

	// Assert exact sub-second timestamp preservation (1700000000123 ms -> 1700000000 sec, 123*1e6 nsec)
	expectedTime := time.Unix(1700000000, 123*int64(time.Millisecond))
	assert.Equal(t, expectedTime, summary.Processes[0].CreateTime)

	// Assert Name fallback
	assert.Equal(t, "test-proc-1", summary.Processes[0].Name)
	assert.Equal(t, "PID-101", summary.Processes[1].Name)

	// Assert raw process status and summary counters
	assert.Equal(t, "R", summary.Processes[0].Status)
	assert.Equal(t, "S", summary.Processes[1].Status)
}

func TestMock_GetProcesses_SortingAndFiltering(t *testing.T) {
	h1 := &mockProcessHandle{pid: 10, name: "alpha", cpuPct: 10.0, memInfo: &process.MemoryInfoStat{RSS: 100}, status: []string{"S"}}
	h2 := &mockProcessHandle{pid: 2, name: "beta", cpuPct: 90.0, memInfo: &process.MemoryInfoStat{RSS: 500}, status: []string{"R"}}
	h3 := &mockProcessHandle{pid: 30, name: "gamma", cpuPct: 50.0, memInfo: &process.MemoryInfoStat{RSS: 300}, status: []string{"S"}}

	provider := &mockProcessProvider{
		processes: []processHandle{h1, h2, h3},
	}
	c := newProcessCollector(provider)
	ctx := context.Background()

	// Sorting by CPU desc
	sumCPU, err := c.GetProcesses(ctx, 0, "cpu", "")
	require.NoError(t, err)
	assert.Equal(t, int32(2), sumCPU.Processes[0].PID)
	assert.Equal(t, int32(30), sumCPU.Processes[1].PID)
	assert.Equal(t, int32(10), sumCPU.Processes[2].PID)

	// Sorting by Memory RSS desc
	sumMem, err := c.GetProcesses(ctx, 0, "mem", "")
	require.NoError(t, err)
	assert.Equal(t, int32(2), sumMem.Processes[0].PID)
	assert.Equal(t, int32(30), sumMem.Processes[1].PID)
	assert.Equal(t, int32(10), sumMem.Processes[2].PID)

	// Sorting by PID asc
	sumPID, err := c.GetProcesses(ctx, 0, "pid", "")
	require.NoError(t, err)
	assert.Equal(t, int32(2), sumPID.Processes[0].PID)
	assert.Equal(t, int32(10), sumPID.Processes[1].PID)
	assert.Equal(t, int32(30), sumPID.Processes[2].PID)

	// Sorting by Name asc
	sumName, err := c.GetProcesses(ctx, 0, "name", "")
	require.NoError(t, err)
	assert.Equal(t, "alpha", sumName.Processes[0].Name)
	assert.Equal(t, "beta", sumName.Processes[1].Name)
	assert.Equal(t, "gamma", sumName.Processes[2].Name)

	// Filtering independently by process name ("gamma")
	sumFilterName, err := c.GetProcesses(ctx, 0, "cpu", "gamma")
	require.NoError(t, err)
	assert.Equal(t, 1, sumFilterName.TotalCount)
	assert.Equal(t, int32(30), sumFilterName.Processes[0].PID)

	// Filtering independently by PID ("10")
	sumFilterPID, err := c.GetProcesses(ctx, 0, "cpu", "10")
	require.NoError(t, err)
	assert.Equal(t, 1, sumFilterPID.TotalCount)
	assert.Equal(t, int32(10), sumFilterPID.Processes[0].PID)
}

func TestMock_GetProcessDetails_SuccessAndIOCounters(t *testing.T) {
	h := &mockProcessHandle{
		pid:        500,
		ppid:       1,
		name:       "worker",
		cpuPct:     25.0,
		memPct:     8.5,
		memInfo:    &process.MemoryInfoStat{RSS: 409600, VMS: 819200},
		numThreads: 8,
		username:   "service",
		status:     []string{"R"},
		createTime: 1700000000123,
		cmdline:    "worker --port 8080",
		exe:        "/usr/bin/worker",
		nice:       -5,
		ioCounters: &process.IOCountersStat{ReadBytes: 1024, WriteBytes: 2048},
	}

	provider := &mockProcessProvider{
		procMap: map[int32]processHandle{500: h},
	}
	c := newProcessCollector(provider)
	ctx := context.Background()

	details, err := c.GetProcessDetails(ctx, 500)
	require.NoError(t, err)
	require.NotNil(t, details)

	assert.Equal(t, int32(500), details.PID)
	assert.Equal(t, "worker", details.Name)
	assert.Equal(t, int32(-5), details.Nice)
	// Verify raw I/O byte-counter propagation (ReadBytesSec and WriteBytesSec retain raw byte counts)
	assert.Equal(t, float64(1024), details.ReadBytesSec)
	assert.Equal(t, float64(2048), details.WriteBytesSec)
	assert.Equal(t, time.Unix(1700000000, 123*int64(time.Millisecond)), details.CreateTime)
}

func TestMock_PreCancelledContext(t *testing.T) {
	h := &mockProcessHandle{pid: 1, name: "proc1", status: []string{"S"}}
	provider := &mockProcessProvider{processes: []processHandle{h}}
	c := newProcessCollector(provider)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Pre-cancelled

	_, err := c.GetProcesses(ctx, 10, "cpu", "")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestMock_CancellationDuringIteration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h1 := &mockProcessHandle{
		pid:    1,
		name:   "proc1",
		status: []string{"S"},
		onName: func() {
			cancel() // Cancel context when processing the first process
		},
	}
	h2 := &mockProcessHandle{
		pid:    2,
		name:   "proc2",
		status: []string{"S"},
	}

	provider := &mockProcessProvider{
		processes: []processHandle{h1, h2},
	}
	c := newProcessCollector(provider)

	summary, err := c.GetProcesses(ctx, 10, "cpu", "")
	assert.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, summary)

	// Verify that the second process's metadata methods were not called
	assert.Equal(t, 1, h1.nameCallCount)
	assert.Equal(t, 0, h2.nameCallCount) // h2 was not processed due to loop termination on ctx.Done() after h1
}

func TestMock_ErrorBehaviorAndMetadataErrors(t *testing.T) {
	// 1. Provider enumeration failure (using assert.ErrorIs)
	listErr := errors.New("enumeration failed")
	providerErr := &mockProcessProvider{listErr: listErr}
	cErr := newProcessCollector(providerErr)
	_, err := cErr.GetProcesses(context.Background(), 10, "cpu", "")
	assert.ErrorIs(t, err, listErr)

	// 2. PID lookup failure (not found)
	providerNotFound := &mockProcessProvider{getErr: os.ErrNotExist}
	cNotFound := newProcessCollector(providerNotFound)
	_, errNotFound := cNotFound.GetProcessDetails(context.Background(), 999)
	assert.ErrorIs(t, errNotFound, os.ErrNotExist)

	// 3. Metadata errors & fallback verification (CPUPercent, MemoryPercent, MemoryInfo, IOCounters errors)
	hErr := &mockProcessHandle{
		pid:        123,
		name:       "err-proc",
		cpuErr:     errors.New("cpu error"),
		memPctErr:  errors.New("mem pct error"),
		memInfoErr: errors.New("mem info error"),
		ioCountErr: errors.New("io error"),
		statusErr:  errors.New("status error"),
	}
	providerMetaErr := &mockProcessProvider{
		processes: []processHandle{hErr},
		procMap:   map[int32]processHandle{123: hErr},
	}
	cMeta := newProcessCollector(providerMetaErr)

	// Verify GetProcesses handles underlying handle errors gracefully (defaults/zero values)
	summary, err := cMeta.GetProcesses(context.Background(), 10, "cpu", "")
	require.NoError(t, err)
	require.Len(t, summary.Processes, 1)
	assert.Equal(t, float64(0), summary.Processes[0].CPUPercent)
	assert.Equal(t, float32(0), summary.Processes[0].MemoryPercent)
	assert.Equal(t, uint64(0), summary.Processes[0].MemoryRSS)
	assert.Equal(t, uint64(0), summary.Processes[0].MemoryVMS)
	assert.Equal(t, "Unknown", summary.Processes[0].Status) // Fallback for status error / empty slice

	// Verify GetProcessDetails handles underlying handle errors gracefully (including zero ReadBytesSec and WriteBytesSec on IOCounters error)
	details, err := cMeta.GetProcessDetails(context.Background(), 123)
	require.NoError(t, err)
	assert.Equal(t, int32(123), details.PID)
	assert.Equal(t, float64(0), details.CPUPercent)
	assert.Equal(t, float32(0), details.MemoryPercent)
	assert.Equal(t, uint64(0), details.MemoryRSS)
	assert.Equal(t, "Unknown", details.Status)
	assert.Equal(t, float64(0), details.ReadBytesSec)
	assert.Equal(t, float64(0), details.WriteBytesSec)
}

func TestNewProcessCollector_DefaultProvider(t *testing.T) {
	// 4. Default provider verification
	// Confirm NewProcessCollector installs the gopsutilProvider implementation.
	cDefault := NewProcessCollector()
	assert.NotNil(t, cDefault)
	assert.NotNil(t, cDefault.provider)
	_, isGopsutil := cDefault.provider.(*gopsutilProvider)
	assert.True(t, isGopsutil, "expected default provider to be *gopsutilProvider")

	// Confirm newProcessCollector(nil) does the same.
	cNil := newProcessCollector(nil)
	assert.NotNil(t, cNil)
	assert.NotNil(t, cNil.provider)
	_, isGopsutilNil := cNil.provider.(*gopsutilProvider)
	assert.True(t, isGopsutilNil, "expected nil provider fallback to be *gopsutilProvider")
}
