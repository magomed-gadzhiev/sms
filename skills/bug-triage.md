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
   активного/review_pending/merged бага (см. маппинг в секции «State & TodoWrite» ниже).

После инициализации — ждём следующих команд пользователя.

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
