package collector

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
	"github.com/shirou/gopsutil/v4/process"
)

// ProcessProvider abstracts gopsutil process provider operations for testability.
type ProcessProvider interface {
	ProcessesWithContext(ctx context.Context) ([]ProcessHandle, error)
	NewProcessWithContext(ctx context.Context, pid int32) (ProcessHandle, error)
}

// ProcessHandle abstracts individual process attribute queries mirroring gopsutil v4 Process methods.
type ProcessHandle interface {
	PID() int32
	NameWithContext(ctx context.Context) (string, error)
	PpidWithContext(ctx context.Context) (int32, error)
	CPUPercentWithContext(ctx context.Context) (float64, error)
	MemoryPercentWithContext(ctx context.Context) (float32, error)
	MemoryInfoWithContext(ctx context.Context) (*process.MemoryInfoStat, error)
	NumThreadsWithContext(ctx context.Context) (int32, error)
	UsernameWithContext(ctx context.Context) (string, error)
	StatusWithContext(ctx context.Context) ([]string, error)
	CreateTimeWithContext(ctx context.Context) (int64, error)
	CmdlineWithContext(ctx context.Context) (string, error)
	ExeWithContext(ctx context.Context) (string, error)
	NiceWithContext(ctx context.Context) (int32, error)
	IOCountersWithContext(ctx context.Context) (*process.IOCountersStat, error)
}

// gopsutilProvider implements ProcessProvider using gopsutil package functions.
type gopsutilProvider struct{}

func (g *gopsutilProvider) ProcessesWithContext(ctx context.Context) ([]ProcessHandle, error) {
	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, err
	}
	handles := make([]ProcessHandle, len(procs))
	for i, p := range procs {
		handles[i] = &gopsutilHandle{p: p}
	}
	return handles, nil
}

func (g *gopsutilProvider) NewProcessWithContext(ctx context.Context, pid int32) (ProcessHandle, error) {
	p, err := process.NewProcessWithContext(ctx, pid)
	if err != nil {
		return nil, err
	}
	return &gopsutilHandle{p: p}, nil
}

// gopsutilHandle implements ProcessHandle wrapping *process.Process.
type gopsutilHandle struct {
	p *process.Process
}

func (h *gopsutilHandle) PID() int32 {
	return h.p.Pid
}

func (h *gopsutilHandle) NameWithContext(ctx context.Context) (string, error) {
	return h.p.NameWithContext(ctx)
}

func (h *gopsutilHandle) PpidWithContext(ctx context.Context) (int32, error) {
	return h.p.PpidWithContext(ctx)
}

func (h *gopsutilHandle) CPUPercentWithContext(ctx context.Context) (float64, error) {
	return h.p.CPUPercentWithContext(ctx)
}

func (h *gopsutilHandle) MemoryPercentWithContext(ctx context.Context) (float32, error) {
	return h.p.MemoryPercentWithContext(ctx)
}

func (h *gopsutilHandle) MemoryInfoWithContext(ctx context.Context) (*process.MemoryInfoStat, error) {
	return h.p.MemoryInfoWithContext(ctx)
}

func (h *gopsutilHandle) NumThreadsWithContext(ctx context.Context) (int32, error) {
	return h.p.NumThreadsWithContext(ctx)
}

func (h *gopsutilHandle) UsernameWithContext(ctx context.Context) (string, error) {
	return h.p.UsernameWithContext(ctx)
}

func (h *gopsutilHandle) StatusWithContext(ctx context.Context) ([]string, error) {
	return h.p.StatusWithContext(ctx)
}

func (h *gopsutilHandle) CreateTimeWithContext(ctx context.Context) (int64, error) {
	return h.p.CreateTimeWithContext(ctx)
}

func (h *gopsutilHandle) CmdlineWithContext(ctx context.Context) (string, error) {
	return h.p.CmdlineWithContext(ctx)
}

func (h *gopsutilHandle) ExeWithContext(ctx context.Context) (string, error) {
	return h.p.ExeWithContext(ctx)
}

func (h *gopsutilHandle) NiceWithContext(ctx context.Context) (int32, error) {
	return h.p.NiceWithContext(ctx)
}

