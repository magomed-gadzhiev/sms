# Messaging Roadmap — Phases Overview

**Roadmap spec:** [2026-04-12-messaging-roadmap-half-year.md](../specs/2026-04-12-messaging-roadmap-half-year.md)

---

## Phases Summary

| Фаза | Фичи | RICE | Статус | План |
|---|---|---|---|---|
| **Phase 1** | Scheduled Messages API (complete) | 36.0 | Ready | [plan](2026-04-12-phase1-scheduled-messages-complete.md) |
| **Phase 2** | Developer Portal + SDK | 32.0 | Ready | [plan](2026-04-12-phase2-developer-portal-sdk.md) |
| **Phase 3a** | WhatsApp Business Channel | 30.4 | Blocked: BSP vs Cloud API decision | [stub](2026-04-12-phase3-whatsapp-channel-stub.md) |
| **Phase 3b** | Viber Business Messages | 25.5 | Blocked: Partnership decision | [stub](2026-04-12-phase3-viber-channel-stub.md) |
| **Phase 4** | Inbound SMS / 2-Way Messaging | 19.8 | Ready | [plan](2026-04-12-phase4-inbound-sms.md) |
| **Phase 5** | Webhook v2 + Event Streaming | 20.0 | Ready | [plan](2026-04-12-phase5-webhook-v2.md) |

---

## Dependency Graph

```
Phase 1 (Scheduled Messages)  ─── independent ───────────────────► Ready NOW
Phase 2 (Developer Portal)    ─── independent ───────────────────► Ready NOW
Phase 4 (Inbound SMS)         ─── independent ───────────────────► Ready NOW
Phase 5 (Webhook v2)          ─── independent ───────────────────► Ready NOW

Phase 3a (WhatsApp)           ─── needs: Meta BSP/Cloud API decision ─► Blocked
Phase 3b (Viber)              ─── needs: Viber Partner/aggregator decision ─► Blocked

Future: Consent Management    ─── depends on: Phase 4 (Inbound SMS)
Future: Smart Send AI         ─── depends on: enough historical data
Future: White-Label Portal    ─── depends on: theming engine
```

---

## Quick Start — что делать прямо сейчас

### Немедленно (1 AI сессия, ~2-3 часа)

Запустить Phase 1 прямо сейчас:

```
@docs/superpowers/plans/2026-04-12-phase1-scheduled-messages-complete.md
```

Что будет сделано:
- `GET /api/v1/sms/scheduled` эндпоинт (list + pagination)
- Proto RPC `ListScheduledMessages`
- Frontend: бейджик "Запланировано" + колонка дата
- Обновлён OpenAPI spec

### Параллельно (можно запустить одновременно с Phase 1)

```
@docs/superpowers/plans/2026-04-12-phase2-developer-portal-sdk.md
```

Что будет сделано:
- Аудит и синхронизация OpenAPI spec
- Quickstart HTML-страница `/docs/quickstart`
- SDK генерация (Go, Python, Node.js) через openapi-generator

---

## Decisions Required Before Phase 3

**До 2026-04-20:**
- [ ] WhatsApp: BSP или Cloud API? (Product Owner)
- [ ] Создать Meta Business Manager + WABA

**До 2026-04-30:**
- [ ] Viber: прямая интеграция или через BSG/другой агрегатор? (Bizdev)

После принятия решений: запустить детализацию стабов через `superpowers:writing-plans`.

---

## Execution Instructions для AI агентов

Каждый план содержит заголовок:
```
REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
```

### Рекомендуемый порядок запуска

**Неделя 1:**
1. Phase 1 (Scheduled Messages) — ~6 часов
2. Phase 2 (Developer Portal) — ~8 часов

**Неделя 2-3:**
3. Phase 4 (Inbound SMS) — ~20 часов  
4. Phase 5 (Webhook v2) — ~16 часов

**После бизнес-решений (Q2):**
5. Phase 3a (WhatsApp) — ~40 часов
6. Phase 3b (Viber) — ~20 часов

---

## Definition of Done

Каждая фаза считается завершённой когда:
- [ ] Все тесты в плане проходят (`go test ./...`)
- [ ] Проект собирается (`go build ./...` + `npm run build`)
- [ ] OpenAPI spec обновлён для новых эндпоинтов
- [ ] Изменения закоммичены с осмысленными сообщениями
