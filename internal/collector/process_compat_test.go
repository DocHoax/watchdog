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

// Mock ProcessProvider and ProcessHandle for deterministic unit testing.

type mockProcessProvider struct {
	processes []ProcessHandle
	listErr   error
	procMap   map[int32]ProcessHandle
	getErr    error
}

func (m *mockProcessProvider) ProcessesWithContext(ctx context.Context) ([]ProcessHandle, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.processes, nil
}

func (m *mockProcessProvider) NewProcessWithContext(ctx context.Context, pid int32) (ProcessHandle, error) {
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
}

func (h *mockProcessHandle) PID() int32 { return h.pid }

func (h *mockProcessHandle) NameWithContext(ctx context.Context) (string, error) {
	if h.nameErr != nil {
		return "", h.nameErr
	}
	return h.name, nil
}

func (h *mockProcessHandle) PpidWithContext(ctx context.Context) (int32, error) {
	return h.ppid, nil
}

func (h *mockProcessHandle) CPUPercentWithContext(ctx context.Context) (float64, error) {
	return h.cpuPct, nil
}

func (h *mockProcessHandle) MemoryPercentWithContext(ctx context.Context) (float32, error) {
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

// Unit Tests exercising production methods via mock provider

func TestMock_GetProcesses_EnumerationAndSummary(t *testing.T) {
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
		createTime: 1700000000000, // 1700000000 seconds in ms
		cmdline:    "/bin/test1 --arg",
		exe:        "/bin/test1",
	}
	h2 := &mockProcessHandle{
		pid:        101,
		ppid:       1,
		name:       "", // tests name fallback to PID-%d
		cpuPct:     10.0,
		memPct:     5.0,
		memInfo:    &process.MemoryInfoStat{RSS: 51200, VMS: 102400},
		numThreads: 2,
		username:   "root",
		status:     []string{"S"}, // Sleeping count
		createTime: 1700000000000,
		cmdline:    "/bin/test2",
		exe:        "/bin/test2",
	}

	provider := &mockProcessProvider{
		processes: []ProcessHandle{h1, h2},
	}
	c := newProcessCollector(provider)
	ctx := context.Background()

	summary, err := c.GetProcesses(ctx, 10, "cpu", "")
	require.NoError(t, err)
	require.NotNil(t, summary)

	assert.Equal(t, 2, summary.TotalCount)
	assert.Equal(t, 1, summary.RunningCount)
	assert.Equal(t, 1, summary.SleepingCount)
	assert.Equal(t, "test-proc-1", summary.Processes[0].Name)
	assert.Equal(t, "PID-101", summary.Processes[1].Name) // Fallback verified
	assert.Equal(t, uint64(102400), summary.Processes[0].MemoryRSS)
}

func TestMock_GetProcesses_SortingAndFiltering(t *testing.T) {
	h1 := &mockProcessHandle{pid: 1, name: "alpha", cpuPct: 10.0, memInfo: &process.MemoryInfoStat{RSS: 100}, status: []string{"S"}}
	h2 := &mockProcessHandle{pid: 2, name: "beta", cpuPct: 90.0, memInfo: &process.MemoryInfoStat{RSS: 500}, status: []string{"R"}}
	h3 := &mockProcessHandle{pid: 3, name: "gamma", cpuPct: 50.0, memInfo: &process.MemoryInfoStat{RSS: 300}, status: []string{"S"}}

	provider := &mockProcessProvider{
		processes: []ProcessHandle{h1, h2, h3},
	}
	c := newProcessCollector(provider)
	ctx := context.Background()

	// Sort by CPU (default desc)
	summaryCPU, err := c.GetProcesses(ctx, 0, "cpu", "")
	require.NoError(t, err)
	assert.Equal(t, int32(2), summaryCPU.Processes[0].PID) // 90.0% CPU

	// Sort by memory RSS desc
	summaryMem, err := c.GetProcesses(ctx, 0, "mem", "")
	require.NoError(t, err)
	assert.Equal(t, int32(2), summaryMem.Processes[0].PID) // 500 RSS

	// Filter by name "gamma" or PID "3"
	summaryFilter, err := c.GetProcesses(ctx, 0, "pid", "gamma")
	require.NoError(t, err)
	assert.Equal(t, 1, summaryFilter.TotalCount)
	assert.Equal(t, int32(3), summaryFilter.Processes[0].PID)
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
		createTime: 1700000000000,
		cmdline:    "worker --port 8080",
		exe:        "/usr/bin/worker",
		nice:       -5,
		ioCounters: &process.IOCountersStat{ReadBytes: 1024, WriteBytes: 2048},
	}

	provider := &mockProcessProvider{
		procMap: map[int32]ProcessHandle{500: h},
	}
	c := newProcessCollector(provider)
	ctx := context.Background()

	details, err := c.GetProcessDetails(ctx, 500)
	require.NoError(t, err)
	require.NotNil(t, details)

	assert.Equal(t, int32(500), details.PID)
	assert.Equal(t, "worker", details.Name)
	assert.Equal(t, int32(-5), details.Nice)
	assert.Equal(t, float64(1024), details.ReadBytesSec)
	assert.Equal(t, float64(2048), details.WriteBytesSec)
	assert.Equal(t, time.Unix(1700000000, 0), details.CreateTime)
}

func TestMock_ErrorHandling_EnumerationAndLookup(t *testing.T) {
	// Provider enumeration error
	providerErr := &mockProcessProvider{listErr: errors.New("procfs read error")}
	cErr := newProcessCollector(providerErr)
	_, err := cErr.GetProcesses(context.Background(), 10, "cpu", "")
	assert.Error(t, err)

	// PID lookup not found error
	providerNotFound := &mockProcessProvider{getErr: os.ErrNotExist}
	cNotFound := newProcessCollector(providerNotFound)
	_, errNotFound := cNotFound.GetProcessDetails(context.Background(), 999)
	assert.Error(t, errNotFound)
}

func TestMock_ContextCancellation(t *testing.T) {
	h := &mockProcessHandle{pid: 1, name: "slow", status: []string{"S"}}
	provider := &mockProcessProvider{processes: []ProcessHandle{h}}
	c := newProcessCollector(provider)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := c.GetProcesses(ctx, 10, "cpu", "")
	assert.Error(t, err)
}
