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

### Security Requirements

| Bind Address | Token Required | TLS Required |
| :--- | :--- | :--- |
| `127.0.0.1` / `localhost` / `::1` | Optional | Optional |
| Any non-loopback (`0.0.0.0`, LAN IP, etc.) | **Mandatory** | **Mandatory** |

The server **refuses to start** if a non-loopback bind address is configured without both a token and TLS certificate/key pair. This prevents accidental exposure of unauthenticated or unencrypted APIs on the network.

Incomplete TLS configurations (certificate without key, or vice versa) are always rejected regardless of bind address.

### Authentication
Remote requests must supply the bearer token via one of two headers:
```http
Authorization: Bearer <CONFIGURED_AGENT_TOKEN>
```
or:
```http
X-Watchdog-Token: <CONFIGURED_AGENT_TOKEN>
```

### Endpoints

#### `GET /health` (Unauthenticated)
Health check / liveness probe endpoint returning HTTP 200 `{"status": "ok", "uptime_seconds": 1234, "version": "1.0.0"}`. Does not require authentication, safe for Kubernetes liveness probes and load balancer health checks.

#### `GET /api/v1/health` (Unauthenticated)
Alias for `/health`.

#### `GET /metrics` (Unauthenticated)
Prometheus text-format metric scrape endpoint. Does not expose sensitive credentials. Suitable for Prometheus ServiceMonitor scraping without authentication.

#### `GET /api/v1/snapshot` (Authenticated)
Returns real-time system metrics snapshot formatted as JSON matching `model.SystemSnapshot`.

#### `GET /api/v1/diagnostics` (Authenticated)
Executes active diagnostic rule checks and returns `model.DiagnosticReport`.

#### `GET /api/v1/alerts` (Authenticated)
Returns active and historical alert events.

#### `GET /api/v1/anomalies` (Authenticated)
Returns statistical anomaly detection Z-scores and scores.

#### `GET /api/v1/audit/events` (Authenticated)
Queries recorded security audit events with flexible filtering and pagination.

**Query Parameters**:
| Parameter | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `since` | string | `""` | Lookback timestamp or duration (e.g. `24h`, `7d`, `2026-09-26T12:00:00Z`) |
| `until` | string | `""` | Upper timestamp or duration cutoff |
| `event_type` | string | `""` | Filter by event type (e.g. `auth.failure`, `server.start`) |
| `severity` | string | `""` | Filter by severity (`info`, `notice`, `warning`, `error`, `critical`) |
| `outcome` | string | `""` | Filter by outcome (`success`, `failure`, `denied`) |
| `source` | string | `""` | Filter by source IP address |
| `actor` | string | `""` | Filter by actor identity |
| `request_id` | string | `""` | Filter by correlation request ID |
| `limit` | int | `100` | Maximum records to return (capped at `audit.max_query_limit`) |
| `offset` | int | `0` | Pagination record offset |

**Response Schema (`application/json`)**:
```json
{
  "total": 42,
  "count": 1,
  "limit": 100,
  "offset": 0,
  "events": [
    {
      "id": "evt-7f8e9d0a1b2c",
      "timestamp": "2026-09-26T14:32:00Z",
      "event_type": "auth.failure",
      "severity": "warning",
      "outcome": "denied",
      "actor": {
        "type": "anonymous_client",
        "identity": "token:sha256:a1b2c3d4"
      },
      "source": {
        "address": "192.168.1.100:54321",
        "endpoint": "/api/v1/snapshot",
        "method": "GET",
        "request_id": "c1f3a2b4-5d6e-4f7a-8b9c-0d1e2f3a4b5c",
        "user_agent": "curl/7.88.1"
      },
      "message": "Authentication failed: invalid credentials"
    }
  ],
  "timestamp": "2026-09-26T14:32:05Z"
}
```

---

## 3. Request Correlation (`X-Request-ID`)

All incoming HTTP requests are assigned a unique Request ID. If the client provides a valid `X-Request-ID` header matching `^[a-zA-Z0-9_-]{1,64}$`, it is preserved; otherwise, a cryptographically random RFC 4122 UUID v4 is generated.

The Request ID is:
- Attached to the request context across internal handlers.
- Echoed back in the HTTP response header `X-Request-ID`.
- Recorded in all associated security audit logs (`source.request_id`) for cross-system distributed tracing and incident investigation.

