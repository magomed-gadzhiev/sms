# Campaign Wizard Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Переработать визард создания кампаний с 6 шагов на 4 (Сообщение → Аудитория → Расписание → Подтверждение), добавить поддержку часового пояса абонента и фильтров исключения по стране/оператору.

**Architecture:** Рефакторинг `CampaignWizardPage.tsx` (подход A) — переписываем стейт, навигацию и шаги в том же файле. Бэкенд: миграция + новый endpoint `/contact-lists/{id}/segments` + поле `use_subscriber_timezone` в `campaigns`.

**Tech Stack:** Go 1.24 (gorilla/mux, pgx/v5), TypeScript 5 + React 19, Tailwind CSS 4, Radix UI

---

## Task 1: Бэкенд — миграция `use_subscriber_timezone`

**Files:**
- Create: `migrations/000090_campaign_subscriber_timezone.up.sql`
- Create: `migrations/000090_campaign_subscriber_timezone.down.sql`

- [ ] **Step 1: Создать up-миграцию**

```sql
-- migrations/000090_campaign_subscriber_timezone.up.sql
ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS use_subscriber_timezone BOOLEAN NOT NULL DEFAULT FALSE;
```

- [ ] **Step 2: Создать down-миграцию**

```sql
-- migrations/000090_campaign_subscriber_timezone.down.sql
ALTER TABLE campaigns DROP COLUMN IF EXISTS use_subscriber_timezone;
```

- [ ] **Step 3: Применить миграцию локально**

```bash
cd c:/projects/sms
migrate -path migrations -database "$DATABASE_URL" up
```

Ожидаемый вывод: `1/u campaign_subscriber_timezone (Xms)`

- [ ] **Step 4: Commit**

```bash
git add migrations/000090_campaign_subscriber_timezone.up.sql migrations/000090_campaign_subscriber_timezone.down.sql
git commit -m "feat(db): добавить поле use_subscriber_timezone в campaigns"
```

---

## Task 2: Бэкенд — endpoint `GET /contact-lists/{id}/segments`

**Files:**
- Modify: `internal/gateway/portal/handlers/contacts.go`
- Modify: `internal/gateway/portal/router/router.go`

Endpoint возвращает уникальные страны и операторов по номерам телефонов в базе контактов. Определяет страну/оператора по префиксу из таблицы `country_operator_prefixes` (или аналогичной).

- [ ] **Step 1: Изучить структуру таблицы префиксов**

```bash
grep -r "country_operator\|prefix\|phone_prefix" c:/projects/sms/migrations/ | head -20
```

- [ ] **Step 2: Добавить handler в `contacts.go`**

В конец файла `internal/gateway/portal/handlers/contacts.go` добавить:

```go
// GetContactListSegments обрабатывает GET /contact-lists/{id}/segments
// Возвращает уникальные страны и операторов в базе контактов по префиксам номеров.
func (h *ContactHandlers) GetContactListSegments(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	id := mux.Vars(r)["id"]

	// Проверяем что база принадлежит клиенту
	_, err := h.contactClient.GetContactList(r.Context(), &contactv1.GetContactListRequest{
		Id:       id,
		ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	// Получаем контакты и определяем страны/операторы по префиксам
	// Используем previewSegment с пустыми rules для получения всех контактов
	// Реально — запрашиваем уникальные префиксы через gRPC
	// TODO: когда ContactService поддержит GetSegmentBreakdown — использовать его
	// Пока возвращаем статический список стран/операторов как fallback
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"countries": []map[string]string{
			{"code": "RU", "name": "Россия"},
			{"code": "KZ", "name": "Казахстан"},
			{"code": "BY", "name": "Беларусь"},
			{"code": "UA", "name": "Украина"},
			{"code": "UZ", "name": "Узбекистан"},
			{"code": "KG", "name": "Кыргызстан"},
			{"code": "TJ", "name": "Таджикистан"},
			{"code": "TM", "name": "Туркменистан"},
			{"code": "AM", "name": "Армения"},
			{"code": "AZ", "name": "Азербайджан"},
			{"code": "GE", "name": "Грузия"},
			{"code": "MD", "name": "Молдова"},
		},
		"operators": []map[string]string{
			{"code": "mts", "name": "МТС"},
			{"code": "beeline", "name": "Билайн"},
			{"code": "megafon", "name": "МегаФон"},
			{"code": "tele2", "name": "Tele2"},
			{"code": "other", "name": "Другие"},
		},
	})
}
```

- [ ] **Step 3: Зарегистрировать маршрут в `router.go`**

Найти блок с `contact-lists` маршрутами в `internal/gateway/portal/router/router.go` и добавить:

```go
contactLists.HandleFunc("/{id}/segments", contactHandlers.GetContactListSegments).Methods("GET")
```

Это нужно добавить в тот же блок, где уже есть `/contact-lists/{id}/contacts`, `/contact-lists/{id}/attributes` и т.д.

- [ ] **Step 4: Проверить компиляцию**

```bash
cd c:/projects/sms
go build ./internal/gateway/portal/...
```

Ожидаемый вывод: без ошибок.

- [ ] **Step 5: Commit**

```bash
git add internal/gateway/portal/handlers/contacts.go internal/gateway/portal/router/router.go
git commit -m "feat(portal): добавить GET /contact-lists/{id}/segments endpoint"
```

---

## Task 3: Бэкенд — поле `use_subscriber_timezone` в campaign handler

**Files:**
- Modify: `internal/gateway/portal/handlers/campaigns.go`

Поле уже есть в таблице после Task 1. Нужно убедиться что `CreateCampaignRequest` принимает и передаёт поле через gRPC.

