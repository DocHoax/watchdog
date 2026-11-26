package collector

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

// ProcfsReader defines the injectable abstraction for reading procfs files and enumerating PIDs.
type ProcfsReader interface {
	ListPIDs(ctx context.Context) ([]int32, error)
	ReadStat(ctx context.Context, pid int32) ([]byte, error)
	ReadStatus(ctx context.Context, pid int32) ([]byte, error)
	ReadStatm(ctx context.Context, pid int32) ([]byte, error)
	ReadCmdline(ctx context.Context, pid int32) ([]byte, error)
	ReadExe(ctx context.Context, pid int32) (string, error)
	GetBootTime(ctx context.Context) (uint64, error)
	GetTotalMemory(ctx context.Context) (uint64, error)
	LookupUsername(uid string) (string, error)
}

// OSProcfsReader is the default production implementation reading from /proc and host OS.
type OSProcfsReader struct {
	procRoot string
}

func NewOSProcfsReader() *OSProcfsReader {
	return &OSProcfsReader{procRoot: "/proc"}
}

func (r *OSProcfsReader) ListPIDs(ctx context.Context) ([]int32, error) {
	root := r.procRoot
	if root == "" {
		root = "/proc"
	}
	d, err := os.Open(root)
	if err != nil {
		return nil, err
	}
	defer d.Close()

	names, err := d.Readdirnames(-1)
	if err != nil {
		return nil, err
	}

	var pids []int32
	for _, name := range names {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		if isDigit(name) {
			pidVal, err := strconv.ParseInt(name, 10, 32)
			if err == nil {
				pids = append(pids, int32(pidVal))
			}
		}
	}
	sort.Slice(pids, func(i, j int) bool { return pids[i] < pids[j] })
	return pids, nil
}

func isDigit(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func (r *OSProcfsReader) ReadStat(ctx context.Context, pid int32) ([]byte, error) {
	root := r.procRoot
	if root == "" {
		root = "/proc"
	}
	path := filepath.Join(root, strconv.Itoa(int(pid)), "stat")
	return readFileWithContext(ctx, path)
}

func (r *OSProcfsReader) ReadStatus(ctx context.Context, pid int32) ([]byte, error) {
	root := r.procRoot
	if root == "" {
		root = "/proc"
	}
	path := filepath.Join(root, strconv.Itoa(int(pid)), "status")
	return readFileWithContext(ctx, path)
}

func (r *OSProcfsReader) ReadStatm(ctx context.Context, pid int32) ([]byte, error) {
	root := r.procRoot
	if root == "" {
		root = "/proc"
	}
	path := filepath.Join(root, strconv.Itoa(int(pid)), "statm")
	return readFileWithContext(ctx, path)
}

func (r *OSProcfsReader) ReadCmdline(ctx context.Context, pid int32) ([]byte, error) {
	root := r.procRoot
	if root == "" {
		root = "/proc"
	}
	path := filepath.Join(root, strconv.Itoa(int(pid)), "cmdline")
	return readFileWithContext(ctx, path)
}

func (r *OSProcfsReader) ReadExe(ctx context.Context, pid int32) (string, error) {
	root := r.procRoot
	if root == "" {
		root = "/proc"
	}
	path := filepath.Join(root, strconv.Itoa(int(pid)), "exe")
	return os.Readlink(path)
}

func (r *OSProcfsReader) GetBootTime(ctx context.Context) (uint64, error) {
	root := r.procRoot
	if root == "" {
		root = "/proc"
	}
	path := filepath.Join(root, "stat")
	data, err := readFileWithContext(ctx, path)
	if err != nil {
		// Fallback to time.Now unix epoch if stat cannot be read
		return uint64(time.Now().Unix() - 86400), nil
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "btime ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return strconv.ParseUint(fields[1], 10, 64)
			}
		}
	}
	return uint64(time.Now().Unix() - 86400), nil
}

