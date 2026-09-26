# 🐺 Watchdog

> **Enterprise-Grade, Cross-Platform System Observability, Automated Diagnostics & Real-Time Terminal Dashboard in Pure Go**

[![Release](https://img.shields.io/badge/release-v1.0.0-blue.svg?style=flat&logo=github)](https://github.com/DocHoax/watchdog/releases)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Zero CGO](https://img.shields.io/badge/CGO-disabled-success?style=flat)](https://github.com/DocHoax/watchdog)
[![Platform Support](https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey?style=flat)](docs/platforms.md)
[![License](https://img.shields.io/badge/license-MIT-green.svg?style=flat)](LICENSE)

Watchdog is an all-in-one system health monitoring, diagnostic automation, and live telemetry CLI. Engineered with a **strict Zero-CGO pure Go architecture**, Watchdog delivers point-in-time heuristic diagnostics, statistical anomaly detection, persistent local time-series metrics, OpenMetrics/Prometheus exposition, and an interactive AltScreen Bubble Tea terminal user interface—with a minimal footprint ($<1\%$ CPU overhead and $<25\text{ MB}$ RSS memory).

---

## 🌟 Architectural Pillars

- ⚡ **Zero-CGO & Pure Go**: Compiles to a single static binary with zero external runtime dependencies (`CGO_ENABLED=0`).
- 🩺 **Automated 10-Rule Diagnostics**: Concurrently evaluates system saturation, inode exhaustion, DNS latency, paging spikes, and container health with actionable `--fix` remediation.
- 🖥️ **Interactive 6-Tab AltScreen TUI**: Real-time terminal interface with per-core CPU bars, memory breakdown, sparkline history, process sorting/filtering/killing, and container states.
- 🚨 **Temporal Alerting Engine**: Hysteresis-aware threshold alerts with verification duration windows, suppression cooldowns, and SQLite event auditing.
- 📈 **Statistical Anomaly Detection**: Pure-Go online Exponentially Weighted Moving Average (EWMA) filtering and rolling $Z$-score metric evaluation ($Z \ge 2.5$).
- 💾 **Embedded SQLite Time-Series**: Embedded WAL-mode database powered by `modernc.org/sqlite` with automated background retention pruning.
- 🛡️ **Structured Security Audit Trails**: Zero-credential-leakage audit logging for authentication, lifecycle, TLS, config, and admin events with SQLite persistence, DoS flood throttling, and CSV formula injection neutralization.
- 📊 **Self-Contained Multi-Format Reports**: Single-file dark-themed HTML5 reports with inline SVG vector sparklines (zero external JS/CDN requests), structured JSON, CSV, and ANSI terminal summaries.
- 🌐 **Prometheus Exporter & REST API**: Native `/metrics` OpenMetrics endpoint, authenticated REST APIs, and runtime `pprof` profiling.

---

## 🖥️ Terminal Dashboard (TUI)

```text
┌─ Watchdog v1.0.0 ───────────────────────────────────────────────────── [Host: prod-db-01] ─┐
│ [1] Dashboard  [2] Processes  [3] Storage & Net  [4] Containers  [5] Services  [6] Diag/Alerts │
├──────────────────────────────────────────────────────────────────────────────────────────────┤
│ CPU [|||||||||||||||||||||||||||||||                    ] 42.5%  Cores: 8  Load: 1.24 1.45 1.10 │
│   Core 0: [||||||||||||||||||||    ] 51.2%    Core 1: [||||||||||||||        ] 36.4%         │
│   Core 2: [||||||||||||||||||||||||] 78.1%    Core 3: [||||||||              ] 22.0%         │
│                                                                                              │
│ Memory [||||||||||||||||||||||||||||||||||              ] 52.6%  8.42 GB / 16.00 GB          │
│ Swap   [||                                              ]  1.5%  128 MB / 8.00 GB            │
│                                                                                              │
│ Disk I/O (nvme0n1)                                    Network (eth0)                         │
│ Read:  1.24 MB/s  ▂▃▅▆▇▆▅▃▂ ▂▃▅                      RX: 42.8 KB/s   ▂▃▅▆▇▆▅▃▂ ▂▃           │
│ Write: 8.45 MB/s   ▂▃▄▅▆▇█▇▆▅▄▃                      TX: 18.2 KB/s   ▂▃▄▅▆▇▆▅▄▃             │
├──────────────────────────────────────────────────────────────────────────────────────────────┤
│ PID    USER       CPU%    MEM%    VIRT      RES       STATE   TIME       COMMAND             │
│ 1042   postgres   14.2%   8.4%    1.2 GB    540 MB    S       14:22.10   postgres: writer    │
│ 1824   node       8.5%    4.1%    850 MB    280 MB    S       08:12.44   node /app/server.js │
│ 942    redis      1.2%    1.8%    320 MB    112 MB    S       02:40.12   redis-server *:6379 │
├──────────────────────────────────────────────────────────────────────────────────────────────┤
│ [Tab/1-6] Switch View  [/] Filter  [c/m/p] Sort  [k] Kill  [r] Refresh  [?] Help  [q] Exit   │
└──────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 💻 Platform Support Matrix

Watchdog is continuously verified across all major enterprise operating systems and architectures:

| Platform | OS / Kernel | Architectures | Collection Backend |
| :--- | :--- | :--- | :--- |
| **Linux** | Kernel 3.10+ (Ubuntu, Debian, RHEL, Fedora, Alpine) | `amd64`, `arm64`, `armv7` | Native `/proc`, `/sys`, `netlink`, Unix sockets |
| **macOS** | macOS 11.0+ (Big Sur, Monterey, Ventura, Sonoma, Sequoia) | `arm64` (Apple Silicon), `amd64` | `sysctl`, Mach Kernel APIs, `launchd` |
| **Windows** | Windows 10/11, Windows Server 2016–2025 | `amd64`, `arm64` | Win32 APIs, WMI, Windows Service Control Manager |
| **Docker** | Engine 20.10+ (Standalone & Swarm) | `linux/amd64`, `linux/arm64` | Docker Engine Unix/Named-Pipe API |
| **Kubernetes**| Kubernetes 1.24+ | Cluster-wide | `kubectl` CLI & In-Cluster API Client |

---

## 📦 Installation

### 1. Pre-Compiled Binary Releases
Download official release archives and system packages for your operating system and architecture from the [GitHub Releases](https://github.com/DocHoax/watchdog/releases) page:

#### Linux (AMD64 / ARM64 / ARMv7)
```bash
# Linux AMD64 (x86_64)
curl -sSL https://github.com/DocHoax/watchdog/releases/download/v1.0.0/watchdog_1.0.0_linux_amd64.tar.gz | tar -xz
sudo mv watchdog /usr/local/bin/
sudo chmod +x /usr/local/bin/watchdog

# Linux ARM64 (AWS Graviton, Raspberry Pi 4/5)
curl -sSL https://github.com/DocHoax/watchdog/releases/download/v1.0.0/watchdog_1.0.0_linux_arm64.tar.gz | tar -xz
sudo mv watchdog /usr/local/bin/
sudo chmod +x /usr/local/bin/watchdog
```

*Package Formats*: Debian/Ubuntu (`.deb`), RHEL/CentOS/Fedora (`.rpm`), and Alpine (`.apk`) packages are available on the [Releases](https://github.com/DocHoax/watchdog/releases/tag/v1.0.0) page.

#### macOS (Apple Silicon / Intel)
```bash
# macOS Apple Silicon (ARM64)
curl -sSL https://github.com/DocHoax/watchdog/releases/download/v1.0.0/watchdog_1.0.0_darwin_arm64.tar.gz | tar -xz
sudo mv watchdog /usr/local/bin/
sudo chmod +x /usr/local/bin/watchdog

# macOS Intel (AMD64)
curl -sSL https://github.com/DocHoax/watchdog/releases/download/v1.0.0/watchdog_1.0.0_darwin_amd64.tar.gz | tar -xz
sudo mv watchdog /usr/local/bin/
sudo chmod +x /usr/local/bin/watchdog
```

#### Windows (PowerShell)
```powershell
# Windows x64 (AMD64) via PowerShell
Invoke-WebRequest -Uri "https://github.com/DocHoax/watchdog/releases/download/v1.0.0/watchdog_1.0.0_windows_amd64.zip" -OutFile "watchdog.zip"
Expand-Archive -Path "watchdog.zip" -DestinationPath "$env:ProgramFiles\Watchdog" -Force
$env:Path += ";$env:ProgramFiles\Watchdog"
```

#### Windows (Command Prompt / cmd.exe)
```cmd
curl.exe -L -o watchdog.zip "https://github.com/DocHoax/watchdog/releases/download/v1.0.0/watchdog_1.0.0_windows_amd64.zip"
tar.exe -xf watchdog.zip
watchdog.exe version
```

### 2. Go Module Install (Go 1.22+)
```bash
# Install latest release binary directly via Go toolchain
go install github.com/DocHoax/watchdog@v1.0.0

# Verify installation
watchdog version
```

### 3. Build from Source
```bash
git clone https://github.com/DocHoax/watchdog.git
cd watchdog
CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/watchdog .
./bin/watchdog version
```

### 4. Container Deployment
```bash
# Build and run locally via Docker
docker build -t watchdog:v1.0.0 .
docker run -it --rm --pid=host --net=host \
  -v /proc:/host/proc:ro -v /sys:/host/sys:ro \
  watchdog:v1.0.0 dash
```

---

## ⚡ Quickstart Commands

```bash
# 1. Launch real-time Terminal Dashboard (TUI)
watchdog dash

# 2. Run automated diagnostic checks across 10 system rules
watchdog diagnose

# 3. Run diagnostic checks with actionable remediation advice
watchdog diagnose --fix

# 4. Generate a standalone, zero-dependency HTML5 health report
watchdog report --format html --output health-report.html --history 2h

# 5. Start Prometheus exporter and authenticated REST API daemon
watchdog server --port 9100 --token "s3cret-token"

# 6. List currently active threshold alerts
watchdog alert list

# 7. Export time-series metrics from embedded SQLite database
watchdog export --format csv --metric cpu_usage_pct --since 24h --output cpu_24h.csv

# 8. List security audit events from the past 24 hours
watchdog audit list --since 24h

# 9. Export audit events to CSV for compliance review
watchdog audit export --since 30d --format csv --output audit.csv

# 10. Check version information
watchdog version
watchdog version --short
watchdog version --json
```

---

## 🛠️ CLI Command Hierarchy

| Command | Subcommands / Aliases | Flags & Options | Description |
| :--- | :--- | :--- | :--- |
| **`watchdog dash`** | `dashboard`, `tui`, `top` | `-i, --interval`, `--per-core`, `--sort`, `--process-limit`, `--remote`, `--token` | Starts interactive full-screen AltScreen TUI. |
| **`watchdog diagnose`** | `diag`, `check`, `doctor` | `-C, --category`, `-F, --fix`, `--json`, `--plain`, `-t, --timeout` | Evaluates 10 diagnostic health rules. |
| **`watchdog report`** | `generate-report`, `export-report` | `-f, --format (html\|json\|csv\|terminal)`, `-o, --output`, `-H, --history`, `-t, --title`, `--raw-json` | Generates point-in-time system health reports. |
| **`watchdog server`** | `serve`, `daemon` | `-p, --port`, `-b, --host`, `--token`, `--tls-cert`, `--tls-key` | Runs Prometheus `/metrics` exporter and REST API. |
| **`watchdog agent`** | — | `-p, --port`, `-b, --host`, `--token`, `--tls-cert`, `--tls-key` | Runs headless background telemetry agent. |
| **`watchdog alert`** | `list`, `history`, `test` | `--limit` | Queries active and historical alerts or emits test events. |
| **`watchdog audit`** | `list`, `export`, `audits` | `--since`, `--until`, `-t, --event-type`, `-s, --severity`, `--outcome`, `--limit`, `--format`, `--json` | Manages and queries security audit event logs. |
| **`watchdog config`** | `init`, `validate`, `show`, `path` | `[path]`, `--json` | Manages and validates YAML configuration. |
| **`watchdog export`** | — | `-f, --format`, `-m, --metric`, `-s, --since`, `-o, --output` | Dumps metrics from embedded SQLite database. |
| **`watchdog completion`** | `bash`, `zsh`, `fish`, `powershell` | — | Generates shell autocomplete scripts. |
| **`watchdog version`** | — | `--json`, `--short` | Displays binary version, commit, and build date. |

---

## ⚙️ Configuration (`~/.watchdog/config.yaml`)

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
    threshold: 90.0                  # Trigger when CPU >= 90%
    duration: 30s                    # Must sustain for 30s
    cooldown: 5m                     # Suppress duplicate alerts for 5m
  memory:
    enabled: true
    threshold: 85.0                  # Trigger when RAM >= 85%
    duration: 30s
    cooldown: 5m
  disk:
    enabled: true
    threshold: 90.0                  # Trigger when partition >= 90%
    duration: 1m
    cooldown: 15m
  process:
    enabled: true
    cpu_threshold: 80.0
    memory_threshold: 70.0
    cooldown: 5m
  network:
    enabled: true
    threshold: 100.0                 # Error packet threshold
    duration: 1m
    cooldown: 10m

anomaly:
  enabled: true
  z_score_threshold: 2.5             # Trigger anomaly when |Z| >= 2.5
  window_size: 60                    # 60-sample rolling ring buffer
  alpha: 0.2                         # EWMA smoothing factor

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

audit:
  enabled: true                      # Structured security audit logging
  retention_days: 90                 # Audit log retention window (in days)
  max_query_limit: 1000              # Maximum events per query
```

---

## 📚 Technical Documentation Suite

| Document | Description |
| :--- | :--- |
| 🚀 [**Getting Started**](docs/getting-started.md) | First-run tour, initial configuration, and basic operations. |
| 📦 [**Installation Guide**](docs/installation.md) | Package managers, tarball verification, container images, and source compilation. |
| 🛠️ [**CLI Command Reference**](docs/cli-reference.md) | Exhaustive breakdown of all CLI subcommands, flags, defaults, and aliases. |
| ⚙️ [**Configuration Reference**](docs/configuration.md) | Full YAML schema specification, precedence hierarchy, and defaults. |
| 🖥️ [**TUI & Real-Time Monitoring**](docs/monitoring.md) | Interactive AltScreen navigation, vim keybindings, process management, and themes. |
| 🩺 [**Diagnostic Heuristics Engine**](docs/diagnostics.md) | Concurrent evaluation of 10 heuristic rules and actionable `--fix` remediation. |
| 🚨 [**Threshold Alerting Engine**](docs/alerts.md) | Temporal duration verification, hysteresis suppression, and state tracking. |
| 💾 [**Embedded SQLite Time-Series**](docs/history.md) | Pure-Go SQLite architecture, WAL mode tuning, and retention pruning. |
| 📈 [**Statistical Anomaly Detection**](docs/anomaly-detection.md) | Ring buffer calculations, EWMA smoothing, and rolling Z-score evaluation. |
| 📊 [**Standalone Reporting**](docs/reporting.md) | Self-contained HTML5 reports with inline SVG vector graphs, CSV, and JSON schemas. |
| 🌐 [**Prometheus & REST API**](docs/prometheus-api.md) | OpenMetrics `/metrics` exposition, Bearer token auth, and remote TUI connection. |
| 🐳 [**Docker Monitoring**](docs/docker.md) | Socket discovery, container resource metrics, and crashloop diagnostics. |
| ☸️ [**Kubernetes Telemetry**](docs/kubernetes.md) | Cluster health, pod restart rates, node capacity, and DaemonSet deployment. |
| 🚦 [**Troubleshooting & Exit Codes**](docs/troubleshooting.md) | Diagnostic resolutions, permission requirements, and deterministic exit codes. |
| 💻 [**Platform Support & Kernel APIs**](docs/platforms.md) | OS compatibility matrix, kernel backends, and capability requirements. |
| 🧪 [**Developer Guide**](docs/development.md) | Zero-CGO build policy, testing standards, benchmark suites, and custom rules. |
| 🏛️ [**System Architecture**](docs/architecture.md) | Subsystem design, concurrency model, data flow, and resource budget limits. |

---

## 🤝 Contributing & Community Governance

We welcome contributions from the community! Please review our governance guidelines before submitting code:

- 📖 [**Contributing Guidelines**](CONTRIBUTING.md)
- 🛡️ [**Security Policy & Vulnerability Reporting**](SECURITY.md)
- 🤝 [**Code of Conduct**](CODE_OF_CONDUCT.md)

---

## 📄 License

Watchdog is licensed under the [MIT License](LICENSE).
