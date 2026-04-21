# Network Tariffs Redesign — Design Spec

**Date:** 2026-04-22
**Status:** Draft (awaiting user review)
**Scope:** `/network/tariffs` (admin) + `/tariffs` (client readonly)
**Data model:** unchanged — reuses tables from `2026-04-16-reseller-subaccount-tariffs-design.md`

## Summary

Переделка страницы тарифов реселлера. Текущий UI (три вкладки: Обзор/Шаблоны/Переопределения с 4-уровневой вложенностью `Шаблон → План → Период → Ступени`) заменяется на три URL-based страницы + матричный редактор в стиле RedSMS с фильтрами сверху и таблицей операторы×ступени. Тот же матричный компонент переиспользуется на клиентской странице `/tariffs` в readonly-режиме. Накрытый гэп — видимость источника цены («из шаблона» vs «переопределение») в одной матрице.

## Goals / Non-goals

**Goals:**
- Убрать 4-уровневую вложенность аккордеонов; прямой доступ к ценам.
- Сделать источник эффективной цены видимым в одной матрице (inherited vs override).
- Дать админу обзор сети «кто с каким тарифом» на первом экране.
- Унифицировать визуал просмотра тарифа между админом и клиентом.
- Исправить a11y-нарушения текущего UI (▶-раскрывашки не-buttonable, keyboard-navigation сломана).

**Non-goals:**
- Изменение data model — переиспользуем таблицы из спеки 2026-04-16.
- Экспорт, сравнение субаккаунтов, графики динамики.
- Версионирование шаблонов, черновики, мультивалютные матрицы на одном экране.
- Уведомления клиенту об изменениях цен.
- Редактирование legacy-тарифов (остаются read-only с бейджем).
- Редизайн страницы «Стоимость имён и шаблонов операторов» (v1.1 Figma) — отдельная ветка.

## 1. Architecture

Три URL-based страницы + один переиспользуемый компонент.

### 1.1 Routes

- **`/network/tariffs`** (admin, `reseller_admin`) — список субаккаунтов со сводкой их тарифа.
- **`/network/tariffs/templates`** (admin) — библиотека шаблонов тарифов.
- **`/network/tariffs/editor/:id`** (admin) — матричный редактор. Режим определяется query: `?mode=template` (редактируем шаблон) или `?mode=override` (редактируем переопределения субаккаунта).
- **`/tariffs`** (client, любая аутентифицированная роль) — переиспользует компонент матрицы в readonly-режиме, показывает эффективную сетку текущего субаккаунта. Нет dropdown'а субаккаунтов, нет кнопок правки.

Redirect со старых URL с query-табом: `?tab=overview` → `/network/tariffs`, `?tab=templates` → `/network/tariffs/templates`, `?tab=overrides` → `/network/tariffs` (без прямого аналога — ведёт на список).

### 1.2 Shared component

`<TariffMatrix>` в `portal-frontend/src/components/tariffs/TariffMatrix.tsx`.

Props:
- `scope: { kind: 'template', templateId } | { kind: 'override', subAccountId } | { kind: 'client' }`
- `editable: boolean`
- `showInheritance: boolean` (true для override-режима, false для template и client)
- `onSave?: (batch) => Promise<void>`

Внутри: вся логика источников, edit-mode, валидация, сабмит batch'а. Наружу — чистый UI-компонент без знания о роутинге.

### 1.3 Удаляется

`portal-frontend/src/pages/network/NetworkTariffsPage.tsx` теряет tabs-логику, превращается в wrapper над списком субаккаунтов.
Удаляются: `TariffOverviewTab.tsx`, `TariffTemplatesTab.tsx`, `TariffOverridesTab.tsx`, `TariffPlanEditor.tsx`.
Клиентский `TariffsPage.tsx` переводится на `<TariffMatrix scope={{kind:'client'}} editable={false} />`.

## 2. `/network/tariffs` — списочный экран

### 2.1 Layout

