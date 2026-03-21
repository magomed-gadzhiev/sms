# Research: Multi-tenant Self-Service Portal + Sub-accounts

**Feature Branch**: `003-self-service-portal`
**Date**: 2026-03-21

## R-001: Сессионная аутентификация для портала

**Decision**: Redis-based sessions с secure HTTP-only cookies.

**Rationale**: Существующая auth система использует JWT + API-ключи для machine-to-machine. Для web-портала нужны сессии: HTTP-only cookies защищают от XSS, серверные сессии позволяют мгновенный logout и контроль concurrent sessions. Redis уже используется в проекте (go-redis/v9) для кеширования.

**Alternatives considered**:
- JWT в cookies: Нет мгновенного logout, сложнее управлять сессиями. Отвергнуто.
- JWT + refresh в Redis: Избыточная сложность, по сути те же серверные сессии с JWT overhead. Отвергнуто.

**Implementation**:
- Сессия: UUID → Redis hash (user_id, client_id, role, created_at, expires_at)
- Cookie: `portal_session`, HttpOnly, Secure, SameSite=Strict, Max-Age=24h
- Redis key: `session:{session_id}`, TTL=24h
- Concurrent sessions: максимум 5 на пользователя

## R-002: TOTP двухфакторная аутентификация

**Decision**: TOTP (RFC 6238) через библиотеку `pquerna/otp`.

**Rationale**: Стандартный протокол, поддерживается всеми authenticator-приложениями (Google Authenticator, Authy, 1Password). Библиотека `pquerna/otp` — наиболее зрелая Go-реализация (>5k stars, активно поддерживается).

**Alternatives considered**:
- WebAuthn/FIDO2: Более безопасно, но сложнее в реализации и требует поддержку на фронтенде. Можно добавить позже. Отвергнуто для MVP.
- SMS-коды: Менее безопасно (SIM swap), создаёт зависимость от собственной SMS-инфраструктуры для auth. Отвергнуто.

**Implementation**:
- TOTP-секрет шифруется перед записью в БД (AES-256-GCM, ключ из env)
- Recovery codes: 10 одноразовых кодов при активации 2FA
- QR-код для добавления в authenticator через `otpauth://` URI
- Verify при каждом логине если 2FA активна

## R-003: Защита от brute-force (login throttling)

**Decision**: Rate limiting на уровне email + IP через Redis counters.

**Rationale**: FR-019 требует блокировку после 5 неудачных попыток. Redis counters с TTL — простое и эффективное решение, уже есть Redis в стеке.

**Alternatives considered**:
- In-memory rate limiter: Не работает с несколькими инстансами gateway. Отвергнуто.
- Captcha: Дополнительная сложность для MVP, можно добавить позже. Отвергнуто.

**Implementation**:
- Redis key: `login_attempts:{email}`, increment на каждую неудачную попытку, TTL=15min
- После 5 попыток — возврат 429 с Retry-After: 900
- Отдельный счётчик по IP: `login_attempts_ip:{ip}`, лимит 20 попыток/15 мин (защита от перебора разных аккаунтов)
- Успешный логин сбрасывает счётчик email

## R-004: Sub-accounts — модель данных

**Decision**: Расширение таблицы `clients` с полями `parent_client_id`, `is_reseller`, `max_sub_accounts`.

**Rationale**: Sub-account — по сути тот же client с ограничениями. Расширение существующей таблицы вместо новой минимизирует изменения: все существующие сервисы (billing, messaging, webhook) уже работают с client_id. Sub-account получает свой client_id и сразу работает со всей инфраструктурой.

**Alternatives considered**:
- Отдельная таблица `sub_accounts`: Дублирование логики client, все сервисы нужно учить работать с двумя типами сущностей. Отвергнуто.
- Hierarchical tenant model с отдельной таблицей `tenants`: Ovengineering для одноуровневой иерархии. Отвергнуто.

**Implementation**:
- `clients.parent_client_id` (uuid, nullable, FK → clients.id) — NULL для обычных клиентов и реселлеров, заполнено для sub-accounts
- `clients.is_reseller` (bool, default false) — активируется администратором
- `clients.max_sub_accounts` (int, default 0) — лимит sub-accounts
- Sub-account при создании получает: нового user'а (auth), новый client (client-service), новый account (billing)
- Constraint: parent_client_id не может ссылаться на sub-account (один уровень)

