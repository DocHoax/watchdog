# Watchdog CLI Command Reference

Comprehensive reference guide for all Watchdog CLI commands, subcommands, flags, environment variables, and exit codes.

---

## Global Options

All commands accept the following global flags:

| Flag | Shorthand | Type | Default | Description |
| :--- | :---: | :---: | :---: | :--- |
| `--config` | `-c` | `string` | Auto | Path to configuration file (e.g. `/etc/watchdog/config.yaml`) |
| `--verbose` | `-v` | `bool` | `false` | Enable verbose diagnostic logging |
| `--quiet` | `-q` | `bool` | `false` | Suppress non-essential output |
| `--json-logs` | | `bool` | `false` | Output logs in structured JSON format |
| `--no-color` | | `bool` | `false` | Disable ANSI color codes and styles |

---

## Deterministic Exit Codes

Watchdog implements standardized, deterministic process exit codes:

| Code | Constant | Meaning |
| :---: | :--- | :--- |
| `0` | `ExitSuccess` | Successful execution; all diagnostic checks passed |
| `1` | `ExitGeneralError` | Runtime error, critical diagnostic issue detected, or unhandled exception |
| `2` | `ExitUsageError` | Invalid CLI flags, unknown arguments, or syntax errors |
| `3` | `ExitConfigError` | Configuration file missing, unparseable, or failed schema validation |
| `4` | `ExitAuthError` | Authentication failed, invalid Bearer token, or unauthorized API access |
| `5` | `ExitNetworkError` | Network unreachable, remote agent connection failed, or socket error |

---

## Command Hierarchy

```
watchdog [command]
├── dash (dashboard, tui, top)    # Interactive real-time terminal UI
├── diagnose (diag, check, doctor)# Automated diagnostic rule evaluation
├── report (generate-report)       # HTML5, JSON, CSV report generator
├── server (serve, daemon)        # Prometheus exporter and REST API server
├── agent                         # Background metrics collector daemon
├── alert                         # Alert rule management and history
│   ├── list                      # List currently firing alerts
│   ├── history                   # Query historical alert records
│   └── test                      # Dispatch synthetic test alert
├── config                        # Manage YAML configuration
│   ├── init                      # Generate default config file
│   ├── validate                  # Validate configuration syntax and ranges
│   ├── show                      # Display resolved active configuration
│   └── path                      # Print configuration file path
├── audit (audits)                # Security audit logging & event management
│   ├── list                      # List and filter security audit events
│   └── export                    # Export audit logs to JSON/CSV format
├── node                          # Local node identity and hardware specs
├── fleet (fleets)                # Centralized fleet cluster management
│   ├── status                    # Aggregate fleet health summary
│   ├── list                      # List and filter registered nodes
│   ├── get                       # Inspect detailed node state & telemetry
│   ├── register                  # Register a node with fleet controller
│   ├── heartbeat                 # Transmit manual heartbeat ping
│   └── deregister                # Decommission and remove a node
├── export                        # Export snapshots or metrics to JSON/CSV
├── completion                    # Generate shell completion scripts
└── version                       # Print version and build metadata
```

---

## Subcommand Details

### 1. `watchdog dash`
*Aliases*: `dashboard`, `tui`, `top`

Launches the interactive AltScreen Bubble Tea terminal dashboard. If no subcommand is specified, `watchdog` launches the dashboard by default.

```bash
watchdog dash [flags]
```

#### Flags
| Flag | Shorthand | Type | Default | Description |
| :--- | :---: | :---: | :---: | :--- |
| `--interval` | `-i` | `duration` | `1s` | Metrics polling refresh interval (e.g. `500ms`, `1s`, `2s`) |
| `--theme` | `-t` | `string` | `default` | Color theme (`default`, `dark`, `light`, `nord`, `monokai`, `solarized`, `dracula`) |
| `--remote` | `-r` | `string` | `""` | Remote agent host address (`host:port` or URL) |
| `--token` | | `string` | `""` | Authentication token for remote agent connection |
| `--token-file` | | `string` | `""` | Path to file containing authentication token |
| `--token-env` | | `string` | `""` | Environment variable name containing authentication token |
| `--insecure`| | `bool` | `false` | Skip TLS certificate verification for remote connections |

#### Examples
```bash
# Launch dashboard with 500ms refresh rate
watchdog dash --interval 500ms

# Launch with Nord theme
watchdog dash --theme nord

# Connect to remote agent over TLS using token from file
watchdog dash --remote https://192.168.1.50:8443 --token-file /etc/watchdog/token
```

---

### 2. `watchdog diagnose`
*Aliases*: `diag`, `check`, `doctor`

