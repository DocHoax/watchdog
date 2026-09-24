package diagnostics

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/pkg/model"
	"github.com/DocHoax/watchdog/pkg/util"
)

// Rule represents a single diagnostic evaluation rule.
type Rule interface {
	ID() string
	Category() string
	Name() string
	Evaluate(ctx context.Context, snapshot *model.SystemSnapshot, cfg *config.Config) model.DiagnosticResult
}

// CPURule checks overall CPU utilization.
type CPURule struct{}

func (r *CPURule) ID() string       { return "cpu-utilization" }
func (r *CPURule) Category() string { return "CPU" }
func (r *CPURule) Name() string     { return "CPU Utilization" }

func (r *CPURule) Evaluate(ctx context.Context, snapshot *model.SystemSnapshot, cfg *config.Config) model.DiagnosticResult {
	res := model.DiagnosticResult{
		ID:          r.ID(),
		Category:    r.Category(),
		Name:        r.Name(),
		Timestamp:   time.Now(),
		MetricValue: "N/A",
	}

	if snapshot == nil || snapshot.CPU == nil {
		res.Status = model.StatusSkip
		res.Severity = model.SeverityInfo
		res.Description = "CPU metrics unavailable"
		return res
	}

	usage := snapshot.CPU.OverallUsage
	res.MetricValue = fmt.Sprintf("%.1f%%", usage)

	warnThresh := 75.0
	critThresh := 90.0
	if cfg != nil && cfg.Alerts.CPU.Enabled {
		if cfg.Alerts.CPU.Threshold > 0 {
			warnThresh = cfg.Alerts.CPU.Threshold
			critThresh = min(warnThresh+15.0, 95.0)
		}
	}
	res.Threshold = fmt.Sprintf("Warn: >%.0f%%, Crit: >%.0f%%", warnThresh, critThresh)

	if usage >= critThresh {
		res.Status = model.StatusFail
		res.Severity = model.SeverityCritical
		res.Description = fmt.Sprintf("CPU utilization is critically high (%.1f%%)", usage)
		res.Recommendation = "Identify top CPU-consuming processes using `watchdog top --sort cpu` and scale or terminate rogue tasks."
	} else if usage >= warnThresh {
		res.Status = model.StatusWarning
		res.Severity = model.SeverityWarning
		res.Description = fmt.Sprintf("CPU utilization is elevated (%.1f%%)", usage)
		res.Recommendation = "Monitor CPU load trend and verify if background tasks or compilation jobs are running."
	} else {
		res.Status = model.StatusPass
		res.Severity = model.SeverityInfo
		res.Description = fmt.Sprintf("CPU utilization is healthy (%.1f%%)", usage)
	}

	return res
}

// CPULoadRule checks system load average against available logical CPU cores.
type CPULoadRule struct{}

func (r *CPULoadRule) ID() string       { return "cpu-load-average" }
func (r *CPULoadRule) Category() string { return "CPU" }
func (r *CPULoadRule) Name() string     { return "CPU Load Average" }

func (r *CPULoadRule) Evaluate(ctx context.Context, snapshot *model.SystemSnapshot, cfg *config.Config) model.DiagnosticResult {
	res := model.DiagnosticResult{
		ID:        r.ID(),
		Category:  r.Category(),
		Name:      r.Name(),
		Timestamp: time.Now(),
	}

	if snapshot == nil || snapshot.CPU == nil || snapshot.CPU.LogicalCores <= 0 {
		res.Status = model.StatusSkip
		res.Severity = model.SeverityInfo
		res.Description = "Load average metrics unavailable"
		return res
	}

	cores := float64(snapshot.CPU.LogicalCores)
	load1 := snapshot.CPU.LoadAverage.Load1
	ratio := load1 / cores
	res.MetricValue = fmt.Sprintf("Load1: %.2f (%.1fx capacity on %d cores)", load1, ratio, int(cores))
	res.Threshold = fmt.Sprintf("Warn: >1.5x (%.1f), Crit: >2.5x (%.1f)", cores*1.5, cores*2.5)

	if ratio >= 2.5 {
		res.Status = model.StatusFail
		res.Severity = model.SeverityCritical
		res.Description = "System process run queue is severely saturated"
		res.Recommendation = "Check I/O wait times, lock contention, or excessive concurrency."
	} else if ratio >= 1.5 {
		res.Status = model.StatusWarning
		res.Severity = model.SeverityWarning
		res.Description = "System load exceeds nominal CPU capacity"
		res.Recommendation = "Check if burst workload is temporary or if additional cores are required."
	} else {
		res.Status = model.StatusPass
		res.Severity = model.SeverityInfo
		res.Description = "Load average is well within core capacity limits"
	}

	return res
}

