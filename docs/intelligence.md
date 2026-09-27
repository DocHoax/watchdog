# Watchdog Phase 2A: Fleet Intelligence & Correlated Health Analysis

Watchdog v1.0.0 incorporates the **Watchdog Intelligence Layer** (`internal/intelligence`). The Intelligence Layer elevates Watchdog from point-in-time telemetry sampling and isolated threshold alerting to a holistic, explainable analytical platform capable of discovering temporal patterns, calculating statistical baselines, scoring node and fleet health, and clustering co-occurring degradation events across an entire infrastructure fleet.

---

## 1. Architectural Invariants & Scope Boundaries

The Watchdog Intelligence Layer adheres to strict operational boundaries:

1. **Strictly Observational & Analytical (Zero Remediation)**:
   - **No Command Execution**: Does not invoke shells, execute binaries, or launch sub-processes.
   - **No State Mutation**: Does not restart systemd services, terminate processes, modify configurations, adjust sysctl values, or alter firewall/routing tables.
   - **Non-Causal Language**: Outputs express statistical relationships and temporal associations (*"temporally associated"*, *"co-occurring"*, *"correlated across N nodes"*, *"exhibits degradation trend"*), deliberately avoiding unverified assertions of root causation.

2. **Deterministic & Explainable (Zero Black-Box Magic)**:
   - **Explainable Health Scores**: Every 0.0–100.0 health score is accompanied by an itemized list of factor contributions, point deductions, weights, and human-readable explanations.
   - **Bounded Mathematical Modeling**: Trend rates-of-change and statistical baselines use exact closed-form linear regression slopes and well-defined sample percentiles ($P_{50}, P_{90}, P_{95}, P_{99}$).
   - **Grounded Evidence**: Every finding references concrete data points (`supporting_evidence`), confidence tiers (`high`, `medium`, `low`), and detected timestamps.

3. **Strict Structural Read-Only Guarantees**:
   - Consumes only read-only interfaces (`ReadOnlyFleetService`, `ReadOnlyStorage`, `anomaly.Detector`).
   - MCP tools are explicitly registered in `AllowedReadOperations` allowlists; any mutation attempt is rejected with JSON-RPC error code `-32601`.

4. **Resource & Memory Bounded**:
   - Zero-CGO pure Go architecture with zero runtime dependencies.
   - Fleet-wide evaluation over 100 nodes executes in $< 500\text{ms}$ with $< 50\text{MB}$ peak heap allocation.

---

## 2. Health Scoring Engine

The Health Scoring Engine (`internal/intelligence/scoring.go`) calculates a normalized **Health Score** ($0.0 \le \text{Score} \le 100.0$) for each node based on telemetry snapshots, system metrics, active alerts, failed diagnostics, and detected statistical anomalies.

### Deduction Categories and Penalty Weights

Starting from a base score of `100.0`, deductions are computed across five operational subsystems:

| Subsystem Category | Max Deduction | Evaluation Criteria |
| :--- | :--- | :--- |
| **CPU Subsystem** | 25.0 pts | Usage $> 85\%$ (-15 pts), Usage $> 95\%$ (-25 pts); Load Avg / Core $> 1.5\times$ (-10 pts), $> 3.0\times$ (-20 pts); Core usage skew $> 40\%$ (-5 pts) |
| **Memory & Swap** | 25.0 pts | RAM used $> 85\%$ (-10 pts), $> 95\%$ (-20 pts); Swap used $> 50\%$ (-10 pts), $> 80\%$ (-20 pts); Available buffer $< 500\text{MB}$ (-5 pts) |
| **Storage & Inodes** | 20.0 pts | Disk space used $> 85\%$ (-10 pts), $> 95\%$ (-20 pts); Inode usage $> 85\%$ (-10 pts), $> 95\%$ (-20 pts) |
| **Alerts & Diagnostics** | 30.0 pts | Critical active alert (-15 pts/each), Warning active alert (-5 pts/each); Failed diagnostic check (-10 pts/each), Warning check (-3 pts/each) |
| **Statistical Anomalies**| 15.0 pts | Detected Z-score or EWMA metric anomaly (-5 pts/each) |

### Authoritative Status Capping

To prevent contradictions between node status and numerical health scores, authoritative status caps are enforced:

- **`NodeStatusCritical`**: Score is strictly capped at `49.0` ($\le 49.0$).
- **`NodeStatusWarning`**: Score is strictly capped at `79.0` ($\le 79.0$).
- **`NodeStatusOffline` / `NodeStatusStale`**: Score is set to `0.0` with an explicit factor deduction record.

