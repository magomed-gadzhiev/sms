# UX Full Re-Audit — Design

**Дата:** 2026-04-29
**Скоуп:** D (полный re-audit с нуля, fix mode + Infrastructure Check + QA full, по всем существующим бизнес-ролям)
**Single source of truth для исполнителей этапов после `/clear`.**

---

## 1. Цель и скоуп

Провести глубокий end-to-end аудит SMS-платформы (Go-микросервисы + React-портал) по всем 30 модулям, перечисленным в разделе 4. Каждый модуль обходится в режиме `fix` скилла `skills/ux.md` с QA Depth `full` и `Infrastructure Check: yes`. Найденные баги исправляются немедленно через обязательный wrapper `/execute-with-review` (CLAUDE.md), а не "fix-on-the-fly". Тестирование первично через браузер (Playwright MCP), верификация — API + PostgreSQL.

**Жертва, признанная явно:** часть из 30 модулей уже прошла аудит на 2026-04-22/23 (см. историю в `docs/ux-audit-progress.md`); пользователь сознательно выбрал скоуп D, понимая что это переделка. Этот документ не оспаривает выбор, фиксирует его.

**Не входит в скоуп:**
- Рефакторинг auth-слоя ради перевода БД-ролей (`admin`/`superadmin`/`client`) на бизнес-имена (`admin`/`aggregator`/`user`/`subaccount`). Маппинг ведётся в доках, см. §2.
- Покрытие фичи "operator-роль" (её нет в коде, только GSM-операторы как сущности).
- Нагрузочное тестирование пайплайна (отдельные специализации, см. spec 008).

---

## 2. Роли

Используем **бизнес-терминологию** во всех тестовых сценариях, прогресс-файле и баг-репортах:

| Бизнес-роль | Описание | Технически в БД |
|---|---|---|
| `admin` | Системный администратор платформы | `users.role = 'admin'` или `'superadmin'` |
| `aggregator` | Клиент-реселлер, видит панель `/portal/network`, может создавать саб-клиентов | `users.role = 'client'` + `clients.is_reseller = true` |
| `user` | Рядовой клиент: кампании, контакты, баланс, Quick Send | `users.role = 'client'` + `is_reseller = false` + `parent_account_id IS NULL` |
| `subaccount` | Саб-клиент агрегатора | `users.role = 'client'` + `parent_account_id IS NOT NULL` |

