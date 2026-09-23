# Diagnostic & Alert Rule Reference

Watchdog includes a built-in diagnostic rule engine that runs automated health checks and root-cause analysis across key system subsystems.

---

## Diagnostic Rules Catalog

| Rule ID | Subsystem | Severity | Description | Remediation Advice |
| :--- | :--- | :--- | :--- | :--- |
| `CPU_HIGH_LOAD` | CPU | CRITICAL / WARNING | Overall CPU utilization exceeding 90% (CRITICAL) or 80% (WARNING) | Identify top CPU-consuming processes using `watchdog top` or `ps aux --sort=-%cpu`. Consider process reprioritization with `renice` or horizontal scaling. |
| `MEM_SATURATION` | Memory | CRITICAL / WARNING | Available system memory < 10% (CRITICAL) or < 20% (WARNING) | Check for memory leaks in long-running services. Inspect top memory consumers and review swap thrashing. |
| `SWAP_THRASH` | Memory | WARNING | Swap utilization > 50% with sustained active paging | System is under heavy memory pressure. Increase physical RAM or tune `vm.swappiness`. |
| `DISK_SPACE_CRIT` | Disk | CRITICAL / WARNING | Root or data mount filesystem utilization > 95% (CRITICAL) or > 85% (WARNING) | Clean up stale log files (`/var/log`), prune container caches (`docker system prune`), or expand disk volumes. |
| `DISK_IO_STALL` | Disk | WARNING | High disk queue latency and read/write I/O saturation | Identify I/O heavy processes using `iotop` or `watchdog diagnose --verbose`. Review storage IOPS limits. |
| `NET_PACKET_LOSS` | Network | WARNING | Interface error or drop rate > 1% of total transmitted packets | Check network cabling, interface MTU configuration, or upstream router/switch buffer congestion. |
| `PROC_ZOMBIE_LEAK` | Process | WARNING | Presence of defunct/zombie processes > 5 | Parent processes are failing to reap child processes with `waitpid()`. Inspect parent PID and restart service. |
| `DNS_LATENCY_SLOW`| Network | WARNING | DNS resolution latency exceeding 250ms | Verify local `/etc/resolv.conf` nameservers, local DNS cache service (systemd-resolved/dnsmasq), and upstream latency. |
| `DOCKER_CRASH_LOOP`| Docker | CRITICAL / WARNING | One or more containers restarting repeatedly or exited with non-zero status | Inspect container logs via `docker logs <container_id>`. Verify health check endpoints and resource limits. |
| `K8S_POD_UNHEALTHY`| Kubernetes | CRITICAL / WARNING | Pods in `CrashLoopBackOff`, `ImagePullBackOff`, or `Pending` state | Check pod events via `kubectl describe pod <pod_name>` and examine container logs. |

---

## Statistical Anomaly Detection Rules

The anomaly detector uses a sliding window of rolling observations to evaluate incoming metrics against normal baselines:
- **Z-Score Formula**: $Z = \frac{x - \mu}{\sigma}$
- **EWMA Smoothing**: $\mu_t = \alpha \cdot x_t + (1 - \alpha) \cdot \mu_{t-1}$
- **Trigger Condition**: When $|Z| \ge \text{threshold}$ (default: 2.5) sustained across consecutive evaluation cycles.
- **Suppression Window**: Cooldown periods suppress duplicate anomaly notifications for the same metric stream within a 5-minute hysteresis window.
