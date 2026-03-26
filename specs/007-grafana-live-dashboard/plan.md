# Implementation Plan: Live Load Test Dashboard

**Branch**: `007-grafana-live-dashboard` | **Date**: 2026-03-26 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/007-grafana-live-dashboard/spec.md`

## Summary

Единый Grafana-дашборд с Prometheus и PostgreSQL datasources для наблюдения за движением данных в реальном времени при нагрузочном тестировании SMS-платформы. Дашборд доступен по публичной ссылке без аутентификации и обновляется каждые 5 секунд, показывая баланс, счётчики сообщений, пропускную способность, delivery rate, работу провайдеров и живую ленту последних сообщений.

Технический подход: Grafana JSON-дашборд с provisioning через YAML, два datasource (Prometheus для time-series метрик, PostgreSQL для бизнес-данных), обновление Prometheus scrape config для текущей архитектуры сервисов, включение анонимного доступа.

## Technical Context

**Language/Version**: Grafana JSON (дашборд), YAML (provisioning), Go 1.24.0 (новая метрика)
**Primary Dependencies**: Grafana 10+ (визуализация), Prometheus (time-series), PostgreSQL 15+ (бизнес-данные), grafana-postgresql-datasource (плагин)
**Storage**: Prometheus (метрики: counters, histograms, gauges), PostgreSQL (accounts, transactions, messages, tarification_log, aggregated_metrics)
**Testing**: Ручная верификация — запуск k6 load test + проверка панелей дашборда
**Target Platform**: Docker Compose (контейнер Grafana)
**Project Type**: Инфраструктурная конфигурация (provisioning + dashboard JSON)
**Performance Goals**: Обновление каждые 5 секунд, загрузка < 3 секунд, работа при 10K msg/s
**Constraints**: Публичный доступ без аутентификации, ≤10 одновременных зрителей, маскирование номеров телефонов
**Scale/Scope**: 1 дашборд (~15 панелей), 2 datasources, 2 template variables

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Принцип | Статус | Обоснование |
|---------|--------|-------------|
| I. DDD | ✅ PASS | Не создаём новый сервис. Расширяем инфраструктуру мониторинга |
| II. Event-Driven | ✅ N/A | Дашборд не взаимодействует с Kafka |
| III. Contract-First | ✅ PASS | Контракты определены: datasources.yml, dashboard-panels.md, prometheus config |
| IV. Observability | ✅ PASS | Фича усиливает observability — это её основная цель |
| V. Data Safety | ✅ PASS | Read-only доступ к БД, номера маскируются (FR-014) |
| VI. Simplicity | ✅ PASS | Используем существующие Grafana + Prometheus, минимум нового кода (1 метрика в Go) |

**Post-design re-check**: См. секцию ниже.

## Project Structure

### Documentation (this feature)

```text
specs/007-grafana-live-dashboard/
├── plan.md              # This file
├── research.md          # Phase 0: исследование datasources, provisioning, метрик
├── data-model.md        # Phase 1: сущности PostgreSQL + Prometheus метрики
├── quickstart.md        # Phase 1: инструкция по запуску
├── contracts/           # Phase 1: provisioning configs, panel layout
│   ├── grafana-datasources.yml
│   ├── grafana-dashboard-provisioning.yml
│   ├── dashboard-panels.md
│   └── prometheus-scrape-config.yml
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
deployments/configs/
├── prometheus.yml                              # ОБНОВИТЬ: актуальные scrape targets
└── grafana/
    ├── dashboards/
    │   ├── overview.json                       # Существующий
    │   ├── api-gateway.json                    # Существующий
    │   ├── provider-health.json                # Существующий
    │   ├── worker.json                         # Существующий
    │   └── load-test-live.json                 # НОВЫЙ: live load test dashboard
    └── provisioning/                           # НОВАЯ папка
        ├── datasources/
        │   └── datasources.yml                 # НОВЫЙ: Prometheus + PostgreSQL
        └── dashboards/
            └── dashboards.yml                  # НОВЫЙ: file provider config

deployments/docker-compose.yml                  # ОБНОВИТЬ: Grafana volumes + env vars

internal/monitoring/metrics.go                  # ОБНОВИТЬ: добавить smpp_messages_delivered_total
```

**Structure Decision**: Инфраструктурный проект — все изменения в `deployments/configs/` (Grafana/Prometheus конфигурация) и одно минимальное изменение в `internal/monitoring/metrics.go` (новый Prometheus counter). Новых сервисов или Go-модулей не создаётся.

## Complexity Tracking

> Нет нарушений Constitution Check — секция не заполняется.