func (r *OSProcfsReader) GetTotalMemory(ctx context.Context) (uint64, error) {
	root := r.procRoot
	if root == "" {
		root = "/proc"
	}
	path := filepath.Join(root, "meminfo")
	data, err := readFileWithContext(ctx, path)
	if err != nil {
		return 0, err
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				val, err := strconv.ParseUint(fields[1], 10, 64)
				if err == nil {
					// MemTotal in /proc/meminfo is in kB
					return val * 1024, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("MemTotal not found in meminfo")
}

func (r *OSProcfsReader) LookupUsername(uid string) (string, error) {
	// Fallback implementation or use user lookup
	// For production, we can inspect /etc/passwd or lookup uid
	return lookupUsernameByUID(uid)
}

func readFileWithContext(ctx context.Context, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func lookupUsernameByUID(uid string) (string, error) {
	if uid == "" {
		return "", fmt.Errorf("empty uid")
	}
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return uid, err
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		parts := strings.Split(line, ":")
		if len(parts) >= 3 && parts[2] == uid {
			return parts[0], nil
		}
	}
	return uid, nil
}

// ProcessCollector gathers running process details using a per-collection procfs snapshot.
type ProcessCollector struct {
	reader ProcfsReader
}

// NewProcessCollector creates a new ProcessCollector instance.
func NewProcessCollector() *ProcessCollector {
	return &ProcessCollector{
		reader: NewOSProcfsReader(),
	}
}

// NewProcessCollectorWithReader creates a ProcessCollector with a custom ProcfsReader for testing.
func NewProcessCollectorWithReader(reader ProcfsReader) *ProcessCollector {
	return &ProcessCollector{
		reader: reader,
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

// GetProcesses retrieves all processes or top N sorted processes with filtering using procfs snapshot.
func (c *ProcessCollector) GetProcesses(ctx context.Context, limit int, sortBy string, filter string) (*model.ProcessSummary, error) {
	reader := c.reader
	if reader == nil {
		reader = NewOSProcfsReader()
	}

	pids, err := reader.ListPIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list PIDs: %w", err)
	}

	bootTime, _ := reader.GetBootTime(ctx)
	totalMem, _ := reader.GetTotalMemory(ctx)
	now := time.Now()
	uptimeSec := uint64(0)
	if bootTime > 0 {
		uptimeSec = uint64(now.Unix()) - bootTime
	}

	var procList []model.ProcessInfo
	var runningCount, sleepingCount, stoppedCount, zombieCount int
	filterLower := strings.ToLower(filter)

	for _, pid := range pids {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		info, err := c.collectProcessInfo(ctx, reader, pid, bootTime, uptimeSec, totalMem)
		if err != nil {
			// Check error type
			if errors.Is(err, syscall.ENOENT) || os.IsNotExist(err) {
				// Process disappearance: skip
				continue
			}
			if errors.Is(err, syscall.EACCES) || os.IsPermission(err) {
				// Permission denial: retain partial record where appropriate
				partial := model.ProcessInfo{
					PID:    pid,
					Name:   fmt.Sprintf("PID-%d", pid),
					Status: "Unknown",
				}
				if filterLower != "" {
					pidStr := strconv.Itoa(int(pid))
					if !strings.Contains(strings.ToLower(partial.Name), filterLower) && !strings.Contains(pidStr, filterLower) {
						continue
					}
				}
				sleepingCount++
				procList = append(procList, partial)
				continue
			}
			// Other filesystem errors / disappearances
			continue
		}

		if info == nil {
			continue
		}

		if filterLower != "" {
			pidStr := strconv.Itoa(int(info.PID))
			if !strings.Contains(strings.ToLower(info.Name), filterLower) && !strings.Contains(pidStr, filterLower) {
				continue
			}
		}

		switch strings.ToUpper(info.Status) {
		case "R", "RUNNING":
			runningCount++
		case "S", "SLEEP", "SLEEPING", "I", "IDLE", "D":
			sleepingCount++
		case "T", "STOPPED":
			stoppedCount++
		case "Z", "ZOMBIE":
			zombieCount++
		default:
			sleepingCount++
		}

		procList = append(procList, *info)
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
		CollectedAt:   now,
	}, nil
}

func (c *ProcessCollector) collectProcessInfo(ctx context.Context, reader ProcfsReader, pid int32, bootTime uint64, uptimeSec uint64, totalMem uint64) (*model.ProcessInfo, error) {
	statBytes, err := reader.ReadStat(ctx, pid)
	if err != nil {
		return nil, err
	}

	statusBytes, _ := reader.ReadStatus(ctx, pid)
	statmBytes, _ := reader.ReadStatm(ctx, pid)
	cmdlineBytes, _ := reader.ReadCmdline(ctx, pid)
	exePath, _ := reader.ReadExe(ctx, pid)

	// Parse /proc/[pid]/stat
	comm, state, ppid, nice, startTimeTicks, cpuTicks, numThreads, err := parseStat(statBytes)
	if err != nil {
		return nil, err
	}

	// Name resolution fallback: status name or comm
	name := comm
	uidStr := ""
	if len(statusBytes) > 0 {
		statusName, statusUID, statusThreads := parseStatus(statusBytes)
		if statusName != "" {
			name = statusName
		}
		if statusUID != "" {
			uidStr = statusUID
		}
		if statusThreads > 0 {
			numThreads = statusThreads
		}
	}

	if name == "" {
		name = fmt.Sprintf("PID-%d", pid)
	}

	// Cmdline fallback if comm is truncated (15 chars) or matches cmdline basename
	if len(cmdlineBytes) > 0 {
		cmdlineStr := parseCmdline(cmdlineBytes)
		if cmdlineStr != "" {
			base := filepath.Base(cmdlineStr)
			// If comm is prefix of base or 15 chars long, consider long-name handling
			if len(comm) >= 15 && strings.HasPrefix(base, comm[:10]) {
				name = base
			}
		}
	}

	// Memory from statm
	var rss, vms uint64
	if len(statmBytes) > 0 {
		rss, vms = parseStatm(statmBytes)
	}

	var memPct float32
	if totalMem > 0 && rss > 0 {
		memPct = float32(rss) / float32(totalMem) * 100.0
	}

	// Username lookup
	username := ""
	if uidStr != "" {
		username, _ = reader.LookupUsername(uidStr)
	}

	// Create time and CPU percentage calculation
	var createTime time.Time
	var createTimeMs int64
	var cpuPct float64

	clkTck := uint64(100) // standard Linux USER_HZ
	if bootTime > 0 && startTimeTicks > 0 {
		createTimeSec := bootTime + (startTimeTicks / clkTck)
		createTimeMs = int64(createTimeSec) * 1000
		createTime = time.Unix(0, createTimeMs*int64(time.Millisecond))
	}

	if bootTime > 0 && uptimeSec > 0 && cpuTicks > 0 {
		// gopsutil lifetime average CPU percentage calculation:
		// total CPU time spent by process (user + system + iowait) / elapsed process lifetime
		elapsedSeconds := float64(time.Now().UnixNano())/1e9 - (float64(bootTime) + float64(startTimeTicks)/float64(clkTck))
		if elapsedSeconds > 0 {
			totalCPUSec := float64(cpuTicks) / float64(clkTck)
			cpuPct = (totalCPUSec / elapsedSeconds) * 100.0
			if cpuPct > 100.0 {
				// Normalize if multi-core accumulation exceeds 100% per core scale or cap if desired,
				// but let's preserve gopsutil semantics (gopsutil CPUPercent can exceed 100 for multithreaded).
			}
		}
	}

	cmdlineStr := parseCmdline(cmdlineBytes)

	status := mapState(state)

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
		CommandLine:   cmdlineStr,
		ExePath:       exePath,
		Nice:          nice,
	}, nil
}

// parseStat parses /proc/[pid]/stat safely handling spaces and parentheses in comm.
// Format: pid (comm) state ppid pgrp session tty_nr tpgid flags minflt cminflt majflt cmajflt utime stime cutime cstime priority nice num_threads itrealvalue start_time ...
// Linux fields (1-indexed): 1=pid, 2=comm, 3=state, 4=ppid, 5=pgrp, 6=session,
//   7=tty_nr, 8=tpgid, 9=flags, 10=minflt, 11=cminflt, 12=majflt, 13=cmajflt,
//   14=utime, 15=stime, 16=cutime, 17=cstime, 18=priority, 19=nice, 20=num_threads,
//   21=itrealvalue, 22=start_time, 42=delayacct_blkio_ticks.
// After extracting comm, the tail starts at field 3 (state) = tailFields[0].
func parseStat(data []byte) (comm string, state string, ppid int32, nice int32, startTimeTicks uint64, cpuTicks uint64, numThreads int32, err error) {
	s := string(bytes.TrimSpace(data))
	openIdx := strings.Index(s, "(")
	closeIdx := strings.LastIndex(s, ")")
	if openIdx == -1 || closeIdx == -1 || openIdx > closeIdx {
		return "", "", 0, 0, 0, 0, 0, fmt.Errorf("malformed stat data: missing parentheses")
	}

	comm = s[openIdx+1 : closeIdx]
	tail := strings.TrimSpace(s[closeIdx+1:])
	fields := strings.Fields(tail)

	// Minimum fields: at least state (fields[0]) and ppid (fields[1])
	// Parse each field individually with error handling; default to zero values
	// only when parsing fails, not merely when fewer fields are present.

	// fields[0] is state (Linux field 3)
	if len(fields) > 0 {
		state = fields[0]
	}

	// fields[1] is ppid (Linux field 4)
	if len(fields) > 1 {
		if v, err := strconv.ParseInt(fields[1], 10, 32); err == nil {
			ppid = int32(v)
		}
	}

	// fields[16] is nice (Linux field 19)
	if len(fields) > 16 {
		if v, err := strconv.ParseInt(fields[16], 10, 32); err == nil {
			nice = int32(v)
		}
	}

	// fields[11] is utime (Linux field 14)
	var utimeVal uint64
	if len(fields) > 11 {
		utimeVal, _ = strconv.ParseUint(fields[11], 10, 64)
	}

	// fields[12] is stime (Linux field 15)
	var stimeVal uint64
	if len(fields) > 12 {
		stimeVal, _ = strconv.ParseUint(fields[12], 10, 64)
	}

	// fields[13] is cutime (Linux field 16)
	var cutimeVal uint64
	if len(fields) > 13 {
		cutimeVal, _ = strconv.ParseUint(fields[13], 10, 64)
	}

	// fields[14] is cstime (Linux field 17)
	var cstimeVal uint64
	if len(fields) > 14 {
		cstimeVal, _ = strconv.ParseUint(fields[14], 10, 64)
	}

	// delayacct_blkio_ticks is Linux field 42 (tail index 39)
	var ioWaitTicks uint64
	if len(fields) > 39 {
		ioWaitTicks, _ = strconv.ParseUint(fields[39], 10, 64)
	}

	cpuTicks = utimeVal + stimeVal + cutimeVal + cstimeVal + ioWaitTicks

	// fields[19] is start_time (Linux field 22)
	if len(fields) > 19 {
		startTimeTicks, _ = strconv.ParseUint(fields[19], 10, 64)
	}

	// fields[17] is num_threads (Linux field 20)
	if len(fields) > 17 {
		if v, err := strconv.ParseInt(fields[17], 10, 32); err == nil {
			numThreads = int32(v)
		}
	}
	if numThreads <= 0 {
		numThreads = 1
	}

	// Set state to "R" only if it's empty (no valid state found)
	if state == "" {
		state = "R"
	}

	return comm, state, ppid, nice, startTimeTicks, cpuTicks, numThreads, nil
}

func parseStatus(data []byte) (name string, uid string, numThreads int32) {
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "Name":
			name = val
		case "Uid":
			uidParts := strings.Fields(val)
			if len(uidParts) > 0 {
				uid = uidParts[0]
			}
		case "Threads":
			t, err := strconv.ParseInt(val, 10, 32)
			if err == nil {
				numThreads = int32(t)
			}
		}
	}
	return name, uid, numThreads
}