### Score Trajectory Classification

The Trajectory Classifier (`internal/intelligence/trajectory.go`) compares the current health score against historical evaluations over sliding time windows (15m, 1h):

- **`improving`**: $\Delta \text{Score} > +5.0$
- **`degrading`**: $\Delta \text{Score} < -5.0$
- **`stable`**: $-5.0 \le \Delta \text{Score} \le +5.0$
- **`volatile`**: High score variance ($> 15.0$) with alternating directional shifts.
- **`unknown`**: Insufficient historical evaluation samples.

---

## 3. Metric Trends & Statistical Baselines

### Linear Slope Trend Detection (`trends.go`)

Calculates rate-of-change across time-series metric points using closed-form ordinary least squares linear regression:

$$m = \frac{N \sum_{i=1}^N (t_i \cdot v_i) - \left(\sum_{i=1}^N t_i\right) \left(\sum_{i=1}^N v_i\right)}{N \sum_{i=1}^N t_i^2 - \left(\sum_{i=1}^N t_i\right)^2}$$

- **Rate of Change**: Expressed per minute (e.g. `+2.45 %/min` or `-12.3 MB/min`).
- **Direction**: `increasing` ($m > \epsilon$), `decreasing` ($m < -\epsilon$), `stable` ($|m| \le \epsilon$), or `insufficient_data` ($N < 3$).
- **Confidence**: Normalized coefficient of determination ($R^2$) indicating regression fit.

### Historical Baselines (`baselines.go`)

Computes statistical distribution benchmarks over historical telemetry windows (1h, 6h, 24h, 7d):

- **Sample Count ($N$)**, **Minimum**, **Maximum**.
- **Sample Mean ($\mu$)**: $\mu = \frac{1}{N}\sum v_i$
- **Standard Deviation ($\sigma$)**: $\sigma = \sqrt{\frac{1}{N}\sum (v_i - \mu)^2}$
- **Percentiles ($P_{50}, P_{90}, P_{95}, P_{99}$)**: Computed using linear interpolation between nearest ranks on sorted sample subsets.

---

## 4. Cross-Signal Correlation & Incident Clustering

### Temporal Pearson Correlation (`correlation.go`)

Discovers co-occurring metric and event dynamics across time by aligning time-series samples into discrete temporal buckets (default: 60s) and computing the Pearson correlation coefficient ($r$):

$$r = \frac{\sum (x_i - \bar{x})(y_i - \bar{y})}{\sqrt{\sum (x_i - \bar{x})^2 \sum (y_i - \bar{y})^2}}$$

- Evaluates signal pairs (e.g., `cpu_usage_pct` $\leftrightarrow$ `network_tx_bytes`, `memory_used_pct` $\leftrightarrow$ `swap_used_pct`).
- Assigns confidence tiers (`high` for $|r| \ge 0.85$ with $N \ge 30$, `medium` for $|r| \ge 0.70$, `low` otherwise).
- Emits non-causal relationship descriptions.

### Incident Clustering & Timeline Builder (`incidents.go`)

Aggregates overlapping active alerts, diagnostic failures, and anomaly detections affecting a node or fleet cluster into coherent `Incident` entities:

- **Unified Title & Primary Symptoms**: Synthesized from constituent alert rules and failing components.
- **Incident Timeline (`IncidentTimelineEvent`)**: Chronological event sequence linking initial detection, severity escalations, and subsequent alert events.
- **Affected Nodes**: List of all fleet nodes exhibiting correlated symptoms.

### Fleet Pattern Analyzer (`fleet.go`)

Monitors fleet-wide telemetry to detect distributed operational degradation:

- **Simultaneous Resource Pressure**: Identifies concurrent CPU or memory spikes occurring on $\ge 3$ nodes within a 5-minute window.
- **Correlated Diagnostic Failures**: Identifies identical diagnostic test failures (e.g. DNS resolution timeout, systemd service failure) spanning multiple cluster members.
- **Fleet Anomaly Clusters**: Detects concurrent statistical anomalies triggered across independent hosts.

---

## 5. REST API Endpoints

