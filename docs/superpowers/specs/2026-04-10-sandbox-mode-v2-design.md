# Sandbox Mode v2 — Design Spec

**Date:** 2026-04-10  
**Status:** Approved  
**Scope:** Реалистичная sandbox-среда с фейковыми DLR, улучшенным UI и тест-формой в портале

---

## Контекст

### Текущее состояние

Sandbox-режим существует как булевый флаг `clients.is_sandbox`. При `is_sandbox=true` в `message_service.go` срабатывает bypass-ветка: сообщение сразу помечается `DELIVERED`, в Kafka ничего не публикуется, DLR не симулируются, задержки нет.

В UI — жёлтый баннер на Dashboard с кнопкой «Перейти в Production».

**Проблемы:**
- Мгновенная доставка не отражает реальное поведение (DLR через 2–30 сек)
- Нет симуляции неудачных статусов (UNDELIV, EXPIRED)
- Нет инструмента для быстрой отправки тестового SMS из портала
- Нет примеров запросов для разработчиков

### Что уже есть

- `internal/smsc/stub_sender.go` — StubSender с поддержкой случайных задержек, DLR-статусов через Kafka, `DLRSuccessRate`
- `IsSimulator(provider)` — проверяет `provider.SystemType == "SIMULATOR"`, используется в пуле
- `sender_factory.go` — роутит по `system_type` на StubSender для SIMULATOR провайдеров
- Таблица `providers` — хранит SMPP провайдеры с полем `system_type`

---

## Дизайн

### Подход: Вариант B — SANDBOX через StubSender

Вместо мгновенного bypass sandbox-сообщения маршрутизируются через виртуальный провайдер `SANDBOX` с `system_type = 'SANDBOX'`, обслуживаемый `StubSender` с заданной конфигурацией. Остальной messaging-пайплайн работает штатно.

**DLR-параметры:**
- Задержка: случайная 2–30 секунд
- DELIVRD: 80%
- UNDELIV: 15%  
- EXPIRED: 5%

---

## Backend

### 1. Миграция: провайдер SANDBOX

Новая миграция создаёт запись-заглушку в таблице `providers`:

```sql
-- migrations/000083_add_sandbox_provider.up.sql
INSERT INTO providers (id, name, system_type, host, port, system_id, password, bind_type, active, priority, throughput_per_second)
VALUES (
  '00000000-0000-0000-0000-000000000001',
  'SANDBOX',
  'SANDBOX',
  'localhost', 0, 'sandbox', 'sandbox',
  'transceiver',
  true, 0, 1000
);
```

Провайдер не используется для реальных подключений — `StubSender` не открывает TCP-соединение.

### 2. `internal/smsc/sender_factory.go`

Добавить ветку для `system_type == "SANDBOX"`:

```go
case provider.SystemType == "SANDBOX":
    return NewStubSender(StubSenderConfig{
        MinDelayMs:     2000,
        MaxDelayMs:     30000,
        FailureRatePct: 0,       // failure через DLR, не через send error
        DLRDelayMs:     0,       // DLR задержка уже в MinDelayMs/MaxDelayMs
        DLRSuccessRate: 80,
        DLRStatuses:    []string{"UNDELIV", "UNDELIV", "UNDELIV", "EXPIRED"}, // 15% + 5% из 20% неуспешных
    }, kafkaProducer, dlrTopic)
```

Аналогично существующей ветке `IsSimulator()`.

### 3. `internal/smsc/pool.go`

Добавить функцию:

```go
func IsSandbox(provider *shared.Provider) bool {
    return provider.SystemType == "SANDBOX"
}
```

Использовать в пуле аналогично `IsSimulator` — не открывать реальное SMPP-соединение.

### 4. `internal/services/messaging/application/message_service.go`

Sandbox-ветка (lines ~125-136) переписывается:

**Было:**
```go
if options != nil && options.IsSandbox {
    msg.MarkAsDelivered()
    if err := s.messageRepo.Create(ctx, msg); err != nil { ... }
    return msg, nil
}
```

**Станет:**
```go
if options != nil && options.IsSandbox {
    sandboxProvider, err := s.providerRepo.GetBySystemType(ctx, "SANDBOX")
    if err != nil {
        return nil, fmt.Errorf("sandbox provider not found: %w", err)
    }
    msg.SetProvider(sandboxProvider.ID)
    // Далее обычный путь: сохранить + публиковать в Kafka
}
```

После этой ветки обычный код сохранения + Kafka-публикации выполняется без изменений.

