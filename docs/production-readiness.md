# Watchdog Production Readiness & Baseline Audit

**Audit Date**: September 23, 2026  
**Document Version**: 1.0.0-baseline  
**Target Application**: Watchdog System Monitoring & Diagnostics Suite (`github.com/DocHoax/watchdog`)

---

## 1. Architecture Overview

Watchdog is structured as a modular, decoupled Go application divided into:
- **CLI Layer (`cmd/`)**: Built with Cobra (`github.com/spf13/cobra`) managing subcommands (`dash`, `diagnose`, `report`, `server`, `agent`, `alert`, `config`, `export`, `version`).
- **Core Domain Models (`pkg/model/`)**: Clean telemetry domain structures covering CPU, Memory, Disk, Network, Processes, Services, Ports, Docker, Kubernetes, Diagnostics, Alerts, and Anomalies.
- **Metric Collectors (`internal/collector/`)**: Coordinated by `collector.Manager` executing concurrent, timeout-bound sampling using `gopsutil`, net dialers, and container clients.
- **Diagnostics Engine (`internal/diagnostics/`)**: Rule evaluation engine assessing 10+ operational health rules and generating automated remediation plans.
- **Alert Engine (`internal/alerts/`)**: Stateful threshold evaluation engine with hysteresis duration tracking and cooldown suppression.
- **Anomaly Detection (`internal/anomaly/`)**: Online baseline estimation using Exponentially Weighted Moving Averages (EWMA) and rolling standard deviation Z-scores.
- **Storage Subsystem (`internal/storage/`)**: Embedded SQLite engine (`modernc.org/sqlite`) running in Write-Ahead Logging (WAL) mode with background metric retention pruning.
- **Reporting Engine (`internal/reporting/`)**: Standalone self-contained HTML (with inline SVG sparklines), JSON, CSV, and ANSI terminal report generators.
- **HTTP & Prometheus Server (`internal/server/`)**: Prometheus `/metrics` exposition and token-authenticated REST APIs (`/api/v1/snapshot`, `/api/v1/diagnose`, `/api/v1/alerts`).
- **Interactive TUI (`internal/tui/`)**: Full-screen 6-tab Bubble Tea (`github.com/charmbracelet/bubbletea`) terminal dashboard with Lip Gloss styling.

---

## 2. Component Inventory

| Subsystem | Package Path | Primary Responsibility | Concurrency / Thread Safety |
| :--- | :--- | :--- | :--- |
| **CLI** | `cmd/` | Flag parsing, global config, subcommands | CLI execution loop |
| **Config** | `internal/config/` | YAML configuration parsing & validation | Read-only post initialization |
| **Collector** | `internal/collector/` | System & container metrics collection | `sync.WaitGroup`, per-collector timeout contexts |
| **Diagnostics** | `internal/diagnostics/` | Rule evaluation & remediation | Stateless evaluator with concurrent checks |
| **Alerts** | `internal/alerts/` | Stateful threshold monitoring & cooldowns | `sync.RWMutex` protecting active alert state |
| **Anomaly** | `internal/anomaly/` | EWMA and Z-score time-series analysis | `sync.RWMutex` protecting statistical histories |
| **Audit** | `internal/audit/` | Structured security audit logging & sanitization | Thread-safe, non-blocking fallback |
| **Storage** | `internal/storage/` | SQLite persistence, queries, pruning & audit | `sync.Mutex` on SQLite connection, WAL mode |
| **Reporting** | `internal/reporting/` | Multi-format report generation | Pure functions & immutable data |
| **Server** | `internal/server/` | Prometheus exporter & REST API | `http.Server`, atomic metrics collection |
| **TUI** | `internal/tui/` | Interactive terminal UI | Bubble Tea event loop, async tick commands |
| **Logger** | `internal/logger/` | Structured levelled logging | `sync.Mutex` protecting output io.Writer |
| **Utilities** | `pkg/util/` | String, byte unit, network probing helpers | Pure stateless helper functions |

---

## 3. Current Capabilities

1. **System Metrics**:
   - Host metadata (OS, kernel, uptime, virtualization detection).
   - CPU aggregate utilization, per-core load, and load averages (Linux/macOS).
   - Memory physical usage, buffers, cache, available RAM, and swap partition pressure.
   - Storage mount points, total/used/free capacity, percentage used, and inode counters (POSIX).
   - Network interface stats (RX/TX bytes, packets, error rates, drop rates) and connection states.
   - Process tree mapping (PID, PPID, CPU %, RSS memory %, execution state, thread count, I/O rates).
   - Listening TCP/UDP network ports and local endpoints.
   - System service states (systemd units on Linux, Service Control Manager on Windows, launchd on macOS).
2. **Container Telemetry**:
   - Docker / Podman container status, image tags, CPU % and Memory RSS limits.
   - Kubernetes pod discovery, namespace mapping, restart counters, and container phase tracking.
