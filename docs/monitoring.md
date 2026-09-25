# Real-Time Monitoring & Interactive TUI Guide

Watchdog features an interactive AltScreen terminal user interface (TUI) powered by Charmbracelet's **Bubble Tea** and **Lip Gloss** frameworks.

---

## 🚀 Launching the Dashboard

```bash
# Launch default real-time dashboard (1-second refresh)
watchdog dash

# Launch with 500ms refresh rate
watchdog dash --interval 500ms

# Launch with custom theme
watchdog dash --theme nord
```

---

## 🖥️ Terminal Dashboard Layout

```
┌─[ 🐺 Watchdog v1.0.0 ]───────[ Host: prod-db-01 ]───[ Uptime: 14d 6h 32m ]────────┐
│ [1 Dashboard]  [2 Processes]  [3 Storage/Net]  [4 Services]  [5 Containers]  [6 Diag] │
├───────────────────────────────────────────────────────────────────────────────────────┤
│ CPU Utilization:  42.5% [████████████████░░░░░░░░░░░░]  Load: 1.24  1.45  1.10        │
│ Core 0: [████████░░░░] 38.2%   Core 1: [██████████░░] 46.8%                           │
│ Core 2: [██████░░░░░░] 31.0%   Core 3: [████████████] 54.1%                           │
├───────────────────────────────────────────────────────────────────────────────────────┤
│ Memory: 8.42 GB / 16.00 GB (52.6%) [█████████████░░░░░░░░░░░]  Swap: 0.12 GB (1.5%)    │
├───────────────────────────────────────────────────────────────────────────────────────┤
│ Disk I/O: Read: 1.4 MB/s  Write: 8.2 MB/s    Net: RX: 450 KB/s  TX: 1.2 MB/s          │
│ ┌─────────────────────────────────────────┐  ┌──────────────────────────────────────┐ │
│ │ Network RX: ▄▆█▇▅▃▂ ▄▆█▇▅▃             │  │ Network TX: ▂▄▆█▇▅▃▂ ▄▆█▇▅           │ │
│ └─────────────────────────────────────────┘  └──────────────────────────────────────┘ │
├───────────────────────────────────────────────────────────────────────────────────────┤
│ Active Alerts: 0 firing | Diagnostics: 10/10 rules passing (HEALTHY)                  │
├───────────────────────────────────────────────────────────────────────────────────────┤
│ [Tab/1-6] Switch View  [j/k] Scroll  [/] Filter  [c/m/p] Sort  [k] Kill  [q] Quit     │
└───────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 📑 Dedicated Views (6 Tabs)

### Tab 1: Dashboard (`1`)
- **System Telemetry**: Hostname, OS platform, kernel architecture, uptime, and load averages (1m, 5m, 15m).
- **CPU Gauges**: Overall percentage and per-core progress bars.
- **Memory & Swap**: Physical RAM used/available/cached and swap space pressure.
- **I/O Throughput**: Live disk read/write bandwidth and network RX/TX sparklines.

### Tab 2: Processes (`2`)
- **Real-Time Process Table**: PID, user, CPU %, Memory %, RSS/VMS, status (running, sleeping, zombie), and process command line.
- **Vim Navigation**: Scroll with `j`/`k`, jump to top/bottom with `g`/`G`.
- **Sorting Modes**: Instant column sorting by CPU (`c`), Memory (`m`), PID (`p`), or Name (`n`).
- **Fuzzy Search / Filtering**: Press `/` to enter search mode and filter processes by name or PID in real-time.
- **Process Termination**: Select a process row and press `k` to open an interactive signal confirmation dialog (`SIGTERM` / `SIGKILL`).

### Tab 3: Storage & Network (`3`)
- **Filesystem Mounts**: Mount points (`/`, `/home`, `/data`), filesystem type (`ext4`, `apfs`, `NTFS`), total/used/free capacity, and inode exhaustion gauges.
- **Network Interfaces**: Active interfaces (`eth0`, `wlan0`, `en0`), IP addresses, MAC addresses, MTU, packet counters, and drop/error statistics.
- **Listening Ports**: Open TCP/UDP ports, protocol, local bind address, and associated owning PID/process name.

### Tab 4: Services (`4`)
- **Service Statuses**: Native OS service management inspection:
  - **Linux**: `systemd` unit states (active, inactive, failed, reloading).
  - **Windows**: Windows Service Control Manager (SCM) service states.
  - **macOS**: `launchd` daemon and agent statuses.
- **Socket Bindings**: Correlated socket states and listening daemon processes.

### Tab 5: Containers (`5`)
- **Docker / Podman Containers**: Running, paused, and stopped containers, image tags, CPU/Memory consumption, port mappings, and restart counters.
- **Kubernetes Pods**: Discovered pods, container statuses, restarts, node placement, and health states (`Running`, `CrashLoopBackOff`, `Pending`).

### Tab 6: Diagnostics & Alerts (`6`)
- **Heuristic Rule Results**: Status badges (`PASS`, `WARN`, `CRIT`) for 10 system rules.
- **Active Firing Alerts**: Threshold alerts with trigger duration and cooldown tracking.
- **Anomaly Detection Stream**: Real-time statistical Z-score anomalies detected by the EWMA engine.
- **Remediation Commands**: Direct terminal commands suggested to fix detected issues.

---

## ⌨️ Comprehensive Keyboard Shortcuts

| Shortcut | Action | Description |
| :--- | :--- | :--- |
| `1` – `6` | Direct Tab Switch | Jump immediately to Tab 1–6 |
| `Tab` / `Shift+Tab` | Cycle Tabs | Move to next / previous tab |
| `h` / `l` or `←` / `→` | Switch Tabs | Move left / right across tabs |
| `j` / `k` or `↑` / `↓` | Scroll Down / Up | Scroll process list, service table, or diagnostic logs |
| `g` / `G` | Jump to Top / Bottom | Fast navigation to start or end of lists |
| `/` | Search & Filter | Open live filter bar in Process and Container views (Esc to clear) |
| `c` | Sort by CPU | Sort processes descending by CPU utilization |
| `m` | Sort by Memory | Sort processes descending by Memory utilization |
| `p` | Sort by PID | Sort processes numerically by Process ID |
| `n` | Sort by Name | Sort processes alphabetically by process executable name |
| `k` | Terminate Process | Open signal prompt to send `SIGTERM`/`SIGKILL` to selected process |
| `r` | Force Refresh | Immediately trigger full metric collection cycle |
| `Space` | Pause / Resume | Freeze or unfreeze live metric updates |
| `?` | Toggle Help | Show keyboard shortcut cheat sheet modal |
| `q` / `Ctrl+C` | Quit | Gracefully clean up terminal AltScreen buffer and exit |

---

## 🎨 Themes & Customization

Watchdog supports multiple built-in color themes optimized for light and dark terminal backgrounds:

| Theme Name | Description |
| :--- | :--- |
| `default` | Standard high-contrast cyan, green, yellow, and red color palette |
| `dark` | Deep contrast palette tailored for dark terminal emulators |
| `light` | Inverted contrast palette designed for white or light terminal backgrounds |
| `nord` | Arctic, north-bluish palette based on the official Nord color scheme |
| `monokai` | Vibrant developer palette inspired by the Monokai syntax theme |
| `solarized` | Low-contrast precision palette based on Solarized Dark |
| `dracula` | Dark theme featuring vibrant purple and pink accents |

Specify the theme at launch:
```bash
watchdog dash --theme nord
# or set in config.yaml:
# dashboard:
#   theme: "nord"
```
