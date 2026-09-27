# Watchdog REST API v1 Specification

Watchdog v1.0.0 provides a production-grade, versioned REST API (`/api/v1/...`) for local observability, centralized fleet management, telemetry ingestion, and cluster health monitoring.

---

## 1. Security & Authentication Architecture

### 1.1 Bearer Token Authentication
All administrative and telemetry ingestion endpoints require authentication via HTTP Bearer tokens. Tokens are transmitted in the `Authorization` header:

```http
Authorization: Bearer <WATCHDOG_API_TOKEN>
```

Token verification is performed using constant-time comparison (`crypto/subtle.ConstantTimeCompare`) to eliminate timing-attack vulnerabilities.

### 1.2 Scopes and Permissions
Tokens can be configured with granular role-based scopes:
- `telemetry:write`: Ingest telemetry snapshots and heartbeat signals (`POST /api/v1/telemetry`, `POST /api/v1/heartbeat`, `POST /api/v1/fleet/register`).
- `fleet:read`: Query node identity, list fleet nodes, inspect node details, and fetch summaries (`GET /api/v1/node`, `GET /api/v1/fleet`, `GET /api/v1/fleet/{node_id}`, `GET /api/v1/fleet/summary`, `GET /api/v1/snapshot`).
- `fleet:write` / `admin`: Administrative operations including node deregistration (`DELETE /api/v1/fleet/{node_id}`).

### 1.3 Rate Limiting & Protection
- Ingestion endpoints (`/api/v1/telemetry`, `/api/v1/heartbeat`) are protected by a per-node token-bucket rate limiter.
- Configurable ingestion rate (`fleet.rate_limit_rate`, default: 10 req/sec) and burst capacity (`fleet.rate_limit_burst`, default: 20 req/sec).
- Excessive requests receive HTTP `429 Too Many Requests` with a standard error envelope.

### 1.4 Request Correlation
All requests support distributed tracing and correlation via the `X-Request-ID` header. If omitted by the caller, the server generates a cryptographically secure UUID and returns it in the response header.

---

## 2. Standardized Error Response Envelope

All API errors return a uniform JSON schema with appropriate HTTP status codes:

```json
{
  "error": {
    "code": "INVALID_TOKEN",
    "message": "Bearer authentication token is missing or invalid",
    "request_id": "c6a2b8e4-18c7-43cf-8ef3-d6c62c2f42a1",
    "timestamp": "2026-09-27T10:15:30Z"
  }
}
```

### Standard Error Codes:
| HTTP Status | Error Code | Description |
| :--- | :--- | :--- |
| `400 Bad Request` | `BAD_REQUEST` / `VALIDATION_FAILED` | Malformed JSON body or invalid parameter values |
| `401 Unauthorized` | `UNAUTHORIZED` / `INVALID_TOKEN` | Missing or invalid Bearer token |
| `403 Forbidden` | `FORBIDDEN` / `INSUFFICIENT_SCOPE` | Token lacks required scope for endpoint |
| `404 Not Found` | `NOT_FOUND` | Requested node or resource does not exist |
| `405 Method Not Allowed` | `METHOD_NOT_ALLOWED` | HTTP method not supported for endpoint |
| `413 Payload Too Large` | `PAYLOAD_TOO_LARGE` | Request payload exceeds 1MB threshold |
| `429 Too Many Requests` | `RATE_LIMITED` | Ingestion rate limit exceeded for node |
| `500 Internal Error` | `INTERNAL_SERVER_ERROR` | Unexpected server processing failure |
| `503 Service Unavailable` | `SERVICE_UNAVAILABLE` | Storage engine unavailable or probe not ready |

---

## 3. API Endpoints

### 3.1 Node Identity & Capabilities
#### `GET /api/v1/node`
Retrieves local node identity, hardware specs, network interfaces, and operational tags.

- **Headers**:
  - `Authorization: Bearer <token>`
