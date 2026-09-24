# Watchdog Subsystem Performance Benchmarks

This document records the empirical benchmark measurements for all core subsystems of Watchdog, profiling operational throughput, latency, heap memory footprint, and garbage collection allocation overhead.

---

## 1. Test Environment & Hardware Specification

The performance benchmarks were executed natively on the following test machine:

| Component | Specification |
|:---|:---|
| **CPU** | Intel(R) Core(TM) i7-7600U CPU @ 2.80GHz (2 Cores, 4 Logical Processors) |
| **RAM** | 8.00 GB DDR4 |
| **Operating System** | Microsoft Windows 11 Enterprise (NT 10.0.22000.0) |
| **Architecture** | `windows/amd64` |
| **Go Runtime** | Go `go1.27.1 windows/amd64` |
| **Storage Backend** | SSD NVMe (SQLite WAL mode with memory cache enabled) |

---

## 2. Performance Summary & SLA Budget Matrix

| Subsystem / Benchmark Target | Target Budget (SLA) | Measured Performance | Latency (ns/op) | Memory (B/op) | Allocs/op | Status |
|:---|:---|:---|:---|:---|:---|:---:|
| **Collector Core Metrics** | < 500 allocs/op | **466 allocs/op** | 110,923,450 ns | 134,711 B | 466 | **PASS** |
| **Collector Full Cycle** | < 10.0s cycle | **4.87s / cycle** | 4,875,632,300 ns | 21,229,360 B | 16,478 | **PASS** |
| **Diagnostic Evaluation** | < 1.0s for 10 rules | **381.3 ms** | 381,347,967 ns | 12,330 B | 150 | **PASS** |
| **Anomaly Stream (100 metrics)** | < 1.0 ms | **70.4 µs** (~704 ns/metric) | 70,439 ns | 5,605 B | 199 | **PASS** |
| **Anomaly Snapshot Evaluation** | < 100 µs | **10.5 µs** (95,547 ops/sec) | 10,466 ns | 5,025 B | 27 | **PASS** |
| **Alert Rule Evaluation** | < 50 µs | **1.98 µs** (505,561 ops/sec) | 1,978 ns | 736 B | 9 | **PASS** |
| **Storage Metric Ingestion** | > 10,000 metrics/s | **76,426 metrics/s** | 13,084,477 ns/1k | 383,094 B | 12,133 | **PASS** |
| **Storage 1-Hour Query (10K)** | < 50.0 ms | **1.93 ms** | 1,931,899 ns | 296,343 B | 7,970 | **PASS** |
| **Storage Time Query (100K)** | < 50.0 ms | **2.43 ms** | 2,432,193 ns | 335,414 B | 11,051 | **PASS** |
| **TUI Dashboard Render Frame** | < 16.6 ms (60 FPS) | **1.80 ms (557 FPS)** | 1,795,448 ns | 192,047 B | 2,733 | **PASS** |
| **TUI Process Table Render** | < 16.6 ms (60 FPS) | **0.55 ms (1,812 FPS)** | 551,732 ns | 96,790 B | 1,716 | **PASS** |
| **HTTP `/api/v1/snapshot`** | > 1,000 req/s | **69,974 req/s** | 14,291 ns | 2,481 B | 9 | **PASS** |
| **HTTP `/metrics` Prometheus** | > 1,000 req/s | **49,463 req/s** | 20,217 ns | 23,629 B | 62 | **PASS** |

---

## 3. Subsystem In-Depth Benchmark Results

### 3.1 Collector Cycle Overhead (`internal/collector`)

```
BenchmarkCollector_FullCycle-4           1    4875632300 ns/op    21229360 B/op    16478 allocs/op
BenchmarkCollector_CoreMetrics-4        10     110923450 ns/op      134711 B/op      466 allocs/op
```

- **Analysis**:
  - `BenchmarkCollector_CoreMetrics` queries CPU, Memory, Disk, and Network interfaces with zero external service enumeration. It completes in ~110 ms with **466 heap allocations**, meeting the target requirement (< 500 allocs/op).
  - `BenchmarkCollector_FullCycle` includes full Windows Management Instrumentation (WMI) service discovery, listening port enumeration, full process tree traversal with command-line parsing, and Docker API probing. Even with full WMI overhead, the complete collection cycle finishes in ~4.87s, well under default polling intervals (5s–10s).

---

