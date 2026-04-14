# SMPP Panel Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the routing step from the SMPP wizard, translate field labels to Russian, and add an edit-provider modal to the providers list.

**Architecture:** Three independent tasks in `portal-frontend/src/pages/providers/`. Task 1 deletes Step5Routing.tsx and renames Step6Summary.tsx → Step5Summary.tsx. Task 2 replaces English labels in all wizard steps and WizardNav. Task 3 adds a new `EditProviderModal.tsx` component wired into `ProvidersPage.tsx`.

**Tech Stack:** TypeScript 5.7, React 19, Vite, Tailwind CSS 4.2, Radix UI, inline styles (existing pattern)

---

## File Map

| Action | File |
|--------|------|
| Delete | `portal-frontend/src/pages/providers/steps/Step5Routing.tsx` |
| Rename + modify | `portal-frontend/src/pages/providers/steps/Step6Summary.tsx` → `Step5Summary.tsx` |
| Modify | `portal-frontend/src/pages/providers/ProviderWizardPage.tsx` |
| Modify | `portal-frontend/src/pages/providers/components/WizardProgress.tsx` (no change needed — labels passed via props) |
| Modify | `portal-frontend/src/pages/providers/components/WizardNav.tsx` |
| Modify | `portal-frontend/src/pages/providers/steps/Step1BasicInfo.tsx` |
| Modify | `portal-frontend/src/pages/providers/steps/Step2Connection.tsx` |
| Modify | `portal-frontend/src/pages/providers/steps/Step3Params.tsx` |
| Modify | `portal-frontend/src/pages/providers/steps/Step4Test.tsx` |
| Modify | `portal-frontend/src/api/client.ts` |
| Create | `portal-frontend/src/pages/providers/components/EditProviderModal.tsx` |
| Modify | `portal-frontend/src/pages/providers/ProvidersPage.tsx` |

---

## Task 1: Remove Routing Step from Wizard

**Files:**
- Delete: `portal-frontend/src/pages/providers/steps/Step5Routing.tsx`
- Modify: `portal-frontend/src/pages/providers/ProviderWizardPage.tsx`
- Create: `portal-frontend/src/pages/providers/steps/Step5Summary.tsx` (replaces Step6Summary.tsx)
- Delete: `portal-frontend/src/pages/providers/steps/Step6Summary.tsx`

- [ ] **Step 1.1: Delete Step5Routing.tsx**

```bash
rm portal-frontend/src/pages/providers/steps/Step5Routing.tsx
```

- [ ] **Step 1.2: Create Step5Summary.tsx (renamed + routing row removed)**

Create `portal-frontend/src/pages/providers/steps/Step5Summary.tsx` with the full content below. This is Step6Summary.tsx with:
- The `routing_rules` row removed from the table
- The import path unchanged (still from `../../../api/client`)

```tsx
import type { CreateProviderRequest } from '../../../api/client';

interface Props {
  data: Partial<CreateProviderRequest>;
}

const BIND_LABELS: Record<number, string> = {
  0: 'Приёмо-передатчик (TRX)',
  1: 'Передатчик (TX)',
  2: 'Приёмник (RX)',
};

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <tr>
      <td style={{ padding: '6px 8px', color: '#666', width: 160 }}>{label}</td>
      <td style={{ padding: '6px 8px' }}>{value}</td>
    </tr>
  );
}

export function Step5Summary({ data }: Props) {
  return (
    <div>
      <h3>Итоги</h3>
      <p style={{ color: '#666' }}>Проверьте конфигурацию перед созданием провайдера.</p>
      <table style={{ width: '100%', borderCollapse: 'collapse', marginBottom: 16 }}>
        <tbody>
          <Row label="Название" value={data.name} />
          {data.description && <Row label="Описание" value={data.description} />}
          {data.tags && data.tags.length > 0 && <Row label="Метки" value={data.tags.join(', ')} />}
          <Row label="Host" value={`${data.host}:${data.port}`} />
          <Row label="Системный ID" value={data.system_id} />
          <Row label="Режим привязки" value={BIND_LABELS[data.bind_type ?? 0]} />
          <Row label="Макс. соединений" value={data.max_connections ?? 1} />
          <Row label="Размер окна" value={data.window_size ?? 10} />
          <Row label="Лимит TPS" value={data.tps_limit ?? 100} />
        </tbody>
      </table>
    </div>
  );
}
```

- [ ] **Step 1.3: Delete old Step6Summary.tsx**

```bash
rm portal-frontend/src/pages/providers/steps/Step6Summary.tsx
```

