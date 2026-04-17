# Spec ↔ Code Drift Tracker

Известные расхождения между спекой и реализацией. Каждая запись — кандидат на фикс кода (по правилу `feedback_spec_is_truth.md`) или на новый brainstorm с обновлённой спекой.

**Формат:** фича / что в спеке / что в коде / последствие / разрешение.

---

## D-01: `use_subscriber_timezone` в кампаниях

**Обнаружено:** 2026-04-18 при подготовке AC для campaign wizard.

**Спека:** [2026-04-13-campaign-wizard-redesign.md](../superpowers/specs/2026-04-13-campaign-wizard-redesign.md), §3 (Schedule) + §"Затрагиваемые файлы / Бэкенд":
> Поле `use_subscriber_timezone` отсутствует в текущей модели — требуется добавить в бэкенд (миграция + proto + handler).

**Что в коде:**
- ✅ Frontend ([CampaignWizardPage.tsx:72, 211](../../portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx)): state + отправляется в POST-запросе
- ✅ API-тип ([campaigns.ts:76](../../portal-frontend/src/api/campaigns.ts)): `use_subscriber_timezone?: boolean`
- ❌ Backend: **ни одного упоминания** `use_subscriber_timezone` / `UseSubscriberTimezone` в `internal/`
- ❌ Proto: нет в `api/proto/campaign/campaign.proto`
- ❌ Миграция: нет ALTER TABLE на поле
- ❌ БД: нет колонки

**Последствие:**
Пользователь ставит галочку "Доставить в указанное время по часовому поясу абонента". Фронт отправляет `use_subscriber_timezone: true`. Бэк молча дропает поле (проходит через `encoding/json` без Unmarshal target). Кампания создаётся без учёта TZ. **Silent data loss.**

Никаким из существующих тестов (`e2e/tests/campaigns/*.spec.ts`) не ловится, потому что тест создаёт кампанию через API и не читает её обратно с проверкой TZ-поля.

**Разрешение:**
По правилу "spec is truth" — код следует спеке, значит фикс на бэке:
1. Миграция `ALTER TABLE campaigns ADD COLUMN use_subscriber_timezone BOOLEAN NOT NULL DEFAULT FALSE`
2. Добавить `UseSubscriberTimezone bool` в `internal/services/campaign/domain/models.go`
3. Добавить в proto `CreateCampaignRequest` и `Campaign`
4. Обновить repository save/load
5. Обновить handler `campaigns.go` для парсинга и передачи

**AC, которое это поймает** (писать в batch 2 "Schedule"):
```
AC-CW-SCH-XX: Флаг use_subscriber_timezone персистится
  ДАНО: созданная кампания со статусом draft
  КОГДА: пользователь в wizard Step 3 отмечает "По часовому поясу абонента" и сохраняет черновик
  ТОГДА: 
    - POST /campaigns получает use_subscriber_timezone=true
    - GET /campaigns/{id} возвращает use_subscriber_timezone=true в ответе
    - БД campaigns.use_subscriber_timezone = true для этой кампании
```

**Статус:** 🔴 OPEN. Назначен на batch 2 AC (Step 3: Schedule) для поимки тестом. Фикс бэка отдельным PR.

---

## D-02: Шаблон обязателен vs свободный текст в Step 1

**Обнаружено:** 2026-04-18 при подготовке AC для batch 1 wizard.

**Спека:** [2026-04-13-campaign-wizard-redesign.md](../superpowers/specs/2026-04-13-campaign-wizard-redesign.md), §"Шаг 1: Сообщение":
> - **Textarea** с текстом сообщения (placeholder: "Введите текст сообщения...")
> - **Кнопка "Выбрать шаблон"** — при выборе шаблона текст подставляется в textarea
> - Валидация: Текст варианта A не пуст

Т.е. модель спеки: **свободный текст — основа, шаблон — удобство**.

**Что в коде** ([CampaignWizardPage.tsx:295-333](../../portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx#L295-L333)):
- **TemplatePicker required**, предупреждение "Для рассылки необходим одобренный шаблон"
- При выборе шаблона textarea **очищается** (строка 307: `if (id) setMessageText('')`)
- Textarea подписана: "Набросок текста (необязательно, для справки)", placeholder "Введите текст, чтобы затем создать из него шаблон..."
- Валидация `canProceed` (строка 173): `if (!templateId) return false` — требует templateId, **не messageText**

Т.е. модель кода: **шаблон обязателен, textarea — вспомогательный черновик для создания нового шаблона**.

**Последствие:**
Пользователь, следуя спеке, ожидает: набрал текст → Далее. По факту: набрал текст → Далее disabled → нужно создать шаблон сначала.

**Вероятная причина drift:** compliance-требование — в РФ SMS-трафик от юрлиц требует одобренных шаблонов (154-ФЗ / SPAM-защита у операторов). Разработчик (или предыдущий агент) скорее всего переделал на "template required" осознанно, но спеку не обновил.

**Это случай `Reasoned deviation` из feedback_spec_is_truth.md.** Требует решения пользователя:
- **Путь A (spec = truth):** откатить код к свободному тексту + опциональный шаблон. Риск: compliance-нарушение.
- **Путь B (code reflects reality, spec outdated):** обновить спеку — "шаблон обязателен, textarea — для создания нового шаблона". Новая версия спеки supersedes старую. Пишем AC по коду.

**Статус:** 🟡 RESOLUTION CHOSEN — Путь A (spec = truth, revert code).

**Решение пользователя (2026-04-18):** Путь A. Код откатывается к поведению спеки — свободный текст основной, шаблон опциональный.

**Код-изменения требуются:**
1. `canProceed` для step 'message' ([строка 173](../../portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx#L173)): `!templateId` → `!messageText.trim()`
2. `canProceed` для A/B ([строка 176](../../portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx#L176)): `!abTemplateIdB` → `!abMessageTextB.trim()`
3. При выборе шаблона: textarea ЗАПОЛНЯЕТСЯ body шаблона ([строка 307](../../portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx#L307): `setMessageText('')` → `setMessageText(tpl.body)`), а не очищается
4. Убрать warning "Для рассылки необходим одобренный шаблон" ([строки 310-315](../../portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx#L310-L315))
5. Label textarea: "Набросок текста (необязательно, для справки)" → "Текст сообщения"
6. Placeholder textarea: "Введите текст, чтобы затем создать из него шаблон..." → "Введите текст сообщения..."
7. Добавить state `abMessageTextB` + textarea (аналогично основному)
8. UI-зависимости в A/B секции — проверить (следующий шаг при применении фикса)

**Риск для проверки на тестах:** если compliance на бэке проверяет обязательность шаблона — кампании с пустым templateId будут падать на API. Тогда нужен DRIFT D-03 (compliance-check на бэке требует проверки).

**Статус:** 🟢 IN PROGRESS — AC пишутся по спеке, код-изменения следующим шагом.