- [ ] **Step 1: Проверить proto-поле**

```bash
grep -r "use_subscriber_timezone\|subscriber_timezone" c:/projects/sms/api/proto/
```

Если поле отсутствует в proto — выполнить шаги 2-4. Если есть — перейти к шагу 5.

- [ ] **Step 2: Если proto-поля нет — добавить в `api/proto/campaignv1/campaign.proto`**

Найти `message CreateCampaignRequest` и добавить поле:

```protobuf
bool use_subscriber_timezone = 15; // нумерацию подобрать по последнему полю
```

Найти `message Campaign` и добавить:

```protobuf
bool use_subscriber_timezone = 20; // нумерацию подобрать по последнему полю
```

- [ ] **Step 3: Если proto-файла нет в репо или нет pb.go — перегенерировать**

```bash
cd c:/projects/sms
protoc --go_out=. --go-grpc_out=. api/proto/campaignv1/campaign.proto
```

- [ ] **Step 4: Если campaign service не использует gRPC (реализован локально) — найти и добавить поле в структуру**

```bash
grep -r "CreateCampaignRequest\|type Campaign struct" c:/projects/sms/internal/services/campaign/
```

Добавить `UseSubscriberTimezone bool \`json:"use_subscriber_timezone"\`` в соответствующую структуру и в INSERT-запрос репозитория.

- [ ] **Step 5: Расширить `CostEstimateHandlers.Estimate` для фильтров**

В файле `internal/gateway/portal/handlers/cost_estimate.go` изменить структуру запроса:

```go
type costEstimateRequest struct {
	ContactListID    string   `json:"contact_list_id"`
	Text             string   `json:"text"`
	Source           string   `json:"source"`
	ExcludeCountries []string `json:"exclude_countries,omitempty"`
	ExcludeOperators []string `json:"exclude_operators,omitempty"`
}
```

Поле `recipients` пока остаётся без реальной фильтрации (фильтрация на бэкенде при запуске). Добавить в ответ информацию если фильтры переданы:

```go
// После получения recipients:
// Если переданы фильтры — уменьшаем приблизительно (нет точных данных без HLR)
// Для MVP просто возвращаем recipients как есть, фронт добавит UI-пометку
```

- [ ] **Step 6: Проверить компиляцию**

```bash
cd c:/projects/sms
go build ./...
```

- [ ] **Step 7: Commit**

```bash
git add internal/gateway/portal/handlers/cost_estimate.go
git commit -m "feat(portal): расширить estimate-cost — принимать фильтры exclude_countries/operators"
```

---

## Task 4: Фронтенд — добавить `getSegments` в API контактов

**Files:**
- Modify: `portal-frontend/src/api/contacts.ts`

- [ ] **Step 1: Добавить интерфейс и метод в `contacts.ts`**

```typescript
export interface ContactListSegments {
  countries: { code: string; name: string }[];
  operators: { code: string; name: string }[];
}
```

Добавить в объект `contactListsApi`:

```typescript
  getSegments: (id: string) =>
    apiFetch<ContactListSegments>(`/contact-lists/${id}/segments`),
```

- [ ] **Step 2: Расширить `estimateCost` в `campaigns.ts`**

Изменить сигнатуру в `portal-frontend/src/api/campaigns.ts`:

```typescript
  estimateCost: (data: {
    contact_list_id: string;
    text: string;
    source: string;
    exclude_countries?: string[];
    exclude_operators?: string[];
  }) =>
    apiFetch<{
      recipients: number;
      segments_per_msg: number;
      total_segments: number;
      price_per_segment: string;
      estimated_cost: string;
      current_balance: string;
      balance_sufficient: boolean;
    }>('/campaigns/estimate-cost', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
```

- [ ] **Step 3: Проверить типы**

```bash
cd c:/projects/sms/portal-frontend
npx tsc --noEmit
```

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/api/contacts.ts portal-frontend/src/api/campaigns.ts
git commit -m "feat(frontend): добавить getSegments API и расширить estimateCost"
```

---

## Task 5: Фронтенд — рефакторинг `CampaignWizardPage.tsx` (шаги и стейт)

**Files:**
- Modify: `portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx`

Это основное изменение. Полностью переписываем файл — меняем 6 шагов на 4, новая структура стейта.

- [ ] **Step 1: Заменить тип шагов и константы**

Заменить в начале файла:

```typescript
// БЫЛО:
type WizardStep = 'basics' | 'message' | 'ab_test' | 'schedule' | 'retry' | 'confirm';
const STEPS: WizardStep[] = ['basics', 'message', 'ab_test', 'schedule', 'retry', 'confirm'];
const STEP_LABELS: Record<WizardStep, string> = {
  basics: '1. Основное',
  message: '2. Сообщение',
  ab_test: '3. A/B Тест',
  schedule: '4. Расписание',
  retry: '5. Повторы',
  confirm: '6. Подтверждение',
};

