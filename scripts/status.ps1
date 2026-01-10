# PowerShell script for checking SMPP Server status

$ErrorActionPreference = "Stop"

# Change to project root
$scriptPath = Split-Path -Parent $MyInvocation.MyCommand.Path
$projectRoot = Split-Path -Parent $scriptPath
Set-Location $projectRoot

Write-Host "=== SMPP Server Status ===" -ForegroundColor Cyan

Write-Host "`nContainer status:" -ForegroundColor Yellow
docker-compose -f deployments/docker-compose.yml ps

Write-Host "`nService health:" -ForegroundColor Yellow

# Check HTTP API
Write-Host "`nHTTP API (http://localhost:8080/health):" -NoNewline
try {
    $response = Invoke-WebRequest -Uri "http://localhost:8080/health" -Method GET -TimeoutSec 5 -ErrorAction Stop
    if ($response.StatusCode -eq 200) {
        Write-Host " [OK]" -ForegroundColor Green
    } else {
        Write-Host " [ERROR] Status: $($response.StatusCode)" -ForegroundColor Red
    }
} catch {
    Write-Host " [ERROR] Not available" -ForegroundColor Red
}

# Check SMPP Server
Write-Host "SMPP Server (localhost:2775):" -NoNewline
try {
    $tcpClient = New-Object System.Net.Sockets.TcpClient
    $tcpClient.Connect("localhost", 2775)
    $tcpClient.Close()
    Write-Host " [OK] Listening" -ForegroundColor Green
} catch {
    Write-Host " [ERROR] Not available" -ForegroundColor Red
}

# Check Prometheus
Write-Host "Prometheus (http://localhost:9091):" -NoNewline
try {
    $response = Invoke-WebRequest -Uri "http://localhost:9091/-/healthy" -Method GET -TimeoutSec 5 -ErrorAction Stop
    if ($response.StatusCode -eq 200) {
        Write-Host " [OK]" -ForegroundColor Green
    } else {
        Write-Host " [ERROR] Status: $($response.StatusCode)" -ForegroundColor Red
    }
} catch {
    Write-Host " [ERROR] Not available" -ForegroundColor Red
}

# Check Grafana
Write-Host "Grafana (http://localhost:3000):" -NoNewline
try {
    $response = Invoke-WebRequest -Uri "http://localhost:3000/api/health" -Method GET -TimeoutSec 5 -ErrorAction Stop
    if ($response.StatusCode -eq 200) {
        Write-Host " [OK]" -ForegroundColor Green
    } else {
        Write-Host " [ERROR] Status: $($response.StatusCode)" -ForegroundColor Red
    }
} catch {
    Write-Host " [ERROR] Not available" -ForegroundColor Red
}

Write-Host "`nResource statistics:" -ForegroundColor Yellow
docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}" `
    $(docker-compose -f deployments/docker-compose.yml ps -q)
