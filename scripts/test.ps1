# Скрипт для запуска тестов (PowerShell для Windows)

param(
    [switch]$Integration,
    [switch]$Load,
    [switch]$Coverage
)

Write-Host "Running tests..." -ForegroundColor Green

# Unit тесты
Write-Host "Running unit tests..." -ForegroundColor Yellow
$testArgs = @("-v", "-race")
if ($Coverage) {
    $testArgs += @("-coverprofile=coverage.out")
}
$testArgs += @("./internal/...")

& go test @testArgs

# Показываем coverage
if ($Coverage -and (Test-Path "coverage.out")) {
    Write-Host "Coverage report:" -ForegroundColor Yellow
    & go tool cover -func=coverage.out | Select-Object -Last 1
    Write-Host "Generating HTML coverage report..." -ForegroundColor Yellow
    & go tool cover -html=coverage.out -o coverage.html
}

# Integration тесты
if ($Integration) {
    Write-Host "Running integration tests..." -ForegroundColor Yellow
    & go test -v -tags=integration ./test/integration/...
}

# Load тесты
if ($Load) {
    Write-Host "Running load tests..." -ForegroundColor Yellow
    & go test -v -tags=load ./test/load/...
}

Write-Host "Tests completed!" -ForegroundColor Green
