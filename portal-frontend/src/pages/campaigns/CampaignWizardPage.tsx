import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { campaignsApi } from '../../api/campaigns';
import { contactListsApi, type ContactList } from '../../api/contacts';
import { ApiError } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Select } from '../../components/ui/Select';
import { TemplatePreview } from '../../components/campaigns/TemplatePreview';

type WizardStep = 'basics' | 'message' | 'schedule' | 'retry' | 'confirm';

const STEPS: WizardStep[] = ['basics', 'message', 'schedule', 'retry', 'confirm'];
const STEP_LABELS: Record<WizardStep, string> = {
  basics: '1. Основное',
  message: '2. Сообщение',
  schedule: '3. Расписание',
  retry: '4. Повторы',
  confirm: '5. Подтверждение',
};

export function CampaignWizardPage() {
  const navigate = useNavigate();
  const [step, setStep] = useState<WizardStep>('basics');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  // Step 1: Basics
  const [name, setName] = useState('');
  const [contactListId, setContactListId] = useState('');
  const [source, setSource] = useState('');
  const [contactLists, setContactLists] = useState<ContactList[]>([]);

  // Step 2: Message
  const [templateId, setTemplateId] = useState('');
  const [messageText, setMessageText] = useState('');

  // Step 3: Schedule
  const [sendMode, setSendMode] = useState<'now' | 'scheduled'>('now');
  const [sendRate, setSendRate] = useState(100);

  // Step 4: Retry
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
    if (currentStepIdx < STEPS.length - 1) {
      setStep(STEPS[currentStepIdx + 1]);
    }
  }

  function prevStep() {
    if (currentStepIdx > 0) {
      setStep(STEPS[currentStepIdx - 1]);
    }
  }

  function canProceed(): boolean {
    switch (step) {
      case 'basics':
        return name.trim().length > 0 && contactListId.length > 0;
      case 'message':
        return templateId.trim().length > 0 || messageText.trim().length > 0;
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
      <nav aria-label="Шаги создания рассылки" className="mb-6">
        <ol className="flex items-center gap-2">
          {STEPS.map((s, idx) => {
            const isActive = s === step;
            const isPast = currentStepIdx > idx;
            const stepStatus = isActive ? 'текущий' : isPast ? 'завершён' : 'ожидает';
            return (
              <li key={s} className="flex items-center gap-2">
                {idx > 0 && (
                  <div
                    className={`w-8 h-0.5 ${isPast ? 'bg-blue-500' : 'bg-gray-300'}`}
                    aria-hidden="true"
                  />
                )}
                <button
                  onClick={() => isPast && setStep(s)}
                  disabled={!isPast}
                  aria-label={`${STEP_LABELS[s]} — ${stepStatus}`}
                  aria-current={isActive ? 'step' : undefined}
                  className={`px-3 py-1.5 rounded-full text-sm font-medium transition-colors ${
                    isActive
                      ? 'bg-blue-100 text-blue-700'
                      : isPast
                        ? 'bg-green-100 text-green-700 cursor-pointer hover:bg-green-200'
                        : 'bg-gray-100 text-gray-500'
                  }`}
                >
                  {STEP_LABELS[s]}
                </button>
              </li>
            );
          })}
        </ol>
      </nav>

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
                label: `${l.name} (${l.contacts_count.toLocaleString()} контактов)`,
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
            <div>
              <Input
                label="ID шаблона"
                type="text"
                value={templateId}
                onChange={(e) => setTemplateId(e.target.value)}
                placeholder="Введите ID шаблона из библиотеки"
              />
              <p className="text-xs text-gray-500 mt-1">
                Укажите ID ранее созданного шаблона сообщения
              </p>
            </div>
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

        {/* Step 3: Schedule */}
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
                      ({selectedList.contacts_count.toLocaleString()} контактов)
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
                  {templateId || 'Произвольный текст'}
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
