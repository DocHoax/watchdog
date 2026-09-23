# Watchdog Configuration Guide

Watchdog loads its configuration hierarchically:
1. Command-line flags (e.g., `--config <path>`)
2. Local directory files (`./watchdog.yaml`, `./watchdog.yml`, `./.watchdog.yaml`)
3. User home directory default (`~/.watchdog/config.yaml`)
4. Built-in defaults

---

## Full Configuration Reference (`watchdog.yaml`)

```yaml
# Polling and refresh frequency for metrics
refresh_interval: 1s

# Persistent SQLite metrics database
storage:
  enabled: true
  db_path: "~/.watchdog/watchdog.db"
  retention_days: 7
  collection_interval: 10s

# Alert thresholds and trigger durations
alerts:
  cpu:
    enabled: true
    threshold: 90.0
    duration: 30s
    cooldown: 5m
  memory:
    enabled: true
    threshold: 85.0
    duration: 30s
    cooldown: 5m
  disk:
    enabled: true
    threshold: 90.0
    duration: 1m
    cooldown: 15m
  process:
    enabled: true
    cpu_threshold: 80.0
    memory_threshold: 70.0
    cooldown: 5m
  network:
    enabled: true
    threshold: 100.0 # dropped or error packets
    duration: 1m
    cooldown: 10m

# Statistical anomaly detection
anomaly:
  enabled: true
  z_score_threshold: 2.5
  window_size: 60
  alpha: 0.2

# Active metric collectors
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

# Terminal UI Settings
dashboard:
  theme: "default" # "default", "monochrome", "nord", "solarized"
  show_per_core_cpu: true
  process_sort_by: "cpu" # "cpu", "memory", "pid", "name"
  process_limit: 50
  pause_on_start: false

# Prometheus Exporter
prometheus:
  enabled: false
  port: 9100
  path: "/metrics"

# Remote Agent Daemon
agent:
  enabled: false
  port: 8443
  bind_address: "127.0.0.1" # Secure default (localhost only)
  token: ""
  tls_cert: ""
  tls_key: ""

# Container & Orchestrator Discovery
docker:
  enabled: true
  host: "" # Defaults to unix:///var/run/docker.sock or npipe:////./pipe/docker_engine
kubernetes:
  enabled: false
  kubeconfig: "" # Defaults to ~/.kube/config
  namespace: ""
```

---

## Validation Rules
- `refresh_interval`: Must be >= 100ms.
- `storage.retention_days`: Must be >= 1.
- `alerts.*.threshold`: Percentage values must be between 0.0 and 100.0.
- `prometheus.port` and `agent.port`: Must be valid TCP ports (1-65535).
- `agent.bind_address`: Defaults to loopback `127.0.0.1` to prevent unintended network exposure.
