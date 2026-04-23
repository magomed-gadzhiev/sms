# Bug Triage Skill Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Реализовать skill `skills/bug-triage.md` — конвейер багов: пользователь кидает описание через `/bug`, основной поток собирает артефакты в браузере и запускает параллельных агентов в worktree-ах, автомёрджит чистые фиксы и батчит деплой.

**Architecture:** Один markdown-файл (skill) + один bash-хелпер для запуска Chrome на Windows. Skill исполняется Claude'ом, не runtime-ом, поэтому тесты — это dry-run-прогон каждой фазы вручную на реальном портале. Хелпер тестируется на locate-check (exit-codes) без реального запуска Chrome.

**Tech Stack:** Markdown (skill-инструкции для Claude), bash (хелпер), Claude Code tool surface: `Agent(run_in_background, isolation=worktree)`, `TodoWrite`, `Bash`, `chrome-devtools-mcp`, `playwright-mcp`, `superpowers:code-reviewer`, `superpowers:using-git-worktrees`.

**Spec:** [docs/superpowers/specs/2026-04-23-bug-triage-skill-design.md](../specs/2026-04-23-bug-triage-skill-design.md)

---

## File Structure

**Создаются:**
- `skills/bug-triage.md` — главный файл skill'а (инструкции для Claude)
- `scripts/bug-triage/chrome-launch.sh` — определение пути к chrome.exe и запуск с `--remote-debugging-port=9222`
- `scripts/bug-triage/chrome-check.sh` — проверка, слушает ли 9222
- `scripts/bug-triage/state-init.sh` — инициализация `.claude/bugs/state.json`, если его нет
- `scripts/bug-triage/tests/chrome-launch.bats` — bats-тесты хелперов
- `.claude/bugs/.gitkeep` — папка для артефактов, в git держим только плейсхолдер

**Не создаются руками (генерируются runtime-ом):**
- `.claude/bugs/<bug_id>/*` — артефакты каждого бага
- `.claude/bugs/chrome-profile/` — Chrome-профиль
- `.claude/bugs/state.json` — state file

**Модифицируются:**
- `.gitignore` — добавить `.claude/bugs/` (кроме `.gitkeep`)

**Не изменяются:**
- Никакой Go/TS код. Skill оркестрирует существующие инструменты.

---

## Task 1: Инфраструктура — папки, .gitignore, skeleton

**Files:**
- Create: `.claude/bugs/.gitkeep`
- Create: `scripts/bug-triage/` (директория)
- Modify: `.gitignore`

- [ ] **Step 1: Проверить текущий `.gitignore` на коллизии**

Run:
```bash
grep -n "claude/bugs" .gitignore || echo "no existing rule"
```
Expected: `no existing rule` (правил по пути ещё нет).

- [ ] **Step 2: Создать плейсхолдер для папки багов**

```bash
mkdir -p .claude/bugs scripts/bug-triage scripts/bug-triage/tests
touch .claude/bugs/.gitkeep
```

- [ ] **Step 3: Добавить в `.gitignore` правило**

Дописать в конец `.gitignore`:

```
# Bug triage runtime artifacts (keep .gitkeep only)
.claude/bugs/*
!.claude/bugs/.gitkeep
```

- [ ] **Step 4: Проверить, что git игнорирует правильно**

Run:
```bash
echo "test" > .claude/bugs/tmpfile && git status --short .claude/bugs/
```
Expected: в выводе должен быть только `.gitkeep` (новый), `tmpfile` игнорируется. Удали tmpfile: `rm .claude/bugs/tmpfile`.

- [ ] **Step 5: Commit**

```bash
git add .claude/bugs/.gitkeep .gitignore scripts/bug-triage
git commit -m "chore(bug-triage): добавить папки и gitignore для runtime-артефактов"
```

---

## Task 2: Хелпер `chrome-check.sh`

**Files:**
- Create: `scripts/bug-triage/chrome-check.sh`
- Test: `scripts/bug-triage/tests/chrome-check.bats`

- [ ] **Step 1: Написать failing bats-тест**

`scripts/bug-triage/tests/chrome-check.bats`:
```bash
#!/usr/bin/env bats

SCRIPT="$BATS_TEST_DIRNAME/../chrome-check.sh"

@test "chrome-check: возвращает 1, если порт 9222 не слушает" {
  run "$SCRIPT"
  [ "$status" -eq 1 ]
  [[ "$output" == *"9222"* ]]
}

@test "chrome-check: печатает OK и возвращает 0, если 9222 отвечает на /json/version" {
  # поднимаем заглушку на 9222
  python -c "
import http.server, threading, time
class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == '/json/version':
            self.send_response(200); self.end_headers()
            self.wfile.write(b'{\"Browser\": \"Chrome/120\"}')
        else:
            self.send_response(404); self.end_headers()
    def log_message(self,*a,**k): pass
srv = http.server.HTTPServer(('127.0.0.1', 9222), H)
t = threading.Thread(target=srv.serve_forever, daemon=True); t.start()
time.sleep(0.1)
import os; os.system('\"$SCRIPT\"')
" &
  sleep 0.5
  run bash -c "curl -s http://localhost:9222/json/version >/dev/null && $SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"OK"* ]]
}
```

