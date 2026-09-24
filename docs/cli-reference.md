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
| `--remote` | `-r` | `string` | `""` | Remote agent host address (`host:port`) |
| `--token` | `-T` | `string` | `""` | Authentication token for remote agent connection |
| `--insecure`| `-k` | `bool` | `false` | Skip TLS certificate verification for remote connections |

#### Examples
```bash
# Launch dashboard with 500ms refresh rate
watchdog dash --interval 500ms

# Launch with Nord theme
watchdog dash --theme nord

# Connect to remote agent over TLS
watchdog dash --remote 192.168.1.50:8443 --token s3cretTok3n
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
| `--port` | `-p` | `int` | `9100` | HTTP listening port |
| `--host` | `-H` | `string` | `127.0.0.1` | Network interface to bind (use `0.0.0.0` for all interfaces) |
| `--token` | `-T` | `string` | `""` | Required Bearer token for REST API endpoints |
| `--tls-cert`| | `string` | `""` | Path to TLS certificate PEM file |
| `--tls-key` | | `string` | `""` | Path to TLS private key PEM file |

#### Examples
```bash
# Launch Prometheus exporter on port 9100 (localhost only)
watchdog server --port 9100

# Launch server on all interfaces with TLS and Bearer authentication
watchdog server --host 0.0.0.0 --port 9100 --token mySecretToken --tls-cert /etc/ssl/cert.pem --tls-key /etc/ssl/key.pem
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
| `--interval` | `-i` | `duration` | `10s` | Sampling and storage persistence interval |
| `--port` | `-p` | `int` | `8443` | Remote agent API listener port |
| `--token` | `-T` | `string` | `""` | Bearer token required for remote client connections |

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

### 8. `watchdog export`

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

### 9. `watchdog completion`

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

### 10. `watchdog version`

Displays detailed version and build metadata.

```bash
# Standard summary
watchdog version

# Machine-readable semantic version string
watchdog version --short

# Full structured JSON metadata
watchdog version --json
```
