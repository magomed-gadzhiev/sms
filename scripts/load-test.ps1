# Скрипт для запуска нагрузочного тестирования с помощью k6
# Использование: .\scripts\load-test.ps1 [-BaseUrl <url>] [-ApiKey <key>] [-Duration <duration>] [-VUs <vus>]

param(
    [string]$BaseUrl = "http://localhost:8080",
    [string]$ApiKey = "test-api-key",
    [int]$Duration = 0,  # 0 = использовать настройки из скрипта
    [int]$VUs = 0       # 0 = использовать настройки из скрипта
)

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Нагрузочное тестирование SMPP Server" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Проверка, что Docker запущен
$dockerRunning = docker ps 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "Ошибка: Docker не запущен или недоступен" -ForegroundColor Red
    exit 1
}

# Проверка, что сервисы запущены
Write-Host "Проверка доступности сервисов..." -ForegroundColor Yellow
$services = docker ps --format "{{.Names}}" | Select-String -Pattern "haproxy|api-gateway"
if (-not $services) {
    Write-Host "Предупреждение: Не найдены запущенные сервисы (haproxy, api-gateway)" -ForegroundColor Yellow
    Write-Host "Убедитесь, что сервисы запущены: .\scripts\start.ps1" -ForegroundColor Yellow
    Write-Host ""
}

# Определение URL для Docker сети
$dockerNetwork = "deployments_smpp-network"
$networkExists = docker network ls --format "{{.Name}}" | Select-String -Pattern $dockerNetwork
if ($networkExists) {
    $testUrl = "http://haproxy:8080"
    Write-Host "Используется Docker сеть: $dockerNetwork" -ForegroundColor Green
    Write-Host "URL для тестирования: $testUrl" -ForegroundColor Green
} else {
    $testUrl = $BaseUrl
    Write-Host "Используется локальный URL: $testUrl" -ForegroundColor Green
}

Write-Host ""
Write-Host "Параметры теста:" -ForegroundColor Cyan
Write-Host "  Base URL: $testUrl"
Write-Host "  API Key: $ApiKey"
if ($Duration -gt 0) {
    Write-Host "  Duration: ${Duration}s"
}
if ($VUs -gt 0) {
    Write-Host "  Virtual Users: $VUs"
}
Write-Host ""

# Подготовка команды k6
$scriptPath = Join-Path $PSScriptRoot "k6_load_test.js"
$absoluteScriptPath = Resolve-Path $scriptPath

$k6Cmd = "docker run --rm"
if ($networkExists) {
    $k6Cmd += " --network $dockerNetwork"
}
$k6Cmd += " -v `"${absoluteScriptPath}:`/scripts/k6_load_test.js:ro`""
$k6Cmd += " grafana/k6 run"
$k6Cmd += " /scripts/k6_load_test.js"
$k6Cmd += " -e BASE_URL=$testUrl"
$k6Cmd += " -e API_KEY=$ApiKey"

if ($Duration -gt 0) {
    $k6Cmd += " --duration ${Duration}s"
}

if ($VUs -gt 0) {
    $k6Cmd += " --vus $VUs"
}

Write-Host "Запуск нагрузочного теста..." -ForegroundColor Yellow
Write-Host "Команда: $k6Cmd" -ForegroundColor Gray
Write-Host ""

# Запуск теста
Invoke-Expression $k6Cmd

Write-Host ""
Write-Host "Тест завершен!" -ForegroundColor Green
