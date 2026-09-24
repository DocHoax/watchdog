## Description
Provide a concise summary of the changes introduced in this pull request and the motivation behind them.

Fixes #(issue number)

## Type of Change
- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Documentation update
- [ ] Performance improvement / Code refactoring
- [ ] CI / Release engineering

## Subsystems Affected
- [ ] CLI Commands (`cmd/`)
- [ ] Metric Collectors (`internal/collector/`)
- [ ] Terminal User Interface (`internal/tui/`)
- [ ] Diagnostics & Rules Engine (`internal/diagnostics/`)
- [ ] Alerting Engine (`internal/alerts/`)
- [ ] Anomaly Detection (`internal/anomaly/`)
- [ ] Storage & Time-Series Engine (`internal/storage/`)
- [ ] HTTP Server & Prometheus Exporter (`internal/server/`)
- [ ] Reporting Engine (`internal/reporting/`)
- [ ] Configuration (`internal/config/`)

## Verification & Testing
Describe the tests you ran to verify your changes:
- [ ] Unit tests pass: `go test -v -race ./...`
- [ ] Linting & Vet pass: `go vet ./...` and `gofmt -l .`
- [ ] Manual verification on target OS (Linux / macOS / Windows)

## Cross-Platform & Zero-CGO Compatibility
- [ ] Confirmed pure Go / `CGO_ENABLED=0` compatibility across Linux, Darwin, and Windows
- [ ] Handled graceful degradation when unprivileged or when APIs/sockets are unavailable
- [ ] Maintained deterministic exit codes (`cmd/exitcodes.go`)

## Checklist
- [ ] My code follows the code style of this project
- [ ] I have performed a self-review of my own code
- [ ] I have commented my code where necessary, particularly in complex algorithmic areas
- [ ] I have made corresponding changes to the documentation (`docs/` and `README.md`)
- [ ] My changes generate no new warnings or compiler errors
