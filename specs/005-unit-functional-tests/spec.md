# Feature Specification: Unit & Functional Tests

**Feature Branch**: `005-unit-functional-tests`
**Created**: 2026-03-21
**Status**: Draft
**Input**: Реализовать юнит и функциональные тесты для всех сервисов SMS-платформы

## Clarifications

### Session 2026-03-21

- Q: Существующие тесты (internal/router/, internal/api/, test/integration/, test/load/) входят в скоуп? → A: Новые тесты + рефакторинг существующих тестов для единообразия (Describe/Context/It структура)
- Q: Как работать с Kafka в функциональных тестах? → A: Kafka мокируется через интерфейс (EventPublisher/EventConsumer) — без реального брокера
- Q: Какой подход к мокированию для новых тестов? → A: testify/mock для всех новых моков, существующие function-field моки оставить как есть
- Q: Нужен ли минимальный порог покрытия кода? → A: 70% для application-слоя, без жёсткого порога для остальных слоёв
- Q: Как изолировать данные между функциональными тестами? → A: Транзакция с rollback — каждый тест оборачивается в транзакцию, которая откатывается после завершения

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Юнит-тесты Application-слоя сервисов (Priority: P1)

Разработчик запускает юнит-тесты и получает уверенность, что бизнес-логика каждого сервиса (auth, messaging, routing, billing, tarification) корректно обрабатывает все ключевые сценарии и граничные случаи без зависимости от внешних систем (БД, Redis, Kafka).

**Why this priority**: Application-слой содержит критическую бизнес-логику: аутентификация, маршрутизация, биллинг, тарификация. Ошибка здесь приводит к финансовым потерям или остановке сервиса.

**Independent Test**: Запускается `go test ./internal/services/*/application/...` — все тесты проходят за секунды без внешних зависимостей, используя моки интерфейсов.

**Acceptance Scenarios**:

1. **Given** AuthService с замоканными UserRepository и TokenService, **When** вызывается AuthenticateByCredentials с валидными логином и паролем, **Then** возвращается User, access token и refresh token
2. **Given** AuthService, **When** вызывается AuthenticateByCredentials с неверным паролем, **Then** возвращается ошибка ErrInvalidCredentials
3. **Given** MessageService с замоканным MessageRepository и EventPublisher, **When** вызывается SendMessage, **Then** создается Message со статусом PENDING и публикуется событие в Kafka
4. **Given** BillingService с замоканным AccountRepository, **When** вызывается ChargeAccount и баланс недостаточен, **Then** возвращается ошибка InsufficientFunds без изменения баланса
5. **Given** HLRService с замоканным HLRCache (cache hit), **When** вызывается LookupNumber, **Then** результат берется из кеша без обращения к провайдеру
6. **Given** HLRService с замоканным HLRCache (cache miss) и HLR-провайдером, **When** провайдер возвращает ошибку, **Then** происходит failover на следующий провайдер по приоритету
7. **Given** TarificationService со стратегией threshold, **When** вызывается CalculatePrice при объеме выше порога, **Then** возвращается цена из следующего тарифного тира

---

### User Story 2 - Юнит-тесты Domain-слоя (Priority: P1)

Разработчик убеждается, что доменные сущности корректно валидируют данные, управляют жизненным циклом состояний и вычисляют бизнес-свойства.

**Why this priority**: Доменные правила — фундамент платформы. Сломанная валидация номера или некорректный переход статуса сообщения приводят к каскадным ошибкам.

**Independent Test**: Запускается `go test ./internal/services/*/domain/...` — тесты чистых функций без зависимостей.

**Acceptance Scenarios**:

1. **Given** Message со статусом PENDING, **When** вызывается MarkAsQueued(), **Then** статус меняется на QUEUED
2. **Given** Message со статусом DELIVERED, **When** вызывается MarkAsFailed(), **Then** возвращается ошибка (недопустимый переход)
3. **Given** User с ролью "operator", **When** проверяется HasPermission("admin.delete"), **Then** возвращается false
4. **Given** LookupResult с number_status="active", **When** вызывается IsDeliverable(), **Then** возвращается true
5. **Given** LookupResult с number_status="invalid", **When** вызывается IsInvalid(), **Then** возвращается true
6. **Given** TariffPlan со стратегией "fixed", **When** проверяются валидационные правила, **Then** план считается валидным

---

### User Story 3 - Юнит-тесты Gateway-слоя (HTTP-хендлеры) (Priority: P2)

Разработчик убеждается, что HTTP-хендлеры корректно парсят запросы, возвращают правильные HTTP-статусы и структурированные ошибки.

**Why this priority**: Gateway — точка входа для клиентов. Некорректный ответ API или неправильный HTTP-статус ломает интеграции клиентов.

