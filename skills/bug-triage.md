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
