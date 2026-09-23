# Watchdog Architecture & Internal Subsystems

Watchdog is an enterprise-grade, zero-dependency, cross-platform system monitoring and automated diagnostic tool designed for high throughput, minimal overhead (<1% CPU, <25MB RSS), and robust reliability across Linux, macOS, and Windows.

---

## High-Level Architecture Diagram

```
+-------------------------------------------------------------------------------+
|                                  CLI & TUI Layer                              |
|   cmd/ (Cobra commands: top, diagnose, storage, alerts, export, report, etc.)  |
|   internal/tui/ (Bubble Tea / Lip Gloss terminal UI with real-time graphs)    |
+-------------------------------------------------------------------------------+
                                        |
+-------------------------------------------------------------------------------+
|                            Core Orchestration Engine                          |
|   internal/collector/ (Collector Manager, Worker Pools, Graceful Degradation) |
+-------------------------------------------------------------------------------+
        |                          |                        |
        v                          v                        v
+------------------+    +--------------------+    +--------------------+
| Metrics Engine   |    | Diagnostic Engine  |    | Anomaly & Alerts   |
| (OS, Docker, K8s)|    | (Rule Engine,      |    | (EWMA, Z-Score,    |
| Pure Go Syscalls |    |  Check Execution)  |    |  State Cooldown)   |
+------------------+    +--------------------+    +--------------------+
        |                          |                        |
        +--------------------------+------------------------+
                                   |
+-------------------------------------------------------------------------------+
|                           Data Persistence & Export                           |
|   internal/storage/ (Pure Go modernc.org/sqlite, WAL mode, Schema Migrations) |
|   internal/export/  (JSON, CSV, Self-Contained HTML5 Reports)                 |
|   internal/exporter/ (Prometheus Metrics HTTP Server)                         |
|   internal/agent/   (mTLS Remote Collector gRPC/HTTP Server)                 |
+-------------------------------------------------------------------------------+
```

---

## Key Subsystems

### 1. Collector Subsystem (`internal/collector/`)
- **Isolation & Concurrency**: Collectors execute concurrently via managed goroutines under strict context deadlines.
- **Graceful Fallback**: If an individual collector fails (e.g., Docker daemon socket unavailable or unprivileged permissions), the manager isolates the failure, records error diagnostics, and allows remaining healthy collectors to complete.
- **Zero-Cgo OS Metrics**: Leverages pure Go syscalls and `/proc` filesystem parsers on Linux, WMI/PDH/Win32 APIs on Windows, and `sysctl`/`mach` ports on macOS.

### 2. Diagnostic & Remediation Engine (`internal/diagnostic/`)
- **Automated Root-Cause Analysis**: Runs rule-based checks spanning CPU bottlenecks, memory saturation, I/O wait storms, zombie process leaks, DNS resolution latency, and Docker/K8s health.
- **Remediation Advice**: Provides actionable diagnostic suggestions, remediation CLI commands, and categorized severity levels (`CRITICAL`, `WARNING`, `HEALTHY`).

### 3. Statistical Anomaly Detection (`internal/anomaly/`)
- **Algorithms**: Uses Exponentially Weighted Moving Average (EWMA) combined with dynamic Welford standard deviation and rolling Z-score thresholds.
- **Noise Suppression**: Incorporates warmup periods and stateful hysteresis cooldowns to eliminate metric jitter and false alert cascades.

### 4. Zero-Cgo SQLite Storage Engine (`internal/storage/`)
- **Engine**: Embedded `modernc.org/sqlite` compiled without CGO dependencies for seamless cross-compilation across all target architectures.
- **Performance**: WAL (Write-Ahead Logging) mode, synchronous PRAGMAs, and periodic automated vacuuming and retention pruning.

### 5. Exporter & Agent (`internal/exporter/`, `internal/agent/`)
- **Prometheus Exporter**: Serves standard Prometheus metrics on configurable endpoints (`/metrics`).
- **Remote Agent**: Provides secure remote system metric streaming protected by token-based authentication and TLS encryption, bound strictly to `127.0.0.1` by default.
