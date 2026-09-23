# Contributing to Watchdog

Thank you for your interest in contributing to Watchdog! Watchdog is an enterprise-grade, high-performance, cross-platform system monitoring and automated diagnostic CLI built in Go.

---

## Code of Conduct

All contributors and maintainers are expected to abide by our [Code of Conduct](CODE_OF_CONDUCT.md).

---

## Development Setup

### Prerequisites
- **Go 1.21+** (Go 1.22 recommended)
- **Git**
- **golangci-lint** (optional, recommended)

### Getting Started
```bash
# Clone repository
git clone https://github.com/watchdog-cli/watchdog.git
cd watchdog

# Verify dependencies
go mod download
go mod verify

# Run test suite
go test -v -race ./...

# Build binary locally
go build -o bin/watchdog .
```

---

## Development Guidelines

1. **Zero Cgo Policy**: Watchdog core must compile cleanly with `CGO_ENABLED=0` across Linux, macOS, and Windows.
2. **Deterministic Exit Codes**: All CLI error pathways must map to the defined status codes in `cmd/exitcodes.go`.
3. **Graceful Degradation**: Collectors and exporters must never crash or panic when hardware counters, OS APIs, or daemon sockets are unavailable or unprivileged.
4. **Security by Default**:
   - Never log sensitive tokens or secrets.
   - Bind remote interfaces to `127.0.0.1` by default.
   - Enforce authentication and TLS for all remote communications.
5. **Code Style**:
   - Run `gofmt -s -w .` before committing.
   - Run `go vet ./...` to check for suspicious constructs.

---

## Submitting Pull Requests

1. Fork the repository and create your feature branch: `git checkout -b feature/my-feature`.
2. Commit your changes with clear, descriptive commit messages.
3. Ensure all unit and integration tests pass: `go test -race ./...`.
4. Open a Pull Request targeting the `main` branch.
