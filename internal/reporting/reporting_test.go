package reporting

import (
	"strings"
	"testing"
	"time"

	"github.com/watchdog-cli/watchdog/pkg/model"
)

func createSampleSnapshot() *model.SystemSnapshot {
	now := time.Now()
	return &model.SystemSnapshot{
		Timestamp: now,
		System: &model.SystemInfo{
			Hostname:        "prod-srv-01",
			OS:              "linux",
			Platform:        "ubuntu",
			PlatformVersion: "22.04",
			Uptime:          72 * time.Hour,
			BootTime:        now.Add(-72 * time.Hour),
		},
		CPU: &model.CPUInfo{
			OverallUsage: 45.5,
			LogicalCores: 8,
			LoadAverage: model.LoadAvg{
				Load1:  2.1,
				Load5:  1.8,
				Load15: 1.5,
			},
		},
		Memory: &model.MemoryInfo{
			TotalBytes:      16 * 1024 * 1024 * 1024,
			UsedBytes:       8 * 1024 * 1024 * 1024,
			AvailableBytes:  8 * 1024 * 1024 * 1024,
			UsedPercent:     50.0,
			SwapTotalBytes:  4 * 1024 * 1024 * 1024,
			SwapUsedBytes:   512 * 1024 * 1024,
			SwapUsedPercent: 12.5,
		},
		Disk: &model.DiskInfo{
			TotalBytes:  500 * 1024 * 1024 * 1024,
			UsedBytes:   250 * 1024 * 1024 * 1024,
			FreeBytes:   250 * 1024 * 1024 * 1024,
			UsedPercent: 50.0,
			IOCounters: []model.DiskIOInfo{
				{
					Name:      "sda",
					ReadRate:  1024 * 1024 * 5,
					WriteRate: 1024 * 1024 * 10,
				},
			},
		},
		Network: &model.NetworkInfo{
			TotalRxRate:    1024 * 500,
			TotalTxRate:    1024 * 250,
			TotalBytesRecv: 1024 * 1024 * 1024 * 10,
			TotalBytesSent: 1024 * 1024 * 1024 * 5,
		},
		Processes: &model.ProcessSummary{
			TotalCount: 150,
			Processes: []model.ProcessInfo{
				{
					PID:           1024,
					PPID:          1,
					Name:          "postgres",
					Username:      "postgres",
					Status:        "running",
					CPUPercent:    25.0,
					MemoryPercent: 15.0,
					MemoryRSS:     1024 * 1024 * 500,
					NumThreads:    12,
					CommandLine:   "/usr/lib/postgresql/bin/postgres -D /var/lib/postgresql/data",
				},
			},
		},
	}
}

func TestBuildReportDataAndJSON(t *testing.T) {
	snap := createSampleSnapshot()
	diag := &model.DiagnosticReport{
		GeneratedAt:    time.Now(),
		OverallStatus:  model.StatusHealthy,
		PassedChecks:   10,
		WarningChecks:  0,
		CriticalChecks: 0,
		Results: []model.DiagnosticResult{
			{
				Category: "CPU",
				RuleName: "CPU Utilization",
				Status:   model.StatusHealthy,
				Message:  "CPU utilization within nominal bounds",
			},
		},
	}

	report := BuildReportData("Test Report", snap, diag, nil, nil)
	if report.Host.Hostname != "prod-srv-01" {
		t.Fatalf("expected hostname prod-srv-01, got %s", report.Host.Hostname)
	}

	jsonData, err := GenerateJSON(report)
	if err != nil {
		t.Fatalf("GenerateJSON failed: %v", err)
	}
	if !strings.Contains(string(jsonData), "prod-srv-01") {
		t.Errorf("JSON output does not contain expected hostname")
	}
}

