# Watchdog Fleet Management & Secure Telemetry Architecture

Watchdog v1.0.0 provides a centralized, secure fleet management subsystem designed for large-scale distributed infrastructure across bare-metal servers, cloud VMs, Kubernetes clusters, and edge appliances.

---

## 1. Architectural Overview

The fleet subsystem operates on a hub-and-spoke model:
- **Central Fleet Server (`watchdog server`)**: Acts as the central registry, ingesting telemetry snapshots, monitoring heartbeats, enforcing rate limits, evaluating node health, and exposing query APIs.
- **Node Agents (`watchdog agent` or headless runners)**: Lightweight background daemons executing on monitored nodes, capturing local system metrics, buffering data during network disruptions, and streaming updates to the controller.

```text
    ┌──────────────────────┐         Heartbeat / Telemetry         ┌──────────────────────┐
    │     Watchdog Node    │ ────────────────────────────────────► │ Central Fleet Server │
    │   (Local Collector)  │                                       │ (Registry & Storage) │
    └──────────────────────┘                                       └──────────────────────┘
               ▲                                                               │
               │ Local Discovery                                               │ Query API
               │                                                               ▼
    ┌──────────────────────┐                                       ┌──────────────────────┐
    │    watchdog node     │                                       │    watchdog fleet    │
    │  (Hardware Identity) │                                       │   (CLI Management)   │
    └──────────────────────┘                                       └──────────────────────┘
```

---

## 2. Core Operational Guarantees & Constraints

1. **Strict Read-Only Observability**: Watchdog agents never execute arbitrary remote shell commands, kill arbitrary processes, alter system configurations, or modify network/firewall/DNS rules.
2. **Persistent Cryptographic Node Identity**: Each node generates a persistent UUID stored in `~/.watchdog/node_id` (or `/etc/watchdog/node_id`), maintaining identity continuity across reboots and IP reassignments.
3. **Resilient Offline-First Telemetry Buffering**: If the central fleet server becomes temporarily unreachable, local agents buffer telemetry up to configurable item and memory thresholds (`fleet.buffer_capacity`, `fleet.buffer_max_bytes`), flushing automatically upon reconnection with backoff and jitter.
4. **Token-Bucket Ingestion Rate Limiting**: The server enforces per-node sliding-window rate limiting (`fleet.rate_limit_rate`, `fleet.rate_limit_burst`) to prevent telemetry storms from overloading the centralized storage engine.
5. **Zero-CGO & Minimal Footprint**: Operates without CGO dependencies or external database servers (powered by pure-Go SQLite with WAL mode).

---

## 3. Node Lifecycle & Health States

Node health is deterministically computed based on heartbeat freshness, diagnostic issues, and alert severity:

| Node State | Criteria |
| :--- | :--- |
| **`HEALTHY`** | Heartbeat received within `StaleThreshold` (default: 2m), diagnostic status is HEALTHY, and no active CRITICAL alerts. |
| **`WARNING`** | Heartbeat fresh, but active WARNING alerts or WARNING diagnostic findings exist. |
| **`CRITICAL`** | Heartbeat fresh, but active CRITICAL alerts or CRITICAL diagnostic failures exist. |
| **`STALE`** | No heartbeat received within `StaleThreshold` (default: 2m), but less than `OfflineThreshold` (default: 10m). |
| **`OFFLINE`** | No heartbeat received within `OfflineThreshold` (default: 10m). |
| **`UNKNOWN`** | Initial un-evaluated or un-registered state. |

---

## 4. Configuration Reference

### 4.1 Server Configuration (`/etc/watchdog/server.yaml`)
```yaml
server:
  port: 8443
  bind_address: "0.0.0.0"
  token_file: "/etc/watchdog/server_token"
  tls_cert_file: "/etc/watchdog/certs/server.crt"
  tls_key_file: "/etc/watchdog/certs/server.key"

storage:
  enabled: true
  path: "/var/lib/watchdog/fleet.db"
  wal_mode: true
  retention_days: 30

fleet:
  enabled: true
  stale_threshold: 2m
  offline_threshold: 10m
  rate_limit_rate: 10.0
  rate_limit_burst: 20
```

### 4.2 Agent Configuration (`/etc/watchdog/agent.yaml`)
```yaml
refresh_interval: 5s

fleet:
  enabled: true
  server_url: "https://fleet.internal:8443"
  token_file: "/etc/watchdog/agent_token"
  node_id_file: "/etc/watchdog/node_id"
  heartbeat_interval: 15s
  telemetry_interval: 30s
  buffer_capacity: 1000
  buffer_max_bytes: 10485760 # 10 MB
  tags:
    env: "production"
    datacenter: "us-east-1"
    role: "worker"
```

---

## 5. CLI Management Guide

### 5.1 Local Node Inspection (`watchdog node`)
Inspect local machine identity, hardware specs, network interfaces, and operational tags:

```bash
# Human-readable output
watchdog node

# Output persistent UUID only
watchdog node --short

# Output structured JSON
watchdog node --json
```

### 5.2 Fleet Status Overview (`watchdog fleet status`)
Display aggregated cluster health counts, active alerts, and resource utilization:

```bash
watchdog fleet status --server https://fleet.internal:8443 --token <token>
```

### 5.3 Listing and Filtering Nodes (`watchdog fleet list`)
```bash
# List all nodes
watchdog fleet list

# Filter by health status
watchdog fleet list --status warning

# Search by hostname or IP
watchdog fleet list --search worker-01

# Filter nodes active within the last 15 minutes
watchdog fleet list --since 15m --sort-by cpu --sort-direction desc
```

### 5.4 Inspecting a Single Node (`watchdog fleet get`)
Inspect hardware specs, active alerts, and recent telemetry for a node:

```bash
watchdog fleet get c56a4180-65aa-42ec-a945-5fd21dec0538
```

### 5.5 Registering a Node (`watchdog fleet register`)
Manually enroll a node or provision metadata:

```bash
watchdog fleet register --tags env=prod,team=data --metadata cloud=aws,instance=t4g.xlarge
```

### 5.6 Deregistering a Node (`watchdog fleet deregister`)
Decommission and remove a node from the fleet registry:

```bash
watchdog fleet deregister node-decom-01 --force
```
