# Watchdog repository instructions

## Project shape

Watchdog is a Go 1.22+ cross-platform system observability CLI. The root
command is wired in `main.go` and `cmd/`; Cobra commands load YAML
configuration, initialize the shared services, and map failures to the
deterministic exit codes in `cmd/exitcodes.go`.

The main runtime pipeline is:

1. `internal/collector` registers platform-aware collectors and runs enabled
   collectors concurrently to assemble a `pkg/model.SystemSnapshot`.
2. The snapshot is consumed by the diagnostic engine
   (`internal/diagnostics`), temporal alert engine (`internal/alerts`), and
   statistical anomaly detector (`internal/anomaly`).
3. `internal/storage` persists snapshots, scalar time-series points, alerts,
   diagnostics, audit events, fleet telemetry, and incidents in pure-Go
   `modernc.org/sqlite` using WAL mode and retention pruning.
4. The same services feed the interactive Bubble Tea TUI
   (`internal/tui`), multi-format reporting (`internal/reporting`), and the
   authenticated HTTP/Prometheus server (`internal/server`).
5. Fleet management and predictive/incident analysis are layered in
   `internal/fleet`, `internal/intelligence`, and `internal/incidents`; the
   read-only MCP integration is in `internal/mcp`.

`pkg/model` contains the shared data contracts used across these layers.
Configuration defaults, YAML parsing, validation, and secret resolution live
in `internal/config`. Platform-specific collector implementations use
OS-specific files such as `service_linux.go`, `service_darwin.go`, and
`service_windows.go`.

## Build, test, and verification commands

Go modules and the toolchain are the only required build dependencies. Keep
the zero-CGO policy intact:

```bash
go mod download
go mod verify

# Local stripped binary
CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/watchdog .
./bin/watchdog version

# Full test suite with race detection
go test -v -race ./...

# Run one package or one test
go test -v -race ./internal/collector
go test -v -race ./internal/collector -run '^TestCollectorName$'

# Disable test caching when reproducing a failure
go test -count=1 ./...

# Static checks used by CI
gofmt -l .
go vet ./...
go mod verify

# Benchmarks
go test -bench=. -benchmem ./internal/...
go test -v -bench='^BenchmarkEvaluateFleetHealth$' ./internal/intelligence
```

CI runs format checking, `go vet`, module verification, platform-specific
tests (race-enabled on Linux/macOS), `CGO_ENABLED=0` cross-compilation, and
reproducibility verification. macOS tests use
`-ldflags="-linkmode=external"`; Windows tests do not use `-race`. Release
configuration is checked with GoReleaser.

For focused behavior, follow the repository's test naming and run a package
with `-run`, for example:

```bash
go test -v ./internal/server -run 'Test(PanicRecoveryMiddleware|MaxBodySizeMiddleware)$'
go test -v ./internal/mcp -run 'TestSecurity_'
go test -v ./cmd -run 'TestIntelligence'
```

## Repository-specific conventions

- Preserve pure-Go, zero-CGO compatibility for Linux, macOS, and Windows.
  Avoid adding dependencies or APIs that require a native runtime.
- Collectors and remote integrations must fail soft: unavailable Docker,
  Kubernetes, `/proc`, WMI, sockets, or privileges should produce partial
  results/status errors rather than panics or process-wide failure. Respect
  `context.Context` cancellation in collection and service loops.
- Collector implementations satisfy `collector.Collector` (`Name` plus
  `Collect(context.Context)`) and are registered through the manager. New
  snapshot data must use the existing `pkg/model` types and be wired into the
  manager's result switch.
- Diagnostic rules implement the `diagnostics.Rule` contract and are
  registered by `diagnostics.NewEngine`; rule evaluation is concurrent and
  results must use the existing pass/warning/fail/skip model.
- Alert rules are stateful: threshold breaches pass through the duration
  verification and cooldown/hysteresis tracker. Persist fired/resolved events
  through the alert engine rather than bypassing its state tracker.
- Shared mutable services use the existing mutex-protected patterns. Avoid
  unbounded buffers; fleet telemetry and anomaly streams have explicit
  capacity/window limits.
- Storage changes must update the `storage.Storage`/`ReadOnlyStorage` contract
  and SQLite migration schema together. Preserve WAL operation, indexed
  time-series queries, retention/pruning behavior, and `Close` lifecycle
  handling.
- CLI command failures should be wrapped with `NewExitError` or
  `WrapExitError` using the categories in `cmd/exitcodes.go`: general `1`,
  usage `2`, configuration `3`, authentication `4`, and network `5`.
- The root `PersistentPreRunE` configures logging and loads configuration
  before subcommands run. Reuse `globalCfg` and existing secret-resolution
  behavior instead of loading configuration independently in each command.
- The HTTP server defaults to loopback (`127.0.0.1`). Non-loopback binding
  requires both an authentication token and complete TLS certificate/key
  configuration; authenticated API routes use the server middleware and
  constant-time token comparison.
- Keep sensitive credentials out of logs, reports, test fixtures, and error
  strings. Use the repository's audit logger/sanitizer for security-relevant
  events.
- The MCP surface is intentionally read-only. New MCP tools/resources must
  follow the existing allowlist, validation, rate limiting, and authenticated
  transport patterns.
- Put package tests beside the implementation in `*_test.go`. This repository
  also uses fuzz tests and benchmarks in several `internal` packages; preserve
  those when changing parsers, protocol handlers, collectors, or hot paths.
- Format Go changes with `gofmt`; CI treats any output from `gofmt -l .` as a
  failure. Update the relevant `docs/` or README material when changing CLI,
  configuration, API, deployment, or operational behavior.

## Useful entry points

- CLI command definitions and wiring: `cmd/`
- Shared models: `pkg/model/`
- Configuration/defaults: `internal/config/`
- Collection orchestration: `internal/collector/`
- Persistence and schema: `internal/storage/`
- API/security middleware: `internal/server/`
- Development and extension guidance: `docs/development.md`
- Architecture details: `docs/architecture.md`
- CI expectations: `.github/workflows/ci.yml`
