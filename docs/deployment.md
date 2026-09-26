# Watchdog Deployment Guide

Watchdog can be deployed as a standalone binary, a Docker container, a systemd service, or a Kubernetes DaemonSet.

> **Security Note:** Watchdog enforces that any non-loopback bind address (`0.0.0.0`, LAN IP, etc.)
> requires **both** an authentication token (`--token`) and TLS encryption (`--tls-cert`, `--tls-key`).
> The server refuses to start without them, printing an actionable error message. Localhost-only
> deployments (`127.0.0.1`, `::1`, `localhost`) work without token or TLS for development convenience.

---

## 1. Standalone Binary Installation

Download the pre-compiled binary for your operating system and architecture from GitHub Releases (see [Release Verification Guide](release-verification.md) to verify signatures and provenance):

```bash
# Example for Linux AMD64
curl -sSL https://github.com/DocHoax/watchdog/releases/download/v1.0.0/watchdog_1.0.0_linux_amd64.tar.gz | tar -xz
sudo mv watchdog /usr/local/bin/
sudo chmod +x /usr/local/bin/watchdog

# Verify installation
watchdog version
```

### Configuration Templates
Watchdog provides curated configuration templates in the `examples/` directory for common deployment archetypes:
- `examples/development.yaml`: Local developer workstations (1s refresh, loopback only, local SQLite storage).
- `examples/production.yaml`: High-availability bare-metal servers & VMs (5s refresh, WAL storage, 30d retention).
- `examples/server.yaml`: Centralized Prometheus exporter & REST API server daemon with TLS and token files.
- `examples/agent.yaml`: Headless lightweight node telemetry agent.

### Local Development (no token or TLS required)
```bash
# Start on localhost using the development template
watchdog --config examples/development.yaml server --port 8443
```

### Network-Exposed Deployment (token + TLS required)
```bash
# Generate or obtain a TLS certificate (e.g. from your CA or Let's Encrypt)
# Then start with all security requirements using production config:
watchdog --config /etc/watchdog/server.yaml server \
  --host 0.0.0.0 \
  --port 8443 \
  --token-file /etc/watchdog/token \
  --tls-cert /etc/ssl/watchdog/cert.pem \
  --tls-key /etc/ssl/watchdog/key.pem
```

---

## 2. Docker Deployment

Run Watchdog as a background container with host metric access:

```bash
docker run -d \
  --name watchdog \
  --restart unless-stopped \
  --pid host \
  --network host \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -v /proc:/host/proc:ro \
  -v /sys:/host/sys:ro \
  -v watchdog-data:/root/.watchdog \
  watchdog:latest server --port 9100
```

By default the Docker image binds to `127.0.0.1` (localhost-only). To expose
on the network, provide token and TLS:

```bash
docker run -d \
  --name watchdog \
  --restart unless-stopped \
  --pid host \
  --network host \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -v /proc:/host/proc:ro \
  -v /sys:/host/sys:ro \
  -v /etc/watchdog/token:/etc/watchdog/token:ro \
  -v /etc/ssl/watchdog:/certs:ro \
  -v watchdog-data:/home/watchdog/.watchdog \
  watchdog:latest server \
    --host 0.0.0.0 \
    --port 9100 \
    --token-file /etc/watchdog/token \
    --tls-cert /certs/cert.pem \
    --tls-key /certs/key.pem
```

---

## 3. Systemd Service Deployment

Create `/etc/systemd/system/watchdog.service`:

```ini
[Unit]
Description=Watchdog Enterprise System Monitoring Agent
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/watchdog server --port 9100
Restart=always
RestartSec=5s
LimitNOFILE=65536

# Security Sandboxing
ProtectSystem=full
ProtectHome=read-only
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

For network-exposed systemd deployments, add token and TLS flags to `ExecStart`:
```ini
ExecStart=/usr/local/bin/watchdog server \
  --host 0.0.0.0 \
  --port 9100 \
  --token-file /etc/watchdog/token \
  --tls-cert /etc/ssl/watchdog/cert.pem \
  --tls-key /etc/ssl/watchdog/key.pem
