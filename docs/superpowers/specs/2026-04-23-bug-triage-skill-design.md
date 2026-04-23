# Bug Triage Skill — Design

**Дата:** 2026-04-23
**Статус:** Draft
**Владелец спеки:** amfoterius@gmail.com
**Целевой файл реализации:** `skills/bug-triage.md`

## 1. Мотивация

Пользователь (соло-разработчик) тестирует портал в браузере и находит баги сериями. Текущий процесс — каждый баг описывается, воспроизводится, диагностируется и чинится последовательно: пока Claude чинит первый, пользователь простаивает. Нужен конвейер: пользователь кидает описание → система параллельно собирает артефакты и запускает агентов-решателей, основной поток остаётся свободным для приёма следующих багов.

## 2. Принятые решения (из брейнсторма)

| # | Вопрос | Решение |
|---|---|---|
| 1 | Scope агента | Полный цикл: диагностика → фикс → тесты → review → merge → deploy |
| 2 | Сбор артефактов | Делает основной поток (не агент) |
| 3 | Асинхронность | `Agent(run_in_background=true)` |
| 4 | Браузер пользователя | Skill сам запускает Chrome с `--remote-debugging-port=9222` и отдельным `--user-data-dir` |
| 5 | Набор артефактов | Расширенный: скрин + console + network + DOM + failed response bodies + backend-логи всего стека за 2 мин |
| 6 | Backend-логи | Гибрид: весь стек + пометка failed-request URLs как подсказка агенту |
| 7 | Merge-gate | Автомёрдж при чистом diff без пересечений с другими worktree; конфликт/пересечение → спрос пользователя |
| 8 | Persistence | `.claude/bugs/state.json` — один JSON-файл |
| 9 | Браузер для агента | `playwright-mcp` (изолированный Chromium) — не конфликтует с chrome-devtools-mcp основного потока |
| 10 | Триггер | Явный: `/bug <описание>` |

## 3. Архитектура

Три роли в одном процессе Claude Code:

### Основной поток (orchestrator)
- Принимает баги от пользователя (текст через `/bug`)
- Собирает bug package через `chrome-devtools-mcp`
- Обновляет TodoWrite и `.claude/bugs/state.json`
- Диспатчит агентов через `Agent(run_in_background=true, isolation="worktree")`
- При уведомлении о готовности агента — запускает merge-gate и батч-деплой
- Никогда не блокируется ожиданием агента

### Фоновый агент (solver)
- Один на баг, работает в отдельном worktree (ветка `fix/bug-<bug_id>`)
- Получает путь к готовому bug package
- Использует `playwright-mcp` для repro и проверки фикса (свой изолированный Chromium, test-креды из `project_sandbox_test_credentials`)
- Проходит `superpowers:code-reviewer` перед выдачей результата
- Возвращает `result.json` — НЕ мёрджит, НЕ деплоит

### Merge-gate (в основном потоке, по уведомлению)
- Читает `result.json`
- Dry-run `git merge --no-commit --no-ff`
- Чистый + не пересекается с активными worktree → автомёрдж
- Иначе → `review_pending`, спрос пользователя
- После N merged в очереди (или тайм-аута 5 мин) → батч `./scripts/server.sh deploy`

## 4. Bug package

Структура `.claude/bugs/<bug_id>/`:

```
<bug_id>/
├── description.md           твой текст + таймстамп
├── screenshot.png           take_screenshot, viewport
├── console.json             list_console_messages, все уровни
├── network.json             list_network_requests (метод, URL, статус, timing)
├── failed-requests.json     подмножество network, status >= 400
├── response-bodies/
│   └── <request-id>.json    get_network_request для каждого failed
├── dom-snapshot.html        take_snapshot
├── url.txt                  текущий URL
└── backend-logs.txt         docker compose logs --since=2m со всего стека
```

**bug_id format:** `YYYYMMDD-HHMMSS-<slug>`, slug генерируется из первых 40 символов описания (snake-case, ASCII).

**Порядок сбора (основной поток):**
1. `mcp__plugin_chrome-devtools-mcp__list_pages` → выбор активной вкладки
2. Параллельно: `take_screenshot`, `list_console_messages`, `list_network_requests`, `take_snapshot`
3. Фильтр `failed-requests.json` из network.json
4. Для каждого failed: `get_network_request` → `response-bodies/<id>.json`
5. `./scripts/server.sh exec "docker compose logs --since=2m"` → `backend-logs.txt` (truncate до последних 1000 строк на сервис, если >1MB)
6. `description.md` с текстом и таймстампом

**Граничные случаи:**
- Несколько вкладок открыто → берём последнюю с фокусом, если не определяется → спрос пользователя
- Chrome не отвечает → см. Section 5

## 5. Запуск Chrome

Первый вызов skill'а в сессии:

1. `curl -s http://localhost:9222/json/version` — проверка порта
2. Если нет ответа — запуск Chrome:
   ```
   Bash run_in_background=true:
   "$CHROME_EXE" \
     --remote-debugging-port=9222 \
     --user-data-dir=%USERPROFILE%\.claude\bugs\chrome-profile \
     $PORTAL_URL
   ```
3. Ждём 3 секунды, повторяем проверку
4. Если не отвечает — ошибка пользователю с инструкцией

**$CHROME_EXE** — skill проверяет в порядке:
- `C:\Program Files\Google\Chrome\Application\chrome.exe`
- `C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`
- `%LOCALAPPDATA%\Google\Chrome\Application\chrome.exe`

**$PORTAL_URL** — из конфига skill'а (дефолт `http://localhost:5173` или prod-адрес портала, настраивается в начале skill.md).

**Профиль** `~/.claude/bugs/chrome-profile` — отдельный от основного Chrome. Cookies/login сохраняются между сессиями Claude Code. Закрывает окно пользователь сам.

## 6. Диспатч агента

При готовом bug package:

1. TodoWrite: добавить `[<bug_id>] <short_desc>` со статусом `in_progress`
2. Обновить `state.json`: bug entry, `status=in_progress`, `agent_started_at`
3. Вызвать:
   ```
   Agent(
     subagent_type="general-purpose",
     run_in_background=true,
     isolation="worktree",
     description="Solve bug <bug_id>",
     prompt=<brief>
   )
   ```

**Brief (полный шаблон):**

```
Ты решаешь баг <bug_id>. Полный пакет артефактов: .claude/bugs/<bug_id>/

Твой workflow:
1. Прочти description.md, screenshot.png, console.json, failed-requests.json, backend-logs.txt
2. Определи корневую причину. Используй Grep/Read по коду, не гадай.
3. У тебя есть playwright-mcp (изолированный Chromium). Используй его для:
   - Воспроизведения бага (убедись, что понял симптом)
   - Проверки фикса после реализации

   НЕ используй chrome-devtools-mcp — он занят основным потоком,
   вызовы туда сломают активную сессию пользователя.

   Креды для логина в портал: aggregator@test.local / Admin123!
   (или admin@example.com / Admin123! для админки)
   URL портала: <PORTAL_URL>

   Сохрани storage_state (localStorage + cookies) после логина в файл в worktree,
   при повторных проверках переиспользуй.

4. Напиши фикс. Минимальный. Не рефактори смежное.
5. Запусти ./scripts/check.sh — должен пройти
6. Добавь/обнови тесты под изменение (AC-based, если поведение пользователя)
7. Вызови субагента superpowers:code-reviewer для review diff'а
   - APPROVED → к шагу 8
   - CHANGES_REQUESTED → исправь, повтори (макс 3 итерации)
   - После 3 неуспешных → верни failed
8. Коммит в текущую ветку worktree (fix/bug-<bug_id>). НЕ merge, НЕ push.
9. Запиши результат в .claude/bugs/<bug_id>/result.json:
   {
     "status": "ready_for_merge" | "failed",
     "branch": "fix/bug-<bug_id>",
     "worktree_path": "<abs_path>",
     "summary": "<1-2 предложения что сделал>",
     "files_changed": ["..."],
     "review_iterations": <N>,
     "failure_reason": "<только если failed>"
   }

Не мёрджи сам. Не деплой сам. Оркестратор сделает это.
```

**Лимит параллельности:** `max_parallel=3` (в state.json, настраивается). При превышении — новый баг → package собирается, но агент ставится в очередь `queued`, стартует когда освободится слот.

## 7. Merge-gate и деплой

Триггер: system-reminder об окончании background-агента.

### Чтение результата

```
1. cat .claude/bugs/<bug_id>/result.json
2. Если status=failed:
   - TodoWrite → failed (✗ префикс)
   - state.json: status=failed, failure_reason
   - Сообщение пользователю + путь к worktree
   - Worktree НЕ удаляется
3. Если status=ready_for_merge: → к Merge gate
```

### Merge gate

```
1. cd в основной репо
2. git fetch <worktree_path> fix/bug-<bug_id>:fix/bug-<bug_id>
3. Посчитать пересечение files_changed с активными/pending worktree'ами из state.json
4. Dry-run merge на временном ref: git merge --no-commit --no-ff fix/bug-<bug_id>
   - Чисто + нет пересечений → коммитим, status=merged, bug → deploy_queue
   - Конфликт ИЛИ пересечение → git merge --abort, status=review_pending,
     сообщение пользователю:
     "Bug <id> готов. Конфликтует/пересекается с <other_id> по файлам <list>.
      [m]erge сейчас, [d]efer, [r]eject?"
```

### Батч-деплой

