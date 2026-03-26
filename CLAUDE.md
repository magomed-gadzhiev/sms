# sms Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-03-21

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
- 006-ux-a11y-audit: Added TypeScript 5.x + React 19 (Vite) + react-router-dom (уже используется), нет новых зависимостей
- 005-unit-functional-tests: Added Go 1.24.0 + testify/assert, testify/require, testify/mock, pgx/v5, go-redis/v9, gorilla/mux, google.golang.org/grpc, zerolog
- 004-hlr-smart-routing: Added Go 1.24.0 + gorilla/mux (HTTP), google.golang.org/grpc v1.78.0 (gRPC), IBM/sarama v1.43.0 (Kafka), jackc/pgx/v5 (PostgreSQL), redis/go-redis/v9 (Redis), rs/zerolog (logging), prometheus/client_golang (metrics)


<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
