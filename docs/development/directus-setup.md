# Руководство по настройке коллекций Directus

## Обзор

Это руководство описывает процесс настройки коллекций Directus для работы с существующими таблицами PostgreSQL. После настройки вы сможете использовать Directus для управления административными сущностями через удобный веб-интерфейс.

## Автоматическая настройка

### Использование скрипта

Самый простой способ - использовать готовый скрипт настройки:

**Linux/Mac:**
```bash
./scripts/setup-directus.sh
```

**Windows PowerShell:**
```powershell
.\scripts\setup-directus.ps1
```

**Docker:**
```bash
docker-compose -f deployments/docker-compose.yml exec dev node scripts/setup-directus.js
```

Скрипт автоматически настроит:
- Relationships между коллекциями
- Защиту чувствительных полей (скрытие/readonly)
- Интерфейсы для полей (dropdowns, JSON редакторы, теги)
- Метаданные коллекций

### Использование SQL миграции

Альтернативный способ - выполнить SQL миграцию напрямую:

```bash
# Убедитесь, что Directus запущен и таблицы созданы
docker-compose -f deployments/docker-compose.yml exec postgres psql -U smpp -d smpp_db -f /path/to/migrations/000007_setup_directus_collections.up.sql
```

**Примечание:** Миграция должна выполняться ПОСЛЕ того, как Directus уже запущен и автоматически создал базовые коллекции.

## Ручная настройка через UI

### Доступ к настройкам

1. Войдите в Directus: http://localhost:8055
2. Перейдите в **Settings** → **Data Model**
3. Выберите коллекцию из списка

### Настройка полей

Для каждого поля можно настроить:

1. **Interface** - тип интерфейса (input, dropdown, JSON и т.д.)
2. **Readonly** - только для чтения
3. **Hidden** - скрыть поле
4. **Required** - обязательное поле
5. **Default Value** - значение по умолчанию
6. **Validation** - правила валидации
7. **Display** - способ отображения

## Настройка коллекций

### users

**Назначение:** Пользователи системы

**Защищенные поля:**
- `password_hash` - **скрыто** (hidden + readonly)
  - Не отображается в интерфейсе
  - Устанавливается только через Auth Service

**Настройки:**
- `role_id` - relationship с `roles` (Many-to-One)
  - Interface: `select-dropdown-m2o`
  - Template: `{{name}}`
- `username` - уникальное поле
- `email` - уникальное поле, валидация email

**Permissions:**
- Administrator: полный доступ
- Operator: только чтение
- Viewer: только чтение

### roles

**Назначение:** Роли пользователей

**Настройки:**
- `name` - уникальное поле, обязательное
- `description` - многострочный текст

**Permissions:**
- Administrator: полный доступ
- Остальные роли: только чтение

### clients

**Назначение:** Клиенты API

**Защищенные поля:**
- `secret` - **скрыто** (hidden + readonly)
- `api_key` - **скрыто** (hidden + readonly)

**Настройки:**
- `name` - обязательное поле
- `allowed_source_addresses` - теги (массив строк)
  - Interface: `tags`
- `metadata` - JSON объект
  - Interface: `input-code`
  - Format: JSON

**Permissions:**
- Administrator: полный доступ
- Operator: только чтение

### providers

**Назначение:** SMSC провайдеры

**Защищенные поля:**
- `password` - **скрыто** (hidden + readonly)
- `system_id` - только для чтения (readonly, но не скрыто)

**Настройки:**
- `name` - обязательное поле
- `host` - обязательное поле
- `port` - число
- `username` - обязательное поле
- `bind_type` - dropdown
  - Choices: Transceiver, Transmitter, Receiver
- `is_active` - булево значение

**Permissions:**
- Administrator: полный доступ
- Operator: только чтение

### routes

**Назначение:** Правила маршрутизации сообщений

**Relationships:**
- `provider_id` → `providers` (Many-to-One)
  - Interface: `select-dropdown-m2o`
  - Template: `{{name}}`
  - On Delete: RESTRICT
- `failover_provider_id` → `providers` (Many-to-One, опционально)
  - Interface: `select-dropdown-m2o`
  - Allow None: true
  - On Delete: SET NULL

**Настройки:**
- `pattern` - обязательное поле
- `pattern_type` - dropdown
  - Choices: Prefix, Regex, Exact
- `priority` - число (меньше = выше приоритет)
- `is_active` - булево значение

**Permissions:**
- Administrator: полный доступ
- Operator: только чтение

### accounts

**Назначение:** Счета клиентов для биллинга

