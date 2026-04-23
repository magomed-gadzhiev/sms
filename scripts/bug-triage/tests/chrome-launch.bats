#!/usr/bin/env bats

SCRIPT="$BATS_TEST_DIRNAME/../chrome-launch.sh"

@test "chrome-launch: DRY_RUN печатает найденный путь к chrome.exe" {
  # Найти хоть один дефолтный путь; если нет — skip
  if [[ ! -f "/c/Program Files/Google/Chrome/Application/chrome.exe" ]] \
     && [[ ! -f "/c/Program Files (x86)/Google/Chrome/Application/chrome.exe" ]] \
     && [[ ! -f "${LOCALAPPDATA:-}/Google/Chrome/Application/chrome.exe" ]]; then
    skip "Chrome не установлен в дефолтных локациях"
  fi
  run bash -c "DRY_RUN=1 $SCRIPT http://localhost:5173"
  [ "$status" -eq 0 ]
  [[ "$output" == *"chrome.exe"* ]]
  [[ "$output" == *"--remote-debugging-port=9222"* ]]
  [[ "$output" == *"--user-data-dir="* ]]
  [[ "$output" == *"http://localhost:5173"* ]]
}

@test "chrome-launch: exit 2, если chrome.exe не найден" {
  run bash -c "CHROME_CANDIDATES=/nonexistent/chrome.exe DRY_RUN=1 $SCRIPT http://localhost:5173"
  [ "$status" -eq 2 ]
  [[ "$output" == *"не найден"* ]]
}

@test "chrome-launch: exit 3 без URL" {
  run "$SCRIPT"
  [ "$status" -eq 3 ]
  [[ "$output" == *"URL"* ]]
}
