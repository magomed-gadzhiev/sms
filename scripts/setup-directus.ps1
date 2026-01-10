# Скрипт настройки коллекций в Directus
#
# Использование:
#   .\scripts\setup-directus.ps1 [-Url http://localhost:8055] [-Email admin@example.com] [-Password admin]

param(
    [string]$Url = $env:DIRECTUS_URL ?? "http://localhost:8055",
    [string]$Email = $env:DIRECTUS_ADMIN_EMAIL ?? "admin@example.com",
    [string]$Password = $env:DIRECTUS_ADMIN_PASSWORD ?? "admin"
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$projectRoot = Split-Path -Parent $scriptDir
Set-Location $projectRoot

Write-Host "=== Настройка коллекций Directus ===" -ForegroundColor Cyan
Write-Host "URL: $Url"
Write-Host "Email: $Email"
Write-Host ""

# Проверка наличия Node.js
try {
    $nodeVersion = node --version
    Write-Host "✓ Node.js найден: $nodeVersion" -ForegroundColor Green
} catch {
    Write-Host "❌ Node.js не найден. Установите Node.js или запустите скрипт в Docker контейнере." -ForegroundColor Red
    exit 1
}

# Запуск скрипта настройки
$env:DIRECTUS_URL = $Url
$env:DIRECTUS_ADMIN_EMAIL = $Email
$env:DIRECTUS_ADMIN_PASSWORD = $Password

node scripts/setup-directus.js `
    --url "$Url" `
    --email "$Email" `
    --password "$Password"

if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Ошибка выполнения скрипта" -ForegroundColor Red
    exit $LASTEXITCODE
}
