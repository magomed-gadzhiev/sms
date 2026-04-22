# API Client & Subaccount Review — Design Spec

**Дата:** 2026-04-21
**Статус:** SUPERSEDED by `2026-04-21-api-review-v2-umbrella.md` (2026-04-21). Оставлен как история решений, исполнять нельзя.
**Форма деливеры:** γ — мелочь фиксим в PR ревью, крупное выносится в отдельные планы

## 1. Скоуп и границы

### В скоупе

**Транспорты:**
- HTTP — `internal/gateway/client/` (внешний интеграционный API, не портал).
- SMPP — `internal/gateway/smpp/` (gateway + `auth_adapter`) и `internal/smpp/` (протокольный уровень + сервер).

**Роли:**
- Обычный клиент.
- Субаккаунт-ребёнок (`is_reseller = false`, `parent_client_id != NULL`).
- Субаккаунт-родитель / агрегатор (`is_reseller = true`).

Дискриминатор ролей — `is_reseller` + `parent_client_id`, согласно зафиксированному решению 2026-04-20 (memory `project_aggregator_decisions.md`). `account_type` использовать запрещено, устаревшие Phase 1 планы игнорировать.

**Оси ревью:**
- **A1** — OpenAPI ↔ HTTP-код соответствие.
- **A2** — единый формат ошибок (HTTP envelope + SMPP ESME_* карта).
- **A3** — идемпотентность write-операций (HTTP `Idempotency-Key`, SMPP дедуп).
- **A6** — SMPP-контракт (поддерживаемые TLV, `data_coding`, длинные сообщения, DLR, `registered_delivery`).
- **C** — сквозная корректность поведения субаккаунта через HTTP и SMPP (биллинг, видимость ресурсов, DLR-роутинг).

### Вне скоупа

- Portal HTTP API (`internal/gateway/portal/`) — UI-бекенд, не интеграционный.
- Admin gateway.
- Общий security review (B из вопроса брейншторма). Tenant-isolation берём только в части субаккаунта; остальной auth/scopes/rate-limiting — отдельная работа.
- A4 (пагинация) и A5 (версионирование API) — отдельные итерации.
- Performance / нагрузочное тестирование.

### Критерий «мелочь vs крупное» (форма γ)

Основа — объём diff'а (ε):
- **Мелочь** = ≤50 строк, 1 файл, без миграций, без новых зависимостей. Фиксим в том же PR.
- **Крупное** = всё остальное. Выносится в отдельный план через `superpowers:write-plan`.

**Хард-гейт поверх ε:** любой diff, трогающий `internal/services/billing/` или `internal/services/tarification/` — автоматически крупное, независимо от размера. Биллинговые регрессии дороже любого удобства.

## 2. Фазы ревью

Фазы выполняются строго последовательно: 0 → A1 → A2 → A3 → A6 → C → F.

### Фаза 0 — карта поверхности

Собрать единый inventory всех роутов и SMPP-команд:

- HTTP: все `router.HandleFunc` из `internal/gateway/client/router/router.go`.
- SMPP: все обрабатываемые `command_id` в `internal/gateway/smpp/server/handler.go` и `internal/smpp/server/handler.go`, плюс поддерживаемые TLV из `internal/smpp/protocol/constants.go`.

**Артефакт:** `docs/reports/2026-04-21-api-surface-inventory.md` — таблица `путь_или_PDU | файл:строка | service-call | что пишет в БД | что возвращает`.

Эта таблица — основа для всех следующих фаз.

### Фаза A1 — OpenAPI ↔ код

Сверка `docs/openapi.yaml` (точное местоположение определяет Фаза 0) с реальными хендлерами из inventory.

Для каждого расхождения решение «правим спеку или правим код» зависит от того, что ближе к используемому интеграторами контракту:
- Если эндпоинт в проде и клиенты им пользуются — код source of truth, спеку подгоняем.
- Если расхождение в формате ответа и спеку никто не реализовывал клиентом — правим код под спеку.

Guard: unit-тест, который при старте читает `openapi.yaml` и проверяет, что каждый задокументированный путь зарегистрирован в router'е и vice versa.

### Фаза A2 — единый формат ошибок

**HTTP.** Canonical envelope:
```
{
  "code": "<machine_readable_token>",
  "message": "<human_readable>",
  "details": { ... },     // optional, structured
  "request_id": "<uuid>"  // всегда
}
```

Привести все хендлеры `internal/gateway/client/handlers/*` к этому envelope'у. Найти через grep по `WriteHeader`, `http.Error`, `json.NewEncoder(w).Encode`.

**SMPP.** Зафиксировать таблицу `бизнес-ошибка → ESME_* код` (ESMEROK, ESMERINVMSGLEN, ESMERINVSRCADR, ESMERINVDSTADR, ESMERSYSERR и т.д.). Список ESME_* фиксирован SMPP v3.4 + vendor TLV, произвольно не расширяется. Единое место мапинга — `internal/smpp/server/errors.go`.