// MemoryRule checks RAM utilization and available memory.
type MemoryRule struct{}

func (r *MemoryRule) ID() string       { return "memory-utilization" }
func (r *MemoryRule) Category() string { return "Memory" }
func (r *MemoryRule) Name() string     { return "RAM Utilization" }

func (r *MemoryRule) Evaluate(ctx context.Context, snapshot *model.SystemSnapshot, cfg *config.Config) model.DiagnosticResult {
	res := model.DiagnosticResult{
		ID:        r.ID(),
		Category:  r.Category(),
		Name:      r.Name(),
		Timestamp: time.Now(),
	}

	if snapshot == nil || snapshot.Memory == nil || snapshot.Memory.TotalBytes == 0 {
		res.Status = model.StatusSkip
		res.Severity = model.SeverityInfo
		res.Description = "Memory metrics unavailable"
		return res
	}

	usedPct := snapshot.Memory.UsedPercent
	res.MetricValue = fmt.Sprintf("%.1f%% (Available: %s / %s)",
		usedPct,
		util.FormatBytes(snapshot.Memory.AvailableBytes),
		util.FormatBytes(snapshot.Memory.TotalBytes),
	)

	warnThresh := 80.0
	critThresh := 92.0
	if cfg != nil && cfg.Alerts.Memory.Enabled && cfg.Alerts.Memory.Threshold > 0 {
		warnThresh = cfg.Alerts.Memory.Threshold
		critThresh = min(warnThresh+10.0, 98.0)
	}
	res.Threshold = fmt.Sprintf("Warn: >%.0f%%, Crit: >%.0f%%", warnThresh, critThresh)

	if usedPct >= critThresh {
		res.Status = model.StatusFail
		res.Severity = model.SeverityCritical
		res.Description = "Critical memory pressure — risk of Out-Of-Memory (OOM) kills"
		res.Recommendation = "Inspect high-memory processes with `watchdog top --sort memory` or increase host RAM."
	} else if usedPct >= warnThresh {
		res.Status = model.StatusWarning
		res.Severity = model.SeverityWarning
		res.Description = "Elevated RAM usage"
		res.Recommendation = "Check for memory leaks or high cache allocations."
	} else {
		res.Status = model.StatusPass
		res.Severity = model.SeverityInfo
		res.Description = "Memory utilization is healthy"
	}

	return res
}

// SwapRule checks swap utilization.
type SwapRule struct{}

func (r *SwapRule) ID() string       { return "swap-utilization" }
func (r *SwapRule) Category() string { return "Memory" }
func (r *SwapRule) Name() string     { return "Swap Space Utilization" }

func (r *SwapRule) Evaluate(ctx context.Context, snapshot *model.SystemSnapshot, cfg *config.Config) model.DiagnosticResult {
	res := model.DiagnosticResult{
		ID:        r.ID(),
		Category:  r.Category(),
		Name:      r.Name(),
		Timestamp: time.Now(),
	}

	if snapshot == nil || snapshot.Memory == nil || snapshot.Memory.SwapTotalBytes == 0 {
		res.Status = model.StatusPass
		res.Severity = model.SeverityInfo
		res.MetricValue = "No Swap configured"
		res.Description = "Swap is not configured on this host"
		return res
	}

	swapPct := snapshot.Memory.SwapUsedPercent
	res.MetricValue = fmt.Sprintf("%.1f%% (%s / %s)",
		swapPct,
		util.FormatBytes(snapshot.Memory.SwapUsedBytes),
		util.FormatBytes(snapshot.Memory.SwapTotalBytes),
	)
	res.Threshold = "Warn: >50%, Crit: >80%"

	if swapPct >= 80.0 {
		res.Status = model.StatusFail
		res.Severity = model.SeverityCritical
		res.Description = "Swap space is critically exhausted — severe page thrashing likely"
		res.Recommendation = "Free up physical RAM or resize swap space to prevent lockups."
	} else if swapPct >= 50.0 {
		res.Status = model.StatusWarning
		res.Severity = model.SeverityWarning
		res.Description = "Swap utilization is high, indicating memory pressure"
		res.Recommendation = "Review processes actively swapping pages to disk."
	} else {
		res.Status = model.StatusPass
		res.Severity = model.SeverityInfo
		res.Description = "Swap usage is within safe operating parameters"
	}

	return res
}

// DiskSpaceRule checks all mounted disk partitions for capacity thresholds.
type DiskSpaceRule struct{}

