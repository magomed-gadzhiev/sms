# Spec №4: Админ → Клиенты — заменить «Удалить» на «Заблокировать» с каскадом

**Дата:** 2026-04-26
**Источник:** MVP-таблица, вкладка «Админка Платформы», строка «Суб-аккаунты»
**Статус:** Сделано на ~25%.

## Контекст

Спека:
> Суб-аккаунты переименовать в Клиенты. Убрать "Удалить субаккаунт". Добавить функционал "Заблокировать аккаунт":
> - приостанавливается работа API клиента
> - отправка неотправленных сообщений
> - ограничивается доступ к аккаунту через авторизацию
> - прекратить все текущие сессии

Аудит:
- UI: [ClientsPage.tsx:103](portal-frontend/src/pages/admin/ClientsPage.tsx) — заголовок уже «Клиенты», в nav метка «Клиенты». Переименование сделано.
- UI: кнопка «Деактивировать» вызывает `clientsApi.delete(client_id)` → DELETE `/admin/v1/clients/:id`. То есть UI смягчён, но бэкенд по-прежнему DELETE.
- БД: поле `active: boolean` существует и используется в `domain/user.go:IsActive`. Из 4 пунктов спеки реализован только этот флаг (1/4).
- Pipeline: `internal/pipeline/router/stage.go` и `persist/stage.go` **не проверяют** `client.active` — заблокированный клиент будет дозревать в очереди.
- Auth: `IsActive` проверяется только при логине (пароль). API-ключи — независимая ветка, без чека `client.active`.
- Refresh-токены: таблица `refresh_tokens` есть, но `RevokeAllForUser`/`RevokeAllForClient` отсутствует.
- SMPP-биндинги клиента: `internal/gateway/smpp/server/redis_store.go` хранит сессии, но триггера на «закрыть всё для клиента N» нет.

## Решение

### Подход

**Soft-block через флаг + чеки на горячих путях + явный отзыв сессий.** Hard-block с Redis-блоклистом JWT не делаем (TTL access-токена ~15 мин, дыра приемлема для MVP). Если позже придёт SLA <1 мин на блокировку — добавляем Redis-блоклист отдельным спеком.

### Что меняем

**1. Backend: новая операция `BlockClient` / `UnblockClient`**

В `internal/services/auth/` (или `internal/services/account/` — где живёт сущность клиента):

```
POST /admin/v1/clients/:id/block
  body: { reason: string (optional, max 500 chars) }
  effect: account.active = false, account.blocked_at = NOW(), account.block_reason = reason

POST /admin/v1/clients/:id/unblock
  body: {}
  effect: account.active = true, account.blocked_at = NULL, account.block_reason = NULL
```

Миграция: `ALTER TABLE accounts ADD COLUMN blocked_at TIMESTAMPTZ NULL, ADD COLUMN block_reason TEXT NULL`. Индекс не нужен (этих записей мало, фильтрация — точечная).

**2. Backend: каскад при блокировке**

`BlockClient` после установки флага синхронно делает:

a. **Отзыв refresh-токенов:** `DELETE FROM refresh_tokens WHERE user_id IN (SELECT id FROM users WHERE client_id = $1)`. Реализовать `refresh_token_repository.RevokeAllForClient(ctx, clientID)`.

b. **Закрытие SMPP-биндингов клиента:** через `internal/gateway/smpp/server/` — найти все активные сессии где `clientID == X`, отправить `Unbind` PDU, убрать из `redis_store`. Реализовать `smppServer.DisconnectClient(ctx, clientID)`.

c. **Audit log:** запись в `audit_log` с `action=client.blocked`, `actor_user_id=admin.id`, `target_client_id=X`, `metadata={"reason": "..."}`.

**3. Backend: чеки на горячих путях**

a. **Client-gateway middleware** ([internal/gateway/portal/router/router.go](internal/gateway/portal/router/router.go) и [internal/gateway/client/](internal/gateway/client/)): после успешной аутентификации (JWT или API-key) проверять `client.active`. Если `false` — 403 `{"error": "client_blocked", "reason": "..."}`. Чек кешировать в memory ~30 сек чтобы не бить БД на каждый запрос; на BlockClient — инвалидировать кеш.

