# Watchdog Configuration Guide

Watchdog features a flexible, hierarchical configuration system loaded from YAML files, command-line arguments, or built-in defaults.

---

## 🔍 Discovery & Precedence Order

When starting up, Watchdog resolves configuration in the following order (highest to lowest priority):

1. **Explicit CLI Flags** (e.g. `--config /etc/watchdog.yaml`, `--interval 500ms`)
2. **Current Working Directory** (`./watchdog.yaml`, `./watchdog.yml`, `./.watchdog.yaml`)
3. **User Home Directory** (`~/.watchdog/config.yaml`)
4. **Compiled-in Production Defaults**

---

## 📄 Annotated `config.yaml` Reference

```yaml
# ==============================================================================
# Watchdog Configuration File
# Default path: ~/.watchdog/config.yaml
# ==============================================================================

# Global polling and refresh frequency for live terminal dashboard
refresh_interval: 1s

# ------------------------------------------------------------------------------
# Embedded Time-Series Storage Configuration
# Pure Go SQLite engine (modernc.org/sqlite) in Write-Ahead Logging (WAL) mode
# ------------------------------------------------------------------------------
storage:
  enabled: true
  db_path: "~/.watchdog/watchdog.db" # Database file path on disk
  retention_days: 7                 # Automatic data pruning retention window (in days)
  collection_interval: 10s           # Metric sample write frequency to disk

# ------------------------------------------------------------------------------
# Threshold Alert Engine Configuration
# Temporal rules with trigger duration tracking and hysteresis cooldowns
# ------------------------------------------------------------------------------
alerts:
  cpu:
    enabled: true
    threshold: 90.0                  # Aggregate CPU utilization percentage
    duration: 30s                    # Sustained window required before firing
    cooldown: 5m                     # Suppression cooldown after resolution
  memory:
    enabled: true
    threshold: 85.0                  # Physical RAM utilization percentage
    duration: 30s
    cooldown: 5m
  disk:
    enabled: true
    threshold: 90.0                  # Filesystem partition capacity percentage
    duration: 1m
    cooldown: 15m
  process:
    enabled: true
    cpu_threshold: 80.0              # Single-process CPU utilization percentage
    memory_threshold: 70.0           # Single-process RAM percentage
    cooldown: 5m
  network:
    enabled: true
    threshold: 100.0                 # Error / dropped packet counter threshold
    duration: 1m
    cooldown: 10m

# ------------------------------------------------------------------------------
# Online Statistical Anomaly Detection
# Real-time EWMA smoothing and rolling Z-score outlier detection
# ------------------------------------------------------------------------------
anomaly:
  enabled: true
  z_score_threshold: 2.5             # Number of standard deviations to trigger anomaly
  window_size: 60                    # Rolling observation sample window size
  alpha: 0.2                         # EWMA smoothing factor (0.0 < alpha <= 1.0)

# ------------------------------------------------------------------------------
# Subsystem Metric Collectors
# Toggle individual hardware and OS collector modules
# ------------------------------------------------------------------------------
collectors:
  cpu: true                          # CPU usage, per-core metrics, load average
  memory: true                       # RAM breakdown, swap utilization, memory paging
  disk: true                         # Partition usage, read/write I/O throughput
  network: true                      # Adapter throughput, packet counters, errors
  process: true                      # Process list, thread counts, CPU/Mem rankings
  service: true                      # systemd / Windows Services / launchd status
  port: true                         # Open listening TCP/UDP sockets
  docker: true                       # Docker container health, states, and limits
  kubernetes: false                  # Kubernetes cluster node/pod diagnostics

# ------------------------------------------------------------------------------
# Interactive Terminal User Interface (TUI)
# Bubble Tea and Lip Gloss presentation options
# ------------------------------------------------------------------------------
dashboard:
  theme: "default"                   # Options: default, dark, light, nord, monokai, solarized, dracula
  show_per_core_cpu: true            # Display individual progress bars for each CPU core
  process_sort_by: "cpu"             # Default process sort column: cpu, memory, pid, name
  process_limit: 50                  # Maximum process rows rendered in table view
  pause_on_start: false              # Launch dashboard in paused state

# ------------------------------------------------------------------------------
# Prometheus Metrics Exporter
# Exposes OpenMetrics / Prometheus /metrics HTTP endpoint
# ------------------------------------------------------------------------------
prometheus:
  enabled: false                     # Enable background Prometheus exporter
  port: 9100                         # Exporter listener TCP port
  path: "/metrics"                   # Metric scraping endpoint URL path

# ------------------------------------------------------------------------------
# Remote Agent Daemon Configuration
# Secure telemetry streaming for multi-node monitoring
# ------------------------------------------------------------------------------
agent:
  enabled: false                     # Enable remote agent listener
  port: 8443                         # Remote agent TCP port
  bind_address: "127.0.0.1"          # Secure default (use 0.0.0.0 for all interfaces)
  token: ""                          # Bearer token for authentication (or use token_file/token_env)
  token_file: ""                     # Path to secret file containing token (e.g. /etc/watchdog/token)
  token_env: ""                      # Environment variable name containing token
  tls_cert: ""                       # Path to TLS certificate PEM file
  tls_cert_file: ""                  # Path to file containing TLS certificate path
  tls_cert_env: ""                   # Environment variable name containing TLS certificate path
  tls_key: ""                        # Path to TLS private key PEM file
  tls_key_file: ""                   # Path to file containing TLS private key path
  tls_key_env: ""                    # Environment variable name containing TLS private key path

# ------------------------------------------------------------------------------
# Container & Orchestration Discovery
# ------------------------------------------------------------------------------
docker:
  enabled: true                      # Query Docker daemon
  host: ""                           # Custom socket URL (defaults to platform socket)

kubernetes:
  enabled: false                     # Query Kubernetes cluster
  kubeconfig: ""                     # Path to kubeconfig (defaults to ~/.kube/config)
  namespace: ""                      # Target namespace filter (empty for all)
```