func (r *DiskSpaceRule) ID() string       { return "disk-space" }
func (r *DiskSpaceRule) Category() string { return "Disk" }
func (r *DiskSpaceRule) Name() string     { return "Disk Partition Capacity" }

func (r *DiskSpaceRule) Evaluate(ctx context.Context, snapshot *model.SystemSnapshot, cfg *config.Config) model.DiagnosticResult {
	res := model.DiagnosticResult{
		ID:        r.ID(),
		Category:  r.Category(),
		Name:      r.Name(),
		Timestamp: time.Now(),
	}

	if snapshot == nil || snapshot.Disk == nil || len(snapshot.Disk.Partitions) == 0 {
		res.Status = model.StatusSkip
		res.Severity = model.SeverityInfo
		res.Description = "Disk partition metrics unavailable"
		return res
	}

	warnThresh := 80.0
	critThresh := 90.0
	if cfg != nil && cfg.Alerts.Disk.Enabled && cfg.Alerts.Disk.Threshold > 0 {
		warnThresh = cfg.Alerts.Disk.Threshold
		critThresh = min(warnThresh+10.0, 95.0)
	}
	res.Threshold = fmt.Sprintf("Warn: >%.0f%%, Crit: >%.0f%%", warnThresh, critThresh)

	var highPartitions []string
	var critPartitions []string
	var maxUsedPct float64

	for _, p := range snapshot.Disk.Partitions {
		if p.TotalBytes == 0 {
			continue
		}
		if p.UsedPercent > maxUsedPct {
			maxUsedPct = p.UsedPercent
		}
		if p.UsedPercent >= critThresh {
			critPartitions = append(critPartitions, fmt.Sprintf("%s (%.1f%%)", p.Mountpoint, p.UsedPercent))
		} else if p.UsedPercent >= warnThresh {
			highPartitions = append(highPartitions, fmt.Sprintf("%s (%.1f%%)", p.Mountpoint, p.UsedPercent))
		}
	}

	res.MetricValue = fmt.Sprintf("Max partition usage: %.1f%% (%d partitions checked)", maxUsedPct, len(snapshot.Disk.Partitions))

	if len(critPartitions) > 0 {
		res.Status = model.StatusFail
		res.Severity = model.SeverityCritical
		res.Description = fmt.Sprintf("Critical disk capacity on: %s", strings.Join(critPartitions, ", "))
		res.Recommendation = "Clean log files, temp directories, or expand disk volume immediately."
	} else if len(highPartitions) > 0 {
		res.Status = model.StatusWarning
		res.Severity = model.SeverityWarning
		res.Description = fmt.Sprintf("High disk capacity on: %s", strings.Join(highPartitions, ", "))
		res.Recommendation = "Audit disk usage with `watchdog disk` and plan partition cleanup."
	} else {
		res.Status = model.StatusPass
		res.Severity = model.SeverityInfo
		res.Description = "All monitored disk partitions have adequate free space"
	}

	return res
}

// InodeRule checks inode exhaustion on Unix filesystems.
type InodeRule struct{}

func (r *InodeRule) ID() string       { return "disk-inodes" }
func (r *InodeRule) Category() string { return "Disk" }
func (r *InodeRule) Name() string     { return "Inode Availability" }

func (r *InodeRule) Evaluate(ctx context.Context, snapshot *model.SystemSnapshot, cfg *config.Config) model.DiagnosticResult {
	res := model.DiagnosticResult{
		ID:        r.ID(),
		Category:  r.Category(),
		Name:      r.Name(),
		Timestamp: time.Now(),
	}

	if snapshot == nil || snapshot.Disk == nil || len(snapshot.Disk.Partitions) == 0 {
		res.Status = model.StatusSkip
		res.Severity = model.SeverityInfo
		res.Description = "Inode metrics unavailable"
		return res
	}

	var critInodes []string
	var warnInodes []string
	hasInodeData := false

	for _, p := range snapshot.Disk.Partitions {
		if p.InodesTotal > 0 {
			hasInodeData = true
			if p.InodesPct >= 95.0 {
				critInodes = append(critInodes, fmt.Sprintf("%s (%.1f%%)", p.Mountpoint, p.InodesPct))
			} else if p.InodesPct >= 85.0 {
				warnInodes = append(warnInodes, fmt.Sprintf("%s (%.1f%%)", p.Mountpoint, p.InodesPct))
			}
		}
	}

	if !hasInodeData {
		res.Status = model.StatusPass
		res.Severity = model.SeverityInfo
		res.MetricValue = "N/A (Non-Unix or unsupported FS)"
		res.Description = "Inode metrics not applicable on this filesystem"
		return res
	}

	res.Threshold = "Warn: >85%, Crit: >95%"
	if len(critInodes) > 0 {
		res.Status = model.StatusFail
		res.Severity = model.SeverityCritical
		res.Description = fmt.Sprintf("Critical inode exhaustion on: %s", strings.Join(critInodes, ", "))
		res.Recommendation = "Delete large directories of small files, temp sessions, or orphaned caches."
	} else if len(warnInodes) > 0 {
		res.Status = model.StatusWarning
		res.Severity = model.SeverityWarning
		res.Description = fmt.Sprintf("Elevated inode usage on: %s", strings.Join(warnInodes, ", "))
		res.Recommendation = "Investigate applications creating numerous tiny files."
	} else {
		res.Status = model.StatusPass
		res.Severity = model.SeverityInfo
		res.Description = "Inode availability is healthy across all partitions"
	}

	return res
}