**Deliverable A2:** `docs/specs/api-error-contract.md` с обеими таблицами.

### Фаза A3 — идемпотентность

**HTTP write-операции.** POST на `/sms/send`, `/sms/batch`, `/cascade/deliveries`, `/webhooks`, `/lookup`, `/lookup/bulk`, `/templates`.

Проверить поддержку `Idempotency-Key` header. Если отсутствует — это **крупное** (требует: таблица `idempotency_keys` с TTL, миграция, middleware, политика expiration). План выносится отдельно.

**SMPP submit_sm.** Проверить дедуп по `user_message_reference` или собственному хранилищу `message_id` с TTL. Текущее поведение задокументировать, недостающее — отдельный план.

### Фаза A6 — SMPP-контракт

Публичный документ уровня «наша поддержка SMPP v3.4»:

- Таблица поддержанных PDU (bind_transmitter/receiver/transceiver, unbind, submit_sm, deliver_sm, enquire_link, generic_nack, query_sm).
- Таблица поддержанных TLV из `constants.go`.
- `data_coding` мапинг: 0 (GSM 7-bit default), 3 (Latin-1), 8 (UCS-2) — и что с остальными.
- Длинные сообщения: UDH vs `sar_msg_ref_num` / `sar_total_segments` / `sar_segment_seqnum` TLV — что принимаем, что рекомендуем.
- `registered_delivery` семантика: какие биты вызывают DLR, на какой этап (final vs intermediate).
- DLR-формат: текст deliver_sm body, поля `id`, `stat`, `err`, `dlvrd`, `sub`, `submit date`, `done date`.

**Deliverable A6:** `docs/specs/smpp-contract.md`.

### Фаза C — корректность субаккаунта сквозь оба транспорта

Для каждой write-операции из inventory проверить:

1. **C-billing.** В записи тарификации `client_id` = тот, чьим API-ключом или SMPP-bind'ом пришёл запрос (НЕ parent). Для субаккаунта-ребёнка при активном dual-charge (Phase 2, memory) — также запись в `aggregator_margin_log` на родителя с корректной маржой.
2. **C-visibility.** GET/list эндпоинты (шаблоны, webhooks, sender-имена, history) возвращают субаккаунту-ребёнку только его ресурсы. Ни родителя, ни других детей родителя не видно, если явно не share'нуто.
3. **C-dlr-routing.** DLR-callback улетает на `webhook_url` того клиента, чей `client_id` в оригинальной записи сообщения, не parent'а.

Dual-charge Phase 2 (из memory) — если не подключён, это **крупное**, отдельный план. Без dual-charge всё ревью C-billing для агрегатора показывает красное, это ожидаемо.

### Фаза F — финализация

- Сборка итоговой матрицы покрытия.
- Сводный отчёт `docs/reports/2026-04-21-api-review-findings.md` — critical/major находки с указанием куда уехал фикс (PR или план).
- Для каждого крупного пункта — `superpowers:write-plan` создаёт отдельный план.

## 3. Матрица покрытия (центральный deliverable)

**Файл:** `docs/reports/2026-04-21-api-coverage-matrix.md` + машиночитаемая `.csv`.

**Схема строки:**

| endpoint_or_pdu | transport | role | axis | current_state | test_file | gap | severity | action |

**Поля:**

- `endpoint_or_pdu` — `POST /sms/send`, `SMPP submit_sm`, `SMPP deliver_sm (DLR)`, и т.д.
- `transport` — `HTTP` / `SMPP`.
- `role` — `client` / `subaccount_child` / `subaccount_parent`.
- `axis` — `A1` / `A2` / `A3` / `A6` / `C-billing` / `C-visibility` / `C-dlr-routing`.
- `current_state` — `OK` / `DRIFT` / `MISSING` / `BROKEN` / `UNKNOWN`. `UNKNOWN` запрещено маскировать под `OK`.
- `test_file` — путь к тесту, покрывающему клетку. Пусто = нет теста.
- `gap` — короткое описание дыры.
- `severity` — `critical` / `major` / `minor`.
- `action` — `fixed-in-PR#<N>` / `plan:<path>` / `test-added:<path>` / `wontfix:<reason>`.

**Правила:**

- Матрица пишется инкрементально по фазам, не одним куском в конце.
- Снапшот на дату ревью — **не** живой документ. Обновляется только при следующем ревью. Живая защита от регрессий — тесты, на которые матрица ссылается.
- Для `C-billing` конкретная проверка: после вызова в `tarification_log` `client_id` = отправителя. Для `subaccount_parent` дополнительно — запись в `aggregator_margin_log`.
- Для `C-visibility` конкретная проверка: subaccount_child с валидным ключом не видит ресурсы родителя в list-эндпоинтах.

Грубая оценка размера: ~21 endpoint × ~1.3 транспорта × 3 роли × ~3 оси ≈ **245 строк**. Читается по фильтру `severity=critical` или `current_state in (MISSING, BROKEN)`.

