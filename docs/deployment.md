# Watchdog Deployment Guide

Watchdog can be deployed as a standalone binary, a Docker container, a systemd service, or a Kubernetes DaemonSet.

> **Security Note:** Watchdog enforces that any non-loopback bind address (`0.0.0.0`, LAN IP, etc.)
> requires **both** an authentication token (`--token`) and TLS encryption (`--tls-cert`, `--tls-key`).
> The server refuses to start without them, printing an actionable error message. Localhost-only
> deployments (`127.0.0.1`, `::1`, `localhost`) work without token or TLS for development convenience.

---

## 1. Standalone Binary Installation

Download the pre-compiled binary for your operating system and architecture from GitHub Releases:

```bash
# Example for Linux AMD64
curl -sSL https://github.com/DocHoax/watchdog/releases/download/v1.0.0/watchdog_1.0.0_linux_amd64.tar.gz | tar -xz
sudo mv watchdog /usr/local/bin/
sudo chmod +x /usr/local/bin/watchdog

# Verify installation
watchdog version
```

### Local Development (no token or TLS required)
```bash
# Start on localhost — accessible only from this machine
watchdog server --port 8443
```

### Network-Exposed Deployment (token + TLS required)
```bash
# Generate or obtain a TLS certificate (e.g. from your CA or Let's Encrypt)
# Then start with all security requirements:
watchdog server \
  --host 0.0.0.0 \
  --port 8443 \
  --token "$(cat /etc/watchdog/token)" \
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
  -v /etc/ssl/watchdog:/certs:ro \
  -v watchdog-data:/home/watchdog/.watchdog \
  watchdog:latest server \
    --host 0.0.0.0 \
    --port 9100 \
    --token "$(cat /etc/watchdog/token)" \
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
  --token ${WATCHDOG_TOKEN} \
  --tls-cert /etc/ssl/watchdog/cert.pem \
  --tls-key /etc/ssl/watchdog/key.pem
```

Store the token in a systemd credential or environment file (`EnvironmentFile=/etc/watchdog/env`) rather than embedding it directly in the unit file.

Enable and start the service:
```bash
sudo systemctl daemon-reload
sudo systemctl enable --now watchdog.service
sudo systemctl status watchdog.service
```

---

## 4. Kubernetes DaemonSet

Deploy Watchdog across all nodes in a cluster to collect node-level metrics and health diagnostics:

```bash
kubectl apply -f deploy/k8s/daemonset.yaml
```

The DaemonSet manifest configures:
- **`hostNetwork: true`**: Required to collect host-level network metrics and bind to the node's IP for Prometheus scraping. Without `hostNetwork`, metric collection would only see the pod's virtual network namespace.
- **`hostPID: true`**: Required to enumerate and monitor all host processes via `/proc`.
- **Secret-based token injection**: The authentication token is stored in a Kubernetes Secret (`watchdog-auth`) and injected via environment variable.
- **Security hardening**: `readOnlyRootFilesystem`, `runAsNonRoot`, `runAsUser: 1000`, `allowPrivilegeEscalation: false`, `drop: ["ALL"]` capabilities.

To create the authentication Secret:
```bash
kubectl create secret generic watchdog-auth \
  --namespace=watchdog-system \
  --from-literal=token="$(openssl rand -base64 32)"
```

*(See `deploy/k8s/daemonset.yaml` for complete RBAC, Secret, and volume mount specifications).*