Маппинг закреплён в коммите [`bc7f556`](#) (`docs(skills): выровнять терминологию ролей в ux/security-audit скиллах`). Тесты RBAC проверяют видимость данных и доступность action'ов **по бизнес-роли**, а не по enum-значению `users.role`.

---

## 3. Структура этапа

Каждый из 30 этапов проходит идентичный цикл:

```
1. ОТКРЫТЬ ЭТАП
   - Прочитать docs/ux-audit-progress.md, последние 200 строк + grep [IN_PROGRESS]
   - Если [IN_PROGRESS] чужой — стоп (lock).
   - Записать собственный [IN_PROGRESS] этой строкой:
     "## [IN_PROGRESS] Этап N/30: <модуль> (<роль>, <URL>, fix + Infra + QA full, YYYY-MM-DD)"
   - Закоммитить progress-коммит с lock.
   - Если этап требует чистого стенда — применить test/load/fixtures/demo_seed.sql.

2. ИНВЕНТАРИЗАЦИЯ (skills/ux.md секция 0)
   - Прочитать исходники: роуты, кнопки, формы, диалоги, табы, dropdown'ы, условные рендеры, таблицы с действиями.
   - Составить матрицу TC: happy / negative / edge / state-transition.
   - TodoWrite — одна задача на TC.
   - Проверить полноту: пустые/единственный/максимальный, цепочки CRUD, статусные переходы, оба состояния условных рендеров.

3. ПРОГОН (Playwright MCP — браузер; psql + curl — верификация)
   - Happy → API verify (GET) → DB verify (SELECT).
   - Negative (10 обязательных по skills/ux.md §3.1 + специфичные модулю).
   - Boundary Value Analysis (skills/ux.md §A).
   - State transition matrix (skills/ux.md §B), если в модуле есть статусные сущности.
   - Three-tier consistency UI=API=DB (skills/ux.md §C) после каждой мутации.
   - RBAC: попытаться достать данные другой роли/тенанта.
   - Infrastructure Check: эндпоинты, миграции, Redis-кэш, Kafka-консьюмеры, gRPC-контракты.

4. БАГИ (на каждый найденный)
   - Записать BUG-N по формату skills/ux.md (UI + API + DB доказательства).
   - Запустить /execute-with-review с конкретным фиксом.
   - После APPROVED commit'а — перепрогнать соответствующие TC.
   - Если фикс ломает соседний TC — это новый BUG, новая итерация.
   - Лимит: 3 review-цикла на 1 баг; на 4-м — эскалация.
   - Лимит на этап: >10 багов или >5 фикс-итераций — закрыть [DONE] частично, открыть N.5/30 с остатком.

5. ЗАКРЫТИЕ ЭТАПА
   - Сменить [IN_PROGRESS] → [DONE] в прогресс-файле + полный отчёт (формат §5.1).
   - Закоммитить progress-коммит закрытия.
   - /clear.
   - Следующий этап.
```

**Между TC внутри одного этапа `/clear` НЕ делаем** — потеряется матрица в TodoWrite. Только на границах этапов.

---

## 4. Порядок этапов

| #  | Модуль                                       | Роль(и)         | URL/Scope                                              | Свежие изменения, на которые обратить внимание                  |
|----|----------------------------------------------|-----------------|--------------------------------------------------------|------------------------------------------------------------------|
| 1  | Auth: login / logout / session               | все 4           | `/login`, JWT/session middleware                       | —                                                                |
| 2  | Auth: register / password-reset / 2FA        | user            | `/register`, `/password-reset`                         | —                                                                |
| 3  | Admin: countries + operators                 | admin           | `/admin/countries`                                     | MCC/MNC schema, seed СНГ (commit 85f2e85)                        |
| 4  | Admin: providers + connections               | admin           | `/admin/providers`, `/admin/connections`               | Edit-визард providers (MVP-feedback №3, 1414b8b/b8e0d6c)         |
| 5  | Admin: routes + client-routes                | admin           | `/admin/routes`, `/admin/client-routes`                | MCC/MNC routing foundation (85f2e85)                             |
| 6  | Admin: HLR                                   | admin           | `/admin/hlr`                                           | —                                                                |
| 7  | Admin: tariffs + tarification                | admin           | `/admin/tariffs`, `/admin/tarification`                | —                                                                |
| 8  | Admin: legal-entities + contracts            | admin           | `/admin/legal-entities`, `/admin/contracts`            | —                                                                |
| 9  | Admin: clients (CRUD + блокировка)           | admin           | `/admin/clients`                                       | модалка блокировки + бейдж (MVP-feedback №4, 1d29699)            |
| 10 | Admin: aggregators (sub-accounts UI)         | admin           | `/admin/aggregators`                                   | —                                                                |
| 11 | Admin: sender-names review/approve           | admin           | `/admin/sender-names`                                  | —                                                                |
| 12 | Admin: operator-templates                    | admin           | `/admin/operator-templates`                            | —                                                                |
| 13 | Admin: webhooks + billing настройки          | admin           | `/admin/webhooks`, `/admin/billing`                    | operation_kind (MVP-feedback №5, dcbab7f)                        |
| 14 | Admin: users + settings                      | admin           | `/admin/users`, `/admin/settings`                      | —                                                                |
| 15 | Admin: monitoring + analytics + audit-log    | admin           | `/admin/monitoring`, `/admin/analytics`, `/admin/audit-log` | sort by status в messages (MVP-feedback №10, 5424ee2)        |
| 16 | Admin: detalization (deprecated)             | admin           | `/admin/detalization`                                  | проверить корректность deprecation (5424ee2)                     |
| 17 | User: dashboard + profile + balance          | user            | `/portal/dashboard`, `/portal/profile`, `/portal/billing` | —                                                            |
| 18 | User: companies + contacts + segments        | user            | `/portal/companies`, `/portal/contacts`, `/portal/segments` | —                                                          |
| 19 | User: sender-names + templates               | user            | `/portal/sender-names`, `/portal/templates`            | —                                                                |
| 20 | User: channels + delivery-strategies         | user            | `/portal/channels`, `/portal/delivery-strategies`      | каскад каналов (spec 012)                                       |
| 21 | User: campaigns + campaign-schedules         | user            | `/portal/campaigns`, `/portal/campaign-schedules`      | default sender + Variant B summary (MVP-feedback №9, a3e8446)    |
| 22 | User: quick-send                             | user            | `/portal/quick-send`                                   | —                                                                |
| 23 | User: messages + cascade-history             | user            | `/portal/messages`, `/portal/cascade-history`          | —                                                                |
| 24 | User: lookup + analytics                     | user            | `/portal/lookup`, `/portal/analytics`                  | —                                                                |
| 25 | User: api-keys + webhooks + notifications    | user            | `/portal/api-keys`, `/portal/webhooks`, `/portal/notifications` | —                                                       |
| 26 | Aggregator: /network/* + sub-accounts        | aggregator      | `/portal/network/*`, `/portal/sub-accounts`            | —                                                                |
| 27 | Subaccount: портал под parent                | subaccount      | `/portal/*` под subaccount-сессией                     | проверка что суб-клиент не видит/не правит чужое                 |
| 28 | Cross-cutting: RBAC + права доступа          | все 4           | API-уровень (cross-tenant попытки)                     | sender-names admin endpoint от user → 403; и т.д.                |
| 29 | Cross-cutting: error states + i18n + a11y    | все 4           | глобальные fallback'и, ErrorBoundary, ru/en           | заодно гейт по spec 006 (a11y)                                   |
| 30 | E2E: создание клиента → отправка → биллинг → отчёт | admin+user | связка ролей                                       | финальная регрессия пайплайна Kafka → DLR → invoice              |

---

## 5. Контракт между этапами

### 5.1. Формат записи в `docs/ux-audit-progress.md`

**Lock на старте** — одна строка наверху файла:
```markdown
## [IN_PROGRESS] Этап N/30: <Название> (<роль(и)>, fix + Infrastructure + QA full, YYYY-MM-DD)
```

**Закрытие этапа** — превращение строки в `[DONE]` + полный отчёт:
```markdown
## [DONE] Этап N/30: ... — YYYY-MM-DD

[Summary] N тест-кейсов, X PASS, Y FAIL, Z багов (C critical, H high, M med, L low)

[BUG LIST]
BUG-N1: <название> — Severity: ... — Категория: ...
  Шаги/Ожидалось/Получилось/Доказательство (UI+API+DB)/Фикс: <commit-sha или эскалирован>
...

[Success Path] <идеальный сценарий одним абзацем>

[Recommendations] <топ-3>

[Test Data]
- <id, тип сущности, роль, для каких TC создан>
```

### 5.2. Типы коммитов на этап

| Тип | Когда | Сообщение | Через |
|---|---|---|---|
| **fix** | каждый approved review-цикл | `fix(<модуль>): <описание> [BUG-N этап M]` | `/execute-with-review` |
| **progress (lock)** | старт этапа | `docs(audit): этап M/30 <модуль> — [IN_PROGRESS]` | обычный commit (только progress-файл) |
| **progress (close)** | конец этапа | `docs(audit): этап M/30 <модуль> — [DONE], N багов исправлено` | обычный commit |
| **test-data** | если seed расширяется | `test(seed): добавить <X> для этапа M/30` | обычный commit |

Pre-commit hook прогоняется на всех. `--no-verify` запрещён.

### 5.3. Протокол восстановления контекста после `/clear`

1. Прочитать `CLAUDE.md` + `MEMORY.md`.
2. Прочитать `docs/superpowers/specs/2026-04-29-ux-full-reaudit-design.md` (этот документ).
3. Прочитать `docs/ux-audit-progress.md`, последние 200 строк.
4. `grep [IN_PROGRESS]` в прогресс-файле:
   - есть → продолжаем застрявший этап.
   - нет → берём следующий не-`[DONE]` номер из таблицы §4.
5. При необходимости — `git log --since=<дата старта этапа> --oneline` для свежего контекста.
6. Запускаем стандартный цикл §3.

### 5.4. Эскалация — стоп и запрос пользователю

| Триггер | Действие |
|---|---|
| 3-й review-цикл по одному багу — CHANGES_REQUESTED | стоп |
| Spec-drift: фикс требует изменения AC из `specs/<NNN>/spec.md` | стоп |
| Миграция БД нужна для фикса | стоп (пользователь решает применять ли в проде) |
| Security-баг (auth-bypass, IDOR, leakage чужих данных) | стоп, отдельный канал |
| Стенд лежит >15 минут | стоп |
| Архитектурное решение, затронет >1 модуля | стоп |

В эскалации: что нашёл, что предлагаю, что блокирует. Жду ответа.

### 5.5. Лимит глубины этапа

>10 багов или >5 фикс-итераций → закрыть `[DONE]` частично с пометкой "частичный, остальное в N.5/30", открыть `N.5/30` с остатком.

### 5.6. Test data

- **Базовый seed** — `test/load/fixtures/demo_seed.sql` (untracked). Применяется один раз перед этапом 1, далее перед этапами, требующими чистого стенда (e2e, billing-регрессия). Факт применения фиксируется в прогресс-файле.
- **Доп. данные на этап** — приоритет UI, фолбэк API, последний фолбэк SQL. Регистрируются в `[Test Data]` отчёта этапа.
- **Чистка между этапами** — НЕ делаем (накопление полезно для регрессии). Полный re-seed только если этап явно требует.

---

## 6. Ограничения и жертвы (явно)

1. **Переделка [DONE] от 2026-04-22/23.** Большая часть из 30 модулей уже прошла QA full недавно. Скоуп D — осознанное решение пользователя. Эту жертву признаём.
2. **Фактическая длительность не оценена.** Ориентир: один этап = от часов до полного дня. 30 этапов = недели календарного времени, при условии что один поток без перерывов.
3. **Один поток, последовательно.** Lock в `[IN_PROGRESS]` запрещает параллельность. Это приоритет надёжности над скоростью.
4. **Все правки кода через `/execute-with-review`.** Медленнее, чем "fix-on-the-fly" из режима `fix` skills/ux.md, но это требование CLAUDE.md и оно выше.
5. **Архитектурные правки за рамками одного модуля** — стоп и эскалация. Этот аудит не реструктурирует систему.

---

## 7. Критерии успеха

Аудит считается завершённым, когда:
- 30/30 этапов в прогресс-файле имеют статус `[DONE]` (или `[DONE] частичный` + соответствующий `[DONE] N.5/30`).
- 0 строк `[IN_PROGRESS]`.
- Каждый `BUG-N` либо имеет `commit-sha` фикса, либо явную пометку «эскалирован, ticket: …» / «вынесен в follow-up: …».
- Финальный сводный коммит: `docs(audit): full re-audit — итоги` со ссылкой на этот дизайн-док.

---

## 8. Открытые вопросы

На момент старта пусто. Пополняется по мере эскалаций (§5.4) — каждая эскалация → строка здесь со статусом `OPEN`/`RESOLVED`.
