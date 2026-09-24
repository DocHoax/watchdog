# Developer Guide & Contribution Architecture

This document details the engineering guidelines, build toolchains, testing standards, and extension interfaces for developing and contributing to Watchdog.

---

## 🛠️ Development Environment Setup

### Prerequisites
- **Go 1.22+**: Required for modern loop semantics and runtime improvements.
- **Git**: Version control.
- **golangci-lint** (recommended): For static analysis and linting.

### Clone & Dependencies
```bash
# Clone the repository
git clone https://github.com/watchdog-cli/watchdog.git
cd watchdog

# Download and verify Go modules
go mod download
go mod verify
```

---

## 🏗️ Build & Compilation

Watchdog strictly enforces a **Zero-CGO** architecture policy. All builds must succeed with `CGO_ENABLED=0` across all supported operating systems.

### Local Development Binary
```bash
# Compile development binary
go build -ldflags="-s -w" -o bin/watchdog .

# Verify binary
./bin/watchdog version
```

### Cross-Compilation Targets
```bash
# Linux AMD64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/watchdog-linux-amd64 .

# Linux ARM64
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bin/watchdog-linux-arm64 .

# macOS (Darwin) Apple Silicon
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o bin/watchdog-darwin-arm64 .

# Windows AMD64
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o bin/watchdog-windows-amd64.exe .
```

---

## 🧪 Testing & Quality Assurance

### 1. Unit & Race Detection Tests
Always run race-detector tests before opening pull requests:
```bash
go test -v -race ./...
```

### 2. Static Analysis & Verification
```bash
# Code formatting check
gofmt -l .

# Go vet static checks
go vet ./...
```

### 3. Performance & Allocation Benchmarks
```bash
# Run all benchmark suites with memory allocation profiling
go test -bench=. -benchmem ./internal/...
```

---

## 🔌 Extending Watchdog Subsystems

### 1. Adding a New Metric Collector
All collectors reside in `internal/collector/` and implement the `Collector` interface:

```go
package collector

import "context"

type CustomCollector struct{}

func NewCustomCollector() *CustomCollector {
    return &CustomCollector{}
}

func (c *CustomCollector) Name() string {
    return "custom_subsystem"
}

func (c *CustomCollector) Collect(ctx context.Context) (any, error) {
    // Read platform-specific metrics non-destructively
    return data, nil
}
```

### 2. Adding an Automated Diagnostic Rule
Diagnostic heuristic rules reside in `internal/diagnostics/` and implement the `Rule` interface:

```go
type CustomRule struct{}

func (r *CustomRule) ID() string       { return "custom-health-check" }
func (r *CustomRule) Category() string { return "Custom" }
func (r *CustomRule) Name() string     { return "Custom Subsystem Check" }

func (r *CustomRule) Evaluate(ctx context.Context, snapshot *model.SystemSnapshot, cfg *config.Config) model.DiagnosticResult {
    // Evaluate conditions and return model.DiagnosticResult (Pass, Warn, Fail)
}
```

---

## 📐 Core Engineering Standards

1. **Deterministic Exit Codes**: All CLI errors must wrap an explicit integer exit code defined in `cmd/exitcodes.go`.
2. **Graceful Degradation**: Collectors must never panic when unprivileged or when external subsystems (Docker daemon, Kubernetes cluster) are unavailable.
3. **Safe Memory & Concurrency**: Avoid unbounded buffer growth, protect shared state with `sync.RWMutex`, and respect incoming `context.Context` cancellation signals.
