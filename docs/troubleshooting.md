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