The Watchdog server provides authenticated REST endpoints under `/api/v1/intelligence/*`:

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/v1/intelligence/fleet` | Fleet health summary, average score, lowest scoring nodes, active incidents, fleet findings |
| `GET` | `/api/v1/intelligence/nodes/{id}` | Detailed node health score, factor breakdown, primary concerns, active incidents, and findings |
| `GET` | `/api/v1/intelligence/nodes/{id}/trends?window=1h` | Time-series trend analysis and rate-of-change across node metrics |
| `GET` | `/api/v1/intelligence/nodes/{id}/baselines?window=24h` | Historical statistical baselines (Min, Max, Mean, StdDev, $P_{50}, P_{90}, P_{95}, P_{99}$) |
| `GET` | `/api/v1/intelligence/incidents` | List all open and active fleet incidents |
| `GET` | `/api/v1/intelligence/incidents/{id}` | Inspect a single incident with full chronological timeline and symptom breakdown |
| `GET` | `/api/v1/intelligence/correlations?window=1h` | List signal pairs exhibiting high temporal correlation |
| `GET` | `/api/v1/intelligence/findings?category=...&min_severity=...` | Query structured intelligence findings with evidence and suggestions |

---

## 6. Command-Line Interface (`watchdog intelligence`)

The CLI provides human-readable, JSON, and YAML views into the Intelligence Layer:

```bash
# Display fleet intelligence summary
watchdog intelligence fleet --server-url https://watchdog.internal:8443 --token-file /etc/watchdog/token

# Inspect health breakdown for a specific node
watchdog intelligence node worker-prod-01

# List metric trends on a node over the last 1 hour
watchdog intelligence trends worker-prod-01 --window 1h

# View statistical baselines over the last 24 hours
watchdog intelligence baselines worker-prod-01 --window 24h

# List active fleet incidents
watchdog intelligence incidents

# Inspect detailed incident timeline
watchdog intelligence incidents inc-9a8f1b2c

# View cross-signal temporal correlations
watchdog intelligence correlations --window 1h

# Filter intelligence findings by severity
watchdog intelligence findings --min-severity warning --format json
```

---

## 7. Model Context Protocol (MCP) Integration

The Watchdog MCP server (`internal/mcp/`) exposes read-only intelligence capabilities to AI agents:

### Intelligence Tools
- `get_fleet_intelligence`: Retrieve fleet health score, degraded nodes, and fleet-wide findings.
- `get_node_intelligence`: Retrieve explainable health score, factor breakdown, and active concerns for a node.
- `get_fleet_incidents`: List active incidents and affected node clusters.
- `get_intelligence_findings`: Filter findings by category and minimum severity.
- `get_node_trends`: Retrieve metric rate-of-change and regression trajectory for a node.

### Intelligence Resources
- `intelligence://fleet/summary`: Direct read-only URI for the fleet health summary.
- `intelligence://incidents/active`: Direct read-only URI for active incidents.
- `intelligence://nodes/{node_id}/summary`: Direct read-only URI for individual node health summaries.

### Intelligence Prompt Templates
- `analyze_fleet_health`: Guides an AI agent through assessing fleet stability and prioritizing degraded nodes.
- `investigate_incident`: Directs AI investigation of a specific incident ID with timeline and evidence.
- `triage_node_degradation`: Guides AI triage of a node exhibiting score degradation or anomalous trends.

---

## 8. Prometheus Metrics

The Intelligence Layer exports operational telemetry to Prometheus via `/metrics`:

```text
# HELP watchdog_intelligence_fleet_health_score Current overall fleet health score (0-100)
# TYPE watchdog_intelligence_fleet_health_score gauge
watchdog_intelligence_fleet_health_score 88.5

# HELP watchdog_intelligence_node_health_score Current health score for a node (0-100)
# TYPE watchdog_intelligence_node_health_score gauge
watchdog_intelligence_node_health_score{node_id="node-01",hostname="worker-01"} 92.0
watchdog_intelligence_node_health_score{node_id="node-02",hostname="worker-02"} 64.0

# HELP watchdog_intelligence_active_incidents_total Number of currently open incidents
# TYPE watchdog_intelligence_active_incidents_total gauge
watchdog_intelligence_active_incidents_total{severity="critical"} 0
watchdog_intelligence_active_incidents_total{severity="warning"} 1

# HELP watchdog_intelligence_findings_total Number of active intelligence findings
# TYPE watchdog_intelligence_findings_total gauge
watchdog_intelligence_findings_total{category="fleet_pattern",severity="warning"} 2
watchdog_intelligence_findings_total{category="resource_exhaustion",severity="critical"} 0
```
