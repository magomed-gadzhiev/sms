# Full Review Checklist: SMS Gateway Platform

**Purpose**: Полная проверка качества требований спецификации перед production — полнота, ясность, консистентность, покрытие сценариев
**Created**: 2026-03-20
**Feature**: [spec.md](../spec.md)
**Depth**: Standard | **Audience**: Author (self-review)

## Requirement Completeness

- [ ] CHK001 - Определены ли требования для всех 3 протоколов приёма сообщений (HTTP, gRPC, SMPP) с одинаковой степенью детализации? [Completeness, Spec §FR-001]
- [ ] CHK002 - Описаны ли требования к формату ответа при частичном успехе batch-отправки (какие поля возвращаются для успешных и отклонённых сообщений)? [Completeness, Spec §FR-009]
- [ ] CHK003 - Определён ли максимальный размер batch (конкретное число) в спецификации, а не только в success criteria? [Completeness, Gap]
- [ ] CHK004 - Документированы ли требования к формату и валидации номера телефона (E.164, длина, допустимые символы)? [Completeness, Gap]
- [ ] CHK005 - Описаны ли требования к максимальной длине поля `body` (количество сегментов × символов)? [Completeness, Spec §FR-021]
- [ ] CHK006 - Определены ли требования к поведению системы при старте (миграции, seed data, health checks)? [Completeness, Gap]
- [ ] CHK007 - Документированы ли все возможные коды ошибок API и их семантика? [Completeness, Gap]
- [ ] CHK008 - Определены ли требования к аудит-логу — какие именно события логируются? [Completeness, Spec §FR-018]

## Requirement Clarity

- [ ] CHK009 - Уточнено ли "configurable routing rules" (FR-004) — кто конфигурирует, через какой интерфейс, с какой гранулярностью? [Clarity, Spec §FR-004]
- [ ] CHK010 - Определён ли "configurable maximum number of attempts" для webhook retry (FR-014) — значение по умолчанию, допустимый диапазон? [Clarity, Spec §FR-014]
- [ ] CHK011 - Конкретизировано ли "cryptographic signature" для webhooks (FR-013) — алгоритм (HMAC-SHA256?), формат заголовка, формат payload для подписи? [Clarity, Spec §FR-013]
- [ ] CHK012 - Определено ли "destination-specific pricing rules" (FR-011) — приоритет правил, формат паттернов, поведение при отсутствии правила? [Clarity, Spec §FR-011]
- [ ] CHK013 - Уточнено ли "preserve message ordering within a single client session" (FR-020) — относится ли это к HTTP, gRPC, SMPP или всем протоколам? [Clarity, Spec §FR-020]
- [ ] CHK014 - Определено ли "automatically failover" (FR-005) — критерии определения недоступности провайдера, таймаут, количество попыток? [Clarity, Spec §FR-005]

## Requirement Consistency

- [ ] CHK015 - Согласованы ли статусы сообщений между spec (FR-006: 8 статусов) и data-model (state transitions)? [Consistency, Spec §FR-006]
- [ ] CHK016 - Согласована ли терминология "segment" vs "part" между спецификацией и clarifications? [Consistency]
- [ ] CHK017 - Согласованы ли требования к rate limiting (FR-008: per second/minute/hour) с US5 (acceptance scenario 3: "throttles or rejects")? [Consistency, Spec §FR-008]
- [ ] CHK018 - Согласован ли лимит планирования "30 days" между FR-010, Clarifications и plan.md constraints? [Consistency, Spec §FR-010]
- [ ] CHK019 - Согласованы ли требования к webhook events между spec (FR-013: "sent, delivered, failed") и data-model (ValidEventTypes включает "expired", "rejected")? [Consistency, Spec §FR-013]

## Acceptance Criteria Quality

- [ ] CHK020 - Можно ли объективно измерить SC-001 "under normal load" — определено ли "normal load"? [Measurability, Spec §SC-001]
- [ ] CHK021 - Можно ли объективно проверить SC-003 "99.9% of messages" — определён ли период измерения и метод подсчёта? [Measurability, Spec §SC-003]
- [ ] CHK022 - Определён ли метод измерения SC-010 "99.95% uptime" — какой компонент считается "down"? [Measurability, Spec §SC-010]
- [ ] CHK023 - Определены ли acceptance scenarios для US8 (Analytics) для случая пустых данных (нет сообщений за период)? [Acceptance Criteria, Spec §US8]
- [ ] CHK024 - Определены ли acceptance scenarios для US9 (SMPP) для случая потери TCP-соединения mid-session? [Acceptance Criteria, Gap]

## Scenario Coverage

- [ ] CHK025 - Описаны ли требования к поведению при одновременной отправке и отмене scheduled сообщения (race condition)? [Coverage, Edge Case]
- [ ] CHK026 - Описаны ли требования к поведению при multipart SMS, когда не все сегменты доставлены (частичная доставка)? [Coverage, Edge Case, Spec §FR-021]
- [ ] CHK027 - Описаны ли требования к поведению при переполнении batch в процессе обработки (out of memory)? [Coverage, Edge Case]
- [ ] CHK028 - Описаны ли требования к поведению при смене провайдера mid-delivery (failover после частичной отправки multipart)? [Coverage, Edge Case, Spec §FR-005]
- [ ] CHK029 - Описаны ли требования к поведению при дублировании DLR от провайдера (idempotency)? [Coverage, Edge Case]
- [ ] CHK030 - Определены ли требования к поведению при concurrent обновлении баланса (race condition при параллельных списаниях)? [Coverage, Edge Case, Spec §FR-011]

## Non-Functional Requirements

- [ ] CHK031 - Определены ли требования к observability — какие метрики, логи, трейсы обязательны? [Non-Functional, Gap]
- [ ] CHK032 - Определены ли требования к latency для каждого критического пути (не только SC-001 для submit)? [Non-Functional, Spec §SC-001]
- [ ] CHK033 - Определены ли требования к graceful degradation — как система ведёт себя при частичной деградации (1 из 3 провайдеров недоступен)? [Non-Functional, Gap]
- [ ] CHK034 - Определены ли требования к backup/restore для PostgreSQL и конфигурации? [Non-Functional, Gap]
- [ ] CHK035 - Определены ли требования к TLS/SSL для всех внешних соединений (SMPP, webhooks, API)? [Non-Functional, Security, Gap]
- [ ] CHK036 - Определены ли требования к защите sensitive данных в логах (API keys, пароли провайдеров, тела сообщений)? [Non-Functional, Security, Gap]
- [ ] CHK037 - Определены ли требования к time zone handling (все timestamps в UTC? client timezone support?)? [Non-Functional, Gap]

## Dependencies & Assumptions

- [ ] CHK038 - Документирована ли зависимость от внешних SMSC провайдеров и их SLA? [Dependency, Gap]
- [ ] CHK039 - Документировано ли предположение о доступности Kafka — что если Kafka кластер недоступен? [Assumption, Spec §Edge Cases]
- [ ] CHK040 - Документированы ли требования к версионированию API — breaking changes, deprecation policy? [Dependency, Gap]

## Notes

- Пометьте пункты как выполненные: `[x]`
- Для не-прошедших пунктов — добавьте комментарий с описанием пробела
- Пункты с `[Gap]` указывают на отсутствующие требования, которые стоит добавить в spec.md