Примечание: в Windows/Git-Bash bats может быть не установлен. Если `bats --version` падает — fallback ручной: исполнить `./scripts/bug-triage/chrome-check.sh` при выключенном Chrome (должен выдать `exit 1`) и при запущенном (`exit 0`). Это задокументировать в комментарии теста.

- [ ] **Step 2: Прогнать тест (должен упасть)**

Run: `bats scripts/bug-triage/tests/chrome-check.bats` или ручной fallback.
Expected: FAIL — скрипта нет.

- [ ] **Step 3: Реализовать `chrome-check.sh`**

`scripts/bug-triage/chrome-check.sh`:
```bash
#!/usr/bin/env bash
# Проверяет, слушает ли Chrome на порту 9222 (CDP).
# Exit 0 = слушает, Exit 1 = нет.

set -euo pipefail

PORT="${CHROME_DEBUG_PORT:-9222}"

if curl -s -m 2 "http://localhost:${PORT}/json/version" >/dev/null 2>&1; then
  echo "OK: Chrome CDP отвечает на ${PORT}"
  exit 0
else
  echo "FAIL: порт ${PORT} не отвечает на /json/version"
  exit 1
fi
```

Сделать исполняемым:
```bash
chmod +x scripts/bug-triage/chrome-check.sh
```

- [ ] **Step 4: Прогнать тест (должен пройти)**

Run: `bats scripts/bug-triage/tests/chrome-check.bats` или fallback.
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add scripts/bug-triage/chrome-check.sh scripts/bug-triage/tests/chrome-check.bats
git commit -m "feat(bug-triage): chrome-check.sh — проверка CDP-порта 9222"
```

---

## Task 3: Хелпер `chrome-launch.sh`

**Files:**
- Create: `scripts/bug-triage/chrome-launch.sh`
- Test: `scripts/bug-triage/tests/chrome-launch.bats`

- [ ] **Step 1: Написать failing тест**

`scripts/bug-triage/tests/chrome-launch.bats`:
```bash
#!/usr/bin/env bats

SCRIPT="$BATS_TEST_DIRNAME/../chrome-launch.sh"

@test "chrome-launch: DRY_RUN печатает найденный путь к chrome.exe" {
  run bash -c "DRY_RUN=1 $SCRIPT http://localhost:5173"
  [ "$status" -eq 0 ]
  [[ "$output" == *"chrome.exe"* ]]
  [[ "$output" == *"--remote-debugging-port=9222"* ]]
  [[ "$output" == *"--user-data-dir="* ]]
  [[ "$output" == *"http://localhost:5173"* ]]
}

@test "chrome-launch: возвращает 2, если chrome.exe не найден" {
  run bash -c "CHROME_CANDIDATES=/nonexistent/chrome.exe DRY_RUN=1 $SCRIPT http://localhost:5173"
  [ "$status" -eq 2 ]
  [[ "$output" == *"не найден"* ]]
}

@test "chrome-launch: без URL падает с кодом 3" {
  run "$SCRIPT"
  [ "$status" -eq 3 ]
  [[ "$output" == *"URL"* ]]
}
```

- [ ] **Step 2: Прогнать тест (должен упасть)**

Run: `bats scripts/bug-triage/tests/chrome-launch.bats`
Expected: FAIL.

- [ ] **Step 3: Реализовать `chrome-launch.sh`**

`scripts/bug-triage/chrome-launch.sh`:
```bash
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

# Default candidates (Windows)
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

# Запускаем в фоне, отвязав от tty
nohup "${CMD[@]}" >/dev/null 2>&1 &
echo "Chrome запущен (PID $!), CDP на порту $PORT, профиль: $PROFILE_DIR"
```

Сделать исполняемым:
```bash
chmod +x scripts/bug-triage/chrome-launch.sh
```

- [ ] **Step 4: Прогнать тест (должен пройти)**

Run: `bats scripts/bug-triage/tests/chrome-launch.bats`
Expected: PASS (3 теста).

- [ ] **Step 5: Commit**

```bash
git add scripts/bug-triage/chrome-launch.sh scripts/bug-triage/tests/chrome-launch.bats
git commit -m "feat(bug-triage): chrome-launch.sh — запуск Chrome с CDP-портом"
```

---

## Task 4: Хелпер `state-init.sh`

**Files:**
- Create: `scripts/bug-triage/state-init.sh`
- Test: `scripts/bug-triage/tests/state-init.bats`

- [ ] **Step 1: Failing-тест**

`scripts/bug-triage/tests/state-init.bats`:
```bash
#!/usr/bin/env bats

SCRIPT="$BATS_TEST_DIRNAME/../state-init.sh"

setup() {
  export BUGS_DIR="$(mktemp -d)"
}

teardown() {
  rm -rf "$BUGS_DIR"
}

@test "state-init: создаёт state.json с version и bugs={}, если его нет" {
  run "$SCRIPT"
  [ "$status" -eq 0 ]
  [ -f "$BUGS_DIR/state.json" ]
  grep -q '"version": 1' "$BUGS_DIR/state.json"
  grep -q '"bugs": {}' "$BUGS_DIR/state.json"
  grep -q '"max_parallel": 3' "$BUGS_DIR/state.json"
}