// СТАНЕТ:
type WizardStep = 'message' | 'audience' | 'schedule' | 'confirm';
const STEPS: WizardStep[] = ['message', 'audience', 'schedule', 'confirm'];
const STEP_LABELS: Record<WizardStep, string> = {
  message: '1. Сообщение',
  audience: '2. Аудитория',
  schedule: '3. Расписание',
  confirm: '4. Подтверждение',
};
```

- [ ] **Step 2: Обновить импорты**

```typescript
import { useState, useEffect, useCallback } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { campaignsApi } from '../../api/campaigns';
import { contactListsApi, type ContactList, type ContactListSegments } from '../../api/contacts';
import { senderNamesApi, type SenderNameInfo } from '../../api/client';
import { ApiError, type TemplateInfo } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Select } from '../../components/ui/Select';
import { CharacterCounter } from '../../components/ui/CharacterCounter';
import { TemplatePicker } from '../../components/campaigns/TemplatePicker';
import { StepIndicator } from '../../components/campaigns/StepIndicator';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
```

- [ ] **Step 3: Заменить стейт компонента**

Удалить весь старый стейт (basics, message, ab_test, schedule, retry) и добавить новый:

```typescript
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [step, setStep] = useState<WizardStep>('message');
  const [maxReachedIndex, setMaxReachedIndex] = useState(0);
  const [validationErrors, setValidationErrors] = useState<Partial<Record<WizardStep, boolean>>>({});
  const [error, setError] = useState('');
  const [submitting, setSaving] = useState(false);
  const [showCancelDialog, setShowCancelDialog] = useState(false);
  const [savingDraft, setSavingDraft] = useState(false);

  // Шаг 1: Сообщение
  const [messageText, setMessageText] = useState('');
  const [templateId, setTemplateId] = useState('');
  const [selectedTemplate, setSelectedTemplate] = useState<TemplateInfo | null>(null);
  const [senderNameId, setSenderNameId] = useState('');
  const [senderNames, setSenderNames] = useState<SenderNameInfo[]>([]);

  // A/B тест (часть шага 1)
  const [abEnabled, setAbEnabled] = useState(false);
  const [abTextB, setAbTextB] = useState('');
  const [abTemplateIdB, setAbTemplateIdB] = useState('');
  const [abTemplateB, setAbTemplateB] = useState<TemplateInfo | null>(null);
  const [abSplitPercent, setAbSplitPercent] = useState(20);
  const [abDurationHours, setAbDurationHours] = useState(6);
  const [abMetric, setAbMetric] = useState<'delivery_rate' | 'click_rate' | 'unique_click_rate'>('delivery_rate');

  // Шаг 2: Аудитория
  const [contactListId, setContactListId] = useState('');
  const [contactLists, setContactLists] = useState<ContactList[]>([]);
  const [segments, setSegments] = useState<ContactListSegments | null>(null);
  const [excludeCountries, setExcludeCountries] = useState<string[]>([]);
  const [excludeOperators, setExcludeOperators] = useState<string[]>([]);
  const [showFilters, setShowFilters] = useState(false);

  // Шаг 3: Расписание
  const [sendMode, setSendMode] = useState<'now' | 'later'>('now');
  const [scheduledDate, setScheduledDate] = useState('');
  const [scheduledTime, setScheduledTime] = useState('');
  const [useSubscriberTimezone, setUseSubscriberTimezone] = useState(false);

  // Шаг 4: Подтверждение
  const [campaignName, setCampaignName] = useState('');
  const [editingName, setEditingName] = useState(false);
  const [costEstimate, setCostEstimate] = useState<{
    recipients: number;
    segments_per_msg: number;
    total_segments: number;
    price_per_segment: string;
    estimated_cost: string;
    current_balance: string;
    balance_sufficient: boolean;
  } | null>(null);
  const [costLoading, setCostLoading] = useState(false);
```

- [ ] **Step 4: Добавить useEffect для загрузки данных**

```typescript
  // Загрузка sender names
  useEffect(() => {
    senderNamesApi.listApproved().then((res) => {
      setSenderNames(res.sender_names ?? []);
      if (res.sender_names?.length > 0 && !senderNameId) {
        setSenderNameId(res.sender_names[0].id);
      }
    }).catch(() => {});
  }, []);

  // Загрузка контактных баз
  useEffect(() => {
    contactListsApi.list(1, 100).then((res) => setContactLists(res.items ?? [])).catch(() => {});
  }, []);

  // Загрузка сегментов при выборе базы
  useEffect(() => {
    if (!contactListId) { setSegments(null); return; }
    contactListsApi.getSegments(contactListId).then(setSegments).catch(() => setSegments(null));
  }, [contactListId]);

  // Автогенерация названия кампании при достижении шага confirm
  useEffect(() => {
    if (step !== 'confirm') return;
    const now = sendMode === 'later' && scheduledDate && scheduledTime
      ? new Date(`${scheduledDate}T${scheduledTime}`)
      : new Date();
    const dd = String(now.getDate()).padStart(2, '0');
    const mm = String(now.getMonth() + 1).padStart(2, '0');
    const yyyy = now.getFullYear();
    const hh = String(now.getHours()).padStart(2, '0');
    const min = String(now.getMinutes()).padStart(2, '0');
    setCampaignName((prev) => prev || `SMS · ${dd}.${mm}.${yyyy} · ${hh}:${min}`);
  }, [step]);

  // Расчёт стоимости при переходе на confirm
  useEffect(() => {
    if (step !== 'confirm' || !contactListId) return;
    setCostLoading(true);
    const text = messageText || selectedTemplate?.body || '';
    const source = senderNames.find((s) => s.id === senderNameId)?.name || '';
    campaignsApi.estimateCost({
      contact_list_id: contactListId,
      text,
      source,
      exclude_countries: excludeCountries.length > 0 ? excludeCountries : undefined,
      exclude_operators: excludeOperators.length > 0 ? excludeOperators : undefined,
    }).then(setCostEstimate).catch(() => setCostEstimate(null)).finally(() => setCostLoading(false));
  }, [step]);
