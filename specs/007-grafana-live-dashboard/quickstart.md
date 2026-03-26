# Quickstart: Live Load Test Dashboard

**Feature**: 007-grafana-live-dashboard

## Предварительные требования

- Docker + Docker Compose
- Запущенная платформа SMS Gateway (`docker compose up -d`)
- Загруженные seed-данные (`test/load/fixtures/seed.sql`)

## Быстрый старт

### 1. Применить конфигурацию

Убедитесь, что в `deployments/docker-compose.yml` Grafana-сервис содержит:

```yaml
grafana:
  image: grafana/grafana:latest
  environment:
    - GF_SECURITY_ADMIN_USER=admin
    - GF_SECURITY_ADMIN_PASSWORD=admin
    - GF_USERS_ALLOW_SIGN_UP=false
    - GF_AUTH_ANONYMOUS_ENABLED=true          # НОВОЕ
    - GF_AUTH_ANONYMOUS_ORG_ROLE=Viewer        # НОВОЕ
  volumes:
    - grafana-data:/var/lib/grafana
    - ./configs/grafana/dashboards:/var/lib/grafana/dashboards:ro                          # ИЗМЕНЕНО
    - ./configs/grafana/provisioning/datasources:/etc/grafana/provisioning/datasources:ro  # НОВОЕ
    - ./configs/grafana/provisioning/dashboards:/etc/grafana/provisioning/dashboards:ro    # НОВОЕ
  depends_on:
    - prometheus
    - postgres                                 # НОВОЕ
```

### 2. Пересоздать контейнеры

```bash
cd deployments
docker compose up -d grafana prometheus
```

### 3. Открыть дашборд

Перейти по ссылке: **http://localhost:3000/d/load-test-live/**

Логин не требуется (анонимный доступ включён).

### 4. Запустить нагрузочный тест

```bash
cd scripts
./load-test.sh   # или k6 run k6_load_test.js
```

### 5. Наблюдать

- Дашборд обновляется каждые 5 секунд
- Баланс уменьшается с каждым отправленным сообщением
- Счётчики растут в реальном времени
- Лента сообщений показывает последние 20 записей

## Выбор клиента

По умолчанию дашборд показывает данные первого активного клиента. Используйте переменную `Client` в верхней части дашборда для выбора конкретного тестового клиента:
- **LoadTest-HighVolume** — 10K msg/s
- **LoadTest-MedVolume** — 1K msg/s
- **LoadTest-LowVolume** — 100 msg/s

## Устранение проблем

| Проблема | Решение |
|----------|---------|
| Дашборд не найден | Проверьте volume mount для provisioning/dashboards |
| "No data" на PostgreSQL-панелях | Проверьте, что postgres доступен из Grafana-контейнера |
| "No data" на Prometheus-панелях | Проверьте, что prometheus.yml обновлён для текущих сервисов |
| Баланс не меняется | Убедитесь, что billing-service и tarification-service запущены |
| Delivery rate = N/A | Нет доставленных/отклонённых сообщений — запустите тест подольше |

## Файлы проекта

```
deployments/configs/
├── prometheus.yml                          # Обновлённый scrape config
└── grafana/
    ├── dashboards/
    │   └── load-test-live.json             # Новый дашборд
    └── provisioning/
        ├── datasources/
        │   └── datasources.yml             # Prometheus + PostgreSQL
        └── dashboards/
            └── dashboards.yml              # File provider config
```
