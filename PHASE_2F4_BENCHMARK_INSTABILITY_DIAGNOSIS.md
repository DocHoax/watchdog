# Phase 2F.2.4 Benchmark Instability Diagnosis

## Investigation timestamp

2026-11-23T14:45 UTC

## Purpose

Diagnose the causes of benchmark instability observed during Phase 2F.2.4, particularly the FullCycle variance of approximately 25.4% and the Process GetProcesses variance of approximately 56.3% (driven by one 272.20 ms outlier).

## Measured evidence at diagnosis time

### CPU frequency scaling

- **Current CPU frequencies:** 1995.47, 1995.47, 1995.49, 1995.45 MHz (all four logical CPUs)
- **Maximum CPU frequencies:** 2000.00 MHz (all four logical CPUs)
- **CPU governor:** `schedutil`
- **Intel i3-5010U base frequency:** 2.10 GHz

**Verdict:** CPU frequency scaling is **verified active**. All cores are running approximately 0.2% below their configured maximum (1995 MHz vs. 2000 MHz cap), and materially below the CPU's rated base frequency of 2.10 GHz. The `schedutil` governor dynamically adjusts frequency in response to workload. This introduces timing variation across benchmark runs depending on when the governor scales frequency up or down.

### CPU thermal state

**Historical evidence:** None captured during the investigation.

**Limitation:** No `sensors` or thermal zone readings were collected before, during, or after benchmarking. CPU temperature, thermal throttling events, and thermal governor state were not measured.

**Verdict:** Thermal throttling is **unverified**. The i3-5010U is a 15W TDP mobile part that can throttle under sustained load, but no evidence was captured to confirm or refute throttling during the benchmark runs.

### Per-core CPU utilization

**Historical evidence:** None captured during the investigation.

**Current snapshot:**
- Load average: 2.14, 1.67, 1.78 (1-min, 5-min, 15-min)
- Top CPU consumers: Chrome (15.8%), code (11.0%), gnome-control-center (10.1%), chrome (10.0%), gnome-shell (8.3%), claude (8.2%), omniroute (5.1%)
- PSI CPU pressure: `some avg10=5.12 avg60=5.39 avg300=4.51` (percentage of time some tasks were stalled on CPU)

**Verdict:** Substantial unrelated CPU contention is **verified active**. The 1-minute load average of 2.14 on a 4-core system indicates ~54% average utilization. PSI metrics show that some tasks have been stalled on CPU for approximately 5% of recent intervals. Chrome, VS Code, GNOME Shell, and control-center processes were consuming material CPU during the diagnosis snapshot.

**Limitation:** Per-core CPU utilization (e.g., via `mpstat` or per-CPU `/proc/stat` samples) was not captured during the actual benchmark runs, so we cannot verify whether specific cores were saturated or whether benchmark iterations landed on contested cores.

### Disk activity and I/O wait

**Historical evidence during benchmarking:** Not quantified. An earlier system snapshot mentioned 30.8% I/O wait, but `iostat` was unavailable.

**Current snapshot:** No direct disk I/O measurement tool was available at diagnosis time.

**Storage type:** Rotational HDD (`/dev/sda`, `ROTA=1`).

**Verdict:** I/O wait was **observed but not quantified** during the investigation. Rotational HDD latency is inherently variable and sensitive to seek patterns, concurrent I/O, and filesystem cache state. The benchmark workloads perform substantial `/proc` filesystem inspection, which can be disk-backed depending on kernel memory pressure.

**Limitation:** No `iostat`, `iotop`, or per-device I/O counters were collected during benchmark execution. Disk queue depth, service times, and await metrics are unavailable.

### Memory pressure and swap

**Historical evidence during benchmarking:**
- Available memory: approximately 3.3–3.4 GiB
- Swap used: approximately 1.17 GiB (steady)

**Current snapshot:**
- Available memory: 2.4 GiB
- Swap used: 1.2 GiB