**Independent Test**: Запускается `go test ./internal/gateway/*/handlers/...` с httptest.NewServer и замоканными gRPC-клиентами.

**Acceptance Scenarios**:

1. **Given** Client Gateway с замоканным messaging gRPC client, **When** POST /send с валидным JSON, **Then** возвращается 200 с messageID
2. **Given** Client Gateway, **When** POST /send без Authorization header, **Then** возвращается 401 с JSON-ошибкой {"code": "UNAUTHORIZED", "message": "..."}
3. **Given** Client Gateway, **When** POST /send с невалидным JSON (пустой destination), **Then** возвращается 400 с описанием ошибки валидации
4. **Given** Portal Gateway, **When** POST /login с валидными credentials и включенным TOTP, **Then** возвращается 200 с requires_2fa=true и login_ticket
5. **Given** Admin Gateway, **When** DELETE /providers/{id} с невалидным UUID, **Then** возвращается 400 с описанием ошибки

---

### User Story 4 - Юнит-тесты Middleware (Priority: P2)

Разработчик убеждается, что middleware (auth, rate-limit, CSRF, recovery, CORS, logging) корректно перехватывает, обрабатывает и пропускает запросы.

**Why this priority**: Middleware отвечает за безопасность и стабильность. Сломанный auth middleware открывает неавторизованный доступ.

**Independent Test**: Запускается `go test ./internal/gateway/*/middleware/...` с httptest и замоканными зависимостями.

**Acceptance Scenarios**:

1. **Given** Auth middleware с замоканным auth gRPC client, **When** запрос с валидным JWT-токеном, **Then** запрос пропускается дальше с user info в контексте
2. **Given** Auth middleware, **When** запрос с просроченным JWT-токеном, **Then** возвращается 401
3. **Given** Rate-limit middleware, **When** превышен лимит запросов, **Then** возвращается 429 Too Many Requests
4. **Given** CSRF middleware (Portal), **When** POST-запрос без CSRF-токена, **Then** возвращается 403
5. **Given** Recovery middleware, **When** handler паникует, **Then** возвращается 500 и паника не пробрасывается наружу

---

### User Story 5 - Юнит-тесты gRPC-серверов (Priority: P2)

Разработчик убеждается, что gRPC-серверы корректно маппят запросы protobuf на вызовы application-сервисов и возвращают правильные gRPC-коды и ответы.

**Why this priority**: gRPC — основной канал межсервисной коммуникации. Некорректный маппинг или неверный gRPC-код нарушает всю цепочку обработки.

**Independent Test**: Запускается `go test ./internal/services/*/grpc/...` с замоканными application-сервисами и bufconn.

**Acceptance Scenarios**:

1. **Given** MessagingServiceServer с замоканным MessageService, **When** вызывается SendMessage с валидным запросом, **Then** возвращается response с messageID и codes.OK
2. **Given** MessagingServiceServer, **When** MessageService возвращает ErrNotFound, **Then** gRPC возвращает codes.NotFound с описанием
3. **Given** AuthServiceServer, **When** вызывается Authenticate с невалидным паролем, **Then** gRPC возвращает codes.Unauthenticated
4. **Given** BillingServiceServer, **When** вызывается ChargeAccount и баланс недостаточен, **Then** gRPC возвращает codes.FailedPrecondition

---

### User Story 6 - Функциональные тесты: SMS-цепочка отправки (Priority: P1)

Разработчик запускает функциональный тест, который проверяет полный путь SMS-сообщения: от HTTP-запроса клиента до создания записи в БД и публикации Kafka-события.

**Why this priority**: Это основной бизнес-сценарий платформы — отправка SMS. Без него платформа не имеет ценности.

**Independent Test**: Запускается `go test ./test/functional/...` с тестовой БД (PostgreSQL) и моком Kafka. Проверяется полная цепочка в рамках одного сервиса.

**Acceptance Scenarios**:

1. **Given** запущенный messaging-service с тестовой БД, **When** gRPC вызов SendMessage(clientID, source="+79001234567", destination="+79009876543", text="Hello"), **Then** в таблице messages появляется запись со статусом PENDING и публикуется событие sms.outgoing в Kafka
2. **Given** сообщение в статусе SENT в БД, **When** приходит DLR с stat=DELIVRD, **Then** статус сообщения обновляется на DELIVERED и создается запись в dlr_receipts
3. **Given** сообщение в статусе PENDING, **When** клиент вызывает CancelMessage, **Then** статус обновляется на CANCELED и повторная отмена возвращает ошибку

---

### User Story 7 - Функциональные тесты: Биллинг и тарификация (Priority: P1)

Разработчик проверяет, что тарификация корректно вычисляет цену на основе оператора, категории и объема, а биллинг корректно списывает средства с аккаунта.

**Why this priority**: Ошибки в биллинге приводят к прямым финансовым потерям.