**Relationships:**
- `client_id` → `clients` (Many-to-One)
  - Interface: `select-dropdown-m2o`
  - Template: `{{name}}`
  - On Delete: CASCADE
  - Обязательное поле

**Настройки:**
- `balance` - только для чтения (readonly)
  - Interface: `input`
  - Format: число с 6 знаками после запятой
  - Изменяется только через транзакции
- `currency` - dropdown
  - Choices: RUB

**Permissions:**
- Administrator: полный доступ
- Operator: только чтение

### transactions

**Назначение:** Транзакции биллинга

**Relationships:**
- `client_id` → `clients` (Many-to-One)
  - Interface: `select-dropdown-m2o`
  - Template: `{{name}}`
  - On Delete: CASCADE

**Настройки:**
- `type` - dropdown
  - Choices: Charge, Credit, Refund, Adjustment
- `amount` - число с 6 знаками после запятой
- `balance_before` - только для чтения (readonly)
- `balance_after` - только для чтения (readonly)
- `metadata` - JSON объект (если существует)
  - Interface: `input-code`

**Permissions:**
- Administrator: полный доступ
- Operator: только чтение

### pricing_rules

**Назначение:** Правила тарификации

**Relationships:**
- `client_id` → `clients` (Many-to-One, опционально)
  - Interface: `select-dropdown-m2o`
  - Template: `{{name}}`
  - Allow None: true
  - On Delete: CASCADE

**Настройки:**
- `price_per_message` - обязательное поле
  - Format: число с 6 знаками после запятой
- `currency` - dropdown
  - Choices: RUB
- `priority` - число (меньше = выше приоритет)

**Permissions:**
- Administrator: полный доступ
- Operator: только чтение

### api_keys

**Назначение:** API ключи пользователей

**Защищенные поля:**
- `key_hash` - **скрыто** (hidden + readonly)

**Relationships:**
- `user_id` → `users` (Many-to-One)
  - Interface: `select-dropdown-m2o`
  - Template: `{{username}} ({{email}})`

**Настройки:**
- `key_prefix` - только для чтения (readonly)
  - Interface: `input`
  - Font: monospace
- `name` - название ключа
- `expires_at` - дата истечения (опционально)

**Permissions:**
- Administrator: полный доступ
- Остальные роли: только чтение своих ключей

### refresh_tokens

**Назначение:** Токены обновления для аутентификации

**Защищенные поля:**
- `token_hash` - **скрыто** (hidden + readonly)

**Relationships:**
- `user_id` → `users` (Many-to-One)
  - Interface: `select-dropdown-m2o`
  - Template: `{{username}}`

**Настройки:**
- `expires_at` - дата истечения
- `is_revoked` - булево значение

**Permissions:**
- Administrator: полный доступ
- Остальные роли: доступ запрещен

## Настройка Relationships

### Many-to-One (M2O)

Используется когда у записи есть связь с другой коллекцией через foreign key.

**Пример:** `routes.provider_id` → `providers.id`

**Настройка через UI:**
1. Откройте коллекцию `routes`
2. Выберите поле `provider_id`
3. В разделе "Relationships" создайте:
   - Many Collection: `routes`
   - Many Field: `provider_id`
   - One Collection: `providers`
   - On Delete: RESTRICT (или SET NULL для опциональных)

**Настройка интерфейса:**
- Interface: `select-dropdown-m2o`
- Template: `{{name}}` (отображение связанной записи)
- Allow None: true/false (опциональное поле)

### One-to-Many (O2M)

Автоматически создается при настройке Many-to-One.

**Пример:** Автоматически создается поле `routes` в коллекции `providers`

**Настройка:**
- Отображает все связанные записи
- Interface: `list-m2o` (по умолчанию)

## Настройка Presets (Представления)

Presets позволяют создать предустановленные фильтры и сортировки для коллекций.

### Создание Preset

1. Откройте коллекцию (например, `clients`)
2. Примените нужные фильтры и сортировку
3. Нажмите на иконку "Bookmark" (сохранить как preset)
4. Введите название (например, "Активные клиенты")

### Примеры Presets

**Активные клиенты:**
- Collection: `clients`
- Filter: `is_active = true`
- Sort: `created_at DESC`

**Последние транзакции:**
- Collection: `transactions`
- Filter: нет
- Sort: `created_at DESC`
- Limit: 50

**Активные провайдеры:**
- Collection: `providers`
- Filter: `is_active = true`
- Sort: `name ASC`

## Кастомные интерфейсы

### JSON поля

Для полей типа JSONB используйте интерфейс `input-code`:

1. Выберите поле (например, `metadata`)
2. Interface: `input-code`
3. Format: JSON
4. Options: настроить подсветку синтаксиса

### Теги (Tags)

Для массивов строк используйте интерфейс `tags`:

1. Выберите поле (например, `allowed_source_addresses`)
2. Interface: `tags`
3. Options: настройте разделители при необходимости

### Dropdown

Для полей с ограниченным набором значений:

1. Выберите поле (например, `bind_type`)
2. Interface: `select-dropdown`
3. Options → Choices: добавьте варианты
   ```
   Text: Transceiver, Value: transceiver
   Text: Transmitter, Value: transmitter
   Text: Receiver, Value: receiver
   ```

## Защита чувствительных данных

### Скрытие полей

Для полей, содержащих секреты (пароли, ключи, хеши):

1. Выберите поле
2. Включите "Hidden" - поле не будет отображаться в интерфейсе
3. Включите "Readonly" - поле нельзя редактировать (если видимо)

### Только для чтения

Для полей, которые должны отображаться, но не редактироваться:

1. Выберите поле
2. Включите "Readonly" - можно просматривать, но нельзя изменять

**Примеры:**
- `accounts.balance` - изменяется только через транзакции
- `providers.system_id` - генерируется системой
- `api_keys.key_prefix` - только для отображения

## Настройка Permissions

### Создание роли

1. Перейдите в **Settings** → **Roles & Permissions**
2. Нажмите "Create Role"
3. Введите название (например, "Operator")
4. Нажмите "Save"

### Настройка Permissions для роли

1. Выберите роль
2. Для каждой коллекции настройте:
   - **Read Access:** кто может читать
   - **Create Access:** кто может создавать
   - **Update Access:** кто может обновлять
   - **Delete Access:** кто может удалять

### Рекомендуемые Permissions

**Administrator:**
- Все коллекции: Full Access

**Operator:**
- `users`: Read Access (All Users)
- `clients`: Read Access (All Items)
- `providers`: Read Access (All Items)
- `routes`: Read Access (All Items)
- `accounts`: Read Access (All Items)
- `transactions`: Read Access (All Items)
- `pricing_rules`: Read Access (All Items)

**Viewer:**
- Все коллекции: Read Access (All Items)
- Остальные: No Access

## Проверка настройки

После настройки проверьте:

1. **Коллекции отображаются:**
   - Settings → Data Model → все коллекции видны

2. **Relationships работают:**
   - Откройте `routes` → поле `provider_id` показывает dropdown с провайдерами

3. **Защищенные поля скрыты:**
   - Откройте `clients` → поля `secret` и `api_key` не видны

4. **Permissions работают:**
   - Создайте тестового пользователя с ролью Operator
   - Войдите под ним → проверьте доступ

## Troubleshooting

### Коллекции не отображаются

**Причина:** Directus еще не обнаружил таблицы PostgreSQL

**Решение:**
1. Убедитесь, что PostgreSQL запущен
2. Проверьте, что таблицы существуют в БД
3. Перезапустите Directus:
   ```bash
   docker-compose -f deployments/docker-compose.yml restart directus
   ```

### Relationships не работают

**Причина:** Foreign keys не настроены или неправильно настроены

**Решение:**
1. Проверьте foreign keys в PostgreSQL:
   ```sql
   SELECT * FROM information_schema.table_constraints 
   WHERE constraint_type = 'FOREIGN KEY';
   ```
2. Настройте relationship вручную через UI
3. Запустите скрипт настройки заново

### Поля не скрываются

**Причина:** Настройки полей не применены

**Решение:**
1. Проверьте настройки поля через UI
2. Выполните SQL миграцию:
   ```bash
   docker-compose -f deployments/docker-compose.yml exec postgres psql -U smpp -d smpp_db -f /path/to/migrations/000007_setup_directus_collections.up.sql
   ```

### Permissions не применяются

**Причина:** Роль не назначена пользователю или permissions не настроены правильно

**Решение:**
1. Проверьте, что роль назначена пользователю
2. Проверьте настройки permissions для роли
3. Выйдите и войдите заново под пользователем

## Дополнительные ресурсы

- [Официальная документация Directus](https://docs.directus.io/)
- [Data Model Guide](https://docs.directus.io/configuration/data-model/)
- [Permissions Guide](https://docs.directus.io/configuration/users-roles-permissions/)
- [Relationships Guide](https://docs.directus.io/configuration/data-model/relationships/)
- [Развертывание Directus](../deployment/directus.md)