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

Inspects, queries, exports, and manages security audit logs stored in the local SQLite database.

```bash
watchdog audit [command] [flags]
```

#### Subcommands
- `watchdog audit list [flags]`: List and filter recorded security audit events (table, JSON, or CSV).
- `watchdog audit export [flags]`: Export audit logs to JSON or CSV file with automated formula injection protection.
- `watchdog audit purge [flags]`: Permanently remove historical audit records older than a retention cutoff.

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
| `--limit` | `-l` | `int` | `100` | Maximum number of events to return |
| `--offset` | | `int` | `0` | Pagination offset |
| `--json` | | `bool` | `false` | Output results in structured JSON |
| `--csv` | | `bool` | `false` | Output results in CSV format |

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
| `--limit` | `-l` | `int` | `1000` | Maximum number of events to export |

#### Flags (`audit purge`)
| Flag | Shorthand | Type | Default | Description |
| :--- | :---: | :---: | :---: | :--- |
| `--retention-days` | | `int` | `0` | Purge records older than N days |
| `--older-than` | | `string` | `""` | Purge records older than duration or timestamp (e.g. `90d`, `720h`) |
| `--force` | `-f` | `bool` | `false` | Confirm purge execution without interactive prompt |

#### Examples
```bash
# List all audit events from the last 24 hours
watchdog audit list

# Filter authentication failures and output JSON
watchdog audit list --event-type auth.failure --json

# Export past 30 days of audit logs to CSV
watchdog audit export --since 30d --format csv --output ./audit_export.csv

# Purge audit logs older than 90 days
watchdog audit purge --retention-days 90 --force
```

---

### 9. `watchdog export`

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

### 10. `watchdog completion`

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

### 11. `watchdog version`

Displays detailed version and build metadata.

```bash
# Standard summary
watchdog version

# Machine-readable semantic version string
watchdog version --short

# Full structured JSON metadata
watchdog version --json
```
