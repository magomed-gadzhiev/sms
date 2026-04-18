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

function pluralContacts(n: number): string {
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) return `${n.toLocaleString()} контакт`;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return `${n.toLocaleString()} контакта`;
  return `${n.toLocaleString()} контактов`;
}

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
  const [sendersError, setSendersError] = useState('');
  const [contactListsError, setContactListsError] = useState('');

  // A/B тест (часть шага 1)
  const [abEnabled, setAbEnabled] = useState(false);
  const [abMessageTextB, setAbMessageTextB] = useState('');
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
    setSendersError('');
    senderNamesApi.listApproved().then((res) => {
      const names = res.sender_names ?? [];
      setSenderNames(names);
      if (names.length > 0) setSenderNameId(names[0].id);
    }).catch((err) => {
      setSendersError(err instanceof ApiError ? err.message : 'Не удалось загрузить имена отправителей');
    });
  }, []);

  // Загрузка контактных баз
  useEffect(() => {
    setContactListsError('');
    contactListsApi.list(1, 100).then((res) => setContactLists(res.items ?? [])).catch((err) => {
      setContactListsError(err instanceof ApiError ? err.message : 'Не удалось загрузить контактные базы');
    });
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
        if (!messageText.trim()) return false;
        if (!senderNameId) return false;
        if (abEnabled) {
          if (!abMessageTextB.trim()) return false;
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
          ? JSON.stringify({ exclude_countries: excludeCountries, exclude_operators: excludeOperators })
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
    const listId = contactListId || contactLists[0]?.id;
    if (!listId) {
      setError('Нет контактных баз для сохранения черновика. Сначала создайте базу контактов.');
      return;
    }
    setSavingDraft(true);
    setError('');
    try {
      const senderName = senderNames.find((s) => s.id === senderNameId);
      await campaignsApi.create({
        name: campaignName.trim() || `Черновик ${new Date().toLocaleDateString('ru')}`,
        contact_list_id: listId,
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

            {/* Template picker — required */}
            {/* Textarea — primary input per spec */}
            <div className="space-y-1">
              <label htmlFor="msg-text" className="text-sm font-medium text-gray-700">
                Текст сообщения *
              </label>
              <textarea
                id="msg-text"
                value={messageText}
                onChange={(e) => setMessageText(e.target.value)}
                rows={3}
                className="w-full rounded border border-gray-300 px-3 py-2 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50 focus-visible:border-primary"
                placeholder="Введите текст сообщения..."
              />
              <div className="flex justify-between items-center">
                <CharacterCounter current={messageText.length} max={160} />
              </div>
            </div>

            {/* Template picker — optional, used to pre-fill textarea */}
            <TemplatePicker
              value={templateId}
              selectedTemplate={selectedTemplate}
              onChange={(id, tpl) => {
                setTemplateId(id);
                setSelectedTemplate(tpl);
                if (id && tpl?.body) setMessageText(tpl.body);
              }}
            />

            {/* Sender name */}
            {sendersError ? (
              <p className="text-xs text-red-600">
                {sendersError}{' '}
                <button
                  type="button"
                  className="underline font-medium"
                  onClick={() => {
                    setSendersError('');
                    senderNamesApi.listApproved().then((res) => {
                      const names = res.sender_names ?? [];
                      setSenderNames(names);
                      if (names.length > 0) setSenderNameId(names[0].id);
                    }).catch((err) => setSendersError(err instanceof ApiError ? err.message : 'Ошибка'));
                  }}
                >
                  Повторить
                </button>
              </p>
            ) : (
              <>
                <Select
                  label="Имя отправителя *"
                  value={senderNameId}
                  onChange={setSenderNameId}
                  options={senderNames.map((s) => ({ value: s.id, label: s.name }))}
                  placeholder="-- Выберите отправителя --"
                />
                {senderNames.length === 0 && (
                  <p className="text-xs text-amber-600 -mt-1">
                    У вас нет одобренных имён отправителей.{' '}
                    <a href="/sender-names" className="underline font-medium">Зарегистрировать →</a>
                  </p>
                )}
              </>
            )}

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

                {/* Textarea — primary input for variant B */}
                <div className="space-y-1">
                  <label htmlFor="msg-text-b" className="text-sm font-medium text-gray-700">
                    Текст варианта B *
                  </label>
                  <textarea
                    id="msg-text-b"
                    value={abMessageTextB}
                    onChange={(e) => setAbMessageTextB(e.target.value)}
                    rows={3}
                    className="w-full rounded border border-gray-300 px-3 py-2 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50 focus-visible:border-primary"
                    placeholder="Введите текст варианта B..."
                  />
                  <CharacterCounter current={abMessageTextB.length} max={160} />
                </div>

                {/* Template picker — optional, pre-fills variant B textarea */}
                <TemplatePicker
                  value={abTemplateIdB}
                  selectedTemplate={abTemplateB}
                  onChange={(id, tpl) => {
                    setAbTemplateIdB(id);
                    setAbTemplateB(tpl);
                    if (id && tpl?.body) setAbMessageTextB(tpl.body);
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
              <p className="text-sm text-red-600">
                {!messageText.trim() && !senderNameId
                  ? 'Введите текст сообщения и укажите имя отправителя.'
                  : !messageText.trim()
                  ? 'Введите текст сообщения.'
                  : abEnabled && !abMessageTextB.trim()
                  ? 'Введите текст варианта B.'
                  : senderNames.length === 0
                  ? 'Нет одобренных имён отправителей — сначала зарегистрируйте имя в разделе «Имена отправителей».'
                  : 'Выберите имя отправителя.'}
              </p>
            )}
          </div>
        )}

        {/* Step 2: Аудитория */}
        {step === 'audience' && (
          <div className="space-y-5 max-w-xl">
            <h3 className="text-lg font-medium text-gray-900">Аудитория</h3>

            {contactListsError && (
              <p className="text-xs text-red-600">{contactListsError}</p>
            )}
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
                label: `${l.name} (${pluralContacts(l.contacts_count ?? 0)})`,
              }))}
              placeholder="-- Выберите базу контактов --"
            />

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

                {(() => {
                  const list = contactLists.find((l) => l.id === contactListId);
                  const total = list?.contacts_count ?? 0;
                  const hasFilters = excludeCountries.length > 0 || excludeOperators.length > 0;
                  return (
                    <div className="bg-gray-50 rounded-lg p-3 text-sm">
                      <span>
                        Контактов к отправке: <strong>{total.toLocaleString()}</strong>
                        {hasFilters && <span className="text-gray-500 ml-1">(фильтры применяются при запуске)</span>}
                        {abEnabled && (
                          <span className="block text-gray-500 mt-1">
                            Тестовая группа: ~{Math.round(total * abSplitPercent / 100).toLocaleString()} контактов ({abSplitPercent}%)
                          </span>
                        )}
                      </span>
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

        {/* Step 3: Расписание */}
        {step === 'schedule' && (
          <div className="space-y-5 max-w-xl">
            <h3 className="text-lg font-medium text-gray-900">Расписание</h3>

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

                <label className="flex items-start gap-3 mt-2 cursor-pointer">
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
                      определённому по номеру телефона.
                    </p>
                  </div>
                </label>

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

        {/* Step 4: Подтверждение */}
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

            {/* Summary */}
            <dl className="bg-gray-50 rounded-lg divide-y divide-gray-200 text-sm overflow-hidden border border-gray-200">
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

              {abEnabled && (
                <div className="p-3 flex justify-between items-start gap-2">
                  <div className="min-w-0">
                    <dt className="text-gray-500 text-xs mb-1">A/B тестирование</dt>
                    <dd className="text-gray-900 truncate">
                      Вариант B: {(abTemplateB?.body || '—').slice(0, 80)}
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

              <div className="p-3 flex justify-between items-start gap-2">
                <div className="min-w-0">
                  <dt className="text-gray-500 text-xs mb-1">Аудитория</dt>
                  <dd className="text-gray-900">
                    {contactLists.find((l) => l.id === contactListId)?.name ?? '—'} ·{' '}
                    {pluralContacts(contactLists.find((l) => l.id === contactListId)?.contacts_count ?? 0)}
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

              <div className="p-3 flex justify-between items-start gap-2">
                <div className="min-w-0">
                  <dt className="text-gray-500 text-xs mb-1">Расписание</dt>
                  <dd className="text-gray-900">
                    {sendMode === 'now' ? 'Сейчас' : (scheduledDate && scheduledTime
                      ? new Date(`${scheduledDate}T${scheduledTime}`).toLocaleString('ru', { day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' })
                      : '—')}
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
                    <span className="font-bold text-base">
                      {parseFloat(costEstimate.estimated_cost).toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ₽
                    </span>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-gray-500">Баланс</span>
                    <span className={costEstimate.balance_sufficient ? 'text-green-600' : 'text-red-600'}>
                      {parseFloat(costEstimate.current_balance).toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ₽{' '}
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

        <ConfirmDialog
          open={showCancelDialog}
          title="Отменить создание рассылки?"
          description="Хотите сохранить текущий прогресс как черновик или выйти без сохранения?"
          onConfirm={handleSaveDraft}
          onCancel={() => { setShowCancelDialog(false); navigate('/campaigns'); }}
          confirmLabel={savingDraft ? 'Сохранение...' : 'Сохранить как черновик'}
          loading={savingDraft}
        />
      </div>
    </div>
  );
}
