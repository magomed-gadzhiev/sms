# Portal UX Fixes — Command Center & Sub-accounts

**Date:** 2026-04-15
**Scope:** 4 точечные правки в пользовательской панели (роль: агрегатор)
**Decision:** Тёмная тема Command Center остаётся (намеренный monitoring-style дашборд). Правим только внутренние UX-проблемы.

## Правка 1: Локализация кнопок Live Feed

**Файл:** `portal-frontend/src/pages/CommandCenter.tsx` (строки 333-335)

**Проблема:** Кнопки `⏸ Pause` / `▶ Resume` на английском в полностью русскоязычном интерфейсе.

**Решение:**
- `⏸ Pause` → `⏸ Пауза`
- `▶ Resume` → `▶ Продолжить`

## Правка 2: Live Feed — отображение всех состояний подключения

**Файл:** `portal-frontend/src/pages/CommandCenter.tsx` (строки 367-376)

**Проблема:** При пустых сообщениях отображается только 2 текста. Хук `useWebSocket` возвращает 4 статуса (`connecting`, `open`, `closed`, `error`), но UI не различает `closed` и `error` — пользователь не понимает, есть ли проблема с соединением.

**Решение:** Отображать все 4 состояния:

| `status` | Текст | Цвет |
|----------|-------|------|
| `connecting` | "Подключение к live-ленте..." | `var(--cc-text-muted)` |
| `open` (0 сообщений) | "Ожидание сообщений..." | `var(--cc-text-muted)` |
| `closed` | "Соединение потеряно. Переподключение..." | `var(--cc-accent-yellow)` |
| `error` | "Ошибка подключения. Переподключение..." | `var(--cc-accent-red)` |

Индикатор-точка рядом с заголовком "Live Feed" (строка 317) уже реагирует на `status` — её не трогаем.

## Правка 3: Баланс — визуальный hint интерактивности

**Файл:** `portal-frontend/src/pages/CommandCenter.tsx` (строки 449-455)

**Проблема:** Кнопка-ссылка баланса в шапке Command Center выглядит как статичный текст. `hover:opacity-80` — слишком тонкий эффект, пользователь не догадывается, что это кликабельно.

**Решение:**
- Добавить стрелку `→` после суммы баланса
- Заменить `hover:opacity-80` на `hover:border-color` с `var(--cc-accent-blue)`
- Добавить `transition-colors` для плавности

**Было:**
```tsx
<Link
  to="/billing"
  style={{ color: 'var(--cc-text)', background: 'var(--cc-surface)', border: '1px solid var(--cc-border)' }}
  className="text-sm px-3 py-1.5 rounded-lg hover:opacity-80 transition-opacity"
>
  {balanceFormatted}
</Link>
```

**Станет:**
```tsx
<Link
  to="/billing"
  style={{ color: 'var(--cc-text)', background: 'var(--cc-surface)', border: '1px solid var(--cc-border)' }}
  className="text-sm px-3 py-1.5 rounded-lg hover:border-[var(--cc-accent-blue)] transition-colors flex items-center gap-1.5"
>
  {balanceFormatted}
  <span style={{ color: 'var(--cc-text-muted)' }} className="text-xs" aria-hidden="true">→</span>
</Link>
```

## Правка 4: Sub-accounts — empty state для 403

**Файл:** `portal-frontend/src/pages/sub-accounts/SubAccountsListPage.tsx` (строки 186-194)

**Проблема:** При отсутствии доступа к суб-аккаунтам (403) показывается amber-box с длинным текстом без визуального якоря и без CTA.

**Решение:**
- Добавить SVG-иконку замка над заголовком
- Сократить описание: "Функция доступна на тарифах с поддержкой реселлерских возможностей."
- Добавить ссылку-кнопку "Связаться с поддержкой" → `https://t.me/sms_support`
- Использовать `border-dashed` стиль (как в существующих empty states на этой же странице, строка 292)

**Было:**
```tsx
<div className="bg-amber-50 border border-amber-200 rounded-lg p-6 text-center">
  <p className="text-amber-800 font-medium text-lg mb-2">Суб-аккаунты недоступны</p>
  <p className="text-amber-600 text-sm">
    Для управления суб-аккаунтами необходим тарифный план с поддержкой реселлерских функций.
    Обратитесь к администратору для обновления тарифа.
  </p>
</div>
```

**Станет:**
```tsx
<div className="border border-dashed border-amber-300 rounded-lg p-12 text-center">
  <svg className="mx-auto mb-3 w-12 h-12 text-amber-400" ...lock icon... />
  <p className="text-amber-800 font-medium text-lg mb-2">Суб-аккаунты недоступны</p>
  <p className="text-amber-600 text-sm mb-4">
    Функция доступна на тарифах с поддержкой реселлерских возможностей.
  </p>
  <a href="https://t.me/sms_support" target="_blank" rel="noopener noreferrer"
     className="inline-flex items-center gap-1 text-sm text-primary hover:underline">
    Связаться с поддержкой →
  </a>
</div>
```

## Не входит в скоуп

- Смена темы Command Center (тёмная тема остаётся)
- Transition при навигации между тёмной/светлой темой
- Изменения в `useWebSocket.ts` (хук уже корректно обрабатывает reconnect)
- Правки на других страницах портала

## Затронутые файлы

1. `portal-frontend/src/pages/CommandCenter.tsx` — правки 1, 2, 3
2. `portal-frontend/src/pages/sub-accounts/SubAccountsListPage.tsx` — правка 4
