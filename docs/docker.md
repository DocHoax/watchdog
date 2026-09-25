# Docker Container Monitoring & Diagnostics

Watchdog provides built-in discovery and telemetry for local Docker environments, streaming container health states, CPU/Memory utilization, restart frequencies, and crashloop diagnostics directly to the TUI and Prometheus exporter.

---

## 🐳 Docker Collector Architecture

The Docker collector automatically discovers the local Docker daemon via socket access or CLI fallback:

```
┌────────────────────────┐
│     Docker Daemon      │
│  /var/run/docker.sock  │
└───────────┬────────────┘
            │
    Query Container States
            │
┌───────────▼────────────┐
│ Watchdog Docker Engine │
└───────────┬────────────┘
            │
   ┌────────┴────────┬───────────────────┐
   ▼                 ▼                   ▼
[ TUI Tab 4 ]  [ /metrics Exporter ]  [ CrashLoop Diagnostic ]
 (Containers)    (OpenMetrics)          (docker-health rule)
```

---

## 📊 Collected Container Metrics

When Docker is running, Watchdog samples:

- **Daemon Metadata**: Engine version, API version, and total image counts.
- **Container Counts**: Total containers, running, paused, and stopped counts.
- **Per-Container Telemetry**:
  - Container ID, Name, Image, and entrypoint command.
  - Lifecycle state (`running`, `paused`, `exited`, `dead`).
  - Active port bindings.
  - Live CPU percentage (`CPU%`) and RAM consumption (`Mem%`).
  - Active process count (`PIDs`).

---

## 🖥️ Interactive TUI Container View

Press **`4`** in the Watchdog TUI to enter the dedicated **Containers** tab:

```text
┌─ Containers ────────────────────────────────────────────────────────────────────────┐
│ ID           NAME             IMAGE                   STATUS          CPU%   MEM%   │
│ 3f4a9b1c2d3e redis-cache      redis:7.2-alpine        Up 4 hours      0.4%   1.2%   │
│ 8a7c6e5d4b3a postgres-db      postgres:16-alpine      Up 2 days       1.8%   8.4%   │
│ 1e2d3c4b5a6f web-frontend     nginx:latest            Up 4 hours      0.1%   0.5%   │
│ 9b8a7c6d5e4f queue-worker     app/worker:v1.2.0       Up 30 minutes   4.5%   3.1%   │
└─────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 🩺 Diagnostic Heuristics (`docker-health`)

The diagnostic engine continuously checks container stability:

- **Rule ID**: `docker-health`
- **Category**: `Docker`
- **Name**: `Container Health & Restarts`
- **Logic**: Flags containers experiencing rapid restart cycles ($\ge 5$ restarts) as `CRITICAL` / `FAIL`, indicating application crashloops or out-of-memory (OOM) kills.

---

## 📦 Running Watchdog Inside Docker

To monitor a host system from within a containerized Watchdog instance:

```bash
# Build local container
docker build -t watchdog:v1.0.0 .

# Run container with host metric access
docker run -d \
  --name watchdog \
  --restart unless-stopped \
  --pid host \
  --net host \
  -v /proc:/host/proc:ro \
  -v /sys:/host/sys:ro \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -v ~/.watchdog:/root/.watchdog \
  watchdog:v1.0.0 \
  server --port 9100
```

### Docker Compose Example (`docker-compose.yml`)

```yaml
version: "3.8"

services:
  watchdog:
    image: watchdog:v1.0.0
    container_name: watchdog
    restart: unless-stopped
    pid: "host"
    network_mode: "host"
    volumes:
      - /proc:/host/proc:ro
      - /sys:/host/sys:ro
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - watchdog_data:/root/.watchdog
    command: ["server", "--port", "9100", "--host", "0.0.0.0"]

volumes:
  watchdog_data:
```
