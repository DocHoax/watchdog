# Watchdog Phase 2F.2.3 — Shared Linux Process Inspection Architecture Design

## 1. Current Architecture and Collection Flow

Watchdog collects system telemetry via `collector.Manager` (`internal/collector/collector.go`), which orchestrates registered collectors in parallel using goroutines synchronized via a `sync.WaitGroup`:

```
                    ┌────────────────────────┐
                    │    Manager.CollectAll  │
                    └───────────┬────────────┘
                                │
         ┌──────────────────────┼──────────────────────┐
         │ (concurrent goroutine)                      │ (concurrent goroutine)
         ▼                                             ▼
┌──────────────────┐                         ┌──────────────────┐
│ ProcessCollector │                         │  PortCollector   │
│ GetProcesses()   │                         │GetListeningPorts()│
└────────┬─────────┘                         └────────┬─────────┘
         │                                             │
         ▼                                             ▼
 gopsutil: process.Processes()               gopsutil: net.Connections()
   - Scans /proc                                 - Scans /proc/net/*
   - Reads status, stat, cmdline                 - Iterates /proc/[pid]/fd
   - Resolves executable/exe                     - Resolves socket inodes
```

---

## 2. Actual Duplicated System Inspection Operations

Profiling evidence (`cpu.prof`, `mem.prof`) established that independent collector execution introduces redundant kernel interactions:
1. **Independent Process Enumeration:** `ProcessCollector` invokes `process.ProcessesWithContext(ctx)`, which reads `/proc` directory entries and opens/reads `/proc/[pid]/...` for every process.
2. **Independent Socket Inode Correlation:** `PortCollector` invokes `netutil.ConnectionsWithContext(ctx, "all")`, which independently scans `/proc/net/tcp`, `udp`, etc., and then traverses `/proc/[pid]/fd` for *every* process on the system to resolve socket inode-to-PID mappings (`getProcInodesAllWithContext`).
3. **Redundant Process Metadata Lookups:** When `PortCollector` correlates socket PIDs, it subsequently re-queries `process.NewProcessWithContext` and `p.NameWithContext` to resolve process names for binding ports, duplicating metadata inspection already performed or capable of being shared with `ProcessCollector`.

---

## 3. Proposed Architecture: Cycle-Scoped Shared Inspection Layer

To eliminate redundant `/proc` traversals without compromising collector independence or introducing global mutable state, we propose a **Cycle-Scoped Shared Linux Process Inspection Cache (`LinuxInspectionContext`)**.

### Conceptual Flow:
```
                    ┌────────────────────────┐
                    │    Manager.CollectAll  │
                    └───────────┬────────────┘
                                │
                 (Initialize Cycle Inspection Context)
                                │
         ┌──────────────────────┼──────────────────────┐
         │ (Passed via context or shared manager scope)
         ▼                                             ▼
┌──────────────────┐                         ┌──────────────────┐
│ ProcessCollector │                         │  PortCollector   │
│ (Reads shared    │                         │ (Reads shared    │
│  process list)   │                         │  socket/inode map│
└──────────────────┘                         └──────────────────┘
```

---

## 4. Component Responsibilities

1. **`LinuxInspectionContext` (Cycle-Scoped):**
   - Instantiated at the beginning of a collection cycle when running on Linux.
   - Executes a single unified scan of `/proc` (or delegates to an optimized single-pass cache).
   - Provides cached process metadata and socket-to-PID inode maps for the duration of the collection cycle.
2. **`ProcessCollector`:**
   - Consumes the shared process snapshot to construct `model.ProcessSummary`.
3. **`PortCollector`:**
   - Consumes the shared connection and process/inode correlation maps to build `[]model.PortInfo` without performing secondary `/proc/[pid]/fd` scans.

---

## 5. Data Structures and Ownership

- **Lifecycle:** Created at the entry of `CollectAll` (or managed via cycle-scoped context values), and garbage-collected immediately upon completion of `CollectAll`.
- **Data Structures:**
  ```go
  type LinuxInspectionSnapshot struct {
      Timestamp time.Time
      Processes map[int32]*cachedProcessData
      Connections []netutil.ConnectionStat
      InodeToPID map[uint64]int32
  }
  ```
- **Ownership:** Owned exclusively by the active collection cycle. No cross-cycle retention.

---

## 6. Concurrency Model

- Because `ProcessCollector` and `PortCollector` run concurrently in separate goroutines, the inspection snapshot is **immutable** once initialized during the initial synchronous phase of `CollectAll` (or protected by a read-only access pattern).
- No locks are required during read operations by individual collectors.

---

## 7. PID Reuse Handling

- **Validation Mechanism:** To prevent stale PID mappings when a PID is recycled mid-cycle, cached process entries store both `PID` and `CreateTime` (boot-relative start time from `/proc/[pid]/stat`).
- If a PID's start time changes or cannot be validated, the inspection layer invalidates the cache entry.

---

## 8. Context and Cancellation Behavior

- All underlying file reads and system inspections respect the caller's `context.Context` timeout and cancellation token. If `ctx.Done()` is closed, the inspection builder aborts immediately.

---

## 9. Cross-Platform Strategy

- **Build Constraints:** The shared inspection layer is enclosed behind `//go:build linux` build tags.
- **Fallbacks:** Non-Linux platforms (Windows, macOS) bypass the shared layer entirely and execute their existing platform-specific `gopsutil` routines without modification.

---

## 10. Performance Hypotheses

- **Reduction Target:** Eliminate double-traversal of `/proc/[pid]/fd` and `/proc/[pid]/status`.
- **Expected Benefit:** Reduce FullCycle allocation volume and CPU time by avoiding duplicate `gopsutil` internal process-listing and file descriptor scans.

---

## 11. Benchmark and Test Plan

- **Validation Benchmarks:** `BenchmarkCollector_FullCycle`, `BenchmarkCollector_Process`, `BenchmarkCollector_Port`.
- **Regression Tests:** Strict concurrency race tests (`-race`), PID reuse simulation tests, and cross-platform compatibility builds.

---

## 12. Migration Plan

1. **Phase 1 (Design Approval):** Review and sign off on this design document (`PHASE_2F2_SHARED_INSPECTION_DESIGN.md`).
2. **Phase 2 (Implementation Sandbox):** Implement `LinuxInspectionContext` behind Linux build tags in `internal/collector/linux_inspection.go`.
3. **Phase 3 (Collector Integration):** Refactor `ProcessCollector` and `PortCollector` to consume the shared inspection context.
4. **Phase 4 (Benchmark Validation):** Compare allocations and latency against the Phase 2F.2.1 baseline.

---

## 13. Risks and Unresolved Questions

- **Risk:** Increased memory overhead during the collection window if all process details are eagerly loaded.
- **Mitigation:** Employ lazy loading within the immutable snapshot (e.g., caching raw process file paths or pids on demand during the cycle while avoiding duplicate inode scans).
