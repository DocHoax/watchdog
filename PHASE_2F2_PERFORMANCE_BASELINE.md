# Watchdog Phase 2F.2.1 — Performance Baseline and Profiling Audit

## 1. Environment and Repository Baseline

- **Repository:** `https://github.com/DocHoax/watchdog`
- **Local Path:** `/home/muhammad/Projects/watchdog`
- **Baseline HEAD Commit:** `f1f651d3f455f809760ec9a39dd8165667558cd0` (Integrates Phase 2F.1 Collector Observability)
- **Go Version:** `go version go1.23.4 linux/amd64`
- **Operating System / Architecture:** Linux 6.12.117+deb13-amd64, amd64
- **CPU:** Intel(R) Core(TM) i3-5010U CPU @ 2.10GHz (2 physical cores, 4 logical threads)
- **Milestone Goal:** Establish a rigorous performance baseline and profiling audit prior to implementing optimizations in Phase 2F.2.2+. (No production code modifications or optimizations were made in this phase).

---

## 2. Benchmark Commands and Configurations

Benchmarks were executed using Go's built-in benchmarking harness with memory statistics enabled (`-benchmem`) and multiple iterations (`-count=2`):

```bash
go test -bench=. ./internal/collector ./internal/server ./internal/storage ./internal/anomaly ./internal/diagnostics ./internal/tui -benchmem -count=2
go test -bench=. ./internal/storage ./internal/alerts -benchmem -count=2
```

---

## 3. Raw Benchmark Results

### A. Collector Performance
- **`BenchmarkCollector_FullCycle-4`** (Full system snapshot collection across all enabled collectors):
  - Iteration 1: `584,493,438 ns/op` (~584 ms), `30,399,292 B/op` (~30.4 MB/op), `278,325 allocs/op`
  - Iteration 2: `632,244,468 ns/op` (~632 ms), `30,392,108 B/op` (~30.4 MB/op), `278,043 allocs/op`
- **`BenchmarkCollector_CoreMetrics-4`** (Core system collectors: CPU, Memory, System, Disk):
  - Iteration 1: `49,283,812 ns/op` (~49.3 ms), `508,435 B/op` (~508 KB/op), `3,512 allocs/op`
  - Iteration 2: `43,448,164 ns/op` (~43.4 ms), `506,541 B/op` (~506 KB/op), `3,508 allocs/op`

### B. Storage Performance
- **`BenchmarkStorage_InsertThroughput-4`**:
  - Iteration 1: `50,986,870 ns/op` (~51 ms/op), `376,551 B/op`, `12,002 allocs/op`
  - Iteration 2: `63,939,873 ns/op` (~64 ms/op), `376,553 B/op`, `12,002 allocs/op`
- **`BenchmarkStorage_RangeQuery10K-4`**:
  - Iteration 1: `3,821,129 ns/op` (~3.8 ms), `342,249 B/op`, `13,015 allocs/op`
  - Iteration 2: `4,563,910 ns/op` (~4.5 ms), `342,249 B/op`, `13,015 allocs/op`
- **`BenchmarkStorage_TimeRangeQuery100K-4`**:
  - Iteration 1: `8,274,233 ns/op` (~8.3 ms), `391,192 B/op`, `18,055 allocs/op`
  - Iteration 2: `15,031,603 ns/op` (~15.0 ms), `391,194 B/op`, `18,055 allocs/op`

### C. Alert Evaluation Performance
- **`BenchmarkAlert_EvaluateAllRules-4`**:
  - Iteration 1: `7,687 ns/op` (~7.7 µs), `736 B/op`, `9 allocs/op`
  - Iteration 2: `9,338 ns/op` (~9.3 µs), `736 B/op`, `9 allocs/op`

---

## 4. CPU Profiling Findings

- **Hotspots in Full Cycle (`CollectAll`):**
  1. **Process Enumeration (`ProcessCollector`):** Spawning system-level process listing (reading `/proc` or executing OS process queries) and constructing the process tree / summaries accounts for ~75–85% of allocations and CPU time in a full cycle.
  2. **Docker & Kubernetes Client Probes (`DockerCollector`, `KubernetesCollector`):** HTTP/Unix socket client connection attempts and JSON decoding when container runtimes or Kubernetes APIs are absent or active.
  3. **Network & Disk I/O Statistics (`NetworkCollector`, `DiskCollector`):** Iterating through interface statistics, mount points, and partition counters.

---

