# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [1.0.0-rc.1] - 2026-09-24

### Added
- **Interactive Terminal UI (TUI)**: Full AltScreen Bubble Tea terminal application featuring 6 navigation tabs, per-core CPU usage meters, memory breakdowns, disk I/O and network throughput sparkline graphs, interactive process list with dynamic sorting (`cpu`, `mem`, `pid`, `name`), live search filtering (`/`), signal transmission (`k`), and modal help overlay (`?`).
- **Automated Diagnostic & Remediation Engine**: Concurrent goroutine evaluation of 35 heuristic health rules across CPU, memory, disk I/O, inode exhaustion, DNS resolution latency, network sockets, process limits, and Docker containers, with automated remediation script generation (`--fix`, `--dry-run`).
- **Statistical Anomaly Detection Engine**: Pure-Go online signal processing utilizing 60-sample rolling ring buffers, Exponentially Weighted Moving Average (EWMA) filtering ($\alpha = 0.2$), and rolling Z-score evaluation ($Z = (x - \mu)/\sigma$) to detect compute bursts, memory leaks, and traffic anomalies.
- **Temporal Threshold Alerting & Hysteresis**: Stateful alerting engine featuring configurable verification duration windows (`duration`), suppression cooldowns (`cooldown`), alert lifecycle tracking (`Active`, `Resolved`), and SQLite event persistence.
- **Zero-CGO SQLite Time-Series Storage**: Pure-Go embedded database powered by `modernc.org/sqlite` with high-performance Write-Ahead Logging (`journal_mode=WAL`, `synchronous=NORMAL`), indexing, and background retention pruning.
- **Standalone Multi-Format Reporting**: Zero-dependency self-contained dark-theme HTML5 reports with inline SVG vector sparklines, structured JSON schema exports, RFC 4180 CSV exports, and ANSI terminal summaries.
- **Prometheus Exporter & Authenticated REST API**: Built-in HTTP daemon exposing standard OpenMetrics `/metrics` endpoint, authenticated `/api/v1/snapshot`, `/api/v1/diagnostics`, `/api/v1/alerts`, and `/api/v1/anomalies` endpoints, and runtime `pprof` profiling.
- **Container & Cluster Observability**: Automatic discovery of local Docker daemon containers (resource limits, states, restart counters) and Kubernetes cluster metadata (node conditions, pod phases, deployment replicas, warning events).
- **Multi-Architecture Release Engineering**: Automated GoReleaser matrix generating stripped binaries and packages (`deb`, `rpm`, `apk`, `tar.gz`, `zip`) for Linux (`amd64`, `arm64`, `armv7`), macOS (`amd64`, `arm64`), and Windows (`amd64`, `arm64`).
- **Deterministic CLI Exit Codes**: Standardized exit codes defined in `cmd/exitcodes.go` (`0` Success, `1` General Error, `2` Usage Error, `3` Config Error, `4` Auth Error, `5` Network Error).
- **Comprehensive Documentation Suite**: 17 technical documentation guides under `docs/` covering architecture, platforms, rules, configuration, API, CLI reference, and troubleshooting.

### Changed
- Refactored CLI command hierarchy under Cobra with standard aliases (`dash`, `diagnose`, `report`, `server`, `agent`, `alert`, `config`, `export`, `completion`, `version`).
- Enhanced SQLite database connection pool management to prevent lock contention under concurrent write workloads.
- Hardened default server bindings to `127.0.0.1` to prevent accidental public network exposure.

### Fixed
- Fixed GoReleaser dirty git state validation by configuring comprehensive `.gitignore` rules for local release build directories.
- Fixed Git commit SHA linker injection by standardizing build metadata flags across Windows and POSIX build targets.
- Fixed process memory calculation under unprivileged execution modes through graceful degradation and permission fallbacks.

### Security
- Enforced constant-time token comparison via `crypto/subtle.ConstantTimeCompare` across all authenticated REST API endpoints.
- Masked sensitive tokens, environment variables, and credentials in logs and report outputs.
- Applied read-only, non-destructive sampling principles across all hardware and OS metric collectors.

---

[1.0.0-rc.1]: https://github.com/DocHoax/watchdog/releases/tag/v1.0.0-rc.1
