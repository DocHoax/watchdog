# Watchdog Phase 2F.2.3.2 — Final Architecture Design Accuracy Corrections

## 1. Final Architecture Decision

- **Decision:** **Retain independent collectors with targeted internal optimizations (`ProcessCollector` and `PortCollector`).**
- **Explicit Restriction:** **Do not implement a shared Linux inspection provider (`LinuxInspectionContext`).**
- **Cache Policy:** **Do not introduce global or cross-cycle caches.**
- **Implementation Status:** Implementation is **not approved**. This document serves solely as a final design record and investigation report.

---

## 2. Corrected Duplication Matrix

| Operation | ProcessCollector | PortCollector | Duplicate? | Evidence & Attribution |
| :--- | :--- | :--- | :--- | :--- |
| **`/proc` process directory enumeration** | Yes (`process.Processes`) | Yes (`net.Connections` via `getProcInodesAll`) | **Yes** | Both independently read `/proc` directory entries (`os.ReadDir` / `Readdirnames`). |
| **`/proc/[pid]/stat`** | Yes (`fillFromStat`) | No | **No** | Read exclusively by `ProcessCollector`. |
| **`/proc/[pid]/status`** | Yes (`fillFromStatus`) | No | **No** | Read exclusively by `ProcessCollector`. |
| **`/proc/[pid]/cmdline`** | Yes (`Cmdline`) | No | **No** | Read exclusively by `ProcessCollector`. |
| **`/proc/[pid]/exe`** | Yes (`Exe`) | No | **No** | Read exclusively by `ProcessCollector`. |
| **`/proc/[pid]/fd` traversal** | No | Yes (`getProcInodesAll`) | **No (PortCollector only)** | Executed exclusively by `PortCollector` during socket inode correlation. |
| **`/proc/net/*` network table reads** | No | Yes (`Connections`) | **No (PortCollector only)** | Read exclusively by `PortCollector`. |
| **Socket inode-to-PID correlation** | No | Yes (`Connections`) | **No (PortCollector only)** | Executed exclusively by `PortCollector`. |
| **Process name resolution** | Yes (`Name`) | Yes (`NameWithContext`) | **Overlapping (Independent)** | `PortCollector` resolves process names for binding ports independently using `NewProcess` / `NameWithContext` (utilizing a localized `procNameCache` map). |
| **Process handle creation** | Yes (`NewProcess`) | Yes (`NewProcess`) | **Overlapping (Independent)** | Both independently instantiate `process.Process` handles for discovered PIDs. |

---

## 3. Process Identity Terminology Correction

- **Distinction of Terms:**
  - **Raw `/proc/[pid]/stat` ticks:** Kernel jiffies representing process start time relative to system boot.
  - **gopsutil `CreateTimeWithContext` return value:** Unix epoch timestamp in milliseconds (converted from kernel jiffies and boot time).
  - **Internal Process Identity Representation:** If process validation were pursued, identity would be defined by the tuple `(PID, CreateTime)`.
- **Unverified Behavior Note:** Where exact cache invalidation mechanics upon PID recycling cannot be guaranteed across concurrent kernel state changes without locking, they are marked unverified rather than assumed.

---

## 4. Cancellation Claims Correction

- **Filesystem Interruptibility:** It is incorrect to claim that all filesystem reads during `/proc` traversal are fully interruptible by `context.Context`.
- **Distinction:**
  - Context-aware gopsutil API methods check `ctx.Err()` between major iteration steps.
  - Filesystem operations (`os.ReadFile`, `os.Readlink`, `syscall.Read`) are synchronous OS system calls and are not guaranteed to be promptly cancelled mid-read unless underlying file descriptors support non-blocking / deadline cancellation.

---

## 5. Profiling Interpretation Correction (`syscall.Syscall6`)

- **Interpretation:** `syscall.Syscall6` flat CPU attribution in Go runtime CPU profiles (`-cpuprofile`) measures the Go runtime transition overhead (`entersyscall` / `exitsyscall` bookkeeping, thread blocking/unblocking, and argument marshalling) when executing system calls.
- **Limitation:** It does **not** directly measure kernel-space execution time or hardware instruction cycles spent inside the Linux kernel. It reflects Go execution overhead interacting with kernel interfaces.

---

## 6. Phase 2F.2.2 Optimization Evaluation Record

- **Tested Changes:** Slice pre-allocation (`procList`, `ports`) and key formatting adjustments (`strconv.Itoa`).
- **Benchmark Findings:** Benchmark results did not establish a reliable or compelling net performance improvement across full collection cycles.
- **Conclusion:** The optimization branch remains unmerged. Optimizations that add complexity without measurable, reproducible performance gains must be discarded.

---

## 7. Localized Caching Documentation

- **Existing Behavior:** `PortCollector.GetListeningPorts` (`internal/collector/port.go`) already maintains a localized process-name cache (`procNameCache := make(map[int32]string)`) scoped strictly to a single collection cycle.
- **Policy:** No new caches (global, cross-cycle, or shared across independent collectors) are approved without empirical performance proof demonstrating substantial allocation or latency reduction.

---

## 8. Final Architecture Summary

1. **Keep `ProcessCollector` and `PortCollector` independent.**
2. **Do not introduce `LinuxInspectionContext`.**
3. **Do not introduce global or cross-cycle caches.**
4. **Preserve existing platform-specific behavior across Linux, Windows, and macOS.**
5. **Revisit shared inspection only if future profiling identifies a substantial, measurable opportunity.** Implementation is not approved.
