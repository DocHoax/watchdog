# Standalone System Health Reporting

Watchdog generates self-contained system health reports in multiple formats (**HTML5**, **JSON**, **CSV**, **Terminal**), allowing operators to capture point-in-time diagnostic snapshots or analyze historical performance trends.

---

## 📄 Supported Output Formats

| Format | CLI Flag | Characteristics | Primary Use Case |
| :--- | :---: | :--- | :--- |
| **HTML5** | `-f html` | Self-contained, dark theme, inline SVG sparklines, zero CDN scripts | Human-readable executive & incident reports |
| **JSON** | `-f json` | Strict machine-readable schema | CI/CD pipelines, automated validation gates |
| **CSV** | `-f csv` | RFC 4180 compliant tabular metric records | Spreadsheet analysis & data warehouse imports |
| **Terminal**| `-f terminal`| Rich ANSI tables and progress gauges | Quick interactive summaries in terminal |

---

## 🌐 HTML5 Standalone Dashboard Reports

The HTML5 generator outputs a single, zero-dependency HTML file suitable for offline viewing, email attachments, and archiving:

```bash
# Generate standalone HTML report with 2 hours of trend charts
watchdog report --format html --output /var/reports/system-health.html --history 2h --title "Production DB Health"
```

### Key Features of HTML5 Reports
- **Zero External Dependencies**: All CSS styles and SVG sparkline charts are rendered inline without external CDN requests or JavaScript frameworks.
- **Embedded SVG Trend Charts**: Renders historical CPU, Memory, Disk I/O, and Network RX/TX trendlines directly as vector graphics.
- **Diagnostic Matrix**: Formatted heuristic check results with color-coded badges (`PASS`, `WARN`, `CRIT`) and actionable remediation commands.
- **Incident Triaging**: Highlights firing threshold alerts and statistical anomalies detected during the lookback window.
- **Raw JSON Embedding**: Pass `--raw-json` to embed the complete raw telemetry payload inside a `<script type="application/json">` tag for programmatic extraction.

---

## 🤖 Machine-Readable JSON Reports

Generate JSON payloads for automation:

```bash
watchdog report --format json --output /tmp/report.json
```

Sample JSON schema structure:
```json
{
  "title": "Watchdog System Health Report",
  "generated_at": "2026-09-24T12:00:00Z",
  "host": {
    "hostname": "prod-db-01",
    "os": "linux",
    "platform": "ubuntu",
    "kernel": "6.5.0-generic",
    "uptime_seconds": 1232800
  },
  "cpu": {
    "overall_usage": 42.5,
    "cores_count": 8,
    "load_average": { "load1": 1.24, "load5": 1.45, "load15": 1.10 }
  },
  "memory": {
    "total_bytes": 17179869184,
    "used_bytes": 9037496320,
    "used_percent": 52.6
  },
  "diagnostics": {
    "overall_status": "PASS",
    "total_checks": 35,
    "passed_checks": 35,
    "warning_checks": 0,
    "critical_checks": 0
  },
  "active_alerts": [],
  "anomalies": {
    "total_checked": 9,
    "anomalies_count": 0
  }
}
```

---

## 📊 CSV Export Format

Export tabular metric time-series:

```bash
watchdog report --format csv --output /tmp/metrics.csv --history 24h
```

Output format:
```csv
timestamp,hostname,metric,value
2026-09-24T10:00:00Z,prod-db-01,cpu_usage_pct,42.50
2026-09-24T10:00:00Z,prod-db-01,memory_used_pct,52.60
2026-09-24T10:00:00Z,prod-db-01,swap_used_pct,1.50
```

---

## 🖥️ Terminal Diagnostic Summary

Output a concise summary table directly into terminal standard output:

```bash
watchdog report --format terminal
```
