# Watchdog API & Prometheus Exporter Reference

Watchdog provides built-in HTTP endpoints for metric scraping, remote diagnostics, and health probes.

---

## 1. Prometheus Exporter Endpoint (`/metrics`)

When Prometheus export is enabled (`watchdog serve --prometheus` or `prometheus.enabled: true` in config), Watchdog serves standard OpenMetrics / Prometheus exposition format.

### Available Prometheus Metrics

```prometheus
# HELP watchdog_cpu_usage_percent Overall CPU utilization in percent
# TYPE watchdog_cpu_usage_percent gauge
watchdog_cpu_usage_percent 24.50

# HELP watchdog_cpu_cores_logical Total number of logical CPU cores
# TYPE watchdog_cpu_cores_logical gauge
watchdog_cpu_cores_logical 8

# HELP watchdog_memory_used_bytes Total used physical memory in bytes
# TYPE watchdog_memory_used_bytes gauge
watchdog_memory_used_bytes 8589934592

# HELP watchdog_memory_total_bytes Total physical memory in bytes
# TYPE watchdog_memory_total_bytes gauge
watchdog_memory_total_bytes 17179869184

# HELP watchdog_memory_used_percent Memory utilization percentage
# TYPE watchdog_memory_used_percent gauge
watchdog_memory_used_percent 50.00

# HELP watchdog_disk_used_percent Disk space utilization percentage per mount
# TYPE watchdog_disk_used_percent gauge
watchdog_disk_used_percent{mount="/",fstype="ext4"} 62.40

# HELP watchdog_net_bytes_rx_total Total received network bytes
# TYPE watchdog_net_bytes_rx_total counter
watchdog_net_bytes_rx_total{interface="eth0"} 1245892019

# HELP watchdog_net_bytes_tx_total Total transmitted network bytes
# TYPE watchdog_net_bytes_tx_total counter
watchdog_net_bytes_tx_total{interface="eth0"} 849201948

# HELP watchdog_processes_total Total number of active processes
# TYPE watchdog_processes_total gauge
watchdog_processes_total 184

# HELP watchdog_processes_zombies Total number of zombie processes
# TYPE watchdog_processes_zombies gauge
watchdog_processes_zombies 0
```

---

## 2. Remote Agent API

When running as an agent daemon (`watchdog agent` or `agent.enabled: true`), Watchdog exposes authenticated JSON endpoints.

### Authentication
Remote requests must supply the bearer token in the `Authorization` header:
```http
Authorization: Bearer <CONFIGURED_AGENT_TOKEN>
```

### Endpoints

#### `GET /api/v1/snapshot`
Returns real-time system metrics snapshot formatted as JSON matching `model.SystemSnapshot`.

#### `GET /api/v1/diagnose`
Executes active diagnostic rule checks and returns `model.DiagnosticReport`.

#### `GET /api/v1/health`
Health check endpoint returning HTTP 200 `{"status": "ok", "uptime_seconds": 1234}` without requiring authentication.
