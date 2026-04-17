# Skill: Spec Testability Check

## Глоссарий

- **AC** (Acceptance Criteria, критерии приёмки) — проверяемый сценарий поведения в строгом формате:
  - **ДАНО** (precondition): состояние системы до действия — с конкретными числами, данными, URL, активной вкладкой
  - **КОГДА** (action): одно действие пользователя или события
  - **ТОГДА** (outcome): наблюдаемый результат с конкретными значениями (URL-параметры, количество элементов, содержимое, CSS-классы, аттрибуты)
  Один AC → один автоматический тест.
- **Feature** — атомарная единица функциональности, на которую можно написать AC (фильтр, кнопка, режим, edge case, tab).
- **Spec** — документ дизайна в `docs/superpowers/specs/`, описывающий функциональность + AC.
- **TZ** — техническое задание от заказчика, обычно в `docs/` вне `specs/`.
- **Gate** — проверка, блокирующая переход к следующему этапу pipeline.
- **Drift** — расхождение между спекой и кодом. Обработка drift описана в feedback-memory `feedback_spec_is_truth.md`.

## Назначение

Gate между `/brainstorm` и `/write-plan`: проверить, что спека содержит **тестируемые AC** для каждой фичи.

**Primary focus:** testability, не полнота описания. Полная описательная спека без AC хуже, чем краткая спека с AC, потому что в первом случае нельзя отличить "работает" от "не работает".

## Основное правило

**Feature без хотя бы одного тестируемого AC = дыра.**

Тестируемый AC — это блок с тремя явными частями:
- **Precondition** (ДАНО / GIVEN): состояние системы до действия
- **Action** (КОГДА / WHEN): конкретное действие пользователя или события
- **Outcome** (ТОГДА / THEN): наблюдаемый, проверяемый результат

Если **одна из трёх частей отсутствует** — AC не тестируем.
Если **outcome субъективен** ("работает корректно", "удобно", "быстро") — AC не тестируем.

## Когда запускать

- После `/brainstorm`, всегда, перед `/write-plan`.
- Работает и без исходного ТЗ — в таком случае проверяется только наличие AC в спеке, без cross-reference.

## Inputs

- `spec_path` (обязательно) — путь к спеке
- `tz_path` (опционально) — путь к исходному ТЗ; если есть, сравнивается feature list
- `ac_doc_path` (опционально) — путь к отдельному файлу с AC, если они вынесены из спеки

## Алгоритм

### Шаг 1: Extract Features

Читай `spec_path` (и `tz_path`, если есть). Составляй список "фич, которые должны иметь поведение" по правилам:

| Правило | Что становится feature |
|---|---|
| F1 | Каждый элемент списка фильтров/колонок/пунктов меню | один feature на элемент |
| F2 | Каждая вкладка / режим / состояние | один feature |
| F3 | Каждое действие пользователя (button, submit, drag, keypress) | один feature |
| F4 | Каждое автоматическое поведение (polling, websocket update, cache refresh) | один feature |
| F5 | Каждое состояние ошибки (network fail, 4xx, 5xx, timeout, empty) | один feature |
| F6 | Каждый edge case в разделе "Negative Path" / "Что не делать" | один feature |

Сохрани в `docs/coverage-runs/<date>-<spec-name>-features.json`:

```json
[
  {"id": "f-1", "source": "spec:315", "category": "filter", "name": "Filter: оператор"},
  {"id": "f-2", "source": "spec:315", "category": "filter", "name": "Filter: канал"},
  {"id": "f-3", "source": "spec:367", "category": "action", "name": "Drill-down: клик по строке открывает drawer"},
  ...
]
```

### Шаг 2: Extract AC

Читай `spec_path` и (если указан) `ac_doc_path`. Ищи AC-блоки по паттернам:

| Паттерн | Требования |
|---|---|
| P1 | Заголовок `## AC-N` или `### AC-N`, под ним блок с ДАНО/КОГДА/ТОГДА | три части обязательны |
| P2 | Блок `**ДАНО:** ... **КОГДА:** ... **ТОГДА:** ...` или английский аналог | три части обязательны |
| P3 | Нумерованный блок в формате `1. Дано ... 2. Когда ... 3. Тогда ...` | три части обязательны |

Каждый найденный блок → один AC. Сохрани в `docs/coverage-runs/<date>-<spec-name>-ac.json`:

```json
[
  {
    "id": "ac-1",
    "source": "spec:412",
    "given": "в таблице 20 строк: MTS=10, Beeline=7, Tele2=3",
    "when": "пользователь выбирает оператор=MTS и нажимает Применить",
    "then": "URL содержит ?operator=mts, в таблице 10 строк, все с operator=MTS",
    "testable": true
  }
]
```

### Шаг 3: Validate Testability

Для каждого AC проверь:

| Правило | Условие fail |
|---|---|
| T1 | Есть GIVEN часть | отсутствует → `testable=false`, `reason="no precondition"` |
| T2 | Есть WHEN часть | отсутствует → `testable=false`, `reason="no action"` |
| T3 | Есть THEN часть | отсутствует → `testable=false`, `reason="no outcome"` |
| T4 | THEN содержит конкретные значения (числа, строки, URL, имена классов, селекторы) | содержит только общие слова ("работает", "корректно", "удобно", "быстро") → `testable=false`, `reason="outcome is subjective"` |
| T5 | THEN не содержит слов "должен/должна/нужно" | содержит → `testable=false`, `reason="outcome describes intent, not observable state"` |
| T6 | WHEN описывает одно действие | цепочка действий ("логинится, переходит, кликает, ждёт") → `testable=partial`, `reason="multiple actions — split into several AC"` |

### Шаг 4: Match Features to AC

Для каждой feature ищи AC с пересекающимся содержанием. Ключевые существительные из `feature.name` должны встречаться в `ac.given|when|then`.

Статусы:
- **TESTED** — есть хотя бы один testable AC
- **DESCRIBED_NOT_TESTED** — есть нерабочий/нетестируемый AC
- **MISSING** — нет никаких AC

### Шаг 5: Report

Генерируй `docs/coverage-runs/<date>-<spec-name>-report.md`:

```
=== SPEC TESTABILITY REPORT ===
Spec:  docs/superpowers/specs/2026-04-17-network-statistics-analytics-monitoring-design.md
TZ:    docs/tz-statistics-analytics-monitoring.md
Date:  2026-04-17

TESTABILITY BY CATEGORY
┌────────────────┬────────┬────────┬───────────┬─────────┐
│ Category       │ Feats  │ Tested │ Described │ Missing │
├────────────────┼────────┼────────┼───────────┼─────────┤
│ mode           │ 3      │ 0      │ 0         │ 3    ❌ │
│ filter         │ 15     │ 0      │ 0         │ 15   ❌ │
│ grouping       │ 12     │ 0      │ 0         │ 12   ❌ │
│ column         │ 14     │ 0      │ 0         │ 14   ❌ │
│ drill-down     │ 8      │ 0      │ 0         │ 8    ❌ │
│ export         │ 3      │ 0      │ 0         │ 3    ❌ │
│ saved-view     │ 4      │ 0      │ 0         │ 4    ❌ │
│ empty-state    │ 4      │ 0      │ 0         │ 4    ❌ │
│ error-state    │ 4      │ 0      │ 0         │ 4    ❌ │
├────────────────┼────────┼────────┼───────────┼─────────┤
│ TOTAL          │ 67     │ 0      │ 0         │ 67   ❌ │
└────────────────┴────────┴────────┴───────────┴─────────┘

AC QUALITY
  Total AC blocks found:         0
  Testable (passed T1-T6):       0
  Non-testable:                  0
  Testability ratio:             N/A (no AC)

FEATURES WITHOUT ANY AC (top 20):
  [f-1]   Mode: Статистика (default)                    spec:325
  [f-2]   Mode: Аналитика                               spec:351
  [f-3]   Mode: Мониторинг                              spec:358
  [f-4]   Filter: period preset "Сегодня"               spec:101
  [f-5]   Filter: period preset "Вчера"                 spec:101
  [f-6]   Filter: period custom range                   spec:101
  [f-7]   Filter: grouping "по 5 минут"                 spec:106
  [f-8]   Filter: grouping "по 15 минут"                spec:106
  ...

GATE: FAIL
  Reason: 67/67 features missing AC (0% testable)
  Threshold: testable ratio >= 90% AND no critical missing
  
NEXT STEP:
  1. Write AC for each MISSING feature in docs/ac/<topic>.md
  2. Use format: ДАНО: ... КОГДА: ... ТОГДА: ... with concrete values
  3. Re-run /spec-coverage
```

