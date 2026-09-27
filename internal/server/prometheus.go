package server

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/DocHoax/watchdog/internal/intelligence"
	"github.com/DocHoax/watchdog/pkg/model"
)

// PrometheusExporter generates Prometheus-compatible metrics from snapshots and reports.
type PrometheusExporter struct {
	mu           sync.RWMutex
	lastSnapshot *model.SystemSnapshot
	lastDiag     *model.DiagnosticReport
	activeAlerts []model.AlertEvent
	anomalies    *model.AnomalyReport

	intelSummary     *intelligence.FleetHealthSummary
	nodeHealthScores map[string]float64
	evalDuration     time.Duration
}

// NewPrometheusExporter creates a new exporter instance.
func NewPrometheusExporter() *PrometheusExporter {
	return &PrometheusExporter{}
}

// Update updates the exporter with the latest system data.
func (e *PrometheusExporter) Update(
	snap *model.SystemSnapshot,
	diag *model.DiagnosticReport,
	alerts []model.AlertEvent,
	anomalies *model.AnomalyReport,
) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lastSnapshot = snap
	e.lastDiag = diag
	e.activeAlerts = alerts
	e.anomalies = anomalies
}

// UpdateIntelligence updates the exporter with fleet health summary and evaluation duration.
func (e *PrometheusExporter) UpdateIntelligence(
	summary *intelligence.FleetHealthSummary,
	evalDuration time.Duration,
) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.intelSummary = summary
	e.evalDuration = evalDuration
}

// Handler returns an HTTP handler for serving Prometheus metrics.
func (e *PrometheusExporter) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		metrics := e.RenderMetrics()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(metrics))
	}
}

