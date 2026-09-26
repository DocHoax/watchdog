# Troubleshooting & Diagnostic Guide

This guide covers common operational challenges, permission requirements, platform-specific edge cases, and deterministic CLI exit code definitions for Watchdog.

---

## 🚦 Deterministic CLI Exit Codes

Watchdog returns strict, deterministic process exit codes to facilitate reliable integration into shell scripts, automation runners, and CI/CD pipelines:

| Exit Code | Constant | Meaning | Common Cause / Action |
| :---: | :--- | :--- | :--- |
| **`0`** | `ExitSuccess` | Clean execution | Operation succeeded; all diagnostic health checks passed. |
| **`1`** | `ExitGeneralError` | General runtime error | Unhandled runtime panic, critical diagnostic failure, or unrecoverable defect. |
| **`2`** | `ExitUsageError` | Invalid CLI syntax | Unknown flag, missing required argument, or invalid command structure. |
| **`3`** | `ExitConfigError` | Configuration error | YAML parsing error, missing configuration file, or invalid threshold range. |
| **`4`** | `ExitAuthError` | Permission / Auth error | Insufficient OS privileges, unreadable procfs, or invalid Bearer token. |
| **`5`** | `ExitNetworkError` | Network / Connection error | Unreachable remote agent host, DNS lookup failure, or TLS handshake error. |

---

## 🔍 Common Issues & Resolutions

### 1. Permission Denied on Linux Process Telemetry
- **Symptom**: Process memory or file descriptor metrics show `0` or permission errors for processes owned by other users.
- **Cause**: Linux kernel security restrictions (`hidepid` mount option or unprivileged user account).
- **Resolution**:
  - Run Watchdog under an account with elevated capabilities:
    ```bash
    sudo setcap cap_sys_ptrace,cap_dac_read_search+ep $(which watchdog)
    ```
  - Or run with `sudo watchdog diagnose`.

---

### 2. Docker Socket Access Denied
- **Symptom**: `Docker daemon is not running or docker CLI is not installed` even though Docker is active.
- **Cause**: Current user account is not a member of the `docker` system group.
- **Resolution**:
  ```bash
  # Add user to docker group
  sudo usermod -aG docker $USER
  # Apply new group membership
  newgrp docker
  ```

---

### 3. TUI Display Artifacts or Window Sizing Issues
- **Symptom**: Broken terminal borders, misaligned sparkline graphs, or text overlapping.
- **Cause**: Terminal emulator lacks UTF-8 / 256-color support or viewport is smaller than minimum required dimensions (80 columns $\times$ 24 rows).
- **Resolution**:
  - Ensure your terminal emulator supports UTF-8 and truecolor (e.g., Alacritty, WezTerm, iTerm2, Windows Terminal).
  - Set the terminal environment variable:
    ```bash
    export TERM=xterm-256color
    ```
  - Resize the terminal window to at least $80 \times 24$.

---

### 4. SQLite Database Lock Contention
- **Symptom**: `database is locked` error during high-frequency persistence.
- **Cause**: External process accessing `watchdog.db` without WAL mode support or simultaneous writes across multiple processes.
- **Resolution**:
  - Watchdog automatically configures `_pragma=journal_mode(WAL)` and `_pragma=busy_timeout(5000)`. Ensure external querying tools also open SQLite in WAL mode.
  - Check file permissions on `~/.watchdog/watchdog.db` and its accompanying `-wal` and `-shm` files.

---

### 5. Remote TLS Connection Refused
- **Symptom**: `watchdog dash --remote <host>:9100` returns `x509: certificate signed by unknown authority`.
- **Cause**: The remote agent is presenting a self-signed TLS certificate not in the local system trust store.
- **Resolution**:
  - Add the custom CA certificate to your local trust store, or supply the root certificate to the CLI environment.

---

### 6. Diagnostic Remediation Guidance (`--fix`)
- **Symptom**: Diagnostic checks report `FAIL` or `WARN` for system health rules.
- **Resolution**:
  - Run the diagnostic engine with actionable remediation advice displayed for each failing check:
    ```bash
    watchdog diagnose --fix
    ```
  - Review the suggested remediation steps and run the recommended commands to resolve the bottleneck.

---

### 7. Kubernetes Readiness Probe Returning 503 Service Unavailable
- **Symptom**: Pod fails Kubernetes `readinessProbe` with `HTTP probe failed with statuscode: 503` on `/readyz`.
- **Cause**: The server has started but the collector manager is executing its first telemetry sampling cycle and has not yet generated an initial `SystemSnapshot`, or the database ping failed.
- **Resolution**:
  - Verify container resource allocation: if CPU is heavily throttled (`< 50m`), collector initialization may take longer than the probe timeout.
  - Adjust `initialDelaySeconds` in DaemonSet (`deploy/k8s/daemonset.yaml`) from `5` to `10` seconds.
  - Check storage mount permissions if SQLite database persistence is enabled.

---

### 8. HTTP 413 Payload Too Large on REST API
- **Symptom**: REST API requests return HTTP status 413 (`Payload Too Large`).
- **Cause**: Incoming HTTP request body exceeds the server's 1MB (`1,048,576` bytes) payload size limit enforced by `MaxBodySizeMiddleware`.
- **Resolution**:
  - Ensure API client payloads (e.g. diagnostic submissions or config payloads) are within 1MB. Watchdog REST endpoints are read-mostly and do not accept arbitrary bulk uploads.

---

### 9. HTTP 500 Internal Server Error & Panic Recovery
- **Symptom**: REST API returns HTTP status 500 (`{"error": "Internal server error"}`) and logs a stack trace.
- **Cause**: A downstream HTTP handler encountered a panic condition; `PanicRecoveryMiddleware` intercepted the panic, emitted a critical security audit event, and prevented the server process from crashing.
- **Resolution**:
  - Check server stderr/logs for the stack trace and request ID (`X-Request-ID`).
  - Check security audit events for event type `server.panic`:
    ```bash
    watchdog audit list --event-type server.panic --since 1h
    ```
  - Report the stack trace and request ID to the repository issue tracker.

---

### 10. Non-Loopback Server Startup Refused (Security Invariant)
- **Symptom**: `watchdog server --host 0.0.0.0 --port 9100` exits immediately with exit code 4 (`ExitAuthError`) or exit code 3 (`ExitConfigError`).
- **Cause**: Watchdog strictly enforces that binding to any non-loopback interface (`0.0.0.0`, LAN IPs) requires **both** an authentication token and valid TLS certificate and key.
- **Resolution**:
  - For local development, bind to `127.0.0.1` (the default).
  - For production network exposure, supply both token and TLS certificates:
    ```bash
    watchdog server \
      --host 0.0.0.0 \
      --port 9100 \
      --token-file /etc/watchdog/token \
      --tls-cert /etc/ssl/watchdog/cert.pem \
      --tls-key /etc/ssl/watchdog/key.pem
    ```

