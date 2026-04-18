# Skill: Execute Plan with Mandatory Code Review

## Назначение

Обёртка над `superpowers:executing-plans`: после реализации обязательно запускает
**code review субагентом** и не даёт коммитить без APPROVED.

Это Deliverable 2 Phase 1 quality gates. Lint/typecheck (Deliverable 1) ловят
синтаксис и типы. Review ловит семантику, архитектуру и соответствие спеке.
Без review баги уровня "код работает, но не то делает" уходят в master.

## Когда использовать

- **Всегда**, когда работа с планом приводит к изменениям кода
- Инвокация: `/execute-with-review <path-to-plan>`
- Альтернатива: если работа уже идёт через `/executing-plans` — вручную вызвать
  ревью-субагента перед коммитом (см. Фазу 2 ниже)

## Когда НЕ использовать

- Только документация без кода (AC-файлы, specs, README)
- Исследовательские запуски без коммита
- Срочный hotfix (review делается post-factum, отдельно)

## Workflow

### Фаза 1: Реализация

1. Прочитай план
2. Вызови `superpowers:executing-plans` через Skill tool
3. Веди список изменённых файлов в TodoWrite

### Фаза 2: Code Review — ОБЯЗАТЕЛЬНЫЙ ГЕЙТ

**Нельзя пропустить, даже если уверен, что всё правильно.**

Вызови субагента:

```
Agent(
  subagent_type="superpowers:code-reviewer",
  description="Review <plan-name> implementation",
  prompt="""
  Review the implementation of <plan-file>.

  Plan goal: <1-2 lines from plan>
  Files modified (git diff --name-only vs master): <list>
  Spec reference (if any): <path>

  Check against:
  - Spec alignment: does implementation match the plan's intent and acceptance criteria?
  - docs/standards/*.md — architecture, security, performance, frontend rules
    (если файлов ещё нет — используй минимальный чек-лист ниже)
  - No dead code, no hardcoded secrets, no console.log/fmt.Println debug statements
  - No new ESLint warnings beyond current baseline (see CLAUDE.md Ratchet section)
  - New behavior has test coverage (AC-based if applicable)
  - Naming conventions match neighbors

  Return exactly one of:
  - "APPROVED: <1 sentence why>"
  - "CHANGES_REQUESTED:" followed by numbered list, each item: file:line — issue — suggested fix
  """
)
```

### Фаза 3: Гейт решения

- **APPROVED** → переход к Фазе 4 (коммит)
- **CHANGES_REQUESTED** → переход к Фазе 5 (итерация)

### Фаза 4: Коммит

Только после APPROVED.

1. `git status` — sanity check (правило из `feedback_git_status_precommit.md`)
2. Явный `git add <files>`, никаких `-A` и `.`
3. Коммит-сообщение содержит в теле строку `Reviewed: superpowers:code-reviewer (APPROVED)`
4. Пре-commit hook прогонит `./scripts/check.sh` — он тоже должен пройти

### Фаза 5: Итерация

1. Для каждого пункта `CHANGES_REQUESTED`:
   - Применить фикс (не спорить, не объяснять)
2. Вернуться в Фазу 2 с новым diff'ом
3. Максимум **3 итерации**. После 3-й — эскалация (Фаза 6)

### Фаза 6: Эскалация

После 3 безуспешных итераций — остановись:
- Выведи нерешённые пункты пользователю
- Опции: override (пользователь явно подтверждает), revise plan, abandon
- НЕ коммить с override без явного подтверждения пользователя в текстовом ответе

## Минимальный чек-лист reviewer'а (до Phase 2 Global Standards)

Пока `docs/standards/*.md` не написаны, субагент использует этот минимум:

### Security
- [ ] Нет hardcoded credentials, API keys, DB passwords
- [ ] Логгирование не содержит секретов (tokens, body запросов с PII)
- [ ] Новые HTTP-эндпоинты защищены auth-middleware

### Code quality
- [ ] Нет закомментированного кода "на будущее"
- [ ] Нет `console.log`, `fmt.Println`, `print` — только через logger
- [ ] Нет `TODO` без owner/context
- [ ] Naming: новые идентификаторы в том же стиле, что соседние в файле

### Architecture
- [ ] Go: хендлеры не дёргают БД напрямую — через repository
- [ ] TS: бизнес-логика не в компонентах — в hooks / services
- [ ] Нет cross-layer dependencies (frontend не импортирует server-side)

### Tests
- [ ] Новое поведение имеет AC-тест (если поведение пользователя)
- [ ] Новая функция с нетривиальной логикой имеет unit-тест

### Spec compliance
- [ ] Implementation соответствует тому, что требует план/спека
- [ ] Лишние фичи не добавлены (scope creep)
- [ ] Если есть drift — зафиксирован в `docs/ac/_DRIFT.md`

### Quality gates
- [ ] ESLint warnings не выросли (baseline в `package.json` lint script)
- [ ] TypeScript строгие проверки проходят
- [ ] Go vet проходит (если Go изменения)

## Красные флаги — сразу эскалация

- Reviewer нашёл credential в коде → остановись, сообщи пользователю
- Reviewer нашёл, что implementation кардинально не то, что в плане → пользователю
- Reviewer вернул неструктурированный ответ (не APPROVED / CHANGES_REQUESTED) → пользователю

## Антипаттерны

- ❌ "Код короткий, review избыточен" — нет
- ❌ "CHANGES_REQUESTED про мелочи, закоммичу" — нет
- ❌ "4-я итерация, ещё чуть-чуть" — нет, эскалация после 3
- ❌ `git commit --no-verify` ради обхода — нет

## Что этот skill даёт в сочетании с Deliverable 1

| Уровень | Deliverable 1 (lint/CI) | Deliverable 2 (этот skill) |
|---|---|---|
| Syntax / types | ✅ ловит | - |
| Lint rules (no-unsafe-finally, etc.) | ✅ ловит | - |
| Semantic issues (логика неверна) | ❌ | ✅ ловит |
| Architecture violations | ❌ | ✅ ловит |
| Spec drift | частично (через AC-тесты) | ✅ ловит явно |
| Security patterns | ❌ | ✅ ловит |

Без Deliverable 2 quality gates — только половина защиты.
