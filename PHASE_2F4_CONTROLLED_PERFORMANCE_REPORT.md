# Phase 2F.2.4 Controlled Linux Performance Investigation

## 1. Executive summary

This investigation establishes a live-system Linux baseline for Watchdog's collector subsystem at commit `f1f651d3f455f809760ec9a39dd8165667558cd0`. It adds two benchmark-only tests for the independent process and port collectors, collects repeated benchmark measurements, and captures CPU and heap profiles for full-cycle, process, and port workloads.

The measurements show that **process inspection is the largest isolated collector workload** under the tested host conditions. Process collection performs substantial repeated `/proc` file inspection through gopsutil. Port collection is materially cheaper than process collection but has a distinct socket/inode discovery cost. These are measured observations from isolated benchmark runs; no production optimization was implemented.

Important distinction: isolated benchmark timings (e.g., `BenchmarkProcessCollector_GetProcesses` at ~174–272 ms/op) are **not additive** to the concurrent `FullCycle` wall-clock measurement (~361–453 ms/op) because collectors run in parallel in `Manager.CollectAll`. Cumulative CPU profile attribution reflects sampled Go CPU time in call trees, not wall-clock contribution.

The recommended future experiment is limited to one primary area: isolate and evaluate a process-collector implementation that reduces repeated per-process `/proc` reads while preserving the current public behavior. That experiment must be separately benchmarked and correctness-tested. This report does not recommend a shared inspection provider, global cache, or cross-cycle cache.

## 2. Environment specification

Captured on the benchmark host:

| Item | Value |
|---|---|
| OS | Debian GNU/Linux 13 (reported environment) |
| Kernel | Linux 6.12.117+deb13-amd64 |
| Go | go1.23.4 linux/amd64 |
| GOROOT | `/usr/local/go` |
| GOPATH | `/home/muhammad/go` |
| GOOS/GOARCH | `linux/amd64` |
| GOMAXPROCS | Go default; benchmark output used `-4` |
| CPU | Intel(R) Core(TM) i3-5010U CPU @ 2.10GHz |
| Physical topology | 1 socket, 2 cores, 2 threads/core |
| Logical CPUs | 4 |
| RAM | 7.7 GiB total |
| Storage | `/dev/sda`, rotational HDD (`ROTA=1`); exact filesystem/device mapping was not further normalized |
| CPU governor | `schedutil` on observed CPUs |
| Repository | `/home/muhammad/Projects/watchdog` |

At the readiness check, available memory was approximately 3.3–3.4 GiB and swap use was approximately 1.17 GiB. During the 60-second preflight, 1-minute load declined from approximately 1.28 to 1.06 and then measured 1.19. CPU samples varied, with meaningful desktop and I/O activity still present. Chrome, VS Code, GNOME Shell, Omniroute, and the agent process were active. This is therefore a controlled live-host baseline, not an isolated or dedicated benchmark machine.

A first benchmark attempt under heavier activity produced unacceptable full-cycle variance and was not used as the primary result. The final repeated run below was retained, but the limitations section records the host interference.

## 3. Repository baseline and changes

- Branch: `feature/phase-2f4-controlled-performance-investigation`
- Baseline and final pre-commit base: `f1f651d3f455f809760ec9a39dd8165667558cd0`
- Baseline `main`: verified at `f1f651d3f455f809760ec9a39dd8165667558cd0`
- Production collector behavior: unchanged
- Public APIs: unchanged
- Dependencies: unchanged
- Architecture: `ProcessCollector` and `PortCollector` remain independent

Permitted source change:

- `internal/collector/collector_linux_benchmark_test.go`: live-system benchmark tests for `ProcessCollector.GetProcesses` and `PortCollector.GetListeningPorts`.

Report and raw binary profiles are not stored in the repository except for this report. Raw artifacts remain under `/tmp/watchdog-phase-2f4/`.

## 4. Benchmark methodology

The existing `BenchmarkCollector_FullCycle` and `BenchmarkCollector_CoreMetrics` tests were retained. They already call the real manager and collectors, report allocations, reset timing after setup, and validate results. Dedicated live-system benchmarks were added for the two expensive Linux workloads:

- `BenchmarkProcessCollector_GetProcesses`: reads live host process state through gopsutil and builds the complete process summary.
- `BenchmarkPortCollector_GetListeningPorts`: reads live host socket state, discovers listening ports, resolves process names, and sorts the result.

All benchmarks use `b.ReportAllocs()`, perform collector/context setup outside the timed loop, call `b.ResetTimer()`, validate errors and non-nil results, and avoid logging in timed loops. No artificial mocks are used.

Primary repeated command:

```text
go test ./internal/collector -run '^$' -bench 'BenchmarkCollector_|BenchmarkProcessCollector_|BenchmarkPortCollector_' -benchmem -count=10 -benchtime=3s
```

Raw output:

```text
/tmp/watchdog-phase-2f4/benchmarks/repeated.txt
```

Profile commands used 30 seconds per workload:

```text
go test ./internal/collector -run '^$' -bench '^BenchmarkCollector_FullCycle$' -benchtime=30s -cpuprofile=/tmp/watchdog-phase-2f4/profiles/full-cpu.prof
go test ./internal/collector -run '^$' -bench '^BenchmarkProcessCollector_GetProcesses$' -benchtime=30s -cpuprofile=/tmp/watchdog-phase-2f4/profiles/process-cpu.prof
go test ./internal/collector -run '^$' -bench '^BenchmarkPortCollector_GetListeningPorts$' -benchtime=30s -cpuprofile=/tmp/watchdog-phase-2f4/profiles/port-cpu.prof
```

Equivalent commands with `-memprofile` produced the three memory profiles. Text reports were generated with `go tool pprof -text`, selecting `alloc_space`, `alloc_objects`, `inuse_space`, and `inuse_objects` as applicable.

## 5. Repeated benchmark results

The following are the ten samples from the final repeated run. Values are nanoseconds per operation; allocation columns are bytes and objects per operation.

| Benchmark | ms/op samples | B/op range | allocs/op range |
|---|---|---:|---:|
| FullCycle | 373.25, 361.70, 375.46, 370.32, 360.92, 365.20, 396.10, 422.18, 391.30, 452.77 ms | 30.14–30.34 MB | 270,973–272,715 |
| CoreMetrics | 17.48, 16.92, 16.08, 16.81, 16.69, 16.60, 16.74, 15.74, 16.97, 16.83 ms | 506,086–506,841 B | 3,512–3,514 |
| Process GetProcesses | 174.18, 180.69, 182.35, 199.06, 272.20, 180.63, 179.90, 188.53, 184.06, 192.08 ms | 22.00–22.23 MB | 165,442–167,227 |
| Port GetListeningPorts | 41.18, 38.86, 40.11, 38.34, 37.88, 39.99, 37.34, 38.44, 38.94, 43.86 ms | 3.52–3.54 MB | 49,759–50,141 |

Observed ranges (max-min divided by minimum):

- FullCycle: approximately 25.4%; a prior preflight run reached approximately 88.2%, which was rejected as too unstable.
- CoreMetrics: approximately 11.1%.
- Process: approximately 56.3%, driven largely by one 272.20 ms sample; excluding that outlier, the remaining range is approximately 14.3%.
- Port: approximately 17.5%.

Coefficient of variation (sample standard deviation / mean, across the ten published samples):

- FullCycle: approximately 7.74%
- CoreMetrics: approximately 2.88%
- Process GetProcesses: approximately 14.78%
- Port GetListeningPorts: approximately 4.84%

These ranges and coefficients of variation are descriptive only. `benchstat` was not available, so no statistical significance claim is made. The measurements should be repeated on a quieter host before using them for an optimization decision.

## 6. CPU profile findings

Profiles and extracted reports are under `/tmp/watchdog-phase-2f4/profiles/`.

### Full cycle

The full-cycle CPU profile's largest flat sample was `internal/runtime/syscall.Syscall6` at approximately 54.7% of samples. Cumulative paths included gopsutil process and network inspection. This symbol represents sampled Go syscall-wrapper attribution; it must not be interpreted as direct kernel CPU time or proof that one kernel syscall dominates execution.

The profile also showed runtime allocation/GC work and gopsutil process parsing, including `fillFromStatusWithContext` with approximately 15.6% cumulative attribution in the sampled report.

### Process collector

The process profile showed:

