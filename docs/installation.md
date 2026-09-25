# Installation Guide

Watchdog is distributed as a single static, zero-dependency binary for Linux, macOS, and Windows. It requires no C runtime libraries, no external daemons, and minimal system permissions.

---

## 📦 Package Managers

### Debian / Ubuntu (`.deb`)
```bash
# Download latest .deb release
curl -sLO https://github.com/DocHoax/watchdog/releases/download/v1.0.0/watchdog_1.0.0_linux_amd64.deb

# Install package
sudo dpkg -i watchdog_1.0.0_linux_amd64.deb
```

### RHEL / CentOS / Rocky Linux / Fedora (`.rpm`)
```bash
# Download latest .rpm release
curl -sLO https://github.com/DocHoax/watchdog/releases/download/v1.0.0/watchdog_1.0.0_linux_amd64.rpm

# Install package
sudo rpm -ivh watchdog_1.0.0_linux_amd64.rpm
```

### Alpine Linux (`.apk`)
```bash
# Download latest .apk release
curl -sLO https://github.com/DocHoax/watchdog/releases/download/v1.0.0/watchdog_1.0.0_linux_amd64.apk

# Install package
sudo apk add --allow-untrusted watchdog_1.0.0_linux_amd64.apk
```

---

## 🌐 Precompiled Binary Downloads

Download the appropriate archive for your operating system and CPU architecture from [GitHub Releases](https://github.com/DocHoax/watchdog/releases):

### Linux (AMD64 / ARM64 / ARMv7)
```bash
# Linux AMD64 (x86_64)
curl -sSL https://github.com/DocHoax/watchdog/releases/download/v1.0.0/watchdog_1.0.0_linux_amd64.tar.gz | tar -xz
sudo mv watchdog /usr/local/bin/
sudo chmod +x /usr/local/bin/watchdog

# Linux ARM64 (AWS Graviton, Raspberry Pi 4/5)
curl -sSL https://github.com/DocHoax/watchdog/releases/download/v1.0.0/watchdog_1.0.0_linux_arm64.tar.gz | tar -xz
sudo mv watchdog /usr/local/bin/
sudo chmod +x /usr/local/bin/watchdog
```

### macOS (Apple Silicon & Intel)
```bash
# Apple Silicon (M1/M2/M3/M4)
curl -sSL https://github.com/DocHoax/watchdog/releases/download/v1.0.0/watchdog_1.0.0_darwin_arm64.tar.gz | tar -xz
sudo mv watchdog /usr/local/bin/
sudo chmod +x /usr/local/bin/watchdog

# Intel Mac (x86_64)
curl -sSL https://github.com/DocHoax/watchdog/releases/download/v1.0.0/watchdog_1.0.0_darwin_amd64.tar.gz | tar -xz
sudo mv watchdog /usr/local/bin/
sudo chmod +x /usr/local/bin/watchdog
```

### Windows (PowerShell)
```powershell
# Windows x64 (AMD64) via PowerShell
Invoke-WebRequest -Uri "https://github.com/DocHoax/watchdog/releases/download/v1.0.0/watchdog_1.0.0_windows_amd64.zip" -OutFile "watchdog.zip"
Expand-Archive -Path "watchdog.zip" -DestinationPath "$env:ProgramFiles\Watchdog" -Force
$env:Path += ";$env:ProgramFiles\Watchdog"
```

### Windows (Command Prompt / cmd.exe)
```cmd
:: Windows x64 (AMD64) via Command Prompt
curl.exe -L -o watchdog.zip "https://github.com/DocHoax/watchdog/releases/download/v1.0.0/watchdog_1.0.0_windows_amd64.zip"
tar.exe -xf watchdog.zip
watchdog.exe version
```

---

## ⚡ Go Toolchain Installation

Install Watchdog directly using the official Go module path (requires Go 1.22+):

```bash
# Install latest release binary into $GOPATH/bin (or ~/go/bin)
go install github.com/DocHoax/watchdog@v1.0.0

# Verify installation
watchdog version
```

---

## 🔨 Building From Source

### Prerequisites
- Go 1.22 or higher
- Git

```bash
# Clone the canonical repository
git clone https://github.com/DocHoax/watchdog.git
cd watchdog

# Build static binary with CGO disabled
CGO_ENABLED=0 go build -ldflags="-s -w -X github.com/DocHoax/watchdog/cmd.Version=1.0.0" -o bin/watchdog .

# Verify installation
./bin/watchdog version
```

### Local Go Toolchain Install from Source
```bash
# Clone and install directly to $GOPATH/bin
git clone https://github.com/DocHoax/watchdog.git
cd watchdog
go install .
```

---

## 🐳 Docker Container

Build and run Watchdog inside a Docker container with host-level metrics:
```bash
# Build local container image
docker build -t watchdog:v1.0.0 .

# Run container daemon
docker run -d \
  --name watchdog \
  --restart unless-stopped \
  --pid host \
  --network host \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -v /proc:/host/proc:ro \
  -v /sys:/host/sys:ro \
  -v watchdog-data:/home/watchdog/.watchdog \
  watchdog:v1.0.0 server --port 9100 --host 0.0.0.0
```

---

## 🔍 Verifying Checksums

Every release includes an official `checksums.txt` containing SHA-256 digests. Verify your downloaded archive:

```bash
# Linux
sha256sum -c checksums.txt --ignore-missing

# macOS
shasum -a 256 -c checksums.txt

# Windows (PowerShell)
Get-FileHash -Algorithm SHA256 .\watchdog_1.0.0_windows_amd64.zip
```

---

## 📋 Verifying Version Output

Verify your installed binary and inspect build metadata:

```bash
# Full version and runtime metadata
watchdog version

# Semantic version string only (e.g. for scripts/CI)
watchdog version --short

# Structured JSON metadata
watchdog version --json
```
