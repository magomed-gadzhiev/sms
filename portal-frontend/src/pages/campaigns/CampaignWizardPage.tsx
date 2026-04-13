import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { campaignsApi } from '../../api/campaigns';
import { contactListsApi, type ContactList, type ContactListSegments } from '../../api/contacts';
import { senderNamesApi, type SenderNameInfo, ApiError, type TemplateInfo } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Select } from '../../components/ui/Select';
import { CharacterCounter } from '../../components/ui/CharacterCounter';
import { TemplatePicker } from '../../components/campaigns/TemplatePicker';
import { StepIndicator } from '../../components/campaigns/StepIndicator';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';

type WizardStep = 'message' | 'audience' | 'schedule' | 'confirm';

const STEPS: WizardStep[] = ['message', 'audience', 'schedule', 'confirm'];
const STEP_LABELS: Record<WizardStep, string> = {
  message: '1. Сообщение',
  audience: '2. Аудитория',
  schedule: '3. Расписание',
  confirm: '4. Подтверждение',
};

export function CampaignWizardPage() {
  const navigate = useNavigate();
  const [step, setStep] = useState<WizardStep>('message');
  const [maxReachedIndex, setMaxReachedIndex] = useState(0);
  const [validationErrors, setValidationErrors] = useState<Partial<Record<WizardStep, boolean>>>({});
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
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

  // Загрузка sender names
  useEffect(() => {
    senderNamesApi.listApproved().then((res) => {
      const names = res.sender_names ?? [];
      setSenderNames(names);
      if (names.length > 0) setSenderNameId(names[0].id);
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

  const currentStepIdx = STEPS.indexOf(step);

  function nextStep() {
    if (!canProceed()) {
      setValidationErrors((prev) => ({ ...prev, [step]: true }));
      return;
    }
    setValidationErrors((prev) => ({ ...prev, [step]: false }));
    if (currentStepIdx < STEPS.length - 1) {
      const nextIdx = currentStepIdx + 1;
      setStep(STEPS[nextIdx]);
      setMaxReachedIndex((prev) => Math.max(prev, nextIdx));
    }
  }

  function prevStep() {
    if (currentStepIdx > 0) {
      setStep(STEPS[currentStepIdx - 1]);
    }
  }

  function handleStepClick(idx: number) {
    if (idx <= maxReachedIndex) {
      setStep(STEPS[idx]);
    }
  }

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

  async function handleLaunch() {
    setSubmitting(true);
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
          { name: 'Вариант A', template_id: templateId, percentage: 100 - abSplitPercent, is_control: true },
          { name: 'Вариант B', template_id: abTemplateIdB, percentage: abSplitPercent, is_control: false },
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
      setSubmitting(false);
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

  const selectedList = contactLists.find((l) => l.id === contactListId);

  return (
    <div>
      <PageHeader
        title="Создать рассылку"
        breadcrumbs={[
          { label: 'Рассылки', href: '/campaigns' },
          { label: 'Новая рассылка' },
        ]}
      />

      {/* Step indicator */}
      <div className="mb-6">
        <StepIndicator
          steps={STEPS.map((s) => ({ key: s, label: STEP_LABELS[s] }))}
          currentIndex={currentStepIdx}
          maxReachedIndex={maxReachedIndex}
          validationErrors={validationErrors as Record<string, boolean>}
          onStepClick={handleStepClick}
        />
      </div>

      {error && (
        <div role="alert" className="bg-red-50 border border-red-200 text-red-700 rounded p-3 mb-4 text-sm">
          {error}
        </div>
      )}

      <div className="bg-white border border-gray-200 rounded-lg p-6">
        {/* Step 1: Сообщение */}
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

                <TemplatePicker
                  value={abTemplateIdB}
                  selectedTemplate={abTemplateB}
                  onChange={(id, tpl) => {
                    setAbTemplateIdB(id);
                    setAbTemplateB(tpl);
                    if (id) setAbTextB('');
                  }}
                />

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

            {validationErrors.message && (
              <p className="text-sm text-red-600">Заполните текст сообщения и выберите имя отправителя.</p>
            )}
          </div>
        )}

        {/* Step 4: Schedule */}
        {step === 'schedule' && (
          <div className="space-y-4 max-w-lg">
            <h3 className="text-lg font-medium text-gray-900 mb-4">
              Расписание отправки
            </h3>
            <fieldset>
              <legend className="text-sm font-medium text-gray-700 mb-2">Режим отправки</legend>
              <div className="flex gap-4">
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="radio"
                    name="sendMode"
                    value="now"
                    checked={sendMode === 'now'}
                    onChange={() => setSendMode('now')}
                    className="text-blue-600"
                  />
                  <span className="text-sm">Отправить сейчас</span>
                </label>
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="radio"
                    name="sendMode"
                    value="scheduled"
                    checked={sendMode === 'scheduled'}
                    onChange={() => setSendMode('scheduled')}
                    className="text-blue-600"
                  />
                  <span className="text-sm">Сохранить как черновик</span>
                </label>
              </div>
            </fieldset>
            <div>
              <Input
                label="Скорость отправки (SMS/сек)"
                type="number"
                value={String(sendRate)}
                onChange={(e) => setSendRate(Number(e.target.value))}
                min={1}
                max={10000}
              />
              <p className="text-xs text-gray-500 mt-1">
                Рекомендуемая скорость: 50-500 SMS/сек
              </p>
            </div>
          </div>
        )}

        {/* Step 5: Confirm */}
        {step === 'confirm' && (
          <div className="space-y-4 max-w-lg">
            <h3 className="text-lg font-medium text-gray-900 mb-4">
              Подтверждение
            </h3>
            <dl className="bg-gray-50 rounded-lg p-4 space-y-3 text-sm">
              <div className="flex justify-between">
                <dt className="text-gray-500">Название:</dt>
                <dd className="font-medium text-gray-900">{name}</dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-gray-500">Контактная база:</dt>
                <dd className="font-medium text-gray-900">
                  {selectedList?.name ?? '—'}
                  {selectedList && (
                    <span className="text-gray-500 ml-1">
                      ({(selectedList.contacts_count ?? 0).toLocaleString()} контактов)
                    </span>
                  )}
                </dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-gray-500">Отправитель:</dt>
                <dd className="font-medium text-gray-900">
                  {source || 'По умолчанию'}
                </dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-gray-500">Шаблон:</dt>
                <dd className="font-medium text-gray-900">
                  {selectedTemplate ? selectedTemplate.name : templateId ? templateId : 'Произвольный текст'}
                </dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-gray-500">Режим:</dt>
                <dd className="font-medium text-gray-900">
                  {sendMode === 'now' ? 'Отправить сейчас' : 'Черновик'}
                </dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-gray-500">Скорость:</dt>
                <dd className="font-medium text-gray-900">
                  {sendRate} SMS/сек
                </dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-gray-500">A/B Тест:</dt>
                <dd className="font-medium text-gray-900">
                  {abEnabled
                    ? `Вкл. (сплит ${abSplitPercent}%, ${abDurationHours}ч, метрика: ${abMetric === 'delivery_rate' ? 'доставляемость' : 'клики'})`
                    : 'Нет'}
                </dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-gray-500">Повторы:</dt>
                <dd className="font-medium text-gray-900">
                  {retryEnabled
                    ? `Да (${maxRetries}x, через ${retryDelay}ч)`
                    : 'Нет'}
                </dd>
              </div>
            </dl>
          </div>
        )}

        {/* Navigation buttons */}
        <div className="flex justify-between mt-8 pt-4 border-t border-gray-200">
          <Button
            variant="secondary"
            onClick={currentStepIdx === 0 ? () => navigate('/campaigns') : prevStep}
          >
            {currentStepIdx === 0 ? 'Отмена' : 'Назад'}
          </Button>
          {step === 'confirm' ? (
            <Button onClick={handleLaunch} disabled={submitting}>
              {submitting
                ? 'Создание...'
                : sendMode === 'now'
                  ? 'Запустить рассылку'
                  : 'Сохранить черновик'}
            </Button>
          ) : (
            <Button onClick={nextStep} disabled={!canProceed()}>
              Далее
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}
