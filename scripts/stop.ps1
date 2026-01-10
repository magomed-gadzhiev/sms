# PowerShell script for stopping SMPP Server

param(
    [switch]$Clean
)

$ErrorActionPreference = "Stop"

Write-Host "=== Stopping SMPP Server ===" -ForegroundColor Cyan

# Change to project root
$scriptPath = Split-Path -Parent $MyInvocation.MyCommand.Path
$projectRoot = Split-Path -Parent $scriptPath
Set-Location $projectRoot

Write-Host "`nStopping services..." -ForegroundColor Yellow

if ($Clean) {
    Write-Host "Stopping with volume removal (all data will be deleted)..." -ForegroundColor Red
    docker-compose -f deployments/docker-compose.yml down -v
} else {
    docker-compose -f deployments/docker-compose.yml down
}

if ($LASTEXITCODE -eq 0) {
    Write-Host "`n[OK] Services stopped" -ForegroundColor Green
} else {
    Write-Host "`n[ERROR] Failed to stop services" -ForegroundColor Red
    exit 1
}
