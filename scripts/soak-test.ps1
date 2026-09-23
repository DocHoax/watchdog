# ==============================================================================
# Watchdog Soak & Stability Test Runner (Windows PowerShell)
# ==============================================================================
param (
    [int]$DurationSeconds = 30,
    [int]$Port = 19100,
    [string]$HostAddress = "127.0.0.1",
    [string]$Token = "soak-test-token-windows"
)

$ErrorActionPreference = "Stop"

Write-Host "Starting Watchdog Soak Test for $DurationSeconds seconds on ${HostAddress}:${Port}..." -ForegroundColor Cyan

# 1. Build temporary binary
$binPath = Join-Path $env:TEMP "watchdog-soak.exe"
Write-Host "Compiling Watchdog binary to $binPath..." -ForegroundColor Yellow
go build -o $binPath ./main.go

# 2. Launch background process
$logOut = Join-Path $env:TEMP "watchdog-soak-out.log"
$logErr = Join-Path $env:TEMP "watchdog-soak-err.log"
$proc = Start-Process -FilePath $binPath -ArgumentList "server", "--port", "$Port", "--host", "$HostAddress", "--token", "$Token" -PassThru -NoNewWindow -RedirectStandardOutput $logOut -RedirectStandardError $logErr

try {
    Start-Sleep -Milliseconds 1000

    $authHeader = @{ "Authorization" = "Bearer $Token" }
    $ready = $false
    for ($i = 0; $i -lt 15; $i++) {
        try {
            $respHealth = Invoke-RestMethod -Uri "http://${HostAddress}:${Port}/health" -TimeoutSec 1 -UseBasicParsing
            if ($respHealth.status -eq "ok") {
                # Check if first snapshot cycle completed
                try {
                    $respSnap = Invoke-RestMethod -Uri "http://${HostAddress}:${Port}/api/v1/snapshot" -Headers $authHeader -TimeoutSec 1 -UseBasicParsing
                    if ($null -ne $respSnap) {
                        $ready = $true
                        break
                    }
                }
                catch {
                    # Still collecting initial snapshot
                }
            }
        }
        catch {
            # Server starting up
        }
        Start-Sleep -Milliseconds 500
    }

    if (-not $ready) {
        Write-Host "Failed to start and warm up Watchdog server within 8 seconds." -ForegroundColor Red
        if (Test-Path $logOut) {
            Get-Content $logOut
        }
        if (Test-Path $logErr) {
            Get-Content $logErr
        }
        exit 1
    }

    Write-Host "Server running & warmed up (PID $($proc.Id)). Starting stress iterations..." -ForegroundColor Green

    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $totalRequests = 0
    $failedRequests = 0

    while ($sw.Elapsed.TotalSeconds -lt $DurationSeconds) {
        # Health check
        try {
            $null = Invoke-RestMethod -Uri "http://${HostAddress}:${Port}/health" -TimeoutSec 2 -UseBasicParsing
            $totalRequests++
        }
        catch {
            Write-Host "Health failed: $_" -ForegroundColor DarkRed
            $failedRequests++
        }

        # Metrics
        try {
            $null = Invoke-WebRequest -Uri "http://${HostAddress}:${Port}/metrics" -TimeoutSec 2 -UseBasicParsing
            $totalRequests++
        }
        catch {
            Write-Host "Metrics failed: $_" -ForegroundColor DarkRed
            $failedRequests++
        }

        # Protected Snapshot
        try {
            $null = Invoke-RestMethod -Uri "http://${HostAddress}:${Port}/api/v1/snapshot" -Headers $authHeader -TimeoutSec 2 -UseBasicParsing
            $totalRequests++
        }
        catch {
            Write-Host "Snapshot failed: $_" -ForegroundColor DarkRed
            $failedRequests++
        }

        # Protected Diagnostics
        try {
            $null = Invoke-RestMethod -Uri "http://${HostAddress}:${Port}/api/v1/diagnostics" -Headers $authHeader -TimeoutSec 2 -UseBasicParsing
            $totalRequests++
        }
        catch {
            Write-Host "Diagnostics failed: $_" -ForegroundColor DarkRed
            $failedRequests++
        }

        Start-Sleep -Milliseconds 250
    }

    $sw.Stop()

    Write-Host ""
    Write-Host "==============================================================================" -ForegroundColor Cyan
    Write-Host "Soak Test Completed Successfully!" -ForegroundColor Cyan
    Write-Host "  Duration        : $($sw.Elapsed.TotalSeconds)s"
    Write-Host "  Total Requests  : $totalRequests"
    Write-Host "  Failed Requests : $failedRequests"
    Write-Host "==============================================================================" -ForegroundColor Cyan

    if ($failedRequests -gt 0) {
        Write-Host "Soak test detected request failures!" -ForegroundColor Red
        exit 1
    }

    Write-Host "Zero failures detected. System stability verified." -ForegroundColor Green
}
catch {
    Write-Host "Error occurred during soak test: $_" -ForegroundColor Red
}
finally {
    if ($proc -and -not $proc.HasExited) {
        Write-Host "Stopping server process (PID $($proc.Id))..." -ForegroundColor Yellow
        Stop-Process -Id $proc.Id -Force
    }
    if (Test-Path $binPath) {
        Remove-Item $binPath -Force -ErrorAction SilentlyContinue
    }
}
