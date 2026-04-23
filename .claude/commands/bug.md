---
description: Зарегистрировать новый баг — собрать пакет артефактов и запустить агента-решателя
argument-hint: "<описание бага одной строкой>"
---

Ты оркестратор bug-triage skill'а. Пользователь описал новый баг — прочти `skills/bug-triage.md` и выполни **Фазу 1 + Фазу 2** согласно его инструкциям.

**Описание бага:** `$ARGUMENTS`

Workflow (из skill'а):

1. **Фаза 0 проверка:** если `./scripts/bug-triage/chrome-check.sh` возвращает exit 1 — сначала сделай инициализацию (запусти Chrome через chrome-launch.sh с `http://72.56.232.202:18085/` или `BUG_TRIAGE_PORTAL_URL`), только потом продолжай.

2. **Фаза 1 — сбор bug package:**
   - Сгенерируй `bug_id` = `YYYYMMDD-HHMMSS-<slug из первых 40 символов описания, ASCII lower-kebab>`
   - Создай `.claude/bugs/<bug_id>/` и `.claude/bugs/<bug_id>/response-bodies/`
   - Запиши `description.md` с таймстампом и исходным текстом
   - Сними через `chrome-devtools-mcp` параллельно (в одном сообщении): `list_pages`, `take_screenshot(filePath=...)`, `list_console_messages`, `list_network_requests`, `take_snapshot(filePath=...)`. **Важно:** console/network отдают markdown, сохраняй как `console.md` / `network.md`; snapshot сохраняется как `.txt`.
   - Отфильтруй failed-requests (парсинг markdown regexp'ом, см. skill §1.4), сохрани в `failed-requests.md`
   - Для каждого failed — `get_network_request(reqid=N, responseFilePath=...)` → файлы `response-bodies/N.response` и `N.request`
   - Собери backend-логи: `./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml logs --since=2m --no-color"` (голый `docker compose logs` не работает), truncate если >1MB (см. skill §1.5)
   - Обнови `state.json` — добавь bug со `status=captured`
   - Создай TodoWrite item

3. **Фаза 2 — диспатч агента:**
   - Проверь лимит `max_parallel=3` из state.json. Если превышен — переведи в `status=queued`, останови здесь.
   - Иначе → обнови `status=in_progress`, `agent_started_at=now()`
   - Вызови `Agent(subagent_type="general-purpose", run_in_background=true, isolation="worktree", ...)` с brief-ом из skill §2.4 (подставь bug_id и PORTAL_URL=`http://72.56.232.202:18085/`)
   - Верни пользователю: "Bug <bug_id> диспатчен. Пиши следующий /bug или ожидай уведомления."

4. Когда придёт уведомление об окончании агента — переходи в Фазу 3 (merge-gate и деплой) согласно skill'у.

**Источник истины:** `skills/bug-triage.md`. При противоречии — слушай skill.
