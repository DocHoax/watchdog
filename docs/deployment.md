# Watchdog Deployment Guide

Watchdog can be deployed as a standalone binary, a Docker container, a systemd service, or a Kubernetes DaemonSet.

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
*(See `deploy/k8s/daemonset.yaml` for complete RBAC and volume mount specifications).*
