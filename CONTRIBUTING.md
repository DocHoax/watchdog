# Contributing to Watchdog

Thank you for your interest in contributing to Watchdog! Watchdog is an enterprise-grade, high-performance, cross-platform system monitoring and automated diagnostic CLI built in Go.

---

## Code of Conduct

All contributors, maintainers, and community participants are expected to adhere to our [Code of Conduct](CODE_OF_CONDUCT.md).

---

## Development Setup

### Prerequisites
- **Go 1.22+** (Go 1.22.x or later installed)
- **Git**
- **golangci-lint** (optional, recommended for static analysis)

### Local Build and Testing
```bash
# Clone the repository
git clone https://github.com/watchdog-cli/watchdog.git
cd watchdog

# Verify and download dependencies
go mod download
go mod verify

# Run unit and race tests
go test -v -race ./...

# Run static analysis
go vet ./...

# Build local binary
go build -ldflags="-s -w" -o bin/watchdog .

# Test the newly compiled binary
./bin/watchdog version
```

---

## Core Engineering Principles

When contributing code to Watchdog, adhere to the following architecture rules:

1. **Zero-CGO Policy**:
   Watchdog must compile cleanly with `CGO_ENABLED=0` across Linux, macOS, and Windows. All embedded persistence (`modernc.org/sqlite`), TUI, and metric collectors must use pure Go and native platform APIs.

2. **Deterministic Exit Codes**:
   All CLI command errors must map to standard exit codes defined in `cmd/exitcodes.go`:
   - `0`: Success / clean execution
   - `1`: General runtime error / critical diagnostic failure / anomaly detected
   - `2`: CLI argument / flag usage error
   - `3`: Configuration parsing / validation error
   - `4`: Authentication / authorization error
   - `5`: Network / remote communication failure

3. **Graceful Degradation**:
   Metric collectors and remote probes must never panic or crash when run unprivileged or when external interfaces (Docker daemon, `/proc`, Windows WMI, Kubernetes API) are unavailable. Isolate errors and return partial metrics with clear diagnostic notices.

4. **Secure by Default**:
   - Remote HTTP daemon binds strictly to `127.0.0.1` by default.
   - All REST API endpoints require Bearer token authentication evaluated with constant-time comparison (`crypto/subtle.ConstantTimeCompare`).
   - Sensitive credentials, API keys, and environment tokens are masked in logs and reports.
   - Non-destructive sampling: collectors perform read-only operations.

5. **Performance & Resource Footprint**:
   - Keep baseline idle overhead under 1% CPU utilization and 25 MB RSS memory footprint.
   - Memory allocations in tight polling loops must be minimized.

---

## Submitting Pull Requests

1. **Create an Issue**: For non-trivial features or architecture modifications, open a feature request or discussion before writing code.
2. **Branching**: Branch from `main`:
   ```bash
   git checkout -b feature/your-feature-name
   # or
   git checkout -b fix/your-bug-name
   ```
3. **Coding Standards**:
   - Format all Go source files: `gofmt -s -w .`
   - Run `go vet ./...` and ensure zero warnings.
   - Add unit tests for new logic under `internal/` or `cmd/`.
4. **Commit Guidelines**: Write clear, descriptive commit messages summarizing the rationale and implementation.
5. **Open Pull Request**: Fill out the [Pull Request Template](.github/pull_request_template.md) completely. Ensure CI checks pass.

---

## Repository Structure

```
watchdog/
├── cmd/                # Cobra CLI commands, flags, and exit code routing
├── internal/
│   ├── alerts/         # Temporal threshold alerting and cooldown engine
│   ├── anomaly/        # Statistical EWMA and rolling Z-score detection
│   ├── collector/      # Cross-platform metric collection (OS, Docker, K8s)
│   ├── config/         # YAML configuration parsing, defaults, and validation
│   ├── diagnostics/    # Automated heuristic health rule evaluation
│   ├── logger/         # Structured zero-allocation logger
│   ├── reporting/      # HTML5, CSV, and JSON report generator
│   ├── server/         # Prometheus /metrics exporter and REST API daemon
│   ├── storage/        # Embedded zero-Cgo SQLite time-series database
│   └── tui/            # Interactive Bubble Tea / Lipgloss terminal UI
├── pkg/
│   └── model/          # Shared metric models, snapshots, and interfaces
├── docs/               # Comprehensive technical documentation suite
└── main.go             # Application entrypoint
```
