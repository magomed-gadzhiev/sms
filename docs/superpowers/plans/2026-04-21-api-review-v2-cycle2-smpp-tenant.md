# Cycle 2 (SMPP + tenant-glue) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development.

**Goal:** Ревью осей A6 (SMPP контракт) и A7 (gRPC tenant propagation). Закрытие B2 (AuthAdapter kludge) — работа внутри этого цикла. Критическая находка A6-auth (SMPP password игнорируется) — характеризуется глубоко, выносится в отдельный fix-план.

**Architecture:** Ветка `review/api-v2-cycle2-smpp-tenant` от master. Один PR к master. Матрица Cycle 2 + findings + критические fix-планы.

**Tech Stack:** Go read-only анализ, markdown матрица, отдельные fix-планы для крупного.

**Spec:** `docs/superpowers/specs/2026-04-21-api-review-v2-umbrella.md` (умбрелла) + `docs/reports/2026-04-21-api-review-v2-surface.md` (inventory, в PR #25).

---

## Known pre-state

- Ветка `review/api-v2-cycle2-smpp-tenant` уже создана контроллером от master.
- Inventory из Phase 0 НЕ в master (он в PR #25). Субагенты не читают surface.md — контроллер передаёт нужные факты в промпте.
- Много untracked файлов в рабочем дереве — FORBIDDEN.

---

## Task 1: Scaffold матрицы Cycle 2 + findings

**Files allowed:**
- Create: `docs/reports/2026-04-21-cycle2-smpp-glue-matrix.md`
- Create: `docs/reports/2026-04-21-cycle2-smpp-glue-matrix.csv`
- Create: `docs/reports/2026-04-21-cycle2-findings.md`

Контроллер делает сам, без субагента (механика).

---

## Task 2: A6-auth deep dive (CRITICAL)

**Finding из Phase 0:** SMPP password игнорируется в `AuthenticateBySystemID`. Нужна глубокая характеристика:

1. Читает ли protocol-layer password из bind PDU, передаёт ли в auth_adapter?
2. AuthenticateBySystemID — что делает с полем password, игнорирует ли фактически или проверяет через authv1.Authenticate?
3. authv1.Authenticate — принимает ли password, проверяет ли?
4. Исторический контекст: всегда ли так было, или регресс?
5. Какой должна быть правильная auth: `Authenticate(system_id, password)` или отдельный flow?

**Files allowed:**
- Modify: `docs/reports/2026-04-21-cycle2-findings.md` — секция A6-auth.
- Create: `docs/superpowers/plans/2026-04-21-fix-smpp-auth-password.md` — отдельный fix-план.

**Files read-only:** `internal/gateway/smpp/server/auth_adapter.go`, `internal/gateway/smpp/server/handler.go` (bind-ветка), `internal/smpp/protocol/pdu.go` (структура bind PDU), `api/proto/authv1/` (proto контракт).

**Subagent task.**

---

## Task 3: A6-tlv + A6-submit-ext анализ

**Finding из Phase 0:** 0 TLV констант, submit_multi_sm и data_sm unhandled.

1. Проверить, что `receipted_message_id` (0x001E), `message_state` (0x0427), `sar_msg_ref_num` (0x020C), `sar_total_segments` (0x020E), `sar_segment_seqnum` (0x020F), `user_message_reference` (0x0204) — нигде не читаются из inbound submit_sm или не пишутся в outbound deliver_sm.
2. Оценить severity отсутствия SAR TLV (разбиение длинных сообщений не будет работать для клиентов, использующих TLV-подход вместо UDH).
3. Оценить risk submit_multi_sm / data_sm silent failure — как часто клиенты это используют в SMPP v3.4.

**Files allowed:** Modify `docs/reports/2026-04-21-cycle2-findings.md` + matrix.

**Files read-only:** `internal/smpp/protocol/*`, `internal/gateway/smpp/server/handler.go`.

**Subagent task.**

---

## Task 4: A6-dlr + A6-registered-delivery

**Finding из Phase 0:** deliver_sm не различает MO и DLR.

1. Найти где `registered_delivery` бит устанавливается на исходящих submit_sm (если вообще).
2. Найти путь DLR от провайдера обратно к клиенту. Через SMPP deliver_sm (внутренний) — не работает. Через smppv1.DeliverDLR gRPC — работает?
3. Проверить: если клиент ждёт DLR по SMPP-bind (receiver/transceiver), как ему DLR доставляется?
4. Зафиксировать gap: нет поддержки inbound DLR по SMPP линку.

**Files allowed:** Modify findings + matrix.

**Files read-only:** SMPP gateway/server/session, `internal/smsc/*` (для outbound DLR).

**Subagent task.**

---

## Task 5: A6-datacoding + A6-long-msg

1. Какие `data_coding` значения handled в submit_sm? (0=GSM7, 3=Latin1, 8=UCS2)
2. Что с другими (4=binary, 6=Cyrillic, etc.)?
3. Длинные сообщения: handled через UDH? через SAR TLV (мы уже знаем — нет)? silently truncated?

**Files allowed:** Modify findings + matrix.

**Files read-only:** submit_sm handler + protocol encoder/decoder.

**Subagent task.**

---

## Task 6: A7 — tenant propagation per RPC

**Finding из Phase 0:** 31 RPC TENANT-MISSING, все через request field (нет metadata).

Для топ-5 сервисов (messagingv1, billingv1, tarificationv1, cascadev1, webhookv1) — для каждого RPC handler:

1. Читается ли `client_id` из request?
2. Используется ли фактически для фильтрации/scope в логике?
3. Есть ли path где `client_id` из request игнорируется (handler использует что-то другое)?
4. Специально: cascadev1.ChannelAdminService и StrategyAdminService (12 RPC без client_id) — защищены ли gateway-уровнем?

Матрица: ~30 RPC tenant-критичных × 3 роли = ~90 строк A7.

**Files allowed:** Modify findings + matrix.

**Files read-only:** top-5 services `internal/services/*/grpc/`.

**Subagent task.**

---

## Task 7: Финализация Cycle 2 + PR

- Update OK-count в заголовке матрицы.
- Сводный `cycle2-findings.md`.
- Проверить что все critical находки имеют либо `fixed-in-PR` либо `plan:<file>`.
- Push + PR к master.

**Контроллер делает сам.**