### Шаг 6: Gate

**PASS условия (все):**
- Features with status TESTED >= 90% of total features
- No feature with category `critical` (explicitly marked or in "Критерии приёмки" of TZ) has status MISSING
- AC testability ratio >= 95% (из найденных AC — минимум 95% прошли T1-T6)

**PASS эффект:** В конец спеки дописывается:
```
---
## Testability Verification
- Checked on: 2026-04-17
- Features: 67 total, 62 tested, 5 described, 0 missing
- AC: 74 blocks, 72 testable (97.3%)
- Report: docs/coverage-runs/2026-04-17-X-report.md
```

**FAIL эффект:** Блокируется переход к `/write-plan`. Пользователь получает отчёт и возвращается к написанию AC.

---

## Жёсткие примеры

### Testable AC (проходит)

```markdown
### AC-12: Filter by operator
**ДАНО:**
  - В таблице 20 строк с распределением: operator=mts (10), beeline (7), tele2 (3)
  - Активный режим = "Статистика"
  - Фильтр "Оператор" пустой

**КОГДА:** пользователь выбирает в dropdown "Оператор" значение "MTS" и нажимает "Применить"

**ТОГДА:**
  - URL содержит `?operator=mts`
  - В таблице отображается 10 строк
  - Все отображаемые строки имеют колонку operator="MTS"
  - KPI-блок "Всего" показывает сумму total по этим 10 строкам
```

T1-T6 все проходят: есть GIVEN (конкретные числа), WHEN (одно действие), THEN (URL параметр, число строк, проверяемое содержимое).

### Non-testable AC (падает на T4)

```markdown
### AC-12: Filter by operator
**ДАНО:** таблица со строками
**КОГДА:** пользователь выбирает оператор
**ТОГДА:** фильтр работает корректно
```

Падает: T4 ("работает корректно" — субъективно), T5 (содержит "работает" в смысле intent).

### Non-AC (не найдётся Шагом 2)

```markdown
### Filter by operator
Фильтр "Оператор" позволяет выбрать одного оператора из списка. 
При выборе таблица обновляется.
```

Это **описание**, а не AC. Нет GIVEN/WHEN/THEN структуры. Не распознаётся P1/P2/P3. Значит, feature "filter-operator" имеет статус MISSING в отчёте.

---

## Критика этого скилла (читать)

1. **Механическое извлечение features — компромисс.** Правило F1 ("каждый элемент списка — feature") породит шум для декоративных списков. Компенсация: категоризация по keywords в `feature.name` помогает отфильтровать; пользователь может вручную пометить `"ignore": true` в `features.json`.

2. **Не ловит "неправильные" AC.** AC "ДАНО: 10 строк, КОГДА: клик, ТОГДА: 100 строк" пройдёт T1-T6, но бизнес-логика неправильная. Это не проблема скилла — это проблема review. Человек читает AC на гейте №1 (в brainstorming).

3. **T4 (конкретные значения) эвристична.** AC "ТОГДА: появляется модалка с заголовком 'Удалить?'" содержит конкретное ("Удалить?") — пройдёт. AC "ТОГДА: появляется подтверждение" — не пройдёт. Разница тонкая, но по сути честная: второе не даёт оракула для теста.

4. **Стоимость: AC на каждую feature — это работа.** 67 features = 67 AC блоков = ~1 день на написание. Но этот день экономит 23 fix-коммита и несколько дней хаоса.

5. **Скилл не заменяет человеческий review спеки.** Он гарантирует только структурную тестируемость. Качество AC (правильные ли числа в GIVEN, правильный ли outcome в THEN) — задача человека и/или последующего code review.

## Что скилл НЕ делает

- Не пишет AC автоматически. Это работа brainstorming-агента (или человека) на основе ТЗ.
- Не пишет тесты. Тесты — следующий этап (TDD-скилл или Playwright-генератор).
- Не оценивает корректность бизнес-логики, только наличие проверяемого критерия.

## Интеграция

Пока `/brainstorm` и `/write-plan` являются плагинскими скиллами (не модифицируемыми), используй вручную:

1. `/brainstorm` → спека готова
2. `/spec-coverage spec=...` → отчёт
3. Если FAIL → допиши AC → goto 2
4. Если PASS → `/write-plan`
