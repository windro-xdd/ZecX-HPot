# Build and run the demo, scrape /metrics, then stop the demo.
# Usage: Open PowerShell in the repo root and run: .\tools\demo\smoke.ps1

$ErrorActionPreference = 'Stop'

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$demoGo = Join-Path $scriptDir 'demo.go'
$demoBin = Join-Path $scriptDir 'demo.exe'

Write-Host "Building demo..."
go build -o "$demoBin" "$demoGo"

Write-Host "Starting demo (background)..."
$proc = Start-Process -FilePath "$demoBin" -PassThru
Start-Sleep -Seconds 1

$metricsUrl = 'http://localhost:9090/metrics'
Write-Host "Fetching metrics from $metricsUrl"
try {
    $resp = Invoke-WebRequest -Uri $metricsUrl -UseBasicParsing -TimeoutSec 5
    $body = $resp.Content
    if ($body -match 'zecx_covert_queue_enqueued_total' -and $body -match 'zecx_covert_queue_depth') {
        Write-Host "Metrics present: OK"
    } else {
        Write-Host "Metrics endpoint returned, but expected metrics not found."
        Write-Host $body
    }
} catch {
    Write-Host "Failed to fetch metrics: $_"
}

Write-Host "Stopping demo (pid=$($proc.Id))"
Stop-Process -Id $proc.Id -Force
Remove-Item -Path "$demoBin" -ErrorAction SilentlyContinue

Write-Host "Done."
