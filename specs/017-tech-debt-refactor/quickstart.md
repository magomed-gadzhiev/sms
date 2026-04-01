# Quickstart: Technical Debt Resolution

**Branch**: `017-tech-debt-refactor` | **Date**: 2026-04-01

## Что делается

Устранение 7 категорий технического долга без изменения внешнего поведения системы:

| # | Категория | Приоритет | Затронутые файлы |
|---|-----------|-----------|-----------------|
| 1 | Дублирование client_id парсинга в gRPC | P1 | `messaging/grpc/server.go`, `client/grpc/server.go` |
| 2 | N+1 запросы в LeastLoadedSelector | P1 | `routing/application/selection_strategy.go`, `routing/domain/repository.go` |
| 3 | Утечка памяти в RoundRobinSelector | P1 | `routing/application/selection_strategy.go` |
| 4 | Магические DLR-строки | P2 | `messaging/application/dlr_service.go` + новый `shared/dlr/status.go` |
| 5 | Заглушка SMPPConnectionAdapter | P2 | `provider/infrastructure/smpp/pool_adapter.go` |
| 6 | Тихие ошибки (Warn→Error) | P2 | `messaging/application/dlr_service.go` |
| 7 | Frontend useMemo/useCallback | P3 | 10 компонентов в `portal-frontend/src/pages/` |

## Проверка после реализации

### Backend

```bash
# 1. Компиляция всего проекта
cd /home/magomed/projects/sms
go build ./...

# 2. Юнит-тесты
go test ./internal/services/routing/... ./internal/services/messaging/... ./internal/shared/...

# 3. Проверка что магических строк DLR не осталось в бизнес-логике
grep -rn '"DELIVRD"\|"EXPIRED"\|"REJECTD"\|"UNDELIV"' internal/ --include="*.go" | grep -v "shared/dlr/status.go"
# Ожидается: нет вывода

# 4. Проверка N+1 — убедиться что GetHealth в цикле заменён на GetHealthBatch
grep -n "GetHealth(ctx" internal/services/routing/application/selection_strategy.go
# Ожидается: нет вывода (только GetHealthBatch)
```

### Frontend

```bash
cd portal-frontend

# TypeScript компиляция
npm run build

# Проверка что useMemo используется для columns
grep -rn "const columns" src/pages/ --include="*.tsx"
# Все должны быть внутри useMemo(
```

## Ключевые архитектурные решения

1. **client_id парсинг**: helper-функция `parseClientID(raw string) (uuid.UUID, error)` в каждом grpc-пакете (не interceptor — т.к. client_id в теле запроса, не в metadata)

2. **Batch health-check**: новый метод `GetHealthBatch(ctx, ids) map[uuid.UUID]*ProviderHealth` в `ProviderRepository` с SQL `WHERE id = ANY($1)`

3. **Round-robin cleanup**: `PurgeStaleRoutes(activeRouteIDs []uuid.UUID)` вызывается в `RoutingService` после загрузки актуального списка маршрутов

4. **DLR константы**: новый пакет `internal/shared/dlr/status.go` — в `shared/` т.к. DLR может обрабатываться в нескольких сервисах

5. **SMPPConnectionAdapter**: удалить `SendMessage` из интерфейса `application.Connection` (после аудита вызовов) или добавить `// Deprecated` комментарий если интерфейс внешний

## Риски

- **ConnectionAdapter audit**: перед удалением `SendMessage` из интерфейса — обязательно `grep -r "\.SendMessage(" internal/services/provider/` чтобы убедиться что нет runtime вызовов
- **GetHealthBatch**: если `provider_health` — не отдельная таблица а вычисляется — потребуется другой SQL паттерн (см. `routing/infrastructure/postgres/provider_repo.go`)
