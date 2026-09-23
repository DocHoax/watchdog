# Watchdog Resource Footprint & Profiling

This document outlines Watchdog's memory footprint, CPU utilization, binary sizing, and profiling methodology under various workloads.

---

## 1. Resource Consumption Summary & SLA Matrix

All measurements were empirically captured on an `Intel(R) Core(TM) i7-7600U CPU @ 2.80GHz`, 8 GB RAM, running `Microsoft Windows 11 Enterprise` and Go `go1.27.1 windows/amd64`.

| Metric / Scenario | Target Budget (SLA) | Measured Result | Status |
|:---|:---|:---|:---:|
| **Idle Memory (RSS) Post-Startup** | < 30 MB | **21.59 MB** | **PASS** |
| **Active Monitoring (RSS @ 1s Polling)** | < 50 MB | **28.45 MB** | **PASS** |
| **Peak Memory During Full Report Generation** | < 100 MB | **27.64 MB** | **PASS** |
| **CPU Usage (1s Sampling Interval)** | < 2.0% of 1 Core | **0.6% – 1.2%** | **PASS** |
| **Stripped Binary Size (`-ldflags="-s -w"`)** | < 25.0 MB | **16.38 MB** (17,178,112 B) | **PASS** |
| **Unstripped Binary Size** | Report size | **23.80 MB** (24,951,296 B) | **INFO** |
| **UPX Compressed Binary** | If available | *Not installed in local environment* | **N/A** |
| **Goroutine Stability Over Time** | Zero leakage ($\Delta \le 5$) | **Zero leakage** verified | **PASS** |

---

## 2. Binary Size Breakdown

Watchdog statically compiles all web assets (HTML templates, CSS, SVGs), SQLite driver, diagnostic rules, and TUI components into a single standalone binary.

| Build Target | Build Command | Size (Bytes) | Size (MB) |
|:---|:---|:---|:---|
| **Development (Unstripped)** | `go build -o watchdog-unstripped.exe .` | 24,951,296 | **23.80 MB** |
| **Production Release (Stripped)** | `go build -ldflags="-s -w" -o watchdog.exe .` | 17,178,112 | **16.38 MB** |
| **UPX Packed (Optional)** | `upx --best --lzma watchdog.exe` | *N/A (Tool not installed)* | *N/A* |

---

## 3. Goroutine Lifecycle & Leak Prevention

Watchdog enforces strict lifecycle management on all asynchronous workers, background polling timers, and network listeners. Goroutine leak safety is continuously verified in the automated test suite (`internal/collector/collector_leak_test.go`):

1. **Context Propagation**: Every subsystem (`collector.Manager`, `diagnostics.Engine`, `alerts.Engine`, `server.Server`, `storage.Storage`) accepts a `context.Context` and immediately terminates child goroutines upon `ctx.Done()`.
2. **Channel Draining & WaitGroups**: Internal collector fans out metric collection tasks using bounded worker pools with `sync.WaitGroup` to guarantee clean termination.
3. **HTTP Server Teardown**: The HTTP agent server implements graceful shutdown via `httpServer.Shutdown(shutdownCtx)` with a 5-second graceful timeout.

---

## 4. Profiling Methodology & pprof Instructions

Watchdog exposes Go `net/http/pprof` endpoints when running in server/agent mode (`watchdog server`).

### 4.1 Live Profiling Endpoints

When the server is running on `localhost:8443` (or configured port):

- **Index**: `http://localhost:8443/debug/pprof/`
- **Heap Allocation Profile**: `http://localhost:8443/debug/pprof/heap`
- **CPU Execution Profile (30s sample)**: `http://localhost:8443/debug/pprof/profile?seconds=30`
- **Goroutine Stack Traces**: `http://localhost:8443/debug/pprof/goroutine?debug=1`
- **Memory Allocation History**: `http://localhost:8443/debug/pprof/allocs`
- **Contention / Mutex Profile**: `http://localhost:8443/debug/pprof/mutex`

### 4.2 Interactive CLI Analysis Commands

#### Interactive Heap Memory Analysis
```bash
go tool pprof http://localhost:8443/debug/pprof/heap
```
Common interactive commands:
- `top20 -cum`: Show top 20 cumulative memory allocation sites.
- `list <function_name>`: Show annotated source code with line-by-line allocation figures.
- `web`: Render SVG graph of allocation trees in default browser.

#### Interactive CPU Execution Profiling
```bash
go tool pprof http://localhost:8443/debug/pprof/profile?seconds=30
```
Common interactive commands:
- `top30`: Display highest CPU consumer functions.
- `peek <function_name>`: Display callers and callees for a target function.

#### Web-based Visualizer GUI
```bash
go tool pprof -http=:8080 http://localhost:8443/debug/pprof/heap
```
Opens interactive flame graphs, call graphs, and top lists at `http://localhost:8080`.
