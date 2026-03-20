# Tasks: SMS Gateway Platform

**Input**: Design documents from `/specs/001-sms-gateway-platform/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Context**: Большая часть платформы уже реализована. Задачи сфокусированы на 4 выявленных доработках (gaps) из plan.md, организованных по затронутым user stories.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Можно выполнять параллельно (разные файлы, нет зависимостей)
- **[Story]**: К какой user story относится задача (US1, US2, и т.д.)
- Указаны точные пути к файлам

---

## Phase 1: Setup

**Purpose**: Подготовка миграций и общих компонентов для всех доработок

- [x] T001 Создать миграцию для добавления поля `segment_count` в таблицу `messages` в migrations/
- [x] T002 [P] Создать миграцию для добавления поля `segment_count` в таблицу `transactions` в migrations/
- [x] T003 [P] Добавить статус `expired` в enum статусов сообщений в internal/shared/models.go
- [x] T004 [P] Добавить конфигурационные переменные `DLR_EXPIRY_TIMEOUT`, `DATA_RETENTION_MESSAGES`, `DATA_RETENTION_AUDIT` в configs/

**Checkpoint**: Миграции и базовые модели готовы для реализации user stories.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Общая инфраструктура для multipart SMS, используемая несколькими user stories

**⚠️ CRITICAL**: US1, US3, US9 зависят от этой фазы

- [x] T005 Реализовать функцию подсчёта сегментов SMS (GSM 7-bit и UCS-2 кодировки, UDH overhead) в internal/shared/sms_segment.go
- [x] T006 Реализовать функцию разбиения текста на сегменты с UDH-заголовками (reference number, total parts, part number) в internal/shared/sms_segment.go
- [x] T007 Добавить поддержку UDH в SMPP PDU encoder — поле ESM class 0x40 и UDH bytes в submit_sm в internal/smpp/protocol/pdu.go
- [x] T008 [P] Обновить модель Message — добавить поле SegmentCount в internal/shared/models.go и internal/services/messaging/domain/message.go

**Checkpoint**: Базовая инфраструктура multipart SMS готова.

---

## Phase 3: User Story 1 — Send Single SMS (Priority: P1) 🎯 MVP

**Goal**: Клиент может отправить SMS любой длины, длинные сообщения автоматически разбиваются на сегменты.

**Independent Test**: Отправить SMS >160 символов через API, подтвердить что `segment_count` > 1 в ответе, и все сегменты доставлены.

### Implementation for User Story 1

- [x] T009 [US1] Интегрировать подсчёт сегментов в messaging-service при создании сообщения — вызов sms_segment.CountSegments() в internal/services/messaging/application/message_service.go
- [x] T010 [US1] Обновить gRPC handler SendMessage — заполнять segment_count в response в internal/services/messaging/grpc/server.go
- [x] T011 [US1] Обновить HTTP handler send SMS — возвращать segment_count в JSON response в internal/gateway/client/handlers/sms.go
- [x] T012 [US1] Обновить worker — при отправке multipart сообщения отправлять несколько submit_sm PDU с UDH в internal/smsc/sender.go
- [x] T013 [US1] Обновить proto-определение SendMessageResponse — добавить поле segment_count в api/proto/messaging/messaging.proto и перегенерировать код

**Checkpoint**: Отправка single SMS с multipart поддержкой работает полностью.

---

## Phase 4: User Story 2 — Track Message Delivery Status (Priority: P1)

**Goal**: Статус сообщений корректно отражает все состояния lifecycle, включая новый статус "expired".

**Independent Test**: Отправить SMS, дождаться истечения DLR timeout, подтвердить что статус изменился на "expired".

### Implementation for User Story 2

- [x] T014 [US2] Реализовать DLR expiry goroutine — фоновый процесс для перевода "sent" → "expired" по таймауту в internal/services/messaging/application/dlr_expiry.go
- [x] T015 [US2] Добавить SQL-запрос для выборки сообщений в статусе "sent" старше DLR_EXPIRY_TIMEOUT в internal/services/messaging/infrastructure/repository/message_repository.go
- [x] T016 [US2] Запустить DLR expiry goroutine в main() messaging-service с конфигурацией из env в cmd/services/messaging-service/main.go
- [x] T017 [US2] Обновить GetMessageStatus gRPC handler — поддержка статуса "expired" с полем expired_at в internal/services/messaging/grpc/server.go
- [x] T018 [P] [US2] Обновить HTTP handler status — возвращать expired_at в JSON response в internal/gateway/client/handlers.go
- [x] T019 [P] [US2] Обновить proto-определение GetStatusResponse — добавить поле expired_at и статус EXPIRED в api/proto/messaging/messaging.proto

**Checkpoint**: Полный lifecycle сообщений с DLR expiry работает корректно.

---

## Phase 5: User Story 3 — Send Batch SMS (Priority: P2)

**Goal**: Batch-отправка корректно считает и тарифицирует сегменты для каждого сообщения.

**Independent Test**: Отправить batch из 3 сообщений (короткое, длинное 2-сегмента, длинное 3-сегмента), проверить что каждое имеет правильный segment_count и списание баланса соответствует сумме сегментов.

### Implementation for User Story 3

- [x] T020 [US3] Обновить SendBatch в messaging-service — подсчёт сегментов для каждого сообщения в batch в internal/services/messaging/application/service.go
- [x] T021 [US3] Обновить SendBatch gRPC handler — возвращать segment_count для каждого сообщения в internal/services/messaging/grpc/server.go
- [x] T022 [US3] Обновить HTTP handler batch — возвращать segment_count в каждом результате в internal/gateway/client/handlers.go

**Checkpoint**: Batch SMS с посегментным подсчётом работает.

---

## Phase 6: User Story 4 — Schedule SMS (Priority: P2)

**Goal**: Валидация максимального горизонта планирования (30 дней).

**Independent Test**: Попробовать запланировать SMS на 31 день вперёд — получить ошибку валидации. Запланировать на 29 дней — успешно.

### Implementation for User Story 4

- [x] T023 [US4] Добавить валидацию scheduled_at <= 30 дней в будущее в gateway handler в internal/gateway/client/handlers.go
- [x] T024 [P] [US4] Добавить валидацию scheduled_at <= 30 дней в messaging-service gRPC handler в internal/services/messaging/grpc/server.go

**Checkpoint**: Планирование SMS с 30-дневным лимитом работает.

---

## Phase 7: User Story 5 — Manage Client Accounts (Priority: P2)

**Goal**: Уже реализовано. Нет доработок.

**Independent Test**: Создать клиента через admin API, установить rate limit, подтвердить throttling.

*Все задачи для этого user story уже выполнены в текущей кодовой базе.*

---

## Phase 8: User Story 6 — Webhooks (Priority: P3)

**Goal**: Webhook уведомления включают информацию о сегментах и поддерживают статус "expired".

**Independent Test**: Зарегистрировать webhook, отправить multipart SMS, подтвердить что callback содержит segment_count и корректный статус.

### Implementation for User Story 6

- [x] T025 [US6] Обновить webhook payload — добавить поле segment_count в уведомление в internal/services/webhook/application/service.go
- [x] T026 [P] [US6] Добавить событие "expired" в список триггеров webhook в internal/services/webhook/domain/events.go
- [x] T027 [P] [US6] Обновить proto-определение webhook event — добавить segment_count и EXPIRED event type в api/proto/webhook/webhook.proto

**Checkpoint**: Webhooks поддерживают multipart и expired статус.

---

## Phase 9: User Story 7 — Templates (Priority: P3)

**Goal**: Уже реализовано. Нет доработок.

**Independent Test**: Создать шаблон, одобрить через admin API, отправить SMS с шаблоном и переменными.

*Все задачи для этого user story уже выполнены в текущей кодовой базе.*

---

## Phase 10: User Story 8 — Analytics (Priority: P3)

**Goal**: Аналитика учитывает сегменты в статистике.

**Independent Test**: Отправить несколько multipart SMS, запросить статистику — увидеть корректный подсчёт сегментов.

### Implementation for User Story 8

- [x] T028 [US8] Обновить агрегацию статистики — считать сегменты помимо сообщений в internal/services/analytics/application/service.go
- [x] T029 [P] [US8] Обновить proto-определение analytics stats — добавить total_segments в api/proto/analytics/analytics.proto

**Checkpoint**: Аналитика отражает посегментную статистику.

---

## Phase 11: User Story 9 — SMPP Protocol (Priority: P3)

**Goal**: SMPP клиенты могут отправлять и получать multipart SMS с UDH.

**Independent Test**: Подключиться по SMPP, отправить длинное сообщение, подтвердить что оно разбивается на сегменты с UDH и DLR приходит для каждого сегмента.

### Implementation for User Story 9

- [x] T030 [US9] Обновить SMPP gateway — обработка входящих submit_sm с UDH, корреляция сегментов в internal/gateway/smpp/handler.go
- [x] T031 [US9] Обновить SMPP gateway — отправка deliver_sm с UDH для multipart DLR в internal/gateway/smpp/handler.go

**Checkpoint**: SMPP протокол полностью поддерживает multipart SMS.

---

## Phase 12: Per-Segment Billing (Cross-cutting)

**Goal**: Биллинг тарифицирует каждый сегмент отдельно вместо целого сообщения.

**Independent Test**: Отправить 3-сегментное SMS, проверить что баланс уменьшился на 3× цену за сегмент. Проверить транзакцию — segment_count = 3.

### Implementation

- [x] T032 Обновить billing-service — умножать стоимость на segment_count при списании в internal/services/billing/application/service.go
- [x] T033 Обновить proto-определение billing — добавить segment_count в ChargeRequest в api/proto/billing/billing.proto
- [x] T034 Обновить messaging-service — передавать segment_count в billing при создании сообщения в internal/services/messaging/application/service.go
- [x] T035 [P] Обновить admin API баланса — показывать segment_count в транзакциях в internal/gateway/admin/handlers.go

**Checkpoint**: Посегментная тарификация работает корректно.

---

## Phase 13: Data Purging (Cross-cutting)

**Goal**: Автоматическое удаление устаревших данных — 90 дней для сообщений, 1 год для аудита.

**Independent Test**: Создать тестовую партицию старше 90 дней, запустить purge, подтвердить удаление партиции.

### Implementation

- [x] T036 Реализовать функцию удаления старых партиций PostgreSQL (DROP PARTITION) в internal/shared/database/partition_purger.go
- [x] T037 Реализовать фоновый процесс data purging с конфигурируемым интервалом (ежедневно) в internal/services/messaging/application/data_purger.go
- [x] T038 Запустить data purger goroutine в main() messaging-service в cmd/services/messaging-service/main.go

**Checkpoint**: Автоматическая очистка данных работает по расписанию.

---

## Phase 14: Polish & Cross-Cutting Concerns

**Purpose**: Финальные улучшения, затрагивающие несколько user stories

- [x] T039 [P] Обновить docker-compose.yml — добавить env переменные DLR_EXPIRY_TIMEOUT, DATA_RETENTION_MESSAGES, DATA_RETENTION_AUDIT в deployments/docker-compose.yml
- [x] T040 [P] Добавить Prometheus метрики для multipart сообщений (segments_total, segments_per_message histogram) в internal/monitoring/metrics.go
- [x] T041 [P] Добавить Prometheus метрики для DLR expiry (expired_messages_total) в internal/monitoring/metrics.go
- [ ] T042 Обновить Grafana dashboard — панели для сегментов и expired сообщений в deployments/grafana/
- [ ] T043 Провести валидацию по quickstart.md — полный end-to-end тест платформы
- [ ] T044 Установить protoc и перегенерировать все proto-файлы: messaging, billing, webhook, analytics в api/proto/

**Checkpoint**: Платформа полностью готова к production.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: Нет зависимостей — можно начинать сразу
- **Phase 2 (Foundational)**: Зависит от Phase 1 — БЛОКИРУЕТ US1, US3, US9
- **Phase 3-4 (US1, US2)**: Зависят от Phase 2. Могут выполняться параллельно друг с другом
- **Phase 5 (US3)**: Зависит от Phase 3 (US1 multipart)
- **Phase 6 (US4)**: Зависит только от Phase 1. Может выполняться параллельно с Phase 2+
- **Phase 8, 11 (US6, US9)**: Зависят от Phase 2 (multipart infrastructure)
- **Phase 10 (US8)**: Зависит от Phase 2 (segment_count)
- **Phase 12 (Billing)**: Зависит от Phase 2 (segment_count в модели)
- **Phase 13 (Purging)**: Зависит только от Phase 1. Может выполняться параллельно
- **Phase 14 (Polish)**: Зависит от всех предыдущих фаз

### User Story Dependencies

- **US1 (Send SMS)**: Phase 2 → Phase 3. Нет зависимостей от других stories
- **US2 (Track Status)**: Phase 1 → Phase 4. Независим от других stories
- **US3 (Batch SMS)**: Phase 3 → Phase 5. Зависит от US1 (multipart)
- **US4 (Schedule SMS)**: Phase 1 → Phase 6. Полностью независим
- **US6 (Webhooks)**: Phase 2 → Phase 8. Независим, но лучше после US1/US2
- **US8 (Analytics)**: Phase 2 → Phase 10. Независим
- **US9 (SMPP)**: Phase 2 → Phase 11. Независим

### Parallel Opportunities

```
Phase 1 (Setup): T001 ─┐
                  T002 ─┤ все [P] параллельно
                  T003 ─┤
                  T004 ─┘

