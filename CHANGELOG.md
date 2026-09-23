# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [1.0.0] - 2026-09-23

### Added
- **Interactive Terminal UI**: Full-screen Bubble Tea dashboard with real-time graphs, tabbed navigation, vim keybindings, and responsive layout.
- **Cross-Platform Metric Collectors**: Zero-Cgo OS metric collection across Linux, macOS, and Windows for CPU, memory, disk, network, processes, services, and open ports.
- **Automated Diagnostic & Rule Engine**: Automated root cause analysis, severity ranking (`CRITICAL`, `WARNING`, `HEALTHY`), and remediation suggestions.
- **Statistical Anomaly Detection**: Rolling Z-score with EWMA smoothing and hysteresis cooldowns for noisy metric filtering.
- **Zero-Cgo SQLite Storage Engine**: Embedded high-performance metrics database with automated WAL mode, retention pruning, and schema migrations.
- **Prometheus Exporter**: OpenMetrics/Prometheus exposition HTTP endpoint (`/metrics`) with counter and gauge metrics.
- **Remote Agent Daemon**: Authenticated HTTP/gRPC remote agent daemon with token authentication and TLS encryption.
- **Multi-Format Reporting & Export**: Standalone HTML5 reports with embedded dark/light themes, structured JSON schemas, and RFC3339 CSV exports.
- **Container Discovery**: Native Docker engine socket integration and Kubernetes cluster pod health diagnostics.
- **Release Automation**: Multi-architecture GoReleaser matrix generating Linux (`deb`, `rpm`, `apk`, `tar.gz`), macOS, and Windows artifacts.
- **Deterministic CLI Exit Codes**: Industry-standard exit codes (`0` Success, `1` General Error, `2` Usage Error, `3` Config Error, `4` Auth Error, `5` Network Error).