### 5. Тарификация — нулевой биллинг для SANDBOX

В `tarification_service.go` добавить проверку перед вызовом `s.saga.Charge()`:

```go
if msg.ProviderSystemType == "SANDBOX" {
    // Записать тарификационную запись с amount=0, пропустить Charge()
    return &TarificationResult{Amount: "0.00", Charged: false}, nil
}
```

Альтернатива: создать тарифный план с `price_per_segment = 0` и назначать его sandbox-сообщениям — выбор реализации на усмотрение разработчика. Главное: баланс клиента не списывается.

---

## Frontend

### 6. `portal-frontend/src/pages/dashboard/DashboardPage.tsx`

Заменить жёлтый баннер на фиолетовый с расширенным текстом:

**Было:** amber background, текст «SMS не отправляются реально», кнопка «Перейти в Production»

**Станет:**
```tsx
<div className="sandbox-banner"> {/* violet/purple */}
  🧪 <strong>Sandbox Mode</strong> — SMS не отправляются реально. 
  Фейковый DLR придёт через 2–30 сек со статусом DELIVRD / UNDELIV / EXPIRED.
  &nbsp;<Link to="/sandbox">Открыть Sandbox →</Link>
  
  <Button onClick={handleDisableSandbox}>Перейти в Production</Button>
</div>
```

### 7. Новая страница `portal-frontend/src/pages/sandbox/SandboxPage.tsx`

**Роутинг:** `/sandbox` в `App.tsx`, защищена guard-ом — если `!profile.is_sandbox` редиректить на `/dashboard`.

**Компоновка:** два столбца

**Левый столбец — форма отправки:**
- Поле «Получатель» (номер телефона)
- Поле «Отправитель» (sender name)
- Textarea «Текст»
- Кнопка «Отправить»
- После отправки: блок с `message_id` и текстом «DLR ожидается через ~2–30 сек»

**Правый столбец — документация:**
- Tabs: `curl` / `HTTP` / `Python` — примеры запросов к API с текущим API-ключом клиента (или placeholder `YOUR_API_KEY`)
- Таблица DLR-статусов:

| Статус   | Вероятность | Описание            |
|----------|-------------|---------------------|
| DELIVRD  | 80%         | Успешная доставка   |
| UNDELIV  | 15%         | Недоставлено        |
| EXPIRED  | 5%          | Истёк срок          |

**Навигация:** страница НЕ добавляется в сайдбар. Доступна только через ссылку «Открыть Sandbox →» в баннере на Dashboard.

### 8. Роутинг

В файле роутинга портала добавить:
```tsx
<Route path="/sandbox" element={<SandboxPage />} />
```

---

## Что НЕ входит в скоуп

- Конфигурируемые DLR-параметры через UI (вероятности фиксированы в коде)
- Лимиты на количество sandbox-сообщений
- Sandbox-режим не в портале (API-клиенты — поведение идентично, просто DLR задерживается)
- Отдельный пункт в сайдбаре для Sandbox

---

## Тестирование

- Unit-тест `message_service_test.go`: sandbox-сообщение публикуется в Kafka с провайдером SANDBOX (не immediate deliver)
- Unit-тест `sender_factory_test.go`: `system_type=SANDBOX` возвращает StubSender с правильными параметрами
- Integration: отправить sandbox SMS → проверить статус `ACCEPTED` → дождаться DLR → статус изменился на DELIVRD/UNDELIV/EXPIRED
- Frontend: SandboxPage не доступна без sandbox-режима (редирект)

---

## Файлы к изменению

| Файл | Изменение |
|------|-----------|
| `migrations/000083_add_sandbox_provider.up.sql` | Новая миграция: INSERT провайдера SANDBOX |
| `internal/smsc/pool.go` | Добавить `IsSandbox()`, не открывать соединение |
| `internal/smsc/sender_factory.go` | Ветка для SANDBOX → StubSender с config |
| `internal/services/messaging/application/message_service.go` | Sandbox-ветка: выбор провайдера SANDBOX вместо immediate deliver |
| `internal/services/tarification/application/tarification_service.go` | Skip billing для SANDBOX провайдера |
| `portal-frontend/src/pages/dashboard/DashboardPage.tsx` | Улучшенный баннер с ссылкой на /sandbox |
| `portal-frontend/src/pages/sandbox/SandboxPage.tsx` | Новый компонент: форма + документация |
| `portal-frontend/src/App.tsx` | Route `/sandbox` → SandboxPage |
