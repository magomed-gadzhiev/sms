# Запуск Playwright-тестов e2e в Docker-контейнере.
# Windows-совместимая альтернатива `ssh -f -N -L 8083:localhost:18085 sms-server && npx playwright test`.
#
# Что делает:
#   1. Поднимает SSH-туннель 8083→sms-server:18085 внутри контейнера (ключи из ~/.ssh хоста)
#   2. Прогоняет playwright test с аргументами, переданными скрипту
#   3. По завершении контейнер + туннель удаляются (--rm)
#
# Требования:
#   - Docker Desktop запущен
#   - ~/.ssh/config содержит alias "sms-server" с путём к private key
#   - Private key читается контейнером (монтируется :ro)
#
# Примеры:
#   .\scripts\run-tests.ps1 tests/network-stats/
#   .\scripts\run-tests.ps1 tests/network-stats/filters-core.spec.ts --reporter=list
#   .\scripts\run-tests.ps1 --list    # без прогона, только листинг
#
# Переменные окружения (перехватываются):
#   HEADED=1    — запустить в headed-режиме (требует X11, обычно не нужно)

param(
    [Parameter(ValueFromRemainingArguments=$true)]
    [string[]]$PlaywrightArgs
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$e2eDir = Split-Path -Parent $scriptDir
$repoRoot = Split-Path -Parent $e2eDir

$image = "mcr.microsoft.com/playwright:v1.52.0-jammy"
$sshDir = Join-Path $HOME ".ssh"

if (-not (Test-Path $sshDir)) {
    Write-Error "SSH directory not found at $sshDir. Expected SSH config + key for 'sms-server' alias."
    exit 1
}

# Нормализуем пути к Unix-формату для Docker volume mount
$repoMount = ($repoRoot -replace '\\', '/')
$sshMount = ($sshDir -replace '\\', '/')

$argsString = $PlaywrightArgs -join ' '
if ([string]::IsNullOrWhiteSpace($argsString)) {
    $argsString = "tests/network-stats/"
}

# Команда внутри контейнера:
#  - копируем ключи в /root/.ssh с правильными правами (volume :ro не даёт chmod,
#    поэтому копируем в rw-путь перед использованием ssh)
#  - поднимаем tunnel в фоне (-fN)
#  - небольшая пауза чтобы туннель успел установиться
#  - npx playwright test с переданными аргументами
#  - сохраняем exit-code playwright'а
$containerCmd = @"
set -e
mkdir -p /root/.ssh
cp -r /ssh-ro/. /root/.ssh/
chmod 700 /root/.ssh
chmod 600 /root/.ssh/* 2>/dev/null || true
chmod 644 /root/.ssh/*.pub 2>/dev/null || true
chmod 644 /root/.ssh/known_hosts 2>/dev/null || true
chmod 644 /root/.ssh/config 2>/dev/null || true

echo "[tunnel] opening ssh -fN -L 8083:localhost:18085 sms-server..."
ssh -fN -o StrictHostKeyChecking=accept-new -L 0.0.0.0:8083:localhost:18085 sms-server
sleep 2

cd /work/e2e
echo "[playwright] npx playwright test $argsString"
npx playwright test $argsString
"@

Write-Host "[docker] image: $image"
Write-Host "[docker] repo:  $repoMount"
Write-Host "[docker] ssh:   $sshMount (ro)"
Write-Host ""

docker run --rm `
    -v "${repoMount}:/work" `
    -v "${sshMount}:/ssh-ro:ro" `
    -w /work/e2e `
    $image `
    bash -lc $containerCmd

exit $LASTEXITCODE
