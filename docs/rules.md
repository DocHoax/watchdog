# Diagnostic & Alert Rule Reference

Watchdog includes a built-in diagnostic rule engine that runs automated health checks and root-cause analysis across key system subsystems.

---

## Diagnostic Rules Catalog

| Rule ID | Subsystem | Severity | Description | Remediation Advice |
| :--- | :--- | :--- | :--- | :--- |
| `cpu-utilization` | CPU | CRITICAL / WARNING | Overall CPU utilization exceeding 90% (CRITICAL) or 75% (WARNING) | Identify top CPU-consuming processes using `watchdog dash` or sort by CPU (`c`). Scale or terminate rogue tasks. |
| `cpu-load-average` | CPU | CRITICAL / WARNING | System load average exceeding 2.5x logical cores (CRITICAL) or 1.5x logical cores (WARNING) | Check I/O wait times, lock contention, or thread pool exhaustion. |
| `memory-utilization` | Memory | CRITICAL / WARNING | RAM utilization exceeding 92% (CRITICAL) or 80% (WARNING) | Inspect high-memory processes with `watchdog top --sort memory` or increase host RAM. |
| `swap-utilization` | Memory | CRITICAL / WARNING | Swap utilization exceeding 80% (CRITICAL) or 50% (WARNING) | Free physical RAM or resize swap space to prevent lockups. |
| `disk-space` | Disk | CRITICAL / WARNING | Partition capacity exceeding 90% (CRITICAL) or 80% (WARNING) | Clean up stale log files (`/var/log`), prune temporary files, or expand disk volume. |
| `disk-inodes` | Disk | CRITICAL / WARNING | Filesystem inode usage exceeding 95% (CRITICAL) or 85% (WARNING) | Delete large directories of small files, temp sessions, or orphaned caches. |
| `dns-resolution` | Network | CRITICAL / WARNING | DNS resolution failed (CRITICAL) or latency > 500ms (WARNING) | Verify local `/etc/resolv.conf` or Windows DNS adapters and configure fallback nameservers. |
| `network-connectivity` | Network | CRITICAL | Outbound TCP connectivity to reliable public endpoints unreachable | Verify network interface connection, default gateway route, and firewall rules. |
| `process-health` | Process | WARNING | Rogue high-CPU processes (>=90%) or excessive zombie processes (>10) | Inspect parent processes failing to wait() on child exits or terminate runaway tasks. |
| `docker-health` | Docker | CRITICAL | One or more containers with >= 5 restart failures | Inspect container logs with `docker logs <container>` to diagnose startup failure. |

---

## Statistical Anomaly Detection Rules

The anomaly detector uses a sliding window of rolling observations to evaluate incoming metrics against normal baselines:
- **Z-Score Formula**: $Z = \frac{x - \mu}{\sigma}$
- **EWMA Smoothing**: $\mu_t = \alpha \cdot x_t + (1 - \alpha) \cdot \mu_{t-1}$
- **Trigger Condition**: When $|Z| \ge \text{threshold}$ (default: 2.5) sustained across consecutive evaluation cycles.
- **Suppression Window**: Cooldown periods suppress duplicate anomaly notifications for the same metric stream within a 5-minute hysteresis window.
