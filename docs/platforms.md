# Watchdog Cross-Platform Support Matrix & Platform Capabilities

Watchdog is engineered from the ground up to compile into a single standalone, zero-dependency binary with `CGO_ENABLED=0` across all major operating systems and CPU architectures.

---

## 1. Supported Platform Matrix

| Operating System | Architecture (`GOARCH`) | Status | Service Provider | Metrics Backend | Primary Target Environments |
|:---|:---|:---:|:---|:---|:---|
| **Linux** | `amd64` | **Tier 1** | `systemd` / `sysvinit` | `/proc`, `/sys`, `netlink` | Servers, Cloud VMs, Bare-Metal |
| **Linux** | `arm64` | **Tier 1** | `systemd` / `sysvinit` | `/proc`, `/sys`, `netlink` | AWS Graviton, GCP Tau T2A, RPi 4/5 |
| **Linux** | `arm` (`GOARM=7`) | **Tier 2** | `sysvinit` / `systemd` | `/proc`, `/sys` | Embedded IoT, Edge Gateways |
| **macOS (Darwin)** | `arm64` (Apple Silicon) | **Tier 1** | `launchd` | `sysctl`, `vm_stat`, `netstat` | Apple Silicon (M1/M2/M3/M4) Workstations |
| **macOS (Darwin)** | `amd64` (Intel) | **Tier 1** | `launchd` | `sysctl`, `vm_stat`, `netstat` | Intel x86_64 Macs |
| **Windows** | `amd64` (x64) | **Tier 1** | Windows Services (`sc.exe`/WMI) | Win32 APIs, PDH counters | Windows 10/11, Windows Server 2016+ |
| **Windows** | `arm64` | **Tier 1** | Windows Services (`sc.exe`/WMI) | Win32 APIs | Windows on ARM (Snapdragon X Elite) |
| **FreeBSD** | `amd64` | **Tier 2** | Fallback stub | `sysctl`, POSIX shims | FreeBSD 13.x, 14.x appliances, TrueNAS |

---

## 2. Compilation Strategy & Zero-CGO Guarantee

Watchdog avoids all Cgo dependencies:
- **SQLite Engine**: Uses `modernc.org/sqlite`, a pure Go transpilation of the official SQLite C codebase.
- **System Metrics**: Uses pure Go bindings and direct kernel APIs (`/proc`, `sysctl`, `syscall.Syscall` on Windows) via `gopsutil/v3` and standard library runtime calls.
- **TUI Engine**: Lipgloss and Bubbletea run without ncurses or C libraries.

### Cross-Compilation Command Examples
```bash
# Linux x86_64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o watchdog-linux-amd64 .

# Linux ARM64 (AWS Graviton)
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o watchdog-linux-arm64 .

# macOS Apple Silicon
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o watchdog-darwin-arm64 .

# Windows x64
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o watchdog-windows-amd64.exe .

# Windows ARM64
CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -ldflags="-s -w" -o watchdog-windows-arm64.exe .

# FreeBSD x86_64
CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build -ldflags="-s -w" -o watchdog-freebsd-amd64 .
```

---

## 3. Platform-Specific Source Code Organization

Build tags are strictly isolated to prevent platform-specific system types from leaking into shared packages:

| Source File | Build Constraint Tag | Functionality |
|:---|:---|:---|
| `internal/collector/service_windows.go` | `//go:build windows` | Windows service querying via PowerShell / `sc.exe` |
| `internal/collector/service_linux.go` | `//go:build linux` | Linux service querying via `systemctl` / `service` |
| `internal/collector/service_darwin.go` | `//go:build darwin` | macOS service querying via `launchctl` |
| `internal/collector/service_other.go` | `//go:build !windows && !linux && !darwin` | Graceful zero-allocation fallback for FreeBSD/Solaris |

---

## 4. Platform Permission & Security Model

Watchdog runs with least privilege by default:

### 4.1 Linux
- **Non-root user**: Can collect all user-space CPU, memory, disk, network, own process tree, and unprivileged port status.
- **Elevated capabilities (`setcap`)**:
  - `CAP_NET_RAW` / `CAP_NET_ADMIN`: For ICMP ping probes and socket inspection without full root.
  - `CAP_SYS_PTRACE`: For inspecting command line arguments and environment of processes owned by other users.

### 4.2 macOS
- **Non-root user**: Can read system statistics and own processes.
- **Full Process Inspection**: Requires terminal to have standard process management privileges.

### 4.3 Windows
- **Standard User**: Can read CPU, memory, disk, network, and own processes.
- **Administrator**: Required for querying full Windows service states and reading command lines for system-level services.