Runs automated heuristic health checks against CPU, Memory, Disk, Network, Process, Service, Docker, and Kubernetes subsystems. Returns exit code `1` if any critical defects are discovered.

```bash
watchdog diagnose [flags]
```

#### Flags
| Flag | Shorthand | Type | Default | Description |
| :--- | :---: | :---: | :---: | :--- |
| `--category` | `-C` | `string` | `""` | Filter checks by category (`CPU`, `Memory`, `Disk`, `Network`, `Process`, `Docker`) |
| `--fix` | `-F` | `bool` | `false` | Print actionable remediation instructions and CLI commands |
| `--json` | `-j` | `bool` | `false` | Output diagnostic report as structured JSON |
| `--output` | `-o` | `string` | `""` | Write diagnostic report to target file path |
| `--plain` | `-p` | `bool` | `false` | Disable ANSI formatting and color styling |
| `--timeout` | `-t` | `duration` | `10s` | Maximum execution timeout for diagnostic checks |

#### Examples
```bash
# Run diagnostics with remediation instructions
watchdog diagnose --fix

# Filter diagnostics strictly to memory subsystem
watchdog diagnose --category Memory --fix

# Export diagnostic report to JSON file
watchdog diagnose --json --output /tmp/diagnostics.json
```

---

### 3. `watchdog report`
*Aliases*: `generate-report`, `export-report`

Generates standalone system health reports with optional embedded SVG trend charts, historical analysis, and multi-format output.

```bash
watchdog report [flags]
```

#### Flags
| Flag | Shorthand | Type | Default | Description |
| :--- | :---: | :---: | :---: | :--- |
| `--format` | `-f` | `string` | `html` | Output format: `html`, `json`, `csv`, `terminal` |
| `--output` | `-o` | `string` | `""` | Target output file path (or `-` for stdout) |
| `--history` | `-H` | `duration` | `1h` | Historical lookback window for trend charts (e.g. `30m`, `2h`, `24h`) |
| `--title` | `-t` | `string` | `""` | Custom title string in generated report header |
| `--charts` | | `bool` | `true` | Embed inline SVG sparkline and trend charts in HTML reports |
| `--raw-json`| | `bool` | `false` | Embed full raw JSON snapshot payload in HTML document |

#### Examples
```bash
# Generate standalone self-contained HTML report with 2 hours of history
watchdog report --format html --output /var/reports/system.html --history 2h --title "Production DB Node Health"

# Output diagnostic summary directly to terminal
watchdog report --format terminal

# Export historical metric CSV
watchdog report --format csv --output /tmp/metrics.csv --history 24h
```

---

### 4. `watchdog server`
*Aliases*: `serve`, `daemon`

Runs the HTTP background service serving Prometheus `/metrics` exposition and authenticated REST API endpoints.

```bash
watchdog server [flags]
```

#### Flags
| Flag | Shorthand | Type | Default | Description |
| :--- | :---: | :---: | :---: | :--- |
| `--port` | `-p` | `int` | `8443` | HTTP listening port |
| `--host` | `-H` | `string` | `127.0.0.1` | Network interface to bind (use `0.0.0.0` for all interfaces) |
| `--token` | `-t` | `string` | `""` | Authentication token required for API endpoints |
| `--token-file` | | `string` | `""` | Path to file containing authentication token |
| `--token-env` | | `string` | `""` | Environment variable name containing authentication token |
| `--tls-cert`| | `string` | `""` | Path to TLS certificate PEM file |
| `--tls-cert-file`| | `string` | `""` | Path to file containing TLS certificate path |
| `--tls-cert-env` | | `string` | `""` | Environment variable name containing TLS certificate path |
| `--tls-key` | | `string` | `""` | Path to TLS private key PEM file |
| `--tls-key-file` | | `string` | `""` | Path to file containing TLS private key path |
| `--tls-key-env`  | | `string` | `""` | Environment variable name containing TLS private key path |

#### Examples
```bash
# Launch Prometheus exporter on port 8443 (localhost only)
watchdog server --port 8443

# Launch server on all interfaces with TLS and token from secret file
watchdog server --host 0.0.0.0 --port 8443 --token-file /etc/watchdog/token --tls-cert /etc/ssl/cert.pem --tls-key /etc/ssl/key.pem
```

---

### 5. `watchdog agent`

Runs a lightweight background metrics collection daemon that periodically samples host telemetry and persists it to local SQLite storage.

```bash
watchdog agent [flags]
```

