# Скрипт для запуска performance тестов
# Использование: .\scripts\run-performance-tests.ps1

param(
    [switch]$Integration,
    [switch]$Load,
    [switch]$Performance,
    [switch]$All
)

$ErrorActionPreference = "Stop"

Write-Host "=== Performance Tests Runner ===" -ForegroundColor Green

# Если указан -All или ничего, запускаем все тесты
if ($All -or (-not $Integration -and -not $Load -and -not $Performance)) {
    $Integration = $true
    $Load = $true
    $Performance = $true
}

# Integration тесты
if ($Integration) {
    Write-Host "`n[1/3] Running Integration Tests..." -ForegroundColor Yellow
    go test -tags=integration -v ./test/integration/... -timeout 10m
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Integration tests failed!" -ForegroundColor Red
        exit 1
    }
}

# Performance тесты
if ($Performance) {
    Write-Host "`n[2/3] Running Performance Tests..." -ForegroundColor Yellow
    go test -tags=integration -v ./test/performance/... -timeout 10m
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Performance tests failed!" -ForegroundColor Red
        exit 1
    }
}

# Load тесты
if ($Load) {
    Write-Host "`n[3/3] Running Load Tests (10K msg/s target)..." -ForegroundColor Yellow
    Write-Host "Note: Load tests require running services" -ForegroundColor Cyan
    
    # Проверяем доступность API Gateway
    try {
        $response = Invoke-WebRequest -Uri "http://localhost:8080/health" -TimeoutSec 5 -UseBasicParsing
        if ($response.StatusCode -eq 200) {
            Write-Host "API Gateway is available, starting load tests..." -ForegroundColor Green
            go test -tags=load -v ./test/load/... -timeout 30m
            if ($LASTEXITCODE -ne 0) {
                Write-Host "Load tests failed!" -ForegroundColor Red
                exit 1
            }
        }
    } catch {
        Write-Host "API Gateway not available at http://localhost:8080" -ForegroundColor Yellow
        Write-Host "Skipping load tests. Start services first with: docker-compose up -d" -ForegroundColor Yellow
    }
}

Write-Host "`n=== All tests completed! ===" -ForegroundColor Green

# Опционально: запуск k6 тестов
$runK6 = Read-Host "Run k6 10K load test? (y/n)"
if ($runK6 -eq "y" -or $runK6 -eq "Y") {
    Write-Host "`nRunning k6 10K load test..." -ForegroundColor Yellow
    k6 run scripts/k6_10k_load_test.js
}