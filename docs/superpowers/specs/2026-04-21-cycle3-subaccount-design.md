# Cycle 3 (Subaccount e2e) — Design Spec

**Parent:** `docs/superpowers/specs/2026-04-21-api-review-v2-umbrella.md`
**Дата:** 2026-04-21
**Форма:** γ (из umbrella). Pure-audit — **никаких** мелких кодовых фиксов в PR ожидать не надо (все C-дыры требуют integration-изменений, выносятся в fix-планы).

## Скоуп

**Оси:** C-billing, C-visibility, C-dlr-routing.
**Транспорты:** HTTP, gRPC-external, SMPP.
**Роли:** client, subaccount_child, subaccount_parent.
**Ожидаемый размер матрицы:** ~90 строк.

## Режим

Cycle 3 идёт в **audit-only** режиме (решение 2026-04-21 брейншторм):
- Статус Cycle 3 = NO-GO per umbrella §1.7 (B1/B2/B3 open). Но γ-форма позволяет аудит без integration-тестов.
- Тесты не пишем. Fake-репозитории не пишем.
- Клетки, уже охарактеризованные в Cycle 1/2 — **reference-строки** с ссылкой на предыдущий finding. Без дублирования характеристики.
- Только новые наблюдения разбираем deep dive в findings.

## Предопределённые клетки (reference-only)

Не тратим time-to-investigate, прямо ссылаемся:

- **C-billing / subaccount_parent × any** — B3 (dual-charge flag OFF by default). current_state=BROKEN, gap="dual-charge не выполняется в prod config", action=`plan:2026-04-21-fix-dual-charge-default-enable.md` (stub).
- **C-visibility / SMPP × subaccount_child** — B2 (client_id=user_id kludge). current_state=BROKEN, gap="see Cycle 2 F3 A7 kludge", action=`plan:2026-04-21-fix-smpp-auth-password.md` (B2 закрывается частично через auth-план; отдельно client-lookup тоже нужен).
- **C-dlr-routing / SMPP inbound via deliver_sm** — Cycle 2 A6-dlr. current_state=BROKEN, gap="deliver_sm не различает MO/DLR", action=`plan:TBD (part of KafkaMessage refactor)`.
- **C-dlr-routing / SMPP outbound via smppv1.DeliverDLR** — Cycle 2 (работает + tests). current_state=OK, test_file=`internal/gateway/smpp/server/grpc_server_test.go`.
- **C-billing / gRPC-external × any role** — A7.3 (stub interceptor → dummy tenant). current_state=BROKEN, gap="all gRPC calls = same dummy client_id", action=`plan:2026-04-21-fix-grpc-auth-interceptor.md`.

## Новые поверхности для deep dive

- **HTTP C-billing** — `POST /sms/send` / `/batch` / `/cascade/deliveries` / `/lookup` / `/lookup/bulk`. Проверить: `client_id` в `tarification_log` после charge = тот, что в ctx (ValidateToken), не из request body. Handler-level enforcement Cycle 2 A7.2 показал корректность HTTP — но downstream service SQL? ON CONFLICT по client_id в `messages` таблице — фильтрует?
- **HTTP C-visibility** — GET/list. Cycle 2 A7.2 показал `req.ClientId` в downstream всегда overridden via ctx. Но: в `GetMessageHistory` / `ListTemplates` / `ListWebhooks` — SQL `WHERE client_id = $1`? Parent vs child параметр — пробрасывается как есть, или есть фильтр на subaccount_child что он видит только свои?
- **gRPC-external C-visibility** — теоретически BROKEN через stub, но внутри downstream — ещё раз проверить что фильтр есть.
- **HTTP C-dlr-routing** — `POST /webhooks`: при создании — привязка к authenticated client_id, не из request. При доставке DLR — какой `webhook_url` используется: клиента-отправителя или parent'а? Если sub-account создаёт webhook, шлёт сообщение, DLR приходит — webhook уходит ребёнку, не родителю? Непроверено.
- **SMPP C-billing** — submit_sm → Kafka → billing. Reference Cycle 2 KafkaMessage.Metadata drop мета-находку, но проверить: `client_id` из SMPP session (несмотря на B2 kludge) доходит до billing корректно? Или там тоже dummy?

## Артефакты

- `docs/reports/2026-04-21-cycle3-subaccount-matrix.md` + `.csv` — матрица ~90 строк.
- `docs/reports/2026-04-21-cycle3-findings.md` — текстовые findings по C-axis.
- 0-3 новых fix-плана при находках.

## Ветка + PR

- Ветка `review/api-v2-cycle3-subaccount` (уже создана).
- Один PR к master.
- Коммиты: scaffold → C-billing → C-visibility → C-dlr-routing → finalize.

## Выход

После merge Cycle 3 — **полный ревью v2 завершён**. Priority queue fix-планов (из всех 3 циклов) готов к реализации по приоритету пользователя.
