# Fix Plan: OpenAPI drift (HTTP client-gateway)

**Severity:** major
**Source:** Cycle 1 finding A1 (`docs/reports/2026-04-21-cycle1-findings.md`)
**Spec reference:** `docs/superpowers/specs/2026-04-21-api-review-v2-umbrella.md` §2 axis A1

## Problem

OpenAPI spec (`api/openapi/openapi.yaml`) покрывает только 11 из 33 роутов клиентского HTTP API:
- **22 роута не задокументированы** (все webhooks, lookup, templates, cascade + 4 docs-роутов).
- **1 роут в OpenAPI но не в коде** (`GET /metrics`).
- **4 response-schema mismatches** (2× status code 202/200, 1× account/balance лишние поля, 1× account/stats **полностью другая схема**).

## Proposed fix

### Phase A: Добавить 22 недостающих эндпоинта
- webhooks (5 CRUD endpoints).
- lookup (3 endpoints — single, bulk, history).
- templates (6 — CRUD + audit).
- cascade (4 — deliveries + stats).
- /docs/* (4 — либо документируем либо помечаем out-of-contract).

### Phase B: Исправить 4 mismatches
- `POST /sms/send` + `POST /sms/batch`: либо openapi → 200, либо handler → 202. Решение: handler → 202 (spec semantically корректнее для async-like).
- `GET /account/balance`: добавить `client_id` и `updated_at` в схему.
- `GET /account/stats`: **переписать полностью** — схема openapi устарела, handler возвращает time-series analytics object.
- `GET /metrics`: удалить из openapi или зарегистрировать в client-router (сейчас monitoring server).

### Phase C: Guard — тест openapi ↔ router
Unit-тест в `internal/gateway/client/router/`: при старте читает openapi.yaml, парсит paths, проверяет что каждый задокументированный path зарегистрирован в router и vice-versa. Прерывает future drift.

## Tasks

- [ ] Задача 1: Инвентарь response schemas всех 22 missing endpoints (читаем handler'ы, достаём shape).
- [ ] Задача 2: Написать openapi yaml блоки для 22 endpoints.
- [ ] Задача 3: Исправить sms/send + sms/batch (handler или openapi, выбрать).
- [ ] Задача 4: Исправить account/balance openapi schema.
- [ ] Задача 5: Переписать account/stats openapi schema.
- [ ] Задача 6: Решение по /docs/* routes (in-contract / out-of-contract comment).
- [ ] Задача 7: Решение по /metrics (удалить из spec или зарегистрировать в client-router).
- [ ] Задача 8: Guard-test openapi↔router.

## Risks

- Клиенты могли построиться против неполной спеки и игнорируют недокументированные эндпоинты — добавление в спеку безопасно.
- `GET /account/stats` schema — клиенты, построенные против старой openapi, получают completely broken responses **уже сейчас**. Fix спеки не ломает их сильнее, но формально меняет контракт.
- `sms/send` → 202 handler fix затронет клиентов, которые checkят на ровно 200 — низкая, но ненулевая вероятность. Обновление openapi → 200 не меняет поведения, zero risk.

## Dependencies

- Нет code-wise. Опциональная зависимость: Cycle 2 `cascade.go` refactor (который приведёт его к общему envelope) — если done, cascade endpoints будут schematically проще.

## Estimated size

Крупное. 22 новых openapi блока + 4 фикса + 1 unit-test. Порядка сотен строк yaml + ~100 строк теста.

## Priority rationale

Major. Прямого финансового риска нет, но integrators, читающие спеку, не могут использовать webhooks/lookup/templates/cascade через контракт — вынуждены reverse-engineer. Блокирует партнёрский онбординг.