// DNSResolutionRule tests DNS lookup latency and success.
type DNSResolutionRule struct{}

func (r *DNSResolutionRule) ID() string       { return "dns-resolution" }
func (r *DNSResolutionRule) Category() string { return "Network" }
func (r *DNSResolutionRule) Name() string     { return "DNS Resolution & Latency" }

func (r *DNSResolutionRule) Evaluate(ctx context.Context, snapshot *model.SystemSnapshot, cfg *config.Config) model.DiagnosticResult {
	res := model.DiagnosticResult{
		ID:        r.ID(),
		Category:  r.Category(),
		Name:      r.Name(),
		Timestamp: time.Now(),
	}

	targetDomain := "cloudflare-dns.com"
	start := time.Now()
	rCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	addrs, err := net.DefaultResolver.LookupHost(rCtx, targetDomain)
	latency := time.Since(start)

	res.Threshold = "Warn: >500ms or lookup error"
	if err != nil {
		res.Status = model.StatusFail
		res.Severity = model.SeverityCritical
		res.MetricValue = fmt.Sprintf("Failed: %v", err)
		res.Description = fmt.Sprintf("DNS lookup for %s failed", targetDomain)
		res.Recommendation = "Verify local DNS settings (/etc/resolv.conf or Windows DNS adapters) and upstream nameservers."
	} else if latency > 500*time.Millisecond {
		res.Status = model.StatusWarning
		res.Severity = model.SeverityWarning
		res.MetricValue = fmt.Sprintf("%d ms (%d IPs)", latency.Milliseconds(), len(addrs))
		res.Description = "DNS resolution latency is unusually high"
		res.Recommendation = "Check DNS server health or configure faster fallback nameservers (e.g. 1.1.1.1, 8.8.8.8)."
	} else {
		res.Status = model.StatusPass
		res.Severity = model.SeverityInfo
		res.MetricValue = fmt.Sprintf("%d ms", latency.Milliseconds())
		res.Description = fmt.Sprintf("DNS lookup successful (%d ms)", latency.Milliseconds())
	}

	return res
}

// NetworkConnectivityRule tests external TCP connectivity to reliable public endpoints.
type NetworkConnectivityRule struct{}

func (r *NetworkConnectivityRule) ID() string       { return "network-connectivity" }
func (r *NetworkConnectivityRule) Category() string { return "Network" }
func (r *NetworkConnectivityRule) Name() string     { return "Internet Connectivity & Gateway" }

func (r *NetworkConnectivityRule) Evaluate(ctx context.Context, snapshot *model.SystemSnapshot, cfg *config.Config) model.DiagnosticResult {
	res := model.DiagnosticResult{
		ID:        r.ID(),
		Category:  r.Category(),
		Name:      r.Name(),
		Timestamp: time.Now(),
	}

	endpoints := []string{"1.1.1.1:53", "8.8.8.8:53"}
	var successful []string
	var latencies []time.Duration

	for _, ep := range endpoints {
		start := time.Now()
		conn, err := net.DialTimeout("tcp", ep, 2*time.Second)
		if err == nil {
			latencies = append(latencies, time.Since(start))
			successful = append(successful, ep)
			conn.Close()
		}
	}

	if len(successful) == 0 {
		res.Status = model.StatusFail
		res.Severity = model.SeverityCritical
		res.MetricValue = "Unreachable"
		res.Description = "No external network connectivity could be established"
		res.Recommendation = "Verify network interface cable/Wi-Fi connection, default gateway route, and firewall rules."
	} else {
		avgLat := latencies[0]
		res.Status = model.StatusPass
		res.Severity = model.SeverityInfo
		res.MetricValue = fmt.Sprintf("%d ms (Connected to %s)", avgLat.Milliseconds(), successful[0])
		res.Description = "Outbound internet connectivity is verified and operational"
	}

	return res
}

