# Security Policy

## Supported Versions

Only the latest major/minor release of Watchdog receives security patches and updates.

| Version | Supported          |
| ------- | ------------------ |
| 1.0.x   | :white_check_mark: |
| < 1.0   | :x:                |

---

## Reporting a Vulnerability

We take the security of Watchdog and its users seriously. If you believe you have discovered a security vulnerability in Watchdog, please report it responsibly:

1. **Do NOT report security vulnerabilities through public GitHub issues, discussions, or pull requests.**
2. Please send an email detailing your discovery to: **security@watchdog-cli.dev** (or open a private GitHub Security Advisory).
3. Include the following details in your advisory:
   - Component / package affected (e.g., `internal/server`, `internal/storage`, `cmd/dash`)
   - Type of vulnerability (e.g., authentication bypass, injection, directory traversal, DoS)
   - Step-by-step reproduction instructions or a minimal Proof of Concept (PoC)
   - Affected platforms (Linux, macOS, Windows) and Go version
   - Potential impact and recommended mitigation or patch

### Response Timeline
- **Initial Acknowledgment**: Within 48 hours.
- **Triage & Assessment**: Within 5 business days.
- **Fix & Disclosure Coordination**: We coordinate a mutual disclosure timeline, typically 30 to 90 days depending on severity.

---

## Security Architecture & Threat Model

Watchdog is designed according to defense-in-depth and secure-by-default engineering principles:

### 1. Local-First & Localhost Binding
- By default, the Watchdog HTTP API and Prometheus exporter bind strictly to loopback interfaces (`127.0.0.1` / `localhost`).
- Remote network exposure (`0.0.0.0`) must be explicitly enabled by the operator via `--host` or `agent.bind_address`.

### 2. Bearer Token Authentication
- The REST API endpoints (`/api/v1/snapshot`, `/api/v1/diagnostics`, `/api/v1/alerts`, `/api/v1/anomalies`) support mandatory Bearer token authentication (`Authorization: Bearer <token>` or `X-Watchdog-Token: <token>`).
- Token validation uses constant-time string comparison (`crypto/subtle.ConstantTimeCompare`) to mitigate side-channel timing attacks.

### 3. TLS Encryption
- Remote agent communications support native TLS (`--tls-cert` and `--tls-key`), ensuring all telemetry over untrusted networks is encrypted in transit.

### 4. Non-Destructive Telemetry Collection
- Metric sampling operates with read-only system calls, `/proc` inspection, and native platform APIs.
- Watchdog does not alter system state during standard metric collection.
- Process termination from the interactive TUI requires explicit confirmation and strictly validates numeric process identifiers (PIDs).

### 5. Secrets and Credential Sanitization
- Watchdog never logs passwords, API keys, private tokens, or sensitive command-line parameters in log outputs or report files.
- Configuration loading masks sensitive values in debug logs.

### 6. Filesystem and Path Safety
- Destination paths for reports, SQLite databases, and exported files are checked to prevent arbitrary file overwrites and path traversal attacks.

---

## Defensive & Authorized Scope

Watchdog is intended solely for authorized system administration, defensive observability, DevOps troubleshooting, CTF infrastructure monitoring, and educational research. The maintainers strictly prohibit utilizing Watchdog for unauthorized surveillance, DoS disruption, or payload evasion.
