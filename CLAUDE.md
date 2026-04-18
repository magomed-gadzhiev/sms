# sms Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-04-18

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
- Go 1.24+ (backend), TypeScript 5.x + React 19 (frontend) + gorilla/mux, google.golang.org/grpc v1.78.0, IBM/sarama v1.43.0, jackc/pgx/v5, redis/go-redis/v9, rs/zerolog, prometheus/client_golang (012-multichannel-cascade)
- PostgreSQL 15+ (pgx, monthly partitioning для `deliveries`, `delivery_attempts`), Redis 7+ (reachability cache TTL 1h) (012-multichannel-cascade)
- Go 1.24.0 + gorilla/mux (HTTP), jackc/pgx/v5 (PostgreSQL), redis/go-redis/v9 (Redis), rs/zerolog (logging), prometheus/client_golang (metrics), IBM/sarama v1.43.0 (Kafka), crypto/hmac (webhook signature) (013-max-messenger-channel)
- PostgreSQL 15+ (существующие таблицы cascade), Redis 7+ (reachability cache) (013-max-messenger-channel)
- Go 1.24.0 (бэкенд), TypeScript 5.x + React 19 (фронтенд) + gorilla/mux, google.golang.org/grpc v1.78.0, IBM/sarama v1.43.0, jackc/pgx/v5, redis/go-redis/v9, rs/zerolog, React 19 + Vite (017-tech-debt-refactor)
- PostgreSQL 15+ (pgx), Redis 7+ (017-tech-debt-refactor)
- TypeScript 5.7 + React 19, Vite 6.0 + React Router 7.1, Tailwind CSS 4.2, Radix UI (018-fix-portal-qa-bugs)
- N/A (frontend-only changes) (018-fix-portal-qa-bugs)
- Go 1.24.0 (backend), TypeScript 5.7 + React 19 (frontend) + gorilla/mux, gRPC (analyticsv1, messagingv1, campaignv1), pgx/v5 (portal's own pool), redis/go-redis/v9 (export jobs), Recharts 3.8.1 (charts), Radix UI Dialog/DropdownMenu/Tabs (UI components), Tailwind CSS 4.2 (019-portal-ux-improvements)
- PostgreSQL 15+ (новая таблица `notifications`), Redis 7+ (export job state, TTL 1h) (019-portal-ux-improvements)

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
- 019-portal-ux-improvements: Added Go 1.24.0 (backend), TypeScript 5.7 + React 19 (frontend) + gorilla/mux, gRPC (analyticsv1, messagingv1, campaignv1), pgx/v5 (portal's own pool), redis/go-redis/v9 (export jobs), Recharts 3.8.1 (charts), Radix UI Dialog/DropdownMenu/Tabs (UI components), Tailwind CSS 4.2
- 018-fix-portal-qa-bugs: Added TypeScript 5.7 + React 19, Vite 6.0 + React Router 7.1, Tailwind CSS 4.2, Radix UI
- 018-fix-portal-qa-bugs: Added [if applicable, e.g., PostgreSQL, CoreData, files or N/A]


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

## Quality Gates

Автоматические проверки запускаются локально (pre-commit hook) и на сервере (GitHub Actions CI).

### Первый запуск (установка)

```bash
# 1. Установить новые frontend-зависимости
cd portal-frontend && npm install && cd ..

# 2. Включить pre-commit hook (hooks живут в .githooks/, не в .git/hooks/)
git config core.hooksPath .githooks

# 3. Прогнать проверки локально вручную
./scripts/check.sh
```

### Что проверяется

| Чек | Где | Когда |
|---|---|---|
| `go vet ./...` | корень | pre-commit + CI |
| `go build ./...` | корень | pre-commit + CI |
| `go test -short ./...` | корень | CI only |
| `tsc --noEmit` | portal-frontend | pre-commit + CI |
| `eslint .` | portal-frontend | pre-commit + CI |

### Основная команда

- `./scripts/check.sh` — быстрые проверки (без тестов)
- `./scripts/check.sh --with-tests` — полный режим (как в CI)

### Обход в экстренной ситуации

Только если сломано что-то внешнее (не твой код):
```bash
git commit --no-verify -m "<сообщение>

<описание, почему обходим hook>"
```

Каждый `--no-verify` должен быть обоснован в теле коммита. Злоупотребление = инфра мёртвая.

### Ratchet на ESLint warnings

Baseline 2026-04-18: 69 warnings (в основном `no-explicit-any`, `exhaustive-deps`).

Гейт в `package.json` lint-команде и в `scripts/check.sh` настроен на `--max-warnings=69`.

**Правило:** любой новый PR может только **уменьшить** число warnings, не увеличить. При устранении warnings обновляй цифру вниз (и в package.json, и в check.sh). Новые warnings не добавляются.

### Go-чеки на Windows под Device Guard

Если `go vet` / `go build` падают с "заблокирован политикой Device Guard" при запуске из-под Claude Code CLI — это ограничение целостности процесса. `scripts/check.sh` это обнаруживает и пропускает Go-чеки с `[SKIP]` warning. **CI (GitHub Actions, Linux) не затронут** — там Go валидируется строго.

Для локальной проверки Go вручную — запусти `./scripts/check.sh` из обычного git-bash или PowerShell.

### Mandatory code review для любой работы с кодом

Deliverable 2 Phase 1. Lint/CI ловят синтаксис и типы. Review ловит семантику, архитектуру и spec-drift.

**Правило:** для любой задачи, меняющей код приложения, используй `/execute-with-review` (см. `skills/execute-with-review.md`), НЕ `/executing-plans` напрямую.

Алгоритм:
1. Реализация через `superpowers:executing-plans` (внутри wrapper'а)
2. **Обязательный** субагент-ревьюер (`superpowers:code-reviewer`) проверяет diff
3. APPROVED → коммит; CHANGES_REQUESTED → фикс-итерация (макс 3 раза)
4. После 3-й неуспешной итерации — эскалация пользователю

**Исключения** (можно `/executing-plans` напрямую):
- Работа только с документами (AC, specs, README) без кода
- Исследование без коммита
- Срочный hotfix — review post-factum

**Обход через `--no-verify`** — табу, всегда через wrapper. Любой обход ставит под сомнение всю Фазу 1.

## Communication Style

Максимальная критика. Пользователь — соло-разработчик, полагается на тебя как на советника, а не как на согласителя. Согласие без критики здесь вредно.

- **Сначала критика, потом согласие.** Когда пользователь предлагает подход или аргумент, первым делом — контраргументы. Если критика не выдерживает — тогда соглашайся. Но никогда не соглашайся просто чтобы закрыть разговор.
- **Критикуй и свои собственные предложения.** Перед тем как что-то рекомендовать, вслух озвучь сильнейший аргумент против. Если аргумент сильнее рекомендации — не рекомендуй.
- **Факты, а не абстракции.** Диагнозы и рекомендации подкрепляй конкретными файлами, номерами строк, git-коммитами, числами. "Это сломано" без цифр — не аргумент.
- **Называй жертвы.** Не подавай выбор как "это лучшее решение", если у него есть реальная цена. Явно проговаривай, что приносится в жертву.
- **Противостой scope creep.** Если пользователь просит X, а нужно Y сначала — скажи об этом, не соглашайся молча.
- **Без лести.** "Отличный вопрос", "хорошая мысль" — не писать. Сразу к содержанию.
- **Когда пользователь давит — пересматривай честно.** Если у него сильный аргумент — обнови позицию явно. Если аргумент слабый — дави обратно, не прогибайся.
- **Ставь под сомнение собственные формулировки.** Если на предыдущем шаге что-то предложил и это оказалось избыточным ограничением — признай это открыто ("я был излишне ограничивал"), не маскируй.

Эта инструкция имеет приоритет над дефолтным поведением "be helpful".

<!-- MANUAL ADDITIONS END -->