**Independent Test**: Запускается функциональный тест с тестовой БД. Проверяется: создание тарифного плана, расчет цены, списание с баланса.

**Acceptance Scenarios**:

1. **Given** аккаунт с балансом 100.00 и тарифный план (fixed, 0.50 за SMS), **When** ChargeAccount на 0.50, **Then** баланс становится 99.50 и создается запись transaction с type=charge
2. **Given** аккаунт с балансом 0.10 и цена SMS 0.50, **When** ChargeAccount, **Then** возвращается ошибка InsufficientFunds и баланс остается 0.10
3. **Given** тарифный план threshold с тирами [0-1000: 0.50, 1001-5000: 0.40], **When** CalculatePrice при объеме 1500, **Then** цена 0.40
4. **Given** успешная транзакция charge, **When** RefundTransaction, **Then** баланс увеличивается обратно и создается транзакция type=refund

---

### User Story 8 - Функциональные тесты: Auth и сессии (Priority: P2)

Разработчик проверяет полный цикл аутентификации: от создания пользователя до входа, работы с токенами, 2FA и API-ключами.

**Why this priority**: Аутентификация — ворота ко всей платформе. Утечка или обход аутентификации — критический инцидент безопасности.

**Independent Test**: Запускается функциональный тест с тестовой БД и тестовым Redis. Проверяется: регистрация, логин, refresh, TOTP, API key CRUD.

**Acceptance Scenarios**:

1. **Given** пользователь с паролем в БД, **When** Authenticate с правильным паролем, **Then** возвращается accessToken и refreshToken, и refreshToken сохраняется в Redis
2. **Given** валидный refreshToken, **When** RefreshToken, **Then** возвращается новый accessToken и старый refreshToken инвалидируется
3. **Given** пользователь с настроенным TOTP, **When** Authenticate без TOTP-кода, **Then** возвращается ошибка с требованием 2FA
4. **Given** созданный API-ключ с scope=["send_sms"], **When** AuthenticateByAPIKey, **Then** возвращается User с ограниченными permissions
5. **Given** API-ключ с allowed_ips=["10.0.0.1"], **When** запрос с IP 192.168.1.1, **Then** возвращается ошибка Unauthorized

---

### User Story 9 - Функциональные тесты: HLR и маршрутизация (Priority: P2)

Разработчик проверяет, что HLR-lookup корректно определяет оператора, кеширует результат и выполняет failover, а маршрутизация выбирает оптимальный маршрут.

**Why this priority**: Smart routing напрямую влияет на стоимость и доставляемость SMS.

**Independent Test**: Запускается функциональный тест с тестовой БД и мок-Redis. Проверяются HLR lookups, кеширование, failover и route selection.

**Acceptance Scenarios**:

1. **Given** HLR-провайдер в БД и пустой Redis-кеш, **When** LookupNumber("+79001234567"), **Then** результат возвращается от провайдера, записывается в кеш и в lookup_log
2. **Given** результат в Redis-кеше, **When** LookupNumber без forceRefresh, **Then** результат берется из кеша (cached=true), провайдер не вызывается
3. **Given** результат в Redis-кеше, **When** LookupNumber с forceRefresh=true, **Then** кеш игнорируется и вызывается провайдер
4. **Given** провайдер 1 (priority=1) недоступен, провайдер 2 (priority=2) доступен, **When** LookupNumber, **Then** используется провайдер 2 и статус провайдера 1 обновляется
5. **Given** маршруты с разными приоритетами и весами, **When** RouteMessage, **Then** выбирается маршрут согласно весам smart routing

---

### Edge Cases

