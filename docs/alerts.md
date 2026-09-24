# Threshold Alerting & Hysteresis Engine

Watchdog features an embedded, stateful threshold alerting engine designed to detect infrastructure degradations while preventing alert fatigue through temporal evaluation windows and cooldown hysteresis.

---

## ⚡ Alert Engine Architecture

The alert engine operates in a continuous lifecycle:

```
    ┌─────────────────────────┐
    │   Sampled Snapshot      │
    └────────────┬────────────┘
                 │
    ┌────────────▼────────────┐
    │  Rule Evaluator         │  Evaluates: CPU, Memory, Disk, Process, Network
    │  (Builtin & Custom)     │
    └────────────┬────────────┘
                 │ EvaluatedAlert stream
    ┌────────────▼────────────┐
    │  StateTracker           │  - Duration window verification (e.g. sustained for 30s)
    │  (Hysteresis & Cooldown)│  - Suppression cooldown (e.g. 5m silence after firing)
    └────────────┬────────────┘
                 │
       ┌─────────┴─────────┐
       ▼                   ▼
[ Fired Alert ]     [ Resolved Alert ]
       │                   │
  ┌────┴───────────────────┴────┐
  │  SQLite Persistent Storage  │
  └─────────────────────────────┘
```

---

## ⚙️ Built-in Alert Rules & Configuration

Alert parameters are configured under the `alerts:` section of `config.yaml`:

```yaml
alerts:
  cpu:
    enabled: true
    threshold: 90.0                  # Trigger when aggregate CPU >= 90%
    duration: 30s                    # Must be sustained for 30 seconds
    cooldown: 5m                     # Suppress duplicate alerts for 5 minutes
  memory:
    enabled: true
    threshold: 85.0                  # Trigger when RAM used >= 85%
    duration: 30s
    cooldown: 5m
  disk:
    enabled: true
    threshold: 90.0                  # Trigger when any partition >= 90%
    duration: 1m
    cooldown: 15m
  process:
    enabled: true
    cpu_threshold: 80.0              # Single-process CPU threshold
    memory_threshold: 70.0           # Single-process RAM threshold
    cooldown: 5m
  network:
    enabled: true
    threshold: 100.0                 # Cumulative dropped/error packet threshold
    duration: 1m
    cooldown: 10m
```

---

## ⏳ Temporal Windows & Hysteresis Cooldowns

To eliminate noisy alerts caused by transient bursts (e.g., short compilation spikes), Watchdog enforces two temporal guards:

1. **Trigger Duration Window (`duration`)**:
   - A threshold breach must persist continuously for the specified duration before transitioning to `Active`.
   - If utilization falls below the threshold before `duration` expires, the timer resets without firing.

2. **Suppression Cooldown (`cooldown`)**:
   - Once an alert fires, subsequent duplicate alerts for the same rule are suppressed for the cooldown duration.
   - Prevents notification spam during oscillating load conditions.

3. **Automatic Resolution**:
   - When metric values return below threshold, the state tracker marks the active alert as resolved, records `ResolvedAt`, and emits a resolution event.

---

## 🛠️ Alert CLI Commands

### 1. List Currently Firing Alerts
```bash
watchdog alert list
```
Displays all active alerts, severity (`WARNING`, `CRITICAL`), trigger duration, and current metric value.

### 2. Query Historical Alert Records
```bash
watchdog alert history
# Query with custom limit
watchdog alert history --limit 100
```
Queries historical alert events persisted in the embedded SQLite database.

### 3. Send Synthetic Test Alert
```bash
watchdog alert test
```
Emits a test alert across notification channels to verify alerting pipelines and terminal alert badges.
