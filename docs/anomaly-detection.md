# Statistical Anomaly Detection Engine

Watchdog features an online statistical anomaly detection engine implemented in pure Go. It detects anomalous system metrics (CPU spikes, memory leaks, I/O bursts, connection floods) in real-time with zero external machine learning dependencies, minimal CPU overhead, and bounded memory consumption.

---

## 📐 Mathematical Foundations

Rather than relying on heavy AI or neural network models, Watchdog leverages established statistical signal processing techniques:

```
                      Raw Metric Sample (x_t)
                                 │
                 ┌───────────────┴───────────────┐
                 ▼                               ▼
       [ Rolling Ring Buffer ]         [ Online EWMA Filter ]
        Window Size: N (e.g. 60)        Smoothing Factor: α (0.2)
                 │                               │
                 ▼                               ▼
        Rolling Baseline Mean (μ)         Smoothed Trend (S_t)
        Rolling StdDev (σ)
                 │
                 ▼
       ┌───────────────────────────────┐
       │   Z-Score Evaluation Engine   │
       │     Z = (x_t - μ) / σ         │
       └───────────────┬───────────────┘
                       │
       ┌───────────────┴───────────────┐
       ▼                               ▼
  |Z| < Threshold                 |Z| >= Threshold
  (Normal Fluctuation)            (Statistical Anomaly)
                                       │
                                 ┌─────┴─────┐
                                 ▼           ▼
                              Spike        Drop
```

### 1. Rolling Ring Buffer
Maintains a fixed-size ring buffer ($N=60$ samples by default) with $O(1)$ sample additions. Computes rolling baseline mean ($\mu$) and sample standard deviation ($\sigma$).

### 2. Exponentially Weighted Moving Average (EWMA)
Smooths short-term fluctuations while tracking baseline drift:
$$S_t = \alpha \cdot x_t + (1 - \alpha) \cdot S_{t-1}$$
Where $\alpha \in (0.0, 1.0]$ is the smoothing factor (default `0.2`).

### 3. Rolling Z-Score Metric
Quantifies how many standard deviations a newly observed data point deviates from the recent baseline:
$$Z = \frac{x_t - \mu}{\sigma}$$

---

## ⚙️ Configuration Parameters

Configure anomaly detection settings in `config.yaml`:

```yaml
anomaly:
  enabled: true
  z_score_threshold: 2.5             # Trigger anomaly when |Z| >= 2.5 standard deviations
  window_size: 60                    # Number of samples in rolling window
  alpha: 0.2                         # EWMA smoothing factor (0.0 < alpha <= 1.0)
```

---

## 🔍 Metric Streams & Scoring

The anomaly engine continuously evaluates key system metrics:

| Metric Stream | Description | Anomaly Pattern |
| :--- | :--- | :--- |
| `cpu_usage_pct` | Aggregate CPU utilization % | Sudden compute bursts or spinlocks |
| `cpu_load_ratio` | 1-minute load average per CPU core | Run queue saturation |
| `memory_used_pct` | Physical RAM consumption % | Memory leaks or rapid buffer allocations |
| `swap_used_pct` | Swap space consumption % | Memory exhaustion and page thrashing |
| `disk_read_rate` | Read throughput (Bytes/sec) | Heavy sequential scans or cache misses |
| `disk_write_rate` | Write throughput (Bytes/sec) | Write bursts, database flushes, logging loops |
| `net_rx_rate` | Inbound network throughput | Traffic surges or DDoS attempts |
| `net_tx_rate` | Outbound network throughput | Data exfiltration or high outbound streaming |
| `process_count` | Total active process count | Fork bombs or process spawning cascades |

---

## 🚦 Severity & Explanation

When $|Z| \ge \text{z\_score\_threshold}$, Watchdog classifies the event:

- **Warning**: $|Z| \ge \text{threshold}$ (e.g. $Z \ge 2.5$)
- **Critical**: $|Z| \ge \text{threshold} + 2.0$ (e.g. $Z \ge 4.5$)
- **Direction**: Flagged as `Spike` ($x_t > \mu$) or `Drop` ($x_t < \mu$).
- **Explanation**: Generates human-readable context, e.g.:
  `"Spike detected: 94.20 (baseline mean: 22.40 ± 6.10, Z: +11.77, dev: +320.5%)"`
