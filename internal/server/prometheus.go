package server

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/watchdog-cli/watchdog/pkg/model"
)

// PrometheusExporter generates Prometheus-compatible metrics from snapshots and reports.
type PrometheusExporter struct {
	mu           sync.RWMutex
	lastSnapshot *model.SystemSnapshot
	lastDiag     *model.DiagnosticReport
	activeAlerts []model.AlertEvent
	anomalies    *model.AnomalyReport
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

	if e.lastSnapshot == nil {
		sb.WriteString("# No snapshot data collected yet\n")
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

		if len(snap.CPU.PerCoreUsage) > 0 {
			sb.WriteString("# HELP watchdog_cpu_core_usage_percent Per-core CPU utilization percentage\n")
			sb.WriteString("# TYPE watchdog_cpu_core_usage_percent gauge\n")
			for i, u := range snap.CPU.PerCoreUsage {
				sb.WriteString(fmt.Sprintf("watchdog_cpu_core_usage_percent{core=\"%d\"} %.2f\n", i, u))
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
				escapedMount := strings.ReplaceAll(p.MountPoint, "\\", "\\\\")
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
		sb.WriteString(fmt.Sprintf("watchdog_docker_containers_total %d\n", snap.Docker.TotalCount))

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

	return sb.String()
}