@test "state-init: не перезаписывает существующий state.json" {
  echo '{"version":1,"custom":"xxx","bugs":{}}' > "$BUGS_DIR/state.json"
  run "$SCRIPT"
  [ "$status" -eq 0 ]
  grep -q '"custom":"xxx"' "$BUGS_DIR/state.json"
}

@test "state-init: при corrupt JSON делает .bak и создаёт новый" {
  echo 'garbage{not json' > "$BUGS_DIR/state.json"
  run "$SCRIPT"
  [ "$status" -eq 0 ]
  [ -f "$BUGS_DIR/state.json.bak" ]
  grep -q '"bugs": {}' "$BUGS_DIR/state.json"
}
```

- [ ] **Step 2: Прогнать тест (FAIL)**

Run: `bats scripts/bug-triage/tests/state-init.bats`
Expected: FAIL.

- [ ] **Step 3: Реализовать `state-init.sh`**

`scripts/bug-triage/state-init.sh`:
```bash
#!/usr/bin/env bash
# Инициализирует .claude/bugs/state.json, если его нет.
# При corrupt JSON делает backup в state.json.bak.
# Env: BUGS_DIR (default: .claude/bugs)

set -euo pipefail

BUGS_DIR="${BUGS_DIR:-.claude/bugs}"
STATE="$BUGS_DIR/state.json"

mkdir -p "$BUGS_DIR"

write_fresh() {
  cat > "$STATE" <<'EOF'
{
  "version": 1,
  "chrome_pid": null,
  "max_parallel": 3,
  "bugs": {},
  "deploy_queue": [],
  "last_deploy_at": null
}
EOF
}

if [[ ! -f "$STATE" ]]; then
  write_fresh
  echo "state.json создан"
  exit 0
fi

# Проверка валидности JSON (python — доступен и на Windows в Git Bash с mise/choco)
if ! python -c "import json, sys; json.load(open('$STATE'))" 2>/dev/null; then
  cp "$STATE" "$STATE.bak"
  write_fresh
  echo "state.json был corrupt, бэкап в $STATE.bak, создан новый"
  exit 0
fi

echo "state.json валиден, ничего не делаю"
```

Сделать исполняемым: `chmod +x scripts/bug-triage/state-init.sh`.

- [ ] **Step 4: Прогнать тест (PASS)**

Run: `bats scripts/bug-triage/tests/state-init.bats`
Expected: PASS (3 теста).

- [ ] **Step 5: Commit**

```bash
git add scripts/bug-triage/state-init.sh scripts/bug-triage/tests/state-init.bats
git commit -m "feat(bug-triage): state-init.sh — инициализация state.json с бэкапом corrupt"
```

---

## Task 5: Skill skeleton — header + триггеры + запуск Chrome (фаза init)

**Files:**
- Create: `skills/bug-triage.md`

- [ ] **Step 1: Написать заголовок и раздел `Когда использовать`**

`skills/bug-triage.md`:
```markdown
# Skill: Bug Triage — конвейер багов через браузер

## Назначение

Конвейер для багрепортов: пользователь нашёл баг в портале → описал текстом → skill
собирает артефакты из браузера, запускает параллельного агента в worktree для
диагностики/фикса/деплоя. Основной поток не блокируется и готов принимать следующие
баги.

**Спецификация:** `docs/superpowers/specs/2026-04-23-bug-triage-skill-design.md` — источник истины.

## Триггеры

- `/bug-triage` — инициализация сессии (запуск Chrome, восстановление state)
- `/bug <описание>` — новый баг: собрать артефакты + диспатчить агента
- `/bug-triage status` — вывод текущего state
- `/bug-triage stop` — завершение: активные агенты → failed, отчёт

## Когда НЕ использовать

- Нет доступного dev-портала (skill заточен под sms-server dev sandbox)
- Пользователь тестирует не в Chrome
- Работа вне `c:\projects\sms`
- Production-инциденты (skill деплоит в dev-окружение через `server.sh deploy`)
```

- [ ] **Step 2: Добавить раздел `Фаза 0: инициализация (/bug-triage)`**

Дописать в `skills/bug-triage.md`:
````markdown
## Фаза 0: Инициализация (`/bug-triage`)

Выполняется один раз в начале сессии или при явном вызове `/bug-triage`.

1. **Проверить state.json:**
   ```bash
   ./scripts/bug-triage/state-init.sh
   ```
   Создаст пустой state.json, если нет; сделает бэкап corrupt JSON.

2. **Проверить CDP-порт:**
   ```bash
   ./scripts/bug-triage/chrome-check.sh
   ```
   - exit 0 → Chrome уже запущен, идём к шагу 4
   - exit 1 → нужно запустить, шаг 3

3. **Запустить Chrome:**
   ```bash
   ./scripts/bug-triage/chrome-launch.sh "$PORTAL_URL"
   ```
   `PORTAL_URL` — по умолчанию `http://localhost:5173`, можно переопределить через
   env var `BUG_TRIAGE_PORTAL_URL`.

   Подождать 3 секунды, повторить `chrome-check.sh`. Если всё ещё exit 1 →
   сообщить пользователю: "Chrome не стартует. Проверьте установку (искал в
   `Program Files`, `Program Files (x86)`, `%LOCALAPPDATA%`) или запустите сами с
   флагом `--remote-debugging-port=9222`."

