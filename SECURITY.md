# Security Policy

## Supported Versions

Only the current major and minor release versions of Watchdog receive official security patches and vulnerability updates.

| Version | Supported          |
| :------ | :----------------- |
| 1.0.x   | :white_check_mark: |
| < 1.0   | :x:                |

---

## Reporting a Vulnerability

We take the security of Watchdog and its users seriously. If you believe you have discovered a security vulnerability in Watchdog, please report it responsibly:

1. **Do NOT report security vulnerabilities through public GitHub issues, discussions, or pull requests.**
2. Report via private email to: **security@watchdog-cli.dev** or open a private [GitHub Security Advisory](https://github.com/watchdog-cli/watchdog/security/advisories/new).
3. Include the following details in your report:
   - Affected subsystem / package (e.g. `internal/server`, `internal/storage`, `cmd/dash`)
   - Nature of vulnerability (e.g. authentication bypass, path traversal, denial of service, injection)
   - Step-by-step reproduction steps or minimal Proof of Concept (PoC)
   - Affected operating systems (Linux, macOS, Windows) and architecture
   - Potential impact assessment and recommended remediation or patch

### Response Timeline SLA
- **Initial Acknowledgment**: Within 48 hours.
- **Triage & Severity Assessment**: Within 5 business days.
- **Fix & Coordinated Disclosure**: Typically within 30 to 90 days depending on severity and patch complexity.

---

## Security Architecture & Threat Model

Watchdog is engineered with defense-in-depth and secure-by-default design principles:

### 1. Localhost Loopback Default
By default, the HTTP daemon, Prometheus exporter, and REST API bind strictly to the loopback interface (`127.0.0.1` / `localhost`). Exposing interfaces externally (`0.0.0.0`) requires explicit operator flags (`--host` or `agent.bind_address`).

### 2. Constant-Time Bearer Token Authentication
All authenticated REST API endpoints (`/api/v1/snapshot`, `/api/v1/diagnostics`, `/api/v1/alerts`, `/api/v1/anomalies`) require Bearer token authentication. Validation uses constant-time string comparison (`crypto/subtle.ConstantTimeCompare`) to prevent timing side-channel attacks.

### 3. Native TLS Encryption
Remote agent communication supports native TLS (`--tls-cert` and `--tls-key`), ensuring all telemetry transmitted over untrusted networks is encrypted in transit.

### 4. Non-Destructive Telemetry Collection
Metric sampling operates strictly with read-only system calls, `/proc` filesystem parsing, and native OS APIs. Watchdog does not modify kernel configurations or system state during monitoring. Process termination from the interactive TUI requires interactive confirmation and validates numeric process IDs.

### 5. Credential Sanitization
Watchdog automatically redacts and masks sensitive tokens, database passwords, and API keys from CLI logs, generated reports, and error traces.

### 6. Filesystem and Path Safety
All output file paths for reports, SQLite databases, and exported data are validated against directory traversal attacks.

---

## Defensive & Authorized Scope

Watchdog is intended solely for authorized system administration, defensive observability, DevOps troubleshooting, CTF infrastructure monitoring, and educational research. The maintainers strictly prohibit utilizing Watchdog for unauthorized surveillance, DoS disruption, or payload evasion.