- **Response**: `200 OK`
```json
{
  "node_id": "c56a4180-65aa-42ec-a945-5fd21dec0538",
  "hostname": "prod-worker-01.internal",
  "os": "linux",
  "platform": "ubuntu",
  "platform_version": "22.04",
  "kernel_version": "5.15.0-101-generic",
  "arch": "amd64",
  "cpu_cores": 16,
  "total_memory": 34359738368,
  "ip_addresses": ["10.0.1.15", "192.168.1.100"],
  "mac_addresses": ["00:1A:2B:3C:4D:5E"],
  "version": "1.0.0",
  "tags": {
    "env": "production",
    "datacenter": "us-east-1",
    "role": "api-gateway"
  },
  "created_at": "2026-09-27T08:00:00Z"
}
```

---

### 3.2 Telemetry Ingestion
#### `POST /api/v1/telemetry`
Ingests a structured telemetry snapshot from an agent node.

- **Headers**:
  - `Authorization: Bearer <token>`
  - `Content-Type: application/json`
  - `X-Watchdog-Node-ID: <node_id>` *(optional if present in payload)*
  - `X-Watchdog-Agent-Version: 1.0.0`
- **Request Body**:
```json
{
  "node_id": "c56a4180-65aa-42ec-a945-5fd21dec0538",
  "timestamp": "2026-09-27T10:30:00Z",
  "snapshot": {
    "timestamp": "2026-09-27T10:30:00Z",
    "cpu": {
      "overall_usage": 32.5,
      "cores_logical": 16,
      "load_1": 1.25,
      "load_5": 1.10,
      "load_15": 0.95
    },
    "memory": {
      "total_bytes": 34359738368,
      "used_bytes": 17179869184,
      "free_bytes": 17179869184,
      "used_percent": 50.0
    },
    "disk": [
      {
        "mount_point": "/",
        "total_bytes": 536870912000,
        "used_bytes": 268435456000,
        "used_percent": 50.0
      }
    ]
  },
  "diagnostic_status": "HEALTHY",
  "active_alerts_count": 0
}
```
- **Response**: `200 OK`
```json
{
  "accepted": true,
  "points_count": 1,
  "buffered": false,
  "timestamp": "2026-09-27T10:30:01Z"
}
```

---

### 3.3 Heartbeat Liveness
#### `POST /api/v1/heartbeat`
Submits periodic liveness signals to keep node state `HEALTHY`.

- **Headers**:
  - `Authorization: Bearer <token>`
  - `Content-Type: application/json`
- **Request Body**:
```json
{
  "node_id": "c56a4180-65aa-42ec-a945-5fd21dec0538",
  "timestamp": "2026-09-27T10:30:15Z",
  "status": "healthy",
  "cpu_usage_percent": 28.4,
  "memory_usage_percent": 48.1,
  "disk_usage_percent": 50.0,
  "load1": 1.12,
  "active_alerts_count": 0,
  "diagnostic_status": "HEALTHY"
}
```
- **Response**: `200 OK`
```json
{
  "acknowledged": true,
  "node_status": "healthy",
  "next_heartbeat_interval": 15,
  "timestamp": "2026-09-27T10:30:15Z"
}
```

---

### 3.4 Fleet Node Registration
#### `POST /api/v1/fleet/register`
Enrolls a new node into centralized fleet tracking.

- **Headers**:
  - `Authorization: Bearer <token>`
  - `Content-Type: application/json`
- **Request Body**:
```json
{
  "identity": {
    "node_id": "c56a4180-65aa-42ec-a945-5fd21dec0538",
    "hostname": "prod-worker-01.internal",
    "os": "linux",
    "platform": "ubuntu",
    "platform_version": "22.04",
    "arch": "amd64",
    "cpu_cores": 16,
    "total_memory": 34359738368,
    "version": "1.0.0",
    "tags": {
      "env": "production",
      "datacenter": "us-east-1"
    }
  },
  "metadata": {
    "instance_type": "c6i.4xlarge",
    "vpc_id": "vpc-0a1b2c3d"
  }
}
```
- **Response**: `200 OK`
```json
{
  "registered": true,
  "node_id": "c56a4180-65aa-42ec-a945-5fd21dec0538",
  "registered_at": "2026-09-27T08:00:00Z",
  "heartbeat_interval_seconds": 15,
  "telemetry_interval_seconds": 30,
  "message": "Node successfully registered with fleet controller"
}
```

