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

### 7. Audit Logging & Security Events Architecture
Watchdog incorporates a structured, machine-readable, and searchable security audit logging subsystem (`internal/audit`) designed with strict data isolation, zero credential exposure, and denial-of-service resilience:

- **Audited Security Events**:
  - *Authentication*: `auth.success`, `auth.failure`, `auth.missing_credentials`, `auth.invalid_credentials`.
  - *Server Lifecycle*: `server.start`, `server.stop`, `server.start.failure`, `server.shutdown`.
  - *TLS Status*: `tls.enabled`, `tls.disabled`, `tls.configuration.failure`, `tls.certificate.failure`, `tls.private_key.failure`.
  - *Configuration & Security*: `config.loaded`, `config.validation.failure`, `config.changed`, `config.save`, `config.save.failure`, `security.auth.configuration.failure`, `security.tls.configuration.failure`, `security.secret.source.failure`.
  - *Administrative Actions*: `admin.configuration.change`, `admin.server.start`, `admin.server.stop`, `admin.export`, `admin.audit.purge`.
- **Strict Data Exclusion Invariants**:
  - Plaintext tokens, passwords, private keys, `Authorization` headers, session cookies, environment variables, full HTTP request bodies, and full header dumps are **strictly excluded** from all audit records and metadata maps.
  - Ordinary high-frequency metric readings, telemetry scrapes, and health probe samples are explicitly separated and never logged as audit events.
- **Zero Credential Exposure & Actor Masking**:
  - Failed authentication attempts never store or log the provided raw token or header value.
  - Actor identities for invalid tokens are deterministically masked using truncated SHA-256 hashes (`token:sha256:<8-hex-prefix>`) to facilitate attack correlation without credential leakage.
  - Centralized metadata sanitization (`audit.SanitizeMetadata`) recursively redacts sensitive keys and values containing token or password patterns.
- **Flood Throttling & DoS Protection**:
  - An in-memory sliding-window burst throttler (`FloodLimiter`) enforces per-IP rate limits on high-frequency authentication failure events to prevent SQLite database write exhaustion and disk saturation during brute-force attacks.
- **Request ID Correlation**:
  - Incoming HTTP requests are correlated with a unique `X-Request-ID` (validated against `^[a-zA-Z0-9_-]{1,64}$` or auto-generated as UUID v4) propagated across request contexts, response headers, and audit records.
- **CSV Formula Injection Neutralization**:
  - All CSV export routines automatically escape leading formula triggers (`=`, `+`, `-`, `@`, `\t`, `\r`) with a prepended single quote (`'`) to protect spreadsheet operators from formula injection attacks (CSV Injection / DDE).
- **SQLite Storage & Tamper Limitations Disclaimer**:
  - Audit records are persisted locally in embedded SQLite with indexed timestamps, event types, severities, and outcomes, alongside configurable automated retention pruning (`audit.retention_days`).
  - *Transparency Disclaimer*: Local SQLite storage on a host filesystem does not provide cryptographic tamper-proofing, hardware write-once-read-many (WORM) immutability, or digital signatures against privileged local root/administrator tampering. For high-assurance compliance environments, forward audit logs to an external immutable SIEM/centralized log collector.

### 8. Software Supply Chain Security, SBOM, Signing & Provenance
Watchdog incorporates a hardened software supply chain framework across all release artifacts and packages:

- **Keyless Sigstore / Cosign Signing**:
  - Every release signs the official `checksums.txt` SHA-256 digest manifest using keyless Sigstore signing via GitHub Actions OpenID Connect (OIDC).
  - Eliminates long-lived static private keys in the repository. Cryptographic signatures are bound to short-lived X.509 certificates issued by the Sigstore Fulcio Certificate Authority and immutably recorded in the public Rekor transparency log.
  - Verification validates the OIDC issuer (`https://token.actions.githubusercontent.com`) and certificate subject (`https://github.com/DocHoax/watchdog/.github/workflows/release.yml@refs/tags/v<version>`).
- **Cryptographic GitHub Artifact Attestations**:
  - Build provenance for all released binary archives (`.tar.gz`, `.zip`) and Linux distribution packages (`.deb`, `.rpm`, `.apk`) is cryptographically attested using `actions/attest-build-provenance@v2` (SLSA Provenance v1 specification).
  - Users can verify artifact lineage, repository source, commit SHA, and release workflow execution with `gh attestation verify <file> --owner DocHoax`.
- **Standardized SPDX Software Bill of Materials (SBOM)**:
  - Every release archive and package includes a corresponding SPDX 2.3 JSON Software Bill of Materials (`*.sbom.json`) generated with Anchore Syft.
  - Documents catalog direct and transitive Go dependencies, exact semantic versions, module identifiers, and package licenses.
  - SBOMs are cryptographically attested against release subjects via `actions/attest-sbom@v2` (`gh attestation verify --predicate-type https://spdx.dev/Document`).
- **Zero CGO & Build Path Trimming (`-trimpath`)**:
  - All binaries compile in pure Go (`CGO_ENABLED=0`) with `-trimpath` enabled, stripping absolute developer workstation and CI build paths from symbol tables and panic traces.
- **Dependency Integrity Verification**:
  - All CI workflows enforce `go mod verify` to guarantee that downloaded modules match cryptographic hashes committed in `go.sum`.
- **Deterministic Build Reproducibility**:
  - Build workflows and verification scripts (`scripts/verify-reproducibility.sh` / `.ps1`) enable independent validation of bit-for-bit identical binary compilation.
- **Trust Model & Transparency Boundaries**:
  - *What is guaranteed*: Cryptographic proof of origin from `DocHoax/watchdog`, tamper detection for release archives, transparent dependency inventory, and path isolation.
  - *What is not guaranteed*: Signatures and SBOMs do not imply an absence of software vulnerabilities or bugs, nor do they replace proactive security audits and vulnerability monitoring.
  - See [`docs/release-verification.md`](docs/release-verification.md) for full step-by-step verification commands.

### 9. Filesystem and Path Safety
All output file paths for reports, SQLite databases, and exported data are validated against directory traversal attacks.

---

## Defensive & Authorized Scope

Watchdog is intended solely for authorized system administration, defensive observability, DevOps troubleshooting, CTF infrastructure monitoring, and educational research. The maintainers strictly prohibit utilizing Watchdog for unauthorized surveillance, DoS disruption, or payload evasion.
