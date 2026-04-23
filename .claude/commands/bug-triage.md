---
description: Bug triage — инициализация / статус / стоп конвейера багов
argument-hint: "[status|stop] (без аргумента = init)"
---

Ты оркестратор bug-triage skill'а. Прочитай `skills/bug-triage.md` и выполни команду согласно его инструкциям.

**Аргумент пользователя:** `$ARGUMENTS`

Ветвление по аргументу:

- **пусто** (или `init`) → выполни **Фазу 0: Инициализация**:
  - `./scripts/bug-triage/state-init.sh`
  - `./scripts/bug-triage/chrome-check.sh`; если exit 1 → запустить Chrome через `./scripts/bug-triage/chrome-launch.sh "$PORTAL_URL"` (PORTAL_URL по умолчанию `http://72.56.232.202:18085/`, override через env `BUG_TRIAGE_PORTAL_URL`), подождать 3с, проверить снова. **Quirk:** Chrome откроется на `about:blank` — после успешного chrome-check вызови `navigate_page(url=$PORTAL_URL)` через chrome-devtools-mcp, чтобы портал реально загрузился.
  - Восстановить state из `.claude/bugs/state.json`: обработать зависшие `in_progress` (>30 мин), `review_pending`, `merged` без деплоя, удалить `deployed` старше 24ч.
  - Синхронизировать TodoWrite.
  - Отчитаться пользователю, чем готов заняться.

- **`status`** → прочитай `.claude/bugs/state.json`, выведи таблицу: активные / очередь деплоя / последний деплой / требуют внимания (см. раздел «Команды управления → /bug-triage status» в skill).

- **`stop`** → выполни секцию «/bug-triage stop» skill'а: активные → failed, спросить про смёрдженные недеплоенные, отчёт.

**Источник истины:** `skills/bug-triage.md`. Если там противоречит с этой инструкцией — слушай skill.
