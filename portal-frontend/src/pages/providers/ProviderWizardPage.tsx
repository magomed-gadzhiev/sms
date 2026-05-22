import { useState, useEffect } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { providersApi, ApiError, type CreateProviderRequest } from '../../api/client';
import { usePageTitle } from '../../contexts/PageTitleContext';
import { WizardProgress } from './components/WizardProgress';
import { WizardNav } from './components/WizardNav';
import { Step1BasicInfo } from './steps/Step1BasicInfo';
import { Step2Connection } from './steps/Step2Connection';
import { Step3Params } from './steps/Step3Params';
import { Step4Test } from './steps/Step4Test';
import { Step6Summary } from './steps/Step6Summary';

const STEP_LABELS = ['Основное', 'Подключение', 'Параметры', 'Тест', 'Итоги'];
const TOTAL_STEPS = 5;

const DEFAULTS: Partial<CreateProviderRequest> = {
  bind_type: 0,
  max_connections: 1,
  window_size: 10,
  tps_limit: 100,
  tags: [],
  routing_rules: [],
};

export function ProviderWizardPage() {
  const navigate = useNavigate();
  const { id: editId } = useParams<{ id: string }>();
  const isEdit = !!editId;
  const [step, setStep] = useState(1);
  const [data, setData] = useState<Partial<CreateProviderRequest>>(DEFAULTS);
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);
  const [loading, setLoading] = useState(isEdit);

  usePageTitle(isEdit ? 'Редактирование подключения' : 'Добавить подключение');

  useEffect(() => {
    if (!editId) return;
    setLoading(true);
    setError('');
    providersApi.get(editId)
      .then((p) => {
        // Provider response не содержит password — он просто не передаётся
        // в форму. Если пользователь оставит password пустым, payload пойдёт
        // без поля; если введёт новый — заменит на бэке.
        setData({
          name: p.name,
          description: p.description,
          tags: p.tags ?? [],
          host: p.host,
          port: p.port,
          system_id: p.system_id,
          bind_type: p.bind_type,
          window_size: p.window_size,
          max_connections: p.max_connections,
          tps_limit: p.tps_limit,
          routing_rules: p.routing_rules ?? [],
        });
      })
      .catch((err) => {
        setError(err instanceof ApiError ? err.message : 'Не удалось загрузить подключение');
      })
      .finally(() => setLoading(false));
  }, [editId]);

  function update(updates: Partial<CreateProviderRequest>) {
    setData(prev => ({ ...prev, ...updates }));
  }

  function validateStep(s: number): string {
    if (s === 1 && !data.name?.trim()) return 'Название обязательно';
    if (s === 2) {
      if (!data.host?.trim()) return 'Хост обязателен';
      if (!data.port || data.port <= 0) return 'Укажите корректный порт';
      if (!data.system_id?.trim()) return 'System ID обязателен';
      // В режиме редактирования пароль не обязателен — пустое поле = «не менять»
      if (!isEdit && !data.password?.trim()) return 'Пароль обязателен';
    }
    return '';
  }

  function handleNext() {
    const err = validateStep(step);
    if (err) { setError(err); return; }
    setError('');
    setStep(s => s + 1);
  }

  function handleBack() {
    setError('');
    setStep(s => s - 1);
  }

  async function handleSubmit() {
    setSaving(true);
    setError('');
    try {
      if (isEdit && editId) {
        // Если password пустой — не отправляем поле (бэк трактует как «не менять»)
        const { password, ...rest } = data;
        const payload: Partial<CreateProviderRequest> = password?.trim()
          ? { ...rest, password }
          : rest;
        await providersApi.update(editId, payload);
      } else {
        await providersApi.create(data as CreateProviderRequest);
      }
      navigate('/providers');
    } catch (err) {
      setError(err instanceof ApiError ? err.message : isEdit ? 'Не удалось сохранить подключение' : 'Не удалось создать подключение');
    } finally {
      setSaving(false);
    }
  }

  if (loading) {
    return (
      <div className="max-w-2xl mx-auto">
        <div className="p-6 border border-gray-200 rounded-lg" role="status">
          Загрузка...
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-2xl mx-auto">
      <WizardProgress currentStep={step} totalSteps={TOTAL_STEPS} labels={STEP_LABELS} />

      <div className="p-6 border border-gray-200 rounded-lg">
        {step === 1 && <Step1BasicInfo data={data} onChange={update} />}
        {step === 2 && <Step2Connection data={data} onChange={update} isEdit={isEdit} />}
        {step === 3 && <Step3Params data={data} onChange={update} />}
        {step === 4 && <Step4Test data={data} isEdit={isEdit} />}
        {step === 5 && <Step6Summary data={data} />}

        {error && <p className="text-red-600 mt-3">{error}</p>}

        <WizardNav
          currentStep={step}
          totalSteps={TOTAL_STEPS}
          onBack={handleBack}
          onNext={handleNext}
          onSubmit={handleSubmit}
          submitLabel={isEdit ? 'Сохранить' : 'Создать'}
          loading={saving}
        />
      </div>
    </div>
  );
}