4. **Восстановить state:** прочитать `.claude/bugs/state.json`, для каждого бага:
   - `in_progress` старше 30 минут без `agent_finished_at` → `status=failed`,
     `failure_reason="session lost (Claude Code restart)"`, уведомить пользователя.
   - `review_pending` → переспросить пользователя (merge/defer/reject).
   - `merged` без `deployed_at` → спросить: "Есть `<N>` смёрдженных, не
     задеплоенных фиксов. Задеплоить сейчас или ждать следующих?"
   - `deployed` с `deployed_at` старше 24 часов → удалить запись из state.json
     (артефакты в `.claude/bugs/<bug_id>/` остаются).

5. **Синхронизировать TodoWrite:** создать in_progress-todo для каждого
   активного/review_pending/merged бага (см. маппинг в спеке, секция 8).

После инициализации — ждём следующих команд пользователя.
````

- [ ] **Step 3: Dry-run: ручная проверка фазы 0**

Имитировать вручную (в терминале, не через Claude):
```bash
./scripts/bug-triage/state-init.sh
./scripts/bug-triage/chrome-check.sh; echo "exit=$?"
```
Expected: state.json создан, chrome-check вернул 1 (если Chrome не запущен).

- [ ] **Step 4: Commit**

```bash
git add skills/bug-triage.md
git commit -m "feat(bug-triage): skeleton skill'а — триггеры, фаза инициализации"
```

---

## Task 6: Skill — сбор bug package (`/bug <описание>`)

**Files:**
- Modify: `skills/bug-triage.md`

- [ ] **Step 1: Дописать раздел `Фаза 1: сбор bug package`**

Добавить в `skills/bug-triage.md`:

````markdown
## Фаза 1: Сбор bug package (`/bug <описание>`)

Когда пользователь пишет `/bug <описание>`, основной поток делает ВСЁ нижеследующее
сам (не через агента), затем передаёт готовый пакет агенту.

### 1.1. Генерация bug_id

```
timestamp = strftime('%Y%m%d-%H%M%S', now())
slug = описание, приведённое к ASCII lower-kebab, первые 40 символов
       (убрать знаки препинания, заменить пробелы на '-', выбросить пустые хвосты)
bug_id = f"{timestamp}-{slug}"
```

Пример: `/bug Сохранение кампании падает 500` →
`20260423-143201-sohranenie-kampanii-padaet-500`.

### 1.2. Создать директорию пакета

```bash
PKG=".claude/bugs/$BUG_ID"
mkdir -p "$PKG/response-bodies"
```

Записать `$PKG/description.md`:
```markdown
# <bug_id>

**Время:** <ISO8601>
**Описание пользователя:**

<original text>
```

### 1.3. Снять артефакты через chrome-devtools-mcp

Сделать **параллельно** (одно сообщение, несколько tool calls):

1. `mcp__plugin_chrome-devtools-mcp__list_pages` → выбрать активную вкладку
   (ту, где `focused: true`; если несколько — спросить пользователя).
2. `mcp__plugin_chrome-devtools-mcp__select_page` для выбранной.
3. `mcp__plugin_chrome-devtools-mcp__take_screenshot` → сохранить в
   `$PKG/screenshot.png`.
4. `mcp__plugin_chrome-devtools-mcp__list_console_messages` → сериализовать в
   `$PKG/console.json` (JSON-массив `{level, text, timestamp, source}`).
5. `mcp__plugin_chrome-devtools-mcp__list_network_requests` → сериализовать в
   `$PKG/network.json` (массив `{id, method, url, status, duration, timestamp}`).
6. `mcp__plugin_chrome-devtools-mcp__take_snapshot` → сохранить в
   `$PKG/dom-snapshot.html`.
7. Записать текущий URL в `$PKG/url.txt` (берётся из `list_pages` / активной
   страницы).

### 1.4. Выделить failed requests

Отфильтровать `network.json` где `status >= 400` или `status == 0` (aborted).
Записать в `$PKG/failed-requests.json`.

Для каждого failed request:
```
mcp__plugin_chrome-devtools-mcp__get_network_request(id=<request_id>)
```
Сохранить тело ответа в `$PKG/response-bodies/<request_id>.json` (или `.txt`,
если не JSON).

### 1.5. Снять backend-логи

```bash
./scripts/server.sh exec "docker compose logs --since=2m --no-color" \
  > "$PKG/backend-logs.txt"
```

Если файл >1MB: не обрезать на уровне скрипта (нужен таймстамп-контекст), а
оставить как есть. Агент сам отфильтрует по `bug_id`-таймстампу.

### 1.6. Обновить state.json

Добавить bug entry, status `captured`:
```json
{
  "bugs": {
    "<bug_id>": {
      "status": "captured",
      "description_short": "<первые 60 символов>",
      "created_at": "<ISO>",
      "package_dir": ".claude/bugs/<bug_id>",
      "worktree_path": null,
      "branch": "fix/bug-<bug_id>",
      "agent_started_at": null,
      "agent_finished_at": null,
      "files_changed": null,
      "review_iterations": null,
      "failure_reason": null,
      "deployed_at": null
    }
  }
}
```