b. **Pipeline router stage** ([internal/pipeline/router/stage.go](internal/pipeline/router/stage.go)): перед отправкой сообщения в провайдера проверять `client.active`. Если `false` — переводить сообщение в статус `cancelled` с `reason=client_blocked` (через ту же `messageRepo.UpdateStatus`), не отправлять в провайдера. Не падаем pipeline — это нормальный исход.

c. **API-key auth middleware:** при валидации API-ключа дополнительно проверять `client.active` (тот же кеш-чек).

**4. Frontend**

a. На [ClientsPage.tsx](portal-frontend/src/pages/admin/ClientsPage.tsx):
   - Кнопку «Деактивировать» переименовать в «Заблокировать», поменять цвет на красный destructive.
   - При нажатии — модалка с обязательным textarea «Причина блокировки» (макс 500 символов) и кнопками «Отмена / Заблокировать».
   - Для заблокированных клиентов в строке таблицы показывать badge «Заблокирован» (красный) и кнопку «Разблокировать» вместо «Заблокировать».

b. **Кнопку DELETE убрать совсем.** Если когда-то потребуется реальное удаление — отдельной операцией под supersuper-admin, не в списке.

c. В колонке «Статус» добавить значение «Заблокирован» с tooltip'ом на причину.

### Что НЕ делаем

- **Hard-block через Redis-блоклист JWT.** Окно неконсистентности 0–15 мин (TTL access-токена) приемлемо для MVP. Если бизнес скажет «надо мгновенно» — добавляем Redis-проверку отдельным спеком.
- **Eventual через Kafka событие `client.blocked`.** Синхронный каскад проще; масштаб не требует.
- **Возобновление cancelled-сообщений при разблокировке.** Сообщения остаются `cancelled`, новые после разблокировки идут нормально. Re-enqueue — отдельная фича по запросу.
- **Удаление параллельного клиентского раздела `pages/sub-accounts/`.** Это другой сценарий (управление субаккаунтами клиентом, не админом), не трогаем.

## Архитектура

```
[Admin UI: Block button]
    │
    ▼
POST /admin/v1/clients/:id/block { reason }
    │
    ▼
BlockClientHandler
    │
    ├─► UPDATE accounts SET active=false, blocked_at=NOW(), block_reason=$1
    ├─► refresh_token_repository.RevokeAllForClient(clientID)
    ├─► smppServer.DisconnectClient(clientID)
    ├─► auditLog.Record(client.blocked, ...)
    └─► cache.InvalidateClientStatus(clientID)
    │
    ▼ (returns 200)

[Pipeline router stage]
    receive message → load client.active (cached 30s) → if false: status=cancelled, reason=client_blocked

[Client-gateway middleware]
    authenticated request → load client.active (cached 30s) → if false: 403
```

## Acceptance criteria

1. В админке кнопка называется «Заблокировать» (не «Деактивировать», не «Удалить»).
2. При блокировке открывается модалка с обязательной причиной.
3. После блокировки:
   - Любой API-запрос от клиента (JWT или API-key) → 403 `client_blocked`.
   - Активные SMPP-сессии клиента — закрыты в течение 5 секунд.
   - Refresh-токены клиента — удалены, повторный логин невозможен.
   - Сообщения в Kafka, дошедшие до router stage, переходят в `cancelled` с `reason=client_blocked`.
   - В audit_log появляется запись `client.blocked` с причиной и админом.
4. Существующие access-токены клиента продолжают работать до истечения TTL (~15 мин) — это известное допущение.
5. Кнопка «Разблокировать» возвращает `active=true`, очищает `blocked_at`/`block_reason`. Cancelled-сообщения **не** возобновляются.
6. Существующий DELETE-эндпоинт `/admin/v1/clients/:id` либо удалён, либо требует extra-флага `?force=true&reason=...` и недоступен из UI.

## Размер задачи

M (3-4 рабочих дня: бэк-каскад с тестами + UI с модалкой + проверка SMPP-disconnect).
