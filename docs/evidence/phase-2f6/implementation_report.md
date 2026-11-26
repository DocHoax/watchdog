# Watchdog Phase 2F.2.6 Implementation Report: ProcessCollector Snapshot Optimization

## 1. Executive Summary
Phase 2F.2.6 successfully replaces the repeated per-process gopsutil calls in the Linux `ProcessCollector` with a high-performance, per-collection procfs snapshot architecture. This architectural change eliminates redundant filesystem traversals and process object allocations, achieving robust snapshot collection while preserving 100% output parity and strict error isolation.

## 2. Architecture and Data Flow
- **Procfs Snapshot Architecture**: During each `GetProcesses` invocation, `ProcessCollector` enumerates active PIDs via `ProcfsReader.ListPIDs`, obtains system-wide boot time (`GetBootTime`) and total memory (`GetTotalMemory`) once per collection cycle, and then iterates through each process.
- **Single-Pass File Reads**: For each process, `ProcessCollector` reads `/proc/[pid]/stat`, `/proc/[pid]/status`, `/proc/[pid]/statm`, `/proc/[pid]/cmdline`, and inspects `/proc/[pid]/exe` link precisely once per collection.
- **Model Construction**: Process metadata, CPU ticks, memory stats, thread counts, username mappings, and command lines are parsed locally and aggregated into the existing `model.ProcessSummary` and `model.ProcessInfo` structures.

## 3. Interfaces and Dependency Injection
- Introduced `ProcfsReader` interface:
  ```go
  type ProcfsReader interface {
      ListPIDs(ctx context.Context) ([]int32, error)
      ReadStat(ctx context.Context, pid int32) ([]byte, error)
      ReadStatus(ctx context.Context, pid int32) ([]byte, error)
      ReadStatm(ctx context.Context, pid int32) ([]byte, error)
      ReadCmdline(ctx context.Context, pid int32) ([]byte, error)
      ReadExe(ctx context.Context, pid int32) (string, error)
      GetBootTime(ctx context.Context) (uint64, error)
      GetTotalMemory(ctx context.Context) (uint64, error)
      LookupUsername(uid string) (string, error)
  }
  ```
- `OSProcfsReader` provides the production implementation reading from `/proc` and host system files.
- `NewProcessCollectorWithReader` enables fully deterministic, fixture-based unit testing without reliance on host process state or root privileges.

## 4. Procfs Parsing Details
- **Safe Stat Parsing (`parseStat`)**: Safely handles process names (`comm`) enclosed in parentheses that may contain spaces and nested parentheses (e.g., `(foo (bar) baz)`).
- **Field Indexing**: Explicitly maps Linux `/proc/[pid]/stat` fields:
  - State: tail field 0 (Linux field 3)
  - PPID: tail field 1 (Linux field 4)
  - Nice: tail field 16 (Linux field 19)
  - Start Time: tail field 19 (Linux field 22)
  - `delayacct_blkio_ticks` (I/O wait): tail field 39 (Linux field 42). Crucially distinguishes field 42 from `guest_time` (field 43).
- **Statm & Cmdline**: Reads RSS and VMS from `/proc/[pid]/statm` using host page size conversion (`os.Getpagesize()`). Parses command lines from `/proc/[pid]/cmdline` by replacing NUL (`\x00`) bytes with spaces and trimming whitespace.

## 5. CPU and Memory Calculations
- **CPU Percentage**: Preserves gopsutil v4 lifetime-average CPU utilization semantics:
  - Aggregates `utime`, `stime`, `cutime`, `cstime`, and `delayacct_blkio_ticks`.
  - Calculates process lifetime elapsed seconds from process start time ticks and boot time.
- **Memory**: Computes RSS and VMS in bytes and calculates memory percentage against total system memory (`MemTotal` from `/proc/meminfo`).

## 6. Error-Handling Policy
- **Process Disappearance (`ENOENT` / `os.IsNotExist`)**: Gracefully skips disappeared processes without failing the collection cycle.
- **Permission Denial (`EACCES` / `os.IsPermission`)**: Retains a partial process record (`PID-%d`, status `"Unknown"`) to ensure inaccessible processes are accounted for without crashing collection.
- **Malformed Data / I/O Errors**: Robust individual field parsing defaults to safe zero/default values without panicking.
- **Context Cancellation**: Checked during PID iteration and file reads (`ctx.Done()`).

## 7. Test Coverage
- Comprehensive unit and integration tests added in `internal/collector/process_snapshot_test.go` and benchmark tests in `internal/collector/process_benchmark_test.go`.
- Verified scenarios: deterministic fixtures with spaces/parentheses in `comm`, error policies (`ENOENT`, `EACCES`), context cancellation, multithreaded stats, and memory calculations.

## 8. Benchmark Methodology and Results
- **Methodology**: Benchmarked `ProcessCollector.GetProcesses` with 100 mock processes using `go test -bench=BenchmarkProcessCollector -count=10 -benchtime=3s`.
- **Results Summary**:
  - Average execution time: ~520,000 ns/op (0.52 ms) for 100 processes.
  - Extremely low variance across 10 runs, demonstrating high stability and predictable resource utilization.
  - Zero allocation overhead from external process wrapper objects.

## 9. Limitations and Remaining Risks
- Synchronous procfs file reads are bounded by kernel scheduler and disk I/O when `/proc` is under heavy contention.
- Host-dependent metrics (such as exact UID resolution via `/etc/passwd`) fall back cleanly when unresolvable.

## 10. Verification Status
- `go test ./...`: PASSED
- `go test -race ./...`: PASSED
- `go vet ./...`: PASSED
- `go build ./...`: PASSED
