# Watchdog Production Readiness & Baseline Audit

**Audit Date**: September 23, 2026  
**Document Version**: 1.0.0-baseline  
**Target Application**: Watchdog System Monitoring & Diagnostics Suite (`github.com/watchdog-cli/watchdog`)

---

## 1. Architecture Overview

Watchdog is structured as a modular, decoupled Go application divided into:
- **CLI Layer (`cmd/`)**: Built with Cobra (`github.com/spf13/cobra`) managing subcommands (`dash`, `diagnose`, `report`, `server`, `agent`, `alert`, `config`, `export`, `version`).
- **Core Domain Models (`pkg/model/`)**: Clean telemetry domain structures covering CPU, Memory, Disk, Network, Processes, Services, Ports, Docker, Kubernetes, Diagnostics, Alerts, and Anomalies.
- **Metric Collectors (`internal/collector/`)**: Coordinated by `collector.Manager` executing concurrent, timeout-bound sampling using `gopsutil`, net dialers, and container clients.
- **Diagnostics Engine (`internal/diagnostics/`)**: Rule evaluation engine assessing 10+ operational health rules and generating automated remediation plans.
- **Alert Engine (`internal/alerts/`)**: Stateful threshold evaluation engine with hysteresis duration tracking and cooldown suppression.
- **Anomaly Detection (`internal/anomaly/`)**: Online baseline estimation using Exponentially Weighted Moving Averages (EWMA) and rolling standard deviation Z-scores.
- **Storage Subsystem (`internal/storage/`)**: Embedded SQLite engine (`modernc.org/sqlite`) running in Write-Ahead Logging (WAL) mode with background metric retention pruning.
- **Reporting Engine (`internal/reporting/`)**: Standalone self-contained HTML (with inline SVG sparklines), JSON, CSV, and ANSI terminal report generators.
- **HTTP & Prometheus Server (`internal/server/`)**: Prometheus `/metrics` exposition and token-authenticated REST APIs (`/api/v1/snapshot`, `/api/v1/diagnose`, `/api/v1/alerts`).
- **Interactive TUI (`internal/tui/`)**: Full-screen 6-tab Bubble Tea (`github.com/charmbracelet/bubbletea`) terminal dashboard with Lip Gloss styling.

---

## 2. Component Inventory

| Subsystem | Package Path | Primary Responsibility | Concurrency / Thread Safety |
| :--- | :--- | :--- | :--- |
| **CLI** | `cmd/` | Flag parsing, global config, subcommands | CLI execution loop |
| **Config** | `internal/config/` | YAML configuration parsing & validation | Read-only post initialization |
| **Collector** | `internal/collector/` | System & container metrics collection | `sync.WaitGroup`, per-collector timeout contexts |
| **Diagnostics** | `internal/diagnostics/` | Rule evaluation & remediation | Stateless evaluator with concurrent checks |
| **Alerts** | `internal/alerts/` | Stateful threshold monitoring & cooldowns | `sync.RWMutex` protecting active alert state |
| **Anomaly** | `internal/anomaly/` | EWMA and Z-score time-series analysis | `sync.RWMutex` protecting statistical histories |
| **Storage** | `internal/storage/` | SQLite persistence, queries & pruning | `sync.Mutex` on SQLite connection, WAL mode |
| **Reporting** | `internal/reporting/` | Multi-format report generation | Pure functions & immutable data |
| **Server** | `internal/server/` | Prometheus exporter & REST API | `http.Server`, atomic metrics collection |
| **TUI** | `internal/tui/` | Interactive terminal UI | Bubble Tea event loop, async tick commands |
| **Logger** | `internal/logger/` | Structured levelled logging | `sync.Mutex` protecting output io.Writer |
| **Utilities** | `pkg/util/` | String, byte unit, network probing helpers | Pure stateless helper functions |

---

## 3. Current Capabilities