3. **Automated Diagnostics**:
   - Evaluates CPU saturation, load balance, memory pressure, swap exhaustion, disk capacity, inode limits, DNS latency, default gateway reachability, zombie tasks, rogue runaway processes, and container crash-loops.
4. **Alerting & Anomaly Detection**:
   - Configurable thresholds with minimum sustained duration before firing and cooldown suppression.
   - Rolling Z-score anomaly detection flags statistical deviations (> 2.5σ) over 60-sample windows.
5. **Storage & Reporting**:
   - Embedded SQLite time-series storage with automated retention pruning (default 7 days).
   - HTML, JSON, CSV, and ANSI Terminal multi-format reports.
6. **Prometheus & REST API**:
   - Standard `/metrics` Prometheus exposition format.
   - Secure REST API with Bearer token authentication.
   - Secure audit events query endpoint (`/api/v1/audit/events`) with pagination and time filtering.
7. **Security Audit Logging**:
   - Structured audit trail for authentication, server lifecycle, TLS status, configuration modifications, and administrative actions.
   - Zero credential leakage via non-reversible SHA-256 token hashing and recursive metadata sanitization.
   - In-memory flood limiter mitigating database write exhaustion from authentication brute-force attacks.
   - CLI audit management (`watchdog audit list`, `watchdog audit export`) with CSV formula injection neutralization.

---

## 4. Known Limitations & Platform-Specific Behaviors

- **Load Average on Windows**: Windows does not provide Unix-style 1/5/15 minute load averages; Watchdog falls back gracefully to aggregate CPU utilization and core count normalization.
- **Inodes on Windows**: NTFS and FAT32 do not expose Unix inode structures; the Inode check reports `N/A (Non-Unix or unsupported filesystem)` as a neutral status rather than a failure.
- **Service Monitoring**:
  - Windows: Uses `golang.org/x/sys/windows/svc/mgr` for native Service Control Manager queries.
  - Linux: Uses `systemctl list-units` / D-Bus systemd queries.
  - macOS: Uses `launchctl list`.
  - Fallback / Containers: Returns graceful empty list with unsupported reason.
- **Temperature Sensors**: Hardware temperature sensors are platform and vendor-dependent and require elevated root/administrator privileges; Watchdog marks temperature as `UNAVAILABLE` when missing without crashing.
- **Docker / Kubernetes Permissions**: Access to `/var/run/docker.sock` or `~/.kube/config` requires appropriate user permissions; collectors gracefully degrade to `DISABLED` or `UNSUPPORTED` when unavailable.

---

## 5. Security-Sensitive Operations & Attack Surfaces

1. **Network Bind Security Validation (`internal/server/server.go`)**:
   - The server enforces a mandatory security invariant at startup: non-loopback bind addresses (`0.0.0.0`, LAN IPs) require **both** a Bearer authentication token and a complete TLS certificate/key pair. The server refuses to start without them and prints actionable error messages.
   - Validation runs twice as defense-in-depth: first in `cmd/server.go` pre-flight (before resource allocation), then again inside `server.Start()`.
   - Empty bind addresses fall back to `127.0.0.1` (loopback), not `0.0.0.0`.
   - Incomplete TLS configurations (cert without key, or vice versa) are always rejected regardless of bind address.
2. **Process Inspection and Termination (`cmd/dash.go` / `internal/tui/`)**:
   - Interactive process termination sends OS signals (`SIGTERM` / `SIGKILL` or `taskkill`). Must verify that PIDs are sanitized numeric integers and never interpolated into raw shell strings.
3. **REST API & Prometheus Exporter (`internal/server/`)**:
   - Default bind address is `127.0.0.1` (localhost) to prevent unintended network exposure.
   - REST API endpoints require Bearer Token authentication via `Authorization: Bearer <token>` or `X-Watchdog-Token: <token>`.
   - Token comparison uses constant-time `crypto/subtle.ConstantTimeCompare` to prevent timing attacks.
   - Prometheus `/metrics` endpoint exposes high-level telemetry without sensitive credentials or command line arguments containing secrets.
   - `/health` and `/metrics` endpoints are intentionally unauthenticated for liveness probes and Prometheus scraping.
4. **Filesystem Writes (`internal/storage/`, `cmd/report.go`, `cmd/config.go`)**:
   - Report outputs, SQLite databases, and config paths must guard against directory traversal and sanitize destination paths.