// ProcessHealthRule inspects processes for zombies and rogue CPU/memory hogs.
type ProcessHealthRule struct{}

func (r *ProcessHealthRule) ID() string       { return "process-health" }
func (r *ProcessHealthRule) Category() string { return "Process" }
func (r *ProcessHealthRule) Name() string     { return "Process Table & Rogue Tasks" }

func (r *ProcessHealthRule) Evaluate(ctx context.Context, snapshot *model.SystemSnapshot, cfg *config.Config) model.DiagnosticResult {
	res := model.DiagnosticResult{
		ID:        r.ID(),
		Category:  r.Category(),
		Name:      r.Name(),
		Timestamp: time.Now(),
	}

	if snapshot == nil || snapshot.Processes == nil {
		res.Status = model.StatusSkip
		res.Severity = model.SeverityInfo
		res.Description = "Process summary unavailable"
		return res
	}

	zombieCount := snapshot.Processes.ZombieCount
	var rogueProcs []string
	currentPID := int32(os.Getpid())

	for _, p := range snapshot.Processes.Processes {
		if p.PID == currentPID {
			continue // Exclude inspecting diagnostic process to avoid self-detection false positives
		}
		if p.CPUPercent >= 90.0 {
			rogueProcs = append(rogueProcs, fmt.Sprintf("%s (PID %d: %.1f%% CPU)", p.Name, p.PID, p.CPUPercent))
		}
	}

	res.MetricValue = fmt.Sprintf("%d total procs (%d zombies, %d rogue procs)",
		snapshot.Processes.TotalCount, zombieCount, len(rogueProcs))

	if zombieCount > 10 {
		res.Status = model.StatusWarning
		res.Severity = model.SeverityWarning
		res.Description = fmt.Sprintf("High number of zombie/defunct processes detected (%d)", zombieCount)
		res.Recommendation = "Inspect parent processes failing to wait() on child exits."
	} else if len(rogueProcs) > 0 {
		res.Status = model.StatusWarning
		res.Severity = model.SeverityWarning
		res.Description = fmt.Sprintf("Rogue high-CPU processes: %s", strings.Join(rogueProcs, ", "))
		res.Recommendation = "Inspect or renice runaway tasks using `watchdog top`."
	} else {
		res.Status = model.StatusPass
		res.Severity = model.SeverityInfo
		res.Description = "Process table state is clean with no rogue tasks"
	}

	return res
}

// DockerHealthRule checks Docker container status if Docker is running.
type DockerHealthRule struct{}

func (r *DockerHealthRule) ID() string       { return "docker-health" }
func (r *DockerHealthRule) Category() string { return "Docker" }
func (r *DockerHealthRule) Name() string     { return "Container Health & Restarts" }

func (r *DockerHealthRule) Evaluate(ctx context.Context, snapshot *model.SystemSnapshot, cfg *config.Config) model.DiagnosticResult {
	res := model.DiagnosticResult{
		ID:        r.ID(),
		Category:  r.Category(),
		Name:      r.Name(),
		Timestamp: time.Now(),
	}

	if snapshot == nil || snapshot.Docker == nil || !snapshot.Docker.Available {
		res.Status = model.StatusSkip
		res.Severity = model.SeverityInfo
		res.MetricValue = "Docker not active"
		res.Description = "Docker daemon is not running on this host"
		return res
	}

	var crashLooping []string
	for _, c := range snapshot.Docker.Containers {
		if c.RestartCount >= 5 {
			name := c.ID
			if len(c.Names) > 0 {
				name = c.Names[0]
			}
			crashLooping = append(crashLooping, fmt.Sprintf("%s (%d restarts)", name, c.RestartCount))
		}
	}

	res.MetricValue = fmt.Sprintf("%d running / %d total containers", snapshot.Docker.RunningCount, snapshot.Docker.ContainersTotal)

	if len(crashLooping) > 0 {
		res.Status = model.StatusFail
		res.Severity = model.SeverityCritical
		res.Description = fmt.Sprintf("Crash-looping containers detected: %s", strings.Join(crashLooping, ", "))
		res.Recommendation = "Inspect container logs with `docker logs <container>` to diagnose startup failure."
	} else {
		res.Status = model.StatusPass
		res.Severity = model.SeverityInfo
		res.Description = "Docker containers are running normally"
	}

	return res
}
