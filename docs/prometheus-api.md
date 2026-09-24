# Prometheus Exporter & REST API Server

Watchdog includes a built-in HTTP server providing an **OpenMetrics / Prometheus `/metrics`** scraping endpoint and an authenticated **REST API** for remote telemetry, distributed monitoring, and fleet management.

---

## 🚀 Starting the Server

```bash
# Start Prometheus exporter on port 9100 (localhost only)
watchdog server --port 9100 --host 127.0.0.1

# Start multi-node server on all interfaces with Bearer token authentication and TLS
watchdog server \
  --host 0.0.0.0 \
  --port 9100 \
  --token "s3cret-cluster-token" \
  --tls-cert /etc/ssl/certs/watchdog.crt \
  --tls-key /etc/ssl/private/watchdog.key
```

---

## 📈 Prometheus `/metrics` Exposition

Watchdog exposes host and application metrics in standard OpenMetrics text format at `/metrics`:

```bash
curl -s http://127.0.0.1:9100/metrics
```

### Key Prometheus Metrics
```prometheus
# HELP watchdog_cpu_usage_percent Overall CPU utilization percentage
# TYPE watchdog_cpu_usage_percent gauge
watchdog_cpu_usage_percent 42.50

# HELP watchdog_cpu_load1 1-minute load average
# TYPE watchdog_cpu_load1 gauge
watchdog_cpu_load1 1.24

# HELP watchdog_memory_used_percent Physical memory utilization percentage
# TYPE watchdog_memory_used_percent gauge
watchdog_memory_used_percent 52.60

# HELP watchdog_disk_used_percent Filesystem partition utilization percentage
# TYPE watchdog_disk_used_percent gauge
watchdog_disk_used_percent{mountpoint="/"} 64.20
watchdog_disk_used_percent{mountpoint="/data"} 82.10

# HELP watchdog_network_rx_bytes_total Inbound network bytes counter
# TYPE watchdog_network_rx_bytes_total counter
watchdog_network_rx_bytes_total{interface="eth0"} 104857600

# HELP watchdog_active_alerts_total Total currently firing threshold alerts
# TYPE watchdog_active_alerts_total gauge
watchdog_active_alerts_total 0

# HELP watchdog_diagnostics_healthy Diagnostic health status (1 = pass, 0 = fail)
# TYPE watchdog_diagnostics_healthy gauge
watchdog_diagnostics_healthy 1
```

### Sample `prometheus.yml` Configuration
```yaml
scrape_configs:
  - job_name: "watchdog"
    scrape_interval: 10s
    static_configs:
      - targets: ["prod-db-01:9100", "prod-app-01:9100"]
```

---

## 🔒 Authenticated REST API Endpoints

All REST API endpoints (except `/health`) require Bearer token authentication when configured:

| Endpoint | Method | Auth Required | Description |
| :--- | :---: | :---: | :--- |
| `/health` | `GET` | No | Liveness and readiness probe for load balancers |
| `/api/v1/health` | `GET` | No | Health check alias |
| `/metrics` | `GET` | No | Prometheus metric exposition |
| `/api/v1/snapshot` | `GET` | **Bearer Token** | Complete raw host `SystemSnapshot` JSON payload |
| `/api/v1/diagnostics` | `GET` | **Bearer Token** | Latest 35-rule diagnostic report |
| `/api/v1/alerts` | `GET` | **Bearer Token** | Array of currently firing `AlertEvent` objects |
| `/api/v1/anomalies` | `GET` | **Bearer Token** | Latest statistical anomaly score stream |

### Querying REST Endpoints
```bash
curl -s -H "Authorization: Bearer s3cret-cluster-token" \
  http://127.0.0.1:9100/api/v1/snapshot | jq .
```

---

## 🛡️ Security Architecture

1. **Localhost Default**: Defaults to binding on `127.0.0.1` to prevent accidental public exposure.
2. **Constant-Time Comparison**: Token verification uses `crypto/subtle.ConstantTimeCompare` to mitigate timing attacks against authentication.
3. **Native TLS Support**: Built-in TLS encryption for secure transport across untrusted networks.
4. **Diagnostic Profiling (pprof)**: Standard `/debug/pprof/*` endpoints available for runtime memory and goroutine profiling.

---

## 🌐 Connecting Remote Dashboards

The Watchdog TUI can connect directly to a remote Watchdog agent server over TLS:

```bash
watchdog dash --remote 192.168.1.50:8443 --token s3cret-cluster-token
```
