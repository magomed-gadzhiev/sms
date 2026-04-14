# Messaging Roadmap: Product Analysis & Prioritized Feature Plan

**Date:** 2026-04-12
**Area:** Messaging
**Mode:** Roadmap
**Segment:** All (SMB / Enterprise / Aggregator)
**Horizon:** 6 months (Q2-Q3 2026)

---

## Opportunity Statement

Платформа покрывает базовый SMS-пайплайн и уже имеет каскад (multi-channel), но не использует потенциал conversational messaging, self-service аналитики и programmatic-интеграций, что ограничивает ARPU и делает продукт уязвимым перед конкурентами, предлагающими omnichannel из коробки.

---

## Current State

Зрелая B2B SMS-система: 100+ REST-эндпоинтов, 13+ микросервисов, каскадная доставка (SMS -> Flash Call -> Max Messenger), A/B-тестирование кампаний, шаблоны с approval-workflow, контакт-менеджмент с сегментацией, биллинг на основе кредитов и real-time аналитика. Архитектура event-driven (Kafka), PostgreSQL с партицированием, Redis для кэширования.

**Сильные стороны:** полный campaign lifecycle, cascade delivery, granular routing, frequency capping, quiet hours, webhook-интеграции.

**Ограничения:** только исходящие сообщения (no inbound/2-way), нет RCS/WhatsApp/Viber каналов, нет Conversation API, нет публичного API-документирования (developer portal), нет SDK.

---

## Market Context

- **RCS Business Messaging** — Apple поддержка с iOS 18 делает RCS mainstream. Table stakes для enterprise CPaaS к 2026.
- **Conversational/2-Way Messaging** — Twilio, Sinch, Infobip предлагают Conversations API. Без inbound теряем use-cases верификации, поддержки, опросов.
- **WhatsApp Business API** — самый быстрорастущий канал (3B+ пользователей). Все крупные CPaaS уже предлагают.
- **Developer Experience** — Twilio стандарт: docs, SDK, sandbox, quickstart за 5 минут. Time-to-First-Message коррелирует с конверсией.
- **AI-персонализация** — автоматический подбор времени отправки, A/B на уровне контента. Differentiator для retention.
- **Compliance automation** — GDPR, TCPA, consent management. Enterprise не купит без compliance-фич.

---

## Voice of Customer

### SMB
- Критичные задачи: массовые рассылки, OTP/верификация, уведомления
- Фрустрация: нет SDK/sandbox, непонятная тарификация, нет WhatsApp
- Готовы платить за: простоту интеграции, WhatsApp, автоматизацию
- Причина ухода: конкурент с WhatsApp + простой API + понятный pricing

### Enterprise
- Критичные задачи: omnichannel, compliance, SLA, кастомная аналитика
- Фрустрация: нет RCS, нет 2-way, ограниченные отчёты
- Готовы платить за: SLA, dedicated support, RCS, consent management
- Причина ухода: отсутствие omnichannel (Infobip/Sinch дают все каналы из одного API)

### Aggregator
- Критичные задачи: white-label, API-first, высокий throughput
- Фрустрация: нет white-label портала, нет sub-account API
- Готовы платить за: volume discounts, white-label, API для sub-accounts
- Причина ухода: лучшие цены + более мощный API

---

## Competitive Analysis

| Feature | Our Platform | Twilio | Infobip | Sinch | BSG |
|---|---|---|---|---|---|
| SMS Send/Batch | Yes | Yes | Yes | Yes | Yes |
| Cascade delivery | Yes | No | Yes | Yes | No |
| A/B testing campaigns | Yes | No | Yes | No | No |
| WhatsApp Business | No | Yes | Yes | Yes | Yes |
| Viber Business | No | No | Yes | Yes | Yes |
| RCS | No | Yes | Yes | Yes | No |
| 2-Way / Inbound SMS | No | Yes | Yes | Yes | Yes |
| Conversations API | No | Yes | Yes | Yes | No |
| Developer Portal / SDK | No | Yes | Yes | Yes | Partial |
| Consent Management | No | Yes | Yes | Yes | No |
| White-label | No | No | Yes | Partial | No |

**Наша ниша:** каскад + campaign management + A/B testing + pricing flexibility.

---

## Feature Proposals (sorted by RICE)