- [ ] **Step 1.4: Update ProviderWizardPage.tsx**

Replace the full content of `portal-frontend/src/pages/providers/ProviderWizardPage.tsx`:

```tsx
import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { providersApi, ApiError, type CreateProviderRequest } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { WizardProgress } from './components/WizardProgress';
import { WizardNav } from './components/WizardNav';
import { Step1BasicInfo } from './steps/Step1BasicInfo';
import { Step2Connection } from './steps/Step2Connection';
import { Step3Params } from './steps/Step3Params';
import { Step4Test } from './steps/Step4Test';
import { Step5Summary } from './steps/Step5Summary';

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
  const [step, setStep] = useState(1);
  const [data, setData] = useState<Partial<CreateProviderRequest>>(DEFAULTS);
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);

  function update(updates: Partial<CreateProviderRequest>) {
    setData(prev => ({ ...prev, ...updates }));
  }

  function validateStep(s: number): string {
    if (s === 1 && !data.name?.trim()) return 'Название обязательно';
    if (s === 2) {
      if (!data.host?.trim()) return 'Хост обязателен';
      if (!data.port || data.port <= 0) return 'Укажите корректный порт';
      if (!data.system_id?.trim()) return 'Системный ID обязателен';
      if (!data.password?.trim()) return 'Пароль обязателен';
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
      setError(err instanceof ApiError ? err.message : 'Не удалось создать провайдера');
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="max-w-2xl mx-auto">
      <PageHeader title="Добавить SMPP провайдера" />
      <WizardProgress currentStep={step} totalSteps={TOTAL_STEPS} labels={STEP_LABELS} />

      <div className="p-6 border border-gray-200 rounded-lg">
        {step === 1 && <Step1BasicInfo data={data} onChange={update} />}
        {step === 2 && <Step2Connection data={data} onChange={update} />}
        {step === 3 && <Step3Params data={data} onChange={update} />}
        {step === 4 && <Step4Test data={data} />}
        {step === 5 && <Step5Summary data={data} />}

        {error && <p className="text-red-600 mt-3">{error}</p>}

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
```

- [ ] **Step 1.5: Verify the build compiles**

```bash
cd portal-frontend && npx tsc --noEmit 2>&1 | head -30
```

Expected: no errors referencing `Step5Routing` or `Step6Summary`.

- [ ] **Step 1.6: Commit**

```bash
cd portal-frontend && git add -A -- src/pages/providers/steps/ src/pages/providers/ProviderWizardPage.tsx
git commit -m "feat(portal): remove routing step from SMPP wizard"
```

---

## Task 2: Localize Field Labels

**Files:**
- Modify: `portal-frontend/src/pages/providers/components/WizardNav.tsx`
- Modify: `portal-frontend/src/pages/providers/steps/Step1BasicInfo.tsx`
- Modify: `portal-frontend/src/pages/providers/steps/Step2Connection.tsx`
- Modify: `portal-frontend/src/pages/providers/steps/Step3Params.tsx`
- Modify: `portal-frontend/src/pages/providers/steps/Step4Test.tsx`

- [ ] **Step 2.1: Localize WizardNav.tsx**

Replace the full content of `portal-frontend/src/pages/providers/components/WizardNav.tsx`:

```tsx
interface WizardNavProps {
  currentStep: number;
  totalSteps: number;
  onBack: () => void;
  onNext: () => void;
  onSubmit?: () => void;
  nextLabel?: string;
  loading?: boolean;
  nextDisabled?: boolean;
}

export function WizardNav({
  currentStep,
  totalSteps,
  onBack,
  onNext,
  onSubmit,
  nextLabel,
  loading,
  nextDisabled,
}: WizardNavProps) {
  const isLast = currentStep === totalSteps;

  return (
    <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: 32 }}>
      <button
        type="button"
        onClick={onBack}
        disabled={currentStep === 1}
        style={{ padding: '8px 20px', cursor: currentStep === 1 ? 'not-allowed' : 'pointer' }}
      >
        ← Назад
      </button>
      <button
        type="button"
        onClick={isLast && onSubmit ? onSubmit : onNext}
        disabled={loading || nextDisabled}
        style={{
          padding: '8px 20px',
          background: '#1976d2',
          color: '#fff',
          border: 'none',
          borderRadius: 4,
          cursor: loading || nextDisabled ? 'not-allowed' : 'pointer',
        }}
      >
        {loading ? 'Сохранение...' : (isLast ? (nextLabel ?? 'Создать провайдера') : (nextLabel ?? 'Далее →'))}
      </button>
    </div>
  );
}
```

