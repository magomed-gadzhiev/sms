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

@test "chrome-launch: DRY_RUN с CHROME_CANDIDATES override (portable happy-path)" {
  # Не зависит от наличия реального Chrome — создаём dummy-файл.
  FAKE_CHROME="$(mktemp --suffix=-chrome.exe 2>/dev/null || mktemp)"
  run bash -c "CHROME_CANDIDATES='$FAKE_CHROME' DRY_RUN=1 $SCRIPT http://localhost:5173"
  local exit_status=$status
  local output_captured=$output
  rm -f "$FAKE_CHROME"

  [ "$exit_status" -eq 0 ]
  [[ "$output_captured" == *"$FAKE_CHROME"* ]]
  [[ "$output_captured" == *"--remote-debugging-port=9222"* ]]
  [[ "$output_captured" == *"--user-data-dir="* ]]
  [[ "$output_captured" == *"http://localhost:5173"* ]]
}
