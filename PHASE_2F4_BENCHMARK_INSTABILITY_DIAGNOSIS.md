# Phase 2F.2.4 Live-System Benchmark Variability Diagnosis

## Investigation timestamp

2026-11-23T14:45 UTC

## Purpose

Provide a technical accuracy review and evidence classification for the benchmark variance observed during the Phase 2F.2.4 live-system baseline (FullCycle range ~25.4%, Process GetProcesses range ~56.3%). The benchmark host was not isolated; these observations are descriptive and environment-dependent.

---

## 1. Evidence Classification and Analysis

### CPU frequency scaling

- **Configured & active:** Governor is `schedutil`; cores reported ~1995 MHz against a 2000 MHz cap and 2.10 GHz base frequency.
- **Evaluation:** The frequency difference is small. No measurements demonstrate that frequency scaling caused the observed benchmark variance.
- **Classification:** **Unverified** contributor to benchmark instability. Operating slightly below base frequency does not prove performance degradation or throttling.

### Memory and swap

- **Observed:** Approximately 1.2 GiB of swap was occupied, and available memory was ~2.4–3.4 GiB.
- **Evaluation:** Swap occupancy does not establish active paging (swap-in/swap-out activity or major page faults) during benchmark execution. Memory PSI and page-fault counters were not measured while benchmarks ran.
- **Classification:** **Unverified hypothesis**. Paging-related latency cannot be claimed as a proven cause without active paging measurements during benchmark runs.

### CPU contention

- **Load average interpretation:** Linux load average reflects the sum of runnable tasks and tasks in uninterruptible sleep (I/O wait), not direct CPU utilization percentage.
- **Timing distinction:** PSI and process snapshots were collected *after* benchmarking concluded and cannot be presented as direct measurements of benchmark-time contention.
- **Classification:** Post-benchmark host contention was **directly observed**, but its exact impact during benchmark execution remains a **plausible contributor** rather than a direct measurement.

### Disk and procfs

- **Virtual filesystem nature:** `/proc` is a virtual kernel filesystem backed by kernel memory and structures, not a physical rotational disk file read.
- **Evaluation:** Inspection latency cannot be automatically attributed to rotational HDD seek latency. Earlier system snapshots reported I/O wait, but disk I/O was not quantified during benchmark execution.
- **Classification:** Disk I/O impact is an **unverified hypothesis** / **measurement unavailable during benchmark execution**.

### Concurrent collector behavior

- **Verified code structure:** `Manager.CollectAll` executes enabled collectors concurrently via goroutines.
- **Evaluation:** While concurrent execution sensitivity and goroutine scheduling are structural characteristics of FullCycle measurements, there is no synchronized evidence linking specific isolated outliers (such as the 272.20 ms ProcessCollector sample) directly to specific FullCycle iterations.
- **Classification:** Concurrent execution is **verified code behavior**; its contribution to run-to-run variance is a **plausible contributor** (scheduling sensitivity).

---

## 2. Final Evidence Classification Table

| Category | Finding / Factor | Evidence Basis |
|---|---|---|
| **Verified code behavior** | `Manager.CollectAll` runs registered collectors concurrently via goroutines. | Code inspection of `internal/collector/collector.go`. |
| **Directly observed environment conditions** | CPU governor is `schedutil`, frequencies ~1995 MHz (max 2000 MHz, base 2.10 GHz); swap occupancy ~1.2 GiB; post-benchmark 1-min load average ~2.14 with active desktop/IDE processes. | System commands (`/proc/cpuinfo`, `/proc/meminfo`, `uptime`, `ps`) executed during environment audits. |
| **Plausible contributors** | Concurrent scheduling sensitivity in FullCycle measurements; post-benchmark CPU contention and memory pressure. | Architectural design of `Manager.CollectAll` combined with observed host state. |
| **Unverified hypotheses** | Direct impact of frequency scaling on benchmark variance; active swapping/paging during benchmark execution; rotational HDD seek latency for `/proc` virtual files; CPU thermal throttling. | Inferred from general OS behavior without targeted telemetry captured during benchmark execution. |
| **Measurements unavailable during benchmark execution** | Per-core CPU utilization during timed benchmarks; disk I/O metrics (`iostat`, `await`, queue depth) during benchmark execution; active swap-in/swap-out page fault counts; CPU temperature and thermal zone states; live process/socket count snapshots during benchmark runs. | Absence of instrumentation tools or historical logs at the exact execution timestamps. |

---

## Conclusion

The observed benchmark variance occurred on a live Linux host where collectors execute concurrently during FullCycle measurements. Concurrent scheduling and environmental conditions are plausible contributors, but the available evidence does not establish their causal impact on specific benchmark samples. Supporting benchmark output and preserved pprof text summaries are in [`docs/performance/phase-2f4/`](docs/performance/phase-2f4/). The original binary `.prof` files are unavailable, so the text summaries' binary source files cannot be independently verified.