- Заголовок «Тарифы субаккаунтов» + кнопка-ссылка справа «Шаблоны →».
- Строка фильтров: поиск по имени (debounced 300ms, клиент-сайд) + чекбокс «только с переопределениями».
- Таблица:

| Субаккаунт | Шаблон | Переопр. | Средняя ₽/SMS | → |
|---|---|---|---:|---|
| Test Heavy (C)<br/>heavy@test.local | `Базовый тариф` (badge sky, клик → editor шаблона) | `3 правила` (badge amber, клик → editor субаккаунта, scroll к первому override) | 2.40 | Открыть ▸ |

- Клик по строке/«Открыть» → `/editor/:sub_account_id?mode=override`.
- Клик по бейджу шаблона → `/editor/:template_id?mode=template` (stopPropagation).
- Сортировка: по имени (default, asc), по средней цене, по кол-ву переопределений.
- Пустое состояние: «Нет субаккаунтов. Добавьте клиента в разделе "Суб-аккаунты".»
- Skeleton при загрузке; inline-alert с retry при ошибке.

### 2.2 Data

Один запрос: `GET /api/network/tariffs/subaccounts-summary` → массив из секции 5.

## 3. `/network/tariffs/templates` — библиотека

### 3.1 Layout

- Заголовок «Шаблоны тарифов» + primary-кнопка `+ Создать шаблон`.
- Таблица:

| Имя | Описание | Планов | Привязано | Действия |
|---|---|---:|---:|---|
| Базовый тариф | Тестовый шаблон тарифов | 3 | 5 | ⋮ |

- Клик по имени → `/editor/:template_id?mode=template`.
- Меню `⋮`: «Привязать…», «Дублировать», «Удалить».
- Удаление при `bound_count > 0` — кнопка disabled + tooltip «Сначала отвяжите шаблон от N субаккаунтов».
- Пустое состояние: иллюстрация + «Создайте первый шаблон…» + кнопка.

### 3.2 Модалки

**Создать шаблон:**
- Поля: имя (required, unique per reseller), описание (optional).
- Чекбокс «Скопировать из…» → dropdown существующих шаблонов. При copy — создаются те же планы/периоды/tiers, без привязок к субаккаунтам.
- Submit → `POST /api/network/tariff-templates` → редирект на `/editor/:new_id?mode=template`.

**Привязать:**
- Список субаккаунтов с чекбоксами + поиск.
- Если у субаккаунта уже есть шаблон — чекбокс помечается бейджем «Заменит Y»; inline-warning «Переопределения субаккаунта сохранятся».
- Submit → bulk `POST /api/network/tariff-templates/:id/bind` с массивом `sub_account_ids`.

**Дублировать:**
- Поле «Новое имя» (prefilled «Копия Базовый тариф»).
- Submit → клонирует шаблон со всеми планами/периодами/tiers, не трогает привязки.

**Удалить:** стандартная confirm-модалка.

## 4. `/network/tariffs/editor/:id` — матричный редактор

### 4.1 Режимы

- `?mode=template` — `:id` = template_id. Заголовок `Базовый тариф`. Матрица = чистые цены шаблона. Нет концепта inheritance.
- `?mode=override` — `:id` = sub_account_id. Заголовок `Test Heavy (C) · шаблон: Базовый тариф`. Матрица показывает эффективные цены с маркерами источника.

### 4.2 Top bar (sticky)

- Breadcrumbs: `Тарифы › <имя>`.
- Tabs каналов: `SMS · HLR · Viber · Voice · …`. Переключение → `?channel=sms`.
- Фильтры: `Страна ▾` · `Тип имени ▾` (sender_category) · `Тип трафика ▾` (traffic_type) · `Период ▾` (все периоды активного плана + пункт `+ Новый период`).
- Справа: `Стратегия: threshold ▾` (принадлежит плану; изменение стратегии — отдельная подтверждающая модалка из-за recalc рисков).
- Кнопки действия (правый край):
  - Read-mode: `Режим правки` (в override-режиме: `Режим правки (3 переопр.)`).
  - Edit-mode: `Сохранить` (primary, disabled при отсутствии изменений) + `Отмена`.