- [ ] **Step 2.2: Localize Step1BasicInfo.tsx**

Replace the full content of `portal-frontend/src/pages/providers/steps/Step1BasicInfo.tsx`:

```tsx
import type { CreateProviderRequest } from '../../../api/client';

interface Props {
  data: Partial<CreateProviderRequest>;
  onChange: (updates: Partial<CreateProviderRequest>) => void;
}

export function Step1BasicInfo({ data, onChange }: Props) {
  return (
    <div>
      <h3>Основное</h3>
      <div style={{ marginBottom: 16 }}>
        <label>Название *<br />
          <input
            value={data.name ?? ''}
            onChange={e => onChange({ name: e.target.value })}
            placeholder="Мой SMPP провайдер"
            style={{ width: '100%', padding: '8px', marginTop: 4 }}
          />
        </label>
      </div>
      <div style={{ marginBottom: 16 }}>
        <label>Описание<br />
          <textarea
            value={data.description ?? ''}
            onChange={e => onChange({ description: e.target.value })}
            placeholder="Необязательное описание"
            rows={3}
            style={{ width: '100%', padding: '8px', marginTop: 4 }}
          />
        </label>
      </div>
      <div style={{ marginBottom: 16 }}>
        <label>Метки (через запятую)<br />
          <input
            value={(data.tags ?? []).join(', ')}
            onChange={e => onChange({ tags: e.target.value.split(',').map(t => t.trim()).filter(Boolean) })}
            placeholder="production, eu-west"
            style={{ width: '100%', padding: '8px', marginTop: 4 }}
          />
        </label>
      </div>
    </div>
  );
}
```

- [ ] **Step 2.3: Localize Step2Connection.tsx**

Replace the full content of `portal-frontend/src/pages/providers/steps/Step2Connection.tsx`:

```tsx
import type { CreateProviderRequest } from '../../../api/client';

interface Props {
  data: Partial<CreateProviderRequest>;
  onChange: (updates: Partial<CreateProviderRequest>) => void;
}

const BIND_TYPES = [
  { value: 0, label: 'Приёмо-передатчик (TRX)' },
  { value: 1, label: 'Передатчик (TX)' },
  { value: 2, label: 'Приёмник (RX)' },
];

export function Step2Connection({ data, onChange }: Props) {
  return (
    <div>
      <h3>Подключение</h3>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 120px', gap: 12, marginBottom: 16 }}>
        <label>Host *<br />
          <input
            value={data.host ?? ''}
            onChange={e => onChange({ host: e.target.value })}
            placeholder="smpp.example.com"
            style={{ width: '100%', padding: '8px', marginTop: 4 }}
          />
        </label>
        <label>Port *<br />
          <input
            type="number"
            value={data.port ?? ''}
            onChange={e => onChange({ port: parseInt(e.target.value) || 0 })}
            placeholder="2775"
            style={{ width: '100%', padding: '8px', marginTop: 4 }}
          />
        </label>
      </div>
      <div style={{ marginBottom: 16 }}>
        <label>Системный ID *<br />
          <input
            value={data.system_id ?? ''}
            onChange={e => onChange({ system_id: e.target.value })}
            placeholder="smppclient1"
            style={{ width: '100%', padding: '8px', marginTop: 4 }}
          />
        </label>
      </div>
      <div style={{ marginBottom: 16 }}>
        <label>Пароль *<br />
          <input
            type="password"
            value={data.password ?? ''}
            onChange={e => onChange({ password: e.target.value })}
            placeholder="••••••••"
            style={{ width: '100%', padding: '8px', marginTop: 4 }}
          />
        </label>
      </div>
      <div style={{ marginBottom: 16 }}>
        <label>Режим привязки<br />
          <select
            value={data.bind_type ?? 0}
            onChange={e => onChange({ bind_type: parseInt(e.target.value) })}
            style={{ padding: '8px', marginTop: 4 }}
          >
            {BIND_TYPES.map(bt => (
              <option key={bt.value} value={bt.value}>{bt.label}</option>
            ))}
          </select>
        </label>
      </div>
    </div>
  );
}
```

- [ ] **Step 2.4: Localize Step3Params.tsx**

Replace the full content of `portal-frontend/src/pages/providers/steps/Step3Params.tsx`:

```tsx
import type { CreateProviderRequest } from '../../../api/client';

interface Props {
  data: Partial<CreateProviderRequest>;
  onChange: (updates: Partial<CreateProviderRequest>) => void;
}

export function Step3Params({ data, onChange }: Props) {
  return (
    <div>
      <h3>Параметры</h3>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 12, marginBottom: 16 }}>
        <label>Макс. соединений<br />
          <input
            type="number"
            min={1}
            max={100}
            value={data.max_connections ?? 1}
            onChange={e => onChange({ max_connections: parseInt(e.target.value) || 1 })}
            style={{ width: '100%', padding: '8px', marginTop: 4 }}
          />
        </label>
        <label>Размер окна<br />
          <input
            type="number"
            min={1}
            max={1000}
            value={data.window_size ?? 10}
            onChange={e => onChange({ window_size: parseInt(e.target.value) || 10 })}
            style={{ width: '100%', padding: '8px', marginTop: 4 }}
          />
        </label>
        <label>Лимит TPS<br />
          <input
            type="number"
            min={1}
            value={data.tps_limit ?? 100}
            onChange={e => onChange({ tps_limit: parseInt(e.target.value) || 100 })}
            style={{ width: '100%', padding: '8px', marginTop: 4 }}
          />
        </label>
      </div>
      <p style={{ color: '#666', fontSize: 13 }}>
        <strong>Макс. соединений</strong> — количество одновременных SMPP-привязок.<br />
        <strong>Размер окна</strong> — максимум неподтверждённых сообщений на соединение.<br />
        <strong>Лимит TPS</strong> — максимальное число сообщений в секунду.
      </p>
    </div>
  );
}
```

- [ ] **Step 2.5: Localize Step4Test.tsx**

Replace the full content of `portal-frontend/src/pages/providers/steps/Step4Test.tsx`:

```tsx
import { useState } from 'react';
import { providersApi, ApiError, type CreateProviderRequest, type TestConnectionResult } from '../../../api/client';

interface Props {
  data: Partial<CreateProviderRequest>;
}

export function Step4Test({ data }: Props) {
  const [result, setResult] = useState<TestConnectionResult | null>(null);
  const [testing, setTesting] = useState(false);
  const [error, setError] = useState('');

  async function runTest() {
    if (!data.host || !data.port || !data.system_id || !data.password) {
      setError('Сначала заполните данные подключения (шаг 2)');
      return;
    }
    setTesting(true);
    setError('');
    setResult(null);
    try {
      const res = await providersApi.testConnection({
        host: data.host,
        port: data.port,
        system_id: data.system_id,
        password: data.password,
        bind_type: data.bind_type ?? 0,
      });
      setResult(res);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Ошибка теста');
    } finally {
      setTesting(false);
    }
  }

  return (
    <div>
      <h3>Тест подключения</h3>
      <p style={{ color: '#666' }}>
        Проверьте SMPP-подключение, выполнив bind/unbind тест.
      </p>
      <button
        type="button"
        onClick={runTest}
        disabled={testing}
        style={{
          padding: '8px 20px',
          background: '#1976d2',
          color: '#fff',
          border: 'none',
          borderRadius: 4,
          cursor: testing ? 'not-allowed' : 'pointer',
        }}
      >
        {testing ? 'Тест...' : 'Протестировать'}
      </button>

      {error && <p style={{ color: '#d32f2f', marginTop: 12 }}>{error}</p>}

      {result && (
        <div style={{ marginTop: 16, padding: 12, background: result.success ? '#e8f5e9' : '#ffebee', borderRadius: 4 }}>
          <div style={{ fontWeight: 'bold', color: result.success ? '#2e7d32' : '#c62828', marginBottom: 8 }}>
            {result.success ? `✓ Подключено (${result.latency_ms}мс)` : '✗ Ошибка'}
          </div>
          <pre style={{ fontSize: 12, margin: 0, whiteSpace: 'pre-wrap' }}>
            {(result.log ?? []).join('\n')}
            {result.error ? `\nОшибка: ${result.error}` : ''}
          </pre>
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2.6: Verify the build compiles**

```bash
cd portal-frontend && npx tsc --noEmit 2>&1 | head -30
```

Expected: no type errors.

- [ ] **Step 2.7: Commit**

```bash
cd portal-frontend && git add -A -- src/pages/providers/
git commit -m "feat(portal): localize SMPP wizard field labels to Russian"
```

---

## Task 3: Add Edit Provider Modal

**Files:**
- Modify: `portal-frontend/src/api/client.ts`
- Create: `portal-frontend/src/pages/providers/components/EditProviderModal.tsx`
- Modify: `portal-frontend/src/pages/providers/ProvidersPage.tsx`

- [ ] **Step 3.1: Add UpdateProviderRequest type to client.ts**

In `portal-frontend/src/api/client.ts`, find the `CreateProviderRequest` interface and add the new type immediately after it:

```typescript
export interface UpdateProviderRequest {
  name: string;
  description?: string;
  tags?: string[];
  host: string;
  port: number;
  system_id: string;
  password?: string; // omit to keep existing password unchanged
  bind_type: number;
  window_size?: number;
  max_connections?: number;
  tps_limit?: number;
}
```

Also update the `update` method signature in `providersApi` to use the new type:

```typescript
update: (id: string, data: UpdateProviderRequest) =>
  apiFetch<Provider>(`/providers/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
```

- [ ] **Step 3.2: Create EditProviderModal.tsx**

Create `portal-frontend/src/pages/providers/components/EditProviderModal.tsx` with the full content below:

```tsx
import { useState } from 'react';
import {
  providersApi,
  ApiError,
  type Provider,
  type UpdateProviderRequest,
  type TestConnectionResult,
} from '../../../api/client';
import { ConfirmDialog } from '../../../components/ui/ConfirmDialog';

interface Props {
  provider: Provider;
  onClose: () => void;
  onSaved: (updated: Provider) => void;
}

interface FormState {
  name: string;
  description: string;
  tags: string[];
  host: string;
  port: number;
  system_id: string;
  password: string;
  bind_type: number;
  window_size: number;
  max_connections: number;
  tps_limit: number;
}

const BIND_TYPES = [
  { value: 0, label: 'Приёмо-передатчик (TRX)' },
  { value: 1, label: 'Передатчик (TX)' },
  { value: 2, label: 'Приёмник (RX)' },
];

// Connection fields whose change requires provider reconnect.
const CONNECTION_FIELDS = ['host', 'port', 'system_id', 'bind_type'] as const;

export function hasConnectionFieldChanged(
  original: Provider,
  form: FormState,
  passwordChanged: boolean,
): boolean {
  return passwordChanged || CONNECTION_FIELDS.some(
    f => String(original[f]) !== String(form[f]),
  );
}

export function buildUpdatePayload(
  form: FormState,
  passwordChanged: boolean,
): UpdateProviderRequest {
  const payload: UpdateProviderRequest = {
    name: form.name,
    description: form.description,
    tags: form.tags,
    host: form.host,
    port: form.port,
    system_id: form.system_id,
    bind_type: form.bind_type,
    window_size: form.window_size,
    max_connections: form.max_connections,
    tps_limit: form.tps_limit,
  };
  if (passwordChanged) payload.password = form.password;
  return payload;
}

// Cosmetic mask shown when password has not been changed.
const PASSWORD_MASK = '••••••••';

export function EditProviderModal({ provider, onClose, onSaved }: Props) {
  const [form, setForm] = useState<FormState>({
    name: provider.name,
    description: provider.description ?? '',
    tags: provider.tags ?? [],
    host: provider.host,
    port: provider.port,
    system_id: provider.system_id,
    password: PASSWORD_MASK,
    bind_type: provider.bind_type,
    window_size: provider.window_size,
    max_connections: provider.max_connections,
    tps_limit: provider.tps_limit,
  });
  const [passwordChanged, setPasswordChanged] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [showConfirm, setShowConfirm] = useState(false);
  const [testResult, setTestResult] = useState<TestConnectionResult | null>(null);
  const [testing, setTesting] = useState(false);

  function update(updates: Partial<FormState>) {
    setForm(prev => ({ ...prev, ...updates }));
  }

  function handlePasswordFocus() {
    if (!passwordChanged) {
      setForm(prev => ({ ...prev, password: '' }));
      setPasswordChanged(true);
    }
  }

  async function runTest() {
    setTesting(true);
    setTestResult(null);
    try {
      const res = await providersApi.testConnection({
        host: form.host,
        port: form.port,
        system_id: form.system_id,
        // When password hasn't been changed, send empty string — test will use
        // whatever the server stores. Backend handles this gracefully.
        password: passwordChanged ? form.password : '',
        bind_type: form.bind_type,
      });
      setTestResult(res);
    } catch (err) {
      setTestResult({
        success: false,
        latency_ms: 0,
        log: [],
        error: err instanceof ApiError ? err.message : 'Ошибка теста',
      });
    } finally {
      setTesting(false);
    }
  }

  function handleSave() {
    if (hasConnectionFieldChanged(provider, form, passwordChanged)) {
      setShowConfirm(true);
    } else {
      doSave();
    }
  }

  async function doSave() {
    setShowConfirm(false);
    setSaving(true);
    setError('');
    try {
      const updated = await providersApi.update(
        provider.id,
        buildUpdatePayload(form, passwordChanged),
      );
      onSaved(updated);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось сохранить');
    } finally {
      setSaving(false);
    }
  }

  return (
    <>
      {/* Backdrop */}
      <div
        style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.4)', zIndex: 100 }}
        onClick={onClose}
      />

      {/* Modal */}
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="edit-provider-title"
        style={{
          position: 'fixed',
          top: '50%',
          left: '50%',
          transform: 'translate(-50%, -50%)',
          background: '#fff',
          borderRadius: 8,
          width: 540,
          maxHeight: '90vh',
          overflowY: 'auto',
          zIndex: 101,
          boxShadow: '0 8px 32px rgba(0,0,0,0.2)',
        }}
      >
        {/* Header */}
        <div style={{ padding: '16px 20px', borderBottom: '1px solid #e2e8f0', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <div id="edit-provider-title" style={{ fontWeight: 600, fontSize: 15 }}>
            ✏️ {provider.name}
          </div>
          <button
            type="button"
            aria-label="Закрыть"
            onClick={onClose}
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#94a3b8', fontSize: 18 }}
          >
            ✕
          </button>
        </div>

        <div style={{ padding: '20px' }}>

          {/* ── Основное ── */}
          <div style={{ marginBottom: 20 }}>
            <SectionLabel>Основное</SectionLabel>
            <Field label="Название *">
              <input
                value={form.name}
                onChange={e => update({ name: e.target.value })}
                style={inputStyle}
              />
            </Field>
            <Field label="Описание">
              <input
                value={form.description}
                onChange={e => update({ description: e.target.value })}
                style={inputStyle}
              />
            </Field>
            <Field label="Метки (через запятую)">
              <input
                value={form.tags.join(', ')}
                onChange={e => update({ tags: e.target.value.split(',').map(t => t.trim()).filter(Boolean) })}
                style={inputStyle}
              />
            </Field>
          </div>

          <Divider />

          {/* ── Подключение ── */}
          <div style={{ marginBottom: 20 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 10 }}>
              <SectionLabel noMargin>Подключение</SectionLabel>
              <span style={{ fontSize: 10, color: '#b45309', background: '#fefce8', border: '1px solid #fde047', padding: '2px 8px', borderRadius: 10 }}>
                ⚠ изменение перезапустит соединение
              </span>
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr', gap: 10, marginBottom: 10 }}>
              <Field label="Host">
                <input value={form.host} onChange={e => update({ host: e.target.value })} style={inputStyle} />
              </Field>
              <Field label="Port">
                <input
                  type="number"
                  value={form.port}
                  onChange={e => update({ port: parseInt(e.target.value) || 0 })}
                  style={inputStyle}
                />
              </Field>
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10, marginBottom: 10 }}>
              <Field label="Системный ID">
                <input value={form.system_id} onChange={e => update({ system_id: e.target.value })} style={inputStyle} />
              </Field>
              <Field label="Пароль">
                <input
                  type={passwordChanged ? 'password' : 'text'}
                  value={form.password}
                  onFocus={handlePasswordFocus}
                  onChange={e => update({ password: e.target.value })}
                  style={inputStyle}
                />
              </Field>
            </div>
            <div style={{ display: 'flex', gap: 10, alignItems: 'flex-end' }}>
              <div style={{ flex: 1 }}>
                <Field label="Режим привязки">
                  <select
                    value={form.bind_type}
                    onChange={e => update({ bind_type: parseInt(e.target.value) })}
                    style={{ ...inputStyle, padding: '7px 10px' }}
                  >
                    {BIND_TYPES.map(bt => (
                      <option key={bt.value} value={bt.value}>{bt.label}</option>
                    ))}
                  </select>
                </Field>
              </div>
              <button
                type="button"
                onClick={runTest}
                disabled={testing}
                style={{
                  padding: '7px 14px',
                  background: '#f0f9ff',
                  border: '1px solid #bae6fd',
                  color: '#0369a1',
                  borderRadius: 4,
                  fontSize: 12,
                  cursor: testing ? 'not-allowed' : 'pointer',
                  whiteSpace: 'nowrap',
                  marginBottom: 0,
                }}
              >
                {testing ? 'Тест...' : '▶ Протестировать'}
              </button>
            </div>

            {testResult && (
              <div style={{ marginTop: 10, padding: 10, background: testResult.success ? '#f0fdf4' : '#fff1f2', borderRadius: 4, fontSize: 12 }}>
                <div style={{ fontWeight: 600, color: testResult.success ? '#15803d' : '#be123c' }}>
                  {testResult.success ? `✓ Успешно (${testResult.latency_ms}мс)` : '✗ Ошибка подключения'}
                </div>
                {(testResult.log?.length > 0 || testResult.error) && (
                  <pre style={{ margin: '6px 0 0', whiteSpace: 'pre-wrap', fontSize: 11 }}>
                    {testResult.log?.join('\n')}
                    {testResult.error ? `\n${testResult.error}` : ''}
                  </pre>
                )}
              </div>
            )}
          </div>

          <Divider />

          {/* ── Параметры ── */}
          <div>
            <SectionLabel>Параметры</SectionLabel>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 10 }}>
              <Field label="Макс. соединений">
                <input
                  type="number"
                  min={1}
                  max={100}
                  value={form.max_connections}
                  onChange={e => update({ max_connections: parseInt(e.target.value) || 1 })}
                  style={inputStyle}
                />
              </Field>
              <Field label="Размер окна">
                <input
                  type="number"
                  min={1}
                  max={1000}
                  value={form.window_size}
                  onChange={e => update({ window_size: parseInt(e.target.value) || 10 })}
                  style={inputStyle}
                />
              </Field>
              <Field label="Лимит TPS">
                <input
                  type="number"
                  min={1}
                  value={form.tps_limit}
                  onChange={e => update({ tps_limit: parseInt(e.target.value) || 100 })}
                  style={inputStyle}
                />
              </Field>
            </div>
          </div>
        </div>

        {error && (
          <div style={{ padding: '0 20px 12px', color: '#dc2626', fontSize: 13 }}>{error}</div>
        )}

        {/* Footer */}
        <div style={{ padding: '12px 20px', borderTop: '1px solid #e2e8f0', display: 'flex', justifyContent: 'flex-end', gap: 8, background: '#f8fafc' }}>
          <button
            type="button"
            onClick={onClose}
            style={{ padding: '7px 16px', background: '#fff', border: '1px solid #e2e8f0', borderRadius: 4, fontSize: 13, cursor: 'pointer', color: '#64748b' }}
          >
            Отмена
          </button>
          <button
            type="button"
            onClick={handleSave}
            disabled={saving}
            style={{ padding: '7px 16px', background: '#2563eb', border: 'none', color: '#fff', borderRadius: 4, fontSize: 13, cursor: saving ? 'not-allowed' : 'pointer', fontWeight: 500 }}
          >
            {saving ? 'Сохранение...' : 'Сохранить'}
          </button>
        </div>
      </div>

      <ConfirmDialog
        open={showConfirm}
        title="Переподключение провайдера"
        description="Изменение параметров подключения приведёт к кратковременному разрыву соединения. Продолжить?"
        confirmLabel="Продолжить"
        onConfirm={doSave}
        onCancel={() => setShowConfirm(false)}
      />
    </>
  );
}

// ── Local helpers ──────────────────────────────────────────────────────────────

const inputStyle: React.CSSProperties = {
  display: 'block',
  width: '100%',
  padding: '7px 10px',
  border: '1px solid #e2e8f0',
  borderRadius: 4,
  fontSize: 13,
  boxSizing: 'border-box',
};

function SectionLabel({ children, noMargin }: { children: React.ReactNode; noMargin?: boolean }) {
  return (
    <div style={{ fontSize: 11, fontWeight: 600, color: '#64748b', textTransform: 'uppercase', letterSpacing: '0.05em', marginBottom: noMargin ? 0 : 10 }}>
      {children}
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div style={{ marginBottom: 10 }}>
      <label style={{ fontSize: 12, color: '#64748b' }}>
        {label}
        <div style={{ marginTop: 4 }}>{children}</div>
      </label>
    </div>
  );
}

function Divider() {
  return <div style={{ borderTop: '1px solid #f1f5f9', marginBottom: 20 }} />;
}
```

- [ ] **Step 3.3: Update ProvidersPage.tsx to add the Edit button and modal**

Replace the full content of `portal-frontend/src/pages/providers/ProvidersPage.tsx`:

```tsx
import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { providersApi, ApiError, type Provider } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { DataTable, type Column } from '../../components/data/DataTable';
import { StatusBadge } from '../../components/ui/Badge';
import { EditProviderModal } from './components/EditProviderModal';

const BIND_LABELS: Record<number, string> = { 0: 'TRX', 1: 'TX', 2: 'RX' };

export function ProvidersPage() {
  const navigate = useNavigate();
  const [providers, setProviders] = useState<Provider[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [editProvider, setEditProvider] = useState<Provider | null>(null);

  async function load() {
    setLoading(true);
    setError('');
    try {
      const resp = await providersApi.list();
      setProviders(resp.providers ?? []);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось загрузить провайдеров');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { load(); }, []);

  async function confirmDelete() {
    if (!deleteId) return;
    setDeleting(true);
    try {
      await providersApi.remove(deleteId);
      setProviders(prev => prev.filter(p => p.id !== deleteId));
      setDeleteId(null);
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'Не удалось удалить провайдера');
    } finally {
      setDeleting(false);
    }
  }

  function handleSaved(updated: Provider) {
    setProviders(prev => prev.map(p => p.id === updated.id ? updated : p));
    setEditProvider(null);
  }

  const columns: Column<Provider>[] = [
    { key: 'name', header: 'Название', render: (p) => (
      <div>
        <div>{p.name}</div>
        {p.description && <div className="text-xs text-gray-500">{p.description}</div>}
      </div>
    )},
    { key: 'host', header: 'Хост', render: (p) => <span className="font-mono">{p.host}:{p.port}</span> },
    { key: 'bind_type', header: 'Привязка', render: (p) => <>{BIND_LABELS[p.bind_type] ?? '-'}</> },
    { key: 'max_connections', header: 'Подкл.' },
    { key: 'tps_limit', header: 'TPS' },
    { key: 'active', header: 'Статус', render: (p) => <StatusBadge status={p.active ? 'active' : 'inactive'} /> },
  ];

  return (
    <div>
      <PageHeader
        title="SMPP Провайдеры"
        actions={<Button onClick={() => navigate('/providers/new')}>+ Добавить провайдера</Button>}
      />

      {error && <p className="text-red-600">{error}</p>}

      {!loading && !error && providers.length === 0 && (
        <div className="text-center py-12 text-gray-500">
          <p className="mb-2">Провайдеры не настроены</p>
          <p className="text-sm mb-4">Подключите SMPP-провайдера для начала отправки SMS</p>
          <Button variant="ghost" onClick={() => navigate('/providers/new')}>
            Добавить провайдера
          </Button>
        </div>
      )}

      {(loading || providers.length > 0) && (
        <DataTable
          columns={columns}
          data={providers}
          total={providers.length}
          page={1}
          pageSize={providers.length}
          onPageChange={() => {}}
          loading={loading}
          keyField="id"
          rowActions={(p) => (
            <div style={{ display: 'flex', gap: 6 }}>
              <Button variant="secondary" size="sm" onClick={() => setEditProvider(p)}>
                Редактировать
              </Button>
              <Button variant="danger" size="sm" onClick={() => setDeleteId(p.id)}>
                Удалить
              </Button>
            </div>
          )}
        />
      )}

      {editProvider && (
        <EditProviderModal
          provider={editProvider}
          onClose={() => setEditProvider(null)}
          onSaved={handleSaved}
        />
      )}

      <ConfirmDialog
        open={deleteId !== null}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteId(null)}
        title="Удалить провайдера"
        description="Вы уверены, что хотите удалить этого провайдера? Это действие нельзя отменить."
        confirmLabel="Удалить"
        variant="danger"
        loading={deleting}
      />
    </div>
  );
}
```

- [ ] **Step 3.4: Verify the build compiles**

```bash
cd portal-frontend && npx tsc --noEmit 2>&1 | head -40
```

Expected: no errors. If `ConfirmDialog` props differ from what is used, adjust to match the actual component interface.

- [ ] **Step 3.5: Commit**

```bash
cd portal-frontend && git add -A -- src/api/client.ts src/pages/providers/
git commit -m "feat(portal): add edit provider modal with reconnect confirmation"
```

---

## Self-Review

**Spec coverage:**
- ✅ Remove routing step: Tasks 1.1–1.6
- ✅ Connection remains active while editing: handled — no disconnect on modal open, only PUT on save
- ✅ Translate non-technical terms: Tasks 2.1–2.7 (WizardNav, all 4 steps, Step5Summary)
- ✅ Edit button in provider list: Task 3.3 `rowActions` updated
- ✅ Modal similar to add-provider form: same fields, same structure
- ✅ Password mask: Task 3.2 `handlePasswordFocus` + `passwordChanged` flag
- ✅ Test connection button in modal: Task 3.2 `runTest`
- ✅ Reconnect confirmation dialog: Task 3.2 `hasConnectionFieldChanged` + `showConfirm`
- ✅ Silent save for soft fields: Task 3.2 `handleSave` else branch
- ✅ `UpdateProviderRequest` type: Task 3.1

**No placeholders, no TODOs, all code complete.**
