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

---

## D-06: Step 2 summary — spec требует client-side exclusion count, код показывает только note

**Обнаружено:** 2026-04-18 при подготовке Batch 3 AC (Step 2 Audience).

**Спека:** [2026-04-13-campaign-wizard-redesign.md](../superpowers/specs/2026-04-13-campaign-wizard-redesign.md), §"Шаг 2 / Сводка (реактивная)":
> Без фильтров: `Контактов к отправке: 1 234`
> С фильтрами: `Контактов к отправке: 1 089 (Исключено: 145)`
> Подсчёт исключений — клиентский (приблизительный, на основе процентного соотношения)

**Что в коде** ([CampaignWizardPage.tsx:577-594](../../portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx#L577-L594)):
```tsx
const total = list?.contacts_count ?? 0;
const hasFilters = excludeCountries.length > 0 || excludeOperators.length > 0;
return (
  <div>
    Контактов к отправке: <strong>{total.toLocaleString()}</strong>
    {hasFilters && <span>(фильтры применяются при запуске)</span>}
    ...
  </div>
);
```

Код показывает **total без вычитания**, с текстовым note "(фильтры применяются при запуске)". Client-side preview exclusion count не реализован.

**Последствие:**
Пользователь выбирает фильтры → total остаётся неизменным в UI → не видит, сколько фактически получат рассылку. Это UX-регрессия против спеки, но не баг поведения: фактическая фильтрация происходит на бэке при запуске.

**Почему код так сделан:** client-side подсчёт по сегментам был бы приблизительным (требует оценки % контактов, попадающих в исключаемый сегмент — не тривиально для не-uniform базы). Разработчик выбрал явное "применяется при запуске" вместо неточного числа.

**Разрешение:**
- **A (spec=truth):** реализовать client-side приблизительный подсчёт по процентам сегментов. Нужно дополнительное поле в `segments` endpoint (percent per country/operator). Средняя работа.
- **B (reasoned deviation):** обновить спеку — признать, что client-side preview убран осознанно, server-side filtering остаётся. AC-CW-A-07 написан по этой модели.

**Приоритет:** LOW. UX-регрессия, не функциональный баг.

**Статус:** 🟡 OPEN-LOW. AC Batch 3 написано по коду. Решение (A или B) — при следующем brainstorm по wizard.

---

## D-07: Step 2 dropdown label — формат отличается от спеки

**Обнаружено:** 2026-04-18 на code review Batch 3 AC (Step 2 Audience).

**Спека:** [2026-04-13-campaign-wizard-redesign.md](../superpowers/specs/2026-04-13-campaign-wizard-redesign.md), §"Шаг 2 / Выбор базы":
> Каждая опция отображает количество контактов: `"База клиентов (1 234)"`

Формат: имя + skobki + голое число.

**Что в коде** ([CampaignWizardPage.tsx:512](../../portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx#L512)):
```tsx
label: `${l.name} (${pluralContacts(l.contacts_count ?? 0)})`,
```

`pluralContacts(n)` возвращает `"1 234 контактов"` или `"1 контакт"` или `"2 контакта"` (русская плюрализация). Формат: имя + скобки + число + слово "контакт[а|ов]".

**Пример:** спека требует `"Сочи (1)"`, код выдаёт `"Сочи (1 контакт)"`.

**Последствие:** UX — лучше в коде (плюрализация читается естественнее), но спека не обновлена. Cosmetic.

**Разрешение:** LOW. Обновить спеку (признать, что плюрализованный формат — reasoned improvement). Либо оставить как есть. AC-CW-A-02 написан против кода (регекс на плюрализацию).

**Статус:** 🟡 OPEN-LOW. Не блокирует. Добавить в список "обновлений спеки" при следующем brainstorm по wizard.

---

## D-08: Cancel dialog discard button — "Отмена" в коде vs "Не сохранять" в спеке

**Обнаружено:** 2026-04-18 при подготовке Batch 5 (Navigation + Drafts).

**Спека:** [2026-04-13-campaign-wizard-redesign.md](../superpowers/specs/2026-04-13-campaign-wizard-redesign.md), §"Кнопки на каждом шаге":
> | "Отмена" | Открывает ConfirmDialog: "Сохранить как черновик" / "Не сохранять" |

Вторая кнопка диалога должна называться `"Не сохранять"`.

**Что в коде** ([ConfirmDialog.tsx:21](../../portal-frontend/src/components/ui/ConfirmDialog.tsx#L21)):
```tsx
<Button variant="secondary" onClick={onCancel}>Отмена</Button>
```

Используется общий `ConfirmDialog` компонент с жёстко прошитым текстом "Отмена" для secondary-кнопки. Для wizard-cancel-dialog не переопределяется — **наследует "Отмена"**.

**Последствие:**
- В wizard-cancel-dialog две кнопки: `"Отмена"` (discard — закрывает диалог + navigate на /campaigns) и `"Сохранить как черновик"` (save + navigate).
- Формулировка "Отмена" неоднозначна — пользователь может подумать "отмена этого действия" (= остаться в wizard), а не "отмена/выход без сохранения".
- Плюс — эта же кнопка **"Отмена"** существует в navigation bar wizard'а (line 847), который открывает этот диалог. Тройное использование одного слова в похожих контекстах.

**Разрешение:**
- **A (spec=truth):** расширить `ConfirmDialog` компонент — добавить prop `cancelLabel`, использовать "Не сохранять" для wizard-cancel. Или переименовать общий label на что-то менее неоднозначное.
- **B (reasoned deviation):** признать, что общий компонент с "Отмена" — осознанное архитектурное решение, обновить спеку.

**Приоритет:** LOW. UX-расплывчатость, не функциональный баг. Тесты в Batch 5 проверяют поведение (navigate на /campaigns), а не текст второй кнопки.

**Статус:** 🟡 OPEN-LOW. Решение — следующий brainstorm.

---

## D-09: POST /campaigns со `scheduled_at` в ISO-строке возвращает 400

**Обнаружено:** 2026-04-18 при AC-S-14 (Phase B) и повторно при проработке AC-C-13.

**Спека:** неявно — через backend proto schema. `CreateCampaignRequest.scheduled_at` имеет тип `google.protobuf.Timestamp`.

**Что в коде** ([campaigns.go handlers](../../internal/gateway/portal/handlers/campaigns.go)):
```go
var req campaignv1.CreateCampaignRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
    respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
    return
}
```

Используется **stdlib `encoding/json`**. Он НЕ знает про специальную обработку protobuf-типа `Timestamp` (которая ожидает либо `{seconds, nanos}`, либо требует `protojson` для парсинга ISO-строк).

**Frontend-поведение** ([CampaignWizardPage.tsx:200-213](../../portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx#L200-L213)):
```tsx
const scheduledAt = sendMode === 'later' && scheduledDate && scheduledTime
  ? new Date(`${scheduledDate}T${scheduledTime}`).toISOString()
  : undefined;

await campaignsApi.create({ ..., scheduled_at: scheduledAt });
```

Фронт отправляет ISO-string (`"2026-04-19T12:00:00Z"`). Бэк пытается json-decode в `Timestamp` struct → **ошибка парсинга** → `400 "Неверный формат запроса"`.

**Последствие:**
**КРИТИЧНО.** Пользователь не может создать кампанию "Позже" через UI — любая попытка планирования падает с generic ошибкой "Неверный формат запроса". Silent data loss в обратную сторону: кампания НЕ создаётся, пользователь теряет весь введённый прогресс.

Воспроизведено через curl:
```bash
POST /portal/v1/campaigns с body {scheduled_at: "2026-04-19T12:00:00Z", ...}
→ {"error":{"code":"INVALID_INPUT","message":"Неверный формат запроса"}}
```

**Почему это не поймали раньше:**
- Существующие campaign-wizard.spec.ts тесты ВСЕ skipped через `test.skip(true, 'Missing prerequisites')`
- AC-CW-S-14 (API round-trip в Batch 2 Phase B) уже обошёл эту проблему, отправив без scheduled_at
- UI-тесты Phase A для wizard не делали реальный submit
- Ручного тестирования "Позже" в недавних релизах, видимо, не проводилось

**Разрешение:**
**A (spec/intent = truth):** Backend handler должен использовать `protojson.Unmarshal` вместо stdlib `json.Decode`. Это корректно парсит ISO-строки в Timestamp и другие proto-специфичные типы. Изменение в всех `campaigns.go` handlers.

```go
import "google.golang.org/protobuf/encoding/protojson"

body, err := io.ReadAll(r.Body)
if err != nil { ... }
if err := protojson.Unmarshal(body, &req); err != nil { ... }
```

**B (frontend workaround):** Отправлять scheduled_at как `{seconds: unixTs}` вместо ISO. Но это нарушает web-convention (ISO-8601 стандарт) и требует особого знания у разработчика.

**Рекомендация:** A — фикс на бэке, стандартный паттерн для proto-HTTP gateway.

**Приоритет:** 🔴 HIGH. Это ломает основной user flow (планирование кампаний). Не LOW.

**Scope фикса (уточнение):** в [handlers/campaigns.go](../../internal/gateway/portal/handlers/campaigns.go) **8 мест** используют stdlib `json.NewDecoder().Decode`: строки 33, 110, 249, 282, 315, 344, 376, 400. Не все принимают Timestamp-поля, но для Create/Update/SetABConfig/SetRetryConfig — точно проблема. Минимальный фикс — заменить `json.Decode` на `protojson.Unmarshal` для `CreateCampaign` (строка 33). Полный фикс — все 8 мест для консистентности.

**Статус:** 🟢 RESOLVED (2026-04-18). Заменён `json.NewDecoder().Decode` на helper `decodeProto` (используется `protojson.Unmarshal`) в 4 proto-direct-decode сайтах: `CreateCampaign` (33), `UpdateCampaign` (110), `SetRetryConfig` (344), `PreviewTemplate` (400). Остальные 4 сайта остались на stdlib json (они декодят local wrapper structs, не proto). AC-CW-C-13 переведён с `test.fail()` в обычный тест.

---

## D-10: `?draft={id}` URL-параметр не реализован во фронтенде

**Обнаружено:** 2026-04-18 при code review Phase B AC (AC-CW-N-11).

**Спека:** [2026-04-13-campaign-wizard-redesign.md](../superpowers/specs/2026-04-13-campaign-wizard-redesign.md), §"Открытие черновика":
> При переходе на `/campaigns/new?draft={id}` — загружаем черновик через `GET /campaigns/{id}` и заполняем стейт визарда. Переходим сразу на шаг 1.

**Что в коде:** [CampaignWizardPage.tsx](../../portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx) — **ноль упоминаний** `draft`, `useSearchParams`, или парсинга URL query. Компонент игнорирует `?draft=` при открытии.

**Как проверено:**
```bash
$ grep -E "draft|useSearchParams|\?draft" CampaignWizardPage.tsx
# No matches found
```

**Последствие:**
Пользователь сохраняет черновик → возвращается в список `/campaigns` → кликает "Редактировать" на черновике (если кнопка есть) → URL `/campaigns/new?draft=<id>` → wizard открывается **ПУСТЫМ**, все введённые данные потеряны. User-visible data loss.

Возможно, в `CampaignsPage` даже нет edit-кнопки для черновиков, поэтому bug не всплывает. Но функционально — черновики создаются без возможности вернуться к ним.

**Разрешение (A=spec=truth):**
Добавить в `CampaignWizardPage`:
```tsx
const [searchParams] = useSearchParams();
const draftId = searchParams.get('draft');

useEffect(() => {
  if (!draftId) return;
  campaignsApi.get(draftId).then((c) => {
    setCampaignName(c.name);
    setContactListId(c.contact_list_id);
    setTemplateId(c.template_id || '');
    setSenderNameId(/* find by source */);
    // ... load all fields
  });
}, [draftId]);
```

**Приоритет:** 🟡 MEDIUM. Drafts сейчас создаются, но redemption broken. Пользователь может восстановить контекст только переcozда́в вручную.

**Статус:** 🟡 OPEN-MEDIUM. AC-CW-N-11 помечен `test.fixme()` — скипается до фикса фронтенда. Когда реализуется — убрать fixme, тест должен стать зелёным.

---

## D-11: Network routes в спеке vs `/reseller/*` в коде

**Обнаружено:** 2026-04-21 при аудите network-statistics AC перед расширением Batch 2.

**Спека:** [2026-04-17-network-statistics-analytics-monitoring-design.md](../superpowers/specs/2026-04-17-network-statistics-analytics-monitoring-design.md), §1 Architecture:
```
GET  /portal/v1/network/statistics
GET  /portal/v1/network/analytics
GET  /portal/v1/network/monitoring
GET  /portal/v1/network/drilldown
POST /portal/v1/network/export
CRUD /portal/v1/network/views
```

**Что в коде:**
- Router ([router.go:466](../../internal/gateway/portal/router/router.go#L466)): handler'ы из пакета `network_statistics` смонтированы под prefix `reseller.HandleFunc("/statistics", ...)`, т.е. итоговый путь — `/portal/v1/reseller/statistics`.
- Frontend ([networkStats.ts:161-219](../../portal-frontend/src/api/networkStats.ts#L161-L219)): все API-вызовы идут на `/reseller/statistics`, `/reseller/analytics-summary`, `/reseller/monitoring`, `/reseller/drilldown`, `/reseller/export`, `/reseller/views`.
- Комментарии в handler'ах ([network_statistics.go:159,186,213](../../internal/gateway/portal/handlers/network_statistics.go#L159)) дезинформируют: `// GetStatistics handles GET /network/statistics` — фактически роута `/network/statistics` не существует.
- AC-18 в [network-statistics-ac.md](network-statistics-ac.md) уже фиксирует реальный `/reseller/*` контракт, но это не было явно проведено как drift.

**Последствие:**
1. Любой новый AC, написанный дословно по спеке (как черновик D1 в ходе работы 2026-04-21), использует несуществующие endpoint'ы и ломает тесты.
2. Внешняя документация/клиенты, следующие спеке, получают 404.
3. Название "reseller" концептуально неверно в этом месте: раздел для **network-partners** (партнёров-агрегаторов), а не для reseller'ов — это отдельная роль. Использование prefix `/reseller` — легаси из ранней архитектуры, когда аггрегаторы обрабатывались как частный случай reseller.

**Разрешение:**
- **A (spec=truth, рекомендуется):** переименовать prefix в router'е на `/network/*`, обновить `networkStats.ts` на фронте, обновить комментарии handler'ов. Фронт и бэк в одном PR. Цена: потенциально ломает существующие e2e-тесты, которые ссылаются на `/reseller/*` (проверить `e2e/tests/`). Плюс: название соответствует доменной модели.
- **B (reasoned deviation):** обновить спеку — заменить `/portal/v1/network/*` на `/portal/v1/reseller/*`, признать исторический prefix. Цена: увековечить confused naming. Новые внешние клиенты будут спотыкаться о "я партнёр сети, почему endpoint называется reseller".

**Приоритет:** 🟡 MEDIUM. Не блокирует работающий flow (фронт и бэк договорились), но блокирует следование спеке при расширении AC и понимании новичками.

**Статус:** 🟡 OPEN-MEDIUM. До разрешения — все новые AC пишутся по фактическому `/reseller/*` (как AC-18). Drift явно отмечен для следующего brainstorm по разделу.

---

## D-12: `validateFilter` возвращает `Internal` вместо `InvalidArgument`

**Обнаружено:** 2026-04-21 при верификации AC-14 (валидация `group_by=5min` на period > 24h).

**Спека:** неявно через §2.5 спеки network-analytics:
> Валидация комбинации period × group_by возвращает `INVALID_ARGUMENT` с описанием лимита.

**Что в коде:**
- Validation-логика есть: [service.go:34-55](../../internal/services/network_analytics/application/service.go#L34-L55) — корректно проверяет `GroupByMaxPeriodHours["5min"] = 24`, возвращает `fmt.Errorf("period of %.0f hours exceeds the maximum of %d hours allowed for group_by=%q", ...)`.
- gRPC wrap не разделяет ошибки: [grpc/server.go:34-36](../../internal/services/network_analytics/grpc/server.go#L34-L36):
  ```go
  result, err := s.service.GetStatistics(ctx, filter)
  if err != nil {
      return nil, status.Errorf(codes.Internal, "get statistics: %v", err)
  }
  ```
- Любая ошибка — и validation, и БД, и нижележащий infra-сбой — маппится в `codes.Internal`, который HTTP-gateway превращает в **HTTP 500**.
- UI на выходе получает generic error, пишется в toast без различения (useNetworkStats.ts:118-120): `setError(err?.message || 'Ошибка загрузки данных')`. Никакого распознавания типа ошибки нет.

**Последствие:**
1. Пользователь, выбравший заведомо невалидную комбинацию (например 5min на 7 днях), видит сообщение вида `rpc error: code = Internal desc = get statistics: period of 168 hours exceeds...`. Это raw-сообщение, без локализации.
2. Мониторинг/alerting склеивает валидационные отказы с реальными авариями сервиса — невозможно построить SLO на "только infra errors".
3. AC-14 в документе описывает "HTTP 400 + локализованный toast" — это желаемое состояние, которое код не обеспечивает. На 2026-04-21 AC-14 обновлён под реальное поведение, чтобы тест мог быть зелёным сейчас; после фикса — AC вернуть к изначальной формулировке.

**Разрешение (A = spec=truth):**
1. Завести sentinel-тип в domain: `var ErrInvalidFilter = errors.New("invalid filter")` или `type ValidationError struct { Msg string }`.
2. `validateFilter` возвращает ошибку-обёртку этого типа с читаемым русским сообщением.
3. gRPC server: `errors.As(err, &ValidationError{})` → `codes.InvalidArgument`; иначе — `codes.Internal`.
4. HTTP handler ([network_statistics.go:177-180](../../internal/gateway/portal/handlers/network_statistics.go#L177-L180)): `respondGRPCError` уже маппит `codes.InvalidArgument` в `ErrInvalidInput` / HTTP 400 ([response.go:72-73](../../internal/api/http/response/response.go#L72-L73)) — дополнительных изменений в HTTP-слое не требуется.
5. Frontend: в useNetworkStats.ts ловить 400 отдельно и показывать локализованный toast из response.

**Связанные AC:** AC-14 (обновлён в текущем коммите, связан с этим drift), AC-B3-* (новые AC по валидации, которые будут в Batch 2/3).

**Приоритет:** 🟡 MEDIUM. UX-плохо, но не блокирует основной flow. Фикс небольшой, стандартный паттерн для gRPC-сервисов.

**Статус:** 🟡 OPEN-MEDIUM. AC-14 переписан по коду. После фикса — вернуть AC-14 к формулировке "HTTP 400 + локализованный toast" и закрыть drift.

---

## D-14: `openDrillDown` callsites hardcode `sliceType='provider'`

**Обнаружено:** 2026-04-21 при написании Batch 2 AC для Statistics Table (AC-30).

**Ожидаемое поведение:** при клике по строке таблицы drill-down должен открываться по **измерению текущей группировки**. Если `group_by=provider` — по провайдеру; если `group_by=operator` — по оператору; `group_by=country` — по стране и т.д.

**Что в коде:**
- Хук `openDrillDown(sliceType, sliceValue, label)` параметризован ([useNetworkStats.ts:183](../../portal-frontend/src/hooks/useNetworkStats.ts#L183)) — сам по себе гибкий.
- Вызывающие стороны (`NetworkStatisticsPage.tsx`) **hardcode первый аргумент как литерал `'provider'`** в трёх местах:
  - Строка 108 (режим stats): `onRowClick={(row: any) => stats.openDrillDown('provider', row.slice, row.slice)}`
  - Строка 122 (режим analytics): то же `'provider'`
  - Строка 144 (режим monitoring): то же `'provider'`

**Последствие:**
- При `group_by=operator` клик по строке "МТС" отправит `GET /portal/v1/reseller/drilldown?slice_type=provider&slice_value=мтс` — бэк попытается найти провайдера с именем "мтс", не оператора.
- В лучшем случае drill-down покажет пустую разбивку, в худшем — ошибочные данные (если имя провайдера и оператора случайно совпадают).
- То же для `group_by=country`, `group_by=channel`, `group_by=login` — drill-down работает корректно **только** при `group_by=provider`.

**Как могло появиться:** изначально drill-down разрабатывался для дефолтной группировки (провайдеры), параметр `sliceType` добавили в хук как хук "на вырост", но callsite не обновили после расширения списка группировок.

**Разрешение (A = spec=truth):**
Ввести мэппер `groupByToSliceType(group_by)` в `useNetworkStats.ts` или утилитный модуль:
```ts
const GROUP_BY_TO_SLICE: Record<string, string> = {
  provider: 'provider', operator: 'operator', channel: 'channel',
  login: 'login', country: 'country',
};
function groupByToSliceType(gb: string): string {
  return GROUP_BY_TO_SLICE[gb] || 'provider'; // fallback для временных группировок (5min, hour, day)
}
```
И изменить все 3 callsites на `stats.openDrillDown(groupByToSliceType(stats.filters.group_by), row.slice, row.slice)`.

Альтернатива — инкапсулировать решение прямо в хуке: `openDrillDownFromRow(row)` сам читает current `group_by`. Более чисто.

**Связанные AC:** AC-30 (зафиксировано bug-first: тест описывает текущее поведение). После фикса — обновить AC-30, чтобы `slice_type` зависел от `group_by`.

**Приоритет:** 🟡 MEDIUM. Функционально ломает drill-down для 4 из 5 dimensional-группировок (operator, channel, login, country), но пользователь, возможно, до сих пор не заметил, если практически использует только group_by=provider.

**Статус:** 🟡 OPEN-MEDIUM. AC-30 фиксирует текущее поведение. Фикс — одно изменение в 3 callsites + 1 utility.

---

## D-15: Абсолютные пороги `pending/error` во фронтенде — без источника истины

**Обнаружено:** 2026-04-21 при self-critique Batch 2 AC (AC-26, AC-27).

**Что в коде:**

Backend ([`domain/models.go:14-21`](../../internal/services/network_analytics/domain/models.go#L14-L21)) объявляет:
```go
DLRRateWarning     = 0.90
DLRRateDanger      = 0.80
LatencyP95WarnMs   = ...
LatencyP95DangerMs = ...
ErrorRateWarning   = ...   // процент, НЕ абсолютный count
ErrorRateDanger    = ...   // процент
PendingCountWarn   = 100   // абсолютный
PendingCountDanger = 500   // абсолютный
```
Эти константы используются только в `deriveHealth(...)` ([models.go:355-359](../../internal/services/network_analytics/domain/models.go#L355-L359)) для вычисления `health ∈ {ok, warning, danger}` на каждой строке.

Frontend независимо вводит свои пороги для визуальной окраски столбцов:
- [StatisticsTable.tsx:43](../../portal-frontend/src/components/network-stats/StatisticsTable.tsx#L43): `r.pending > 100 ? 'text-amber-600' : ''` — совпадает с `PendingCountWarn=100`, но порог 500 (`PendingCountDanger`) не используется, нет red-варианта.
- [StatisticsTable.tsx:45](../../portal-frontend/src/components/network-stats/StatisticsTable.tsx#L45): `r.error > 500 ? 'text-red-600' : 'text-amber-600'` (при error>0). **Число 500 не соответствует ни одной backend-константе** — в backend нет абсолютных error thresholds, только `ErrorRateWarning/Danger` (проценты).
- [MonitoringTable.tsx:45](../../portal-frontend/src/components/network-stats/MonitoringTable.tsx#L45): `r.pending > 100 ? 'text-amber-600 font-medium' : ''` — совпадает со StatisticsTable.
- [MonitoringTable.tsx:47](../../portal-frontend/src/components/network-stats/MonitoringTable.tsx#L47): `r.error > 0 ? 'text-red-600 font-medium'` — любой error сразу красный, **другая логика** по сравнению с StatisticsTable.
- [DrillDownDrawer.tsx:140](../../portal-frontend/src/components/network-stats/DrillDownDrawer.tsx#L140): `row.error > 0 ? 'text-red-600'` — как MonitoringTable.

**Три разные логики окраски `error` в трёх местах + число 500 без основания + DLR/profit используют отдельные пороги (AC-22/AC-23/AC-25).**

**Последствие:**
1. Пороги дрейфуют без уведомления: если backend поднимет `PendingCountWarn` до 200, StatisticsTable и MonitoringTable останутся на 100.
2. Нет согласованной UX-политики: одна и та же строка в StatisticsTable (error=300) будет amber, а в MonitoringTable — red.
3. AC-26/AC-27 тестируют магические числа, которые никто не согласовывал — тесты станут хрупкими при любой ревизии.

**Разрешение:**
- **A (spec=truth, рекомендуется):** вынести пороги в `@portal-frontend/src/constants/thresholds.ts` или получать из backend response (например, KPI-ответ содержит порог в payload). Использовать **одно** место истины во фронтенде, которое можно держать синхронным с `domain/models.go` через сгенерированную константу или ручной sync.
- **B (reasoned deviation):** признать, что цветовая раскраска — frontend concern, и можно иметь разные пороги в разных таблицах (drill-down может быть строже основной). Тогда — минимум, документировать каждое число inline и добавить unit-test `expect(PENDING_WARN_THRESHOLD).toBe(100)` рядом с backend-константой.

**Связанные AC:** AC-26 (pending > 100), AC-27 (error > 500), AC-22/AC-23 (DLR thresholds). Все в Batch 2.

**Приоритет:** 🟡 LOW. UX-симметрия нарушена, но не ломает функциональность. Реальная цена проявится при первой ревизии порогов.

**Статус:** 🟡 OPEN-LOW. При разрешении — упростить AC-26/AC-27, сослать на константы вместо hard-coded "100"/"500" в тексте AC.

---

## D-16: Поля `chart` / `trends` / `previous_kpis` — dead data на wire

**Обнаружено:** 2026-04-21 при Batch 4 (Drill-down drawer) AC и self-critique Batch 3.

**Что в protocol / API:**
- [`MonitoringResponse.chart: MetricPoint[]`](../../portal-frontend/src/api/networkStats.ts#L102-L107) — исторический график throughput/latency для режима мониторинга.
- [`AnalyticsResponse.trends: {metric, points}[]`](../../portal-frontend/src/api/networkStats.ts#L93-L100) — трендовые серии для режима аналитики.
- [`AnalyticsResponse.previous_kpis: KPI[]`](../../portal-frontend/src/api/networkStats.ts#L95) — KPI предыдущего периода для delta-сравнения.
- [`DrillDownResponse.trends: {metric, points}[]`](../../portal-frontend/src/api/networkStats.ts#L109-L114) — трендовые серии внутри drill-down.
- Все четыре поля заполняются бэкендом ([network_analytics/grpc/server.go:56-91](../../internal/services/network_analytics/grpc/server.go#L56-L91)): `Trends`, `PreviousKpis`, `Chart` — всё попадает в ответ.

**Что в UI:**
- `grep '\.chart\|\.trends\|previous_kpis' portal-frontend/src/components/network-stats/*` → **пусто**.
- `KPI.delta` используется — но только в [StatisticsKPIStrip.tsx:57-59](../../portal-frontend/src/components/network-stats/StatisticsKPIStrip.tsx#L57-L59) (стрелка ↑/↓ + процент). `previous_kpis` как отдельное поле не читается.
- Recharts 3.8.1 объявлен в зависимостях (plan 019-portal-ux-improvements), но ни один компонент в `network-stats/` не импортирует `recharts`.

**Последствие:**
1. **UX-gap.** Спека упоминает: "Mode Analytics — тренды, сигналы", "Mode Monitoring — throughput chart", "Drill-down — временная динамика". Ни одна из этих функций не реализована на фронтенде — только таблицы.
2. **Wire waste.** Backend вычисляет `chart`/`trends`/`previous_kpis` (включая запросы к `network_stats_hourly` для трендов) и отправляет клиенту. Пропускная способность потребляется без пользы. Для drill-down, вызываемого на каждый клик строки, это заметный overhead.
3. **Protocol drift.** Proto-схема [api/proto/network_analytics/*.proto](../../api/proto/network_analytics/) декларирует поля как часть контракта; если в будущем кто-то решит их реализовать — нужна сверка, что формат не сдрейфовал от backend'а.

**Разрешение:**
- **A (complete the feature):** реализовать графики. `AnalyticsResponse.trends` → новый компонент `TrendsChart` (Recharts), рендерится над таблицей в mode=analytics. `MonitoringResponse.chart` → `ThroughputChart` в mode=monitoring. `DrillDownResponse.trends` → отдельный tab в drawer (или дополнение к `timeline` tab). `previous_kpis` → вычисление delta клиентом ИЛИ использование уже готового `KPI.delta`, но тогда `previous_kpis` становится избыточен. **Большая работа**: ~3-4 новых компонента + стилизация.
- **B (amputate):** удалить поля из proto, репо, API-типа. Чище — wire/API отражают только то, что используется. **Плюс:** проще поддерживать. **Минус:** если фичу задумано восстановить — придётся мигрировать proto назад.
- **C (document as planned):** пометить поля как "reserved for Phase 2", без немедленной реализации. Требует отметки в спеке и readme.

**Приоритет:** 🟡 LOW. Функциональность работает без графиков (таблицы дают ту же информацию, хоть и без визуализации). Но это тип drift'а, который растёт со временем — каждый новый backend-разработчик добавляет поле "на будущее", фронтенд его не видит.

**Связанные AC:** ни один AC не зависит от этих полей. В Batch 4 self-critique point 4 зафиксировано.

**Статус:** 🟡 OPEN-LOW. Решение — при следующем brainstorm'е по разделу (вместе с Analytics Phase 2).

