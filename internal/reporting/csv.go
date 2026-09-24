package reporting

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

// GenerateSnapshotsCSV exports a series of system snapshots to a CSV format.
func GenerateSnapshotsCSV(snapshots []*model.SystemSnapshot) ([]byte, error) {
	buf := new(bytes.Buffer)
	w := csv.NewWriter(buf)

	// Header
	header := []string{
		"timestamp",
		"cpu_overall_usage",
		"cpu_load1",
		"cpu_load5",
		"cpu_load15",
		"memory_used_pct",
		"memory_used_bytes",
		"memory_total_bytes",
		"swap_used_pct",
		"disk_used_pct",
		"disk_read_bytes_sec",
		"disk_write_bytes_sec",
		"net_rx_bytes_sec",
		"net_tx_bytes_sec",
		"processes_total",
		"processes_zombies",
	}
	if err := w.Write(header); err != nil {
		return nil, err
	}

	for _, s := range snapshots {
		if s == nil {
			continue
		}

		var cpuUsage, load1, load5, load15 float64
		if s.CPU != nil {
			cpuUsage = s.CPU.OverallUsage
			load1 = s.CPU.LoadAverage.Load1
			load5 = s.CPU.LoadAverage.Load5
			load15 = s.CPU.LoadAverage.Load15
		}

		var memPct, swapPct float64
		var memUsed, memTotal uint64
		if s.Memory != nil {
			memPct = s.Memory.UsedPercent
			memUsed = s.Memory.UsedBytes
			memTotal = s.Memory.TotalBytes
			swapPct = s.Memory.SwapUsedPercent
		}

		var diskPct float64
		var diskReadRate, diskWriteRate float64
		if s.Disk != nil {
			diskPct = s.Disk.UsedPercent
			for _, io := range s.Disk.IOCounters {
				diskReadRate += io.ReadRate
				diskWriteRate += io.WriteRate
			}
		}

		var netRxRate, netTxRate float64
		if s.Network != nil {
			netRxRate = s.Network.TotalRxRate
			netTxRate = s.Network.TotalTxRate
		}

		var procTotal, procZombie int
		if s.Processes != nil {
			procTotal = s.Processes.TotalCount
			procZombie = s.Processes.ZombieCount
		}

		row := []string{
			s.Timestamp.Format(time.RFC3339),
			fmt.Sprintf("%.2f", cpuUsage),
			fmt.Sprintf("%.2f", load1),
			fmt.Sprintf("%.2f", load5),
			fmt.Sprintf("%.2f", load15),
			fmt.Sprintf("%.2f", memPct),
			fmt.Sprintf("%d", memUsed),
			fmt.Sprintf("%d", memTotal),
			fmt.Sprintf("%.2f", swapPct),
			fmt.Sprintf("%.2f", diskPct),
			fmt.Sprintf("%.2f", diskReadRate),
			fmt.Sprintf("%.2f", diskWriteRate),
			fmt.Sprintf("%.2f", netRxRate),
			fmt.Sprintf("%.2f", netTxRate),
			fmt.Sprintf("%d", procTotal),
			fmt.Sprintf("%d", procZombie),
		}

		if err := w.Write(row); err != nil {
			return nil, err
		}
	}

	w.Flush()
	return buf.Bytes(), w.Error()
}

// GenerateProcessesCSV exports process table to CSV.
func GenerateProcessesCSV(procs []model.ProcessInfo) ([]byte, error) {
	buf := new(bytes.Buffer)
	w := csv.NewWriter(buf)

	header := []string{"pid", "ppid", "name", "username", "status", "cpu_percent", "mem_percent", "rss_bytes", "num_threads", "command"}
	if err := w.Write(header); err != nil {
		return nil, err
	}

	for _, p := range procs {
		row := []string{
			fmt.Sprintf("%d", p.PID),
			fmt.Sprintf("%d", p.PPID),
			p.Name,
			p.Username,
			p.Status,
			fmt.Sprintf("%.2f", p.CPUPercent),
			fmt.Sprintf("%.2f", p.MemoryPercent),
			fmt.Sprintf("%d", p.MemoryRSS),
			fmt.Sprintf("%d", p.NumThreads),
			p.CommandLine,
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}

	w.Flush()
	return buf.Bytes(), w.Error()
}

// GenerateAlertsCSV exports alert events to CSV.
func GenerateAlertsCSV(alerts []model.AlertEvent) ([]byte, error) {
	buf := new(bytes.Buffer)
	w := csv.NewWriter(buf)

	header := []string{"id", "rule_id", "rule_name", "severity", "is_active", "metric_name", "actual_value", "threshold", "message", "fired_at", "resolved_at"}
	if err := w.Write(header); err != nil {
		return nil, err
	}

	for _, a := range alerts {
		resolvedAtStr := ""
		if a.ResolvedAt != nil {
			resolvedAtStr = a.ResolvedAt.Format(time.RFC3339)
		}

		isActiveStr := "RESOLVED"
		if a.IsActive {
			isActiveStr = "ACTIVE"
		}

		row := []string{
			a.ID,
			a.RuleID,
			a.RuleName,
			string(a.Severity),
			isActiveStr,
			a.MetricName,
			fmt.Sprintf("%.2f", a.ActualValue),
			fmt.Sprintf("%.2f", a.Threshold),
			a.Message,
			a.FiredAt.Format(time.RFC3339),
			resolvedAtStr,
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}

	w.Flush()
	return buf.Bytes(), w.Error()
}
