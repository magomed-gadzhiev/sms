# sms Development Guidelines

SMS-платформа: набор Go-микросервисов + React-портал. Spec-driven development через `.specify/`.

## Stack

**Backend (Go 1.24.0):** gorilla/mux (HTTP), google.golang.org/grpc v1.78.0 (gRPC), IBM/sarama v1.43.0 (Kafka), jackc/pgx/v5 (PostgreSQL), redis/go-redis/v9 (Redis), rs/zerolog (logging), spf13/viper (config), golang-jwt/jwt/v5 (auth), pquerna/otp (TOTP 2FA), prometheus/client_golang (metrics), stretchr/testify (tests).

**Frontend (`portal-frontend/`):** TypeScript 5.7 + React 19, Vite 6, React Router 7.1, Tailwind CSS 4.2, Radix UI, Recharts 3.8.

**Infra:** PostgreSQL 15+ (monthly partitioning для `messages`, `audit_log`, `lookup_log`, `deliveries`, `delivery_attempts`), Redis 7+ (sessions, rate-limit, cache), Apache Kafka (pipeline), Prometheus + Grafana.

## Repo map

```
cmd/                       # main-пакеты сервисов: admin-gateway, api, client-gateway,
                           # dlr-delivery, pipeline-worker, portal-gateway, seed-admin,
                           # services, smpp-gateway, smpp-server, worker
internal/
  api/                     # HTTP/gRPC handlers
  config/                  # viper конфигурация
  gateway/                 # SMPP-шлюзы
  monitoring/              # Prometheus exporters
  pipeline/                # Kafka-стадии (message processing)
  queue/                   # очереди и DLQ
  router/                  # smart routing
  services/                # бизнес-сервисы (tarification, billing, audit, ...)
  shared/                  # общие модели/утилиты
  smpp/, smsc/             # SMPP-протокол
  storage/                 # pgx-репозитории
  testutil/                # тест-хелперы
api/                       # proto-определения и сгенерированные стабы
portal-frontend/           # SPA (React 19 / Vite / TS)
deployments/               # docker-compose, configs, directus, docker/
migrations/                # SQL-миграции (golang-migrate формат)
specs/                     # spec-driven фичи (см. ниже)
docs/                      # архитектура, AC, отчёты, deployment-гайды
scripts/                   # server.sh, check.sh, утилиты
test/, tests/, e2e/        # интеграционные/E2E тесты
.specify/                  # spec-kit конфигурация
.githooks/                 # pre-commit
skills/                    # локальные skills для Claude
```

## Specs

Активные спецификации (`specs/<NNN>-<slug>/`):

| ID | Slug | Статус |
|----|------|--------|
| 001 | sms-gateway-platform | ядро платформы |
| 002 | operator-tarification | тарификация по операторам |
| 003 | self-service-portal | клиентский портал |
| 004 | hlr-smart-routing | HLR + smart routing |
| 005 | unit-functional-tests | тестовый каркас |
| 006 | ux-a11y-audit | a11y портала |
| 007 | grafana-live-dashboard | Grafana-дашборд |
| 008 | high-throughput-pipeline | pipeline на Kafka |
| 010 | sender-names-templates | sender names + шаблоны |
| 011 | operator-sender-billing | биллинг по операторам |
| 012 | multichannel-cascade | каскад каналов |
| 013 | max-messenger-channel | MAX мессенджер |
| 015 | architecture-data-flows | потоки данных (для онбординга) |
| 017 | tech-debt-refactor | техдолг |
| 018 | fix-portal-qa-bugs | QA-фиксы портала |
| 019 | portal-ux-improvements | UX-улучшения портала |

Каждая спека: `spec.md` (обязательно), плюс опционально `plan.md`, `tasks.md`, `data-model.md`, `quickstart.md`, `research.md`, `checklists/`.

## Code Style

Go 1.24.0: стандартные конвенции (`go vet`, `go fmt`). TypeScript: ESLint baseline 69 warnings (см. ниже).

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