func (h *gopsutilHandle) IOCountersWithContext(ctx context.Context) (*process.IOCountersStat, error) {
	return h.p.IOCountersWithContext(ctx)
}

// ProcessCollector gathers running process details.
type ProcessCollector struct {
	provider ProcessProvider
}

// NewProcessCollector creates a new ProcessCollector instance using default production gopsutil provider.
func NewProcessCollector() *ProcessCollector {
	return &ProcessCollector{
		provider: &gopsutilProvider{},
	}
}

// newProcessCollector creates a ProcessCollector with an injected provider for unit testing.
// If provider is nil, it falls back to the default production gopsutil provider.
func newProcessCollector(provider ProcessProvider) *ProcessCollector {
	if provider == nil {
		provider = &gopsutilProvider{}
	}
	return &ProcessCollector{
		provider: provider,
	}
}

// Name returns the identifier of the collector.
func (c *ProcessCollector) Name() string {
	return "process"
}

// Collect retrieves current process snapshot.
func (c *ProcessCollector) Collect(ctx context.Context) (any, error) {
	return c.GetProcesses(ctx, 0, "cpu", "")
}

// GetProcesses retrieves all processes or top N sorted processes with filtering.
func (c *ProcessCollector) GetProcesses(ctx context.Context, limit int, sortBy string, filter string) (*model.ProcessSummary, error) {
	provider := c.provider
	if provider == nil {
		provider = &gopsutilProvider{}
	}

	procs, err := provider.ProcessesWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve processes: %w", err)
	}

	var procList []model.ProcessInfo
	var runningCount, sleepingCount, stoppedCount, zombieCount int

	filterLower := strings.ToLower(filter)

	for _, p := range procs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		pid := p.PID()
		name, _ := p.NameWithContext(ctx)
		if name == "" {
			name = fmt.Sprintf("PID-%d", pid)
		}

		if filterLower != "" {
			pidStr := fmt.Sprintf("%d", pid)
			if !strings.Contains(strings.ToLower(name), filterLower) && !strings.Contains(pidStr, filterLower) {
				continue
			}
		}

		ppid, _ := p.PpidWithContext(ctx)
		cpuPct, _ := p.CPUPercentWithContext(ctx)
		memPct, _ := p.MemoryPercentWithContext(ctx)
		memInfo, _ := p.MemoryInfoWithContext(ctx)
		numThreads, _ := p.NumThreadsWithContext(ctx)
		username, _ := p.UsernameWithContext(ctx)
		statusSlice, _ := p.StatusWithContext(ctx)
		createTimeMs, _ := p.CreateTimeWithContext(ctx)
		cmdline, _ := p.CmdlineWithContext(ctx)
		exe, _ := p.ExeWithContext(ctx)

		status := "Unknown"
		if len(statusSlice) > 0 {
			status = statusSlice[0]
		}

		switch strings.ToUpper(status) {
		case "R", "RUNNING":
			runningCount++
		case "S", "SLEEP", "SLEEPING", "I", "IDLE":
			sleepingCount++
		case "T", "STOPPED":
			stoppedCount++
		case "Z", "ZOMBIE":
			zombieCount++
		default:
			sleepingCount++
		}

		var rss, vms uint64
		if memInfo != nil {
			rss = memInfo.RSS
			vms = memInfo.VMS
		}

		var createTime time.Time
		if createTimeMs > 0 {
			createTime = time.Unix(0, createTimeMs*int64(time.Millisecond))
		}

		procList = append(procList, model.ProcessInfo{
			PID:           pid,
			PPID:          ppid,
			Name:          name,
			Username:      username,
			CPUPercent:    cpuPct,
			MemoryPercent: memPct,
			MemoryRSS:     rss,
			MemoryVMS:     vms,
			Status:        status,
			NumThreads:    numThreads,
			CreateTime:    createTime,
			CommandLine:   cmdline,
			ExePath:       exe,
		})
	}

	// Sorting
	switch strings.ToLower(sortBy) {
	case "mem", "memory":
		sort.Slice(procList, func(i, j int) bool {
			return procList[i].MemoryRSS > procList[j].MemoryRSS
		})
	case "pid":
		sort.Slice(procList, func(i, j int) bool {
			return procList[i].PID < procList[j].PID
		})
	case "name":
		sort.Slice(procList, func(i, j int) bool {
			return strings.ToLower(procList[i].Name) < strings.ToLower(procList[j].Name)
		})
	case "cpu":
		fallthrough
	default:
		sort.Slice(procList, func(i, j int) bool {
			return procList[i].CPUPercent > procList[j].CPUPercent
		})
	}

	total := len(procList)
	if limit > 0 && len(procList) > limit {
		procList = procList[:limit]
	}

	return &model.ProcessSummary{
		TotalCount:    total,
		RunningCount:  runningCount,
		SleepingCount: sleepingCount,
		StoppedCount:  stoppedCount,
		ZombieCount:   zombieCount,
		Processes:     procList,
		CollectedAt:   time.Now(),
	}, nil
}

