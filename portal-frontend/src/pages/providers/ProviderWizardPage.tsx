import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { providersApi, ApiError, type CreateProviderRequest } from '../../api/client';
import { WizardProgress } from './components/WizardProgress';
import { WizardNav } from './components/WizardNav';
import { Step1BasicInfo } from './steps/Step1BasicInfo';
import { Step2Connection } from './steps/Step2Connection';
import { Step3Params } from './steps/Step3Params';
import { Step4Test } from './steps/Step4Test';
import { Step5Routing } from './steps/Step5Routing';
import { Step6Summary } from './steps/Step6Summary';

const STEP_LABELS = ['Basic Info', 'Connection', 'Parameters', 'Test', 'Routing', 'Summary'];
const TOTAL_STEPS = 6;

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
  const [step, setStep] = useState(1);
  const [data, setData] = useState<Partial<CreateProviderRequest>>(DEFAULTS);
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);

  function update(updates: Partial<CreateProviderRequest>) {
    setData(prev => ({ ...prev, ...updates }));
  }

  function validateStep(s: number): string {
    if (s === 1 && !data.name?.trim()) return 'Name is required';
    if (s === 2) {
      if (!data.host?.trim()) return 'Host is required';
      if (!data.port || data.port <= 0) return 'Valid port is required';
      if (!data.system_id?.trim()) return 'System ID is required';
      if (!data.password?.trim()) return 'Password is required';
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
      await providersApi.create(data as CreateProviderRequest);
      navigate('/providers');
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to create provider');
    } finally {
      setSaving(false);
    }
  }

  return (
    <div style={{ maxWidth: 680, margin: '0 auto' }}>
      <h2>Add SMPP Provider</h2>
      <WizardProgress currentStep={step} totalSteps={TOTAL_STEPS} labels={STEP_LABELS} />

      <div style={{ padding: 24, border: '1px solid #e0e0e0', borderRadius: 8 }}>
        {step === 1 && <Step1BasicInfo data={data} onChange={update} />}
        {step === 2 && <Step2Connection data={data} onChange={update} />}
        {step === 3 && <Step3Params data={data} onChange={update} />}
        {step === 4 && <Step4Test data={data} />}
        {step === 5 && <Step5Routing data={data} onChange={update} />}
        {step === 6 && <Step6Summary data={data} />}

        {error && <p style={{ color: '#d32f2f', marginTop: 12 }}>{error}</p>}

        <WizardNav
          currentStep={step}
          totalSteps={TOTAL_STEPS}
          onBack={handleBack}
          onNext={handleNext}
          onSubmit={handleSubmit}
          loading={saving}
        />
      </div>
    </div>
  );
}