```

Store the token in a dedicated secret file with `0600` permissions (`/etc/watchdog/token`) or use systemd credential management rather than embedding plaintext credentials directly in unit files.

Enable and start the service:
```bash
sudo systemctl daemon-reload
sudo systemctl enable --now watchdog.service
sudo systemctl status watchdog.service
```

---

## 4. Kubernetes DaemonSet

Deploy Watchdog across all nodes in a cluster to collect node-level metrics and health diagnostics using production manifests located in `deploy/k8s/`:

```bash
# 1. Apply Namespace, ServiceAccount, RBAC roles, and DaemonSet
kubectl apply -f deploy/k8s/configmap.yaml
kubectl apply -f deploy/k8s/secret.yaml
kubectl apply -f deploy/k8s/daemonset.yaml
kubectl apply -f deploy/k8s/service.yaml
kubectl apply -f deploy/k8s/servicemonitor.yaml
```

The DaemonSet manifest (`deploy/k8s/daemonset.yaml`) configures:
- **`hostNetwork: true`**: Required to collect host-level network metrics (RX/TX, dropped packets, errors) and bind to the node's IP for Prometheus scraping.
- **`hostPID: true`**: Required to enumerate and monitor all host processes via `/proc` for zombie detection, process trees, and resource accounting.
- **Liveness Probe (`/healthz`)**: Verifies HTTP server process is running and answering requests on port 9100.
- **Readiness Probe (`/readyz`)**: Verifies metric collectors have completed initial snapshot generation and storage is accessible before receiving traffic.
- **Security Hardening**: `readOnlyRootFilesystem: true`, `runAsNonRoot: true`, `runAsUser: 1000`, `allowPrivilegeEscalation: false`, and `drop: ["ALL"]` capabilities.
- **Resource Bounds**: Configured with explicit requests (`50m` CPU, `32Mi` RAM) and limits (`200m` CPU, `128Mi` RAM).

---

## 5. Health, Liveness & Readiness Probes

Watchdog provides dedicated, unauthenticated HTTP health and readiness probe endpoints conforming to Kubernetes, load balancer, and container orchestrator conventions:

| Endpoint | Probe Type | Success Code | Failure Code | Semantics |
| :--- | :---: | :---: | :---: | :--- |
| `/health`, `/healthz`, `/api/v1/health` | **Liveness** | `200 OK` | — | Confirms HTTP event loop is alive and serving requests. |
| `/ready`, `/readyz`, `/api/v1/ready` | **Readiness** | `200 OK` | `503 Service Unavailable` | Confirms collector initialization, initial snapshot availability, and storage connectivity. |

### Liveness Probe Response
```json
{
  "status": "ok",
  "uptime_seconds": 342.1,
  "timestamp": "2026-09-26T12:00:00Z",
  "version": "1.0.0"
}
```

### Readiness Probe Response (Ready: HTTP 200)
```json
{
  "status": "ready",
  "collectors": "ok",
  "diagnostics": "ok",
  "storage": "ok",
  "timestamp": "2026-09-26T12:00:00Z",
  "version": "1.0.0"
}
```

### Readiness Probe Response (Degraded: HTTP 503)
```json
{
  "status": "not_ready",
  "collectors": "no snapshot collected yet",
  "diagnostics": "ok",
  "storage": "ok",
  "timestamp": "2026-09-26T12:00:00Z",
  "version": "1.0.0"
}
```

---

## 6. Security Audit Logging & Production Retention

Watchdog records structured security audit events for all authentication attempts, server lifecycle transitions, TLS configurations, configuration modifications, and administrative operations.

### Audit Persistence & Volume Management
In containerized (Docker / Kubernetes) deployments, ensure the data directory (default: `~/.watchdog` containing `watchdog.db`) is mounted to a persistent volume so that security audit logs are retained across container lifecycles.

### Automated Retention & Periodic Pruning
- Configure `audit.retention_days` in `config.yaml` (default: 90 days) for automatic retention pruning during background collection cycles.
- Retention cleanup runs automatically based on the configured retention window without requiring manual intervention.

### Auditing & Compliance Operations
- List recent security events:
  ```bash
  watchdog audit list --since 24h
  ```
- Export audit logs to CSV for compliance reviews (with automated formula injection protection):
  ```bash
  watchdog audit export --since 30d --format csv --output /var/log/watchdog/audit_report.csv
  ```
- Query audit events over REST API (requires Bearer token):
  ```bash
  curl -H "Authorization: Bearer <TOKEN>" "https://localhost:8443/api/v1/audit/events?event_type=auth.failure&since=24h"
  ```