## R-005: Балансовые переводы (реселлер → sub-account)

**Decision**: Атомарная транзакция с двумя записями в transactions + обновление балансов.

**Rationale**: Конституция требует атомарности балансовых операций (Principle V). Перевод — это две операции: списание с реселлера + зачисление на sub-account. Обе должны быть в одной PostgreSQL-транзакции.

**Implementation**:
- Новый TransactionType: `transfer_out` (реселлер) и `transfer_in` (sub-account)
- Одна PostgreSQL-транзакция: UPDATE accounts SET balance (оба) + INSERT transactions (два)
- Проверка: parent_client_id sub-account == client_id реселлера
- Проверка: достаточный баланс реселлера

## R-006: Audit logging

**Decision**: Shared пакет `internal/shared/audit/` + Kafka topic `audit.events` + consumer в worker.

**Rationale**: Audit — кросс-сервисная задача. Публикация через Kafka обеспечивает асинхронность (не блокирует основные операции) и единую точку записи. Consumer в существующем worker (конституция: не создавать новые сервисы).

**Alternatives considered**:
- Синхронная запись в БД из каждого сервиса: Увеличивает латентность, связывает сервисы с audit-схемой. Отвергнуто.
- Отдельный audit-service: Нарушает Principle VI (Simplicity). Отвергнуто.

**Implementation**:
- Kafka topic: `audit.events`, retention 7 days
- Event: `{event_id, tenant_id, user_id, action, resource, resource_id, details, ip_address, timestamp}`
- Consumer пишет в `audit_log` (monthly partitioned)
- Portal запрашивает audit_log напрямую через audit gRPC service (query-only, read path отделён от write path)
- Retention: 1 год (Principle V: Data Safety)

## R-007: Frontend технология

**Decision**: React 19 + Vite + TypeScript.

**Rationale**: React — стандарт для сложных интерактивных порталов. Vite — быстрая сборка. TypeScript — типобезопасность. Нужны графики (recharts), таблицы с фильтрацией, формы — всё это хорошо покрывается React-экосистемой. Команда может не быть экспертами в React, но это наиболее документированный и поддерживаемый фреймворк.

**Alternatives considered**:
- Go templates + HTMX: Проще, но ограничен для дашбордов с графиками и сложной фильтрацией. Отвергнуто.
- Vue.js: Хорошая альтернатива, но меньше экосистема компонентов. Отвергнуто.
- Svelte: Новее, меньше сообщество. Отвергнуто.

**Implementation**:
- SPA served через nginx (отдельный контейнер)
- API calls к portal-gateway через /portal/v1/
- HAProxy проксирует: /portal/v1/* → portal-gateway, /* → nginx (frontend)
- UI library: shadcn/ui (headless, кастомизируемый)
- Charts: recharts
- State management: React Query (TanStack Query) для серверного стейта

## R-008: Password reset flow

**Decision**: Одноразовый токен в email, действителен 1 час.

**Rationale**: Стандартный подход, соответствует FR-018. Токен хешируется перед записью в БД (как API-ключи).

**Implementation**:
- Генерация: crypto/rand, 32 bytes → base64url
- Хранение: SHA256 hash в таблице `password_reset_tokens`
- TTL: 1 час, одноразовый (удаляется после использования)
- Email: ссылка `https://portal.example.com/reset-password?token={token}`
- Rate limit: максимум 3 запроса на сброс в час на email

## R-009: CSRF-защита

**Decision**: Double-submit cookie pattern с crypto-random token.

**Rationale**: SPA отправляет API-запросы через fetch — стандартный CSRF-паттерн: cookie + заголовок X-CSRF-Token. Простой и надёжный для SPA-архитектуры.

**Implementation**:
- При создании сессии: генерировать CSRF-токен, записать в cookie `csrf_token` (NOT HttpOnly, чтобы JS мог прочитать)
- Фронтенд: читает cookie, отправляет в заголовке `X-CSRF-Token`
- Middleware: проверяет совпадение cookie и заголовка для POST/PUT/DELETE
- GET-запросы не проверяются