### 1.7. Обновить TodoWrite

Создать todo: `[<bug_id>] <description_short>`, status `in_progress`.

После завершения фазы 1 — переходим к фазе 2 (диспатч агента) в том же сообщении,
не ожидая пользователя.

### Граничные случаи

- **Chrome закрыт пользователем между сессиями сбора** — `list_pages` вернёт
  пусто или ошибку. Повторить фазу 0 (запустить Chrome), попросить пользователя
  повторить `/bug` после того, как воспроизведёт баг.
- **Несколько активных вкладок** — спросить пользователя, какую брать; не гадать.
- **Нулевое число failed-requests** — это нормально (баг не обязан проявляться как
  HTTP-ошибка). Пакет без `response-bodies/*.json`.
- **backend-logs.txt пустой** — значит, `server.sh exec` вернул пусто или упал.
  Не блокирует фазу, агенту виднее по console и network.
````

- [ ] **Step 2: Dry-run фазы 1 вручную через Claude**

Через Claude Code:
1. Запустить Chrome вручную с портом 9222, залогиниться в портал.
2. Имитировать `/bug тестовая страница` (попросить Claude выполнить шаги 1.1–1.7).
3. Проверить: `ls -la .claude/bugs/<bug_id>/` — должны быть все файлы, кроме
   response-bodies, если багов в сети нет.
4. Проверить state.json: `python -c "import json; print(json.load(open('.claude/bugs/state.json')))"`.

Expected: пакет собрался, state.json обновлён.

- [ ] **Step 3: Commit**

```bash
git add skills/bug-triage.md
git commit -m "feat(bug-triage): фаза 1 — сбор bug package через chrome-devtools-mcp"
```

---

## Task 7: Skill — диспатч агента (фаза 2)

**Files:**
- Modify: `skills/bug-triage.md`

- [ ] **Step 1: Дописать раздел `Фаза 2: диспатч агента`**

````markdown
## Фаза 2: Диспатч агента (solver)

Сразу после фазы 1, в том же сообщении.

### 2.1. Проверить лимит параллельности

Посчитать в state.json количество бугов со статусом `in_progress`. Если ≥
`max_parallel` (по умолчанию 3):
- Обновить status нового бага на `queued` в state.json.
- Оставить todo в `in_progress` с пометкой "(queued)".
- Вернуться к пользователю: "Пакет собран, агент в очереди. Активно: N/3."
- НЕ диспатчить агента. Когда кто-то освободится (фаза 3), взять из очереди.

Если `< max_parallel` → шаг 2.2.

### 2.2. Обновить state и todo

- state.json: `status = "in_progress"`, `agent_started_at = now()`
- TodoWrite: без изменений (уже in_progress)

### 2.3. Запустить агента

```
Agent(
  subagent_type="general-purpose",
  run_in_background=true,
  isolation="worktree",
  description="Solve bug <bug_id>",
  prompt=<BRIEF>
)
```

### 2.4. Brief для агента (подставить bug_id и PORTAL_URL)

```
Ты решаешь баг <bug_id>. Полный пакет артефактов в основном репозитории:
.claude/bugs/<bug_id>/

Тебе выделен отдельный worktree (isolation=worktree). Все коммиты — в нём.

Твой workflow:

1. Прочти артефакты в следующем порядке:
   - description.md — что сказал пользователь
   - screenshot.png — что он видел
   - failed-requests.json — какие запросы упали
   - response-bodies/*.json — тела ответов failed-запросов
   - console.json — логи браузера
   - backend-logs.txt — логи всего docker-стека за 2 минуты до снятия пакета
   - url.txt, dom-snapshot.html — контекст страницы

2. Определи корневую причину. Используй Grep/Read по коду, не гадай.
   Опирайся на спеку проекта (docs/superpowers/specs/*) и AC (docs/ac/*),
   если нашёл соответствующий feature.

3. Воспроизведи баг через playwright-mcp:
   - URL портала: <PORTAL_URL>
   - Креды: aggregator@test.local / Admin123! (или
     admin@example.com / Admin123! для админки)
   - Сохрани storage_state после логина в <worktree>/playwright-state.json,
     переиспользуй в последующих заходах.
   - НЕ используй chrome-devtools-mcp — он занят основным потоком, твои
     tool calls туда сломают активную сессию пользователя.

4. Напиши фикс. Минимальный. Не рефактори смежное.

5. Прогони ./scripts/check.sh (go vet + build + tsc + eslint). Должен пройти.

6. Добавь/обнови тесты под изменение (AC-based, если поведение пользователя).

7. Через playwright-mcp проверь, что баг больше не воспроизводится.

8. Вызови субагента superpowers:code-reviewer для review diff'а:
   - APPROVED → шаг 9
   - CHANGES_REQUESTED → фикс, повтор review (максимум 3 итерации)
   - После 3 неуспешных → сформируй result.json со status=failed

9. Коммит в текущую ветку worktree (fix/bug-<bug_id>). НЕ merge, НЕ push.
   В теле коммита — строка "Reviewed: superpowers:code-reviewer (APPROVED)".

10. Запиши результат в <абсолютный путь к основному репо>/.claude/bugs/<bug_id>/result.json:
    {
      "status": "ready_for_merge" | "failed",
      "branch": "fix/bug-<bug_id>",
      "worktree_path": "<абсолютный путь к твоему worktree>",
      "summary": "<1-2 предложения что сделал>",
      "files_changed": ["relative/path.go", ...],
      "review_iterations": <N>,
      "failure_reason": "<только если failed>"
    }

Не мёрджи сам. Не деплой сам. Оркестратор сделает это.
```