Триггер:
- Таймер 5 мин с первого попадания в `deploy_queue`
- ИЛИ немедленно, если очереди `in_progress` + `queued` пусты

Команда:
- Все merged в одном сервисе → `./scripts/server.sh deploy <service>`
- Разные сервисы → `./scripts/server.sh deploy` (весь стек)

После деплоя:
- Все `merged` → `deployed`, TodoWrite → completed
- `deployed_at` в state.json
- Очистка worktree: `git worktree remove <path>` (ветка остаётся в репо)
- Уведомление: "Задеплоено: N фиксов. <bug_ids>"

Если деплой упал:
- Merge НЕ откатывается (код уже в master)
- Все `merged` → обратно в `review_pending` с пометкой "deploy failed"
- Сообщение пользователю с выхлопом ошибки

## 8. State & TodoWrite

### `.claude/bugs/state.json`

```json
{
  "version": 1,
  "chrome_pid": 12345,
  "max_parallel": 3,
  "bugs": {
    "<bug_id>": {
      "status": "captured | queued | in_progress | review_pending | ready_for_merge | merged | deployed | failed",
      "description_short": "<первые 60 символов description.md>",
      "created_at": "<ISO8601>",
      "package_dir": ".claude/bugs/<bug_id>",
      "worktree_path": "<abs>",
      "branch": "fix/bug-<bug_id>",
      "agent_started_at": "<ISO8601 | null>",
      "agent_finished_at": "<ISO8601 | null>",
      "files_changed": ["..."] | null,
      "review_iterations": <N> | null,
      "failure_reason": "<str> | null",
      "deployed_at": "<ISO8601 | null>"
    }
  },
  "deploy_queue": ["<bug_id>", ...],
  "last_deploy_at": "<ISO8601 | null>"
}
```

### Маппинг TodoWrite

| state.json status | TodoWrite status | Префикс |
|---|---|---|
| captured, queued, in_progress | in_progress | — |
| review_pending | in_progress | ⏸ |
| ready_for_merge, merged | in_progress | — |
| deployed | completed | — |
| failed | completed | ✗ |

### Восстановление после новой сессии

При первом `/bug` в новой сессии:
1. Читает `state.json`
2. Для каждого `in_progress` старше 30 мин без `agent_finished_at` → `failed`, уведомление
3. Для `review_pending` → переспрашивает
4. Для `merged` без деплоя → спрашивает: деплоить сейчас или ждать?
5. `deployed` старше 24ч → удаляются из state.json (артефакты остаются)

## 9. Триггер и error handling

### Триггер

- `/bug-triage` — инициализация (запуск Chrome, проверка state.json, восстановление)
- `/bug <описание>` — новый баг (собрать package + диспатч агента)
- `/bug-triage status` — вывод текущего state (активные, очередь, последний деплой)
- `/bug-triage stop` — завершение: кидает все активные worktree'ы в `failed`, пишет отчёт

### Error handling

| Фейл | Поведение |
|---|---|
| Chrome не стартует | Вывод путей, по которым искали, инструкция запустить вручную |
| 9222 не отвечает через 3с | То же + предложение проверить, не блокирует ли антивирус |
| `chrome-devtools-mcp` не подключается | Просьба рестартовать Claude Code после ручного запуска Chrome |
| Агент завис (>30 мин без обновления `agent_finished_at`) | → failed, worktree сохранён |
| `server.sh deploy` упал | merged → review_pending с пометкой, вывод ошибки |
| state.json corrupt | Backup в `state.json.bak`, старт с пустого, уведомление |
| Нет вкладок в Chrome | Ошибка пользователю: "Открой портал в окне Chrome и повтори" |

## 10. Границы (YAGNI)

**Skill НЕ делает:**
- Performance-профилирование (отдельная задача)
- Отчёты/аналитику по багам сверх state.json
- Rollback деплоя при поломке (сломал → новый баг)
- Работу вне `c:\projects\sms`
- Автозакрытие Chrome (окно пользователя)
- Автоочистку веток `fix/bug-*` (руками/cron)
- Поддержку нескольких браузеров (только Chrome)
- Мониторинг продакшена — только sms-server (dev sandbox)

## 11. Зависимости

- `chrome-devtools-mcp` MCP-сервер (подключён)
- `playwright-mcp` MCP-сервер (для агентов)
- `superpowers:using-git-worktrees` skill
- `superpowers:code-reviewer` skill (agent)
- `./scripts/server.sh` (deploy, logs, exec)
- `./scripts/check.sh` (agent запускает)
- Chrome установлен в одной из стандартных локаций Windows
- Test-креды из `project_sandbox_test_credentials` (память)

## 12. Открытые вопросы

Нет — все развилки закрыты в брейнсторме. Пользователь ревьюит перед переходом к плану.