func TestGenerateCSV(t *testing.T) {
	snap := createSampleSnapshot()
	csvData, err := GenerateSnapshotsCSV([]*model.SystemSnapshot{snap})
	if err != nil {
		t.Fatalf("GenerateSnapshotsCSV failed: %v", err)
	}
	if !strings.Contains(string(csvData), "cpu_overall_usage") {
		t.Errorf("CSV header missing")
	}

	procCSV, err := GenerateProcessesCSV(snap.Processes.Processes)
	if err != nil {
		t.Fatalf("GenerateProcessesCSV failed: %v", err)
	}
	if !strings.Contains(string(procCSV), "postgres") {
		t.Errorf("Process CSV missing postgres")
	}
}

func TestGenerateTerminal(t *testing.T) {
	snap := createSampleSnapshot()
	diag := &model.DiagnosticReport{
		GeneratedAt:    time.Now(),
		OverallStatus:  model.StatusHealthy,
		PassedChecks:   8,
		WarningChecks:  0,
		CriticalChecks: 0,
		Results: []model.DiagnosticResult{
			{
				Category: "Memory",
				RuleName: "RAM Saturation",
				Status:   model.StatusHealthy,
				Message:  "Memory healthy",
			},
		},
	}

	report := BuildReportData("Terminal Test", snap, diag, nil, nil)
	termOut := GenerateTerminal(report)
	if !strings.Contains(termOut, "PROD-SRV-01") {
		t.Errorf("terminal report missing hostname banner")
	}
	if !strings.Contains(termOut, "postgres") {
		t.Errorf("terminal report missing process table entry")
	}
}

func TestGenerateHTML(t *testing.T) {
	snap1 := createSampleSnapshot()
	snap2 := createSampleSnapshot()
	snap2.CPU.OverallUsage = 60.0
	snap2.Memory.UsedPercent = 55.0

	diag := &model.DiagnosticReport{
		GeneratedAt:    time.Now(),
		OverallStatus:  model.StatusWarning,
		PassedChecks:   7,
		WarningChecks:  1,
		CriticalChecks: 0,
		Results: []model.DiagnosticResult{
			{
				Category:    "Disk",
				RuleName:    "Root Partition Space",
				Status:      model.StatusWarning,
				Message:     "Disk space at 82%",
				Remediation: "Clean up /var/log",
			},
		},
	}

	alerts := []model.AlertEvent{
		{
			ID:          1,
			RuleName:    "High Disk Space",
			MetricName:  "disk_used_pct",
			Severity:    model.SeverityWarning,
			Status:      model.AlertStatusActive,
			ActualValue: 82.0,
			Threshold:   80.0,
			Message:     "Disk space exceeded 80%",
			FiredAt:     time.Now(),
		},
	}

	anomalies := &model.AnomalyReport{
		GeneratedAt:    time.Now(),
		TotalChecked:   10,
		AnomaliesCount: 1,
		Scores: []model.AnomalyScore{
			{
				MetricName:   "disk_used_pct",
				CurrentValue: 82.0,
				Mean:         65.0,
				StdDev:       3.0,
				ZScore:       5.66,
				DeviationPct: 26.1,
				IsAnomaly:    true,
				Severity:     model.SeverityWarning,
				Explanation:  "Spike detected: 82.0",
			},
		},
	}

	report := BuildReportData("Production Dashboard", snap2, diag, alerts, anomalies)
	htmlBytes, err := GenerateHTML(report, HTMLReportOptions{
		Title:         "Production Server Health Dashboard",
		History:       []*model.SystemSnapshot{snap1, snap2},
		IncludeCharts: true,
	})
	if err != nil {
		t.Fatalf("GenerateHTML failed: %v", err)
	}

	htmlStr := string(htmlBytes)
	if !strings.Contains(htmlStr, "<!DOCTYPE html>") {
		t.Errorf("HTML missing doctype")
	}
	if !strings.Contains(htmlStr, "Production Server Health Dashboard") {
		t.Errorf("HTML missing custom title")
	}
	if !strings.Contains(htmlStr, "sparkline") {
		t.Errorf("HTML missing rendered SVG sparklines")
	}
	if !strings.Contains(htmlStr, "High Disk Space") {
		t.Errorf("HTML missing alert rule name")
	}
	if !strings.Contains(htmlStr, "Clean up /var/log") {
		t.Errorf("HTML missing remediation suggestion")
	}
}