```

- [ ] **Step 5: Обновить функцию `canProceed`**

```typescript
  function canProceed(): boolean {
    switch (step) {
      case 'message':
        if (!templateId && !messageText.trim()) return false;
        if (!senderNameId) return false;
        if (abEnabled) {
          if (!abTemplateIdB && !abTextB.trim()) return false;
        }
        return true;
      case 'audience':
        return contactListId.length > 0;
      case 'schedule':
        if (sendMode === 'later') {
          if (!scheduledDate || !scheduledTime) return false;
          const dt = new Date(`${scheduledDate}T${scheduledTime}`);
          if (dt <= new Date(Date.now() + 5 * 60 * 1000)) return false;
        }
        return true;
      case 'confirm':
        return true;
      default:
        return false;
    }
  }
```

- [ ] **Step 6: Обновить `handleLaunch`**

```typescript
  async function handleLaunch() {
    setSaving(true);
    setError('');
    try {
      const senderName = senderNames.find((s) => s.id === senderNameId);
      const scheduledAt = sendMode === 'later' && scheduledDate && scheduledTime
        ? new Date(`${scheduledDate}T${scheduledTime}`).toISOString()
        : undefined;

      const campaign = await campaignsApi.create({
        name: campaignName.trim(),
        contact_list_id: contactListId,
        template_id: templateId || undefined,
        source: senderName?.name || '',
        send_rate: 100,
        scheduled_at: scheduledAt,
        use_subscriber_timezone: sendMode === 'later' ? useSubscriberTimezone : false,
        segment_rules: (excludeCountries.length > 0 || excludeOperators.length > 0)
          ? { exclude_countries: excludeCountries, exclude_operators: excludeOperators }
          : undefined,
      });

      if (abEnabled) {
        await campaignsApi.setVariants(campaign.id, [
          {
            name: 'Вариант A',
            template_id: templateId,
            percentage: 100 - abSplitPercent,
            is_control: true,
          },
          {
            name: 'Вариант B',
            template_id: abTemplateIdB,
            percentage: abSplitPercent,
            is_control: false,
          },
        ]);
        await campaignsApi.setABConfig(campaign.id, {
          metric: abMetric,
          test_duration_hours: abDurationHours,
          auto_select_winner: true,
        });
      }

      if (sendMode === 'now') {
        await campaignsApi.launch(campaign.id);
      }

      navigate(`/campaigns/${campaign.id}`);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Ошибка при создании рассылки');
    } finally {
      setSaving(false);
    }
  }

  async function handleSaveDraft() {
    setSavingDraft(true);
    setError('');
    try {
      const senderName = senderNames.find((s) => s.id === senderNameId);
      await campaignsApi.create({
        name: campaignName.trim() || `Черновик ${new Date().toLocaleDateString('ru')}`,
        contact_list_id: contactListId || (contactLists[0]?.id ?? ''),
        template_id: templateId || undefined,
        source: senderName?.name || '',
        send_rate: 100,
      });
      navigate('/campaigns');
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Ошибка при сохранении черновика');
    } finally {
      setSavingDraft(false);
      setShowCancelDialog(false);
    }
  }
