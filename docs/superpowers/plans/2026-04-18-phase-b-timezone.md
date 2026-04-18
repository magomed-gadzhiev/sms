# Phase B Plan — Timezone Checkbox + D-01 Backend Fix

**Дата:** 2026-04-18
**Цель:** закрыть drift D-01 (`use_subscriber_timezone` не в backend) и D-04 (checkbox disabled в UI).
**Спека источника:** [2026-04-13-campaign-wizard-redesign.md](../specs/2026-04-13-campaign-wizard-redesign.md) §"Шаг 3: Расписание / Часовой пояс абонента"

## Scope — 14 файлов

### Backend (8 файлов)

| Файл | Изменение |
|---|---|
| `migrations/000103_campaigns_use_subscriber_timezone.up.sql` | NEW — `ALTER TABLE campaigns ADD COLUMN use_subscriber_timezone BOOLEAN NOT NULL DEFAULT FALSE` |
| `migrations/000103_campaigns_use_subscriber_timezone.down.sql` | NEW — `ALTER TABLE campaigns DROP COLUMN use_subscriber_timezone` |
| `api/proto/campaign/campaign.proto` | add `bool use_subscriber_timezone` в `Campaign` (field 23) и `CreateCampaignRequest` (field 10) |
| `api/proto/campaignv1/campaign.pb.go` | REGEN через `scripts/generate-proto.sh` |
| `api/proto/campaignv1/campaign_grpc.pb.go` | REGEN (grpc-файл тот же скрипт) |
| `internal/services/campaign/domain/models.go` | `Campaign.UseSubscriberTimezone bool` |
| `internal/services/campaign/application/campaign_service.go` | `CreateCampaign(..., useSubscriberTimezone bool)` — новый параметр, прокидывается в domain |
| `internal/services/campaign/grpc/server.go` | `CreateCampaign` читает `req.GetUseSubscriberTimezone()`, передаёт в service |
| `internal/services/campaign/grpc/mappers.go` | `campaignToProto` заполняет `pb.UseSubscriberTimezone = c.UseSubscriberTimezone` |
| `internal/services/campaign/infrastructure/repository/campaign_repository.go` | 1) `campaignRow.UseSubscriberTimezone bool` с tag; 2) `toDomain()`; 3) `campaignColumns`; 4) INSERT + UPDATE query и параметры |

### Frontend (1 файл)

| Файл | Изменение |
|---|---|
| `portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx` строки 665-680 | Включить checkbox: убрать `disabled`, убрать жёсткий `checked={false}` → связать с `useSubscriberTimezone` state, убрать "Функционал в разработке", tooltip заменить на смысл спеки |

### AC + Tests (5 файлов)

| Файл | Изменение |
|---|---|
| `docs/ac/campaign-wizard-ac.md` | Добавить раздел "Batch 2 Phase B" с 6 AC (S-09..S-14) |
| `docs/ac/_DRIFT.md` | Mark D-01 и D-04 как 🟢 RESOLVED с коммит-хэшами |
| `e2e/pages/CampaignsPage.ts` | Helpers: `timezoneCheckbox()`, `isTimezoneCheckboxEnabled()`, `toggleTimezoneCheckbox()` |
| `e2e/tests/campaigns/wizard-step3-timezone.spec.ts` | NEW — 5 UI AC (S-09..S-13), интерцепт POST |
| `e2e/tests/campaigns/campaign-timezone-persistence.spec.ts` | NEW — 1 API-only round-trip (S-14) |

## 6 AC (скелет)

- **AC-CW-S-09:** Checkbox `disabled` когда mode = "Сейчас" (блок не рендерится в DOM вообще — см. §"Активен только при выборе Позже" в спеке)
- **AC-CW-S-10:** Checkbox появляется и `enabled` когда mode = "Позже"
- **AC-CW-S-11:** Клик по checkbox → POST /campaigns шлёт `use_subscriber_timezone: true`
- **AC-CW-S-12:** Повторный клик → POST шлёт `false`
- **AC-CW-S-13:** Переключение "Позже" → "Сейчас" → checkbox снят (state reset), следующий POST (если переключить обратно на Позже и Submit без галки) шлёт `false`
- **AC-CW-S-14 (API round-trip):** POST /campaigns с `use_subscriber_timezone: true` → GET /campaigns/{id} возвращает `use_subscriber_timezone: true` → DELETE (cleanup)

## Потенциальные блокеры

1. **Proto regen требует protoc + protoc-gen-go + protoc-gen-go-grpc.** Если Device Guard блокирует Go в текущей сессии, regen надо запускать из обычного терминала. Альтернатива: сделать файлы `.pb.go` вручную, но это хрупко. **Лучше предупредить пользователя.**

2. **Миграция на sandbox.** После `scripts/server.sh deploy` миграция не запустится автоматически — нужно `scripts/server.sh migrate`. Проверить, есть ли такая команда (по CLAUDE.md: **есть**, "migrate — применить миграции БД").

3. **`campaign_service.go CreateCampaign` signature change** — ломает всех callers (grpc server, возможно тесты). Надо grep'ом найти всех вызывающих и обновить. Обход: передавать через опции (functional options), но это over-engineering для одного поля.

4. **Frontend отправляет use_subscriber_timezone сейчас** (строка 211 в CampaignWizardPage.tsx). Это значит — до backend fix'а поле уже идёт в API, но бэк его игнорирует. После fix'а — тот же JSON, но бэк его сохранит. **Обратная совместимость ОК.**

## План коммитов

### Вариант A: Двухпроходный (прозрачный two-phase)

- **Commit 1 (tests + AC + drift):** AC-docs + тесты + drift updates + Page Object helpers
  - Прогон: ожидаемо 0-3 red по timezone-тестам (D-04 drift не исправлен)
- **Commit 2 (backend + frontend fix):** все 9 бэкендных + 1 фронт файл
  - Deploy + migrate
  - Прогон: ожидаемо 6/6 green по timezone-тестам, существующие batch 2 Phase A не сломаны (регрессия)

### Вариант B: Однопроходный (прагматичный)

- **Один commit** — всё вместе
  - Проще деплой
  - Нет ред-грин демонстрации

**Рекомендую Вариант A.** Накопленный опыт двух прошлых batch'ей показал, что red-green цикл — не формальность, он реально валидирует, что тесты ловят именно drift. В Phase B drift'а две точки (D-01 и D-04), прогон с старым кодом покажет, какие именно тесты реагируют.

## Итерации review

- Full-form review после Commit 1 и Commit 2 (оба нетривиальные)
- Short-form для коррекций

## Closure criteria

- Все 6 AC Phase B зелёные на master после deploy + migrate
- D-01 и D-04 marked RESOLVED в `_DRIFT.md`
- Batch 2 Phase A (8 старых тестов) не сломаны (регрессия-чек)

## Время (оценка)

- Exploration: сделано (~30 мин)
- Plan: этот файл (~15 мин)
- Implementation: 1-1.5 часа
- Review iteration(s): 30-60 мин
- Deploy + re-run: 15-20 мин
- Итого: **3-4 часа** работы моей + реал-time пользователя на деплой.
