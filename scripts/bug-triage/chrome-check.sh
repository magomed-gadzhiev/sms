#!/usr/bin/env bash
# Проверяет, слушает ли Chrome на порту CDP (по умолчанию 9222).
# Exit 0 = слушает, Exit 1 = нет.
#
# Env:
#   CHROME_DEBUG_PORT — порт (default 9222)

set -euo pipefail

PORT="${CHROME_DEBUG_PORT:-9222}"

if curl -s -m 2 "http://localhost:${PORT}/json/version" >/dev/null 2>&1; then
  echo "OK: Chrome CDP отвечает на ${PORT}"
  exit 0
else
  echo "FAIL: порт ${PORT} не отвечает на /json/version"
  exit 1
fi