### 4.3 Матрица

- Строки: операторы активного канала/страны (с иконками).
- Колонки: tiers плана по `from_quantity` (`0+`, `1000+`, `10k+`, `50k+`).
- Ячейки — state machine:
  - **template-mode, set:** число в текущей валюте.
  - **template-mode, unset:** `—` серым (в edit — click adds).
  - **override-mode, inherited:** серое число italic, tooltip `Из шаблона Базовый`. В edit-mode клик → ячейка становится override (акцентная рамка + белый фон), появляется `×` для revert.
  - **override-mode, overridden:** жирное число на `bg-amber-50`, маркер `●` слева, бейдж-tooltip `Переопределение`. В edit-mode `×` убирает override (возврат к inherited).
  - **override-mode, no template & no override:** `—` серым; в edit — клик создаёт override.
- Группы строк (если в плане несколько `sender_category`/`traffic_type` слотов на одной странице) — разделяются подзаголовком, не разными таблицами.

### 4.4 Период-level действия

Под матрицей:
- `+ Добавить период` → модалка:
  - Поля: `Начало` (date, default = сегодня), `Конец` (date, optional = бессрочно), `Скопировать цены из` (dropdown существующих периодов этого плана; «Не копировать»), radio `Оставить tiers выбранного периода` vs `Новые tiers` (дефолт — оставить).
  - Валидация: не пересекается с существующими периодами этого плана.
- `Удалить период` → confirm. Нельзя удалить единственный период активного плана.

### 4.5 Edit mode

- Вход по кнопке `Режим правки`. Сбрасывается на выход со страницы без сохранения (с confirm).
- Inline-input в каждой ячейке: `<input type="text" inputmode="decimal">` с locale-aware парсингом (RU: запятая → точка).
- Валидация inline:
  - < 0 или не-число: красная рамка + tooltip «Некорректная цена».
  - > 999: yellow warning, но не блокирует (бизнес-решение).
- Keyboard: `Enter` → вниз, `Tab` → вправо, `Shift+Tab` → влево, `Esc` → откат значения ячейки к исходному.
- Footer (sticky внизу): `Несохранённых изменений: N · [Сохранить] [Отмена]`. `role="status"` для а11y.
- Сохранение → batch `PATCH /api/network/tariff-plans/:plan_id/bulk` (section 5.1.6). Транзакция на сервере. При успехе — выход из edit-mode, toast «Изменения сохранены». При ошибке валидации — inline на проблемных ячейках, edit-mode остаётся активен.
- Навигация с несохранёнными изменениями → `beforeunload` + React Router blocker.

### 4.6 Создание override (safety)

Override создаётся только если пользователь ввёл **значение, отличное от inherited**. Просто клик по unlocked-ячейке без ввода — не создаёт запись. Защита от случайных override.

### 4.7 Empty states

