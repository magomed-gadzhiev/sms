# Развертывание Directus админ-панели

## Обзор

Directus работает как административная панель поверх существующей PostgreSQL базы данных. Он предоставляет веб-интерфейс для управления административными сущностями, в то время как существующие Go сервисы продолжают обрабатывать бизнес-логику через gRPC/HTTP API.

## Архитектура

```
┌─────────────┐
│  Directus   │  Админ-панель (UI)
│   :8055     │  REST/GraphQL API
└──────┬──────┘
       │
       │ (read/write)
       │
┌──────▼──────────────────────┐
│   PostgreSQL                │
│   (smpp_db)                 │
│                             │
│   - users                   │
│   - roles                   │
│   - clients                 │
│   - providers               │
│   - routes                  │
│   - accounts                │
│   - transactions            │
│   - pricing_rules           │
└──────┬──────────────────────┘
       │
       │ (read/write через gRPC)
       │
┌──────▼──────┐
│ Go Services │  Бизнес-логика
│ (Auth,      │  API Gateway
│  Client,    │  Worker
│  Billing)   │
└─────────────┘
```

## Требования

- Docker Engine 20.10+
- Docker Compose 2.0+
- PostgreSQL база данных (уже настроена в docker-compose)
- Минимум 512MB RAM для Directus контейнера

## Развертывание

### Первый запуск

1. **Убедитесь, что PostgreSQL запущен**

```bash
docker-compose -f deployments/docker-compose.yml up -d postgres
```

Подождите, пока PostgreSQL будет готов (обычно 10-30 секунд).

2. **Запустите Directus**

```bash
docker-compose -f deployments/docker-compose.yml up -d directus
```

3. **Проверьте статус**

```bash
docker-compose -f deployments/docker-compose.yml ps directus
```

4. **Проверьте логи**

```bash
docker-compose -f deployments/docker-compose.yml logs -f directus
```

### Первый вход

После первого запуска Directus автоматически создаст административную учетную запись на основе переменных окружения:

1. Откройте браузер: http://localhost:8055
2. Войдите с учетными данными из `deployments/directus/.env`:
   - Email: значение `DIRECTUS_ADMIN_EMAIL`
   - Password: значение `DIRECTUS_ADMIN_PASSWORD`

### Настройка коллекций

После первого входа Directus автоматически обнаружит существующие таблицы PostgreSQL. Для полной настройки коллекций выполните скрипт:

**Linux/Mac:**
```bash
./scripts/setup-directus.sh
```

**Windows PowerShell:**
```powershell
.\scripts\setup-directus.ps1
```

**Или через Docker:**
```bash
docker-compose -f deployments/docker-compose.yml exec dev node scripts/setup-directus.js
```

Этот скрипт настроит:
- Relationships между коллекциями
- Защиту чувствительных полей
- Интерфейсы для полей (dropdowns, JSON, теги и т.д.)

## Конфигурация

### Переменные окружения

Основные переменные настраиваются в `deployments/directus/.env`:

```env
# База данных
DIRECTUS_DATABASE_CLIENT=pg
DIRECTUS_DATABASE_HOST=postgres
DIRECTUS_DATABASE_PORT=5432
DIRECTUS_DATABASE_DATABASE=smpp_db
DIRECTUS_DATABASE_USER=smpp
DIRECTUS_DATABASE_PASSWORD=change-me

# Безопасность
DIRECTUS_KEY=<секретный ключ для шифрования>
DIRECTUS_SECRET=<секретный ключ для JWT>

# Администратор
DIRECTUS_ADMIN_EMAIL=admin@example.com
DIRECTUS_ADMIN_PASSWORD=admin

# Общие настройки
DIRECTUS_PORT=8055
DIRECTUS_PUBLIC_URL=http://localhost:8055
```

**Важно:** В production окружении:
- Используйте сильные секретные ключи для `DIRECTUS_KEY` и `DIRECTUS_SECRET`
- Измените пароль администратора
- Настройте `DIRECTUS_PUBLIC_URL` на реальный URL
- Используйте HTTPS через reverse proxy

