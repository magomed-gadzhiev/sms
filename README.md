# SMPP Server

Аналог smpp сервера Kanel + smsbox с поддержкой gRPC + HTTP на языке Go.

## Архитектура

Микросервисная архитектура состоит из:
- **API Gateway** - HTTP/gRPC API (масштабируемый, множественные инстансы)
- **SMPP Server** - прием входящих SMPP соединений от клиентов
- **Worker** - обработка очереди Kafka и отправка в SMSC провайдеры

## Структура проекта

```
smpp-server/
├── cmd/              # Точки входа сервисов
├── internal/         # Внутренние пакеты
├── pkg/              # Публичные пакеты
├── api/              # API спецификации (proto, openapi)
├── docs/             # Документация
├── deployments/      # Docker и конфигурации
├── migrations/       # БД миграции
└── scripts/          # Утилитные скрипты
```

## Разработка

Все команды выполняются в Docker контейнерах. См. [docs/development/setup.md](docs/development/setup.md) для настройки окружения.

- **Pre-commit hooks:** см. [docs/development/pre-commit-hooks.md](docs/development/pre-commit-hooks.md). После клона выполнить `git config core.hooksPath .githooks` — иначе quality-gates не запустятся локально.

## Документация

- [Архитектура](docs/architecture/)
- [API документация](docs/api/)
- [Развертывание](docs/deployment/)
- [Разработка](docs/development/)
