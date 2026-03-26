# Research: UX/UI and Accessibility — Паттерны и решения

**Branch**: `006-ux-a11y-audit` | **Date**: 2026-03-21

---

## 1. Focus Trap (формы-в-потоке)

**Decision**: Реализовать `useFocusTrap` hook через нативный DOM API без внешних зависимостей.

**Rationale**: Конституция требует Simplicity (VI). Библиотека `focus-trap-react` добавит ~30KB. Нативная реализация через `querySelectorAll('[tabindex]:not([tabindex="-1"]), button:not([disabled]), ...)` покрывает все наши случаи. Паттерн: при монтировании формы сохраняем предыдущий `document.activeElement`, устанавливаем фокус на первый интерактивный элемент формы, перехватываем `Tab`/`Shift+Tab` для цикличной навигации, при размонтировании возвращаем фокус.

**Alternatives considered**:
- `focus-trap-react` — отклонено: лишняя зависимость для простого случая
- `@radix-ui/react-dialog` — отклонено: формы не являются модальными диалогами, это inline-паттерн

**Implementation**:
```typescript
// Список selectors для фокусируемых элементов (WCAG-совместимый список)
const FOCUSABLE = 'a[href], button:not([disabled]), textarea, input:not([disabled]), select, [tabindex]:not([tabindex="-1"])'
```

---

## 2. aria-live Regions — Объявления для AT

**Decision**: Использовать `role="alert"` для срочных уведомлений (ошибки, критические банеры) и `aria-live="polite"` для информационных сообщений (успех, загрузка завершена).

**Rationale**: WCAG 4.1.3 требует уведомления о статусных изменениях для пользователей AT. `role="alert"` эквивалентен `aria-live="assertive"` — использовать только для ошибок и критических банеров (API Key, Webhook Secret), чтобы не перебивать текущее чтение.

**Alternatives considered**:
- Перемещение фокуса программно (`element.focus()`) — работает, но менее элегантно и нарушает пользовательский flow при пагинации

**Mapping по компонентам**:
| Элемент | Значение |
|---------|----------|
| Сообщения об ошибках в формах | `role="alert"` |
| Банер API Key (показывается только раз) | `role="alert"` |
| Банер Webhook Secret | `role="alert"` |
| Состояния загрузки (`Loading...`) | `role="status"` (эквивалент `aria-live="polite"`) |
| Сообщения об успехе (Profile saved, 2FA enabled) | `role="status"` |
| Тест webhook (информационный) | `role="status"` |

---

## 3. Skip Navigation Link

**Decision**: Реализовать видимый при фокусе skip-link как первый элемент `<body>`, ведущий на `#main-content`.

**Rationale**: WCAG 2.4.1 (bypass blocks) — обязательное требование уровня A. Без skip-link пользователь клавиатуры должен проходить 8 пунктов навигации на каждой странице.

**Alternatives considered**:
- Landmarks (nav + main) без skip-link — недостаточно для WCAG 2.4.1

**Implementation**: Компонент `<SkipLink>` с CSS `position: absolute; transform: translateY(-100%)` по умолчанию и `transform: translateY(0)` при `:focus`. Это общепринятый паттерн, не требующий изменений CSS-архитектуры (inline styles совместимы с текущим подходом проекта).

---

## 4. aria-current для Навигации

**Decision**: `aria-current="page"` на активном `<Link>` компоненте.

**Rationale**: WCAG 2.4.3 (focus order) и лучшая практика навигационных landmarks. Скринридеры объявляют "текущая страница" при встрече `aria-current="page"`.

**Implementation**: В `App.tsx` — добавить `aria-current={location.pathname === item.path ? 'page' : undefined}` к `<Link>` элементам.

---

## 5. Цветовые Индикаторы Статуса (Use of Color)

**Decision**: Добавить текстовый/иконочный дублёр к каждому цветовому индикатору.

**Rationale**: WCAG 1.4.1 запрещает передавать информацию только через цвет. Паттерн: `●` (bullet) или короткий ASCII-символ + текст, стилизованные цветом. Альтернатива с Unicode-символами (✓/✗) имеет проблемы с некоторыми скринридерами — предпочтительнее просто `(active)` текст с `aria-label`.

**Mapping**:
| Компонент | Текущее | Исправление |
|-----------|---------|-------------|
| API Keys status | `color: #4caf50` "Active" | `aria-label="Status: Active"` + визуальный индикатор |
| Webhooks status | `color: #4caf50` "Active" | `aria-label="Status: Active"` + визуальный индикатор |
| Sub-accounts active | `color: #4caf50` "Yes" / "No" | Заменить на "Active" / "Inactive" с aria-label |

---

## 6. Таблицы и Caption

**Decision**: Добавить `<caption>` к каждой таблице со смысловым описанием.

**Rationale**: WCAG 1.3.1 — таблицы должны иметь программно определяемое назначение. `<caption>` — нативный HTML-элемент, предпочтительнее `aria-label` на `<table>`.

**Caption для каждой таблицы**:
| Страница | Caption |
|----------|---------|
| Messages | "Messages list" |
| API Keys | "API Keys" |
| Webhooks | "Webhook subscriptions" |
| Sub-accounts | "Sub-accounts" |
| Audit Log | "Audit log entries" |
| Analytics Timeline | "Message statistics by period" |
| Analytics By Country | "Message statistics by country" |

---

## 7. Touch Target Sizes

**Decision**: Минимальный padding `8px 12px` для всех кнопок-действий в таблицах.

**Rationale**: WCAG 2.5.5 (AAA) рекомендует 44×44px; практичный минимум для мобильных пользователей — 36px. Текущий `padding: '4px 12px'` даёт ~28px. Изменение не влияет на layout (flexbox адаптируется).

---

## 8. Контрастность Цветов

**Decision**: Существующая палитра в целом соответствует WCAG AA; адресовать пограничные случаи.

**Rationale**: Проверка контрастности:
- `#666` на `#fff`: 5.74:1 — ✅ WCAG AA (4.5:1 для нормального текста)
- `#999` на `#fff` (пустые состояния): 2.85:1 — ❌ не соответствует WCAG AA
- `#4caf50` на `#fff`: 2.52:1 — ❌ для текста не соответствует; нужен `#2e7d32` (4.54:1)
- `#f44336` на `#fff`: 3.99:1 — ❌ немного ниже 4.5:1; нужен `#c62828` (5.8:1)

**Исправления**:
| Текущий цвет | Новый цвет | Контраст |
|--------------|-----------|---------|
| `#999` (placeholder/empty) | `#767676` | 4.54:1 ✅ |
| `#4caf50` (active status text) | Добавить `aria-label`, убрать цветовую зависимость | — |
| `#f44336` (error/revoked) | `#d32f2f` для текста | 4.59:1 ✅ |

---

## 9. Выравнивание решений с Конституцией

Все решения соответствуют Принципу VI (Simplicity):
- Нет новых npm-зависимостей (кроме нативных браузерных API)
- 2 новых файла (`useFocusTrap.ts`, `SkipLink.tsx`) — оправдано: используются в 5+ местах
- Прямые изменения в существующих компонентах без промежуточных абстракций
- Нет изменений бэкенда, базы данных или Kafka
