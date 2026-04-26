# Spec №3: SMPP → «Подключения» — переименование, удаление дублирующего раздела, локализация, Edit

**Дата:** 2026-04-26
**Источник:** MVP-таблица, строка «Интеграции → SMPP»
**Статус:** Не сделано.

## Контекст

Спека требует:
1. Переименовать раздел «SMPP» в «Подключения».
2. В визарде добавления вырезать этап «Маршрутизация» (regex предзагружен — будет в отдельном разделе «Маршрутизация»).
3. Перевести нетехнические термины на русский на каждом этапе.
4. В списке провайдеров добавить кнопку «Редактировать» (открывает то же окно как добавление; подключение остаётся активным).

Аудит:
- В навигации **два пункта одновременно** ([UserLayout.tsx:20,23](portal-frontend/src/components/layout/UserLayout.tsx)): `/settings/smpp` и `/providers`.
- [SmppSettingsPage.tsx](portal-frontend/src/pages/settings/SmppSettingsPage.tsx) — заглушка из 24 строк, кнопка ведёт на `/providers`. Это рудимент.
- [ProvidersPage.tsx](portal-frontend/src/pages/providers/ProvidersPage.tsx) — реальный список, заголовок «SMPP Провайдеры». В строках только кнопка «Удалить», Edit нет.
- [ProviderWizardPage.tsx:14](portal-frontend/src/pages/providers/ProviderWizardPage.tsx#L14) — 6 шагов: `['Основное', 'Подключение', 'Параметры', 'Тест', 'Маршрутизация', 'Итоги']`. «Маршрутизация» — это шаг 5, не 6 (спека ошибается в нумерации).
- [Step5Routing.tsx](portal-frontend/src/pages/providers/wizard/Step5Routing.tsx) — целиком на английском (`Routing Rules`, `Define number patterns…`, `+ Add Rule`).

## Решение

### Что меняем

**Маршрутизация и навигация:**
1. Удалить пункт меню `/settings/smpp` из [UserLayout.tsx](portal-frontend/src/components/layout/UserLayout.tsx).
2. Удалить файл [SmppSettingsPage.tsx](portal-frontend/src/pages/settings/SmppSettingsPage.tsx) и роут `/settings/smpp` в [App.tsx](portal-frontend/src/App.tsx).
3. На уровне роутинга добавить redirect `/settings/smpp → /providers` (для bookmark'ов; реализуется через `<Navigate to="/providers" replace />`).
4. В навигации (`UserLayout.tsx`) пункт `/providers` переименовать с «Провайдеры» на «Подключения».
5. Заголовок страницы в `ProvidersPage.tsx` поменять с «SMPP Провайдеры» на «Подключения».

> Замечание: в админке (`/admin/providers`) — отдельная страница, не трогаем. Спека про клиентский раздел.

**Визард:**
6. В [ProviderWizardPage.tsx:14](portal-frontend/src/pages/providers/ProviderWizardPage.tsx#L14) убрать шаг `'Маршрутизация'` из `STEPS`. Финальный список: `['Основное', 'Подключение', 'Параметры', 'Тест', 'Итоги']` (5 шагов).
7. Удалить файл `Step5Routing.tsx`. Логика regex-предзагрузки (если есть) переезжает в дефолт-значение при создании провайдера на бэке (`internal/services/routing/application/provider_service.go` — добавить дефолтное правило `.*` или пустое).
8. В оставшихся шагах визарда найти и локализовать английские термины:
   - Step1 (Основное): проверить лейблы.
   - Step2 (Подключение): `host`, `port` оставляем (технические); `system_id` → «Идентификатор системы (System ID)», `password` → «Пароль», `system_type` → «Тип системы (System Type)», `bind_type` → «Режим подключения» с опциями TX/RX/TRX переведёнными как «Только отправка / Только приём / Двусторонний».
   - Step3 (Параметры): `Max connections` → «Макс. одновременных подключений», `window size` → «Размер окна (window)», `TPS limit` → «Лимит TPS (сообщений/сек)», `Throughput per second` → то же.
   - Step4 (Тест): кнопка «Test» → «Проверить подключение».
   - Step5 (Итоги): `Summary` → «Сводка».
9. Технические аббревиатуры (TPS, TLV, PDU, bind_type внутри тех. описания) **оставляем латиницей** — это термины SMPP-протокола, перевод их искажает.

**Edit-кнопка:**
10. В таблице списка провайдеров ([ProvidersPage.tsx:103-107](portal-frontend/src/pages/providers/ProvidersPage.tsx#L103)) добавить кнопку «Редактировать» рядом с «Удалить».
11. Edit открывает тот же `ProviderWizardPage` в режиме `edit` через query-параметр или `:id`. На каждом шаге значения предзаполнены текущими из БД.
12. На бэке проверить наличие `providersApi.update` (gRPC + HTTP). Если нет — реализовать `UpdateProvider(id, fields)` в `internal/services/routing/`. **Требование:** при апдейте подключение **не должно прерываться** — изменение `host/port/system_id/password` создаёт новое подключение в фоне, после успешного бинда — переключает трафик и закрывает старое (graceful reconnect). Это уже реализовано в `internal/gateway/smpp/` — нужно проверить.

### Что НЕ делаем

- Переезд URL на `/connections`. Оставляем `/providers` — миграция URL ради косметики не нужна.
- Удаление колонки `route_pattern` из таблицы `providers` (если есть). Чистка БД — отдельная задача.
- Локализацию технических SMPP-полей.

## Acceptance criteria

1. В nav пункт называется «Подключения», ведёт на `/providers`.
2. Пункт `/settings/smpp` отсутствует в меню; прямой переход редиректит на `/providers`.
3. Заголовок `ProvidersPage` — «Подключения».
4. Визард 5 шагов, без «Маршрутизация».
5. Все нетехнические лейблы в визарде на русском (см. список выше).
6. В таблице рядом с «Удалить» есть «Редактировать».
7. Редактирование провайдера не приводит к разрыву существующего SMPP-подключения (проверить тестом: бинд активен → меняем параметры → бинд переходит на новые параметры без потери unbound-периода).
8. Существующие закладки на `/settings/smpp` редиректят на `/providers`.

## Размер задачи

S (1 рабочий день фронт + 0.5 дня бэк, если `UpdateProvider` уже есть; +1 день, если надо реализовывать graceful reconnect).
