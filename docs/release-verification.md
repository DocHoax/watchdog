# Watchdog Release Verification & Software Supply Chain Guide

> **Target Application**: Watchdog System Monitoring & Diagnostics CLI (`github.com/DocHoax/watchdog`)  
> **Standard Version**: v1.0.0  
> **Supply Chain Standards**: SPDX 2.3, Sigstore Cosign (Keyless OIDC), SLSA Provenance v1 via GitHub Artifact Attestations

---

## 1. Overview & Supply Chain Architecture

Watchdog implements a defense-in-depth software supply chain security framework designed to allow system administrators, DevOps engineers, and security compliance teams to independently verify the authenticity, integrity, and provenance of all released binaries and packages.

Every official release published to [GitHub Releases](https://github.com/DocHoax/watchdog/releases) includes cryptographic signatures, transparency log entries, verifiable build provenance, and standard machine-readable Software Bill of Materials (SBOMs):

```text
                                Official Release Assets
  ┌─────────────────────────────────┬────────────────────────────────────────────────────────┐
  │ Asset Class                     │ File Pattern                                           │
  ├─────────────────────────────────┼────────────────────────────────────────────────────────┤
  │ Release Archives                │ watchdog_<version>_<os>_<arch>.(tar.gz|zip)            │
  │ Distribution Packages           │ watchdog_<version>_<os>_<arch>.(deb|rpm|apk)           │
  │ SHA-256 Digest Manifest         │ checksums.txt                                          │
  │ Sigstore Signature Bundle       │ checksums.txt.sigstore.json                            │
  │ SPDX 2.3 SBOM Documents         │ <archive-or-package-filename>.sbom.json                │
  │ GitHub SLSA Build Provenance    │ Verifiable via `gh attestation verify`                 │
  │ GitHub SPDX SBOM Attestation    │ Verifiable via `gh attestation verify --predicate-type`│
  └─────────────────────────────────┴────────────────────────────────────────────────────────┘
```

---

## 2. Quickstart: Downloading Release Artifacts

You can download release artifacts using your web browser, `curl`, or the official GitHub CLI (`gh`):

```bash
# Set release version
export WATCHDOG_VERSION="1.0.0"

# Using GitHub CLI (downloads archive, checksums, sigstore bundle, and SBOM)
gh release download "v${WATCHDOG_VERSION}" --repo DocHoax/watchdog

# OR using curl (example for Linux AMD64)
curl -sSLO "https://github.com/DocHoax/watchdog/releases/download/v${WATCHDOG_VERSION}/watchdog_${WATCHDOG_VERSION}_linux_amd64.tar.gz"
curl -sSLO "https://github.com/DocHoax/watchdog/releases/download/v${WATCHDOG_VERSION}/checksums.txt"
curl -sSLO "https://github.com/DocHoax/watchdog/releases/download/v${WATCHDOG_VERSION}/checksums.txt.sigstore.json"
curl -sSLO "https://github.com/DocHoax/watchdog/releases/download/v${WATCHDOG_VERSION}/watchdog_${WATCHDOG_VERSION}_linux_amd64.tar.gz.sbom.json"
```

---

## 3. Step 1: Checksum Manifest Verification

The first layer of validation ensures that downloaded archives have not suffered bit rot, truncation, or accidental corruption during download.

### Linux / POSIX (GNU coreutils)
```bash
# Verify downloaded archive against checksums.txt
sha256sum -c checksums.txt --ignore-missing
```

### macOS (BSD / shasum)
```bash
# Verify downloaded archive against checksums.txt
shasum -a 256 -c checksums.txt
```

### Windows (PowerShell)
```powershell
# Verify SHA-256 hash in PowerShell
$hash = (Get-FileHash -Path ".\watchdog_1.0.0_windows_amd64.zip" -Algorithm SHA256).Hash.ToLower()
$expected = (Get-Content ".\checksums.txt" | Select-String "watchdog_1.0.0_windows_amd64.zip").Line.Split(" ")[0].ToLower()

if ($hash -eq $expected) {
    Write-Host "Checksum verification PASSED" -ForegroundColor Green
} else {
    Write-Error "Checksum verification FAILED: $hash != $expected"
}
```

---

## 4. Step 2: Keyless Sigstore Signature Verification (Cosign)

Watchdog signs `checksums.txt` using **keyless Sigstore signing** via GitHub Actions OpenID Connect (OIDC). This eliminates the risk of compromised or leaked static private keys by utilizing short-lived cryptographic certificates issued by the Sigstore Fulcio Certificate Authority and immutably recorded in the Rekor transparency log.

### Prerequisites
Install the official [Cosign](https://github.com/sigstore/cosign) CLI (v2.2.0 or higher):
```bash
# macOS via Homebrew
brew install cosign

# Linux via Go
go install github.com/sigstore/cosign/v2/cmd/cosign@latest

# Linux via direct binary download
curl -LO https://github.com/sigstore/cosign/releases/latest/download/cosign-linux-amd64
sudo mv cosign-linux-amd64 /usr/local/bin/cosign && sudo chmod +x /usr/local/bin/cosign
```

### Verification Command

Run `cosign verify-blob` against the `checksums.txt` manifest using the accompanying `checksums.txt.sigstore.json` bundle:

```bash
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp "^https://github.com/DocHoax/watchdog/\.github/workflows/release\.yml@refs/tags/v[0-9]+\.[0-9]+\.[0-9]+" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  checksums.txt
```

### Expected Output
```text
Verified OK
```

### Expected Cryptographic Identity Specification
- **OIDC Issuer (`--certificate-oidc-issuer`)**: `https://token.actions.githubusercontent.com`
- **Certificate Subject / SAN (`--certificate-identity`)**: `https://github.com/DocHoax/watchdog/.github/workflows/release.yml@refs/tags/v<VERSION>`
- **Transparency Log**: Immutable public Rekor entry bound to the release workflow execution.

---

## 5. Step 3: GitHub Artifact Build Provenance Verification (`gh attestation`)

Watchdog generates cryptographically signed in-toto SLSA build provenance attestations for all release binaries, tarballs, zip archives, and Linux packages (`.deb`, `.rpm`, `.apk`) directly within GitHub Actions using the `actions/attest-build-provenance` action.

### Prerequisites
Install the [GitHub CLI (`gh`)](https://cli.github.com/) (v2.49.0 or higher).

### Verifying Binary Archive Provenance
```bash
# Verify provenance for Linux tarball
gh attestation verify watchdog_1.0.0_linux_amd64.tar.gz --owner DocHoax

# Verify provenance for macOS tarball
gh attestation verify watchdog_1.0.0_darwin_arm64.tar.gz --owner DocHoax

# Verify provenance for Windows zip archive
gh attestation verify watchdog_1.0.0_windows_amd64.zip --owner DocHoax
```

### Verifying Package Provenance
```bash
# Verify Debian/Ubuntu package provenance
gh attestation verify watchdog_1.0.0_linux_amd64.deb --owner DocHoax

# Verify RPM package provenance
gh attestation verify watchdog_1.0.0_linux_amd64.rpm --owner DocHoax

# Verify Alpine APK package provenance
gh attestation verify watchdog_1.0.0_linux_amd64.apk --owner DocHoax
```

### Verifying Checksum Manifest Provenance
```bash
# Verify provenance of the checksum manifest itself
gh attestation verify checksums.txt --owner DocHoax
```

### Verification Properties Established by Provenance
When `gh attestation verify` succeeds, it establishes:
1. **Repository Identity**: The artifact was compiled exclusively in the official `DocHoax/watchdog` repository.
2. **Release Tag / Commit**: The artifact originated from the exact git tag and commit referenced in the release.
3. **Workflow Integrity**: The artifact was built inside the official `.github/workflows/release.yml` GitHub Actions pipeline.
4. **Non-Repudiation**: The attestation signature is anchored in GitHub's internal transparency log.

---

## 6. Step 4: Software Bill of Materials (SBOM) Inspection & Verification

Every Watchdog binary archive and package is accompanied by a standardized **SPDX 2.3 JSON Software Bill of Materials** generated using Anchore Syft.

### 6.1 Verifying SBOM Attestation
You can verify the cryptographic link between a release artifact and its attested SBOM:

```bash
gh attestation verify watchdog_1.0.0_linux_amd64.tar.gz \
  --owner DocHoax \
  --predicate-type https://spdx.dev/Document
```

### 6.2 Inspecting SBOM Contents with `jq`

The SPDX JSON documents provide complete transparency into direct and transitive Go dependencies, exact semantic versions, and software licenses.

```bash
# Download the SBOM for Linux AMD64
curl -sSLO "https://github.com/DocHoax/watchdog/releases/download/v1.0.0/watchdog_1.0.0_linux_amd64.tar.gz.sbom.json"

# 1. View document metadata and SPDX specification version
jq '{spdxVersion, name, documentNamespace, creationInfo}' watchdog_1.0.0_linux_amd64.tar.gz.sbom.json

# 2. List all packaged Go modules and their resolved versions
jq -r '.packages[] | "\(.name) \(.versionInfo // "N/A")"' watchdog_1.0.0_linux_amd64.tar.gz.sbom.json | sort

# 3. List third-party package licenses declared in the release
jq -r '.packages[] | "\(.name): \(.licenseDeclared // .licenseConcluded // "NOASSERTION")"' watchdog_1.0.0_linux_amd64.tar.gz.sbom.json | sort -u

# 4. Count total components in the release artifact
jq '.packages | length' watchdog_1.0.0_linux_amd64.tar.gz.sbom.json
```

### 6.3 Inspecting SBOM with Syft CLI
If you have [Syft](https://github.com/anchore/syft) installed, you can analyze the binary archive or inspect the SBOM directly:

```bash
# Tabular dependency report from archive
syft watchdog_1.0.0_linux_amd64.tar.gz

# Compare local inspection against released SPDX document
syft watchdog_1.0.0_linux_amd64.tar.gz -o spdx-json=local.sbom.json
```

---

## 7. Step 5: Build Reproducibility Verification

Watchdog is engineered for **deterministic, reproducible compilation**:
- Pinned Go 1.22 toolchain (`golang:1.22-alpine` / GitHub Actions `setup-go` pinned to `1.22`).
- Strict Zero-CGO pure Go compilation (`CGO_ENABLED=0`).
- Build path trimming (`-trimpath`) to strip absolute host filesystem paths from symbol tables and runtime panic traces.
- Deterministic linker flags (`-s -w` and controlled metadata injection).

### Running Independent Reproducibility Verification

You can independently verify that two clean builds produced from the same source code yield bit-for-bit identical binaries:

#### Linux / macOS (Bash)
```bash
# Clone repository
git clone https://github.com/DocHoax/watchdog.git
cd watchdog

# Run reproducibility verification script
chmod +x scripts/verify-reproducibility.sh
./scripts/verify-reproducibility.sh
```

#### Windows (PowerShell)
```powershell
# Clone repository
git clone https://github.com/DocHoax/watchdog.git
cd watchdog

# Run PowerShell reproducibility verification
& ".\scripts\verify-reproducibility.ps1"
```

### Expected Output
```text
========================================================
 Watchdog Build Reproducibility Verification
========================================================
Project Root: /path/to/watchdog
Build Dir 1:  /tmp/watchdog-repro-1.XXXXXX
Build Dir 2:  /tmp/watchdog-repro-2.XXXXXX
Building Run 1 (CGO_ENABLED=0, -trimpath)...
Building Run 2 (CGO_ENABLED=0, -trimpath)...
Calculating SHA-256 Checksums...
Build 1 Hash: 3024dda98a39f30540dbd1a48a68f381df5807771fa8ae152e5e4c770b8e3a43
Build 2 Hash: 3024dda98a39f30540dbd1a48a68f381df5807771fa8ae152e5e4c770b8e3a43
SUCCESS: Builds are byte-for-byte identical!
Deterministic build verification passed.
```

---

## 8. Trust Model & Security Boundaries

Transparency requires clear boundaries regarding what cryptographic signatures, build provenance, and SBOMs do and do not guarantee:

### What Verification Proves:
1. **Authentic Origin**: The binary was built by GitHub Actions from the canonical `DocHoax/watchdog` repository under the specified release tag.
2. **Tamper Detection**: The artifact has not been modified or replaced since it was compiled and signed in the CI pipeline.
3. **Dependency Transparency**: The SPDX SBOM accurately records the dependency tree and versions embedded in the compiled binary.
4. **Environment Isolation**: Path trimming (`-trimpath`) ensures no sensitive developer or build-host filesystem paths leaked into binary symbols.

### What Verification Does Not Prove:
1. **Absence of Vulnerabilities**: Passing provenance and signature verification does not mean the software is free of security flaws or vulnerabilities.
2. **Upstream Dependency Trustworthiness**: An SBOM identifies dependencies, but operators must still perform vulnerability management against reported CVEs.
3. **Infrastructure Trust Assumption**: Keyless signing and build provenance trust GitHub Actions runners and the Sigstore Public Good infrastructure (Fulcio, Rekor).