### #1: Scheduled Messages API
**Проблема:** API позволяет только немедленную отправку. Клиенты реализуют scheduling самостоятельно.
**Сегмент:** all | **Ценность:** volume up, NPS up | **Сложность:** S
**RICE:** Reach=60% x Impact=1 x Confidence=90% / Effort=1.5w = **36.0**
**Конкуренты:** Twilio, Infobip, BSG
**Монетизация:** Базовый тариф (competitive parity)
**Риски:** Минимальные. Scheduler worker + поле `scheduled_at`.
**Зависимости:** Нет блокеров. Campaign scheduler переиспользуем.

### #2: Developer Portal + SDK
**Проблема:** Time-to-First-Message высокий. Нет документации, SDK, sandbox.
**Сегмент:** smb, aggregator | **Ценность:** volume up, churn down | **Сложность:** M
**RICE:** Reach=80% x Impact=2 x Confidence=80% / Effort=4w = **32.0**
**Конкуренты:** Twilio (эталон), Infobip, Sinch
**Монетизация:** Бесплатно (acquisition tool)
**Риски:** Поддержка актуальности документации. Версионирование SDK.
**Зависимости:** Стабильный REST API есть. Нужен OpenAPI spec.

### #3: WhatsApp Business Channel
**Проблема:** Клиенты уходят без WhatsApp. Table stakes для CPaaS 2026.
**Сегмент:** all | **Ценность:** revenue up, churn down, volume up | **Сложность:** L
**RICE:** Reach=90% x Impact=3 x Confidence=90% / Effort=8w = **30.4**
**Конкуренты:** Twilio, Infobip, Sinch, BSG
**Монетизация:** Usage-based (per-conversation markup 15-25%)
**Риски:** Meta BSP сертификация. Сложная conversation-based pricing модель.
**Зависимости:** Cascade архитектура готова. Нужен provider adapter.

### #4: Viber Business Messages
**Проблема:** Viber - второй мессенджер в СНГ/Восточной Европе. Каскад без него неполноценный.
**Сегмент:** all | **Ценность:** revenue up, volume up | **Сложность:** M
**RICE:** Reach=60% x Impact=2 x Confidence=85% / Effort=4w = **25.5**
**Конкуренты:** Infobip, Sinch, BSG
**Монетизация:** Usage-based (per-message markup 20-30%)
**Риски:** Viber Business верификация. API менее стабильный.
**Зависимости:** Cascade архитектура готова.

### #5: Consent Management / Opt-out Automation
**Проблема:** Compliance блокер для enterprise. Нет автообработки STOP, нет consent registry.
**Сегмент:** enterprise | **Ценность:** churn down, NPS up | **Сложность:** M
**RICE:** Reach=40% x Impact=2 x Confidence=80% / Effort=3w = **21.3**
**Конкуренты:** Twilio, Infobip, Sinch
**Монетизация:** Enterprise plan feature
**Риски:** Полная автоматизация зависит от Inbound SMS.
**Зависимости:** Inbound SMS для полной автоматизации. Opt-out list частично есть.

### #6: Webhook v2 + Event Streaming
**Проблема:** Текущие webhooks basic (DLR only). Нет retry, нет event catalog.
**Сегмент:** aggregator, enterprise | **Ценность:** churn down, NPS up | **Сложность:** M
**RICE:** Reach=50% x Impact=1.5 x Confidence=80% / Effort=3w = **20.0**
**Конкуренты:** Twilio (Event Streams), Infobib
**Монетизация:** Basic бесплатно. Event Streaming - enterprise plan.
**Риски:** Инфраструктура reliable delivery (retry queue, dead letter).
**Зависимости:** Webhook-service и Kafka уже есть.

### #7: Inbound SMS / 2-Way Messaging
**Проблема:** Без входящих невозможны: OTP-reply, опросы, STOP-автоматизация.
**Сегмент:** all | **Ценность:** revenue up, churn down, NPS up | **Сложность:** L
**RICE:** Reach=70% x Impact=2 x Confidence=85% / Effort=6w = **19.8**
**Конкуренты:** Twilio, Infobip, Sinch, BSG
**Монетизация:** Базовый тариф. Premium: auto-reply rules, keyword routing.
**Риски:** Требует выделенных номеров/short codes. MO-трафик через операторов.
**Зависимости:** SMPP Gateway расширение. Новые Kafka topics.

