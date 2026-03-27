# Remote Server Management — Design Spec

## Overview

Управление удалённым сервером через единый bash-скрипт `scripts/server.sh`, вызываемый AI-агентом (Claude Code) или вручную. Деплой через rsync + docker compose.

## Параметры сервера

- **Хост**: `claude@72.56.232.202`
- **SSH-алиас**: `sms-server`
- **Порт**: 22
- **Директория на сервере**: `/opt/sms`
- **Требования**: Docker и Docker Compose уже установлены

## SSH-настройка

Перед использованием скрипта необходимо:

1. Сгенерировать SSH-ключ (если нет): `ssh-keygen`
2. Скопировать на сервер: `ssh-copy-id claude@72.56.232.202`
3. Добавить алиас в `~/.ssh/config`:
   ```
   Host sms-server
       HostName 72.56.232.202
       User claude
       Port 22
   ```
4. Проверить подключение: `ssh sms-server echo "ok"`

Скрипт использует алиас `sms-server` — смена адреса в одном месте (`~/.ssh/config`).

## Скрипт `scripts/server.sh`

### Конфигурация

Переменные в начале скрипта:

```bash
SSH_HOST="sms-server"
REMOTE_DIR="/opt/sms"
COMPOSE_FILE="deployments/docker-compose.yml"
```

### Команды

| Команда | Описание | Пример |
|---------|----------|--------|
| `setup` | Первоначальная подготовка: создаёт `/opt/sms`, синхронизирует файлы, собирает и запускает стек | `./scripts/server.sh setup` |
| `deploy` | rsync + пересборка и перезапуск изменённых сервисов | `./scripts/server.sh deploy` |
| `deploy <service>` | Пересборка и перезапуск одного сервиса | `./scripts/server.sh deploy messaging-service` |
| `exec <command>` | Произвольная команда на сервере | `./scripts/server.sh exec "docker ps"` |
| `logs <service>` | Логи сервиса (последние 100 строк + follow) | `./scripts/server.sh logs messaging-service` |
| `status` | Статус всех контейнеров | `./scripts/server.sh status` |
| `restart <service>` | Перезапуск конкретного сервиса | `./scripts/server.sh restart worker` |
| `stop` | Остановка всего стека | `./scripts/server.sh stop` |
| `migrate` | Применение миграций БД | `./scripts/server.sh migrate` |
| `seed` | Накатка тестовых данных из seed.sql | `./scripts/server.sh seed` |
| `sync` | Только rsync без перезапуска | `./scripts/server.sh sync` |

### Rsync

Синхронизация локальной директории проекта → `/opt/sms` на сервере.

Исключения:
- `.git/`
- `node_modules/`
- `test/`
- `docs/`
- `specs/`
- `*.md`
- `pipeline-worker` (скомпилированный бинарник)
- `worker` (скомпилированный бинарник)

### Логика команд

**`setup`**:
1. `ssh sms-server "mkdir -p /opt/sms"`
2. rsync всех файлов
3. `ssh sms-server "cd /opt/sms && docker compose -f deployments/docker-compose.yml up -d --build"`

**`deploy`** (без аргумента — полный деплой):
1. rsync изменённых файлов
2. `ssh sms-server "cd /opt/sms && docker compose -f deployments/docker-compose.yml up -d --build"`

**`deploy <service>`** (один сервис):
1. rsync изменённых файлов
2. `ssh sms-server "cd /opt/sms && docker compose -f deployments/docker-compose.yml up -d --build <service>"`

**`exec <command>`**:
1. `ssh sms-server "<command>"`

**`logs <service>`**:
1. `ssh sms-server "cd /opt/sms && docker compose -f deployments/docker-compose.yml logs --tail=100 -f <service>"`

**`status`**:
1. `ssh sms-server "cd /opt/sms && docker compose -f deployments/docker-compose.yml ps"`

**`restart <service>`**:
1. `ssh sms-server "cd /opt/sms && docker compose -f deployments/docker-compose.yml restart <service>"`

**`stop`**:
1. `ssh sms-server "cd /opt/sms && docker compose -f deployments/docker-compose.yml down"`

**`migrate`**:
1. `ssh sms-server "docker run --rm -v /opt/sms/migrations:/migrations --network deployments_smpp-network migrate/migrate -path /migrations -database 'postgres://smpp:smpp_password@postgres:5432/smpp_db?sslmode=disable' up"`

Используется Docker-образ `migrate/migrate` — тот же подход что в `scripts/start.sh`.

**`seed`**:
1. rsync `test/load/fixtures/seed.sql` на сервер отдельно (эта директория исключена из основного rsync)
2. `ssh sms-server "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T postgres psql -U smpp -d smpp_db < test/load/fixtures/seed.sql"`

## Интеграция с AI-агентом

Добавить в `CLAUDE.md`:

```markdown
## Server Management

Сервер: claude@72.56.232.202 (SSH alias: sms-server), деплой в /opt/sms

Управление через `scripts/server.sh <command>`:
- setup — первоначальная настройка сервера
- deploy — синхронизировать и перезапустить
- deploy <service> — обновить один сервис
- status — статус контейнеров
- logs <service> — логи сервиса
- exec "<cmd>" — выполнить команду на сервере
- migrate — применить миграции
- seed — накатить тестовые данные
- restart/stop — управление стеком
- sync — только синхронизация файлов

Перед деплоем всегда проверяй что код компилируется локально.
```

Агент вызывает команды через стандартный Bash tool — дополнительных плагинов не требуется.

## Что НЕ входит в скоуп

- CI/CD pipeline (GitHub Actions)
- Docker registry / образы
- Provisioning сервера (установка Docker, firewall)
- SSL/TLS, домены, reverse proxy
- Мониторинг доступности сервера
