#!/usr/bin/env bash
# Запускает Chrome с --remote-debugging-port=9222 и отдельным --user-data-dir.
# Usage: chrome-launch.sh <URL>
# Env:
#   DRY_RUN=1              — только напечатать команду, не запускать
#   CHROME_CANDIDATES      — двоеточие-разделённый список путей (override дефолтов)
#   CHROME_DEBUG_PORT      — порт CDP (default 9222)

set -euo pipefail

URL="${1:-}"
if [[ -z "$URL" ]]; then
  echo "ERROR: укажите URL портала первым аргументом" >&2
  exit 3
fi

PORT="${CHROME_DEBUG_PORT:-9222}"
PROFILE_DIR="${USERPROFILE:-$HOME}/.claude/bugs/chrome-profile"

DEFAULT_CANDIDATES=(
  "/c/Program Files/Google/Chrome/Application/chrome.exe"
  "/c/Program Files (x86)/Google/Chrome/Application/chrome.exe"
  "${LOCALAPPDATA:-}/Google/Chrome/Application/chrome.exe"
)

if [[ -n "${CHROME_CANDIDATES:-}" ]]; then
  IFS=':' read -r -a CANDIDATES <<< "$CHROME_CANDIDATES"
else
  CANDIDATES=("${DEFAULT_CANDIDATES[@]}")
fi

CHROME=""
for c in "${CANDIDATES[@]}"; do
  if [[ -x "$c" ]] || [[ -f "$c" ]]; then
    CHROME="$c"
    break
  fi
done

if [[ -z "$CHROME" ]]; then
  echo "ERROR: chrome.exe не найден. Проверены: ${CANDIDATES[*]}" >&2
  exit 2
fi

mkdir -p "$PROFILE_DIR"

CMD=("$CHROME"
     "--remote-debugging-port=$PORT"
     "--user-data-dir=$PROFILE_DIR"
     "$URL")

if [[ "${DRY_RUN:-0}" == "1" ]]; then
  printf '%s ' "${CMD[@]}"
  echo
  exit 0
fi

nohup "${CMD[@]}" >/dev/null 2>&1 &
echo "Chrome запущен (PID $!), CDP на порту $PORT, профиль: $PROFILE_DIR"