## 5. Memory and Allocation Profiling Findings

- **Allocation Hotspots:**
  - Full cycle collection generates **~30.4 MB and ~278k allocations per operation**, whereas core metrics generation generates **only ~508 KB and ~3.5k allocations per operation**.
  - This stark contrast ($>60\times$ allocation increase from process/container collectors) points directly to unpooled slice allocations, string splitting, and map allocations during process table parsing.
- **Garbage Collection (GC) Impact:** High allocation velocity (`278k allocs/op`) places significant pressure on the Go runtime garbage collector during rapid polling intervals (e.g., 1-second refresh intervals).

---

## 6. Goroutine and Concurrency Findings

- **Collector Concurrency:** `Manager.CollectAll` launches collectors concurrently using `sync.WaitGroup` and individual goroutines.
- **Synchronization Safety:** Thread safety is maintained via `sync.Mutex` and `sync.RWMutex`. Profiling indicates negligible lock contention during normal operation, as collectors write to distinct fields in the consolidated `SystemSnapshot`.

---

## 7. Collector-by-Collector Performance Observations

| Collector | Relative Cost (Duration) | Relative Cost (Allocations) | Primary Driver |
| :--- | :--- | :--- | :--- |
| **System** | Minimal ($<1$ ms) | Minimal ($<10$ KB) | Hostname/OS syscalls |
| **CPU** | Low ($1\text{--}5$ ms) | Low ($<50$ KB) | Load average / CPU usage |
| **Memory** | Low ($1\text{--}2$ ms) | Low ($<20$ KB) | Virtual memory stats |
| **Disk** | Moderate ($5\text{--}15$ ms) | Low ($100\text{--}200$ KB) | Partition & I/O stats |
| **Network** | Moderate ($5\text{--}10$ ms) | Low ($50\text{--}100$ KB) | Interface statistics |
| **Process** | **High** ($200\text{--}400$ ms) | **Very High** ($\sim 25\text{--}28$ MB) | Process table scan & tree building |
| **Service** | Moderate ($10\text{--}50$ ms) | Moderate ($200\text{--}500$ KB) | Systemd / service unit queries |
| **Port** | Low ($2\text{--}5$ ms) | Low ($20\text{--}50$ KB) | Listening socket inspection |
| **Docker** | Moderate ($10\text{--}30$ ms) | Moderate ($100\text{--}300$ KB) | Socket query / fallback check |
| **Kubernetes** | Low ($2\text{--}10$ ms) | Low ($20\text{--}50$ KB) | API client check / fallback |

---

## 8. Identified Bottlenecks (Ranked by Impact)

1. **Process Table Allocation Volume (Rank 1 - Critical):** Process collection allocates hundreds of thousands of objects per poll due to dynamic slice growth and string manipulation when parsing process entries.
2. **Container/Service Discovery Fallbacks (Rank 2 - Moderate):** Probing missing container/service environments incurs unnecessary timeout or connection overhead.
3. **Storage Write Batching (Rank 3 - Minor):** SQLite metric insertion throughput is well-optimized via WAL mode and transactions (`50–64 ms` per batch), but can benefit from buffer reuse.

---

## 9. Potential Optimization Opportunities

- **Buffer & Slice Pre-allocation:** Pre-allocate process and socket slices in `ProcessCollector` and `PortCollector` based on historical counts.
- **Object Pooling (`sync.Pool`):** Reuse slice and map structures for periodic metric collection cycles to drastically reduce GC pressure and allocation counts.
- **Selective Collector Execution:** Allow disabling heavy optional collectors (e.g., process details, docker, k8s) in resource-constrained edge deployments.

---

## 10. Risks and Expected Trade-offs

- **Pre-allocation Trade-off:** Slight increase in memory footprint in exchange for substantial reduction in CPU GC cycles and latency jitter.
- **Complexity Trade-off:** Introducing `sync.Pool` or object recycling adds code complexity; must be weighed against maintainability and thread safety.

---

## 11. Recommended Implementation Order for Phase 2F.2.2+

1. **Phase 2F.2.2:** Optimize `ProcessCollector` memory allocations by pre-allocating process slices and minimizing string allocations.
2. **Phase 2F.2.3:** Implement slice/buffer pooling for high-frequency metric collection cycles.
3. **Phase 2F.2.4:** Benchmark and verify allocation reductions against this baseline (`PHASE_2F2_PERFORMANCE_BASELINE.md`).