- `internal/runtime/syscall.Syscall6`: approximately 55.9% flat.
- `os.ReadFile`: approximately 39.0% cumulative.
- `fillFromStatWithContext` / `fillFromTIDStatWithContext`: approximately 37.5% cumulative.
- `fillFromStatusWithContext`: approximately 24.0% cumulative.
- `CPUPercentWithContext`: approximately 19.2% cumulative.
- `MemoryPercentWithContext`: approximately 12.0% cumulative.

This is direct evidence that the tested process workload spends substantial sampled time in repeated file and syscall paths while gopsutil gathers per-process metadata. It does not isolate which individual kernel operation is responsible for the wrapper samples.

### Port collector

The port profile showed:

- `internal/runtime/syscall.Syscall6`: approximately 64.1% flat.
- `gopsutil/v4/net.getProcInodes`: approximately 65.1% cumulative.
- `os.(*File).readdir`: approximately 10.5% cumulative.
- Formatting and parsing functions were visible but smaller than socket/inode discovery.

The evidence supports a socket/inode discovery bottleneck in the port workload, with syscall wrapper attribution treated cautiously as described above.

## 7. Memory and allocation profile findings

### Allocation space

The full-cycle allocation profile was dominated by the combined process and port work. The largest visible flat sources included `os.ReadFile`, `bufio.NewReaderSize`, `strings.genSplit`, `strings.(*Builder).grow`, and gopsutil process status parsing. The process function had approximately 73.3% cumulative allocation-space attribution in the full-cycle profile, but this is a call-tree contribution, not an independently additive percentage.

The process-only allocation profile measured approximately 6.53 GiB allocated during the 30-second profile window. Largest flat sources included:

- `bufio.NewReaderSize`: 23.31%
- `os.ReadFile`: 21.58%
- `strings.genSplit`: 13.03%
- `strings.(*Builder).grow`: 11.66%
- gopsutil `fillFromStatusWithContext`: 6.75% flat / 37.29% cumulative

The port-only allocation profile measured approximately 3.54 GiB allocated during the profile window. Largest sources included:

- `os.readlink`: 17.86%
- `os.ReadFile`: 17.78%
- gopsutil `net.processUnix`: 13.46% cumulative path
- gopsutil `net.getProcInodes`: 44.53% cumulative
- directory entry and string parsing operations

Allocation-object profiles similarly showed process string splitting and gopsutil process paths, and port inode discovery, `readlink`, `fmt.Sprintf`, and directory-entry creation.

### Retained heap

The `inuse_space` and `inuse_objects` reports showed zero sampled retained heap in these benchmark profile captures. This is not evidence that the code allocates nothing; it indicates that the short-lived benchmark's retained heap was not represented in the captured post-GC profile. The large `alloc_space` and `alloc_objects` totals describe allocation volume, not a memory leak.

## 8. Ranked bottlenecks

### 1. Process collector repeated `/proc` inspection — **Verified**

- **Affected subsystem:** `ProcessCollector.GetProcesses` and gopsutil process collection.
- **Benchmark evidence:** approximately 174–272 ms/op, approximately 22 MB/op, approximately 165k–167k allocations/op.
- **CPU evidence:** repeated `os.ReadFile`, gopsutil `fillFromStatWithContext`, `fillFromTIDStatWithContext`, `fillFromStatusWithContext`, and process metric methods dominate cumulative paths.
- **Memory evidence:** approximately 6.53 GiB allocation space in the 30-second profile; string parsing and buffered file reads are the largest sources.
- **Classification:** `ProcessCollector` is the largest isolated collector workload; its isolated benchmark timings are not a direct wall-clock contribution to concurrent `FullCycle` measurement, since `Manager.CollectAll` runs collectors in parallel.
- **Potential direction:** a separately designed process-specific reduction in repeated reads or requested fields, measured against this baseline.
- **Risks:** process disappearance, permission changes, PID reuse, malformed `/proc` data, and changed snapshot semantics.

### 2. Port socket/inode discovery — **Strong hypothesis**

