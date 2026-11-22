# Watchdog Phase 2F.2.3.1 — Shared Linux Inspection Architecture Design Reconciliation

## 1. Verified Repository Lineage and Base Commit

- **Repository:** `https://github.com/DocHoax/watchdog`
- **Baseline Commit:** `f1f651d3f455f809760ec9a39dd8165667558cd0` (Phase 2F.1 Merge Commit)
- **Current Branch:** `feature/phase-2f3-shared-inspection-design`
- **Design Commit:** `47d9653a11908b2e5cbe01ef26b7cc0cb4d2c5bd` (to be updated by this reconciliation)
- **Dependency Version:** `github.com/shirou/gopsutil/v4 v4.25.1`

---

## 2. Actual gopsutil Call-Path Investigation

Inspection of `github.com/shirou/gopsutil/v4@v4.25.1` source code (`process/process_linux.go`, `net/net_linux.go`, `internal/common/common_linux.go`) reveals the exact execution mechanics for each operation:

1. **`process.ProcessesWithContext(ctx)`:**
   - Opens `/proc`, reads directory entries (`Readdirnames`), filters numeric directory names corresponding to PIDs.
   - For each PID, instantiates a lightweight `Process` struct (`&Process{Pid: pid}`). It does **not** read `/proc/[pid]/*` files during `ProcessesWithContext`.
2. **`net.ConnectionsWithContext(ctx, "all")` (`net_linux.go`):**
   - Parses `/proc/net/tcp`, `udp`, `tcp6`, `udp6`, etc.
   - Invokes `getProcInodesAllWithContext`, which scans `/proc` for numeric PID directories, opens `/proc/[pid]/fd`, reads symbolic links (`readlinkat`) for socket inodes (e.g., `socket:[12345]`), and builds a system-wide map of socket inode $\to$ (PID, FD).
3. **`process.NewProcessWithContext(ctx, pid)` / `Process.NameWithContext`:**
   - Instantiates a `Process` struct and reads `/proc/[pid]/comm` or `/proc/[pid]/status`.

---

## 3. Evidence-Based Duplication Matrix

| Operation | ProcessCollector | PortCollector | Duplicate? | Evidence & Source Path |
| :--- | :--- | :--- | :--- | :--- |
| **`/proc` process directory enumeration** | Yes (`process.Processes`) | Yes (`net.Connections` via `getProcInodesAll`) | **Yes** | Both independently read `/proc` directory entries to discover active PIDs (`os.ReadDir` / `Readdirnames`). |
| **`/proc/[pid]/stat`** | Yes (`fillFromStat`) | No | **No** | Read exclusively by `ProcessCollector`. |
| **`/proc/[pid]/status`** | Yes (`fillFromStatus`) | No | **No** | Read exclusively by `ProcessCollector`. |
| **`/proc/[pid]/cmdline`** | Yes (`Cmdline`) | No | **No** | Read exclusively by `ProcessCollector`. |
| **`/proc/[pid]/exe`** | Yes (`Exe`) | No | **No** | Read exclusively by `ProcessCollector`. |
| **`/proc/[pid]/fd` traversal** | No | Yes (`getProcInodesAll`) | **Yes (PortCollector only)** | Executed exclusively by `PortCollector` during network socket inode correlation. |
| **`/proc/net/*` network table reads** | No | Yes (`Connections`) | **Yes (PortCollector only)** | Read exclusively by `PortCollector`. |
| **Socket inode-to-PID correlation** | No | Yes (`Connections`) | **Yes (PortCollector only)** | Executed exclusively by `PortCollector`. |
| **Process name resolution** | Yes (`Name`) | Yes (`NameWithContext`) | **Overlapping** | `PortCollector` resolves process names for binding ports independently using `NewProcess` / `NameWithContext`. |
| **Process handle creation** | Yes (`NewProcess`) | Yes (`NewProcess`) | **Overlapping** | Both instantiate `process.Process` handles for discovered PIDs. |

---

## 4. Architectural Alternatives & Trade-Off Evaluation

We evaluated four architectural approaches:

### A. Eager Immutable Snapshot Built Before Collector Goroutines Start
- **Pros:** Maximum thread safety; eliminates duplicate `/proc` directory listing.
- **Cons:** High upfront latency before any collector can start; eagerly collects metrics that certain collectors may not need; complicates modularity.

### B. Cycle-Scoped Concurrency-Safe Lazy Provider with Synchronized Caching
- **Pros:** On-demand loading; eliminates redundant file reads when shared; preserves collector independence.
- **Cons:** Requires internal synchronization mutexes; increases implementation complexity.

### C. Explicit Optional Collector Capability/Interface
- **Pros:** Clean separation of concerns; zero overhead for independent collectors.
- **Cons:** Over-engineering for a monolithic agent daemon where collectors execute concurrently.

### D. Independent Collectors with Targeted Internal Optimizations
- **Pros:** Zero cross-package coupling; extremely simple, robust, and safe; avoids locking overhead.
- **Cons:** Retains independent `/proc` directory enumeration between `ProcessCollector` and `PortCollector`.

**Recommendation:** **Approach D (Independent Collectors with Targeted Internal Optimizations)** combined with localized caching within `PortCollector` is recommended. The profiling evidence (Phase 2F.2.1/2F.2.2) demonstrates that the vast majority of CPU time and allocation pressure in `PortCollector` and `ProcessCollector` stems from kernel syscall execution (`syscall.Syscall6`, `os.Readlink`, `os.ReadFile`), not Go struct allocation overhead. Introducing a complex shared inspection framework would add concurrency and synchronization risks without eliminating the underlying kernel syscall cost.

---

## 5. PID Identity & PID Reuse Strategy

To robustly handle PID recycling during collection cycles:
1. **Boot-Relative Start Time Validation (`CreateTime`):** A process is uniquely identified by the tuple `(PID, CreateTime)`.
2. **Invalidation:** If a PID is reused mid-cycle, its `CreateTime` in `/proc/[pid]/stat` will differ from the cached inspection window, triggering cache invalidation.
3. **Vanished Processes:** Handled gracefully by gopsutil returning `os.ErrNotExist`, which is treated as an empty/default result rather than a fatal collection error.

---

## 6. Context Cancellation & Error Handling

- **Context Propagation:** All collector methods accept `context.Context` and propagate it to gopsutil calls.
- **Filesystem Errors:** Permission-denied errors (`EACCES`) and vanished processes (`ENOENT`) are caught and logged as debug/warning events rather than aborting the entire collection cycle, ensuring partial metrics collection remains resilient.

---

## 7. Reconciled Performance Opportunities

- **Proven Opportunities:** Pre-allocating slice capacities (completed in Phase 2F.2.2) and avoiding formatting allocations (`strconv.Itoa`).
- **Hypotheses (Deferred):** A unified `/proc` scanner could theoretically reduce `/proc` directory re-scanning, but the architectural complexity outweighs the minor syscall reduction.

---

## 8. Phased Migration Plan

1. **Phase 1 (Design Approval):** Finalized via this reconciliation document.
2. **Phase 2 (Targeted Profiling):** Monitor production CPU/allocation profiles under varying load.
3. **Phase 3 (Upstream Collaboration):** Contribute upstream optimizations to `gopsutil` for efficient socket-to-PID inode mapping if required.

---

## 9. Validation Results & Git Status

- **Validation:** `go test ./...`, `go test -race ./...`, `go vet ./...`, `go build ./...` all **PASS**.
- **Remote Synchronization:** Synchronized with `origin/feature/phase-2f3-shared-inspection-design`.
- **Production Code:** **Zero production code files were modified.**