---

### 3.5 Fleet List & Filtering
#### `GET /api/v1/fleet`
Lists registered fleet nodes with status, resource usage, and pagination.

- **Query Parameters**:
  - `status` *(optional)*: `healthy`, `warning`, `critical`, `stale`, `offline`, `unknown`
  - `search` *(optional)*: Match against node ID, hostname, or IP address
  - `limit` *(optional, default: 50)*: Maximum nodes to return (max: 500)
  - `offset` *(optional, default: 0)*: Pagination offset
  - `sort_by` *(optional)*: `hostname`, `last_heartbeat`, `cpu`, `memory`, `status`
  - `sort_direction` *(optional)*: `asc`, `desc`
  - `since` *(optional)*: Active since RFC3339 timestamp or relative duration (e.g. `10m`, `2h`)
- **Response**: `200 OK`
```json
{
  "total": 1,
  "count": 1,
  "nodes": [
    {
      "identity": {
        "node_id": "c56a4180-65aa-42ec-a945-5fd21dec0538",
        "hostname": "prod-worker-01.internal",
        "os": "linux",
        "platform": "ubuntu",
        "tags": { "env": "production" }
      },
      "status": "healthy",
      "registered_at": "2026-09-27T08:00:00Z",
      "last_heartbeat": "2026-09-27T10:30:15Z",
      "summary": {
        "cpu_usage_percent": 28.4,
        "memory_usage_percent": 48.1,
        "disk_usage_percent": 50.0,
        "load1": 1.12,
        "active_alerts_count": 0,
        "diagnostic_status": "HEALTHY"
      }
    }
  ]
}
```

---

### 3.6 Fleet Node Detail & Telemetry
#### `GET /api/v1/fleet/{node_id}`
Inspects comprehensive point-in-time state, hardware specs, diagnostics, and recent telemetry history for a specific node.

- **Response**: `200 OK`
```json
{
  "node": {
    "identity": { ... },
    "status": "healthy",
    "registered_at": "2026-09-27T08:00:00Z",
    "last_heartbeat": "2026-09-27T10:30:15Z",
    "summary": { ... },
    "metadata": { ... }
  },
  "recent_telemetry": [ ... ],
  "active_alerts": [ ... ]
}
```

---

### 3.7 Fleet Node Deregistration
#### `DELETE /api/v1/fleet/{node_id}`
Removes a node from the fleet registry.

- **Headers**:
  - `Authorization: Bearer <token>`
- **Response**: `200 OK`
```json
{
  "deleted": true,
  "node_id": "c56a4180-65aa-42ec-a945-5fd21dec0538"
}
```

---

### 3.8 Fleet Summary & Health
#### `GET /api/v1/fleet/summary`
Retrieves aggregated cluster-wide health counts and average resource utilization.

- **Response**: `200 OK`
```json
{
  "total_nodes": 120,
  "healthy_nodes": 115,
  "warning_nodes": 3,
  "critical_nodes": 2,
  "stale_nodes": 0,
  "offline_nodes": 0,
  "unknown_nodes": 0,
  "avg_cpu_percent": 34.2,
  "avg_memory_percent": 58.7,
  "total_alerts": 5,
  "last_updated": "2026-09-27T10:30:30Z"
}
```

---

### 3.9 Redacted Point-in-Time Snapshot
#### `GET /api/v1/snapshot`
Returns the latest local telemetry snapshot with credential masking applied.

- **Response**: `200 OK`
```json
{
  "timestamp": "2026-09-27T10:30:00Z",
  "cpu": { ... },
  "memory": { ... },
  "disk": [ ... ],
  "network": [ ... ],
  "processes": [ ... ]
}
```

---

### 3.10 Liveness & Readiness Probes
#### `GET /health` / `GET /healthz`
Liveness probe returning `200 OK` when the process is operational.

#### `GET /ready` / `GET /readyz`
Readiness probe verifying collector availability and storage connectivity. Returns `200 OK` when ready, `503 Service Unavailable` otherwise.
