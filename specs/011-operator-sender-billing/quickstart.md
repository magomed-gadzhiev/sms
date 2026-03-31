# Quickstart: Operator Sender Name Billing

**Feature**: 011-operator-sender-billing

## Обзор

Фича добавляет ежемесячную тарификацию платных имён отправителей:

1. У оператора появляется `monthly_tariff_amount` (тариф в RUB)
2. При регистрации платного имени клиент видит стоимость за текущий месяц
3. В начале каждого месяца планировщик автоматически начисляет плату за все активные платные имена
4. Начисления идемпотентны — повторный запуск безопасен

## Затронутые сервисы

| Сервис | Изменения |
|--------|-----------|
| `routing-service` | Добавить `monthly_tariff_amount` в домен/репо/gRPC |
| `tarification-service` | Новый домен + репо + сервис + планировщик |
| `admin-gateway` | Обновить operator endpoint |
| `portal-gateway` | Добавить tariff endpoint + billing history |
| Frontend (admin) | Форма оператора с тарифом |
| Frontend (portal) | Inline стоимость + история начислений |

## Порядок реализации (Tasks → Implement)

```
1. DB Migrations (000065, 000066)
2. routing-service: обновить proto → домен → репо → gRPC
3. tarification-service: создать billing domain → репо → сервис → планировщик → gRPC
4. admin-gateway: обновить operator handlers
5. portal-gateway: добавить tariff endpoint + billing history
6. Frontend: admin operator form
7. Frontend: portal sender name form (inline cost) + billing history
8. Tests: unit + integration
```

## Запуск после реализации

```bash
# Применить миграции
scripts/server.sh migrate

# Пересобрать затронутые сервисы
scripts/server.sh deploy routing-service
scripts/server.sh deploy tarification-service
scripts/server.sh deploy admin-gateway
scripts/server.sh deploy portal-gateway
```

## Проверка cron-планировщика вручную

Планировщик запускается в первый день месяца в 00:01. Для ручной проверки:
- Смотреть логи tarification-service: `scripts/server.sh logs tarification-service`
- В логах искать: `"billing scheduler run"` и `"billing records created"`

## Ключевые файлы

| Файл | Назначение |
|------|-----------|
| `migrations/000065_operator_tariff.up.sql` | Добавить тариф в operators |
| `migrations/000066_sender_name_billing.up.sql` | Создать таблицу billing records |
| `internal/services/tarification/domain/sender_billing.go` | Домен billing record |
| `internal/services/tarification/application/billing_scheduler.go` | Cron-планировщик |
| `api/proto/routing/routing.proto` | Обновить Operator messages |
| `api/proto/tarification/tarification.proto` | Добавить billing RPCs |

## Edge Cases

| Сценарий | Поведение |
|----------|-----------|
| Регистрация в последний день месяца | Полная плата за месяц (без пропорций) |
| Повторный запуск планировщика в том же месяце | INSERT ON CONFLICT DO NOTHING — пропускается |
| Деактивированное имя при наступлении нового месяца | Не включается в запрос (status != 'active') |
| Изменение тарифа в середине месяца | Новый тариф применяется в следующем месяце |
