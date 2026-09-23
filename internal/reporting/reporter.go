package reporting

import (
	"sort"
	"time"

	"github.com/watchdog-cli/watchdog/pkg/model"
)

// BuildReportData constructs a unified ReportData object from snapshots and sub-reports.
func BuildReportData(
	title string,
	snap *model.SystemSnapshot,
	diag *model.DiagnosticReport,
	alerts []model.AlertEvent,
	anomalies *model.AnomalyReport,
) *model.ReportData {
	if snap == nil {
		snap = &model.SystemSnapshot{Timestamp: time.Now()}
	}

	report := &model.ReportData{
		Title:        title,
		GeneratedAt:  time.Now(),
		Diagnostics:  diag,
		ActiveAlerts: alerts,
		Anomalies:    anomalies,
	}

	if report.Title == "" {
		report.Title = "Watchdog System Health Report"
	}

	if snap.System != nil {
		report.Host = *snap.System
	} else {
		report.Host = model.SystemInfo{
			Hostname:    "unknown",
			CollectedAt: snap.Timestamp,
		}
	}

	if snap.CPU != nil {
		report.CPU = *snap.CPU
	}
	if snap.Memory != nil {
		report.Memory = *snap.Memory
	}
	if snap.Disk != nil {
		report.Disk = *snap.Disk
	}
	if snap.Network != nil {
		report.Network = *snap.Network
	}
	if snap.Processes != nil && len(snap.Processes.Processes) > 0 {
		// Sort top processes by CPU descending
		procsCopy := make([]model.ProcessInfo, len(snap.Processes.Processes))
		copy(procsCopy, snap.Processes.Processes)
		sort.Slice(procsCopy, func(i, j int) bool {
			return procsCopy[i].CPUPercent > procsCopy[j].CPUPercent
		})
		if len(procsCopy) > 20 {
			procsCopy = procsCopy[:20]
		}
		report.TopProcesses = procsCopy
	}
	if snap.Docker != nil {
		report.Docker = snap.Docker
	}
	if snap.Kubernetes != nil {
		report.Kubernetes = snap.Kubernetes
	}

	return report
}
