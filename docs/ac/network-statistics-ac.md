# Acceptance Criteria: Network Statistics / Analytics / Monitoring

**Spec:** `docs/superpowers/specs/2026-04-17-network-statistics-analytics-monitoring-design.md`
**Date:** 2026-04-18
**Batch:** 1 / N (filter behavior)

---

## Что такое Acceptance Criteria

**Acceptance Criteria (AC, критерии приёмки)** — это проверяемые сценарии поведения фичи. Каждый AC имеет три обязательных части:

- **ДАНО** — конкретное состояние системы до действия (числа, данные, URL, активная вкладка)
- **КОГДА** — одно действие пользователя или события
- **ТОГДА** — наблюдаемый результат с конкретными значениями (URL, количество элементов, содержимое, CSS-класс)

**Правила:**
- AC без одной из трёх частей → не тестируем → не принимается
- "ТОГДА: работает корректно" → не тестируем (субъективно)
- "ТОГДА: 10 строк с status=sent" → тестируем (конкретно)
- Один AC = одно действие = один тест

Каждый AC здесь становится одним Playwright E2E тестом на этапе реализации.

---

## Условные обозначения

- `URL` — текущий URL страницы целиком
- `Таблица` — основная таблица статистики (`StatisticsTable`)
- `KPI` — полоска KPI карточек (`StatisticsKPIStrip`)
- `Apply` — кнопка "Применить" в конце Filter Bar
- `Пред-данные` — seed-данные для теста, создаются через API в `beforeEach`

---

## Batch 1: Filter Behavior

### Группа A: Кнопка "Применить" — ядро поведения

#### AC-01: Фильтры НЕ применяются автоматически

**ДАНО:**
- Пользователь на странице `/network/statistics?mode=stats`
- В таблице 10 строк (любые)
- Кнопка "Применить" в состоянии disabled (нет изменений)

**КОГДА:** пользователь выбирает в dropdown "Оператор" значение "MTS"

**ТОГДА:**
- URL НЕ изменился (всё ещё `/network/statistics?mode=stats`)
- Таблица показывает те же 10 строк (сетевой запрос НЕ ушёл)
- Кнопка "Применить" становится enabled
- В dropdown "Оператор" отображается выбранное "MTS"

---

#### AC-02: Нажатие "Применить" применяет все изменения

**ДАНО:**
- Пользователь на странице `/network/statistics?mode=stats`
- В dropdown выбраны: Оператор="MTS", Статус="delivered"
- Кнопка "Применить" enabled
- Запрос к API ещё не уходил

**КОГДА:** пользователь нажимает "Применить"

**ТОГДА:**
- URL становится `/network/statistics?mode=stats&operator=mts&status=delivered`
- Уходит ровно один GET-запрос на `/portal/v1/network/statistics` с query `operator=mts&status=delivered`
- Таблица обновляется данными из ответа
- Кнопка "Применить" возвращается в disabled (нет новых изменений)

---

#### AC-03: Двойной клик "Применить" не дублирует запрос

**ДАНО:**
- Активны фильтры Оператор="MTS", кнопка "Применить" enabled
- Предыдущий запрос не завершён (имитируется задержкой сети 2 сек)

**КОГДА:** пользователь жмёт "Применить" дважды в течение 300 мс

**ТОГДА:**
- Уходит ровно один сетевой запрос на `/portal/v1/network/statistics`
- Повторный запрос не дублируется даже при продолжении кликов
- Кнопка "Применить" в состоянии loading (disabled + spinner) во время запроса

---

### Группа B: Быстрые фильтры (Ряд 2 Filter Bar)

#### AC-04: Фильтр "Логин" работает

**ДАНО:**
- Пред-данные: 30 строк с распределением login={client1: 10, client2: 15, client3: 5}
- Пользователь на странице без фильтров, таблица показывает все 30 строк

**КОГДА:** пользователь вводит в поле "Логин" значение "client2" и нажимает "Применить"

**ТОГДА:**
- URL содержит `&login=client2`
- Таблица содержит 15 строк
- Все строки в колонке "Срез" (или detail modal) имеют login="client2"
- KPI "Всего" показывает значение 15

---

#### AC-05: Фильтр "Оператор" работает

**ДАНО:**
- Пред-данные: 25 строк с распределением operator={mts: 10, beeline: 10, tele2: 5}
- Таблица без фильтров, 25 строк

**КОГДА:** пользователь выбирает в dropdown "Оператор" значение "Билайн" и нажимает "Применить"

**ТОГДА:**
- URL содержит `&operator=beeline`
- Таблица содержит 10 строк
- Все строки имеют operator="beeline" (видно в drill-down)

---

#### AC-06: Фильтр "Канал" работает

**ДАНО:**
- Пред-данные: 20 строк с распределением channel={sms: 12, viber: 5, whatsapp: 3}

**КОГДА:** пользователь выбирает Канал="Viber" и нажимает "Применить"

**ТОГДА:**
- URL содержит `&channel=viber`
- Таблица содержит 5 строк
- KPI обновились на основе этих 5 строк

---

### Группа C: Дополнительные фильтры (свёрнутый блок)

#### AC-07: Блок "Ещё фильтры" свёрнут по умолчанию

**ДАНО:** пользователь открывает страницу `/network/statistics` впервые (localStorage пуст)

**КОГДА:** страница полностью загружена

**ТОГДА:**
- В Filter Bar отображаются только Ряд 1 и Ряд 2
- Поля "Имя отправителя", "Провайдер", "Страна" и т.д. НЕ видны в DOM
- Кнопка "Ещё фильтры" показывает иконку `Plus` (не `Minus`/`X`)

---

#### AC-08: Блок "Ещё фильтры" разворачивается по клику

**ДАНО:** блок "Ещё фильтры" свёрнут (AC-07)

**КОГДА:** пользователь нажимает кнопку "Ещё фильтры"

**ТОГДА:**
- Появляется блок с полями: Имя отправителя, Тип трафика, Статус, Провайдер, Страна, Менеджер, Код ошибки (7 полей)
- URL НЕ меняется (раскрытие чисто UI, не state)

