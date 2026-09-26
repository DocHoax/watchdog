<#
.SYNOPSIS
    Watchdog Deterministic Build Reproducibility Verification Script (PowerShell)
.DESCRIPTION
    Verifies that independent clean builds with -trimpath and CGO_ENABLED=0 produce identical byte-for-byte binaries.
#>

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$rootDir = (Get-Item $scriptDir).Parent.FullName

$tempDir1 = Join-Path $env:TEMP ("watchdog-repro-1-" + [System.Guid]::NewGuid().ToString("N"))
$tempDir2 = Join-Path $env:TEMP ("watchdog-repro-2-" + [System.Guid]::NewGuid().ToString("N"))

New-Item -ItemType Directory -Path $tempDir1 -Force | Out-Null
New-Item -ItemType Directory -Path $tempDir2 -Force | Out-Null

try {
    Write-Host "========================================================" -ForegroundColor Cyan
    Write-Host " Watchdog Build Reproducibility Verification" -ForegroundColor Cyan
    Write-Host "========================================================" -ForegroundColor Cyan
    Write-Host "Project Root: $rootDir"
    Write-Host "Build Dir 1:  $tempDir1"
    Write-Host "Build Dir 2:  $tempDir2"

    $fixedVersion = "1.0.0-repro"
    $fixedCommit = "0000000000000000000000000000000000000000"
    $fixedDate = "2026-09-26T00:00:00Z"
    $fixedBuiltBy = "repro-verifier"

    $ldflags = "-s -w -X github.com/DocHoax/watchdog/cmd.Version=$fixedVersion -X github.com/DocHoax/watchdog/cmd.GitCommit=$fixedCommit -X github.com/DocHoax/watchdog/cmd.BuildDate=$fixedDate -X github.com/DocHoax/watchdog/cmd.BuiltBy=$fixedBuiltBy"

    Write-Host "Building Run 1 (CGO_ENABLED=0, -trimpath)..."
    $env:CGO_ENABLED = "0"
    $exe1 = Join-Path $tempDir1 "watchdog.exe"
    Push-Location $rootDir
    try {
        & go build -trimpath "-ldflags=$ldflags" -o $exe1 .
    } finally {
        Pop-Location
    }

    Write-Host "Building Run 2 (CGO_ENABLED=0, -trimpath)..."
    $exe2 = Join-Path $tempDir2 "watchdog.exe"
    Push-Location $rootDir
    try {
        & go build -trimpath "-ldflags=$ldflags" -o $exe2 .
    } finally {
        Pop-Location
    }

    Write-Host "Calculating SHA-256 Checksums..."
    $hash1 = (Get-FileHash -Path $exe1 -Algorithm SHA256).Hash.ToLower()
    $hash2 = (Get-FileHash -Path $exe2 -Algorithm SHA256).Hash.ToLower()

    Write-Host "Build 1 Hash: $hash1"
    Write-Host "Build 2 Hash: $hash2"

    if ($hash1 -eq $hash2) {
        Write-Host "SUCCESS: Builds are byte-for-byte identical!" -ForegroundColor Green
        Write-Host "Deterministic build verification passed." -ForegroundColor Green
        exit 0
    } else {
        Write-Error "FAILURE: Builds produced mismatched binary outputs."
        exit 1
    }
} finally {
    if (Test-Path $tempDir1) { Remove-Item -Recurse -Force $tempDir1 }
    if (Test-Path $tempDir2) { Remove-Item -Recurse -Force $tempDir2 }
}