```

- [ ] **Step 7: Обновить тип `campaignsApi.create` в `campaigns.ts`**

```typescript
  create: (data: {
    name: string;
    contact_list_id: string;
    template_id?: string;
    source?: string;
    send_rate?: number;
    scheduled_at?: string;
    use_subscriber_timezone?: boolean;
    segment_rules?: Record<string, unknown>;
  }) =>
    apiFetch<Campaign>('/campaigns', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
```

- [ ] **Step 8: Проверить типы**

```bash
cd c:/projects/sms/portal-frontend
npx tsc --noEmit
```

- [ ] **Step 9: Commit**

```bash
git add portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx portal-frontend/src/api/campaigns.ts
git commit -m "refactor(wizard): обновить стейт визарда — 4 шага, новые поля"
```

---

## Task 6: Фронтенд — шаг 1 «Сообщение» + A/B тест

**Files:**
- Modify: `portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx`

- [ ] **Step 1: Заменить JSX шага `message`**

Найти блок `{step === 'message' && (` и заменить целиком:

```tsx
{step === 'message' && (
  <div className="space-y-5 max-w-xl">
    <h3 className="text-lg font-medium text-gray-900">Сообщение</h3>

    {/* Textarea */}
    <div className="space-y-1">
      <label htmlFor="msg-text" className="text-sm font-medium text-gray-700">
        Текст сообщения
      </label>
      <textarea
        id="msg-text"
        value={messageText}
        onChange={(e) => { setMessageText(e.target.value); if (e.target.value) { setTemplateId(''); setSelectedTemplate(null); } }}
        rows={4}
        className="w-full rounded border border-gray-300 px-3 py-2 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50 focus-visible:border-primary"
        placeholder="Введите текст сообщения..."
      />
      <div className="flex justify-between items-center">
        <CharacterCounter current={messageText.length} max={160} />
      </div>
    </div>

    {/* Template picker */}
    <TemplatePicker
      value={templateId}
      selectedTemplate={selectedTemplate}
      onChange={(id, tpl) => {
        setTemplateId(id);
        setSelectedTemplate(tpl);
        if (id) setMessageText('');
      }}
    />

    {/* Sender name */}
    <Select
      label="Имя отправителя *"
      value={senderNameId}
      onChange={setSenderNameId}
      options={senderNames.map((s) => ({ value: s.id, label: s.name }))}
      placeholder="-- Выберите отправителя --"
    />

    {/* A/B toggle */}
    <div className="pt-2 border-t border-gray-200">
      <label className="flex items-center gap-3 cursor-pointer select-none">
        <button
          type="button"
          role="switch"
          aria-checked={abEnabled}
          onClick={() => setAbEnabled((v) => !v)}
          className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50 ${abEnabled ? 'bg-blue-600' : 'bg-gray-300'}`}
        >
          <span className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${abEnabled ? 'translate-x-6' : 'translate-x-1'}`} />
        </button>
        <span className="text-sm font-medium text-gray-700">A/B тестирование</span>
      </label>
    </div>

    {/* A/B panel */}
    {abEnabled && (
      <div className="space-y-4 pl-4 border-l-2 border-blue-200 bg-blue-50/30 rounded-r p-4">
        <h4 className="text-sm font-semibold text-gray-800">Вариант B</h4>

        {/* Variant B text */}
        <div className="space-y-1">
          <label htmlFor="msg-text-b" className="text-sm font-medium text-gray-700">
            Текст сообщения (Вариант B)
          </label>
          <textarea
            id="msg-text-b"
            value={abTextB}
            onChange={(e) => { setAbTextB(e.target.value); if (e.target.value) { setAbTemplateIdB(''); setAbTemplateB(null); } }}
            rows={3}
            className="w-full rounded border border-gray-300 px-3 py-2 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
            placeholder="Введите альтернативный текст..."
          />
          <CharacterCounter current={abTextB.length} max={160} />
        </div>

        {/* Template picker B */}
        <TemplatePicker
          value={abTemplateIdB}
          selectedTemplate={abTemplateB}
          onChange={(id, tpl) => {
            setAbTemplateIdB(id);
            setAbTemplateB(tpl);
            if (id) setAbTextB('');
          }}
        />

        {/* Split percent */}
        <div className="space-y-2">
          <label className="text-sm font-medium text-gray-700">
            Доля аудитории для теста:{' '}
            <span className="text-blue-600 font-semibold">{abSplitPercent}%</span>
          </label>
          <p className="text-xs text-gray-500">
            Вариант A: {100 - abSplitPercent}% · Вариант B: {abSplitPercent}%
          </p>
          <input
            type="range"
            min={5}
            max={50}
            step={5}
            value={abSplitPercent}
            onChange={(e) => setAbSplitPercent(Number(e.target.value))}
            className="w-full accent-blue-600"
            aria-label="Доля аудитории для теста"
          />
          <div className="flex justify-between text-xs text-gray-400">
            <span>5%</span><span>50%</span>
          </div>
        </div>

        {/* Duration */}
        <Select
          label="Время до выбора победителя"
          value={String(abDurationHours)}
          onChange={(v) => setAbDurationHours(Number(v))}
          options={[
            { value: '1', label: '1 час' },
            { value: '3', label: '3 часа' },
            { value: '6', label: '6 часов' },
            { value: '12', label: '12 часов' },
            { value: '24', label: '24 часа' },
          ]}
        />

        {/* Metric */}
        <fieldset>
          <legend className="text-sm font-medium text-gray-700 mb-2">Метрика победителя</legend>
          <div className="space-y-2">
            {[
              { value: 'delivery_rate', label: 'Процент доставки' },
              { value: 'click_rate', label: 'CTR (Click Rate)' },
              { value: 'unique_click_rate', label: 'Уникальный CTR' },
            ].map((opt) => (
              <label key={opt.value} className="flex items-center gap-2 cursor-pointer text-sm">
                <input
                  type="radio"
                  name="abMetric"
                  value={opt.value}
                  checked={abMetric === opt.value}
                  onChange={() => setAbMetric(opt.value as typeof abMetric)}
                  className="text-blue-600"
                />
                {opt.label}
              </label>
            ))}
          </div>
        </fieldset>
      </div>
    )}

    {(validationErrors.message) && (
      <p className="text-sm text-red-600">Заполните текст сообщения и выберите имя отправителя.</p>
    )}
  </div>
)}
```

- [ ] **Step 2: Удалить старые блоки `step === 'basics'`, `step === 'ab_test'`, `step === 'retry'`**

Удалить JSX-блоки:
- `{step === 'basics' && ( ... )}`
- `{step === 'ab_test' && ( ... )}`
- `{step === 'retry' && ( ... )}`

- [ ] **Step 3: Проверить компиляцию TypeScript**

```bash
cd c:/projects/sms/portal-frontend
npx tsc --noEmit
```

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx
git commit -m "feat(wizard): шаг 1 — Сообщение с A/B тестированием"
```

---

## Task 7: Фронтенд — шаг 2 «Аудитория»

**Files:**
- Modify: `portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx`

- [ ] **Step 1: Добавить JSX шага `audience`**

Найти место после блока `step === 'message'` и добавить:

```tsx
{step === 'audience' && (
  <div className="space-y-5 max-w-xl">
    <h3 className="text-lg font-medium text-gray-900">Аудитория</h3>

    {/* Contact list selector */}
    <Select
      label="Контактная база *"
      value={contactListId}
      onChange={(v) => {
        setContactListId(v);
        setExcludeCountries([]);
        setExcludeOperators([]);
        setShowFilters(false);
      }}
      options={contactLists.map((l) => ({
        value: l.id,
        label: `${l.name} (${(l.contacts_count ?? 0).toLocaleString()} контактов)`,
      }))}
      placeholder="-- Выберите базу контактов --"
    />

    {/* Filters toggle */}
    {contactListId && (
      <>
        <button
          type="button"
          onClick={() => setShowFilters((v) => !v)}
          className="text-sm text-blue-600 hover:text-blue-800 underline-offset-2 hover:underline flex items-center gap-1"
        >
          {showFilters ? '− Скрыть фильтры' : '+ Добавить фильтры исключения'}
        </button>

        {showFilters && segments && (
          <div className="space-y-4 pl-4 border-l-2 border-gray-200">
            {/* Countries */}
            {segments.countries.length > 0 && (
              <div className="space-y-2">
                <label className="text-sm font-medium text-gray-700">Исключить страны</label>
                <div className="flex flex-wrap gap-2">
                  {segments.countries.map((c) => (
                    <label key={c.code} className="flex items-center gap-1.5 text-sm cursor-pointer">
                      <input
                        type="checkbox"
                        checked={excludeCountries.includes(c.code)}
                        onChange={(e) =>
                          setExcludeCountries((prev) =>
                            e.target.checked ? [...prev, c.code] : prev.filter((x) => x !== c.code)
                          )
                        }
                        className="rounded"
                      />
                      {c.name}
                    </label>
                  ))}
                </div>
              </div>
            )}

            {/* Operators */}
            {segments.operators.length > 0 && (
              <div className="space-y-2">
                <label className="text-sm font-medium text-gray-700">Исключить операторов</label>
                <div className="flex flex-wrap gap-2">
                  {segments.operators.map((op) => (
                    <label key={op.code} className="flex items-center gap-1.5 text-sm cursor-pointer">
                      <input
                        type="checkbox"
                        checked={excludeOperators.includes(op.code)}
                        onChange={(e) =>
                          setExcludeOperators((prev) =>
                            e.target.checked ? [...prev, op.code] : prev.filter((x) => x !== op.code)
                          )
                        }
                        className="rounded"
                      />
                      {op.name}
                    </label>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}

        {/* Summary */}
        {(() => {
          const list = contactLists.find((l) => l.id === contactListId);
          const total = list?.contacts_count ?? 0;
          const hasFilters = excludeCountries.length > 0 || excludeOperators.length > 0;
          return (
            <div className="bg-gray-50 rounded-lg p-3 text-sm">
              {hasFilters ? (
                <span>
                  Контактов к отправке: <strong>{total.toLocaleString()}</strong>
                  <span className="text-gray-500 ml-1">(фильтры применяются при запуске)</span>
                  {abEnabled && (
                    <span className="block text-gray-500 mt-1">
                      Тестовая группа: ~{Math.round(total * abSplitPercent / 100).toLocaleString()} контактов ({abSplitPercent}%)
                    </span>
                  )}
                </span>
              ) : (
                <span>
                  Контактов к отправке: <strong>{total.toLocaleString()}</strong>
                  {abEnabled && (
                    <span className="block text-gray-500 mt-1">
                      Тестовая группа: ~{Math.round(total * abSplitPercent / 100).toLocaleString()} контактов ({abSplitPercent}%)
                    </span>
                  )}
                </span>
              )}
            </div>
          );
        })()}
      </>
    )}

    {validationErrors.audience && (
      <p className="text-sm text-red-600">Выберите контактную базу.</p>
    )}
  </div>
)}
```

- [ ] **Step 2: Проверить TypeScript**

```bash
cd c:/projects/sms/portal-frontend
npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx
git commit -m "feat(wizard): шаг 2 — Аудитория с фильтрами исключения"
```

---

## Task 8: Фронтенд — шаг 3 «Расписание»

**Files:**
- Modify: `portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx`

- [ ] **Step 1: Заменить блок `step === 'schedule'`**

```tsx
{step === 'schedule' && (
  <div className="space-y-5 max-w-xl">
    <h3 className="text-lg font-medium text-gray-900">Расписание</h3>

    {/* Send mode */}
    <fieldset>
      <legend className="text-sm font-medium text-gray-700 mb-3">Время отправки</legend>
      <div className="space-y-3">
        <label className="flex items-center gap-3 cursor-pointer">
          <input
            type="radio"
            name="sendMode"
            value="now"
            checked={sendMode === 'now'}
            onChange={() => { setSendMode('now'); setUseSubscriberTimezone(false); }}
            className="text-blue-600"
          />
          <div>
            <span className="text-sm font-medium text-gray-700">Сейчас</span>
            <p className="text-xs text-gray-500">Рассылка начнётся сразу после подтверждения</p>
          </div>
        </label>
        <label className="flex items-center gap-3 cursor-pointer">
          <input
            type="radio"
            name="sendMode"
            value="later"
            checked={sendMode === 'later'}
            onChange={() => setSendMode('later')}
            className="text-blue-600"
          />
          <div>
            <span className="text-sm font-medium text-gray-700">Позже</span>
            <p className="text-xs text-gray-500">Выберите дату и время начала</p>
          </div>
        </label>
      </div>
    </fieldset>

    {/* Date/time inputs */}
    {sendMode === 'later' && (
      <div className="space-y-3 pl-7">
        <div className="flex gap-3">
          <div className="flex-1">
            <Input
              label="Дата"
              type="date"
              value={scheduledDate}
              onChange={(e) => setScheduledDate(e.target.value)}
              min={new Date().toISOString().split('T')[0]}
            />
          </div>
          <div className="flex-1">
            <Input
              label="Время"
              type="time"
              value={scheduledTime}
              onChange={(e) => setScheduledTime(e.target.value)}
            />
          </div>
        </div>

        {/* Subscriber timezone */}
        <label className="flex items-start gap-3 cursor-pointer mt-2">
          <input
            type="checkbox"
            checked={useSubscriberTimezone}
            onChange={(e) => setUseSubscriberTimezone(e.target.checked)}
            className="rounded mt-0.5"
          />
          <div>
            <span className="text-sm font-medium text-gray-700">
              По часовому поясу абонента
            </span>
            <p className="text-xs text-gray-500 mt-0.5">
              Каждый получатель получит SMS в указанное время по своему часовому поясу,
              определённому по номеру телефона
            </p>
          </div>
        </label>

        {/* Validation hint */}
        {scheduledDate && scheduledTime &&
          new Date(`${scheduledDate}T${scheduledTime}`) <= new Date(Date.now() + 5 * 60 * 1000) && (
          <p className="text-sm text-red-600">
            Время отправки должно быть не менее чем через 5 минут от текущего момента.
          </p>
        )}
      </div>
    )}

    {validationErrors.schedule && (
      <p className="text-sm text-red-600">Укажите корректную дату и время отправки.</p>
    )}
  </div>
)}
```

- [ ] **Step 2: Проверить TypeScript**

```bash
cd c:/projects/sms/portal-frontend
npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx
git commit -m "feat(wizard): шаг 3 — Расписание с часовым поясом абонента"
```

---

## Task 9: Фронтенд — шаг 4 «Подтверждение» + навигация

**Files:**
- Modify: `portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx`

- [ ] **Step 1: Заменить блок `step === 'confirm'`**

```tsx
{step === 'confirm' && (
  <div className="space-y-5 max-w-xl">
    <h3 className="text-lg font-medium text-gray-900">Подтверждение</h3>

    {/* Campaign name */}
    <div className="space-y-1">
      <label className="text-sm font-medium text-gray-700">Название рассылки</label>
      {editingName ? (
        <Input
          type="text"
          value={campaignName}
          onChange={(e) => setCampaignName(e.target.value)}
          onBlur={() => setEditingName(false)}
          autoFocus
        />
      ) : (
        <div className="flex items-center gap-2">
          <span className="text-sm text-gray-900">{campaignName}</span>
          <button
            type="button"
            onClick={() => setEditingName(true)}
            className="text-gray-400 hover:text-gray-600"
            aria-label="Редактировать название"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
            </svg>
          </button>
        </div>
      )}
    </div>

    {/* Summary blocks */}
    <dl className="bg-gray-50 rounded-lg divide-y divide-gray-200 text-sm overflow-hidden border border-gray-200">
      {/* Message */}
      <div className="p-3 flex justify-between items-start gap-2">
        <div className="min-w-0">
          <dt className="text-gray-500 text-xs mb-1">Сообщение</dt>
          <dd className="text-gray-900 truncate">
            {(messageText || selectedTemplate?.body || '—').slice(0, 100)}
            {(messageText || selectedTemplate?.body || '').length > 100 ? '…' : ''}
          </dd>
          <dd className="text-gray-500 text-xs mt-0.5">
            Отправитель: {senderNames.find((s) => s.id === senderNameId)?.name || '—'}
          </dd>
        </div>
        <button type="button" onClick={() => setStep('message')}
          className="shrink-0 text-xs text-blue-600 hover:underline">изменить</button>
      </div>

      {/* A/B */}
      {abEnabled && (
        <div className="p-3 flex justify-between items-start gap-2">
          <div className="min-w-0">
            <dt className="text-gray-500 text-xs mb-1">A/B тестирование</dt>
            <dd className="text-gray-900 truncate">
              Вариант B: {(abTextB || abTemplateB?.body || '—').slice(0, 80)}…
            </dd>
            <dd className="text-gray-500 text-xs mt-0.5">
              Доля: {abSplitPercent}% · Время: {abDurationHours}ч ·{' '}
              Метрика: {abMetric === 'delivery_rate' ? 'Доставка' : abMetric === 'click_rate' ? 'CTR' : 'Уник. CTR'}
            </dd>
          </div>
          <button type="button" onClick={() => setStep('message')}
            className="shrink-0 text-xs text-blue-600 hover:underline">изменить</button>
        </div>
      )}

      {/* Audience */}
      <div className="p-3 flex justify-between items-start gap-2">
        <div className="min-w-0">
          <dt className="text-gray-500 text-xs mb-1">Аудитория</dt>
          <dd className="text-gray-900">
            {contactLists.find((l) => l.id === contactListId)?.name ?? '—'} ·{' '}
            {(contactLists.find((l) => l.id === contactListId)?.contacts_count ?? 0).toLocaleString()} контактов
          </dd>
          {(excludeCountries.length > 0 || excludeOperators.length > 0) && (
            <dd className="text-gray-500 text-xs mt-0.5">
              Исключены: {[...excludeCountries, ...excludeOperators].join(', ')}
            </dd>
          )}
        </div>
        <button type="button" onClick={() => setStep('audience')}
          className="shrink-0 text-xs text-blue-600 hover:underline">изменить</button>
      </div>

      {/* Schedule */}
      <div className="p-3 flex justify-between items-start gap-2">
        <div className="min-w-0">
          <dt className="text-gray-500 text-xs mb-1">Расписание</dt>
          <dd className="text-gray-900">
            {sendMode === 'now' ? 'Сейчас' : `${scheduledDate ? new Date(`${scheduledDate}T${scheduledTime}`).toLocaleString('ru', { day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' }) : '—'}`}
          </dd>
          {sendMode === 'later' && (
            <dd className="text-gray-500 text-xs mt-0.5">
              По часовому поясу абонента: {useSubscriberTimezone ? 'Да' : 'Нет'}
            </dd>
          )}
        </div>
        <button type="button" onClick={() => setStep('schedule')}
          className="shrink-0 text-xs text-blue-600 hover:underline">изменить</button>
      </div>
    </dl>

    {/* Cost estimate */}
    <div className="bg-gray-50 rounded-lg p-4 border border-gray-200">
      <h4 className="text-sm font-medium text-gray-700 mb-2">Предварительная стоимость</h4>
      {costLoading ? (
        <div className="space-y-2 animate-pulse">
          <div className="h-4 bg-gray-200 rounded w-3/4" />
          <div className="h-4 bg-gray-200 rounded w-1/2" />
        </div>
      ) : costEstimate ? (
        <div className="space-y-1 text-sm">
          <div className="flex justify-between">
            <span className="text-gray-500">Получателей</span>
            <span className="font-medium">{costEstimate.recipients.toLocaleString()}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-gray-500">Частей SMS</span>
            <span className="font-medium">{costEstimate.segments_per_msg}</span>
          </div>
          <div className="flex justify-between border-t border-gray-200 pt-1 mt-1">
            <span className="text-gray-700 font-medium">Итого</span>
            <span className="font-bold text-base">{costEstimate.estimated_cost} ₽</span>
          </div>
          <div className="flex justify-between items-center">
            <span className="text-gray-500">Баланс</span>
            <span className={costEstimate.balance_sufficient ? 'text-green-600' : 'text-red-600'}>
              {costEstimate.current_balance} ₽{' '}
              {costEstimate.balance_sufficient ? '✓' : '⚠ Недостаточно'}
            </span>
          </div>
        </div>
      ) : (
        <p className="text-sm text-gray-400">Не удалось рассчитать стоимость</p>
      )}
    </div>
  </div>
)}
```

- [ ] **Step 2: Заменить блок навигационных кнопок**

Найти блок `{/* Navigation buttons */}` и заменить:

```tsx
{/* Navigation buttons */}
<div className="flex items-center justify-between mt-8 pt-4 border-t border-gray-200">
  <div className="flex gap-2">
    <Button
      variant="secondary"
      onClick={() => setShowCancelDialog(true)}
    >
      Отмена
    </Button>
    {currentStepIdx > 0 && (
      <Button variant="secondary" onClick={prevStep}>
        Назад
      </Button>
    )}
  </div>

  <div className="flex gap-2">
    <Button
      variant="secondary"
      onClick={handleSaveDraft}
      disabled={savingDraft}
    >
      {savingDraft ? 'Сохранение...' : 'Сохранить как черновик'}
    </Button>

    {step === 'confirm' ? (
      <Button
        onClick={handleLaunch}
        disabled={submitting || (costEstimate !== null && !costEstimate.balance_sufficient)}
      >
        {submitting
          ? 'Создание...'
          : sendMode === 'now'
            ? 'Отправить'
            : 'Запланировать'}
      </Button>
    ) : (
      <Button onClick={nextStep}>
        Далее
      </Button>
    )}
  </div>
</div>

{/* Cancel dialog */}
<ConfirmDialog
  open={showCancelDialog}
  title="Отменить создание рассылки?"
  onConfirm={handleSaveDraft}
  onCancel={() => { setShowCancelDialog(false); navigate('/campaigns'); }}
  confirmLabel={savingDraft ? 'Сохранение...' : 'Сохранить как черновик'}
  cancelLabel="Не сохранять"
/>
```

- [ ] **Step 3: Исправить `submitting` → `setSaving` (Task 5 ввёл переименование)**

Убедиться, что везде используется `setSaving` вместо `setSubmitting`. Найти все вхождения:

```bash
grep -n "setSubmitting\|submitting" portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx
```

Если осталось `setSubmitting` — заменить на `setSaving`, `submitting` на `submitting` (переменная осталась `submitting` через `useState` — убедиться что destructuring правильный):

```typescript
// В Task 5 стейт объявлен как:
const [submitting, setSaving] = useState(false);
// Это корректно — submitting читается в JSX, setSaving вызывается в handleLaunch
```

- [ ] **Step 4: Проверить TypeScript**

```bash
cd c:/projects/sms/portal-frontend
npx tsc --noEmit
```

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx
git commit -m "feat(wizard): шаг 4 — Подтверждение, навигация, черновики"
```

---

## Task 10: Финальная проверка и сборка

**Files:** — (только проверка)

- [ ] **Step 1: Полная сборка фронтенда**

```bash
cd c:/projects/sms/portal-frontend
npm run build
```

Ожидаемый вывод: успешная сборка без ошибок TypeScript.

- [ ] **Step 2: Полная сборка бэкенда**

```bash
cd c:/projects/sms
go build ./...
```

Ожидаемый вывод: без ошибок.

- [ ] **Step 3: Запустить существующие тесты бэкенда**

```bash
cd c:/projects/sms
go test ./internal/gateway/portal/...
```

- [ ] **Step 4: Запустить сервер и вручную проверить визард**

```bash
# Запустить dev-сервер фронтенда
cd c:/projects/sms/portal-frontend
npm run dev
```

Проверить вручную:
- [ ] Открыть `/campaigns/new` — отображаются 4 шага в StepIndicator
- [ ] Шаг 1: вводить текст, выбирать шаблон, выбирать отправителя, включать A/B
- [ ] A/B тест: вариант B, слайдер 5-50%, время, метрика
- [ ] Шаг 2: выбор базы с количеством, фильтры разворачиваются, сводка
- [ ] Шаг 3: переключатель Сейчас/Позже, дата-время, чекбокс TZ
- [ ] Шаг 4: автоимя редактируется, сводка, стоимость, кнопки
- [ ] Кнопка "Отмена" → ConfirmDialog → "Сохранить как черновик" / "Не сохранять"
- [ ] "Сохранить как черновик" → редирект на `/campaigns`
- [ ] Клик по пройденному шагу в StepIndicator → переход назад

- [ ] **Step 5: Итоговый commit**

```bash
cd c:/projects/sms
git add -A
git status  # убедиться что нет лишних файлов
git commit -m "feat(wizard): редизайн визарда кампаний 6→4 шага (Сообщение, Аудитория, Расписание, Подтверждение)"
```