### Генерация секретных ключей

Для генерации безопасных ключей используйте:

```bash
# Генерация DIRECTUS_KEY (32 символа)
openssl rand -base64 32

# Генерация DIRECTUS_SECRET (32 символа)
openssl rand -base64 32
```

Или в PowerShell:

```powershell
# Генерация DIRECTUS_KEY
[Convert]::ToBase64String([System.Security.Cryptography.RandomNumberGenerator]::GetBytes(24))

# Генерация DIRECTUS_SECRET
[Convert]::ToBase64String([System.Security.Cryptography.RandomNumberGenerator]::GetBytes(24))
```

## Порты и доступ

- **Directus UI:** http://localhost:8055
- **Directus API:** http://localhost:8055
- **Health Check:** http://localhost:8055/server/health

## Volumes

Directus создает следующие volumes:

- `directus-uploads` - загруженные файлы
- `directus-extensions` - кастомные расширения

## Health Checks

Health check настроен в docker-compose и проверяет:
- Доступность HTTP API
- Подключение к базе данных

Проверка вручную:

```bash
curl http://localhost:8055/server/health
```

Ответ должен быть:
```json
{
  "status": "ok"
}
```

## Управление коллекциями

### Автоматическое обнаружение

Directus автоматически создает коллекции для всех существующих таблиц PostgreSQL. После первого запуска вы увидите коллекции:

- `users` - пользователи системы
- `roles` - роли пользователей
- `permissions` - права доступа
- `role_permissions` - связи ролей и прав
- `clients` - клиенты API
- `providers` - SMSC провайдеры
- `routes` - правила маршрутизации
- `accounts` - счета клиентов
- `transactions` - транзакции биллинга
- `pricing_rules` - правила тарификации
- `api_keys` - API ключи
- `refresh_tokens` - токены обновления

### Ручная настройка

Для ручной настройки коллекций используйте веб-интерфейс Directus:

1. Войдите в Directus
2. Перейдите в Settings → Data Model
3. Выберите коллекцию
4. Настройте поля, relationships, permissions

Подробнее см. [Руководство по настройке коллекций](../development/directus-setup.md)

## Права доступа

### Роли

По умолчанию Directus создает роли:
- `Administrator` - полный доступ
- `Authenticated` - для авторизованных пользователей
- `Public` - для публичного доступа

### Настройка permissions

Рекомендуемые настройки permissions:

**Administrator:**
- Полный доступ ко всем коллекциям (CRUD)

**Operator** (создайте вручную):
- `users`: Read
- `clients`: Read
- `providers`: Read
- `routes`: Read
- `accounts`: Read
- `transactions`: Read
- `pricing_rules`: Read

**Viewer** (создайте вручную):
- Read-only доступ ко всем коллекциям

## Защита чувствительных полей

Следующие поля настроены как скрытые или только для чтения:

- `users.password_hash` - скрыто
- `clients.secret` - скрыто
- `clients.api_key` - скрыто
- `providers.password` - скрыто
- `api_keys.key_hash` - скрыто
- `refresh_tokens.token_hash` - скрыто

Эти поля защищены через SQL миграцию `000007_setup_directus_collections.up.sql` или через скрипт настройки.

## Логи

### Просмотр логов

```bash
# Все логи
docker-compose -f deployments/docker-compose.yml logs -f directus

# Последние 100 строк
docker-compose -f deployments/docker-compose.yml logs --tail=100 directus
```

### Уровни логирования

Настройка через переменную окружения:
```env
DIRECTUS_LOGGER_LEVEL=info  # trace, debug, info, warn, error, fatal
```

## Обновление

### Обновление Directus

1. **Остановите сервис**
```bash
docker-compose -f deployments/docker-compose.yml stop directus
```

2. **Обновите образ**
```bash
docker-compose -f deployments/docker-compose.yml pull directus
```