## 4. Стратегия тестирования

Три уровня, разные цели.

### Unit (`*_test.go` рядом с кодом)

- A1: парсеры/валидаторы request-body, мапперы response.
- A2: хендлеры ошибок, мапинг бизнес-ошибка → HTTP-код или ESME_*. Тест-таблица: добавление новой ошибки без записи = провал.
- A3: middleware идемпотентности с mock-хранилищем.
- A6: PDU encode/decode, TLV-парсинг, UDH-сборка — частично есть (`pdu_test.go`, `pdu_helper_test.go`, `encoder_test.go`, `decoder_test.go`), дополняем.

Не дублировать уже существующие тесты из `git status`.

### Integration (`*_integration_test.go` с build-тегом `integration`)

Поднимает реальные pgx + redis. Запуск: `go test -tags=integration ./...`.

- C-billing: submit через HTTP и SMPP, проверка записи в `tarification_log` и `aggregator_margin_log`.
- C-visibility: два разных `client_id` (родитель и ребёнок), list-эндпоинты не пересекаются.
- C-dlr-routing: симуляция DLR → webhook летит правильному клиенту.
- A3: идемпотентность через реальный Redis/Postgres (ретрай возвращает кешированный ответ).

Build-тег обязателен, иначе `scripts/check.sh` без Postgres упадёт.

**Риск:** если Фаза 0 обнаружит, что integration-инфры нет в CI — слой придётся упростить до unit с fakes. Принимается как возможный поворот.

### E2E (`e2e/tests/integration-api/*.spec.ts` — новая директория)

- Живой HTTP через API-ключ к client-gateway.
- Живая SMPP-сессия к smpp-gateway.
- Сквозные сценарии: SMPP submit → webhook DLR → HTTP balance check, всё в одном клиентском контексте.
- Запуск с сервера (memory: «Run tests from server»).

### Чего не делаем

- Mock-heavy integration-тестов (unit под другим именем).
- Тестов-копий логики хендлера.
- Тестов для `wontfix` / `UNKNOWN` клеток.

### Ratchet

После каждой фазы фиксируем число `OK`-строк с непустым `test_file` в заголовке матрицы. Следующая фаза не уменьшает это число. Автоматизация не в скоупе.

## 5. Артефакты и коммит-стратегия

### Артефакты

1. `docs/reports/2026-04-21-api-surface-inventory.md` — Фаза 0.
2. `docs/reports/2026-04-21-api-coverage-matrix.md` + `.csv` — инкрементально.
3. `docs/specs/api-error-contract.md` — Фаза A2.
4. `docs/specs/smpp-contract.md` — Фаза A6.
5. `docs/reports/2026-04-21-api-review-findings.md` — сводный отчёт Фазы F.
6. `docs/superpowers/plans/2026-04-21-api-review-<topic>.md` — отдельные планы на каждое крупное.
7. Тесты: unit рядом с кодом, integration с тегом, e2e в `e2e/tests/integration-api/`.

### Коммит-стратегия

Каждая фаза = серия коммитов в таком порядке:

1. `chore(api-review): <фаза> inventory/matrix rows` — только документы, без кода.
2. `fix(api): <конкретная мелочь>` × N — один фикс = один коммит. Rationale ссылается на строку матрицы.
3. `test(api): <фаза> coverage` — unit/integration/e2e тесты.
4. `docs(api-review): <фаза> findings` — обновление findings-документа.
5. Для крупного: `docs(plan): api review — <topic>` — отдельный план. Код не коммитится.

### Ветвление: per-фаза PR

Каждая фаза — **отдельный PR к master**, сливается после APPROVED код-ревьюером, следующая фаза начинает от обновлённого master'а. Одна большая PR на 30+ коммитов — отклонено как нечитаемая.

### Review-gate

Каждая фаза → `/execute-with-review` (из CLAUDE.md). Код-ревьюер видит diff только этой фазы. Обход `--no-verify` запрещён.

### Чего не будет

- AC-файлов (дублирование со спеками dual-charge).
- Release-notes / changelog.
- Диаграмм (если только точечно не понадобятся для SMPP-контракта).

## 6. Открытые риски

- **Integration-инфра.** Если в CI нет Postgres/Redis для `-tags=integration` — Фаза C теряет половину проверок. Разведка — в Фазе 0, до старта A1.
- **Dual-charge Phase 2.** Если не доделан, C-billing для агрегатора стабильно красный. Это ожидаемо, выносится отдельным планом, не блокирует остальные фазы.
- **Размер матрицы.** 245 строк — ручной труд на заполнение. Риск выгорания к Фазе C. Митигация: матрицу заполняем инкрементально, а не в конце. Если к Фазе C обнаружится, что матрица не даёт value — упрощаем схему задним числом (меньше строк, группировка по эндпоинту).
- **OpenAPI spec location.** Если `openapi.yaml` сгенерирован из кода, а не написан вручную — Фаза A1 бессмысленна (drift невозможен). Разведка в Фазе 0.
