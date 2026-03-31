import { useState, useEffect, type FormEvent } from 'react';
import { cascadeStrategiesApi, cascadeChannelsApi, type DeliveryStrategy, type DeliveryChannel } from '../../api/cascade';
import { Modal } from '../../components/ui/Modal';
import { Input } from '../../components/ui/Input';
import { Button } from '../../components/ui/Button';

interface StepInput {
  channel_id: string;
  step_order: number;
  timeout_s: number;
  billable: boolean;
}

interface Props {
  strategy: DeliveryStrategy | null;
  onClose: () => void;
  onSaved: () => void;
}

export function StrategyFormModal({ strategy, onClose, onSaved }: Props) {
  const isEdit = strategy !== null;

  const [name, setName] = useState(strategy?.name ?? '');
  const [description, setDescription] = useState(strategy?.description ?? '');
  const [mode, setMode] = useState(strategy?.mode ?? 'sequential');
  const [steps, setSteps] = useState<StepInput[]>(
    strategy?.steps?.map((s) => ({
      channel_id: s.channel_id,
      step_order: s.step_order,
      timeout_s: s.timeout_s,
      billable: s.billable,
    })) ?? []
  );
  const [channels, setChannels] = useState<DeliveryChannel[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    cascadeChannelsApi.list().then((res) => setChannels(res.channels ?? [])).catch(() => {});
  }, []);

  const addStep = () => {
    setSteps((prev) => [
      ...prev,
      { channel_id: channels[0]?.channel_id ?? '', step_order: prev.length + 1, timeout_s: 30, billable: true },
    ]);
  };

  const removeStep = (idx: number) => {
    setSteps((prev) => prev.filter((_, i) => i !== idx).map((s, i) => ({ ...s, step_order: i + 1 })));
  };

  const updateStep = (idx: number, patch: Partial<StepInput>) => {
    setSteps((prev) => prev.map((s, i) => (i === idx ? { ...s, ...patch } : s)));
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError('');
    if (steps.length === 0) {
      setError('Добавьте хотя бы один шаг');
      return;
    }
    setSubmitting(true);
    try {
      const payload = { name, description, mode, steps };
      if (isEdit) {
        await cascadeStrategiesApi.update(strategy.strategy_id, payload);
      } else {
        await cascadeStrategiesApi.create(payload);
      }
      onSaved();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Ошибка сохранения');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal title={isEdit ? 'Редактировать стратегию' : 'Добавить стратегию'} onClose={onClose}>
      <form onSubmit={handleSubmit} className="space-y-4 max-h-[70vh] overflow-y-auto pr-1">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Название</label>
          <Input value={name} onChange={(e) => setName(e.target.value)} required placeholder="flash-call-then-sms" />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Описание</label>
          <Input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Описание (необязательно)" />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-2">Режим</label>
          <div className="flex gap-4">
            {(['sequential', 'parallel'] as const).map((m) => (
              <label key={m} className="flex items-center gap-2 cursor-pointer">
                <input
                  type="radio"
                  name="mode"
                  value={m}
                  checked={mode === m}
                  onChange={() => setMode(m)}
                  className="text-primary"
                />
                <span className="text-sm">{m === 'sequential' ? 'Последовательный' : 'Параллельный'}</span>
              </label>
            ))}
          </div>
        </div>

        <div>
          <div className="flex items-center justify-between mb-2">
            <label className="text-sm font-medium text-gray-700">Шаги</label>
            <Button type="button" variant="outline" size="sm" onClick={addStep}>
              + Шаг
            </Button>
          </div>

          <div className="space-y-3">
            {steps.map((step, idx) => (
              <div key={idx} className="border border-gray-200 rounded-md p-3 space-y-2 bg-gray-50">
                <div className="flex items-center justify-between">
                  <span className="text-sm font-medium text-gray-600">Шаг {step.step_order}</span>
                  <button
                    type="button"
                    onClick={() => removeStep(idx)}
                    className="text-xs text-red-500 hover:text-red-700"
                  >
                    Удалить
                  </button>
                </div>

                <div>
                  <label className="text-xs text-gray-500 mb-1 block">Канал</label>
                  <select
                    value={step.channel_id}
                    onChange={(e) => updateStep(idx, { channel_id: e.target.value })}
                    className="w-full border border-gray-300 rounded px-2 py-1.5 text-sm"
                    required
                  >
                    {channels.map((ch) => (
                      <option key={ch.channel_id} value={ch.channel_id}>
                        {ch.name} ({ch.channel_type})
                      </option>
                    ))}
                  </select>
                </div>

                <div className="grid grid-cols-2 gap-2">
                  <div>
                    <label className="text-xs text-gray-500 mb-1 block">Таймаут (сек)</label>
                    <input
                      type="number"
                      min={1}
                      value={step.timeout_s}
                      onChange={(e) => updateStep(idx, { timeout_s: Number(e.target.value) })}
                      className="w-full border border-gray-300 rounded px-2 py-1.5 text-sm"
                    />
                  </div>
                  <div className="flex items-end pb-1.5">
                    <label className="flex items-center gap-2 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={step.billable}
                        onChange={(e) => updateStep(idx, { billable: e.target.checked })}
                        className="text-primary"
                      />
                      <span className="text-sm text-gray-700">Тарифицировать</span>
                    </label>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>

        {error && <p className="text-sm text-red-600">{error}</p>}

        <div className="flex justify-end gap-3 pt-2">
          <Button type="button" variant="outline" onClick={onClose} disabled={submitting}>
            Отмена
          </Button>
          <Button type="submit" loading={submitting}>
            {isEdit ? 'Сохранить' : 'Создать'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
