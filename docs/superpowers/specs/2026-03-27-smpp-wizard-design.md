# SMPP Wizard — Пошаговый UI подключения провайдера

**Дата:** 2026-03-27
**Статус:** Approved
**Область:** Portal (клиент-facing), provider-service, portal-gateway

---

## 1. Цель

Клиент (SMS-агрегатор с прямым договором с оператором) может самостоятельно подключить своего SMPP-провайдера через портал без участия admin. Wizard проводит через 6 шагов с live-тестом соединения.

---

## 2. Архитектура

### 2.1 Новые маршруты (frontend)

| Путь | Компонент | Описание |
|------|-----------|----------|
| `/providers` | `ProvidersPage` | Список провайдеров клиента + кнопка "Подключить" |
| `/providers/new?step=N` | `ProviderWizardPage` | 6-шаговый wizard |

### 2.2 Новые HTTP-эндпоинты (portal-gateway)

| Метод | Путь | Действие |
|-------|------|----------|
| GET | `/portal/v1/providers` | Список провайдеров клиента |
| POST | `/portal/v1/providers` | Создать провайдера |
| GET | `/portal/v1/providers/:id` | Получить провайдера |
| PUT | `/portal/v1/providers/:id` | Обновить провайдера |
| DELETE | `/portal/v1/providers/:id` | Удалить провайдера |
| POST | `/portal/v1/providers/test` | Тест соединения (без сохранения) |

### 2.3 Новые gRPC-методы (provider-service)

```protobuf
rpc CreateClientProvider(CreateClientProviderRequest) returns (Provider);
rpc ListClientProviders(ListClientProvidersRequest) returns (ListClientProvidersResponse);
rpc GetClientProvider(GetClientProviderRequest) returns (Provider);
rpc UpdateClientProvider(UpdateClientProviderRequest) returns (Provider);
rpc DeleteClientProvider(DeleteClientProviderRequest) returns (google.protobuf.Empty);
rpc TestClientProviderConnection(TestConnectionRequest) returns (TestConnectionResponse);

message TestConnectionResponse {
  bool success = 1;
  int64 latency_ms = 2;
  repeated string log = 3;
  string error = 4;
}
```

### 2.4 DB Migration

```sql
-- migrations/000011_client_providers.up.sql
ALTER TABLE providers ADD COLUMN client_id UUID REFERENCES clients(id);
CREATE INDEX idx_providers_client_id ON providers(client_id);
-- Существующие admin-провайдеры: client_id = NULL
```

---

## 3. Frontend

### 3.1 Структура файлов

```
portal-frontend/src/pages/providers/
├── ProvidersPage.tsx
├── ProviderWizardPage.tsx
├── steps/
│   ├── Step1BasicInfo.tsx      # Название, описание, теги
│   ├── Step2Connection.tsx     # Host, port, system_id, password, bind_type
│   ├── Step3Params.tsx         # Window size, max_connections, TPS
│   ├── Step4Test.tsx           # Live SMPP bind-тест + лог
│   ├── Step5Routing.tsx        # Префиксы/паттерны, приоритет
│   └── Step6Summary.tsx        # Обзор + кнопка "Активировать"
└── components/
    ├── WizardProgress.tsx      # Прогресс-бар (шаги 1–6)
    └── WizardNav.tsx           # Кнопки Назад/Далее + валидация
```

### 3.2 State management

- `useReducer` в `ProviderWizardPage` — хранит всё состояние wizard
- Текущий шаг — `?step=N` в URL (поддержка refresh и browser history)
- Черновик не сохраняется на сервере между шагами — только финальный `POST /providers`
- Шаг 4 делает отдельный `POST /providers/test` без сохранения

### 3.3 Роутинг (App.tsx)

```tsx
// Добавить в protected routes:
<Route path="/providers" element={<ProvidersPage />} />
<Route path="/providers/new" element={<ProviderWizardPage />} />
```

### 3.4 API (client.ts — новый providersApi)

```ts
providersApi = {
  list():           GET    /portal/v1/providers
  create(data):     POST   /portal/v1/providers
  get(id):          GET    /portal/v1/providers/:id
  update(id, data): PUT    /portal/v1/providers/:id
  delete(id):       DELETE /portal/v1/providers/:id
  test(config):     POST   /portal/v1/providers/test
  // response: { success: bool, latency_ms: number, log: string[], error?: string }
}
```

---

## 4. Шаги Wizard

| Шаг | Поля | Валидация |
|-----|------|-----------|
| 1 — Основное | name (required), description, tags (string[], optional) | name непустое |
| 2 — Подключение | host (required), port (required), system_id (required), password (required), bind_type: TX/RX/TRX | все поля required |
| 3 — Параметры | window_size (1–1000, default 10), max_connections (1–10, default 1), tps_limit (1–1000) | числа в диапазоне |
| 4 — Тест | кнопка "Проверить", лог SMPP handshake, латентность | тест не блокирует переход |
| 5 — Маршруты | routing rules: pattern (E.164 префикс или regex), priority (число) | минимум 0 правил (провайдер inactive) |
| 6 — Итог | read-only обзор всех настроек, кнопка "Активировать" | — |

---

## 5. Ошибки и крайние случаи

### Шаг 4 (тест соединения)
- Таймаут 10s → `{ success: false, error: "connection timeout" }`
- SMPP `ESME_RBINDFAIL` → UI показывает "Неверный system_id или пароль"
- TCP error → лог показывает, на каком шаге упало соединение
- Тест не прошёл → можно продолжить с предупреждением: _"Провайдер будет создан со статусом inactive"_

### Навигация
- "Назад" всегда доступен, данные не теряются (в `useReducer`)
- "Далее" требует валидации текущего шага
- Navigate away → `window.confirm("Прогресс будет потерян")`

### Безопасность и изоляция
- `client_id` берётся исключительно из session-контекста, никогда из тела запроса
- PUT/DELETE проверяют `provider.client_id == session.client_id` → 403 при несовпадении
- Лимит провайдеров по тарифному плану: Free/Starter — 1, Business — 5, Pro — unlimited
- Превышение лимита → 402 с сообщением о необходимости апгрейда плана

### Шаг 5 (routing)
- Если routing rules не добавлены — провайдер создаётся со статусом `inactive` (трафик не идёт)
- Pattern — E.164 префикс (например `+7`) или regex
- Priority — целое число (меньше = выше приоритет)

---

## 6. Backend — provider-service

### 6.1 Новые use cases
- `CreateClientProvider(ctx, clientID, config)` — создаёт провайдера с `client_id`, валидирует лимит по плану
- `ListClientProviders(ctx, clientID)` — список провайдеров клиента
- `TestConnection(ctx, config)` — SMPP dial+bind+unbind с `context.WithTimeout(10s)`, возвращает лог-строки

### 6.2 Repository
- Новые методы с WHERE `client_id = $1`
- `CountByClientID(clientID)` — для проверки лимита

### 6.3 Portal-gateway handler
- Новый файл: `internal/gateway/portal/handlers/providers.go`
- Регистрируется в `internal/gateway/portal/router.go`
- Использует существующий `providerClient` gRPC

---

## 7. Тестирование

- Unit: валидация каждого шага wizard (Step1–Step6)
- Integration: `POST /portal/v1/providers` → проверка `client_id` изоляции
- Integration: `POST /portal/v1/providers/test` → mock SMPP server (таймаут, bind fail, success)
- E2E (ручное): полный wizard от шага 1 до "Активировать"
