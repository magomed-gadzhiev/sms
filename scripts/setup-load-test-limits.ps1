# Скрипт для настройки лимитов rate limiting для нагрузочного тестирования
# Использование: .\scripts\setup-load-test-limits.ps1

param(
    [string]$ApiKey = "test-api-key",
    [int]$PerSecond = 10000,
    [int]$PerMinute = 600000,
    [int]$PerHour = 36000000
)

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Настройка лимитов для нагрузочного тестирования" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Проверка Docker
if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    Write-Host "Ошибка: Docker не установлен" -ForegroundColor Red
    exit 1
}

# Проверка доступности PostgreSQL
$postgresRunning = docker ps --format "{{.Names}}" | Select-String -Pattern "postgres"
if (-not $postgresRunning) {
    Write-Host "Ошибка: PostgreSQL контейнер не запущен" -ForegroundColor Red
    Write-Host "Запустите сервисы: .\scripts\start.ps1" -ForegroundColor Yellow
    exit 1
}

# Проверка доступности Redis
$redisRunning = docker ps --format "{{.Names}}" | Select-String -Pattern "redis"
if (-not $redisRunning) {
    Write-Host "Предупреждение: Redis контейнер не запущен" -ForegroundColor Yellow
    Write-Host "Rate limiting может не работать без Redis" -ForegroundColor Yellow
}

Write-Host "Параметры:" -ForegroundColor Cyan
Write-Host "  API Key: $ApiKey"
Write-Host "  Лимит в секунду: $PerSecond"
Write-Host "  Лимит в минуту: $PerMinute"
Write-Host "  Лимит в час: $PerHour"
Write-Host ""

# Обновление лимитов в PostgreSQL
Write-Host "Обновление лимитов в базе данных..." -ForegroundColor Yellow
$updateQuery = "UPDATE clients SET rate_limit_per_second = $PerSecond, rate_limit_per_minute = $PerMinute, rate_limit_per_hour = $PerHour WHERE api_key = '$ApiKey';"
$result = docker exec postgres psql -U smpp -d smpp_db -c $updateQuery 2>&1

if ($LASTEXITCODE -ne 0) {
    Write-Host "Ошибка при обновлении лимитов в БД:" -ForegroundColor Red
    Write-Host $result -ForegroundColor Red
    exit 1
}

# Проверка результата
$checkQuery = "SELECT name, api_key, rate_limit_per_second, rate_limit_per_minute, rate_limit_per_hour FROM clients WHERE api_key = '$ApiKey';"
$checkResult = docker exec postgres psql -U smpp -d smpp_db -c $checkQuery 2>&1

if ($checkResult -match "0 rows") {
    Write-Host "Предупреждение: Клиент с API ключом '$ApiKey' не найден в базе данных" -ForegroundColor Yellow
    Write-Host "Создайте клиента перед запуском нагрузочного тестирования" -ForegroundColor Yellow
} else {
    Write-Host "Лимиты успешно обновлены:" -ForegroundColor Green
    Write-Host $checkResult
}

# Очистка счетчиков в Redis (если доступен)
if ($redisRunning) {
    Write-Host ""
    Write-Host "Очистка счетчиков rate limiting в Redis..." -ForegroundColor Yellow
    $flushResult = docker exec redis redis-cli FLUSHDB 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Счетчики Redis очищены" -ForegroundColor Green
    } else {
        Write-Host "Предупреждение: Не удалось очистить Redis" -ForegroundColor Yellow
        Write-Host $flushResult -ForegroundColor Yellow
    }
}

Write-Host ""
Write-Host "Настройка завершена! Теперь можно запускать нагрузочное тестирование." -ForegroundColor Green
Write-Host "Команда: .\scripts\load-test.ps1" -ForegroundColor Cyan