- Что происходит при таймауте подключения к БД при создании сообщения? Тест должен вернуть ошибку без изменения состояния
- Что происходит при одновременном списании с одного аккаунта (race condition)? Тест должен гарантировать атомарность через SELECT FOR UPDATE
- Что происходит при невалидном E.164 номере в HLR lookup? Должна вернуться ошибка валидации без обращения к провайдеру
- Что происходит при истечении TTL всех HLR-провайдеров? Должна вернуться последняя доступная информация или ошибка
- Что происходит при попытке отправить SMS с пустым текстом? Валидация на уровне domain должна отклонить
- Что происходит при создании тарифного плана с пересекающимися тирами? Должна вернуться ошибка валидации
- Что происходит при refresh токена после его revoke? Должна вернуться ошибка InvalidToken
- Что происходит при панике в Kafka consumer? Recovery middleware должна перехватить и залогировать

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Система ДОЛЖНА иметь юнит-тесты для application-слоя всех 7 сервисов (auth, messaging, routing, billing, tarification, analytics, client) с изоляцией через моки интерфейсов
- **FR-002**: Система ДОЛЖНА иметь юнит-тесты для domain-слоя: валидация сущностей, переходы состояний, вычисление бизнес-свойств
- **FR-003**: Система ДОЛЖНА иметь юнит-тесты для HTTP-хендлеров всех трёх gateway (admin, client, portal) с проверкой HTTP-статусов и структуры ответов
- **FR-004**: Система ДОЛЖНА иметь юнит-тесты для middleware (auth, rate-limit, CSRF, recovery, CORS) каждого gateway
- **FR-005**: Система ДОЛЖНА иметь юнит-тесты для gRPC-серверов с проверкой маппинга ошибок domain в gRPC codes
- **FR-006**: Система ДОЛЖНА иметь функциональные тесты для полной цепочки отправки SMS (создание, статус, DLR, обновление)
- **FR-007**: Система ДОЛЖНА иметь функциональные тесты для цепочки биллинг + тарификация (расчет цены, списание, рефанд)
- **FR-008**: Система ДОЛЖНА иметь функциональные тесты для auth-цепочки (login, token, refresh, TOTP, API key)
- **FR-009**: Система ДОЛЖНА иметь функциональные тесты для HLR lookup + smart routing (lookup, cache, failover, route selection)
- **FR-010**: Все юнит-тесты ДОЛЖНЫ запускаться без внешних зависимостей (без БД, Redis, Kafka)
- **FR-011**: Функциональные тесты ДОЛЖНЫ использовать тестовую БД (PostgreSQL) с изоляцией через транзакцию с rollback, и, где необходимо, тестовый Redis. Kafka ДОЛЖНА мокироваться через интерфейсы EventPublisher/EventConsumer без реального брокера
- **FR-012**: Тесты ДОЛЖНЫ проверять не только наличие ответа, но и структуру бизнес-ошибок (код, сообщение, детали)
- **FR-013**: Тесты ДОЛЖНЫ использовать testify/assert, testify/require и, при необходимости, testify/mock для мокирования интерфейсов
- **FR-014**: Юнит-тесты ДОЛЖНЫ использовать вложенную структуру: Describe (компонент), Context (условие), It (ожидание) через t.Run с соответствующим неймингом
- **FR-015**: Существующие тесты (internal/router/, internal/api/, internal/smpp/, internal/monitoring/, internal/shared/) ДОЛЖНЫ быть рефакторены для единообразной структуры Describe/Context/It
- **FR-016**: Существующие integration/load тесты (test/integration/, test/load/) ДОЛЖНЫ быть приведены к единой структуре именования и организации
- **FR-017**: Функциональные тесты ДОЛЖНЫ использовать build tag `//go:build functional` для изоляции от `go test ./...`. Запуск: `go test -tags=functional ./test/functional/...`

### Key Entities

- **TestSuite**: Набор связанных тестов для одного компонента с shared setup/teardown
- **Mock**: Тестовый двойник интерфейса, реализующий контракт без реальной логики
- **Fixture**: Фабрика тестовых данных для создания валидных сущностей с предсказуемыми значениями
- **TestDB**: Изолированная тестовая база данных с изоляцией через транзакцию с rollback после каждого теста

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Юнит-тесты покрывают все публичные методы application-слоя всех 7 сервисов (auth, messaging, routing, billing, tarification, analytics, client)
- **SC-002**: Каждый публичный метод application-сервиса имеет минимум 2 теста: Happy Path и основной Error Case
- **SC-003**: Domain-тесты покрывают все методы переходов состояний и валидации сущностей
- **SC-004**: HTTP-хендлеры тестируются на все HTTP-статусы, которые они могут вернуть (200, 400, 401, 403, 404, 500)
- **SC-005**: Функциональные тесты покрывают 4 ключевые бизнес-цепочки: SMS-отправка, биллинг, аутентификация, HLR/маршрутизация
- **SC-006**: Все юнит-тесты проходят за менее чем 30 секунд без внешних зависимостей
- **SC-007**: Бизнес-ошибки проверяются на структуру: наличие кода ошибки, описания и, где применимо, деталей
- **SC-008**: gRPC-серверы тестируются на корректный маппинг доменных ошибок в gRPC status codes
- **SC-009**: Покрытие кода application-слоя каждого сервиса составляет не менее 70% (go test -cover)

## Assumptions

- Проект уже использует testify/assert и testify/require (подтверждено анализом go.mod)
- Существующие тестовые утилиты в internal/testutil/ (mocks.go, fixtures.go, testdb.go) будут расширены, а не заменены
- Новые моки создаются через testify/mock (AssertExpectations, матчеры). Существующие hand-written function-field моки в internal/testutil/mocks.go остаются как есть
- Функциональные тесты требуют доступной тестовой PostgreSQL (настраивается через TEST_DB_* env vars)
- Функциональные тесты для кеширования используют тестовый Redis (настраивается через TEST_REDIS_* env vars)
