# Embedded Time-Series Storage & History

Watchdog embeds a zero-CGO, pure-Go SQLite storage engine powered by `modernc.org/sqlite` to persist local metrics, diagnostic health records, and alert histories without external database dependencies.

---

## 💾 Storage Engine Architecture

```
                    ┌─────────────────────────┐
                    │ System Metric Collector │
                    └────────────┬────────────┘
                                 │
                     Persist Snapshot (10s)
                                 │
                    ┌────────────▼────────────┐
                    │ SQLite Storage Engine   │
                    │   (modernc.org/sqlite)  │
                    │   journal_mode = WAL    │
                    └────────────┬────────────┘
         ┌───────────────────────┼───────────────────────┐
         ▼                       ▼                       ▼
┌──────────────────┐   ┌───────────────────┐   ┌────────────────────┐
│ metrics_history  │   │  alerts_history   │   │diagnostics_history │
│ (Time-series)    │   │  (Alert Events)   │   │(Rule Evaluations)  │
└──────────────────┘   └───────────────────┘   └────────────────────┘
         │
         ▼
┌───────────────────────────────────────────────────────────────────┐
│ Automatic Pruning Worker (Prunes samples older than retention_days│
└───────────────────────────────────────────────────────────────────┘
```

---

## ⚡ High-Performance WAL Configuration

Watchdog tunes SQLite for concurrent metric ingestion and low query latency:

- **Write-Ahead Logging (WAL)**: `_pragma=journal_mode(WAL)` enables concurrent non-blocking reads while metrics are written.
- **Synchronous Normal**: `_pragma=synchronous(NORMAL)` ensures data integrity while eliminating synchronous disk flush bottlenecks.
- **Busy Timeout**: `_pragma=busy_timeout(5000)` prevents lock contention failures under heavy I/O loads.
- **Connection Pool**: Default pool size of 10 open/idle connections managed within memory limits.

---

## 📊 Database Schema Design

### 1. `metrics_history` Table
Stores raw time-series metric samples:
```sql
CREATE TABLE IF NOT EXISTS metrics_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp INTEGER NOT NULL,
    metric TEXT NOT NULL,
    value REAL NOT NULL,
    hostname TEXT,
    tags_json TEXT
);
CREATE INDEX IF NOT EXISTS idx_metrics_metric_time ON metrics_history(metric, timestamp);
CREATE INDEX IF NOT EXISTS idx_metrics_time ON metrics_history(timestamp);
```

### 2. `alerts_history` Table
Records all fired and resolved alert lifecycle events.

### 3. `diagnostics_history` Table
Maintains periodic snapshots of heuristic rule evaluations for trend analysis.

---

## ⚙️ Configuration & Retention Policies

Configure storage parameters in `config.yaml`:

```yaml
storage:
  enabled: true
  db_path: "~/.watchdog/watchdog.db" # Database file path on disk
  retention_days: 7                 # Automatically prune metrics older than 7 days
  collection_interval: 10s           # Persistence frequency
```

### Automatic Pruning
A background maintenance task automatically prunes metrics exceeding `retention_days`. Resolved alerts and historical diagnostics older than `2 * retention_days` are cleaned periodically to keep disk usage compact.

---

## 📤 Exporting Historical Telemetry

Use `watchdog export` to dump time-series metrics from SQLite:

```bash
# Export historical CPU metrics over the past 2 hours as CSV
watchdog export --metric cpu_usage_pct --since 2h --format csv --output ./cpu_2h.csv

# Export memory utilization as JSON
watchdog export --metric memory_used_pct --since 24h --format json --output ./mem_24h.json
```