Phase 2 + Phase 6:  T005-T008 ║ T023-T024 (параллельно, разные зависимости)

Phase 3 ║ Phase 4:  US1 (multipart) ║ US2 (DLR expiry) — параллельно

Phase 12 ║ Phase 13: Billing ║ Purging — параллельно (разные сервисы)
```

---

## Parallel Example: User Story 1

```bash
# После Phase 2, запустить параллельно:
Task: T009 "Интегрировать подсчёт сегментов в messaging-service"
Task: T013 "Обновить proto-определение SendMessageResponse"

# Затем последовательно:
Task: T010 "Обновить gRPC handler SendMessage"
Task: T011 "Обновить HTTP handler send SMS"
Task: T012 "Обновить worker для multipart submit_sm"
```

---

## Implementation Strategy

### MVP First (US1 + US2)

1. Phase 1: Setup (миграции, конфигурация)
2. Phase 2: Foundational (multipart инфраструктура)
3. Phase 3: US1 — Send SMS с multipart
4. Phase 4: US2 — DLR expiry
5. **STOP и VALIDATE**: Отправить multipart SMS, проверить сегменты, дождаться expiry
6. Deploy/demo

### Incremental Delivery

1. Setup + Foundational → Инфраструктура готова
2. US1 + US2 → MVP: multipart + expiry ✅
3. US3 + Billing → Batch + посегментная тарификация ✅
4. US4 → 30-дневный лимит планирования ✅
5. US6 + US8 + US9 → Webhooks, аналитика, SMPP ✅
6. Data Purging → Автоочистка ✅
7. Polish → Метрики, дашборды ✅

### Parallel Team Strategy

С несколькими разработчиками после Phase 1+2:
- **Разработчик A**: US1 (multipart) → US3 (batch) → Billing
- **Разработчик B**: US2 (DLR expiry) → Data Purging → Polish
- **Разработчик C**: US4 (scheduling) → US6 (webhooks) → US8 (analytics) → US9 (SMPP)

---

## Notes

- [P] задачи = разные файлы, нет зависимостей
- US5 (Client Management) и US7 (Templates) уже полностью реализованы — пропущены
- Все пути файлов соответствуют существующей структуре проекта
- Proto файлы требуют перегенерации Go-кода после изменений
- Коммит после каждой задачи или логической группы