### 3.2 Diagnostic Engine Rule Evaluation (`internal/diagnostics`)

```
BenchmarkDiagnostics_EngineEvaluation-4   3     381347967 ns/op       12330 B/op      150 allocs/op
```

- **Analysis**:
  - The diagnostic engine concurrently evaluates 10 built-in diagnostic rules across CPU, memory, disk, network connectivity, DNS resolution, system services, and container health.
  - Evaluation latency is dominated by real-world network probes (DNS lookup and ICMP/TCP ping checks), completing in **381.3 ms** across all 10 checks with only 150 allocations and 12.3 KB of memory consumed.

---

### 3.3 Statistical Anomaly Detection (`internal/anomaly`)

```
BenchmarkAnomaly_Feed100Metrics-4    15226         70439 ns/op        5605 B/op      199 allocs/op
BenchmarkAnomaly_FeedSnapshot-4     109294         10466 ns/op        5025 B/op       27 allocs/op
```

- **Analysis**:
  - `Feed100Metrics`: Feeding 100 unique real-time metric streams through the online rolling statistics window (Welford's algorithm + Exponentially Weighted Moving Average) takes **70.4 µs** total, translating to **~704 ns per metric stream**.
  - `FeedSnapshot`: Ingesting a full comprehensive system snapshot takes **10.5 µs** (**95,547 snapshots/sec**) with just 27 allocations.

---

### 3.4 Alert Rule Evaluation (`internal/alerts`)

```
BenchmarkAlert_EvaluateAllRules-4   548026          1978 ns/op         736 B/op        9 allocs/op
```

- **Analysis**:
  - Evaluating all configured alert rules with cooldown timers, sustained duration thresholds, and state transitions completes in **1.98 µs** per evaluation (**505,561 evaluations/sec**) with only 9 allocations and 736 B of memory.

---

### 3.5 Database Storage Throughput & Query Latency (`internal/storage`)

```
BenchmarkStorage_InsertThroughput-4     82      13084477 ns/op      383094 B/op    12133 allocs/op
BenchmarkStorage_RangeQuery10K-4       663       1931899 ns/op      296343 B/op     7970 allocs/op
BenchmarkStorage_TimeRangeQuery100K-4  456       2432193 ns/op      335414 B/op    11051 allocs/op
```

- **Analysis**:
  - **Batch Write Throughput**: Inserting a batch of 1,000 metrics takes **13.08 ms** in SQLite WAL mode, achieving **~76,426 metrics/second** write throughput.
  - **10K Rows Range Query**: A 1-hour time-range query over a 10,000-row table completes in **1.93 ms** (budget: < 50 ms).
  - **100K Rows Range Query**: A 5,000-record range query over a 100,000-row table completes in **2.43 ms**, proving B-Tree composite index efficiency (`(metric, timestamp)`).

---

### 3.6 Terminal User Interface (TUI) Render Budget (`internal/tui`)

```
BenchmarkTUI_DashboardRender-4         654       1795448 ns/op      192047 B/op     2733 allocs/op
BenchmarkTUI_ProcessesRender-4        1962        551732 ns/op       96790 B/op     1716 allocs/op
```

- **Analysis**:
  - Target 60 FPS frame time budget: **16.6 ms (16,666,666 ns)**.
  - `BenchmarkTUI_DashboardRender`: Measures full Lipgloss styling, ANSI progress bars, sparkline generation, and dual-column grid composition. Render time is **1.80 ms** (**557 FPS**), utilizing only **10.8% of the 60 FPS frame budget**.
  - `BenchmarkTUI_ProcessesRender`: Formats 100 process records with sorting, filtering, and table formatting in **0.55 ms** (**1,812 FPS**), using only **3.3% of the frame budget**.

---

### 3.7 HTTP REST Server & Prometheus Exposition (`internal/server`)

```
BenchmarkServer_SnapshotEndpoint-4   79915         14291 ns/op        2481 B/op        9 allocs/op
BenchmarkServer_PrometheusEndpoint-4 56602         20217 ns/op       23629 B/op       62 allocs/op
```

- **Analysis**:
  - `/api/v1/snapshot`: Authenticated JSON snapshot serialization serves **69,974 requests/sec** with 14.3 µs latency and 9 allocations per request.
  - `/metrics`: Prometheus text exposition formatting serves **49,463 scrapes/sec** with 20.2 µs latency.
