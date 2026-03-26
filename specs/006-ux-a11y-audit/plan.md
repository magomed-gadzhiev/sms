# Implementation Plan: UX/UI and Accessibility Audit

**Branch**: `006-ux-a11y-audit` | **Date**: 2026-03-21 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/006-ux-a11y-audit/spec.md`

## Summary

Исправление 18 нарушений UX/доступности в `portal-frontend` — React 19 + Vite SPA. Изменения охватывают исключительно фронтенд (HTML-семантика, ARIA-атрибуты, управление фокусом, цветовые индикаторы). Никаких изменений бэкенда, API-контрактов, баз данных или инфраструктуры не требуется. Подход: точечные inline-правки в 10 существующих компонентах без введения новых абстракций или зависимостей.

## Technical Context

**Language/Version**: TypeScript 5.x + React 19 (Vite)
**Primary Dependencies**: react-router-dom (уже используется), нет новых зависимостей
**Storage**: N/A (фронтенд-только)
**Testing**: Браузер + ручное тестирование скринридером (NVDA/VoiceOver); axe DevTools для автоматической проверки
**Target Platform**: Браузер (Chrome, Firefox, Safari) с поддержкой WCAG 2.1 AA
**Project Type**: SPA (Single Page Application), фронтенд-только
**Performance Goals**: N/A — изменения только в HTML/ARIA, без логических изменений
**Constraints**: Без внешних библиотек для focus trap / скринридера; реализация только встроенными средствами HTML и React
**Scale/Scope**: 10 страниц × 16 файлов `portal-frontend/src`

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Принцип | Статус | Комментарий |
|---------|--------|-------------|
| I. Domain-Driven Design | ✅ N/A | Фронтенд-только, бэкенд-сервисы не затронуты |
| II. Event-Driven Architecture | ✅ N/A | Нет изменений Kafka-событий |
| III. Contract-First APIs | ✅ N/A | Нет новых API-эндпоинтов |
| IV. Observability | ✅ N/A | Нет изменений в сервисах с метриками |
| V. Data Safety | ✅ N/A | Нет изменений схемы БД или хранения данных |
| VI. Simplicity | ✅ PASS | Расширяем существующие компоненты; нет новых файлов кроме одного хука `useFocusTrap`; нет преждевременных абстракций |

**Все gates пройдены. Violations отсутствуют.**

## Project Structure

### Documentation (this feature)

```text
specs/006-ux-a11y-audit/
├── plan.md              # Этот файл
├── research.md          # Phase 0: WCAG-паттерны и решения
├── data-model.md        # Phase 1: компонентные контракты (accessibility props)
├── quickstart.md        # Phase 1: как тестировать доступность
├── contracts/           # Phase 1: UI-контракты компонентов
└── tasks.md             # Phase 2: задачи (/speckit.tasks)
```

### Source Code (repository root)

```text
portal-frontend/
├── src/
│   ├── hooks/
│   │   └── useFocusTrap.ts          # Новый: focus trap для форм-в-потоке
│   ├── components/
│   │   └── SkipLink.tsx             # Новый: skip navigation link
│   ├── App.tsx                      # Изменения: nav aria-label, aria-current, skip-link
│   ├── contexts/
│   │   └── AuthContext.tsx          # Без изменений
│   └── pages/
│       ├── auth/
│       │   ├── LoginPage.tsx        # FR-001: aria-describedby, role="alert"
│       │   ├── PasswordResetRequestPage.tsx  # FR-001: aria-describedby
│       │   └── PasswordResetPage.tsx         # FR-001: aria-describedby
│       ├── dashboard/
│       │   └── DashboardPage.tsx    # FR-010, FR-011: role="status", ссылки-карточки
│       ├── messages/
│       │   └── MessagesPage.tsx     # FR-006, FR-010, FR-014, FR-016, FR-017, FR-018
│       ├── api-keys/
│       │   └── APIKeysPage.tsx      # FR-002, FR-003, FR-005, FR-006, FR-016, FR-017
│       ├── webhooks/
│       │   └── WebhooksPage.tsx     # FR-002, FR-003, FR-006, FR-016, FR-017
│       ├── analytics/
│       │   └── AnalyticsPage.tsx    # FR-006, FR-010, FR-015
│       ├── sub-accounts/
│       │   ├── SubAccountsListPage.tsx  # FR-006, FR-010, FR-016
│       │   └── SubAccountDetailPage.tsx # FR-006
│       ├── profile/
│       │   └── ProfilePage.tsx      # FR-002, FR-010
│       └── audit/
│           └── AuditLogPage.tsx     # FR-006, FR-010, FR-017
```

**Structure Decision**: Фронтенд-только SPA. Два новых файла (`useFocusTrap.ts`, `SkipLink.tsx`) + изменения в 12 существующих. Нет изменений в бэкенде, migrations или proto-файлах.

## Complexity Tracking

> Нет нарушений — раздел не требуется.
