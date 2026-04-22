# Fix Plan: gRPC auth interceptor stub

**Severity:** critical
**Source:** Cycle 2 finding A7.3 (`docs/reports/2026-04-21-cycle2-findings.md`)
**Spec reference:** `docs/superpowers/specs/2026-04-21-api-review-v2-umbrella.md`

## Problem

Оба gRPC unary interceptor'а для внешнего трафика — `internal/gateway/client/grpc/interceptor.go` и `internal/api/grpc/interceptors.go` — захардкожены как stub: устанавливают фиксированный dummy UUID `00000000-0000-0000-0000-000000000001` в контекст каждого запроса. Параметр `authClient` принимается, но `authClient.ValidateToken()` не вызывается. Это не конфигурационный флаг и не load-test mode — просто dead stub.

**Следствие:**
- Любой вызов gRPC на `CLIENT_GRPC_PORT` (9090 по умолчанию) получает один и тот же dummy tenant.
- Multi-tenant изоляция на gRPC-интерфейсе отсутствует полностью.
- messagingv1.GetMessageStatus — единственный RPC, где downstream handler допускает nil clientID и пропускает ownership check; через proxy это не эксплуатируется (stub всё равно инжектит non-nil dummy UUID), но если кто-то добавит ещё такое поведение — дополнительная поверхность.

## Proposed fix

1. Заменить stub-логику на вызов `authClient.ValidateToken(ctx, &authv1.ValidateTokenRequest{Token: extractedToken})` и использовать `resp.ClientId` вместо dummy.
2. Token extraction — аналогично HTTP middleware: из metadata `authorization: Bearer <...>` или `x-api-key: <...>`.
3. Ошибку ValidateToken конвертировать в gRPC `codes.Unauthenticated` с осмысленным сообщением.
4. Инжектить `client_id` в ctx через тот же context key, что использует HTTP middleware (проверить `ClientAuthMiddleware` и использовать идентичный ключ).
5. Предусмотреть `LOAD_TEST_MODE` env-var bypass (если HTTP такой имеет — matching).

## Tasks (to be detailed when taken up)

- [ ] Задача 1: Прочитать HTTP `ClientAuthMiddleware` полностью — context key, error mapping, bypass условия.
- [ ] Задача 2: Реализовать ValidateToken-based interceptor в `internal/gateway/client/grpc/interceptor.go`.
- [ ] Задача 3: То же для `internal/api/grpc/interceptors.go` (cmd/api legacy — см. R1).
- [ ] Задача 4: Unit-тест: вызов без token → `codes.Unauthenticated`.
- [ ] Задача 5: Unit-тест: валидный token → ctx содержит real client_id.
- [ ] Задача 6: Unit-тест: invalid token → `codes.Unauthenticated`.
- [ ] Задача 7: Integration-тест: два клиента с разными ключами видят только свои данные.
- [ ] Задача 8: Security-review: каждый RPC handler читает client_id из ctx, не из request.

## Risks

- `cmd/api` помечен legacy (см. R1 Phase 0 finding), но пока он не retired — чинить оба interceptor'а. Если retirement случится раньше — только client-gateway.
- Рefactor handler'ов чтобы читать из ctx вместо req может задеть 7+ RPC — значительный объём.
- Load-testing сценарии могут сломаться если ранее полагались на auto-tenant — проверить с owner'ом load-test setup'а.

## Dependencies

- HTTP auth middleware как источник истины (context key, signature).
- Phase 0 R1 решение: retired `cmd/api` или нет.
- Координация с B2 fix (SMPP auth_adapter password) — оба касаются auth, но независимы.

## Estimated size

Крупное. Предположительно ~3-5 файлов, плюс тесты. Auth hard-gate per ε+hard-gate.

## Priority rationale

Critical, но не самый срочный в прод:
- HTTP путь работает корректно (multi-tenant изоляция на нём).
- Phase 0 не нашёл клиентов, которые используют gRPC external (messagingv1 — единственный зарегистрирован, 2 из 7 RPC — Unimplemented).
- Если реальный траффик на gRPC порту = 0, критичность в проде низкая, но поверхность присутствует и один случайный клиент откроет дыру.

Рекомендуется запустить после A6-auth fix, но до широкого анонса gRPC API.
