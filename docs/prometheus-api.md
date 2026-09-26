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
# HELP watchdog_build_info Watchdog build information
# TYPE watchdog_build_info gauge
watchdog_build_info{commit="unknown",go_version="go1.22.0",platform="linux/amd64",version="1.0.0"} 1

# HELP watchdog_up Watchdog daemon operational status (1 = operational, 0 = failing)
# TYPE watchdog_up gauge
watchdog_up 1

# HELP watchdog_health_status System diagnostic health status (1 = active status)
# TYPE watchdog_health_status gauge
watchdog_health_status{status="ok"} 1
watchdog_health_status{status="warning"} 0
watchdog_health_status{status="critical"} 0

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

### Prometheus Operator `ServiceMonitor` Integration
For Kubernetes clusters running the Prometheus Operator, Watchdog provides a native `ServiceMonitor` manifest (`deploy/k8s/servicemonitor.yaml`):

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: watchdog-agent
  namespace: watchdog-system
  labels:
    app.kubernetes.io/name: watchdog
    release: prometheus-stack
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: watchdog
  endpoints:
    - port: prometheus
      path: /metrics
      interval: 15s
      scrapeTimeout: 10s
```

---

## 🔒 REST API Endpoints & Health Probes

All administrative and telemetry REST API endpoints require Bearer token authentication when configured. Operational health, readiness, and metrics endpoints are unauthenticated:

| Endpoint | Method | Auth Required | Description |
| :--- | :---: | :---: | :--- |
| `/health`, `/healthz`, `/api/v1/health` | `GET` | No | Liveness probe indicating HTTP server responsiveness |
| `/ready`, `/readyz`, `/api/v1/ready` | `GET` | No | Readiness probe indicating collector and storage availability |
| `/metrics` | `GET` | No | Prometheus / OpenMetrics metric exposition |
| `/api/v1/snapshot` | `GET` | **Bearer Token** | Complete raw host `SystemSnapshot` JSON payload |
| `/api/v1/diagnostics` | `GET` | **Bearer Token** | Latest 10-rule diagnostic report |
| `/api/v1/alerts` | `GET` | **Bearer Token** | Array of currently firing `AlertEvent` objects |
| `/api/v1/anomalies` | `GET` | **Bearer Token** | Latest statistical anomaly score stream |
| `/api/v1/audit/events` | `GET` | **Bearer Token** | Query and filter security audit event trail |

### Querying REST Endpoints
```bash
curl -s -H "Authorization: Bearer s3cret-cluster-token" \
  http://127.0.0.1:9100/api/v1/snapshot | jq .
```

---

## 🛡️ Security & Middleware Pipeline

Watchdog's HTTP server incorporates a hardened middleware pipeline:

1. **Request ID Propagation (`RequestIDMiddleware`)**: Validates or injects `X-Request-ID` headers (UUID v4) on every request and response, binding request IDs into log lines and audit events.
2. **Panic Recovery (`PanicRecoveryMiddleware`)**: Intercepts unhandled HTTP handler panics, safely logs the stack trace to structured stderr without terminating the process, records a `CRITICAL` audit event, and returns HTTP 500 JSON (`{"error": "Internal server error"}`).
3. **Payload Limiting (`MaxBodySizeMiddleware`)**: Enforces a strict 1MB (1,048,576 bytes) limit on incoming request bodies via `http.MaxBytesReader` to mitigate denial-of-service memory exhaustion, returning HTTP 413 Payload Too Large on overflow.
4. **Structured Request Logging (`RequestLoggerMiddleware`)**: Records request method, path, remote IP, HTTP status, latency, and request ID.
5. **Constant-Time Token Comparison**: Token verification uses `crypto/subtle.ConstantTimeCompare` to eliminate timing attack vectors.
6. **Localhost Default**: Defaults to binding on `127.0.0.1` unless token and TLS are configured for non-loopback exposure.
3. **Native TLS Support**: Built-in TLS encryption for secure transport across untrusted networks.
4. **Diagnostic Profiling (pprof)**: Standard `/debug/pprof/*` endpoints available for runtime memory and goroutine profiling.

---

## 🌐 Connecting Remote Dashboards

The Watchdog TUI can connect directly to a remote Watchdog agent server over TLS:

```bash
watchdog dash --remote 192.168.1.50:8443 --token s3cret-cluster-token
```