### #8: Smart Send Time Optimization (AI)
**Проблема:** Клиенты не знают оптимальное время. Endpoint optimal-time существует, но нет ML-модели.
**Сегмент:** all | **Ценность:** volume up (delivery rate up) | **Сложность:** M
**RICE:** Reach=50% x Impact=1 x Confidence=60% / Effort=3w = **10.0**
**Конкуренты:** Infobip, Sinch
**Монетизация:** Freemium. Per-contact optimization - premium.
**Риски:** Требует достаточного объёма исторических данных.
**Зависимости:** Данные в tarification_log и message_stats уже есть.

### #9: White-Label Portal
**Проблема:** Aggregator'ы хотят портал под своим брендом.
**Сегмент:** aggregator | **Ценность:** revenue up (new segment) | **Сложность:** L
**RICE:** Reach=15% x Impact=3 x Confidence=70% / Effort=6w = **5.3**
**Конкуренты:** Infobip
**Монетизация:** Premium add-on (monthly fee + per-sub-account)
**Риски:** Значительная доработка фронтенда.
**Зависимости:** Текущая sub-account система.

### #10: RCS Business Messaging
**Проблема:** RCS - будущее messaging. Apple поддержка с iOS 18.
**Сегмент:** enterprise | **Ценность:** revenue up (premium pricing) | **Сложность:** XL
**RICE:** Reach=30% x Impact=2 x Confidence=50% / Effort=10w = **3.0**
**Конкуренты:** Twilio, Infobip, Sinch
**Монетизация:** Premium add-on
**Риски:** Google RBM партнёрство. Фрагментированная экосистема.
**Зависимости:** Cascade архитектура. Rich Card модель в templates.

---

## Roadmap Timeline

### Quick Wins (1-2 weeks)
- Scheduled Messages API (RICE: 36.0) - `scheduled_at` + scheduler worker
- OpenAPI Spec Generation - автогенерация из хендлеров, публикация `/docs`

### Q2 2026 (April - June)
- Developer Portal MVP (RICE: 32.0) - OpenAPI docs, quickstart, Postman, sandbox
- WhatsApp Business Channel (RICE: 30.4) - provider adapter, Meta BSP, cascade step
- Viber Business Messages (RICE: 25.5) - provider adapter, cascade step (параллельно с WhatsApp)

### Q3 2026 (July - September)
- Inbound SMS / 2-Way (RICE: 19.8) - MO handling, Kafka topic, webhooks, auto-reply
- Consent Management (RICE: 21.3) - consent registry, STOP automation, double opt-in
- Webhook v2 (RICE: 20.0) - event catalog, retry, dead letter, streaming API
- SDK v1 - Go, Python, Node.js, PHP (генерация из OpenAPI)

### Strategic (6-12 months)
- Smart Send Time AI (RICE: 10.0) - ML-модель на DLR данных
- White-Label Portal (RICE: 5.3) - theming, custom domains, branding
- RCS Business Messaging (RICE: 3.0) - Google RBM, rich cards

---

## Decisions Needed

1. **WhatsApp BSP vs Cloud API** - BSP (больше контроля) vs Cloud API (быстрый старт). Блокирует разработку WhatsApp channel.
2. **Inbound SMS: номера vs short codes** - нужны ли договоры с операторами на MO-трафик? Определяет scope фичи.
3. **Developer Portal: self-hosted vs managed** - Docusaurus/Mintlify vs Readme.com/Stoplight. Влияет на effort.
4. **SDK: manual vs auto-generated** - генерация из OpenAPI vs ручное написание. Определяет DX quality vs effort.
5. **Viber: прямая интеграция vs через агрегатора** - собственный Viber Partner vs через BSG/Infobip. Влияет на маржу и скорость.

---

## Next Steps

1. Принять решение по WhatsApp BSP vs Cloud API - product owner - до 2026-04-20
2. Реализовать Scheduled Messages API (quick win) - backend team - 2 недели
3. Начать генерацию OpenAPI spec из хендлеров - backend team - 1 неделя
4. Провести discovery по Viber Partner / агрегатору - bizdev - до 2026-04-30
5. Запланировать техническое проектирование WhatsApp adapter - архитектор - после решения п.1