### 2.5. Ответ пользователю

После запуска Agent(run_in_background=true) — короткое сообщение:
"Bug <bug_id> диспатчен. Пиши следующий `/bug` или ожидай уведомления."

### Граничные случаи

- **Agent tool не принимает `isolation=worktree`** — значит, плагин worktrees
  не подключён. Проверить `superpowers:using-git-worktrees`. Без worktree —
  отказать: "Нельзя диспатчить без изоляции. Установите superpowers."
- **Превышение max_parallel = 3** — ставить в `queued`, стартовать из фазы 3.
````

- [ ] **Step 2: Dry-run фазы 2**

Через Claude Code — попросить Claude выполнить фазу 2 для тестового bug_id из
фазы 1. Проверить: появился background agent, state.json обновлён на `in_progress`.
Не обязательно дожидаться результата — только проверить, что запуск сработал.

- [ ] **Step 3: Commit**

```bash
git add skills/bug-triage.md
git commit -m "feat(bug-triage): фаза 2 — диспатч background-агента в worktree"
```

---

## Task 8: Skill — merge-gate и деплой (фаза 3)

**Files:**
- Modify: `skills/bug-triage.md`

- [ ] **Step 1: Дописать раздел `Фаза 3: merge-gate и деплой`**

````markdown
## Фаза 3: Merge-gate и деплой

Триггер: system-reminder об окончании background-агента (формат:
"Background agent <id> completed").

### 3.1. Прочитать результат

```bash
cat .claude/bugs/<bug_id>/result.json
```

Если файла нет (агент упал, не успел записать):
- state.json: `status=failed`, `failure_reason="agent did not produce result.json"`
- Сообщение пользователю с путём к worktree.
- К шагу 3.7 (поднять следующего из очереди queued).

### 3.2. Ветвление по status

**Если `status == "failed"`:**
- state.json: `status=failed`, `failure_reason=<из result.json>`,
  `agent_finished_at=now()`
- TodoWrite: в completed с префиксом ✗
- Сообщение пользователю с путём к worktree и кратким summary
- К шагу 3.7

**Если `status == "ready_for_merge"`:**
- Идём в merge-gate (3.3).

### 3.3. Merge-gate: проверка пересечений

Из state.json взять `files_changed` всех бугов со статусом
`in_progress` или `ready_for_merge` (кроме текущего).

Пересечение = непустое `set(files_changed_current) ∩ set(files_changed_other)`.

Если есть пересечение → `review_pending` (шаг 3.5), пропустить 3.4.

### 3.4. Dry-run merge

```bash
# Подтянуть ветку из worktree (ветка уже есть в git-репо, worktree просто
# делит тот же .git — fetch не нужен)
git merge --no-commit --no-ff fix/bug-<bug_id>
MERGE_STATUS=$?

if [[ $MERGE_STATUS -eq 0 ]]; then
  # Чистый merge
  git commit -m "merge: bug <bug_id> — <summary>"
  # state.json: status=merged, добавить в deploy_queue
else
  git merge --abort
  # → review_pending (шаг 3.5)
fi
```

### 3.5. Review pending (конфликт или пересечение)

- state.json: `status=review_pending`
- TodoWrite: in_progress с префиксом ⏸
- Спросить пользователя:
  ```
  Bug <bug_id> готов к merge.
  Причина задержки: <conflict | overlap with bug <other_id>>
  Файлы: <list>
  Варианты:
  - m (merge now) — я помогу разрулить конфликт вручную
  - d (defer) — пусть ждёт, пока другой завершится
  - r (reject) — отклонить фикс, worktree оставить на память
  ```
- Ответ пользователя обрабатывается в следующем сообщении (не блокирует поток).

### 3.6. Триггер батч-деплоя

После попадания в `deploy_queue`:
- Запустить таймер 5 минут (через фоновый sleep + background task, или просто
  отметить в state.json `deploy_scheduled_at`).
- ИЛИ: если очереди `in_progress` и `queued` пусты (все агенты закончили) →
  немедленный деплой.

### 3.7. Батч-деплой

```bash
# Выбрать команду
if все merged фиксы в одном сервисе (по files_changed path mapping):
  ./scripts/server.sh deploy <service>
else:
  ./scripts/server.sh deploy
```

Ожидание завершения команды (foreground, блокирует поток — deploy идёт 1–2
минуты, это приемлемо, пользователь видит, что работа идёт).

**Успех (exit 0):**
- Все `merged` → `deployed`, `deployed_at=now()`
- TodoWrite → completed
- `deploy_queue` очистить
- `last_deploy_at=now()` в state.json
- Очистить worktree: для каждого deployed bug —
  `git worktree remove <worktree_path>`. Ветка `fix/bug-*` остаётся.