**Verdict:** Memory pressure is **verified present**. Approximately 1.2 GiB of swap is in use, indicating historical memory pressure. Available memory has declined from 3.3 GiB to 2.4 GiB between readiness check and diagnosis time. Swap-backed pages being accessed during benchmarks would introduce latency spikes.

### Live process and socket count variability

**Historical evidence:** Not quantified during benchmarking.

**Current snapshot:**
- Running processes: 269
- Listening TCP/UDP sockets: 0 (measurement may be incomplete; `netstat` and `ss` both returned zero)

**Verdict:** Process count variability is **unverified but plausible**. The benchmark workloads inspect live host state; process creation/termination and socket open/close events during a benchmark run would directly affect the work performed. With 269 processes and active desktop/browser workloads, process churn is expected but was not quantified.

### Concurrent collector execution

**Verified from code inspection:** `Manager.CollectAll` launches all enabled collectors concurrently via goroutines and waits for all to complete. The FullCycle benchmark measures wall-clock time for the entire concurrent collection.

**Implication:** The FullCycle wall-clock measurement is sensitive to:
1. The slowest collector in each iteration (Amdahl's law for parallel work).
2. Goroutine scheduling variance.
3. Concurrent resource contention (CPU, memory, disk, `/proc` filesystem).
4. Live host state differences between iterations.

**Verdict:** Concurrent execution is **verified** and is a structural source of variance. If the process collector encounters a 272 ms outlier due to host interference (as observed in sample 5 of the Process benchmark), and that collector is the longest-running in a FullCycle iteration, the entire FullCycle measurement will reflect that outlier.

## Summary of verified causes

1. **CPU frequency scaling (verified):** `schedutil` governor is active and CPUs are running below base frequency.
2. **Unrelated CPU contention (verified):** Chrome, VS Code, GNOME Shell, and other desktop processes consumed material CPU during diagnosis; PSI metrics confirm CPU stalling.
3. **Memory pressure (verified):** 1.2 GiB swap usage indicates memory pressure; swap-backed access introduces latency variance.
4. **I/O wait (observed, not quantified):** Rotational HDD with observed I/O wait in earlier snapshots; no detailed I/O metrics captured.
5. **Concurrent collector execution (verified structural):** Parallel goroutine execution makes FullCycle sensitive to the slowest collector and scheduling variance.

## Hypotheses without direct evidence

1. **CPU thermal throttling:** Plausible for a 15W mobile CPU under sustained load, but no thermal measurements were captured.
2. **Per-core saturation:** Plausible given load average and active processes, but per-core utilization was not captured during benchmarking.
3. **Live process/socket churn:** Plausible given 269 running processes and desktop workloads, but churn was not quantified.
4. **Filesystem cache thrashing:** Plausible given memory pressure and rotational storage, but cache hit/miss rates were not measured.

## Limitations

- No CPU temperature or thermal zone data.
- No per-core CPU utilization during benchmark execution.
- No disk I/O metrics (`iostat`, `iotop`, await, queue depth).
- No process/socket count samples during benchmark runs.
- No filesystem cache statistics.
- No scheduler or context-switch metrics.
- Current diagnostic snapshot was taken approximately 14 minutes after benchmark completion; host state may have changed.

## Conclusion

The observed benchmark variance is **multifactorial**. Verified contributors include CPU frequency scaling, unrelated CPU contention, memory pressure with swap usage, and the structural sensitivity of concurrent collector execution to the slowest component and scheduling variance. I/O wait on rotational storage was observed but not quantified. Additional hypotheses (thermal throttling, per-core saturation, live state churn) are plausible but lack direct supporting evidence.

A stable, repeatable benchmark requires:
- Idle or near-idle host (close unrelated workloads).
- CPU governor set to `performance` mode (disable frequency scaling).
- Sufficient available memory to avoid swap activity.
- Dedicated benchmark machine or scheduler isolation.
- Consistent live host state (or synthetic/mocked data sources).

The measurements collected are valid as a live-system baseline under the documented host conditions, but the variance limits their use for precise optimization comparison without substantially tighter control.