#### Flags
| Flag | Shorthand | Type | Default | Description |
| :--- | :---: | :---: | :---: | :--- |
| `--interval` | `-i` | `duration` | `2s` | Sampling and storage persistence interval |
| `--port` | `-p` | `int` | `8443` | Remote agent API listener port |
| `--token` | `-t` | `string` | `""` | Bearer token required for remote client connections |
| `--token-file` | | `string` | `""` | Path to file containing authentication token |
| `--token-env` | | `string` | `""` | Environment variable name containing authentication token |
| `--tls-cert`| | `string` | `""` | Path to TLS certificate PEM file |
| `--tls-cert-file`| | `string` | `""` | Path to file containing TLS certificate path |
| `--tls-cert-env` | | `string` | `""` | Environment variable name containing TLS certificate path |
| `--tls-key` | | `string` | `""` | Path to TLS private key PEM file |
| `--tls-key-file` | | `string` | `""` | Path to file containing TLS private key path |
| `--tls-key-env`  | | `string` | `""` | Environment variable name containing TLS private key path |

---

### 6. `watchdog alert`

Inspect and test threshold alert configurations and historical triggers.

```bash
watchdog alert [command]
```

#### Subcommands
- `watchdog alert list`: Displays currently active / firing alerts.
- `watchdog alert history`: Queries historical alert records from SQLite database (`--limit 50`).
- `watchdog alert test`: Sends a synthetic test alert to verify notification channels and terminal alerts.

---

### 7. `watchdog config`

Manages the YAML configuration file (`~/.watchdog/config.yaml`).

```bash
watchdog config [command]
```

#### Subcommands
- `watchdog config init [path]`: Generates a fully documented default configuration file (`--force` to overwrite).
- `watchdog config validate`: Validates syntax, required fields, and threshold ranges.
- `watchdog config show`: Prints the resolved active configuration (`--json` for JSON output).
- `watchdog config path`: Prints the absolute filesystem path of the loaded configuration file.

---

### 8. `watchdog audit`
*Aliases*: `audits`

Inspects, queries, and exports security audit logs stored in the local SQLite database.

```bash
watchdog audit [command] [flags]
```

#### Subcommands
- `watchdog audit list [flags]`: List and filter recorded security audit events (table or JSON).
- `watchdog audit export [flags]`: Export audit logs to JSON or CSV format.

#### Flags (`audit list`)
| Flag | Shorthand | Type | Default | Description |
| :--- | :---: | :---: | :---: | :--- |
| `--since` | | `string` | `24h` | Filter events created since duration or timestamp (e.g. `2h`, `7d`, `2026-09-26T12:00:00Z`) |
| `--until` | | `string` | `""` | Filter events created until duration or timestamp |
| `--event-type` | `-t` | `string` | `""` | Filter by audit event type (e.g. `auth.failure`, `server.start`) |
| `--severity` | `-s` | `string` | `""` | Filter by severity (`info`, `warning`, `error`, `critical`) |
| `--outcome` | | `string` | `""` | Filter by outcome (`success`, `failure`, `denied`) |
| `--source` | | `string` | `""` | Filter by source IP address |
| `--actor` | | `string` | `""` | Filter by actor identity |
| `--request-id` | | `string` | `""` | Filter by correlation request ID |
| `--limit` | `-l` | `int` | `100` | Maximum number of events to return (default 100, max 1000) |
| `--offset` | | `int` | `0` | Pagination offset |
| `--format` | `-f` | `string` | `terminal` | Output format: `terminal` or `json` |
| `--json` | | `bool` | `false` | Output results in structured JSON (shorthand for `--format json`) |

#### Flags (`audit export`)
| Flag | Shorthand | Type | Default | Description |
| :--- | :---: | :---: | :---: | :--- |
| `--output` | `-o` | `string` | `""` | File path to write exported records (default: stdout) |
| `--format` | `-f` | `string` | `json` | Export format: `json` or `csv` |
| `--since` | | `string` | `""` | Filter events created since duration or timestamp |
| `--until` | | `string` | `""` | Filter events created until duration or timestamp |
| `--event-type` | `-t` | `string` | `""` | Filter by audit event type |
| `--severity` | `-s` | `string` | `""` | Filter by severity |
| `--outcome` | | `string` | `""` | Filter by outcome |
| `--source` | | `string` | `""` | Filter by source IP address |
| `--actor` | | `string` | `""` | Filter by actor identity |
| `--limit` | `-l` | `int` | `1000` | Maximum number of events to export (default 1000, max 1000) |

#### Examples
```bash
# List all audit events from the last 24 hours
watchdog audit list

# Filter authentication failures and output JSON
watchdog audit list --event-type auth.failure --format json

# Export past 30 days of audit logs to JSON
watchdog audit export --since 30d --format json --output ./audit_export.json
```

---

### 9. `watchdog node`

Inspects local machine identity, persistent cryptographic UUID, hardware specifications, network interfaces, and operational tags.

