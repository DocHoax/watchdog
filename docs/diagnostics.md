# Automated System Diagnostics Engine

Watchdog features an automated heuristic diagnostic engine designed to detect infrastructure degradations, resource saturation, container crashloops, network anomalies, and zombie processes across Windows, Linux, and macOS.

---

## ⚡ Overview & Architecture

When running `watchdog diagnose` or viewing Tab 6 in the TUI, the diagnostic engine executes registered heuristic health rules concurrently against a freshly sampled `model.SystemSnapshot`.

```
                    ┌─────────────────────────┐
                    │   System Collector      │
                    │   (Snapshot Generator)  │
                    └────────────┬────────────┘
                                 │
                     SystemSnapshot Payload
                                 │
                    ┌────────────▼────────────┐
                    │   Diagnostic Engine     │
                    └────────────┬────────────┘
         ┌───────────────┬───────┴───────┬───────────────┐
         ▼               ▼               ▼               ▼
   [ CPU Rules ]  [ Memory Rules ] [ Disk Rules ] [ Network Rules ]
         │               │               │               │
   Goroutine 1     Goroutine 2     Goroutine 3     Goroutine 4
         └───────────────┬───────────────┴───────────────┘
                         │
                 Aggregated Report
                         │
        ┌────────────────┼────────────────┐
        ▼                ▼                ▼
  Terminal Table     JSON Payload    HTML Dashboard
```

---

## 🚦 Rule Statuses & Severity Levels

Every rule evaluation generates a `DiagnosticResult` with standardized statuses:

| Status Badge | Severity | Exit Code Impact | Description |
| :---: | :---: | :---: | :--- |
| `PASS` | `INFO` | `0` | Subsystem operating within nominal thresholds |
| `WARN` | `WARNING` | `0` (or `1` if strict) | Approaching resource capacity; non-fatal degradation |
| `CRIT` / `FAIL` | `CRITICAL` | `1` | Immediate critical issue, data loss risk, or outage condition |
| `SKIP` | `INFO` | `0` | Collector disabled or subsystem not applicable (e.g. Inodes on NTFS) |

---

## 📋 Standard Diagnostic Rule Categories

### 1. CPU Rules
- **CPU Utilization (`cpu-utilization`)**:
  - *Trigger*: Warn if overall CPU >= 75%, Crit if >= 90% (customizable via `alerts.cpu.threshold`).
  - *Remediation*: Identify top CPU-consuming tasks using `watchdog dash` or sort by CPU (`c`).
- **CPU Load Average (`cpu-load-average`)**:
  - *Trigger*: Warn if 1-minute load exceeds 1.5x logical core count; Crit if > 2.5x logical core count.
  - *Remediation*: Check I/O wait times, lock contention, or thread pool exhaustion.

### 2. Memory & Swap Rules
- **RAM Utilization (`memory-utilization`)**:
  - *Trigger*: Warn if used RAM >= 80%, Crit if >= 92% (configurable in `config.yaml`).
  - *Remediation*: Inspect high-memory processes to prevent Linux OOM-killer or Windows memory page thrashing.
- **Swap Utilization (`swap-utilization`)**:
  - *Trigger*: Warn if Swap > 50%, Crit if > 80%.
  - *Remediation*: Free physical RAM or expand swap space.

### 3. Disk & Storage Rules
- **Partition Capacity (`disk-space`)**:
  - *Trigger*: Warn if partition utilization >= 80%, Crit if >= 90%.
  - *Remediation*: Rotate log files, purge temporary files, or expand disk volume.
- **Inode Availability (`disk-inodes`)**:
  - *Trigger*: Warn if Inodes >= 85%, Crit if >= 95% on Unix filesystems.
  - *Remediation*: Remove directories containing millions of zero-byte temporary files.

### 4. Network & Connectivity Rules
- **DNS Resolution (`dns-resolution`)**:
  - *Trigger*: Warn if lookup latency > 500ms; Crit if lookup fails completely.
  - *Remediation*: Inspect local `/etc/resolv.conf` or Windows DNS adapters and fallback to secondary resolvers.
- **Internet Connectivity (`network-connectivity`)**:
  - *Trigger*: Tests TCP connectivity to reliable endpoints (`1.1.1.1:53`, `8.8.8.8:53`). Crit if unreachable.
  - *Remediation*: Verify default gateway route, firewall rules, and interface link states.

### 5. Process & Container Rules
- **Process Health (`process-health`)**:
  - *Trigger*: Warn if zombie process count > 10 or rogue process CPU >= 90%.
  - *Remediation*: Inspect parent process reap handlers or terminate defunct child processes.
- **Docker Health (`docker-health`)**:
  - *Trigger*: Crit if any container has >= 5 consecutive restart failures.
  - *Remediation*: Inspect container logs (`docker logs <id>`) to diagnose startup crashes.

---

## 🛠️ CLI Usage & Practical Workflows

### 1. Basic Heuristic Check
```bash
watchdog diagnose
```

### 2. Display Actionable Remediation Instructions (`--fix`)
```bash
watchdog diagnose --fix
```

Output example:
```
┌──────────────────────┬─────────┬───────────────────────────┬───────────────────────────────────────────────┐
│ CATEGORY / RULE      │ STATUS  │ METRIC VALUE              │ REMEDIATION ADVICE                            │
├──────────────────────┼─────────┼───────────────────────────┼───────────────────────────────────────────────┤
│ CPU Utilization      │ PASS    │ 12.4%                     │ Nominal                                       │
│ RAM Utilization      │ WARN    │ 84.2% (13.4GB / 16.0GB)   │ Inspect high-memory processes with `watchdog` │
│ Disk Partition       │ CRIT    │ /data (94.1%)             │ Purge old logs in /var/log or expand disk     │
│ DNS Resolution       │ PASS    │ 18 ms                     │ Nominal                                       │
│ Process Health       │ PASS    │ 184 procs (0 zombies)     │ Nominal                                       │
└──────────────────────┴─────────┴───────────────────────────┴───────────────────────────────────────────────┘
```

### 3. Filter by Specific Subsystem (`--category`)
```bash
watchdog diagnose --category Memory --fix
watchdog diagnose --category Disk
```

### 4. Machine-Readable Automation Output
Export structured JSON for CI/CD pipelines or monitoring orchestrators:
```bash
watchdog diagnose --json --output /tmp/diagnostic-results.json
```
