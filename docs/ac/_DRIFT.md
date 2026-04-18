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

**Статус:** 🟢 RESOLVED (2026-04-18, Batch 2 Phase B). Backend миграция 000103 + обновление domain/repo/service/grpc-mapper. AC-CW-S-14 round-trip тест проверяет persistence.

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

---

## D-03: CharacterCounter формат "N осталось" vs "символы"

**Обнаружено:** 2026-04-18 при прогоне 1 batch 1 wizard AC, через ложное assertion в AC-M-05.

**Спека:** [2026-04-13-campaign-wizard-redesign.md](../superpowers/specs/2026-04-13-campaign-wizard-redesign.md), §"Шаг 1":
> CharacterCounter под textarea: **символы + количество частей SMS** (GSM7: 160/153 на часть, Unicode: 70/67)

Ожидается compound-формат: "12 символов · 1 SMS" или эквивалентный.

**Что в коде:** [CharacterCounter.tsx](../../portal-frontend/src/components/ui/CharacterCounter.tsx):
```tsx
{remaining >= 0 ? `${remaining} осталось` : `+${Math.abs(remaining)} сверх`}
{segments > 1 && ` · ${segments} SMS`}
```

Показывает **remaining** (сколько осталось до лимита), не **current** (сколько введено). Часть "количество SMS" появляется только когда segments > 1.

**Последствие:** низкое. UX функционально работает, пользователь понимает, сколько ещё можно ввести. Это формулировка, не фундамент.

**Разрешение:** LOW-priority. По правилу spec=truth, следовало бы подтянуть формат к спеке ("12 символов · 1 SMS"). Но реальная польза спорна — "осталось" более практично для пользователя. Возможно, следующий brainstorm по wizard должен обновить спеку: явно выбрать формат и задокументировать.

**Статус:** 🟡 OPEN-LOW. AC-M-05 сформулирован по реальному выводу ("148 осталось"), не по спеке. При переходе D-02 → fix формат counter не меняется.

---

## D-04: Timezone checkbox в Step 3 wizard — disabled placeholder

**Обнаружено:** 2026-04-18 при подготовке batch 2 AC для wizard Step 3.

**Спека:** [2026-04-13-campaign-wizard-redesign.md](../superpowers/specs/2026-04-13-campaign-wizard-redesign.md), §"Шаг 3":
> Checkbox: "Доставить в указанное время по часовому поясу абонента"
> Активен только при выборе "Позже"; при "Сейчас" — неактивен и снят

**Что в коде** ([CampaignWizardPage.tsx:665-680](../../portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx#L665-L680)):
```tsx
<div className="... opacity-50 cursor-not-allowed" title="Функционал в разработке">
  <input type="checkbox" checked={false} disabled ... />
  <div>
    <span>По часовому поясу абонента</span>
    <p>Функционал в разработке</p>
  </div>
</div>
```

- `checked={false}` жёстко — игнорирует state `useSubscriberTimezone`
- `disabled` — пользователь физически не может включить
- Надпись "Функционал в разработке"

**Последствие:**
1. Фича **видима** юзерам как плейсхолдер — они видят опцию, пробуют, не работает
2. Поскольку пользователь не может включить — `useSubscriberTimezone` state всегда false → отправляется `use_subscriber_timezone: false` на бэк
3. Бэк (D-01) это поле и не знает, но даже если бы знал — всегда получает false
4. Результат: фича существует в 3 местах (спека + UI + API) и **ни в одном не работает**

**Связь с D-01:** D-01 про бэкенд без этого поля. D-04 про UI-checkbox disabled. **Оба надо чинить вместе**, иначе фронт пошлёт true, бэк его проигнорирует (или наоборот — бэк сохранит, а UI не пошлёт).

**Разрешение по Пути A (spec = truth):**
1. Frontend: включить checkbox, связать с `useSubscriberTimezone` state, убрать надпись "в разработке"
2. Backend (параллельно): D-01 fix — миграция + proto + handler + repository
3. Оба в одном PR, чтобы не было промежуточного состояния silent loss

**Альтернатива:** убрать checkbox из UI совсем (признать недоступность функционала). Требует обновления спеки.

**Статус:** 🟢 RESOLVED (2026-04-18, Batch 2 Phase B). UI checkbox включён, связан с `useSubscriberTimezone` state, убрана надпись "Функционал в разработке". AC-CW-S-10 проверяет, что checkbox `enabled` при mode="Позже"; AC-CW-S-11/S-12 — что значение доходит до POST; AC-CW-S-13 — сброс при переключении.

---

## D-05: Date picker `min` — date-only, а спека требует datetime

**Обнаружено:** 2026-04-18 при code review Batch 2 AC (боевой тест wrapper'а `execute-with-review.md`).

**Спека:** [2026-04-13-campaign-wizard-redesign.md](../superpowers/specs/2026-04-13-campaign-wizard-redesign.md), §"Шаг 3":
> Минимальное значение: текущее время + 5 минут

Спека подразумевает декларативное ограничение на **datetime** (дата + время).

**Что в коде** ([CampaignWizardPage.tsx:652](../../portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx#L652)):
```tsx
<Input type="date" min={new Date().toISOString().split('T')[0]} />
```

`<input type="date">` HTML-элемент может ограничивать **только дату**, не время. Плюс runtime-проверка (строка 682-687):
```tsx
{scheduledDate && scheduledTime &&
  new Date(`${scheduledDate}T${scheduledTime}`) <= new Date(Date.now() + 5*60*1000) && ...}
```

**Последствие:**
AC-CW-S-04 корректно описывает **код** (`min=today`), но не **спеку** (`min=now+5min`). Пользователь может выбрать сегодняшнюю дату + время в прошлом → картинка разрешает, inline-ошибка появляется только после ввода. Этот gap — результат HTML-ограничения, не баг разработчика.

**Возможные разрешения:**
- **A (низкий приоритет):** заменить `<input type="date" + input type="time">` на custom datetime-picker, который умеет min datetime. Сложно, UX может стать хуже.
- **B (прагматичный):** обновить спеку — "min=today для date + inline-ошибка для time < now+5min". Признаём HTML-ограничение в спеке.

**Приоритет:** LOW. Inline-ошибка ловит попытку отправки в прошлое на Next-клике. Пользователь не может отправить кампанию с невалидным временем. Просто UX-трение в момент ввода.

**Статус:** 🟡 OPEN-LOW. AC-CW-S-04 остаётся по коду. При ревизии спеки (следующий brainstorm по wizard) предложить Вариант B.