// GetProcessDetails fetches full metadata for a single PID.
func (c *ProcessCollector) GetProcessDetails(ctx context.Context, pid int32) (*model.ProcessInfo, error) {
	provider := c.provider
	if provider == nil {
		provider = &gopsutilProvider{}
	}

	p, err := provider.NewProcessWithContext(ctx, pid)
	if err != nil {
		return nil, fmt.Errorf("process with PID %d not found: %w", pid, err)
	}

	name, _ := p.NameWithContext(ctx)
	ppid, _ := p.PpidWithContext(ctx)
	cpuPct, _ := p.CPUPercentWithContext(ctx)
	memPct, _ := p.MemoryPercentWithContext(ctx)
	memInfo, _ := p.MemoryInfoWithContext(ctx)
	numThreads, _ := p.NumThreadsWithContext(ctx)
	username, _ := p.UsernameWithContext(ctx)
	statusSlice, _ := p.StatusWithContext(ctx)
	createTimeMs, _ := p.CreateTimeWithContext(ctx)
	cmdline, _ := p.CmdlineWithContext(ctx)
	exe, _ := p.ExeWithContext(ctx)
	nice, _ := p.NiceWithContext(ctx)
	ioCounters, _ := p.IOCountersWithContext(ctx)

	status := "Unknown"
	if len(statusSlice) > 0 {
		status = statusSlice[0]
	}

	var rss, vms uint64
	if memInfo != nil {
		rss = memInfo.RSS
		vms = memInfo.VMS
	}

	var createTime time.Time
	if createTimeMs > 0 {
		createTime = time.Unix(0, createTimeMs*int64(time.Millisecond))
	}

	var readRate, writeRate float64
	if ioCounters != nil {
		readRate = float64(ioCounters.ReadBytes)
		writeRate = float64(ioCounters.WriteBytes)
	}

	return &model.ProcessInfo{
		PID:           pid,
		PPID:          ppid,
		Name:          name,
		Username:      username,
		CPUPercent:    cpuPct,
		MemoryPercent: memPct,
		MemoryRSS:     rss,
		MemoryVMS:     vms,
		Status:        status,
		NumThreads:    numThreads,
		CreateTime:    createTime,
		CommandLine:   cmdline,
		ExePath:       exe,
		Nice:          nice,
		ReadBytesSec:  readRate,
		WriteBytesSec: writeRate,
	}, nil
}

// KillProcess terminates a process by PID with appropriate OS signal.
func (c *ProcessCollector) KillProcess(pid int32, sig os.Signal) error {
	p, err := os.FindProcess(int(pid))
	if err != nil {
		return fmt.Errorf("process %d not found: %w", pid, err)
	}

	if sig == nil {
		sig = os.Kill
	}

	if err := p.Signal(sig); err != nil {
		return fmt.Errorf("failed to send signal to PID %d: %w", pid, err)
	}

	return nil
}

// KillProcess is a convenience function to terminate a process by PID.
func KillProcess(pid int) error {
	c := NewProcessCollector()
	return c.KillProcess(int32(pid), os.Kill)
}
