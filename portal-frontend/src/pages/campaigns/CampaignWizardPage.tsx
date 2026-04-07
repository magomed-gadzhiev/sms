import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { campaignsApi } from '../../api/campaigns';
import { contactListsApi, type ContactList } from '../../api/contacts';
import { ApiError, type TemplateInfo } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Select } from '../../components/ui/Select';
import { TemplatePreview } from '../../components/campaigns/TemplatePreview';
import { TemplatePicker } from '../../components/campaigns/TemplatePicker';
import { StepIndicator } from '../../components/campaigns/StepIndicator';

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

export function CampaignWizardPage() {
  const navigate = useNavigate();
  const [step, setStep] = useState<WizardStep>('basics');
  const [maxReachedIndex, setMaxReachedIndex] = useState(0);
  const [validationErrors, setValidationErrors] = useState<Partial<Record<WizardStep, boolean>>>({});
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  // Step 1: Basics
  const [name, setName] = useState('');
  const [contactListId, setContactListId] = useState('');
  const [source, setSource] = useState('');
  const [contactLists, setContactLists] = useState<ContactList[]>([]);

  // Step 2: Message
  const [templateId, setTemplateId] = useState('');
  const [selectedTemplate, setSelectedTemplate] = useState<TemplateInfo | null>(null);
  const [messageText, setMessageText] = useState('');

  // Step 3: A/B Test
  const [abEnabled, setAbEnabled] = useState(false);
  const [abSplitPercent, setAbSplitPercent] = useState(20);
  const [abDurationHours, setAbDurationHours] = useState(24);
  const [abMetric, setAbMetric] = useState<'delivery_rate' | 'click_rate'>('delivery_rate');
  const [abVariantBTemplateId, setAbVariantBTemplateId] = useState('');
  const [abVariantBTemplate, setAbVariantBTemplate] = useState<TemplateInfo | null>(null);
  const [abVariantBText, setAbVariantBText] = useState('');

  // Step 4: Schedule
  const [sendMode, setSendMode] = useState<'now' | 'scheduled'>('now');
  const [sendRate, setSendRate] = useState(100);

  // Step 5: Retry
  const [retryEnabled, setRetryEnabled] = useState(false);
  const [retryDelay, setRetryDelay] = useState(1);
  const [maxRetries, setMaxRetries] = useState(2);

  useEffect(() => {
    contactListsApi
      .list(1, 100)
      .then((resp) => setContactLists(resp.items ?? []))
      .catch(() => {});
  }, []);

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
      case 'basics':
        return name.trim().length > 0 && contactListId.length > 0;
      case 'message':
        return templateId.trim().length > 0 || messageText.trim().length > 0;
      case 'ab_test':
        if (!abEnabled) return true;
        return abVariantBTemplateId.trim().length > 0 || abVariantBText.trim().length > 0;
      case 'schedule':
        return true;
      case 'retry':
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
      const campaign = await campaignsApi.create({
        name: name.trim(),
        contact_list_id: contactListId,
        template_id: templateId.trim() || undefined,
        source: source.trim() || undefined,
        send_rate: sendRate,
      });

      // Set A/B test config if enabled
      if (abEnabled) {
        await campaignsApi.setVariants(campaign.id, [
          {
            name: 'Вариант A',
            template_id: templateId.trim(),
            percentage: 100 - abSplitPercent,
            is_control: true,
          },
          {
            name: 'Вариант B',
            template_id: abVariantBTemplateId.trim(),
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

      // Set retry config if enabled
      if (retryEnabled) {
        await campaignsApi.setRetryConfig(campaign.id, {
          enabled: true,
          delay_hours: retryDelay,
          max_retries: maxRetries,
        });
      }

      // Launch if "now" mode
      if (sendMode === 'now') {
        await campaignsApi.launch(campaign.id);
      }

      navigate(`/campaigns/${campaign.id}`);
    } catch (err) {
      setError(
        err instanceof ApiError
          ? err.message
          : 'Ошибка при создании рассылки',
      );
    } finally {
      setSubmitting(false);
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
        {/* Step 1: Basics */}
        {step === 'basics' && (
          <div className="space-y-4 max-w-lg">
            <h3 className="text-lg font-medium text-gray-900 mb-4">
              Основные настройки
            </h3>
            <Input
              label="Название рассылки *"
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Например: Новогодняя акция"
              autoFocus
              required
            />
            <Select
              label="Контактная база *"
              value={contactListId}
              onChange={(v) => setContactListId(v)}
              options={contactLists.map((l) => ({
                value: l.id,
                label: `${l.name} (${(l.contacts_count ?? 0).toLocaleString()} контактов)`,
              }))}
              placeholder="-- Выберите базу --"
            />
            <div>
              <Input
                label="Имя отправителя (Sender ID)"
                type="text"
                value={source}
                onChange={(e) => setSource(e.target.value)}
                placeholder="Например: MyCompany"
              />
              <p className="text-xs text-gray-500 mt-1">
                Если не указан, будет использоваться ID отправителя по умолчанию
              </p>
            </div>
          </div>
        )}

        {/* Step 2: Message */}
        {step === 'message' && (
          <div className="space-y-4 max-w-lg">
            <h3 className="text-lg font-medium text-gray-900 mb-4">
              Сообщение
            </h3>
            <TemplatePicker
              value={templateId}
              selectedTemplate={selectedTemplate}
              onChange={(id, tpl) => { setTemplateId(id); setSelectedTemplate(tpl); }}
            />
            <div className="flex flex-col gap-1">
              <label htmlFor="campaign-msg-text" className="text-sm font-medium text-gray-700">
                Или текст сообщения
              </label>
              <textarea
                id="campaign-msg-text"
                value={messageText}
                onChange={(e) => setMessageText(e.target.value)}
                rows={4}
                className="rounded border border-gray-300 px-3 py-2 text-sm transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50 focus-visible:border-primary"
                placeholder="Введите текст SMS сообщения..."
                aria-describedby="campaign-msg-hint"
              />
              <p id="campaign-msg-hint" className="text-xs text-gray-500">
                {messageText.length} / 160 символов
                {messageText.length > 160 &&
                  ` (${Math.ceil(messageText.length / 153)} SMS)`}
              </p>
            </div>
            {messageText && (
              <TemplatePreview
                templateText={messageText}
                contactListId={contactListId}
              />
            )}
          </div>
        )}

        {/* Step 3: A/B Test */}
        {step === 'ab_test' && (
          <div className="space-y-6 max-w-lg">
            <h3 className="text-lg font-medium text-gray-900 mb-2">
              A/B Тестирование
            </h3>
            <p className="text-sm text-gray-500 -mt-4">
              Протестируйте два варианта сообщения и автоматически выберите победителя.
            </p>

            <label className="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                checked={abEnabled}
                onChange={(e) => setAbEnabled(e.target.checked)}
                className="rounded w-4 h-4"
              />
              <span className="text-sm font-medium text-gray-700">
                Включить A/B тест
              </span>
            </label>

            {abEnabled && (
              <div className="space-y-5 pl-6 border-l-2 border-blue-200">
                {/* Variant B message */}
                <div className="space-y-3">
                  <h4 className="text-sm font-semibold text-gray-800">Вариант B — сообщение</h4>
                  <p className="text-xs text-gray-500">
                    Вариант A — это основное сообщение, выбранное на предыдущем шаге.
                  </p>
                  <TemplatePicker
                    value={abVariantBTemplateId}
                    selectedTemplate={abVariantBTemplate}
                    onChange={(id, tpl) => {
                      setAbVariantBTemplateId(id);
                      setAbVariantBTemplate(tpl);
                      if (id) setAbVariantBText('');
                    }}
                  />
                  {!abVariantBTemplateId && (
                    <div className="flex flex-col gap-1">
                      <label htmlFor="ab-variant-b-text" className="text-sm font-medium text-gray-700">
                        Или текст варианта B
                      </label>
                      <textarea
                        id="ab-variant-b-text"
                        value={abVariantBText}
                        onChange={(e) => setAbVariantBText(e.target.value)}
                        rows={3}
                        className="rounded border border-gray-300 px-3 py-2 text-sm transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50 focus-visible:border-primary"
                        placeholder="Альтернативный текст для варианта B..."
                      />
                      <p className="text-xs text-gray-500">{abVariantBText.length} / 160 символов</p>
                    </div>
                  )}
                </div>

                {/* Split percentage */}
                <div className="space-y-2">
                  <label className="text-sm font-medium text-gray-700">
                    Доля аудитории для теста: <span className="text-blue-600 font-semibold">{abSplitPercent}%</span>
                  </label>
                  <p className="text-xs text-gray-500">
                    Вариант A: {100 - abSplitPercent}% &nbsp;·&nbsp; Вариант B: {abSplitPercent}%
                  </p>
                  <input
                    type="range"
                    min={10}
                    max={50}
                    step={5}
                    value={abSplitPercent}
                    onChange={(e) => setAbSplitPercent(Number(e.target.value))}
                    className="w-full accent-blue-600"
                    aria-label="Процент сплита"
                  />
                  <div className="flex justify-between text-xs text-gray-400">
                    <span>10%</span>
                    <span>50%</span>
                  </div>
                </div>

                {/* Test duration */}
                <Input
                  label="Время до выбора победителя (часы)"
                  type="number"
                  value={String(abDurationHours)}
                  onChange={(e) => setAbDurationHours(Number(e.target.value))}
                  min={1}
                  max={168}
                />

                {/* Winning metric */}
                <Select
                  label="Метрика победителя"
                  value={abMetric}
                  onChange={(v) => setAbMetric(v as 'delivery_rate' | 'click_rate')}
                  options={[
                    { value: 'delivery_rate', label: 'Доставляемость (delivery rate)' },
                    { value: 'click_rate', label: 'Кликабельность (click rate)' },
                  ]}
                />
              </div>
            )}

            {!abEnabled && (
              <p className="text-sm text-gray-400 italic">
                A/B тест отключён — будет использован один вариант сообщения.
              </p>
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

        {/* Step 4: Retry */}
        {step === 'retry' && (
          <div className="space-y-4 max-w-lg">
            <h3 className="text-lg font-medium text-gray-900 mb-4">
              Повторная отправка
            </h3>
            <label className="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={retryEnabled}
                onChange={(e) => setRetryEnabled(e.target.checked)}
                className="rounded"
              />
              <span className="text-sm font-medium text-gray-700">
                Повторять отправку недоставленных сообщений
              </span>
            </label>
            {retryEnabled && (
              <div className="space-y-4 pl-6 border-l-2 border-blue-200">
                <Input
                  label="Задержка перед повтором (часы)"
                  type="number"
                  value={String(retryDelay)}
                  onChange={(e) => setRetryDelay(Number(e.target.value))}
                  min={1}
                  max={72}
                />
                <Input
                  label="Максимум повторов"
                  type="number"
                  value={String(maxRetries)}
                  onChange={(e) => setMaxRetries(Number(e.target.value))}
                  min={1}
                  max={5}
                />
              </div>
            )}
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
