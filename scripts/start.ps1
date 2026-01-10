# PowerShell script for starting SMPP Server

param(
    [switch]$Build,
    [switch]$Migrate,
    [switch]$Clean
)

$ErrorActionPreference = "Stop"

Write-Host "=== SMPP Server Startup Script ===" -ForegroundColor Cyan

# Check Docker
Write-Host "`nChecking Docker..." -ForegroundColor Yellow
try {
    $dockerVersion = docker --version
    Write-Host "[OK] Docker installed: $dockerVersion" -ForegroundColor Green
} catch {
    Write-Host "[ERROR] Docker not found. Please install Docker Desktop" -ForegroundColor Red
    exit 1
}

# Check docker-compose
Write-Host "`nChecking docker-compose..." -ForegroundColor Yellow
try {
    $composeVersion = docker-compose --version
    Write-Host "[OK] docker-compose installed: $composeVersion" -ForegroundColor Green
} catch {
    Write-Host "[ERROR] docker-compose not found" -ForegroundColor Red
    exit 1
}

# Change to project root
$scriptPath = Split-Path -Parent $MyInvocation.MyCommand.Path
$projectRoot = Split-Path -Parent $scriptPath
Set-Location $projectRoot

Write-Host "`nWorking directory: $projectRoot" -ForegroundColor Cyan

# Cleanup (if flag is set)
if ($Clean) {
    Write-Host "`nCleaning containers and volumes..." -ForegroundColor Yellow
    docker-compose -f deployments/docker-compose.yml down -v
    Write-Host "[OK] Cleanup completed" -ForegroundColor Green
}

# Start infrastructure services
Write-Host "`nStarting infrastructure services..." -ForegroundColor Yellow
docker-compose -f deployments/docker-compose.yml up -d postgres redis zookeeper kafka

Write-Host "`nWaiting for PostgreSQL to be ready..." -ForegroundColor Yellow
$retries = 30
$ready = $false
for ($i = 0; $i -lt $retries; $i++) {
    try {
        $status = docker-compose -f deployments/docker-compose.yml ps postgres --format json | ConvertFrom-Json
        if ($status.Health -eq "healthy") {
            $ready = $true
            break
        }
    } catch {}
    Start-Sleep -Seconds 2
    Write-Host "." -NoNewline
}
Write-Host ""

if (-not $ready) {
    Write-Host "[ERROR] PostgreSQL did not start" -ForegroundColor Red
    exit 1
}
Write-Host "[OK] PostgreSQL is ready" -ForegroundColor Green

# Apply migrations (if flag is set)
if ($Migrate) {
    Write-Host "`nApplying database migrations..." -ForegroundColor Yellow
    docker run --rm `
        -v "${projectRoot}/migrations:/migrations" `
        --network deployments_smpp-network `
        migrate/migrate `
        -path /migrations `
        -database "postgres://smpp:smpp_password@postgres:5432/smpp_db?sslmode=disable" `
        up
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "[OK] Migrations applied" -ForegroundColor Green
    } else {
        Write-Host "[ERROR] Migration failed" -ForegroundColor Red
        exit 1
    }
}

# Build and start applications
if ($Build) {
    Write-Host "`nBuilding and starting applications..." -ForegroundColor Yellow
    docker-compose -f deployments/docker-compose.yml up -d --build `
        api-gateway-1 api-gateway-2 smpp-server worker-1 worker-2 `
        haproxy prometheus grafana
} else {
    Write-Host "`nStarting applications..." -ForegroundColor Yellow
    docker-compose -f deployments/docker-compose.yml up -d `
        api-gateway-1 api-gateway-2 smpp-server worker-1 worker-2 `
        haproxy prometheus grafana
}

if ($LASTEXITCODE -ne 0) {
    Write-Host "[ERROR] Failed to start applications" -ForegroundColor Red
    exit 1
}

# Check status
Write-Host "`nWaiting for services to start..." -ForegroundColor Yellow
Start-Sleep -Seconds 10

Write-Host "`nService status:" -ForegroundColor Cyan
docker-compose -f deployments/docker-compose.yml ps

Write-Host "`n=== SMPP Server started ===" -ForegroundColor Green
Write-Host "`nAvailable endpoints:" -ForegroundColor Cyan
Write-Host "  HTTP API:    http://localhost:8080" -ForegroundColor White
Write-Host "  gRPC API:    http://localhost:9090" -ForegroundColor White
Write-Host "  SMPP Server: localhost:2775" -ForegroundColor White
Write-Host "  Grafana:     http://localhost:3000 (admin/admin)" -ForegroundColor White
Write-Host "  Prometheus:  http://localhost:9091" -ForegroundColor White
Write-Host "  HAProxy:     http://localhost:8404/stats" -ForegroundColor White

Write-Host "`nTo view logs:" -ForegroundColor Yellow
Write-Host "  docker-compose -f deployments/docker-compose.yml logs -f [service-name]" -ForegroundColor Gray

Write-Host "`nTo stop:" -ForegroundColor Yellow
Write-Host "  .\scripts\stop.ps1" -ForegroundColor Gray