- Новый шаблон без планов: центральный call-to-action «У шаблона ещё нет планов. Выберите канал/страну/тип и добавьте первый период.» + кнопка `+ Создать план` (открывает модалку с dropdown'ами измерений + автосоздание одного бессрочного периода).
- Субаккаунт без привязанного шаблона и без overrides: «Субаккаунт работает на legacy-тарифах. Привяжите шаблон в разделе "Шаблоны".»

## 5. API

Все пути в `cmd/portal-frontend` под `/api/network/*`; gRPC бэкенд — существующий `tariffv1`.

### 5.1 Endpoints

**5.1.1 `GET /api/network/tariffs/subaccounts-summary`**
```json
[
  {
    "sub_account_id": "uuid",
    "sub_account_name": "Test Heavy (C)",
    "sub_account_email": "heavy@test.local",
    "template_id": "uuid|null",
    "template_name": "Базовый тариф|null",
    "override_count": 3,
    "avg_price_per_sms": 2.40,
    "currency": "RUB"
  }
]
```
Сервер вычисляет `avg_price_per_sms` как простое среднее первой tier всех `(RU × платные × операторы)` из эффективной сетки. Cache Redis 5 мин по ключу `tariffs:summary:<reseller_id>`.

**5.1.2 `GET /api/network/tariff-templates`**
```json
[{"id":"uuid","name":"Базовый тариф","description":"...","plans_count":3,"bound_subaccount_count":5,"created_at":"2026-04-16T..."}]
```

**5.1.3 `POST /api/network/tariff-templates`** — body `{name, description, copy_from_id?}` → `{id}`. Валидация: имя unique per reseller.

**5.1.4 `POST /api/network/tariff-templates/:id/bind`** — body `{sub_account_ids: [uuid]}` → `{bound:[uuid], replaced:[{sub_account_id, old_template_id}]}`.

**5.1.5 `GET /api/network/tariff-editor/:id`**
Query: `mode=template|override&channel=sms&country=RU&sender_category=paid&traffic_type=any&period_id=<uuid>` (period_id опционален — default активный).
Response:
```json
{
  "scope": {"kind":"override","sub_account_id":"uuid","sub_account_name":"..."},
  "template": {"id":"uuid","name":"Базовый тариф"},
  "plan": {"id":"uuid","strategy":"threshold","currency":"RUB"},
  "periods": [{"id":"uuid","from":"2026-05-01","to":null,"active":true}],
  "active_period_id": "uuid",
  "operators": [{"id":"uuid","name":"МТС","icon":"mts"}],
  "tiers": [{"id":"uuid","from_quantity":0},{"id":"uuid","from_quantity":1000}],
  "cells": [
    {"operator_id":"uuid","tier_id":"uuid","price_template":3.20,"price_override":null,"effective":3.20,"source":"template"},
    {"operator_id":"uuid","tier_id":"uuid","price_template":3.60,"price_override":3.20,"effective":3.20,"source":"override"}
  ]
}
```

**5.1.6 `PATCH /api/network/tariff-plans/:plan_id/bulk`**
Body:
```json
{
  "period_id": "uuid",
  "tiers_upsert": [{"id":"uuid|null","from_quantity":5000}],
  "tiers_delete": ["uuid"],
  "cells_upsert": [
    {"operator_id":"uuid","tier_id":"uuid","price":3.15,"scope":"template|override","sub_account_id":"uuid|null"}
  ],
  "cells_delete": [{"operator_id":"uuid","tier_id":"uuid","scope":"override","sub_account_id":"uuid"}]
}
```
Response: `{ok:true}` или `{ok:false, errors:[{operator_id, tier_id, reason}]}`. Транзакция; при частичной ошибке — rollback всего batch'а.

**5.1.7 `POST /api/network/tariff-plans/:plan_id/periods`** — body `{from, to?, copy_from_period_id?, keep_tiers:bool}` → `{id}`. Валидация пересечений на сервере.

**5.1.8 `GET /api/client/tariffs/effective`**
Query: `channel=sms&country=RU&sender_category=paid&traffic_type=any`. Скопирован под текущую сессию клиента.
Response — та же структура что 5.1.5, но без поля `price_template`, без поля `source` (только `effective`), и без period-навигации (возвращает только активный период).

### 5.2 Permissions

- Все `/api/network/*` → требуют роли `reseller_admin` в middleware.
- `/api/client/tariffs/effective` → требует `authenticated` + resolved `sub_account_id` из session context.

### 5.3 Counters

Для быстрого `/tariff-templates` нужны `plans_count`, `bound_subaccount_count`. Два варианта (решаем на этапе плана):
- **Counter columns** на `reseller_tariff_templates` + триггеры `AFTER INSERT/DELETE` на `reseller_tariff_plans` и `reseller_tariff_template_bindings`.
- **On-read COUNT'ы** в запросе с `LATERAL` подзапросами. Дешевле в миграции, дороже в рантайме.

Default — on-read; переход на counter columns только при p95 > 200ms.

## 6. Accessibility

Таргет — WCAG 2.1 AA.

- Все `▶/▼` раскрывашки, бейджи-кнопки — `<button>` с `aria-expanded`/`aria-label`.
- Матрица — `<table>` с `<caption class="sr-only">` (`Тарифная сетка: Базовый тариф, канал SMS, страна RU, платные`), `<th scope="row">` для колонки операторов, `<th scope="col">` для ступеней.
- Ячейки с overrides:
  - Визуальный маркер — цвет фона + иконка `●` + жирность.
  - Screen-reader: `aria-label="3.20 рубля, переопределение поверх значения 3.60 из шаблона Базовый"`.
- Источник НЕ передаётся только цветом (inherited/override) — добавлен `<span class="sr-only">` с текстом и графический маркер (●).
- Контрасты:
  - `bg-amber-50` (override fill) с текстом `slate-900` — OK (≥ 7:1).
  - Inherited grey `text-slate-500` на `bg-white` — OK (≥ 4.5:1).
  - Маркер `●` в `amber-600` — OK (≥ 3:1 vs белый фон).
- Edit-mode footer: `role="status"` с live-update счётчика изменений.
- Модалки: Radix `<Dialog>` (уже в проекте) — focus-trap, Esc, aria-атрибуты из коробки.
- Keyboard-нав по матрице: Tab между ячейками в edit-mode; в read-mode — `Tab` между интерактивными элементами (edit button, filter dropdowns, ссылки на шаблон).

## 7. Визуальная система

- Стек без изменений: Tailwind 4 + Radix.
- Цвета:
  - Бейдж шаблона: `bg-sky-100 text-sky-800`.
  - Бейдж переопределения: `bg-amber-100 text-amber-800`.
  - Ячейка override (fill): `bg-amber-50`, текст `slate-900`, font-weight 600.
  - Ячейка inherited: `text-slate-500 italic`.
  - Legacy-бейдж: `bg-slate-200 text-slate-700` (сохраняется из текущего UI до её удаления legacy-тарифов).
- Иконки операторов: используем существующий набор в проекте (`portal-frontend/src/assets/operators/` — уточнить на этапе плана); fallback — инициал оператора в `rounded-full bg-slate-200`.
- Плотность матрицы: `py-1.5` (compact), `text-sm`.
- Модалки: Radix `<Dialog>` с размерами `sm` (простые формы) / `lg` (Привязать с multiselect).

## 8. Risks и tradeoffs

1. **Cache средней цены на списке.** Изменение шаблона/override не инвалидирует кеш немедленно → в списке может быть устаревшая цена до 5 мин. Приемлемо, но нужен ручной «Обновить» в UI + invalidate в handler'е `PATCH bulk`.
2. **Удаление старых URL с tabs query.** Сохранённые закладки/ссылки ломаются. Redirect'ы решают частично, но `?tab=overrides` не имеет прямого аналога — ведём на список.
3. **Клик по inherited-ячейке в edit-mode.** Риск случайного override. Защита §4.6: override создаётся только при вводе отличного значения. Тестить UX с реальным админом.
4. **Изменение стратегии плана** (`threshold → fixed` и т.п.) — может обесценить существующие tiers. Вынесено в подтверждающую модалку. В этом спеке не прорабатываем детально — отдельный scenario.
5. **Одна «средняя ₽/SMS»** в списке — усреднение скрывает разброс. Альтернатива (min–max) — отложили как avg-to-improve.

## 9. Open questions

Все уточняются на этапе плана, не блокируют спек.

- Серверный компонент `avg_price_per_sms` — простое среднее или взвешенное по истории трафика? Влияет на сложность backend.
- Counter columns vs on-read COUNT для `plans_count` / `bound_count`.
- Точный набор иконок операторов — есть в проекте или нужно добавить.
- Мобильная адаптация матрицы — отдельная ветка (сейчас предполагаем desktop-first 1280+).