5. **Credential & Secret Sanitization & Resolution**:
   - Secret precedence hierarchy (CLI flags > Secret files > Environment variables > Plain configuration).
   - Secret files verified for existence, non-emptiness, whitespace trimming, and POSIX permissions (`0600`/`0400`).
   - Configuration redaction (`Redacted()`) prevents plaintext token and TLS key exposure during `watchdog config show` or JSON serialization.
   - `Config.Save()` ensures secrets resolved dynamically from files/env are not inadvertently written in plaintext to config YAML.
   - All CLI flags and help examples sanitized to `<token>`.
   - CI/CD workflow security includes automated Gitleaks secret scanning.
   - No plaintext passwords, API keys, or tokens are logged in application logs or embedded in HTML/JSON/CSV diagnostic reports.
   - Error messages from security validation are actionable but never include the configured token or key values.

6. **Structured Audit Logging & Zero Credential Exposure (`internal/audit/`, `internal/storage/`)**:
   - Machine-readable audit events recorded for auth attempts, lifecycle transitions, TLS configuration, config changes, and administrative actions.
   - Plaintext tokens, passwords, private keys, `Authorization` headers, and sensitive environment variables are strictly excluded from all audit records and metadata maps.
   - Failed authentication attempts record masked actor identities (`token:sha256:<8-hex-prefix>`) without exposing the candidate token.
   - In-memory rate limiting (`FloodLimiter`) prevents SQLite write exhaustion during authentication brute-force attacks.
7. **CSV Formula Injection Neutralization (`cmd/audit.go`)**:
   - CSV export operations prepend a single quote (`'`) to any cell starting with `=`, `+`, `-`, `@`, `\t`, or `\r` to neutralize formula injection (CSV/DDE) vulnerabilities in spreadsheet viewers.
8. **SQLite Tamper Limitations & Transparency**:
   - Audit records stored in local embedded SQLite do not provide cryptographic tamper-proofing or hardware WORM immutability against root/administrator modification on the host. High-assurance environments should export or forward logs to external immutable log aggregation systems.
9. **Software Supply Chain Hardening & Build Provenance**:
   - Releases are signed keylessly with Cosign via GitHub Actions OIDC; long-lived private signing keys are eliminated.
   - SLSA Build Provenance and SPDX 2.3 SBOMs are cryptographically attested via `actions/attest-build-provenance` and `actions/attest-sbom`.
   - Pure Go zero-CGO compilation with `-trimpath` strips developer and CI build machine filesystem paths from compiled binaries.
   - Go dependency integrity is continuously enforced with `go mod verify` in CI.
   - Independent build reproducibility is verified across clean dual-build verification scripts (`scripts/verify-reproducibility.sh` / `.ps1`).

---

## 6. Current Test Coverage Baseline

- All unit and integration tests across all 13 Go packages pass (`cmd`, `internal/alerts`, `internal/anomaly`, `internal/audit`, `internal/collector`, `internal/config`, `internal/diagnostics`, `internal/logger`, `internal/reporting`, `internal/server`, `internal/storage`, `internal/tui`, `pkg/model`, `pkg/util`).
- Security validation tests comprehensively cover: `IsLoopback()` for all address types, `ValidateServerSecurity()` for all bind address × token × TLS combinations, HTTP-level auth enforcement via httptest, concurrent request safety, method restriction, path traversal rejection, Request ID propagation, audit sanitization, token masking, flood limiting, SQLite audit queries and pruning, and graceful shutdown.
- Supply chain security tests cover: version metadata formatting (`cmd/version_test.go`), deterministic build reproducibility verification (`cmd/reproducibility_test.go`), GoReleaser v2 configuration validation (`goreleaser check`), and module checksum verification (`go mod verify`).

---

## 7. Software Supply Chain & Release Verification Checklist

| Security Control | Implementation | Verification Tool / Command | Status |
| :--- | :--- | :--- | :--- |
| **SPDX 2.3 SBOMs** | Anchore Syft via GoReleaser | `jq -e '.spdxVersion' <artifact>.sbom.json` | :white_check_mark: Verified |
| **Keyless Sigstore Signing** | Cosign + GitHub Actions OIDC | `cosign verify-blob --bundle checksums.txt.sigstore.json ...` | :white_check_mark: Verified |
| **SLSA Build Provenance** | GitHub Artifact Attestations | `gh attestation verify <artifact> --owner DocHoax` | :white_check_mark: Verified |
| **SBOM Attestation** | GitHub SBOM Attestations | `gh attestation verify --predicate-type https://spdx.dev/Document` | :white_check_mark: Verified |
| **Zero CGO (`CGO_ENABLED=0`)** | Static Pure Go | `go version -m <binary>` / CI cross-compile matrix | :white_check_mark: Verified |
| **Path Trimming (`-trimpath`)** | Strips build machine paths | `go build -trimpath` / Reproducibility verification | :white_check_mark: Verified |
| **Dependency Integrity** | Locked `go.sum` hashes | `go mod verify` in CI and Release workflows | :white_check_mark: Verified |
| **Build Reproducibility** | Dual-build SHA256 script | `./scripts/verify-reproducibility.sh` | :white_check_mark: Verified |
