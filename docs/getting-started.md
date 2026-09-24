# Getting Started with Watchdog

Watchdog is a high-performance, cross-platform system observability, real-time diagnostic, and monitoring CLI written in pure Go. This guide will walk you through your first 5 minutes with Watchdog.

---

## ⚡ 30-Second Quickstart

### 1. Download or Build Watchdog
```bash
# Build locally with Go 1.22+
go build -ldflags="-s -w" -o watchdog .

# Or check version
./watchdog version
```

### 2. Launch the Interactive Dashboard
```bash
./watchdog dash
```
This launches the full-screen terminal user interface (TUI) showing real-time CPU gauges, memory usage, disk I/O, network throughput, process table, and active alerts. Press `q` to exit.

### 3. Run Automated System Diagnostics
```bash
./watchdog diagnose --fix
```
Watchdog inspects 10 system health rules (CPU saturation, memory paging, disk inode capacity, socket exhaustions, DNS latency, zombie processes, container health) and prints actionable remediation advice for any detected warning or critical condition.

---

## 🎯 Common Workflows

### Generating System Health Reports
Export a self-contained HTML5 diagnostic report with embedded dark theme and inline SVG trend charts:
```bash
watchdog report --format html --output /tmp/system-report.html --history 2h
```
Or output machine-readable JSON for integration into automation pipelines:
```bash
watchdog report --format json --output /tmp/report.json
```

### Running the Background Prometheus Exporter
Expose standard Prometheus metrics on port 9100:
```bash
watchdog server --port 9100 --host 127.0.0.1
```
Scrape metrics via HTTP:
```bash
curl http://127.0.0.1:9100/metrics
```

### Inspecting Alerts and Historical Metrics
```bash
# List all active threshold alerts
watchdog alert list

# Query historical alerts stored in SQLite
watchdog alert history

# Export historical CPU metrics over the past 4 hours as CSV
watchdog export --metric cpu_usage_pct --since 4h --format csv --output ./cpu_history.csv
```

---

## ⚙️ Initializing Configuration

Watchdog works out of the box with sensible defaults. To customize metric thresholds, collection intervals, or storage retention:

```bash
# Initialize default configuration file at ~/.watchdog/config.yaml
watchdog config init

# Validate configuration syntax
watchdog config validate

# View active resolved configuration
watchdog config show
```

---

## 🧭 Next Steps

- Explore the [Installation Guide](installation.md) for package manager installations (DEB, RPM, APK, Docker).
- Review the [CLI Reference](cli-reference.md) for full documentation of commands and flags.
- Learn about the [Interactive TUI & Keybindings](monitoring.md).
- Dive into the [Automated Diagnostics Engine](diagnostics.md) and [Rule Reference](rules.md).
- Integrate with Prometheus using the [API & Prometheus Guide](prometheus-api.md).