1. **System Metrics**:
   - Host metadata (OS, kernel, uptime, virtualization detection).
   - CPU aggregate utilization, per-core load, and load averages (Linux/macOS).
   - Memory physical usage, buffers, cache, available RAM, and swap partition pressure.
   - Storage mount points, total/used/free capacity, percentage used, and inode counters (POSIX).
   - Network interface stats (RX/TX bytes, packets, error rates, drop rates) and connection states.
   - Process tree mapping (PID, PPID, CPU %, RSS memory %, execution state, thread count, I/O rates).
   - Listening TCP/UDP network ports and local endpoints.
   - System service states (systemd units on Linux, Service Control Manager on Windows, launchd on macOS).
2. **Container Telemetry**:
   - Docker / Podman container status, image tags, CPU % and Memory RSS limits.
   - Kubernetes pod discovery, namespace mapping, restart counters, and container phase tracking.
3. **Automated Diagnostics**:
   - Evaluates CPU saturation, load balance, memory pressure, swap exhaustion, disk capacity, inode limits, DNS latency, default gateway reachability, zombie tasks, rogue runaway processes, and container crash-loops.
4. **Alerting & Anomaly Detection**:
   - Configurable thresholds with minimum sustained duration before firing and cooldown suppression.
   - Rolling Z-score anomaly detection flags statistical deviations (> 2.5σ) over 60-sample windows.
5. **Storage & Reporting**:
   - Embedded SQLite time-series storage with automated retention pruning (default 7 days).
   - HTML, JSON, CSV, and ANSI Terminal multi-format reports.
6. **Prometheus & REST API**:
   - Standard `/metrics` Prometheus exposition format.
   - Secure REST API with Bearer token authentication.

---

## 4. Known Limitations & Platform-Specific Behaviors

- **Load Average on Windows**: Windows does not provide Unix-style 1/5/15 minute load averages; Watchdog falls back gracefully to aggregate CPU utilization and core count normalization.
- **Inodes on Windows**: NTFS and FAT32 do not expose Unix inode structures; the Inode check reports `N/A (Non-Unix or unsupported filesystem)` as a neutral status rather than a failure.
- **Service Monitoring**:
  - Windows: Uses `golang.org/x/sys/windows/svc/mgr` for native Service Control Manager queries.
  - Linux: Uses `systemctl list-units` / D-Bus systemd queries.
  - macOS: Uses `launchctl list`.
  - Fallback / Containers: Returns graceful empty list with unsupported reason.
- **Temperature Sensors**: Hardware temperature sensors are platform and vendor-dependent and require elevated root/administrator privileges; Watchdog marks temperature as `UNAVAILABLE` when missing without crashing.
- **Docker / Kubernetes Permissions**: Access to `/var/run/docker.sock` or `~/.kube/config` requires appropriate user permissions; collectors gracefully degrade to `DISABLED` or `UNSUPPORTED` when unavailable.

---

## 5. Security-Sensitive Operations & Attack Surfaces

1. **Process Inspection and Termination (`cmd/dash.go` / `internal/tui/`)**:
   - Interactive process termination sends OS signals (`SIGTERM` / `SIGKILL` or `taskkill`). Must verify that PIDs are sanitized numeric integers and never interpolated into raw shell strings.
2. **REST API & Prometheus Exporter (`internal/server/`)**:
   - Default bind address must be `127.0.0.1` (localhost) to prevent unintended network exposure.
   - REST API endpoints require Bearer Token authentication via `Authorization: Bearer <token>`.
   - Prometheus `/metrics` endpoint exposes high-level telemetry without sensitive credentials or command line arguments containing secrets.
3. **Filesystem Writes (`internal/storage/`, `cmd/report.go`, `cmd/config.go`)**:
   - Report outputs, SQLite databases, and config paths must guard against directory traversal and sanitize destination paths.
4. **Credential & Secret Sanitization**:
   - No plaintext passwords, API keys, or tokens must be logged in application logs or embedded in HTML/JSON/CSV diagnostic reports.

---

## 6. Current Test Coverage Baseline

- All unit tests across all 11 Go packages pass (`cmd`, `internal/alerts`, `internal/anomaly`, `internal/collector`, `internal/config`, `internal/diagnostics`, `internal/logger`, `internal/reporting`, `internal/server`, `internal/storage`, `internal/tui`, `pkg/util`).
- Next steps will validate race conditions (`go test -race ./...`), resource leaks, HTTP API security, anomaly detector scenarios, and database edge cases.
