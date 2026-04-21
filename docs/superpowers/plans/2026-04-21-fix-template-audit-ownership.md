# Fix Plan: Template audit log ownership bypass (F-C1)

**Severity:** major
**Source:** Cycle 3 Task 3 finding F-C1 (`docs/reports/2026-04-21-cycle3-findings.md`)

## Problem

`GET /api/v1/templates/{id}/audit` — `handlers/templates.go:184` извлекает `clientID` из ctx, но сразу discard'ит через `_`. gRPC request `GetTemplateAuditLog` не содержит client_id. `template/grpc/server.go:246` принимает без ownership check. SQL `WHERE template_id = $1` — только template_id, без tenant-фильтра.

**Exploit:** любой authenticated клиент, знающий `template_id` (UUID другого клиента), может прочитать полную audit history — старое/новое тело шаблона, actor_id, reason, timestamps.

## Proposed fix

1. Не discard'ить clientID в handler. Передавать как `req.ClientId`.
2. Добавить `client_id` поле в `GetTemplateAuditLogRequest` proto.
3. В `template/grpc/server.go:GetAuditLog` — сначала fetch template by id, verify `template.client_id == req.client_id`. Если не совпадает — `codes.NotFound` (не 403, чтобы не leak existence).
4. Либо inline check в SQL: `SELECT ... WHERE template_id = $1 AND client_id = $2` после JOIN с templates table.

## Tasks

- [ ] Задача 1: Добавить поле `client_id` в `GetTemplateAuditLogRequest` в .proto (если source есть — см. Cycle 1 A1-grpc про отсутствие .proto source; если нет — обновить .pb.go напрямую или восстановить .proto из .pb.go).
- [ ] Задача 2: Убрать discard в handler, пробросить client_id.
- [ ] Задача 3: Добавить ownership check в GetAuditLog (fetch template + compare).
- [ ] Задача 4: Unit-тест: клиент A пытается читать audit template'а клиента B → NotFound.
- [ ] Задача 5: Unit-тест: клиент A читает свой audit → OK.
- [ ] Задача 6: Security regression test в audit suite.

## Risks

- Breaking change: клиент, случайно полагающийся на возможность читать чужие audit'ы (маловероятно, но теоретически), сломается. Риск низкий — вряд ли это было документировано.
- Proto change (если добавляем поле) — deployment ordering: обновлённый клиент гейтвея должен работать со старым сервером (новое поле = optional default empty). Обновлённый сервер должен rejectать запросы без client_id. Рекомендуется feature flag или двухфазный roll.

## Dependencies

- A1-grpc missing .proto source — решается отдельным планом, но для этого fix'а можно временно обойтись правкой .pb.go или восстановить .proto из legacy `api/proto/template/template.proto` если есть.

## Estimated size

Мелкое-среднее. ~30-50 строк кода + 2-3 теста + proto adjustment.

## Priority rationale

Major. Security gap ownership bypass, но impact ограничен: audit history содержит body шаблона + actor + reason — чувствительно, но не catastrophic (не платёжные данные, не PII напрямую). Рекомендуется включить в ближайший security-hardening sprint.