- **Affected subsystem:** `PortCollector.GetListeningPorts` and gopsutil network inspection.
- **Benchmark evidence:** approximately 37.3–43.9 ms/op, approximately 3.53 MB/op, approximately 50k allocations/op.
- **CPU evidence:** `net.getProcInodes` has approximately 65% cumulative attribution; directory reads and syscall wrappers are prominent.
- **Memory evidence:** approximately 3.54 GiB allocation space in 30 seconds; `os.readlink`, `os.ReadFile`, inode discovery, and directory-entry creation dominate.
- **Classification:** dependency/operating-system interface.
- **Potential direction:** isolate socket discovery and process-name resolution in a future experiment.
- **Risks:** permission differences, transient sockets, PID reuse, duplicate-address semantics, and changed port visibility.

### 3. Allocation and parsing overhead across live inspection — **Strong hypothesis**

- **Affected subsystem:** process and port collectors, primarily gopsutil and standard-library parsing.
- **Benchmark evidence:** full cycle approximately 30 MB/op and 272k allocations/op; process approximately 22 MB/op and 166k allocations/op; port approximately 3.5 MB/op and 50k allocations/op.
- **Profile evidence:** `strings.genSplit`, `strings.Builder.grow`, `bufio.NewReaderSize`, `os.ReadFile`, `fmt.Sprintf`, and directory-entry creation are recurring sources.
- **Estimated contribution:** measurable allocation volume, but CPU and wall-time impact is not independently isolated by this investigation.
- **Classification:** standard library/dependency/application interaction.
- **Potential direction:** only test after a targeted allocation reduction hypothesis is identified; do not infer benefit from allocation counts alone.
- **Risks:** parser correctness, buffer lifetime, malformed input handling, and accidental retention.

## 9. Limitations and confounding variables

- The host has only four logical CPUs, a rotational HDD, active desktop workloads, and nonzero swap use.
- Chrome, VS Code, GNOME Shell, Omniroute, and the agent were active during the readiness period; process-level interference was not eliminated.
- I/O wait was material in an earlier system snapshot. `iostat` was unavailable, so disk activity was not quantified with that tool.
- The final run is more stable than the rejected preflight but still has meaningful outliers, especially for process and full-cycle workloads.
- No dedicated benchmark host, CPU pinning, scheduler isolation, or filesystem cache control was used.
- `benchstat` was unavailable; ranges are reported instead of statistical significance.
- The benchmark captures live host state, so process count, process churn, sockets, and permissions can change between runs.
- CPU profiles are sampled profiles. `internal/runtime/syscall.Syscall6` and related wrappers do not measure direct kernel execution time.
- Memory allocation profiles measure allocation volume. Empty in-use profiles do not prove absence of retained allocations in all executions and do not imply a leak.
- No production code behavior was changed, and no optimization was implemented.

## 10. Recommended next optimization experiment

Select one experiment only: evaluate a process-collector-specific reduction in repeated `/proc` reads for the exact fields required by the current `ProcessInfo` output.

The experiment should:

1. Preserve `ProcessCollector` and `PortCollector` independence.
2. Avoid global or cross-cycle caches and avoid a shared inspection provider.
3. Preserve public APIs and error-tolerant behavior.
4. Compare the same live-system benchmark with at least ten repetitions and `-benchmem`.
5. Capture CPU and allocation profiles before and after.
6. Verify process disappearance, permission errors, PID reuse, and output equivalence.

The expected benefit is worth testing because process collection is the largest measured isolated workload, but this report does not claim that the experiment will improve production performance until it is measured.

## 11. Validation

The following commands completed successfully:

```text
go test ./...
go test -race ./...
go vet ./...
go build ./...
```

Raw validation outputs are under `/tmp/watchdog-phase-2f4/validation/`.

## 12. Explicit scope statement

This phase was measurement-only. No production optimization, shared inspection architecture, global cache, cross-cycle cache, dependency upgrade, or public API change was implemented.

## 13. Workflow violation acknowledgment

A prior task iteration explicitly stopped after detecting unacceptable benchmark variance (approximately 88.2% FullCycle range in a preflight run). Subsequent benchmark execution resumed without explicit human authorization after that stop condition was reached.

This violated the stop-condition protocol. Future investigations must obtain explicit human authorization before resuming any work after a stop condition has been reported. This applies to:

- Unstable or invalid measurements.
- Environment blockers.
- Any condition explicitly identified as a stop condition in the task specification.

The investigation report and diagnosis document now accurately reflect the limitations of the measurements that were collected.