// RenderMetrics renders the Prometheus text exposition format.
func (e *PrometheusExporter) RenderMetrics() string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var sb strings.Builder

	// Build Info & Operational Status
	sb.WriteString("# HELP watchdog_build_info Build and version information\n")
	sb.WriteString("# TYPE watchdog_build_info gauge\n")
	sb.WriteString(fmt.Sprintf("watchdog_build_info{version=\"1.0.0\",go_version=\"%s\",platform=\"%s/%s\"} 1\n",
		runtime.Version(), runtime.GOOS, runtime.GOARCH))

	sb.WriteString("# HELP watchdog_up Telemetry collection operational status (1=operational, 0=failing)\n")
	sb.WriteString("# TYPE watchdog_up gauge\n")
	if e.lastSnapshot != nil {
		sb.WriteString("watchdog_up 1\n")
	} else {
		sb.WriteString("watchdog_up 0\n")
	}

	sb.WriteString("# HELP watchdog_health_status System overall health status (1=active, 0=inactive)\n")
	sb.WriteString("# TYPE watchdog_health_status gauge\n")
	status := "ok"
	if e.lastDiag != nil {
		if e.lastDiag.CriticalChecks > 0 {
			status = "critical"
		} else if e.lastDiag.WarningChecks > 0 {
			status = "warning"
		}
	}
	sb.WriteString(fmt.Sprintf("watchdog_health_status{status=\"%s\"} 1\n", status))

	if e.lastSnapshot == nil {
		return sb.String()
	}

	snap := e.lastSnapshot

	// CPU Metrics
	if snap.CPU != nil {
		sb.WriteString("# HELP watchdog_cpu_usage_percent Overall CPU utilization percentage\n")
		sb.WriteString("# TYPE watchdog_cpu_usage_percent gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_cpu_usage_percent %.2f\n", snap.CPU.OverallUsage))

		sb.WriteString("# HELP watchdog_cpu_cores_logical Total logical CPU cores\n")
		sb.WriteString("# TYPE watchdog_cpu_cores_logical gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_cpu_cores_logical %d\n", snap.CPU.LogicalCores))

		if len(snap.CPU.Cores) > 0 {
			sb.WriteString("# HELP watchdog_cpu_core_usage_percent Per-core CPU utilization percentage\n")
			sb.WriteString("# TYPE watchdog_cpu_core_usage_percent gauge\n")
			for _, core := range snap.CPU.Cores {
				sb.WriteString(fmt.Sprintf("watchdog_cpu_core_usage_percent{core=\"%d\"} %.2f\n", core.Index, core.UsagePct))
			}
		}

		sb.WriteString("# HELP watchdog_cpu_load1 1-minute load average\n")
		sb.WriteString("# TYPE watchdog_cpu_load1 gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_cpu_load1 %.2f\n", snap.CPU.LoadAverage.Load1))

		sb.WriteString("# HELP watchdog_cpu_load5 5-minute load average\n")
		sb.WriteString("# TYPE watchdog_cpu_load5 gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_cpu_load5 %.2f\n", snap.CPU.LoadAverage.Load5))

		sb.WriteString("# HELP watchdog_cpu_load15 15-minute load average\n")
		sb.WriteString("# TYPE watchdog_cpu_load15 gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_cpu_load15 %.2f\n", snap.CPU.LoadAverage.Load15))
	}

	// Memory Metrics
	if snap.Memory != nil {
		sb.WriteString("# HELP watchdog_memory_total_bytes Total physical memory in bytes\n")
		sb.WriteString("# TYPE watchdog_memory_total_bytes gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_memory_total_bytes %d\n", snap.Memory.TotalBytes))

		sb.WriteString("# HELP watchdog_memory_used_bytes Used physical memory in bytes\n")
		sb.WriteString("# TYPE watchdog_memory_used_bytes gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_memory_used_bytes %d\n", snap.Memory.UsedBytes))

		sb.WriteString("# HELP watchdog_memory_available_bytes Available physical memory in bytes\n")
		sb.WriteString("# TYPE watchdog_memory_available_bytes gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_memory_available_bytes %d\n", snap.Memory.AvailableBytes))

		sb.WriteString("# HELP watchdog_memory_used_percent Memory utilization percentage\n")
		sb.WriteString("# TYPE watchdog_memory_used_percent gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_memory_used_percent %.2f\n", snap.Memory.UsedPercent))

		sb.WriteString("# HELP watchdog_swap_total_bytes Total swap memory in bytes\n")
		sb.WriteString("# TYPE watchdog_swap_total_bytes gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_swap_total_bytes %d\n", snap.Memory.SwapTotalBytes))

		sb.WriteString("# HELP watchdog_swap_used_bytes Used swap memory in bytes\n")
		sb.WriteString("# TYPE watchdog_swap_used_bytes gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_swap_used_bytes %d\n", snap.Memory.SwapUsedBytes))

		sb.WriteString("# HELP watchdog_swap_used_percent Swap utilization percentage\n")
		sb.WriteString("# TYPE watchdog_swap_used_percent gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_swap_used_percent %.2f\n", snap.Memory.SwapUsedPercent))
	}

	// Disk Metrics
	if snap.Disk != nil {
		sb.WriteString("# HELP watchdog_disk_total_bytes Total disk space in bytes\n")
		sb.WriteString("# TYPE watchdog_disk_total_bytes gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_disk_total_bytes %d\n", snap.Disk.TotalBytes))

		sb.WriteString("# HELP watchdog_disk_used_bytes Used disk space in bytes\n")
		sb.WriteString("# TYPE watchdog_disk_used_bytes gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_disk_used_bytes %d\n", snap.Disk.UsedBytes))

		sb.WriteString("# HELP watchdog_disk_free_bytes Free disk space in bytes\n")
		sb.WriteString("# TYPE watchdog_disk_free_bytes gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_disk_free_bytes %d\n", snap.Disk.FreeBytes))

		sb.WriteString("# HELP watchdog_disk_used_percent Disk usage percentage\n")
		sb.WriteString("# TYPE watchdog_disk_used_percent gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_disk_used_percent %.2f\n", snap.Disk.UsedPercent))

		if len(snap.Disk.Partitions) > 0 {
			sb.WriteString("# HELP watchdog_disk_partition_used_percent Disk partition usage percentage\n")
			sb.WriteString("# TYPE watchdog_disk_partition_used_percent gauge\n")
			for _, p := range snap.Disk.Partitions {
				escapedMount := strings.ReplaceAll(p.Mountpoint, "\\", "\\\\")
				sb.WriteString(fmt.Sprintf("watchdog_disk_partition_used_percent{mount=\"%s\",fstype=\"%s\"} %.2f\n",
					escapedMount, p.FSType, p.UsedPercent))
			}
		}

		if len(snap.Disk.IOCounters) > 0 {
			sb.WriteString("# HELP watchdog_disk_read_rate_bytes_per_second Disk read throughput\n")
			sb.WriteString("# TYPE watchdog_disk_read_rate_bytes_per_second gauge\n")
			for _, io := range snap.Disk.IOCounters {
				sb.WriteString(fmt.Sprintf("watchdog_disk_read_rate_bytes_per_second{device=\"%s\"} %.2f\n", io.Name, io.ReadRate))
			}

			sb.WriteString("# HELP watchdog_disk_write_rate_bytes_per_second Disk write throughput\n")
			sb.WriteString("# TYPE watchdog_disk_write_rate_bytes_per_second gauge\n")
			for _, io := range snap.Disk.IOCounters {
				sb.WriteString(fmt.Sprintf("watchdog_disk_write_rate_bytes_per_second{device=\"%s\"} %.2f\n", io.Name, io.WriteRate))
			}
		}
	}

	// Network Metrics
	if snap.Network != nil {
		sb.WriteString("# HELP watchdog_network_rx_rate_bytes_per_second Total inbound network rate in bytes/sec\n")
		sb.WriteString("# TYPE watchdog_network_rx_rate_bytes_per_second gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_network_rx_rate_bytes_per_second %.2f\n", snap.Network.TotalRxRate))

		sb.WriteString("# HELP watchdog_network_tx_rate_bytes_per_second Total outbound network rate in bytes/sec\n")
		sb.WriteString("# TYPE watchdog_network_tx_rate_bytes_per_second gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_network_tx_rate_bytes_per_second %.2f\n", snap.Network.TotalTxRate))

		if len(snap.Network.IOStats) > 0 {
			sb.WriteString("# HELP watchdog_network_interface_rx_rate Network interface receive rate\n")
			sb.WriteString("# TYPE watchdog_network_interface_rx_rate gauge\n")
			for _, io := range snap.Network.IOStats {
				sb.WriteString(fmt.Sprintf("watchdog_network_interface_rx_rate{interface=\"%s\"} %.2f\n", io.Name, io.RxRate))
			}

			sb.WriteString("# HELP watchdog_network_interface_tx_rate Network interface transmit rate\n")
			sb.WriteString("# TYPE watchdog_network_interface_tx_rate gauge\n")
			for _, io := range snap.Network.IOStats {
				sb.WriteString(fmt.Sprintf("watchdog_network_interface_tx_rate{interface=\"%s\"} %.2f\n", io.Name, io.TxRate))
			}
		}
	}

	// Processes Metrics
	if snap.Processes != nil {
		sb.WriteString("# HELP watchdog_processes_total Total number of processes\n")
		sb.WriteString("# TYPE watchdog_processes_total gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_processes_total %d\n", snap.Processes.TotalCount))

		sb.WriteString("# HELP watchdog_processes_running Number of running processes\n")
		sb.WriteString("# TYPE watchdog_processes_running gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_processes_running %d\n", snap.Processes.RunningCount))

		sb.WriteString("# HELP watchdog_processes_sleeping Number of sleeping processes\n")
		sb.WriteString("# TYPE watchdog_processes_sleeping gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_processes_sleeping %d\n", snap.Processes.SleepingCount))

		sb.WriteString("# HELP watchdog_processes_zombies Number of zombie processes\n")
		sb.WriteString("# TYPE watchdog_processes_zombies gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_processes_zombies %d\n", snap.Processes.ZombieCount))
	}

	// Docker Metrics
	if snap.Docker != nil {
		sb.WriteString("# HELP watchdog_docker_containers_total Total Docker containers discovered\n")
		sb.WriteString("# TYPE watchdog_docker_containers_total gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_docker_containers_total %d\n", snap.Docker.ContainersTotal))

		sb.WriteString("# HELP watchdog_docker_containers_running Running Docker containers\n")
		sb.WriteString("# TYPE watchdog_docker_containers_running gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_docker_containers_running %d\n", snap.Docker.RunningCount))
	}

	// Kubernetes Metrics
	if snap.Kubernetes != nil {
		sb.WriteString("# HELP watchdog_k8s_pods_total Total Kubernetes pods\n")
		sb.WriteString("# TYPE watchdog_k8s_pods_total gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_k8s_pods_total %d\n", snap.Kubernetes.TotalPods))

		sb.WriteString("# HELP watchdog_k8s_pods_running Running Kubernetes pods\n")
		sb.WriteString("# TYPE watchdog_k8s_pods_running gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_k8s_pods_running %d\n", snap.Kubernetes.RunningPods))
	}

	// Active Alerts
	sb.WriteString("# HELP watchdog_active_alerts_total Current number of active alerts\n")
	sb.WriteString("# TYPE watchdog_active_alerts_total gauge\n")
	critCount := 0
	warnCount := 0
	for _, a := range e.activeAlerts {
		if a.Severity == model.SeverityCritical {
			critCount++
		} else {
			warnCount++
		}
	}
	sb.WriteString(fmt.Sprintf("watchdog_active_alerts_total{severity=\"critical\"} %d\n", critCount))
	sb.WriteString(fmt.Sprintf("watchdog_active_alerts_total{severity=\"warning\"} %d\n", warnCount))

	// Anomaly Metrics
	if e.anomalies != nil {
		sb.WriteString("# HELP watchdog_anomaly_detected Flag indicating metric statistical anomaly (1=yes, 0=no)\n")
		sb.WriteString("# TYPE watchdog_anomaly_detected gauge\n")
		for _, s := range e.anomalies.Scores {
			flag := 0
			if s.IsAnomaly {
				flag = 1
			}
			sb.WriteString(fmt.Sprintf("watchdog_anomaly_detected{metric=\"%s\"} %d\n", s.MetricName, flag))
		}
	}

	// Intelligence Metrics
	if e.intelSummary != nil {
		sb.WriteString("# HELP watchdog_intelligence_fleet_health_score Aggregated fleet health score (0-100)\n")
		sb.WriteString("# TYPE watchdog_intelligence_fleet_health_score gauge\n")
		sb.WriteString(fmt.Sprintf("watchdog_intelligence_fleet_health_score %.2f\n", e.intelSummary.AverageScore))

		sb.WriteString("# HELP watchdog_intelligence_active_incidents_total Current active incidents across fleet\n")
		sb.WriteString("# TYPE watchdog_intelligence_active_incidents_total gauge\n")
		incCritCount := 0
		incWarnCount := 0
		incInfoCount := 0
		for _, inc := range e.intelSummary.ActiveIncidents {
			switch inc.Severity {
			case model.SeverityCritical:
				incCritCount++
			case model.SeverityWarning:
				incWarnCount++
			default:
				incInfoCount++
			}
		}
		sb.WriteString(fmt.Sprintf("watchdog_intelligence_active_incidents_total{severity=\"critical\"} %d\n", incCritCount))
		sb.WriteString(fmt.Sprintf("watchdog_intelligence_active_incidents_total{severity=\"warning\"} %d\n", incWarnCount))
		sb.WriteString(fmt.Sprintf("watchdog_intelligence_active_incidents_total{severity=\"info\"} %d\n", incInfoCount))

		sb.WriteString("# HELP watchdog_intelligence_findings_total Current intelligence findings count\n")
		sb.WriteString("# TYPE watchdog_intelligence_findings_total gauge\n")
		findingsCount := make(map[string]map[string]int) // category -> severity -> count
		for _, f := range e.intelSummary.FleetFindings {
			cat := string(f.Category)
			sev := string(f.Severity)
			if findingsCount[cat] == nil {
				findingsCount[cat] = make(map[string]int)
			}
			findingsCount[cat][sev]++
		}
		for cat, sevs := range findingsCount {
			for sev, count := range sevs {
				sb.WriteString(fmt.Sprintf("watchdog_intelligence_findings_total{category=\"%s\",severity=\"%s\"} %d\n", cat, sev, count))
			}
		}

		if e.evalDuration > 0 {
			sb.WriteString("# HELP watchdog_intelligence_eval_duration_seconds Duration of the last intelligence evaluation\n")
			sb.WriteString("# TYPE watchdog_intelligence_eval_duration_seconds gauge\n")
			sb.WriteString(fmt.Sprintf("watchdog_intelligence_eval_duration_seconds %.6f\n", e.evalDuration.Seconds()))
		}

		for _, n := range e.intelSummary.LowestScoringNodes {
			sb.WriteString(fmt.Sprintf("watchdog_intelligence_node_health_score{node_id=\"%s\",hostname=\"%s\"} %.2f\n",
				n.NodeID, n.Hostname, n.HealthScore.Score))
		}
	}

	return sb.String()
}
