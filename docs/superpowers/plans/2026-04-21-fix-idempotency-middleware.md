# Fix Plan: Idempotency-Key middleware

**Severity:** critical
**Source:** Cycle 1 finding A3 (`docs/reports/2026-04-21-cycle1-findings.md`)
**Spec reference:** `docs/superpowers/specs/2026-04-21-api-review-v2-umbrella.md` §2 axis A3

## Problem

Нет middleware Idempotency-Key ни в `internal/gateway/client/middleware/`, ни в `internal/api/middleware/`. Downstream billing-guard (`commit_idempotency_guard` с `ON CONFLICT`) keyed by `message_id`, который gateway генерирует через `uuid.New()` на каждый входящий запрос. Retry = новый UUID = обход guard'а = двойное списание.

Все 3 транспорта небезопасны:
- **HTTP**: `POST /sms/send`, `/batch`, `/cascade/deliveries`, `/lookup`, `/lookup/bulk`, `/templates`, `/webhooks` — retry = дубль записи + дубль charge.
- **gRPC**: `messagingv1.SendMessage` / `SendBatch` — нет `idempotency_key` поля в proto, нет interceptor проверки.
- **SMPP**: `user_message_reference` (TLV 0x0204) никогда не читается. `uuid.New()` безусловно.

## Proposed fix

Два альтернативных подхода, выбор через брейншторм:

**Подход A — gateway-level idempotency cache.**
- Middleware читает `Idempotency-Key` header (HTTP) / metadata (gRPC) / `user_message_reference` TLV (SMPP).
- Redis-backed cache: ключ = `client_id + idempotency_key`, TTL = 24h.
- Первый запрос: выполняется, ответ + request-hash кешируется.
- Retry с тем же ключом: если request-hash matches → возвращается cached response. Если отличается → `409 Conflict`.

**Подход Б — client-supplied message_id.**
- Делаем `message_id` в proto/request — **обязательным client-supplied** полем.
- Downstream guard (`commit_idempotency_guard`) начинает работать для client retries «из коробки».
- Требует breaking proto change + миграция клиентов.

**Рекомендация:** А (менее invasive, Б ломает клиентов). Возможен поэтапный путь: сначала А, потом Б как опция для клиентов с гарантированно стабильным UUID-генератором.

## Tasks

- [ ] Задача 1: Дизайн idempotency-middleware (cache schema, TTL, collision behavior, response replay).
- [ ] Задача 2: Миграция `idempotency_keys` table (postgres backup если Redis дропнет).
- [ ] Задача 3: Реализация HTTP middleware для 7 POST эндпоинтов.
- [ ] Задача 4: Реализация gRPC interceptor для SendMessage/SendBatch (после closure A7.3 fix-план — зависимость).
- [ ] Задача 5: SMPP submit_sm dedup — чтение user_message_reference TLV (зависит от A6-tlv fix — KafkaMessage рефактор).
- [ ] Задача 6: Unit-тест: retry с тем же ключом → тот же response, без повторной работы.
- [ ] Задача 7: Unit-тест: retry с другим body → 409 Conflict.
- [ ] Задача 8: Integration-тест: двойной send через HTTP не создаёт двух записей в messages, не списывает дважды.
- [ ] Задача 9: Integration-тест: тот же сценарий через gRPC.
- [ ] Задача 10: Integration-тест: SMPP submit_sm с тем же user_message_reference дважды — одно сообщение.

## Risks

- **Retro-compatibility:** клиенты, не посылающие Idempotency-Key, продолжают работать (middleware skip'ает проверку — auto-generates UUID как сейчас). Но это означает что retry-безопасность зависит от клиента.
- **Redis TTL:** если Redis дропается до retry — оба проходят. Mitigation: Postgres backup (задача 2).
- **Request-hash sensitivity:** если хешируем весь body, клиент не может retrы с тем же ключом но разным payload (intended). Если хешируем только ключ + ignore body — клиент может случайно reuse ключ для разных сообщений. Semantic decision.

## Dependencies

- A7.3 (gRPC auth interceptor) — для gRPC идемпотентности нужно сначала починить auth, чтобы `client_id` был настоящий.
- A6-tlv / KafkaMessage рефактор — SMPP path.
- B1 (integration CI infra) — integration-тесты требуют реальных Redis+Postgres.

## Estimated size

Крупное. 5-8 файлов, миграция, тесты.

## Priority rationale

Critical. Retry-driven double-charge — прямой финансовый риск. В проде вероятность вырастает с нестабильными сетями клиентов. Рекомендуется до широкого анонса публичного API.