- Сообщение: "Задеплоено: <N> фиксов. <bug_ids>."

**Фейл (exit != 0):**
- Merge НЕ откатывается (код уже в master).
- Все `merged` → `review_pending` с пометкой `failure_reason="deploy failed: <stderr tail>"`.
- Сообщение пользователю с выхлопом и предложением: "Деплой упал. Код в
  master. Варианты: повторить деплой, откатить merge вручную."

### 3.8. Подхват очереди

Если были `queued` баги и освободился слот (ещё один `in_progress` стал не
`in_progress`) → взять первый из `queued`, перейти в фазу 2 для него.
````

- [ ] **Step 2: Dry-run**

Проработать сценарии вручную (без реального агента):
1. Создать искусственный `result.json` со `status=ready_for_merge`.
2. Пройти 3.1–3.4 руками.
3. Проверить, что state.json корректно обновляется.

- [ ] **Step 3: Commit**

```bash
git add skills/bug-triage.md
git commit -m "feat(bug-triage): фаза 3 — merge-gate и батч-деплой"
```

---

## Task 9: Skill — команды `status` и `stop`

**Files:**
- Modify: `skills/bug-triage.md`

- [ ] **Step 1: Дописать раздел `Команды управления`**

````markdown
## Команды управления

### `/bug-triage status`

Прочитать state.json, вывести таблицу:
```
Активные (<N>/3):
  <bug_id> | <status> | <elapsed> | <short_desc>
  ...
В очереди на деплой (<M>):
  <bug_id> | merged at <time> | <files_changed summary>
Последний деплой: <ISO> (<elapsed> назад)
Требуют внимания:
  <bug_id> | review_pending | <reason>
```

### `/bug-triage stop`

1. Для каждого `in_progress` / `queued` / `review_pending`:
   - status → `failed`, `failure_reason="session stopped by user"`
   - TodoWrite → completed с ✗
2. Для каждого `merged` не задеплоенного:
   - Спросить: "Есть <N> смёрдженных. Задеплоить перед выходом? [y/n]"
   - y → фаза 3.7 батчем
   - n → оставить как есть, пользователь задеплоит руками
3. Не убивать Chrome — окно пользователя, пусть сам закроет.
4. Не трогать state.json окончательно — он нужен при следующем старте для истории.
````

- [ ] **Step 2: Commit**

```bash
git add skills/bug-triage.md
git commit -m "feat(bug-triage): команды status/stop"
```

---

## Task 10: Skill — антипаттерны, error handling, интеграции

**Files:**
- Modify: `skills/bug-triage.md`

- [ ] **Step 1: Дописать финальные разделы**

````markdown
## Error handling — когда что-то пошло не так

| Фейл | Поведение |
|---|---|
| Chrome не стартует | Вывод путей, по которым искали, инструкция запуска вручную |
| CDP-порт 9222 не отвечает через 3с после запуска | То же + предложение проверить антивирус/firewall |
| chrome-devtools-mcp не подключён в сессии | Просьба рестартовать Claude Code |
| Background-агент завис (>30 мин без `agent_finished_at`) | → failed, worktree сохранён |
| `server.sh deploy` упал | merged → review_pending с выхлопом |
| state.json corrupt | `state-init.sh` делает .bak и стартует чистый |
| Нет активных вкладок Chrome при `/bug` | Ошибка: "Открой портал в окне Chrome и повтори" |
| `Agent(isolation=worktree)` возвращает ошибку | Skill не продолжает — требует `superpowers:using-git-worktrees` |
| playwright-mcp не подключён | Агент в своём brief-е отметит: "не могу repro, диагностика только по пакету". Это ухудшает качество, но не блокер. |

## Антипаттерны

- ❌ Снимать артефакты **внутри** агента через chrome-devtools-mcp — сломает
  сессию пользователя (MCP single-session).
- ❌ Коммитить/мёрджить из агента — это делает оркестратор после merge-gate.
- ❌ Пропускать code-reviewer внутри агента — gated как в
  `/execute-with-review`.
- ❌ Автодеплоить после каждого merged — ломает батчинг, устаревшие деплои
  перекрываются.
- ❌ Запускать >3 агентов параллельно — ресурсы машины + риск merge-конфликтов
  возрастает нелинейно.
- ❌ `git merge --no-verify` при автомёрдже — pre-commit hook должен пройти.
- ❌ Переиспользовать твой рабочий Chrome (без отдельного `--user-data-dir`) —
  cookies/localStorage сломаются после сессии.

## Интеграции

- **superpowers:using-git-worktrees** — изоляция агентов. Без него skill не работает.
- **superpowers:code-reviewer** — обязательный гейт внутри агента.
- **chrome-devtools-mcp** — для основного потока.
- **playwright-mcp** — для агентов (repro + проверка фикса).
- **scripts/server.sh** — deploy, logs через SSH на sms-server.
- **scripts/check.sh** — пре-коммит-гейт агента.

## Что skill НЕ делает (YAGNI)

См. секцию 10 спеки. Вкратце: не профилирует производительность, не ведёт
аналитику, не откатывает деплой, не поддерживает другие браузеры, не работает
вне `c:\projects\sms`.
````

