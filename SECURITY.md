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
2. Report privately by opening a [GitHub Security Advisory](https://github.com/DocHoax/watchdog/security/advisories/new).
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

### 2. Mandatory Auth + TLS for External Binding
Watchdog enforces a security invariant at startup: **binding to a non-loopback address requires both a Bearer authentication token and a complete TLS certificate/key pair**. The server refuses to start if either is missing and prints an actionable error message guiding the operator to provide the missing configuration or switch back to localhost. This validation runs both in the CLI pre-flight (`cmd/server.go`) and again inside `server.Start()` as defense-in-depth.

Incomplete TLS configurations (certificate without key, or vice versa) are always rejected regardless of bind address.

### 3. Constant-Time Bearer Token Authentication
All authenticated REST API endpoints (`/api/v1/snapshot`, `/api/v1/diagnostics`, `/api/v1/alerts`, `/api/v1/anomalies`) require Bearer token authentication when a token is configured. Validation uses constant-time string comparison (`crypto/subtle.ConstantTimeCompare`) to prevent timing side-channel attacks. Tokens can be supplied via the `Authorization: Bearer <token>` header or the `X-Watchdog-Token` custom header.

### 4. Native TLS Encryption
Remote agent communication supports native TLS (`--tls-cert` and `--tls-key`), ensuring all telemetry transmitted over untrusted networks is encrypted in transit. TLS is mandatory for non-loopback deployments (see §2 above).

### 5. Non-Destructive Telemetry Collection
Metric sampling operates strictly with read-only system calls, `/proc` filesystem parsing, and native OS APIs. Watchdog does not modify kernel configurations or system state during monitoring. Process termination from the interactive TUI requires interactive confirmation and validates numeric process IDs.

### 6. Secrets & Credential Handling Architecture
Watchdog enforces strict secret isolation and management practices:
- **Unified Secret Precedence**: Explicit CLI flags (`--token`, `--token-file`) > Secret files (`agent.token_file`, `agent.tls_key_file`, `agent.tls_cert_file`) > Environment variables (`WATCHDOG_AGENT_TOKEN` / `agent.token_env`, `WATCHDOG_AGENT_TLS_KEY`, `WATCHDOG_AGENT_TLS_CERT`) > Plain configuration fields (`agent.token`, `agent.tls_key`, `agent.tls_cert`).
- **Platform-Aware Secret File Validation**: Secret files are checked for existence, readability, non-emptiness, and whitespace trimming. On POSIX systems, file permissions are validated to warn or error on overly permissive file modes (e.g. group/world readable permissions).
- **Configuration Redaction**: `watchdog config show` and structured outputs automatically mask sensitive fields (`Token`, `TLSKey`) as `"[REDACTED]"`.
- **Serialization Protection**: `Config.Save()` never writes resolved plaintext secrets back to disk if they originated from secret files or environment variables, preventing unintended secret persistence.
- **CI/CD Secret Scanning**: All commits and pull requests are continuously audited via automated Gitleaks secret scanning.

### 7. Filesystem and Path Safety
All output file paths for reports, SQLite databases, and exported data are validated against directory traversal attacks.

---

## Defensive & Authorized Scope

Watchdog is intended solely for authorized system administration, defensive observability, DevOps troubleshooting, CTF infrastructure monitoring, and educational research. The maintainers strictly prohibit utilizing Watchdog for unauthorized surveillance, DoS disruption, or payload evasion.