func parseStatm(data []byte) (rss uint64, vms uint64) {
	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return 0, 0
	}
	pageSz := uint64(os.Getpagesize())
	if pageSz == 0 {
		pageSz = 4096
	}
	// statm: size resident share text lib dt
	vmsPages, _ := strconv.ParseUint(fields[0], 10, 64)
	rssPages, _ := strconv.ParseUint(fields[1], 10, 64)
	vms = vmsPages * pageSz
	rss = rssPages * pageSz
	return rss, vms
}

func parseCmdline(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	// Replace NUL bytes with spaces
	buf := make([]byte, len(data))
	copy(buf, data)
	for i := range buf {
		if buf[i] == 0 {
			buf[i] = ' '
		}
	}
	return strings.TrimSpace(string(buf))
}

func mapState(state string) string {
	switch strings.ToUpper(state) {
	case "R", "RUNNING":
		return "Running"
	case "S", "SLEEPING":
		return "Sleeping"
	case "D", "DISK_SLEEP", "WAIT":
		return "Sleeping"
	case "Z", "ZOMBIE":
		return "Zombie"
	case "T", "STOPPED":
		return "Stopped"
	case "I", "IDLE":
		return "Sleeping"
	default:
		return "Sleeping"
	}
}

// GetProcessDetails fetches full metadata for a single PID using the procfs snapshot reader.
func (c *ProcessCollector) GetProcessDetails(ctx context.Context, pid int32) (*model.ProcessInfo, error) {
	reader := c.reader
	if reader == nil {
		reader = NewOSProcfsReader()
	}

	bootTime, _ := reader.GetBootTime(ctx)
	totalMem, _ := reader.GetTotalMemory(ctx)
	now := time.Now()
	uptimeSec := uint64(0)
	if bootTime > 0 {
		uptimeSec = uint64(now.Unix()) - bootTime
	}

	info, err := c.collectProcessInfo(ctx, reader, pid, bootTime, uptimeSec, totalMem)
	if err != nil {
		return nil, fmt.Errorf("process with PID %d not found: %w", pid, err)
	}

	// Add I/O counters if available from reader or stat
	statBytes, err := reader.ReadStat(ctx, pid)
	var nice int32
	if err == nil {
		_, _, _, nice, _, _, _, _ = parseStat(statBytes)
	}
	info.Nice = nice

	return info, nil
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