**Источник истины:** [StatisticsFilterBar.tsx:278-301](../../portal-frontend/src/components/network-stats/StatisticsFilterBar.tsx#L278-L301).

**Правка 2026-04-21:** Изначальная формулировка (11 полей: Платность, Международное, Цена, Способ, Ошибка) — по черновику спеки. Фактически в коде 7 полей, иконка кнопки `Plus` не меняется (StatisticsFilterBar.tsx:262-267 — `<Plus>` рендерится безусловно).

---

#### AC-09: Фильтр "Провайдер" работает

**ДАНО:**
- Блок "Ещё фильтры" развёрнут
- Пред-данные: 40 строк с провайдерами {prov1: 20, prov2: 15, prov3: 5}

**КОГДА:** пользователь вводит в поле "Провайдер" значение "prov2" и нажимает "Применить"

**ТОГДА:**
- URL содержит `&provider=prov2`
- Таблица содержит 15 строк

---

### Группа D: Период

#### AC-10: Preset "Сегодня" при первом заходе

**ДАНО:**
- Пред-данные: строки с датами в диапазоне [сегодня 00:00 — текущий момент]: 5 штук; вчерашних: 20 штук

**КОГДА:** пользователь открывает `/network/statistics` без query-параметров

**ТОГДА:**
- URL **остаётся** `/network/statistics` без query — `setSearchParams` не вызывается при первичном render (useNetworkStats.ts:59-63 — только `useSearchParams()`, без записи)
- В Filter Bar активен preset "7 дней" (visual highlight на основе state-дефолта из `parseFiltersFromURL`, useNetworkStats.ts:43-45)
- Ушёл `GET /portal/v1/reseller/statistics?period_preset=7d&group_by=day&...` (дефолты в запросе есть, хотя в URL их нет)
- Таблица содержит строки за последние 7 дней (≥ 25 строк в нашем примере)

**Правка 2026-04-21:** Исходная формулировка "URL автоматически становится `?mode=stats&period_preset=7d`" — неверна. По коду [useNetworkStats.ts:145,177,342](../../portal-frontend/src/hooks/useNetworkStats.ts#L145) `setSearchParams` вызывается только при `setMode()`, `applyFilters()` и `loadView()` — не при init. URL наполняется query только после первого действия пользователя. Эта разница важна: deep-link без query не идентичен `?period_preset=7d` после применения.

---

#### AC-11: Смена preset на "Сегодня" меняет данные

**ДАНО:**
- Активный preset "7 дней", в таблице 25 строк

**КОГДА:** пользователь кликает preset "Сегодня" и нажимает "Применить"

**ТОГДА:**
- URL содержит `&period_preset=today`
- В таблице 5 строк (только сегодняшние, см. AC-10)
- Preset "Сегодня" визуально активен, preset "7 дней" неактивен

---

#### AC-12: Произвольный диапазон через date picker

**ДАНО:**
- Активный preset "7 дней"
- Пред-данные: 30 строк с датами 2026-04-01 ... 2026-04-18, равномерно

**КОГДА:** пользователь открывает date picker, выбирает диапазон 2026-04-10 — 2026-04-15 и нажимает "Применить"

**ТОГДА:**
- URL содержит `&date_from=2026-04-10&date_to=2026-04-15` и НЕ содержит `period_preset`
- Все preset-кнопки визуально неактивны
- В таблице строки только с датами из выбранного диапазона (включительно, 6 дней)

---

### Группа E: Группировка

#### AC-13: Группировка "по дням" — дефолт для Статистики

**ДАНО:** пользователь на режиме Статистика, период "7 дней", пред-данные за 7 дней

**КОГДА:** страница загружена, изменений фильтров не было

**ТОГДА:**
- URL содержит `&group_by=day`
- Таблица содержит 7 строк (по одной на день периода)
- Колонка "Срез" содержит даты в формате YYYY-MM-DD

---

#### AC-14: Валидация: группировка "по 5 минут" недопустима для периода > 24 часа

**ДАНО:**
- Активный preset "7 дней"
- Группировка в dropdown = "по дням"

**КОГДА:** пользователь выбирает в dropdown группировки "по 5 минут" и нажимает "Применить"

**ТОГДА:**
- Сетевой запрос возвращает HTTP **500** (не 400) с gRPC-кодом `Internal` (см. D-12 в `_DRIFT.md`)
- Тело ответа содержит message вида `rpc error: code = Internal desc = get statistics: period of 168 hours exceeds the maximum of 24 hours allowed for group_by="5min"`
- В UI появляется красный toast в правом нижнем углу с generic текстом (как правило, message из ответа — useNetworkStats.ts:118-120). Русского локализованного сообщения "доступна только для периода до 24 часов" сейчас нет
- URL **меняется** на новый (`applyFilters()` вызывается до запроса — useNetworkStats.ts:176-178), то есть `?group_by=5min` записывается даже при фейле
- Таблица пустеет или показывает спиннер до ошибки, затем остаётся пустой

**Правка 2026-04-21 + drift D-12:** Исходная формулировка "HTTP 400 INVALID_ARGUMENT + локализованный toast" описывала **желаемое** поведение, не фактическое. Validation существует ([service.go:34-55](../../internal/services/network_analytics/application/service.go#L34-L55)), но [grpc/server.go:35](../../internal/services/network_analytics/grpc/server.go#L35) оборачивает любую ошибку в `codes.Internal`, поэтому клиент не различает validation vs infra-ошибку. Фикс: sentinel-error в validateFilter + маппинг в `codes.InvalidArgument`, отдельная ветка в HTTP-хендлере + русский toast.

---

### Группа F: Комбинирование и сброс

#### AC-15: Несколько фильтров применяются одновременно (AND)

**ДАНО:**
- Пред-данные: 100 строк, из них 20 имеют (operator=mts AND status=delivered), остальные — любые другие комбинации

**КОГДА:** пользователь выбирает Оператор="MTS" И Статус="delivered" и нажимает "Применить"

**ТОГДА:**
- URL содержит `&operator=mts&status=delivered`
- Таблица содержит ровно 20 строк
- KPI "Всего" = 20

---

#### AC-16: Открытие страницы по URL с фильтрами применяет их

**ДАНО:** пользователь копирует URL `/network/statistics?mode=stats&operator=mts&status=delivered&period_preset=7d` и открывает в новом окне

**КОГДА:** страница полностью загружена

**ТОГДА:**
- Активная вкладка "Статистика"
- В dropdown "Оператор" выбрано "MTS"
- В dropdown "Статус" выбрано "delivered"
- Preset "7 дней" визуально активен
- Таблица уже отфильтрована (сетевой запрос ушёл автоматически с этими параметрами)
- Кнопка "Применить" в disabled (нет новых изменений)

---

#### AC-17: Переключение между вкладками сохраняет фильтры

**ДАНО:**
- Пользователь на вкладке "Статистика"
- Применены фильтры Оператор="MTS", Статус="delivered"
- URL = `/network/statistics?mode=stats&operator=mts&status=delivered`

**КОГДА:** пользователь кликает вкладку "Аналитика"

**ТОГДА:**
- URL становится `/network/statistics?mode=analytics&operator=mts&status=delivered`
- В Filter Bar всё ещё выбраны Оператор="MTS" и Статус="delivered"
- Уходит запрос на `/portal/v1/network/analytics` с теми же фильтрами
- KPI и чарт аналитики отображаются для этого фильтра

---

### Группа G: Wiring вкладок к API (дописано на основе практики)

#### AC-18: Каждая вкладка вызывает свой API-endpoint

**ДАНО:**
- Пользователь на `/network/statistics`, активная вкладка "Статистика"
- Страница полностью загружена

**КОГДА:** пользователь последовательно кликает вкладки "Аналитика" → "Мониторинг" → "Статистика"

**ТОГДА:**
- Клик "Аналитика" вызвал запрос на `/portal/v1/reseller/analytics-summary`
- Клик "Мониторинг" вызвал запрос на `/portal/v1/reseller/monitoring`
- Клик "Статистика" вызвал запрос на `/portal/v1/reseller/statistics`
- URL меняет только `mode=` параметр, остальные фильтры сохранены

**Обоснование:** добавлено после обнаружения drift в AC-тестах: spec говорит `/network/...`, фактический API `/reseller/...`. Этот AC фиксирует реальный контракт и ловит, если вкладка "перепутает" endpoint.

---

## Итого в Batch 1

- **18 AC** покрывают: Apply button, 3 быстрых фильтра, 1 дополнительный фильтр, свёртка блока, 3 preset'а периода, дефолтная группировка + валидация, комбинирование, URL sync, переключение вкладок с сохранением фильтров, wiring вкладок к API.
- **Не покрыто в этом batch** (будут в следующих):
  - Остальные 10 дополнительных фильтров (аналогичный AC-09 шаблон)
  - Остальные 5 period preset'ов (аналогичный AC-11 шаблон)
  - Остальные 10 группировок + валидационные таблицы
  - Drill-down
  - Сортировка/колонки/пагинация
  - Экспорт
  - Saved views
  - Режимы Аналитики и Мониторинга (live-индикатор, polling)
  - Empty/error/loading состояния
  - Подсветка и conditional formatting

## Критика своего первого батча

1. **AC-10 "Preset Сегодня при первом заходе" противоречит спеке.** В URL-примере (§1) стоит `period=7d`, но default preset может быть иной. Перед генерацией теста — подтвердить. Это ровно тот случай, когда AC вскрывает неоднозначность в спеке.

2. **AC-07/08 ("свёртка блока") — состояние не в URL.** Это UI-state, не модель. Значит, при refresh страницы он сбросится. Возможно, нужно было сохранять в localStorage — в спеке явно не сказано. Флаг.

3. **AC-14 (валидация группировки) предполагает backend-валидацию.** Спека §2.5 подтверждает: сервер вернёт `INVALID_ARGUMENT`. Но UI должен показать читаемый toast. Это граница: тест проверяет и backend (HTTP 400), и frontend (toast). Можно разделить на два AC для чистоты.

4. **Селекторы в AC не упомянуты.** Это осознанное решение: AC описывает поведение на уровне пользователя, не на уровне DOM. Селекторы попадут в тест, не в AC.

5. **17 AC покрывают < 25% batch "filter behavior".** Остальные ~40 — через копирование шаблона.

---

## Batch 2: Statistics Table (колонки, форматирование, сортировка, пагинация)

**Scope batch'а:** поведение основной таблицы режима "Статистика" после того, как фильтры применены и данные получены. Содержит: количество и идентичность колонок, conditional formatting, клик-drilldown, сортировка, пагинация, состояния loading/empty, строка "Итого".

**Источники истины (верифицировано 2026-04-21):**
- [portal-frontend/src/components/network-stats/StatisticsTable.tsx](../../portal-frontend/src/components/network-stats/StatisticsTable.tsx)
- [portal-frontend/src/pages/network/NetworkStatisticsPage.tsx](../../portal-frontend/src/pages/network/NetworkStatisticsPage.tsx) (блок "Показано X–Y из N", onRowClick)
- [portal-frontend/src/hooks/useNetworkStats.ts](../../portal-frontend/src/hooks/useNetworkStats.ts) (reset page=1, openDrillDown)

### Группа H: Колонки и идентичность таблицы

#### AC-19: Ровно 12 колонок в шапке таблицы Статистики

**ДАНО:**
- Mode=stats, фильтры применены, в ответе ≥1 строка
- Таблица отрендерена

**КОГДА:** осмотр шапки таблицы

**ТОГДА:**
- В `<thead>` ровно **12** `<th>` с заголовками по порядку: "Срез", "Всего", "Достав.", "Не достав.", "Ожидание", "Таймаут", "Ошибки", "DLR%", "Выручка", "Прибыль", "Маржа", "" (пустой заголовок колонки health-индикатора)
- Заголовок "Канал" как отдельная колонка ОТСУТСТВУЕТ (канал остаётся только в фильтре)

**Drift note:** спека §"13 колонок" (f-15) не соответствует коду. Правка — либо добавить 13-ю колонку, либо обновить спеку. До разрешения — AC по факту (12 колонок). Возможна отдельная drift-запись D-14 при необходимости.

---

#### AC-20: Срез в формате ISO-даты обрезается до 10 символов

**ДАНО:**
- Mode=stats, `group_by=day`, есть строка с `slice="2026-04-15T00:00:00Z"` (или `"2026-04-15 00:00:00"`)
- Таблица отрендерена

**КОГДА:** просмотр первой ячейки "Срез"

**ТОГДА:**
- Текст ячейки — ровно "2026-04-15" (первые 10 символов), без времени и timezone
- Стиль: `text-blue-600 font-medium`

**Источник:** [StatisticsTable.tsx:39](../../portal-frontend/src/components/network-stats/StatisticsTable.tsx#L39) — regex `/^\d{4}-\d{2}-\d{2}/.test(s)` → `s.slice(0, 10)`.

---

#### AC-21: Срез не-ISO (например имя провайдера) отображается целиком

**ДАНО:**
- Mode=stats, `group_by=provider`, есть строка с `slice="mts"`

**КОГДА:** просмотр ячейки "Срез"

**ТОГДА:**
- Текст ячейки — ровно "mts" (не обрезается)
- Стиль тот же: `text-blue-600 font-medium`

---

### Группа I: Conditional formatting

#### AC-22: DLR% < 80% подсвечивается красным

**ДАНО:**
- Пред-данные: строка с `total=100, delivered=75` → `dlr_rate=0.75`

**КОГДА:** таблица отрендерилась

**ТОГДА:**
- Ячейка "DLR%" содержит текст "75.0%"
- Ячейка имеет класс `text-red-600 font-medium`

**Источник:** [StatisticsTable.tsx:18-22](../../portal-frontend/src/components/network-stats/StatisticsTable.tsx#L18-L22).

---

#### AC-23: DLR% 80.0–89.9% подсвечивается amber; ≥90% — emerald

**ДАНО:**
- Пред-данные: строка A с `dlr_rate=0.85`; строка B с `dlr_rate=0.95`

**КОГДА:** таблица отрендерилась

**ТОГДА:**
- Строка A: ячейка DLR% содержит "85.0%", класс `text-amber-600`
- Строка B: ячейка DLR% содержит "95.0%", класс `text-emerald-600`

---

#### AC-24: Строка с `health="danger"` имеет красный фон

**ДАНО:**
- Пред-данные: строка с `health="danger"` (например, через искусственно заниженный DLR)

**КОГДА:** таблица отрендерилась

**ТОГДА:**
- `<tr>` имеет класс `bg-red-50` (и при hover — `bg-red-100`)
- Последняя ячейка (health-индикатор) содержит элемент с классом `bg-red-600 w-2 h-2 rounded-full`
- Остальные строки с `health="warning"` имеют `hover:bg-gray-50` и dot `bg-amber-500`; `health="ok"` — dot `bg-emerald-600`

**Источник:** [StatisticsTable.tsx:28-36](../../portal-frontend/src/components/network-stats/StatisticsTable.tsx#L28-L36).

---

#### AC-25: Прибыль < 0 красная; ≥ 0 emerald

**ДАНО:**
- Пред-данные: строка A с `profit=-100`; строка B с `profit=+500`

**КОГДА:** таблица отрендерилась

**ТОГДА:**
- Строка A: ячейка "Прибыль" содержит "-100 ₽" (форматирование `toLocaleString('ru-RU')` + " ₽"), класс `text-red-600 font-medium`
- Строка B: ячейка "Прибыль" содержит "500 ₽", класс `text-emerald-600`

---

#### AC-26: Ожидание > 100 — amber; ≤ 100 — обычный; 0 — текст "0"

**ДАНО:**
- Пред-данные: строка A `pending=5`, строка B `pending=150`, строка C `pending=0`

**КОГДА:** таблица отрендерилась

**ТОГДА:**
- Строка A: ячейка "Ожидание" содержит "5", без специального цвета
- Строка B: ячейка "Ожидание" содержит "150", класс `text-amber-600`
- Строка C: ячейка "Ожидание" содержит "0" (литерал), без окраски

**Источник:** [StatisticsTable.tsx:43](../../portal-frontend/src/components/network-stats/StatisticsTable.tsx#L43).

---

#### AC-27: Ошибки > 500 — красный; 1..500 — amber; 0 — "0"

**ДАНО:**
- Пред-данные: строка A `error=0`, строка B `error=50`, строка C `error=700`

**КОГДА:** таблица отрендерилась

**ТОГДА:**
- Строка A: "0", без цвета
- Строка B: "50", класс `text-amber-600`
- Строка C: "700", класс `text-red-600`

**Источник:** [StatisticsTable.tsx:45](../../portal-frontend/src/components/network-stats/StatisticsTable.tsx#L45).

---

### Группа J: Сортировка

#### AC-28: Клик по числовой колонке применяет сортировку немедленно

**ДАНО:**
- Mode=stats, данные показаны, сортировка не активна (`sort_by` отсутствует в URL)

**КОГДА:** пользователь кликает по заголовку "DLR%"

**ТОГДА:**
- Немедленно уходит GET-запрос на `/portal/v1/reseller/statistics` с `sort_by=dlr_rate&sort_dir=desc` в query (без ожидания кнопки "Применить")
- URL содержит `&sort_by=dlr_rate&sort_dir=desc`
- Повторный клик по тому же заголовку: уходит запрос с `sort_by=dlr_rate&sort_dir=asc`
- Третий клик: снова `desc`

**Источник:** [StatisticsTable.tsx:54-58](../../portal-frontend/src/components/network-stats/StatisticsTable.tsx#L54-L58) — `handleSort` вызывает `onApply()` сразу.

---

#### AC-29: Колонка Health не сортируется

**ДАНО:**
- Mode=stats, таблица отрендерена

**КОГДА:** пользователь кликает по шапке последней (пустой) колонки health-индикатора

**ТОГДА:**
- Запрос на `/portal/v1/reseller/statistics` НЕ уходит
- URL не изменяется, `sort_by`/`sort_dir` не добавляются
- Шапка не имеет иконки `ChevronsUpDown` (в отличие от других 11 колонок)

**Источник:** [StatisticsTable.tsx:90,94](../../portal-frontend/src/components/network-stats/StatisticsTable.tsx#L90) — `col.key !== 'health'` исключает колонку из handleSort и из рендера иконки.

---

### Группа K: Клик-drilldown

#### AC-30: Клик по строке таблицы открывает drill-down по провайдеру

**ДАНО:**
- Mode=stats, `group_by=provider`, есть строка с `slice="mts"`
- Drill-down drawer закрыт (`stats.drillDown === null`)

**КОГДА:** клик по `<tr>` строки

**ТОГДА:**
- Уходит GET-запрос `/portal/v1/reseller/drilldown` с параметрами `slice_type=provider&slice_value=mts&detail_view=operators&<активные фильтры>`
- После успешного ответа в DOM появляется drawer с `aria-label` или data-атрибутом открытого состояния (точный селектор — в Batch 5)
- Активная вкладка внутри drawer — `operators` (дефолт `drillDownView='operators'`)

**Источник:** [NetworkStatisticsPage.tsx:108](../../portal-frontend/src/pages/network/NetworkStatisticsPage.tsx#L108) — `onRowClick={(row) => stats.openDrillDown('provider', row.slice, row.slice)}`; [useNetworkStats.ts:183-195](../../portal-frontend/src/hooks/useNetworkStats.ts#L183-L195).

**Drift note:** `openDrillDown` вызывается с фиксированным `sliceType='provider'` независимо от текущей `group_by`. Т.е. если `group_by=operator`, клик по строке "mts" всё равно запустит `slice_type=provider&slice_value=mts`. Это bug — код не адаптирует sliceType под текущую группировку. Возможна отдельная drift-запись D-15 (после подтверждения).

---

### Группа L: Строка "Итого"

#### AC-31: Строка "Итого" считает агрегаты корректно

**ДАНО:**
- Пред-данные: 2 строки:
  - Строка 1: `total=100, delivered=90, failed=5, pending=3, timeout=2, error=0, revenue=1000, profit=200`
  - Строка 2: `total=200, delivered=150, failed=30, pending=15, timeout=5, error=0, revenue=2000, profit=300`

**КОГДА:** таблица отрендерилась

**ТОГДА:**
- В `<tfoot>` одна строка с текстом в первой ячейке "Итого" (bold)
- Значения по колонкам:
  - Всего: "300"
  - Достав.: "240"
  - Не достав.: "35"
  - Ожидание: "18"
  - Таймаут: "7"
  - Ошибки: "0"
  - DLR%: "80.0%" (= 240/300, `delivered/total`)
  - Выручка: "3 000 ₽" (с неразрывным пробелом от `toLocaleString('ru-RU')`)
  - Прибыль: "500 ₽"
  - Маржа: "16.7%" (= 500/3000, `profit/revenue`)
  - Health: пусто

**Источник:** [StatisticsTable.tsx:66-78,131-134](../../portal-frontend/src/components/network-stats/StatisticsTable.tsx#L66-L78).

---

#### AC-32: Строка "Итого" скрывается при пустом наборе строк

**ДАНО:**
- Пред-данные: фильтр без совпадений (API вернул `rows: []`)

**КОГДА:** таблица отрендерилась

**ТОГДА:**
- `<tfoot>` НЕ рендерится (условие `rows.length > 0`)
- Вместо таблицы — одна строка с текстом "Нет данных"

---

#### AC-33: Итого — divide-by-zero safe

**ДАНО:**
- Пред-данные: 1 строка `total=0, delivered=0, revenue=0, profit=0` (теоретически редкий случай, но возможен при фильтре по оператору без трафика)

**КОГДА:** таблица отрендерилась

**ТОГДА:**
- Ячейка "DLR%" в "Итого" содержит "—" (em-dash), не "NaN%" и не "0.0%"
- Ячейка "Маржа" в "Итого" содержит "—"

**Источник:** [StatisticsTable.tsx:131,134](../../portal-frontend/src/components/network-stats/StatisticsTable.tsx#L131) — `totals.total > 0 ? fmtPct(...) : '—'`.

---

### Группа M: Loading / Empty / Pagination

#### AC-34: Loading state при первом запросе — "Загрузка..."

**ДАНО:**
- Mode=stats, таблица ещё не получала данных (`rows=[]`, `loading=true`)

**КОГДА:** запрос в процессе

**ТОГДА:**
- `<tbody>` содержит одну строку с текстом "Загрузка..." (стиль `text-gray-400 py-12 text-center`)
- Заголовки таблицы видны
- `<tfoot>` не рендерится

**Источник:** [StatisticsTable.tsx:101-102](../../portal-frontend/src/components/network-stats/StatisticsTable.tsx#L101-L102).

---

#### AC-35: Loading поверх существующих строк не стирает таблицу

**ДАНО:**
- Mode=stats, таблица уже показывает 10 строк
- Пользователь нажимает "Применить" → новый запрос

**КОГДА:** запрос в процессе (state: `loading=true, rows=<старые 10>`)

**ТОГДА:**
- `<tbody>` продолжает показывать старые 10 строк (НЕ "Загрузка..." — условие `loading && !rows.length`)
- Кнопка "Применить" в Filter Bar в состоянии `disabled` с текстом "Загрузка..." (AC-02/AC-17 покрывают)

---

#### AC-36: Empty state — "Нет данных"

**ДАНО:**
- Mode=stats, запрос завершён, `rows=[]`, `loading=false`

**КОГДА:** таблица отрендерилась

**ТОГДА:**
- `<tbody>` содержит одну строку с `<td colspan="12">` и текстом "Нет данных" (стиль `text-gray-400 py-12 text-center`)
- `<tfoot>` не рендерится
- Блок пагинации (если он был) не рендерится

---

#### AC-37: Индикатор "Показано X–Y из N" при множественных страницах

**ДАНО:**
- Mode=stats, `page_size=20` (дефолт), API вернул `pagination: { page: 1, page_size: 20, total_rows: 45, total_pages: 3 }`

**КОГДА:** страница отрендерилась

**ТОГДА:**
- Над таблицей (в области между KPI-полосой и таблицей) текст: "Показано 1–20 из 45"
- После клика "→" (AC-38): страница=2, текст обновляется на "Показано 21–40 из 45"
- После клика "→" ещё раз: страница=3, текст "Показано 41–45 из 45"

**Источник:** [NetworkStatisticsPage.tsx:91-97](../../portal-frontend/src/pages/network/NetworkStatisticsPage.tsx#L91-L97).

---

#### AC-38: Пагинация: кнопки ← → и счётчик "Страница X из Y"

**ДАНО:**
- Mode=stats, `pagination.total_pages=3`, `page=2`

**КОГДА:** блок пагинации отрендерился

**ТОГДА:**
- Под таблицей виден блок из трёх элементов:
  1. Кнопка "←" (enabled)
  2. Текст "Страница 2 из 3"
  3. Кнопка "→" (enabled)
- На странице 1: "←" имеет `disabled`; на странице 3: "→" имеет `disabled`
- Клик "→" вызывает `onFiltersChange({page: 3})` + `onApply()` → URL получает `&page=3`, уходит запрос с `page=3`
- При `total_pages=1` блок пагинации НЕ рендерится вовсе (условие `pagination.total_pages > 1`)

**Источник:** [StatisticsTable.tsx:60-63,143-149](../../portal-frontend/src/components/network-stats/StatisticsTable.tsx#L60-L63).

---

#### AC-39: Смена не-пагинационного фильтра сбрасывает page на 1

**ДАНО:**
- Mode=stats, URL содержит `&page=3`, таблица показывает страницу 3

**КОГДА:** пользователь выбирает канал "SMS" → клик "Применить"

**ТОГДА:**
- В URL параметр `page` отсутствует (значение 1 как дефолт не сериализуется в URL, AC-D1-05-паттерн) ИЛИ `page=1`
- Запрос на `/portal/v1/reseller/statistics` содержит `page=1` (или его отсутствие, если backend принимает дефолт)
- Таблица показывает первую страницу новой выборки

**Источник:** [useNetworkStats.ts:164-168](../../portal-frontend/src/hooks/useNetworkStats.ts#L164-L168) — при любом изменении non-pagination фильтра `next.page = 1`.

---

## Итого в Batch 2

- **21 AC (AC-19..AC-39)** покрывают: идентичность колонок таблицы (AC-19..AC-21), conditional formatting (AC-22..AC-27), сортировка (AC-28..AC-29), клик-drilldown (AC-30), строка Итого (AC-31..AC-33), loading/empty (AC-34..AC-36), пагинация и индикатор (AC-37..AC-39).
- **Зафиксированные потенциальные drift'ы** (для следующей drift-записи):
  - 12 колонок в коде vs 13 в спеке (AC-19)
  - `openDrillDown` всегда использует `sliceType='provider'` независимо от `group_by` (AC-30 drift note)
- **Не покрыто в этом batch** (будут позже):
  - Монторинг-таблица (другая компонента `MonitoringTable`) — Batch 3
  - KPI-полоса (`StatisticsKPIStrip`) — отдельно или в Batch 3 (Monitoring KPI)
  - Drill-down drawer глубоко (5 tabs внутри, breadcrumb) — Batch 4
  - Saved views UI — Batch 5
  - Analytics Phase 2 (если доживёт)

## Критика своего второго батча

1. **AC-26/AC-27 проверяют пороги `pending>100`, `error>500`.** Это магические числа, они нигде в документации не зафиксированы, только в коде. При ревизии порогов (кем-то в будущем) тесты упадут тихо — никто не вспомнит про AC. Нужен либо константный экспорт (`PENDING_WARN_THRESHOLD = 100` рядом с функцией), либо явная связь с `domain/models.go:19-23` (`PendingCountWarn = 100`). Сейчас frontend дублирует константу — потенциальный бэк/фронт drift.

2. **AC-30 фиксирует bug (sliceType='provider' всегда), не требует фикса.** Это сознательно: bug-first, тест становится зелёным на текущем коде. Когда код фиксируется — тест обновляется, drift закрывается. Альтернатива: писать тест с `.fixme()` и пометкой "разблокировать после фикса sliceType" — но тогда оракул не работает сейчас.

3. **AC-33 (divide-by-zero) — edge case, который вряд ли воспроизведётся через обычные фильтры.** Риск: тест будет хрупким (нужна генерация данных `total=0` в seed, что может быть нетривиально). Альтернатива: unit-тест на `fmt`-функции вместо e2e. Флаг для Batch 6 решения о распределении unit vs e2e.

4. **Селекторы снова опущены.** То же осознанное решение из Batch 1. При генерации тестов — использовать Playwright ARIA + `getByText`, не CSS-классы. Цвета (`text-red-600` и т.д.) проверяются через computed style, а не селектор.

5. **Отсутствует AC на "клик по строке при открытом drawer".** Поведение не очевидно: перетирает ли новый клик drill-down стек? Это связано с Batch 4 (drill-down), но пересечение нужно явно задокументировать при написании тестов.

---

## Batch 3: Monitoring Mode (live polling, KPI grid, Monitoring table)

**Scope batch'а:** поведение режима "Мониторинг" (mode=monitoring): live-polling 10 сек, live-индикатор и пауза в FilterBar, MonitoringKPIGrid (4-колоночная сетка), MonitoringTable (12 колонок с throughput/latency/top_error), conditional formatting по latency p95, остановка polling при уходе с вкладки.

**Источники истины (верифицировано 2026-04-21):**
- [portal-frontend/src/hooks/usePolling.ts](../../portal-frontend/src/hooks/usePolling.ts)
- [portal-frontend/src/hooks/useNetworkStats.ts](../../portal-frontend/src/hooks/useNetworkStats.ts) (polling wiring — строки 129-139)
- [portal-frontend/src/components/network-stats/StatisticsFilterBar.tsx](../../portal-frontend/src/components/network-stats/StatisticsFilterBar.tsx) (live-индикатор в 203-218)
- [portal-frontend/src/components/network-stats/MonitoringKPIGrid.tsx](../../portal-frontend/src/components/network-stats/MonitoringKPIGrid.tsx)
- [portal-frontend/src/components/network-stats/MonitoringTable.tsx](../../portal-frontend/src/components/network-stats/MonitoringTable.tsx)
- [portal-frontend/src/pages/network/NetworkStatisticsPage.tsx](../../portal-frontend/src/pages/network/NetworkStatisticsPage.tsx) (mode=monitoring, 127-147)

### Группа N: Live-polling

#### AC-40: Активация Monitoring-tab запускает polling и немедленно делает запрос

**ДАНО:**
- Пользователь на mode=stats, polling не идёт
- В URL нет `?mode=monitoring`

**КОГДА:** клик по Tabs.Trigger "Мониторинг"

**ТОГДА:**
- URL меняется на `/network/statistics?mode=monitoring&...` (с сохранёнными non-mode фильтрами)
- Немедленно уходит `GET /portal/v1/reseller/monitoring?...` (первый запрос до таймера)
- После него через 10 секунд — второй запрос; ещё через 10 — третий (интервал 10000 ms задан в `useNetworkStats.ts:129`)
- В FilterBar-строке справа появляется блок: зелёный pulse-dot, текст "Обновлено N сек. назад", кнопка "Пауза"

**Источник:** [usePolling.ts:3-28](../../portal-frontend/src/hooks/usePolling.ts#L3-L28), [useNetworkStats.ts:129-139](../../portal-frontend/src/hooks/useNetworkStats.ts#L129-L139), [StatisticsFilterBar.tsx:204-218](../../portal-frontend/src/components/network-stats/StatisticsFilterBar.tsx#L204-L218).

---

#### AC-41: Индикатор "Обновлено N сек. назад" обновляется после каждого polling-tick'а

**ДАНО:**
- Mode=monitoring, первый запрос завершён T секунд назад (T < 10)
- В индикаторе текст "Обновлено T сек. назад"

**КОГДА:** проходит 10 секунд, polling отправляет новый запрос и получает ответ

**ТОГДА:**
- `lastUpdated` обновляется на `new Date()` ([usePolling.ts:12](../../portal-frontend/src/hooks/usePolling.ts#L12))
- Текст индикатора пересчитывается до "Обновлено 0 сек. назад" (или "1 сек. назад" — зависит от момента проверки)
- Pulse-dot продолжает пульсировать (анимация `animate-pulse`)

---

#### AC-42: Клик "Пауза" останавливает polling и меняет UI

**ДАНО:**
- Mode=monitoring, polling активен (прошло минимум 1 запрос)
- Кнопка показывает иконку `Pause` и текст "Пауза"

**КОГДА:** клик по кнопке "Пауза"

**ТОГДА:**
- `polling.pause()` → `isPaused=true` ([usePolling.ts:30](../../portal-frontend/src/hooks/usePolling.ts#L30))
- `clearInterval` выполнен ([usePolling.ts:21](../../portal-frontend/src/hooks/usePolling.ts#L21)) — новых тиков нет
- Текст кнопки меняется на "Продолжить", иконка — на `Play`
- В течение 30+ секунд после клика запросы `GET /portal/v1/reseller/monitoring` НЕ уходят
- Индикатор "Обновлено N сек. назад" продолжает увеличивать N (отсчёт от `lastUpdated`, который не обновляется)

**Источник:** [StatisticsFilterBar.tsx:210-216](../../portal-frontend/src/components/network-stats/StatisticsFilterBar.tsx#L210-L216), [useNetworkStats.ts:415](../../portal-frontend/src/hooks/useNetworkStats.ts#L415).

---

#### AC-43: Клик "Продолжить" возобновляет polling

**ДАНО:**
- Mode=monitoring, polling на паузе (AC-42), прошло 30 секунд с момента паузы
- Индикатор показывает "Обновлено 30 сек. назад" (или около того)

**КОГДА:** клик по кнопке "Продолжить"

**ТОГДА:**
- `polling.resume()` → `isPaused=false` ([usePolling.ts:31](../../portal-frontend/src/hooks/usePolling.ts#L31))
- `setInterval` перезапускается с тем же intervalMs=10000 ([usePolling.ts:24](../../portal-frontend/src/hooks/usePolling.ts#L24))
- В течение следующих 10 секунд уходит очередной запрос (первый тик после resume)
- Текст кнопки "Пауза", иконка `Pause`
- Индикатор "Обновлено 0/1 сек. назад" после возврата ответа

**Drift note (опционально):** текущая реализация `resume()` НЕ запускает `execute()` немедленно — только перезапускает `setInterval`. Первый запрос после возобновления произойдёт **через 10 секунд**, не сразу. Это может восприниматься как задержка. Фиксируется как ожидаемое поведение, но при необходимости — дополнительная drift-запись.

---

#### AC-44: Уход с Monitoring-tab останавливает polling

**ДАНО:**
- Mode=monitoring, polling активен

**КОГДА:** клик по Tabs.Trigger "Статистика"

**ТОГДА:**
- URL меняется на `?mode=stats&...`
- `useEffect` в [useNetworkStats.ts:133-139](../../portal-frontend/src/hooks/useNetworkStats.ts#L133-L139) вызывает `polling.pause()` (ветка `else` при `isMonitoringMode===false`)
- Запросы `GET /portal/v1/reseller/monitoring` больше не уходят
- Live-индикатор (зелёный dot + "Обновлено...") скрывается (в FilterBar блок рендерится только при `isMonitoring && polling`)
- Активный запрос `GET /portal/v1/reseller/statistics` уходит (через useEffect при смене mode)

---

### Группа O: MonitoringKPIGrid

#### AC-45: KPI-сетка показывает 4 колонки карточек

**ДАНО:**
- Mode=monitoring, в ответе `kpis` — массив из 6 элементов: `throughput`, `dlr_rate`, `pending`, `errors`, `timeouts`, `unhealthy_providers`

**КОГДА:** страница отрендерилась

**ТОГДА:**
- Отображается CSS grid с `grid-cols-4` — 4 колонки на строку ([MonitoringKPIGrid.tsx:43](../../portal-frontend/src/components/network-stats/MonitoringKPIGrid.tsx#L43))
- 6 KPI располагаются как 4+2 (первый ряд — 4 карточки, второй — 2)
- Каждая карточка содержит: маленький серый uppercase-label сверху (название KPI на русском через MONITORING_KPI_LABELS) + крупное bold-значение внизу

**Drift note:** grid-cols-4 фиксирован — при >4 KPI получается неровный второй ряд (2 карточки на 4-колоночной сетке с прижатием влево). Это не bug, но дизайнерская особенность. Не требует фикса.

---

#### AC-46: KPI-значения форматируются в соответствии с name-prefix

**ДАНО:**
- Mode=monitoring, ответ содержит:
  - `{name: "throughput", value: 125.7, status: "ok"}`
  - `{name: "dlr_rate", value: 0.87, status: "warning"}`
  - `{name: "pending", value: 4521, status: "ok"}`
  - `{name: "latency_p95", value: 15500, status: "warning"}`

**КОГДА:** страница отрендерилась

**ТОГДА:**
- Карточка "Пропускная способность": значение "126 msg/s" (`toFixed(0) + ' msg/s'`)
- Карточка "Доставляемость": значение "87.0%" (`(value*100).toFixed(1) + '%'`)
- Карточка "Ожидание": значение "4 521" (`toLocaleString('ru-RU')` с неразрывным пробелом)
- Карточка с именем, содержащим `latency/p50/p95`: значение "15.5 сек" (`(value/1000).toFixed(1) + ' сек'`)

**Источник:** [MonitoringKPIGrid.tsx:16-32](../../portal-frontend/src/components/network-stats/MonitoringKPIGrid.tsx#L16-L32).

---

#### AC-47: KPI-статусы раскрашивают значение

**ДАНО:**
- KPI карточки с разными `status`: "ok", "warning", "danger"

**КОГДА:** страница отрендерилась

**ТОГДА:**
- `status="danger"`: значение имеет класс `text-red-600`
- `status="warning"`: класс `text-amber-600`
- `status="ok"` или без статуса: класс `text-slate-900`
- Лейбл сверху (`uppercase tracking-wide text-gray-400`) не меняет цвет от статуса

**Источник:** [MonitoringKPIGrid.tsx:34-38,47](../../portal-frontend/src/components/network-stats/MonitoringKPIGrid.tsx#L34-L38).

---

### Группа P: MonitoringTable

#### AC-48: MonitoringTable — ровно 12 колонок

**ДАНО:**
- Mode=monitoring, в ответе ≥1 строка

**КОГДА:** таблица отрендерилась

**ТОГДА:**
- `<thead>` содержит 12 `<th>` по порядку: "Провайдер", "msg/s", "Отпр.", "Достав.", "Ожидание", "Таймаут", "Ошибки", "p50", "p95", "DLR%", "Осн. ошибка", "" (пустой заголовок health)
- Колонки 1-11 имеют иконку `ChevronsUpDown`; колонка 12 (health) имеет иконку `Activity` вместо стрелки сортировки

**Источник:** [MonitoringTable.tsx:40-53,81](../../portal-frontend/src/components/network-stats/MonitoringTable.tsx#L40-L53).

---

#### AC-49: Latency p95 > 30 сек — красный; 10–30 сек — amber

**ДАНО:**
- Пред-данные: строка A `dlr_latency_p95=35000` (35 сек), строка B `dlr_latency_p95=15000` (15 сек), строка C `dlr_latency_p95=5000` (5 сек)

**КОГДА:** таблица отрендерилась

**ТОГДА:**
- Строка A: ячейка p95 содержит "35.0с", класс `text-red-600 font-medium`
- Строка B: "15.0с", класс `text-amber-600`
- Строка C: "5.0с", без специального цвета (пустой класс)
- Колонка p50 (`dlr_latency_p50`) показывает то же форматирование "N.Nс", но БЕЗ conditional color

**Источник:** [MonitoringTable.tsx:15,18-22,48-49](../../portal-frontend/src/components/network-stats/MonitoringTable.tsx#L15-L22).

---

#### AC-50: Timeout > 0 — amber; Error > 0 — red; колонка "Осн. ошибка" — моноширинный код или "—"

**ДАНО:**
- Пред-данные:
  - Строка A: `timeout=50, error=0, top_error=null`
  - Строка B: `timeout=0, error=200, top_error="0x0003"`

**КОГДА:** таблица отрендерилась

**ТОГДА:**
- Строка A: ячейка "Таймаут" содержит "50", класс `text-amber-600`; ячейка "Ошибки" — "0" (литерал, без класса); ячейка "Осн. ошибка" — "—" класс `text-gray-300`
- Строка B: ячейка "Таймаут" — "0"; ячейка "Ошибки" — "200", класс `text-red-600 font-medium`; ячейка "Осн. ошибка" — текст "0x0003", класс `font-mono text-[11px] text-red-600`

**Drift note:** пороги MonitoringTable (timeout>0, error>0) отличаются от StatisticsTable (pending>100, error>500). Зафиксировано в D-15.

**Источник:** [MonitoringTable.tsx:46-47,51](../../portal-frontend/src/components/network-stats/MonitoringTable.tsx#L46-L51).

---

#### AC-51: Throughput без десятичных и с суффиксом "msg/s"

**ДАНО:**
- Пред-данные: строки с `throughput=42.3`, `throughput=0.9`, `throughput=1500`

**КОГДА:** таблица отрендерилась

**ТОГДА:**
- Ячейки содержат: "42" (округление вниз/вверх зависит от toFixed, фактически "42"), "1", "1500"
- Заголовок колонки — "msg/s", без дополнительных суффиксов в ячейках (суффикс только в заголовке и в KPI-карточке)

**Источник:** [MonitoringTable.tsx:42](../../portal-frontend/src/components/network-stats/MonitoringTable.tsx#L42) — `(r.throughput ?? 0).toFixed(0)`.

---

### Группа Q: Общее поведение Monitoring

#### AC-52: Клик по строке MonitoringTable открывает drill-down

**ДАНО:**
- Mode=monitoring, строка с `slice="mts"`

**КОГДА:** клик по `<tr>`

**ТОГДА:**
- Уходит `GET /portal/v1/reseller/drilldown?slice_type=provider&slice_value=mts&detail_view=operators&...`
- Drawer открывается (как AC-30)
- Polling **продолжает работать** (таблица обновляется раз в 10 сек), drawer виден независимо от tick'а

**Drift note:** то же что AC-30 — sliceType hardcoded 'provider' в [NetworkStatisticsPage.tsx:144](../../portal-frontend/src/pages/network/NetworkStatisticsPage.tsx#L144). D-14 покрывает.

---

#### AC-53: Loading/empty/pagination — как у StatisticsTable

**ДАНО:** Mode=monitoring с разными состояниями (loading, rows=[], total_pages>1)

**КОГДА:** таблица рендерится

**ТОГДА:**
- Loading (первый запрос, rows=[]): строка "Загрузка..."
- Loading с уже имеющимися строками: старые строки остаются, индикатор "Обновлено N сек. назад" продолжает идти (живое обновление — ключевое отличие от StatisticsTable)
- Empty (rows=[]): "Нет данных"
- Pagination: кнопки ←/→ и "Страница X из Y" появляются при total_pages > 1

**Источник:** [MonitoringTable.tsx:88-91,112-118](../../portal-frontend/src/components/network-stats/MonitoringTable.tsx#L88-L118).

---

## Итого в Batch 3

- **14 AC (AC-40..AC-53)** покрывают: live-polling активация/остановка/пауза/resume (AC-40..AC-44), MonitoringKPIGrid (4 колонки, форматирование, статус-цвета — AC-45..AC-47), MonitoringTable (12 колонок, latency/timeout/error color, top_error, throughput — AC-48..AC-51), общее поведение (drill-down, loading/empty/pagination — AC-52..AC-53).
- **Зафиксированные потенциальные drift'ы:**
  - Resume не запускает execute немедленно, только restart interval (AC-43 note) — возможный D-16, опционально
  - grid-cols-4 фиксирован — при 5+ или 7+ KPI рядом неровный ряд (AC-45 note) — дизайнерская особенность, не bug
- **Не покрыто в этом batch** (Batch 4+):
  - Interaction polling + drill-down (что происходит, если drawer открыт во время polling-тика)
  - Keyboard-accessibility таблицы
  - MonitoringResponse.chart (поле `chart` в API-ответе — не рендерится в UI, проверить в Batch 4 если нужно)

## Критика своего третьего батча

1. **AC-41 зависит от времени.** Тест "проверить текст через 10 сек" — timing-sensitive. В Playwright нужно `expect.poll` или `waitForResponse` на 2-й запрос `/monitoring`. В AC не указано как именно ловить второй тик — это оставлено на этапе написания тестов. Флаг для инструкции написания.

2. **AC-43 drift (resume не execute сразу) — на границе bug/feature.** Пользователь жмёт "Продолжить" и ожидает немедленного обновления, но видит старое "Обновлено N сек. назад" ещё до 10 секунд. UX-сомнение, не функциональный бан. Решать при следующем brainstorm'е.

3. **AC-51 о throughput `toFixed(0)`** — `toFixed(0)` округляет по HALF_EVEN / HALF_AWAY_FROM_ZERO в зависимости от движка. "42.3" → "42" всегда, но "42.5" → "43"/"42" зависит. Если seed-данные используют `.5` — тест флейки на кросс-браузере. Флаг.

4. **Drift D-15 активен и в Monitoring — три разные error-логики.** Batch 3 AC-50 снова тестирует error > 0 → red (без ступени 500 как в Statistics). Это осознанно — AC по факту кода. После разрешения D-15 все AC-26/27/50 будут переписаны единообразно.

5. **KPI `chart` (historical window) не покрыт ни одним AC.** В `MonitoringResponse.chart` есть поле с metric points (я видел при чтении server.go:77-82). В UI рендеринг chart'а отсутствует (нет компонента, который его читает). Это значит — либо chart вообще не использован, либо используется только в drill-down drawer (Batch 4). Если не использован — это dead data, drift-кандидат. Проверить при Batch 4.
