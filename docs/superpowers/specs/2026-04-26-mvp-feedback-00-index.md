# MVP-feedback 2026-04-26 — индекс спек-документов

Источник требований: [Google Sheets «MVP»](https://docs.google.com/spreadsheets/d/1hc-rHKxwpA5vblfZy9uY6-NGH_RexP7lrf7FgXG6yWI/edit), 5 вкладок. Аудит расхождений и обратная связь менеджеру: [docs/mvp-feedback-2026-04-25.docx](../../mvp-feedback-2026-04-25.docx).

Все архитектурные развилки закрыты автором без ожидания решения менеджера (по запросу пользователя «делай всё на своё усмотрение»). Если что-то из принятых решений менеджер захочет пересмотреть — переписываем соответствующий спек, не код.

## Сводка

| # | Тема | Размер | Статус сейчас | Ссылка |
|---|---|---|---|---|
| 1 | Быстрая отправка | trivial (правка спеки) | ✅ закрыт 2026-04-26 — код соответствует, действий по коду нет | [01](2026-04-26-mvp-feedback-01-quick-send-design.md) |
| 2 | Шаблоны клиента — переменные | S | ✅ закрыт 2026-04-26 — chip-ряд + удалено поле «Тип трафика» (commit 110784b) | [02](2026-04-26-mvp-feedback-02-templates-variables-design.md) |
| 3 | SMPP → «Подключения» | S | ✅ закрыт 2026-04-26 (commits 762f11a/b8e0d6c/1414b8b). Backend: graceful reconnect SMPP при update — отдельная проверка | [03](2026-04-26-mvp-feedback-03-smpp-connections-rename-design.md) |
| 4 | Админ → Заблокировать клиента | M | UI готов (2026-04-26): кнопка/модалка/бейдж/разблокировка, статус через UpdateClient(active). Осталось: backend каскад (refresh-tokens revoke, SMPP disconnect, pipeline cancel, audit-log persistence reason, blocked_at/block_reason миграция) | [04](2026-04-26-mvp-feedback-04-admin-block-client-design.md) |
| 5 | Биллинг — фильтры + группировка | M | foundation (2026-04-26): миграция 000121 добавляет `operation_kind` + backfill heuristics + индекс. Осталось: proto-extension `GetTransactionHistoryRequest` (kind/group_by), новый SQL агрегации в repository, backend kind при создании tx во всех местах, UI фильтр Операция + Группировка | [05](2026-04-26-mvp-feedback-05-billing-filters-design.md) |
| 6 | Dadata в Профиле | M | интеграции нет вообще | [06](2026-04-26-mvp-feedback-06-dadata-profile-design.md) |
| 7 | Провайдеры — бизнес-карточка | L | требуется миграция модели (1:1 → 1:N) | [07](2026-04-26-mvp-feedback-07-provider-business-card-design.md) |
| 8 | MCC/MNC — схема + seed | M | foundation (2026-04-26): миграция 000122 (колонки + индексы) + 000123 (seed 35 операторов СНГ из 11 стран). Осталось: proto-extension OperatorInfo, repository SELECT mcc/mnc, service-валидация, обновить sender_names.go:211-224 чтобы вернуть реальные mcc вместо op.Code | [08](2026-04-26-mvp-feedback-08-mcc-mnc-import-design.md) |
| 9 | Кампании-wizard — косметика | S | частично (2026-04-26): «Расписания» скрыты, default sender (sms), Variant B summary. Осталось: audience preview-эндпоинт, draft без contact_list (нужна миграция БД + domain change) | [09](2026-04-26-mvp-feedback-09-campaigns-wizard-cosmetics-design.md) |
| 10 | Детализация — UX-полировка | M | частично (2026-04-26): сортировка по статусу (frontend + backend whitelist), скрытие /admin/detalization из меню + баннер deprecation. Осталось: dropdown XLSX/CSV + BOM + лимит 100k экспорта (новый XLSX worker), client-фильтр для admin на /messages | [10](2026-04-26-mvp-feedback-10-detalization-polish-design.md) |

## Принятые архитектурные решения (без ожидания менеджера)

| ID | Развилка | Принятое решение |
|---|---|---|
| A | Шаблоны `Уважаем{ый\|ая}` | Литерал; условный рендер по полу — не в MVP |
| B | Сообщения в Kafka при блокировке | Drop в router stage → `status=cancelled, reason=client_blocked` |
| C | Hard/soft block | Soft (флаг + middleware-чек, кеш 30с); Redis-блоклист — позже по SLA |
| D | Модель Провайдеров | Гибрид: `providers` → `smpp_connections`, новая `provider_cards` (FK 1:N) |
| E | Договор/Менеджер | Текстовые поля на `provider_cards`; reuse-сущности — позже |
| F | Биллинг «Операции» | Новая колонка `operation_kind` ENUM + backfill |
| G | Биллинг агрегация | `date_trunc + SUM` на лету, индекс `(client_id, created_at)` |
| H | Dadata | Прокси через portal-gateway, кеш Redis по ИНН TTL 7 дней, 429 при превышении 10к/день |
| I | MCC/MNC источник | Курированный CSV в `migrations/seed-data/`, политика merge UPSERT по `(mcc, mnc)` |
| J | SMPP→Подключения URL | Удалить `/settings/smpp`, оставить `/providers` + redirect для bookmark'ов |
| K | Повторы кампаний | Скрыть `CampaignSchedulesPage` из меню; код оставить под прямым URL |
| L | Экспорт детализации | XLSX дефолт + CSV (с BOM) альтернатива; лимит 100k строк |

## Спорное и пропущенное

Эти пункты намеренно **не вошли** ни в один спек — спека противоречит самой себе или не определена:

| Пункт | Конфликт |
|---|---|
| Шаблоны операторов у клиента/партнёра | Tab1 говорит MVP=+ для партнёра, Tab2 говорит MVP=− |
| Имена по умолчанию для клиента | Tab1 MVP=−, Tab3 MVP=+ |
| БДПН (НИЦ) | В коде стоит HLR — это другая интеграция |
| Аналитика → Статистика | «ТЗ в телеге» — не определено |
| Дубликат Статистики для партнёра | «продублировать в Партнёр» без деталей |

Эти пункты ждут разрешения менеджером. Документ обратной связи отправлен.

## Следующий шаг

Перейти к написанию плана реализации (skill `superpowers:writing-plans`) для одного или нескольких из 10 спеков. Рекомендую порядок реализации:

1. **№1 (5 минут)** — обновить спеку.
2. **№2, №3, №9** (S, можно одним спринтом) — быстрые UI-правки. Закрывают 30% спеки одним мерж-окном.
3. **№4, №5, №6, №8, №10** (M, можно параллельно разными ребятами) — основной объём.
4. **№7** (L) — последним, требует осторожной миграции БД.
