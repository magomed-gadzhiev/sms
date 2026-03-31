# sms Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-03-31

## Active Technologies
- Go 1.24.0 + gorilla/mux (HTTP), google.golang.org/grpc v1.78.0 (gRPC), IBM/sarama v1.43.0 (Kafka), jackc/pgx/v5 (PostgreSQL), redis/go-redis/v9 (Redis), rs/zerolog (logging), spf13/viper (config), prometheus/client_golang (metrics), stretchr/testify (testing) (002-operator-tarification)
- PostgreSQL 15+ (pgx driver, monthly partitioning for high-volume tables), Redis 7+ (caching) (002-operator-tarification)
- Go 1.24.0 (backend), TypeScript (frontend SPA) + gorilla/mux (HTTP), google.golang.org/grpc v1.78.0, IBM/sarama v1.43.0 (Kafka), jackc/pgx/v5, redis/go-redis/v9, rs/zerolog, golang-jwt/jwt/v5, pquerna/otp (TOTP 2FA), React 19 + Vite (frontend) (003-self-service-portal)
- PostgreSQL 15+ (pgx, monthly partitioning для audit_log), Redis 7+ (сессии, rate-limiting, кеш) (003-self-service-portal)
- Go 1.24.0 + gorilla/mux (HTTP), google.golang.org/grpc v1.78.0 (gRPC), IBM/sarama v1.43.0 (Kafka), jackc/pgx/v5 (PostgreSQL), redis/go-redis/v9 (Redis), rs/zerolog (logging), prometheus/client_golang (metrics) (004-hlr-smart-routing)
- PostgreSQL 15+ (pgx driver, monthly partitioning для lookup_log), Redis 7+ (HLR cache) (004-hlr-smart-routing)
- Go 1.24.0 + testify/assert, testify/require, testify/mock, pgx/v5, go-redis/v9, gorilla/mux, google.golang.org/grpc, zerolog (005-unit-functional-tests)
- PostgreSQL 15+ (тестовая БД через pgx), Redis 7+ (тестовый для сессий/кеша) (005-unit-functional-tests)
- TypeScript 5.x + React 19 (Vite) + react-router-dom (уже используется), нет новых зависимостей (006-ux-a11y-audit)
- N/A (фронтенд-только) (006-ux-a11y-audit)
- Grafana JSON (дашборд), YAML (provisioning), Go 1.24.0 (новая метрика) + Grafana 10+ (визуализация), Prometheus (time-series), PostgreSQL 15+ (бизнес-данные), grafana-postgresql-datasource (плагин) (007-grafana-live-dashboard)
- Prometheus (метрики: counters, histograms, gauges), PostgreSQL (accounts, transactions, messages, tarification_log, aggregated_metrics) (007-grafana-live-dashboard)
- PostgreSQL 15+ (pgx, monthly partitioning для messages/audit_log), Redis 7+ (rate-limiting, cache), Apache Kafka (inter-stage messaging) (008-high-throughput-pipeline)
- Go 1.24.0 (backend), TypeScript 5.x + React 19 (frontend) + gorilla/mux, google.golang.org/grpc v1.78.0, IBM/sarama v1.43.0, jackc/pgx/v5, redis/go-redis/v9, rs/zerolog, prometheus/client_golang, stretchr/testify (010-sender-names-templates)
- PostgreSQL 15+ (pgx driver); новые таблицы `sender_names`, `sender_name_status_history`; ALTER TABLE `templates` (010-sender-names-templates)
- Go 1.24.0 (backend), TypeScript 5.x + React 19 / Vite (frontend) + gorilla/mux, google.golang.org/grpc v1.78.0, IBM/sarama v1.43.0, jackc/pgx/v5, redis/go-redis/v9, rs/zerolog, prometheus/client_golang (011-operator-sender-billing)
- PostgreSQL 15+ (pgx driver); таблицы `operators` (ALTER), новая `sender_name_billing_records` (011-operator-sender-billing)

- Go 1.24.0 + gorilla/mux (HTTP), google.golang.org/grpc v1.78.0 (gRPC), IBM/sarama v1.43.0 (Kafka), jackc/pgx/v5 (PostgreSQL), redis/go-redis/v9 (Redis), rs/zerolog (logging), spf13/viper (config), golang-jwt/jwt/v5 (auth), prometheus/client_golang (metrics), stretchr/testify (testing) (001-sms-gateway-platform)

## Project Structure

```text
src/
tests/
```

## Commands

# Add commands for Go 1.24.0

## Code Style

Go 1.24.0: Follow standard conventions

## Recent Changes
- 011-operator-sender-billing: Added Go 1.24.0 (backend), TypeScript 5.x + React 19 / Vite (frontend) + gorilla/mux, google.golang.org/grpc v1.78.0, IBM/sarama v1.43.0, jackc/pgx/v5, redis/go-redis/v9, rs/zerolog, prometheus/client_golang
- 010-sender-names-templates: Added Go 1.24.0 (backend), TypeScript 5.x + React 19 (frontend) + gorilla/mux, google.golang.org/grpc v1.78.0, IBM/sarama v1.43.0, jackc/pgx/v5, redis/go-redis/v9, rs/zerolog, prometheus/client_golang, stretchr/testify
- 008-high-throughput-pipeline: Added Go 1.24.0 + gorilla/mux (HTTP), google.golang.org/grpc v1.78.0 (gRPC), IBM/sarama v1.43.0 (Kafka), jackc/pgx/v5 (PostgreSQL), redis/go-redis/v9 (Redis), rs/zerolog (logging), spf13/viper (config), prometheus/client_golang (metrics), stretchr/testify (testing)


## Server Management

Сервер: claude@72.56.232.202 (SSH alias: sms-server), деплой в /opt/sms

Деплой через git (код пушится в GitHub, на сервере git pull).

Управление через `scripts/server.sh <command>`:
- setup — клонировать репо, собрать и запустить
- deploy — git pull + пересборка контейнеров
- deploy <service> — обновить один сервис
- status — статус контейнеров
- logs <service> — логи сервиса (tail -f)
- exec "<cmd>" — выполнить команду на сервере
- migrate — применить миграции БД
- seed — накатить тестовые данные
- restart <service> / stop — управление стеком
- sync — только git pull (без перезапуска)

Переменная `DEPLOY_BRANCH` — ветка для деплоя (по умолчанию master).

Перед деплоем: запушить изменения в GitHub, проверить что код компилируется.

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
