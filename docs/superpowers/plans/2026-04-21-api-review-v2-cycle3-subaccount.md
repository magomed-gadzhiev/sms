# Cycle 3 (Subaccount e2e / audit-only) Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development.

**Goal:** Аудит C-billing, C-visibility, C-dlr-routing через HTTP/gRPC/SMPP × 3 роли. Audit-only режим (no integration tests — B1 open).

**Spec:** `docs/superpowers/specs/2026-04-21-cycle3-subaccount-design.md` (parent: umbrella).

**Branch:** `review/api-v2-cycle3-subaccount` (уже создана).

## Tasks

1. **Scaffold** — spec + plan + matrix + findings caркас. Контроллер.
2. **C-billing: HTTP + gRPC + SMPP analysis.** Subagent. Фокус: `client_id` в `tarification_log` = отправитель. Для subaccount_parent → reference B3. gRPC → reference A7.3. SMPP → проверить доходит ли real client_id до billing несмотря на B2.
3. **C-visibility: HTTP + gRPC + SMPP.** Subagent. Фокус: SQL фильтрация по `client_id` в downstream service handlers (GetMessageHistory, ListTemplates, ListWebhooks, ListCascadeDeliveries, etc.). Субаккаунт-ребёнок не видит родителя?
4. **C-dlr-routing: HTTP + gRPC + SMPP.** Subagent. Фокус: webhook создание — привязка к authenticated client_id. DLR доставка — `webhook_url` из записи отправителя, не parent'а. Для SMPP inbound → reference Cycle 2.
5. **Finalize + PR.** Контроллер.

Каждый task — один коммит. Доп. fix-планы если находим новое.
