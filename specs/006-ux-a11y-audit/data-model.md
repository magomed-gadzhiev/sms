# Data Model: Accessibility Patterns & Component Contracts

**Branch**: `006-ux-a11y-audit` | **Date**: 2026-03-21

> Для фронтенд-аудита "data model" — это компонентные интерфейсы и паттерны доступности, которые должны применяться единообразно.

---

## Паттерн 1: Обёртка сообщения об ошибке

**Назначение**: Программная связь ошибки формы с полем через `aria-describedby`.

```
ErrorMessage
├── id: string              (уникальный, совпадает с aria-describedby поля)
├── message: string         (текст ошибки)
├── role: "alert"           (обязательно для ошибок)
└── визуальный стиль:
    ├── color: #d32f2f       (WCAG AA контраст)
    └── margin-top: 4px
```

**State transitions**: Пусто → Ошибка → Пусто (при успешной повторной отправке)

---

## Паттерн 2: Статусный банер (одноразовый секрет)

**Назначение**: Уведомление об API Key / Webhook Secret, который показывается только один раз.

```
SecretBanner
├── role: "alert"            (обязательно: AT объявляет немедленно)
├── heading: string          ("API Key created successfully." / "Webhook created.")
├── secret: string           (значение ключа/секрета)
├── onCopy: () => void        (действие копирования)
├── onDismiss: () => void     (действие закрытия)
└── copied: boolean           (состояние кнопки Copy)
```

**State transitions**: Hidden → Visible (после создания) → Hidden (после Dismiss)

---

## Паттерн 3: Загрузочное состояние

**Назначение**: Объявление состояния загрузки для AT.

```
LoadingIndicator
├── role: "status"           (aria-live="polite" по умолчанию)
├── message: string          ("Loading...", "Loading messages...", etc.)
└── aria-label: string       (то же, что message, для полной совместимости)
```

---

## Паттерн 4: Статусный бейдж

**Назначение**: Визуальный индикатор состояния без нарушения WCAG 1.4.1.

```
StatusBadge
├── active: boolean
├── activeLabel: string      ("Active" по умолчанию)
├── inactiveLabel: string    ("Revoked" / "Inactive" / "No")
└── aria-label: string       ("Status: Active" / "Status: Revoked")
```

**Визуальный стиль**: `color` + `font-weight: bold` (цвет остаётся, но информация дублируется через text + aria-label)

---

## Паттерн 5: Кнопка действия с контекстом

**Назначение**: Кнопки в таблицах с именем конкретного ресурса.

```
ActionButton
├── label: string            ("Revoke" / "Delete" / "Edit" / "Test")
├── resourceName: string     (имя ключа/webhook для aria-label)
└── aria-label: string       (auto: `${label} ${resourceName}`)

Пример: aria-label="Revoke Production Key"
```

---

## Паттерн 6: Таблица с caption

**Назначение**: Все таблицы должны иметь семантически связанный заголовок.

```
AccessibleTable
├── caption: string          (описание содержимого таблицы)
├── captionVisible: boolean  (false = visually hidden, но доступен AT)
└── children: ReactNode      (стандартный thead/tbody)
```

---

## Паттерн 7: useFocusTrap Hook

**Назначение**: Замкнуть Tab-навигацию внутри открытой inline-формы.

```
useFocusTrap(
  containerRef: RefObject<HTMLElement>,
  isActive: boolean
) → void

Поведение:
├── При isActive=true: переместить фокус на первый focusable элемент
├── При Tab: если фокус на последнем → переместить на первый
├── При Shift+Tab: если фокус на первом → переместить на последний
├── При Escape: вызвать onClose callback (если передан)
└── При isActive=false: вернуть фокус на previouslyFocusedElement
```

---

## Паттерн 8: SkipLink

**Назначение**: Первый элемент страницы для обхода навигации.

```
SkipLink
├── targetId: string         ("main-content")
├── label: string            ("Skip to main content")
└── стиль:
    ├── default: position:absolute; transform:translateY(-100%)
    └── :focus: transform:translateY(0)
```

---

## Карта применения паттернов

| Компонент | Паттерны |
|-----------|---------|
| `App.tsx` | SkipLink (8), aria-current nav |
| `LoginPage.tsx` | ErrorMessage (1), LoadingIndicator (3) |
| `PasswordResetRequestPage.tsx` | ErrorMessage (1), LoadingIndicator (3) |
| `PasswordResetPage.tsx` | ErrorMessage (1), LoadingIndicator (3) |
| `DashboardPage.tsx` | LoadingIndicator (3) |
| `APIKeysPage.tsx` | SecretBanner (2), StatusBadge (4), ActionButton (5), Table (6), useFocusTrap (7) |
| `WebhooksPage.tsx` | SecretBanner (2), StatusBadge (4), ActionButton (5), Table (6), useFocusTrap (7) |
| `MessagesPage.tsx` | Table (6), LoadingIndicator (3) |
| `AnalyticsPage.tsx` | LoadingIndicator (3), Table (6) |
| `SubAccountsListPage.tsx` | StatusBadge (4), Table (6), useFocusTrap (7) |
| `ProfilePage.tsx` | ErrorMessage (1), LoadingIndicator (3) |
| `AuditLogPage.tsx` | Table (6), LoadingIndicator (3) |