- [ ] **Step 2: Self-review на placeholders**

Пройти `skills/bug-triage.md` глазами:
- Искать "TBD", "TODO", "добавь сюда", незаполненные куски. Должно быть пусто.
- Проверить, что везде, где упомянут внешний скрипт/команда, путь полный.
- Проверить, что bug_id-формат одинаково описан во всех местах.

- [ ] **Step 3: Commit**

```bash
git add skills/bug-triage.md
git commit -m "feat(bug-triage): error handling, антипаттерны, интеграции"
```

---

## Task 11: End-to-end dry-run на тестовом баге

**Files:**
- None (validation only)

- [ ] **Step 1: Подготовка**

1. Убедиться, что `./scripts/check.sh` проходит на чистом master:
   ```bash
   ./scripts/check.sh
   ```
2. Убедиться, что playwright-mcp и chrome-devtools-mcp подключены:
   ```
   ls ~/.claude/mcp-servers/ 2>/dev/null || claude --help | grep -i mcp
   ```
3. Убедиться, что dev-портал работает (локально или на sms-server).

- [ ] **Step 2: Прогон фазы 0**

В новой Claude-сессии:
```
/bug-triage
```
Expected: Chrome запустился, state.json создан, Claude рапортует "готов принимать баги".

- [ ] **Step 3: Подготовка искусственного бага**

1. В запущенном Chrome залогиниться в портал с test-кредами.
2. Открыть любую страницу, где есть заведомо нерабочая кнопка (либо временно
   поломать UI — например, в DevTools удалить onClick handler).

- [ ] **Step 4: Прогон фазы 1 + 2**

```
/bug кнопка сохранения не срабатывает на странице настроек
```
Expected:
- В `.claude/bugs/<bug_id>/` собраны: screenshot, console.json, network.json,
  failed-requests.json (пустой, если баг UI-only), dom-snapshot.html, url.txt,
  backend-logs.txt.
- state.json: bug entry с `status=in_progress`.
- TodoWrite содержит todo по бугу.
- Background-агент запущен.

- [ ] **Step 5: Дождаться уведомления от агента**

Смотреть на system-reminder. Как только придёт "Background agent completed":
- Проверить `.claude/bugs/<bug_id>/result.json` — должен быть `ready_for_merge`
  или `failed`.
- Проверить, что Claude пошёл в фазу 3.

- [ ] **Step 6: Прогон фазы 3**

Expected (для чистого diff'а):
- Merge-gate прошёл без пересечений.
- Dry-run merge чистый.
- Commit в master.
- state.json: status=merged.
- Таймер деплоя (или немедленный, если нет других активных).

- [ ] **Step 7: Проверить деплой**

После срабатывания деплоя:
- `./scripts/server.sh status` — контейнеры на сервере обновлены.
- state.json: status=deployed, `deployed_at` заполнен.
- TodoWrite: completed.
- Worktree удалён (`git worktree list`).
- Ветка `fix/bug-<bug_id>` осталась.

- [ ] **Step 8: Откат тестового бага**

Если баг был искусственным — вернуть исходное состояние:
```bash
git revert HEAD
./scripts/server.sh deploy
```

- [ ] **Step 9: Self-check — что если всё прошло не так**

Задокументировать в `docs/superpowers/specs/2026-04-23-bug-triage-skill-design.md`
(или в issue) найденные проблемы: какая фаза, что было, что должно быть.

НЕ коммитить фиксы до устного разбора с пользователем — end-to-end тест
подсвечивает дизайн-дыры, которые лучше обсудить, чем тихо залатать.

- [ ] **Step 10: Commit (если всё прошло)**

Ничего коммитить не нужно — фаза валидационная. Если что-то правилось в skill —
отдельный коммит: `fix(bug-triage): e2e dry-run выявил ...`.

---

## Self-Review (пройдено автором плана)

**1. Spec coverage:**
- §1–2 (мотивация, решения) → нет задач, это контекст, не реализация.
- §3 (архитектура) → Task 5 (skeleton + фаза 0), Task 7 (фаза 2), Task 8 (фаза 3).
- §4 (bug package) → Task 6.
- §5 (запуск Chrome) → Task 3 (launch) + Task 5 (использование в skill).
- §6 (диспатч агента) → Task 7.
- §7 (merge-gate и деплой) → Task 8.
- §8 (state & TodoWrite) → Task 4 (init) + Task 5 (восстановление).
- §9 (триггер и error handling) → Task 5 (триггеры) + Task 9 (status/stop) + Task 10 (error handling).
- §10 (YAGNI) → Task 10.
- §11 (зависимости) → Task 10.
- End-to-end валидация → Task 11.

**Gaps:** нет.

**2. Placeholder scan:** проверено, нет TBD/TODO/«добавь ...».

**3. Type consistency:** bug_id-формат один во всех задачах (YYYYMMDD-HHMMSS-<slug>).
Поля state.json одни и те же в Task 4, 6, 7, 8. Статусы — согласованный набор
из спеки §8.

---

## Execution Handoff

План готов и будет сохранён в `docs/superpowers/plans/2026-04-23-bug-triage-skill.md`.