---

## 🛡️ Validation Rules & Constraints

Watchdog validates all configuration properties on startup or via `watchdog config validate`:

| Setting | Constraint | Error Condition |
| :--- | :--- | :--- |
| `refresh_interval` | `>= 100ms` | Fails if refresh interval is too aggressive |
| `storage.retention_days` | `>= 1` | Retention period must be at least 1 day |
| `alerts.*.threshold` | `0.0 <= x <= 100.0` | Percentage thresholds must be bounded within 0–100% |
| `anomaly.z_score_threshold` | `> 0.0` | Z-score threshold must be strictly positive |
| `anomaly.alpha` | `0.0 < alpha <= 1.0` | EWMA alpha smoothing factor must be within `(0.0, 1.0]` |
| `prometheus.port` | `1 <= port <= 65535` | Port must be a valid TCP port number |
| `agent.port` | `1 <= port <= 65535` | Port must be a valid TCP port number |

---

## 🔒 Secret Resolution & Redaction

Watchdog supports flexible secret provisioning across files, environment variables, and configuration values:

### Precedence Order
1. **Explicit CLI Flags** (`--token`, `--token-file`, `--tls-key`, etc.)
2. **Secret Files** (`agent.token_file`, `agent.tls_key_file`, `agent.tls_cert_file`)
3. **Environment Variables** (`WATCHDOG_AGENT_TOKEN` / `agent.token_env`, `WATCHDOG_AGENT_TLS_KEY`, `WATCHDOG_AGENT_TLS_CERT`)
4. **Configuration Fields** (`agent.token`, `agent.tls_key`, `agent.tls_cert`)

### Secret Redaction in `config show`
Running `watchdog config show` or `watchdog config show --json` automatically masks sensitive values (`token`, `tls_key`) with `[REDACTED]` to prevent secret leakage in console scrollback and logs.

---

## 🛠️ Configuration CLI Commands

```bash
# Generate a new default config file
watchdog config init

# Generate to a custom path (with overwrite flag)
watchdog config init /etc/watchdog/config.yaml --force

# Validate existing configuration
watchdog config validate

# Display loaded configuration in YAML
watchdog config show

# Display loaded configuration in JSON
watchdog config show --json

# Print the active configuration file path on disk
watchdog config path
```
