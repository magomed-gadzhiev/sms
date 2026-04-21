# Fix Plan: DLR webhook delivery completely dead (F-D1 + F-D2)

**Severity:** critical
**Source:** Cycle 3 Task 4 findings F-D1, F-D2 (`docs/reports/2026-04-21-cycle3-findings.md`)
**Spec reference:** `docs/superpowers/specs/2026-04-21-cycle3-subaccount-design.md`

## Problem

**F-D1 (critical):** `pipeline/sender/stage.go:81-104` создаёт `queue.DLRMessage` с полем `MessageID = uuid.Nil` (никогда не заполняется). `delivery_service.go:HandleDLR:168` вызывает `msgRepo.GetEnrichment(ctx, uuid.Nil)` — всегда возвращает nil. Хендлер логирует "message not found for DLR, skipping" и возвращается без вызова webhook'а. **Ни один DLR callback никогда не доставляется ни одному клиенту — через любой транспорт.**

**F-D2 (major):** `status/stage.go:223` использует тот же `uuid.Nil` для DB upsert. DLR статусы (DELIVRD, UNDELIV) пишутся против UUID `00000000-...` — матчат 0 строк. Таблица `messages` никогда не получает DLR-обновления через этот путь.

**Cross-tenant:** корректное `WHERE client_id = $1` в `ListActiveByClientID` — но unreachable из-за F-D1. Тенант-isolation дизайнится правильно, но код мёртв.

## Root cause

В `pool_async.go` или соседнем коде DLR-пайплайна извлекается `data.SMPPMessageID` (провайдерский message id), но отсутствует шаг "резолви internal UUID из SMPP message id". Метод `storage.MessageRepository.GetBySMPPMessageID(ctx, smppMsgID)` **существует** (`storage/message_repository.go:114`) — но нигде не вызывается перед DLR dispatch.

## Proposed fix

В DLR callback'е провайдерского пула (или в sender stage, откуда строится `DLRMessage`):
1. После извлечения `data.SMPPMessageID` вызвать `storage.MessageRepository.GetBySMPPMessageID(ctx, data.SMPPMessageID)`.
2. Проверить not-nil (провайдер мог прислать unknown id — log warning и skip).
3. Установить `dlrMsg.MessageID = msg.ID` (реальный internal UUID) и `dlrMsg.ClientID = msg.ClientID` (для downstream webhook resolution).

После этого `HandleDLR` найдёт enrichment, `dispatchToSubscriptions` пойдёт искать webhook'и по правильному `client_id`, F-D2 автоматически начнёт работать (upsert найдёт правильный row в `messages`).

## Tasks

- [ ] Задача 1: Найти точку вызова DLR callback'а в `pipeline/sender/stage.go:81-104` — контекст, зависимости от storage.
- [ ] Задача 2: Доступ к `MessageRepository` в sender stage — already there или нужна DI.
- [ ] Задача 3: Реализовать lookup + nil-guard + populate MessageID/ClientID в DLRMessage.
- [ ] Задача 4: То же в `status/stage.go:223` (если это отдельный код путь).
- [ ] Задача 5: Unit-тест: DLR с неизвестным `SMPPMessageID` → skip + warn log, никаких panic.
- [ ] Задача 6: Unit-тест: DLR с валидным `SMPPMessageID` → `HandleDLR` вызывается, webhook dispatched.
- [ ] Задача 7: Integration-тест (требует B1 CI): полный round-trip submit_sm → провайдер ACK → DLR → webhook callback получен с правильным client_id.

## Risks

- Lookup на каждый DLR — добавляет DB read. Проверить что провайдер не шлёт `duplicate_reply`-style потоки, которые перегрузят `GetBySMPPMessageID`.
- Если `SMPPMessageID` не уникален per-provider (у разных провайдеров могут быть одинаковые) — нужен дополнительный index (provider_id, smpp_message_id). Проверить в миграциях.
- Historical DLRs в Kafka, застрявшие из-за бага — при fix вдруг начнут обрабатываться. Возможен всплеск webhook callbacks. Рекомендуется canary rollout + rate limit.

## Dependencies

- B1 (integration CI) — для integration-теста Задачи 7. Unit-тесты (5-6) выполнимы сразу.
- KafkaMessage refactor (Cycle 2 мета-находка) — не блокирует, но если делается параллельно — учесть DLRMessage.

## Estimated size

Крупное. Ядро фикса — ~20-40 строк (lookup + populate), но тестовое покрытие и проверка истории DLR — значительно больше.

## Priority rationale

**Самый срочный фикс из всех в ревью v2.** Это не security gap, не edge-case — это core feature (DLR доставка) полностью нерабочий. Клиенты, подключившие webhook для DLR, **никогда не получали callback'ы**. Скорее всего никто не заметил потому что:
- Либо клиенты используют polling вместо webhook'ов для DLR.
- Либо SMPP receiver-bind путь (`smppv1.DeliverDLR` → outbound deliver_sm) работает корректно и покрывает SMPP-клиентов.
- Либо DLR webhook как фича не оживлялся.

Перед началом любой работы по Cycle 2 фиксам имеет смысл выяснить: **используется ли DLR webhook хоть кем-то** (query DB: есть ли активные `webhook_subscriptions` c event_type="delivery_receipt").