3. **Запустите снова**
```bash
docker-compose -f deployments/docker-compose.yml up -d directus
```

### Резервное копирование

**Важно:** Делайте backup перед обновлением!

1. **Backup базы данных PostgreSQL** (включая таблицы Directus)
2. **Backup volumes:**
```bash
docker run --rm \
  -v smpp-server_directus-uploads:/data \
  -v $(pwd)/backup:/backup \
  alpine tar czf /backup/directus-uploads.tar.gz -C /data .
```

3. **Восстановление:**
```bash
docker run --rm \
  -v smpp-server_directus-uploads:/data \
  -v $(pwd)/backup:/backup \
  alpine tar xzf /backup/directus-uploads.tar.gz -C /data
```

## Troubleshooting

### Directus не запускается

1. **Проверьте логи:**
```bash
docker-compose -f deployments/docker-compose.yml logs directus
```

2. **Проверьте подключение к БД:**
```bash
docker-compose -f deployments/docker-compose.yml exec postgres psql -U smpp -d smpp_db -c "SELECT 1;"
```

3. **Проверьте переменные окружения:**
```bash
docker-compose -f deployments/docker-compose.yml exec directus env | grep DIRECTUS
```

### Ошибки подключения к базе данных

1. Убедитесь, что PostgreSQL запущен и доступен
2. Проверьте правильность переменных `DIRECTUS_DATABASE_*`
3. Убедитесь, что PostgreSQL и Directus в одной Docker сети

### Коллекции не отображаются

1. Убедитесь, что миграции БД выполнены
2. Проверьте, что таблицы существуют в PostgreSQL
3. Выполните скрипт настройки коллекций:
```bash
./scripts/setup-directus.sh
```

### Не могу войти в систему

1. Проверьте переменные `DIRECTUS_ADMIN_EMAIL` и `DIRECTUS_ADMIN_PASSWORD`
2. Проверьте логи на наличие ошибок аутентификации
3. Если нужно сбросить пароль, остановите Directus, очистите таблицу `directus_users` (или пересоздайте контейнер)

## Production рекомендации

### Безопасность

1. **Используйте HTTPS:**
   - Настройте reverse proxy (nginx, traefik) с SSL сертификатами
   - Установите `DIRECTUS_PUBLIC_URL=https://yourdomain.com`

2. **Ограничьте доступ:**
   - Настройте firewall правила
   - Используйте VPN или whitelist IP адресов
   - Ограничьте доступ к Directus только для администраторов

3. **CORS:**
   - Настройте `DIRECTUS_CORS_ENABLED=true`
   - Настройте `DIRECTUS_CORS_ORIGIN` для разрешенных доменов

4. **Секреты:**
   - Используйте Docker Secrets или внешние системы управления секретами (Vault)
   - Не храните секреты в `.env` файлах в репозитории

### Производительность

1. **Кэширование:**
   - Настройте Redis для кэширования:
   ```env
   DIRECTUS_CACHE_ENABLED=true
   DIRECTUS_CACHE_STORE=redis
   DIRECTUS_CACHE_REDIS_HOST=redis
   DIRECTUS_CACHE_REDIS_PORT=6379
   ```

2. **Масштабирование:**
   - Directus поддерживает горизонтальное масштабирование
   - Используйте общий Redis для кэширования
   - Используйте общий PostgreSQL

### Мониторинг

1. **Health checks:**
   - Настройте мониторинг `/server/health` endpoint
   - Интегрируйте с Prometheus/Grafana

2. **Логирование:**
   - Настройте централизованное логирование (ELK, Loki)
   - Настройте ротацию логов

3. **Метрики:**
   - Directus предоставляет метрики через API
   - Настройте сбор метрик

## Дополнительная документация

- [Руководство по настройке коллекций](../development/directus-setup.md)
- [Официальная документация Directus](https://docs.directus.io/)
- [Docker Compose развертывание](docker-compose.md)
- [Конфигурация](configuration.md)