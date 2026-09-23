# 🐺 Watchdog

> **Enterprise-Grade Cross-Platform System Health Monitoring, Diagnostics & Interactive TUI**

[![Go Version](https://img.shields.io/badge/Go-1.27.1+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-blue)](https://github.com/watchdog-cli/watchdog)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

**Watchdog** is an all-in-one system observability, real-time monitoring, automated diagnostics, and interactive terminal dashboard application written in Go. Designed with low overhead and zero external runtime dependencies, Watchdog operates smoothly on physical workstations, bare-metal servers, containerized environments (Docker / Podman), and Kubernetes clusters.

---

## 🌟 Key Features

- 🖥️ **Interactive Bubble Tea TUI**: 6-tab terminal interface featuring sparkline graphs, per-core CPU usage, memory breakdown, process tree with sorting/filtering, live disk I/O, network telemetry, container status, and diagnostics.
- 🩺 **Automated Diagnostics Engine**: Evaluates 10+ operational health rules (CPU saturation, paging, swap exhaustion, disk/inode limits, DNS latency, gateway reachability, zombie processes, container crash-loops) with automated remediation advice.
- ⚡ **Multi-Subsystem Metric Collectors**: Concurrent sampling of CPU, Memory, Disk, Network, Process trees, System Services (systemd/Windows SCM/launchd), Open Ports, Docker containers, and Kubernetes pods.
- 🚨 **Temporal Alerting Engine**: Rule-based threshold alerts with hysteresis, cooldown suppression, firing duration tracking, and SQLite history persistence.
- 📈 **Online Anomaly Detection**: Statistical anomaly scoring utilizing Exponentially Weighted Moving Averages (EWMA) and rolling standard deviation Z-scores.
- 💾 **Embedded Time-Series Storage**: SQLite storage engine configured with Write-Ahead Logging (WAL) and automated background pruning based on retention policies.
- 📊 **Multi-Format Report Generation**: Standalone self-contained dark-themed HTML reports with embedded SVG sparklines, structured JSON, CSV time-series exports, and ANSI terminal summaries.
- 🌐 **Prometheus Exporter & REST API**: Native `/metrics` endpoint compatible with Prometheus/Grafana and authenticated REST endpoints (`/api/v1/snapshot`, `/api/v1/diagnose`, `/api/v1/alerts`).
- 🔒 **Secure by Default**: Explicit token authentication, input sanitization, TLS support, and non-destructive read-only sampling.

---

## 🚀 Quick Start

### Installation

```bash
# Build from source
go build -o watchdog main.go

# Verify installation
./watchdog version
```

### Common Commands

```bash
# Launch the interactive real-time Terminal Dashboard
watchdog dash

# Run comprehensive system diagnostics
watchdog diagnose

# Run diagnostics with actionable remediation recommendations
watchdog diagnose --fix

# Generate a standalone self-contained HTML report with inline SVG graphs
watchdog report --format html --output report.html

# Launch Prometheus metrics exporter on port 9100
watchdog server --port 9100

# Inspect active firing threshold alerts
watchdog alert list

# Export historical metric time-series to JSON or CSV
watchdog export --format json --output metrics.json
```

---

## ⌨️ Interactive TUI Navigation & Shortcuts

When running `watchdog dash`, control the interface using the following keyboard shortcuts:

| Key Binding | Action |
| :--- | :--- |
| `1` - `6` | Quick-switch directly to Tab 1–6 |
| `Tab` / `Shift+Tab` | Cycle forward / backward through tabs |
| `h` / `l` or `←` / `→` | Switch active tab |
| `j` / `k` or `↑` / `↓` | Scroll lists (process table, services, containers, alerts) |
| `g` / `G` | Jump to top / bottom of lists |
| `/` | Enter filter / search mode in Process and Container views |
| `c` / `m` / `p` / `n` | Sort processes by CPU %, Memory %, PID, or Name |
| `k` | Send signal / terminate selected process (interactive prompt) |
| `r` | Manually trigger instant metrics refresh |
| `p` or `Space` | Pause / resume real-time metrics polling |
| `?` | Toggle Help & Keybinding overlay modal |
| `q` / `Ctrl+C` | Gracefully exit Watchdog |

### TUI Tabs

1. **Dashboard (`1`)**: Host overview, CPU gauges (aggregate & per-core), Memory/Swap usage, Disk partition meters, and Network throughput sparklines.
2. **Processes (`2`)**: Real-time process list with PID, user, CPU%, Mem%, state, thread count, I/O rates, search filtering, and process termination.
3. **Storage & Net (`3`)**: Filesystem mount breakdown, total/used/free space, inode capacity, network adapter addresses, packet counters, and active open ports.
4. **Services (`4`)**: System service statuses (systemd on Linux, Windows Services on Windows, launchd on macOS) and listening TCP/UDP sockets.
5. **Containers (`5`)**: Docker / Podman containers and Kubernetes pods with CPU/Memory limits, restart counts, image tags, and state badges.
6. **Diagnostics & Alerts (`6`)**: Live health checks, anomaly detection status, active firing alerts, and remediation guidance.

---

## 🛠️ CLI Command Reference

### `watchdog dash`
Starts the interactive full-screen Bubble Tea terminal user interface.
```bash
watchdog dash [flags]
  -i, --interval duration   Metrics sampling interval (e.g. 500ms, 1s, 2s) (default 1s)
      --per-core            Display individual per-core CPU bars (default true)
      --process-limit int   Maximum processes displayed in process table (default 50)
      --sort string         Default process sort column: cpu, mem, pid, name (default "cpu")
```

### `watchdog diagnose`
Executes automated diagnostic checks evaluating system stability and saturation limits.
```bash
watchdog diagnose [flags]
  -C, --category string   Filter checks by category (CPU, Memory, Disk, Network, Process, Docker)
  -F, --fix               Print detailed actionable remediation steps for warnings/critical issues
      --json              Output raw diagnostic report in JSON format
      --plain             Disable ANSI color codes and styles
  -t, --timeout duration  Diagnostic execution timeout (default 10s)
```

### `watchdog report`
Generates standalone multi-format system health reports.
```bash
watchdog report [flags]
  -f, --format string     Output format: html, json, csv, terminal (default "html")
  -o, --output string     Destination file path (or '-' for stdout)
  -H, --history duration  Historical time-window for sparklines (default 1h)
  -t, --title string      Custom report header title
      --charts            Include inline SVG trend charts in HTML output (default true)
      --raw-json          Embed full raw snapshot payload in HTML report (default false)
```

### `watchdog server`
Launches the background HTTP server with Prometheus metrics export and REST APIs.
```bash
watchdog server [flags]
  -p, --port int          HTTP listener port (default 9100)
  -b, --bind string       Listen IP interface (default "0.0.0.0")
      --metrics-path str  Prometheus metrics path (default "/metrics")
      --api               Enable REST API endpoints (default true)
      --auth-token str    Bearer token required for REST API calls
```

### `watchdog alert`
Manages threshold rules and queries firing or historical alert events.
```bash
watchdog alert list       # List currently firing alerts
watchdog alert history    # Query historical alerts from SQLite database (--limit 50)
watchdog alert test       # Dispatch a synthetic test alert to verify notification channels
```

### `watchdog config`
Manages the Watchdog YAML configuration file.
```bash
watchdog config init [path]   # Generate default config file
watchdog config validate      # Validate existing configuration syntax
watchdog config show          # Display active merged configuration (--json supported)
watchdog config path          # Print active configuration file path
```

### `watchdog export`
Exports historical snapshots and metrics from the local SQLite storage engine.
```bash
watchdog export [flags]
  -f, --format string     Export format: json, csv, sqlite (default "json")
  -o, --output string     Output file destination
  -s, --since duration    Time-series export lookback window (e.g. 1h, 6h, 24h)
```

---

## ⚙️ Configuration (`config.yaml`)

Watchdog can be configured using a YAML configuration file located at `~/.watchdog/config.yaml` or specified via the `--config` flag.

```yaml
refresh_interval: 1s

storage:
  enabled: true
  db_path: "~/.watchdog/watchdog.db"
  retention_days: 7
  collection_interval: 10s

alerts:
  cpu:
    enabled: true
    threshold: 90.0      # CPU utilization percentage
    duration: 30s        # Sustained duration before firing
    cooldown: 5m         # Suppression window between alerts
  memory:
    enabled: true
    threshold: 85.0      # Memory utilization percentage
    duration: 30s
    cooldown: 5m
  disk:
    enabled: true
    threshold: 90.0      # Partition utilization percentage
    duration: 1m
    cooldown: 15m
  process:
    enabled: true
    cpu_threshold: 80.0
    memory_threshold: 70.0
    cooldown: 5m
  network:
    enabled: true
    threshold: 100.0     # MB/s throughput threshold
    duration: 1m
    cooldown: 10m

anomaly:
  enabled: true
  zscore_threshold: 2.5
  window_size: 60
  alpha: 0.2

collectors:
  cpu: true
  memory: true
  disk: true
  network: true
  process: true
  service: true
  port: true
  docker: true
  kubernetes: false

dashboard:
  theme: "default"
  show_per_core_cpu: true
  process_sort_by: "cpu"
  process_limit: 50
  pause_on_start: false

prometheus:
  enabled: false
  port: 9100
  path: "/metrics"

agent:
  enabled: false
  port: 8443
  bind_address: "127.0.0.1"
  token: ""
  tls_cert: ""
  tls_key: ""
```

---

## 🏛️ Architecture Overview

```
                          ┌───────────────────────────┐
                          │   CLI Entrypoint (Cobra)  │
                          │        watchdog <cmd>     │
                          └─────────────┬─────────────┘
                                        │
        ┌───────────────────────────────┼───────────────────────────────┐
        │                               │                               │
        ▼                               ▼                               ▼
┌───────────────┐               ┌───────────────┐               ┌───────────────┐
│ Interactive   │               │ Diagnostics & │               │ HTTP Server & │
│ Terminal TUI  │               │ Alert Engine  │               │ Prometheus    │
│ (Bubble Tea)  │               │ (Concurrent)  │               │ Exporter      │
└───────┬───────┘               └───────┬───────┘               └───────┬───────┘
        │                               │                               │
        └───────────────────────┬───────┴───────────────────────────────┘
                                │
                                ▼
                ┌───────────────────────────────┐
                │   Central Metric Collectors   │
                │ ───────────────────────────── │
                │  CPU  •  Memory  •  Disk      │
                │  Net  •  Process •  Service   │
                │  Port •  Docker  •  K8s       │
                └───────────────┬───────────────┘
                                │
                ┌───────────────┴───────────────┐
                ▼                               ▼
        ┌───────────────┐               ┌───────────────┐
        │ Anomaly       │               │ SQLite Time-  │
        │ Detection     │               │ Series Store  │
        │ (EWMA/Z-Score)│               │ (WAL & Prune) │
        └───────────────┘               └───────────────┘
```

---

## 🧪 Testing

Run all unit and integration tests:

```bash
# Run all tests across modules
go test -v ./...

# Run tests with race condition detection
go test -race ./...
```

---

## 📄 License

This project is licensed under the MIT License — see the [LICENSE](LICENSE) file for details.