```bash
watchdog node [flags]
```

#### Flags
| Flag | Shorthand | Type | Default | Description |
| :--- | :---: | :---: | :---: | :--- |
| `--short` | `-s` | `bool` | `false` | Output only the persistent node ID |
| `--json` | | `bool` | `false` | Output node identity in structured JSON format |
| `--node-id` | | `string` | `""` | Explicit node ID to use instead of auto-generated UUID |
| `--node-id-file` | | `string` | `""` | File path for persistent node ID storage (default: `~/.watchdog/node_id`) |
| `--tags` | | `[]string` | `nil` | Comma-separated `key=value` operational tags |

#### Examples
```bash
# Print formatted node identity summary
watchdog node

# Output node UUID only for shell scripting
NODE_ID=$(watchdog node --short)

# Output JSON identity payload
watchdog node --json
```

---

### 10. `watchdog fleet`
*Aliases*: `fleets`

Centralized fleet cluster management and observability querying against a remote fleet server.

```bash
watchdog fleet [command] [flags]
```

#### Global Fleet Flags
| Flag | Type | Default | Description |
| :--- | :---: | :---: | :--- |
| `--server` | `string` | `""` | Central fleet server URL (e.g. `https://fleet.internal:8443`) |
| `--token` | `string` | `""` | Authentication Bearer token |
| `--token-file` | `string` | `""` | File path containing authentication token |
| `--token-env` | `string` | `""` | Environment variable name containing token |
| `--insecure` | `bool` | `false` | Skip TLS certificate verification |
| `--timeout` | `duration` | `10s` | HTTP request timeout |
| `--json` | `bool` | `false` | Output results in structured JSON format |

#### Subcommands
- `watchdog fleet status`: Display aggregated cluster health counts, node breakdown, and average resource utilization.
- `watchdog fleet list`: List registered nodes with health status, CPU, memory, and last heartbeat.
  - `--status`: Filter by state (`healthy`, `warning`, `critical`, `stale`, `offline`, `unknown`)
  - `--search`: Search by node ID, hostname, or IP
  - `--since`: Filter nodes active since duration (e.g. `15m`, `2h`)
  - `--sort-by`: Sort field (`hostname`, `last_heartbeat`, `cpu`, `memory`, `status`)
  - `--sort-direction`: `asc` or `desc`
  - `--limit`, `--offset`: Pagination controls
- `watchdog fleet get <node_id>`: Inspect detailed hardware specs, operational metadata, active alerts, and recent telemetry history for a specific node.
- `watchdog fleet register`: Enrolls a node in the centralized fleet registry.
  - `--node-id`, `--hostname`, `--tags`, `--metadata`
- `watchdog fleet heartbeat`: Submits a manual liveness heartbeat ping for testing.
  - `--node-id`, `--status`
- `watchdog fleet deregister <node_id>`: Decommissions and removes a node from fleet tracking.
  - `--force`: Force removal without prompt

#### Examples
```bash
# View fleet cluster summary
watchdog fleet status --server https://fleet.internal:8443 --token <token>

# List nodes with warning or critical status
watchdog fleet list --status warning

# Inspect detailed node metadata and telemetry
watchdog fleet get c56a4180-65aa-42ec-a945-5fd21dec0538

# Deregister decommissioned node
watchdog fleet deregister worker-old-01 --force
```

---

### 11. `watchdog export`

Exports real-time snapshot data or historical time-series metric series from the local SQLite storage engine.

```bash
watchdog export [flags]
```

#### Flags
| Flag | Shorthand | Type | Default | Description |
| :--- | :---: | :---: | :---: | :--- |
| `--format` | `-f` | `string` | `json` | Export format: `json`, `csv` |
| `--metric` | `-m` | `string` | `""` | Specific time-series metric name (e.g. `cpu_usage_pct`, `memory_used_pct`) |
| `--output` | `-o` | `string` | `""` | File destination path (default stdout) |
| `--since` | `-s` | `duration` | `1h` | Historical lookback window |
| `--limit` | `-n` | `int` | `1000` | Maximum number of records to export |

---

### 12. `watchdog completion`

Generates autocompletion scripts for supported shells.

```bash
# Bash
source <(watchdog completion bash)

# Zsh
watchdog completion zsh > "${fpath[1]}/_watchdog"

# Fish
watchdog completion fish | source

# PowerShell
watchdog completion powershell | Out-String | Invoke-Expression
```

---

### 13. `watchdog version`

Displays detailed version and build metadata.

```bash
# Standard summary
watchdog version

# Machine-readable semantic version string
watchdog version --short

# Full structured JSON metadata
watchdog version --json
```
