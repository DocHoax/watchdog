# ==============================================================================
# 🐺 Watchdog Soak & Stability Test Runner (Windows PowerShell)
# ==============================================================================
param (
    [int]$DurationSeconds = 30,
    [int]$Port = 19100,
    [string]$HostAddress = "127.0.0.1",
    [string]$Token = "soak-test-token-windows"
)

$ErrorActionPreference = "Stop"

Write-Host "🐺 Starting Watchdog Soak Test for $DurationSeconds seconds on ${HostAddress}:${Port}..." -ForegroundColor Cyan

# 1. Build temporary binary
$binPath = Join-Path $env:TEMP "watchdog-soak.exe"
Write-Host "🔨 Compiling Watchdog binary to $binPath..." -ForegroundColor Yellow
go build -o $binPath ./main.go

# 2. Launch background process
$logFile = Join-Path $env:TEMP "watchdog-soak.log"
$proc = Start-Process -FilePath $binPath -ArgumentList "server", "--port", "$Port", "--host", "$HostAddress", "--token", "$Token" -PassThru -NoNewWindow -RedirectStandardOutput $logFile -RedirectStandardError $logFile

try {
    Start-Sleep -Milliseconds 1500

    $ready = $false
    for ($i = 0; $i -lt 10; $i++) {
        try {
            $resp = Invoke-RestMethod -Uri "http://${HostAddress}:${Port}/health" -TimeoutSec 1
            if ($resp.status -eq "ok") {
                $ready = $true
                break
            }
        } catch {
            Start-Sleep -Milliseconds 500
        }
    }

    if (-not $ready) {
        Write-Host "❌ Failed to start Watchdog server within 5 seconds." -ForegroundColor Red
        if (Test-Path $logFile) {
            Get-Content $logFile
        }
        exit 1
    }

    Write-Host "✅ Server running (PID $($proc.Id)). Starting stress iterations..." -ForegroundColor Green

    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $totalRequests = 0
    $failedRequests = 0
    $authHeader = @{ "Authorization" = "Bearer $Token" }

    while ($sw.Elapsed.TotalSeconds -lt $DurationSeconds) {
        # Health check
        try {
            $null = Invoke-RestMethod -Uri "http://${HostAddress}:${Port}/health" -TimeoutSec 2
            $totalRequests++
        } catch { $failedRequests++ }

        # Metrics
        try {
            $null = Invoke-WebRequest -Uri "http://${HostAddress}:${Port}/metrics" -TimeoutSec 2
            $totalRequests++
        } catch { $failedRequests++ }

        # Protected Snapshot
        try {
            $null = Invoke-RestMethod -Uri "http://${HostAddress}:${Port}/api/v1/snapshot" -Headers $authHeader -TimeoutSec 2
            $totalRequests++
        } catch { $failedRequests++ }

        # Protected Diagnostics
        try {
            $null = Invoke-RestMethod -Uri "http://${HostAddress}:${Port}/api/v1/diagnostics" -Headers $authHeader -TimeoutSec 2
            $totalRequests++
        } catch { $failedRequests++ }

        Start-Sleep -Milliseconds 250
    }

    $sw.Stop()

    Write-Host ""
    Write-Host "==============================================================================" -ForegroundColor Cyan
    Write-Host "📊 Soak Test Completed Successfully!" -ForegroundColor Cyan
    Write-Host "  Duration        : $($sw.Elapsed.TotalSeconds)s"
    Write-Host "  Total Requests  : $totalRequests"
    Write-Host "  Failed Requests : $failedRequests"
    Write-Host "==============================================================================" -ForegroundColor Cyan

    if ($failedRequests -gt 0) {
        Write-Host "❌ Soak test detected request failures!" -ForegroundColor Red
        exit 1
    }

    Write-Host "🎉 Zero failures detected. System stability verified." -ForegroundColor Green
}
catch {
    Write-Host "❌ Error occurred during soak test: $_" -ForegroundColor Red
}
finally {
    if ($proc -and -not $proc.HasExited) {
        Write-Host "🛑 Stopping server process (PID $($proc.Id))..." -ForegroundColor Yellow
        Stop-Process -Id $proc.Id -Force
    }
    if (Test-Path $binPath) {
        Remove-Item $binPath -Force -ErrorAction SilentlyContinue
    }
}
