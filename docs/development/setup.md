# Настройка окружения разработки

## Требования

- Docker и Docker Compose
- Git

## Первоначальная настройка

1. Клонировать репозиторий
2. Запустить dev контейнер:

```bash
docker-compose -f deployments/docker-compose.yml up -d dev
```

3. Войти в контейнер:

```bash
docker-compose -f deployments/docker-compose.yml exec dev sh
```

4. В контейнере выполнить:

```bash
go mod download
```

## Работа с проектом

Все команды Go выполняются внутри dev контейнера.
