# Installation Guide

Watchdog is distributed as a single static, zero-dependency binary for Linux, macOS, and Windows. It requires no C runtime libraries, no external daemons, and minimal system permissions.

---

## 📦 Package Managers

### Debian / Ubuntu (`.deb`)
```bash
# Download latest .deb release
curl -sLO https://github.com/watchdog-cli/watchdog/releases/download/v1.0.0-rc.1/watchdog_1.0.0-rc.1_linux_amd64.deb

# Install package
sudo dpkg -i watchdog_1.0.0-rc.1_linux_amd64.deb
```

### RHEL / CentOS / Rocky Linux / Fedora (`.rpm`)
```bash
# Download latest .rpm release
curl -sLO https://github.com/watchdog-cli/watchdog/releases/download/v1.0.0-rc.1/watchdog_1.0.0-rc.1_linux_amd64.rpm

# Install package
sudo rpm -ivh watchdog_1.0.0-rc.1_linux_amd64.rpm
```

### Alpine Linux (`.apk`)
```bash
# Download latest .apk release
curl -sLO https://github.com/watchdog-cli/watchdog/releases/download/v1.0.0-rc.1/watchdog_1.0.0-rc.1_linux_amd64.apk

# Install package
sudo apk add --allow-untrusted watchdog_1.0.0-rc.1_linux_amd64.apk
```

---

## 🌐 Precompiled Binary Downloads

Download the appropriate archive for your operating system and CPU architecture from [GitHub Releases](https://github.com/watchdog-cli/watchdog/releases):

### Linux (AMD64 / ARM64 / ARMv7)
```bash
# Linux AMD64 (x86_64)
curl -sSL https://github.com/watchdog-cli/watchdog/releases/download/v1.0.0-rc.1/watchdog_1.0.0-rc.1_linux_amd64.tar.gz | tar -xz
sudo mv watchdog /usr/local/bin/
sudo chmod +x /usr/local/bin/watchdog

# Linux ARM64 (AWS Graviton, Raspberry Pi 4/5)
curl -sSL https://github.com/watchdog-cli/watchdog/releases/download/v1.0.0-rc.1/watchdog_1.0.0-rc.1_linux_arm64.tar.gz | tar -xz
sudo mv watchdog /usr/local/bin/
sudo chmod +x /usr/local/bin/watchdog
```

### macOS (Apple Silicon & Intel)
```bash
# Apple Silicon (M1/M2/M3/M4)
curl -sSL https://github.com/watchdog-cli/watchdog/releases/download/v1.0.0-rc.1/watchdog_1.0.0-rc.1_darwin_arm64.tar.gz | tar -xz
sudo mv watchdog /usr/local/bin/
sudo chmod +x /usr/local/bin/watchdog

# Intel Mac (x86_64)
curl -sSL https://github.com/watchdog-cli/watchdog/releases/download/v1.0.0-rc.1/watchdog_1.0.0-rc.1_darwin_amd64.tar.gz | tar -xz
sudo mv watchdog /usr/local/bin/
sudo chmod +x /usr/local/bin/watchdog
```

### Windows (PowerShell)
```powershell
# Windows x64 (AMD64)
Invoke-WebRequest -Uri "https://github.com/watchdog-cli/watchdog/releases/download/v1.0.0-rc.1/watchdog_1.0.0-rc.1_windows_amd64.zip" -OutFile "watchdog.zip"
Expand-Archive -Path "watchdog.zip" -DestinationPath "$env:ProgramFiles\Watchdog"
$env:Path += ";$env:ProgramFiles\Watchdog"
```

---

## 🔨 Building From Source

### Prerequisites
- Go 1.22 or higher
- Git

```bash
# Clone the repository
git clone https://github.com/watchdog-cli/watchdog.git
cd watchdog

# Build static binary with CGO disabled
CGO_ENABLED=0 go build -ldflags="-s -w -X github.com/watchdog-cli/watchdog/cmd.Version=1.0.0-rc.1" -o bin/watchdog .

# Verify installation
./bin/watchdog version
```

### Using `go install`
```bash
go install github.com/watchdog-cli/watchdog@latest
```

---

## 🐳 Docker Container

Run Watchdog inside a Docker container with host-level metrics:
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
  watchdog-cli/watchdog:latest server --port 9100 --host 0.0.0.0
```

---

## 🔍 Verifying the Checksums

Every release includes an official `checksums.txt` containing SHA-256 digests. Verify your downloaded archive:

```bash
# Linux
sha256sum -c checksums.txt --ignore-missing

# macOS
shasum -a 256 -c checksums.txt

# Windows (PowerShell)
Get-FileHash -Algorithm SHA256 .\watchdog_1.0.0-rc.1_windows_amd64.zip
```
