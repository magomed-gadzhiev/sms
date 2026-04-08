import { useState, useEffect, useCallback } from 'react';
import { Modal } from '../../components/ui/Modal';
import { Button } from '../../components/ui/Button';
import { Select } from '../../components/ui/Select';
import {
  routesApi,
  providersApi,
  routingApi,
  ApiError,
  type Provider,
  type OperatorInfo,
} from '../../api/client';
import type { RouteFormData, ScheduleJSON } from './types';
import { ConditionEditor } from './components/ConditionEditor';
import { ScheduleEditor } from './components/ScheduleEditor';
import { RouteSummary } from './components/RouteSummary';

interface RouteModalProps {
  open: boolean;
  onClose: () => void;
  onSaved: () => void;
  routeId?: string | null;
}

const EMPTY_FORM: RouteFormData = {
  name: '',
  comment: '',
  status: 'draft',
  route_type: 'sms',
  provider_id: '',
  priority: 100,
  share: 100,
  condition_groups: [{ logic_op: 'IF', conditions: [{ type: 'operator', value: '' }] }],
  schedules: [],
};

const ROUTE_TYPES = [
  { value: 'sms', label: 'SMS' },
  { value: 'hlr', label: 'HLR' },
  { value: 'max', label: 'MAX' },
] as const;

export function RouteModal({ open, onClose, onSaved, routeId }: RouteModalProps) {
  const [form, setForm] = useState<RouteFormData>({ ...EMPTY_FORM });
  const [providers, setProviders] = useState<Provider[]>([]);
  const [, setOperators] = useState<OperatorInfo[]>([]);
  const [saving, setSaving] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const isEdit = !!routeId;

  const loadData = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const [provsResp, opsResp] = await Promise.all([
        providersApi.list(),
        routingApi.listOperators(),
      ]);
      setProviders(provsResp.providers ?? []);
      setOperators(opsResp.operators ?? []);

      if (routeId) {
        const route = await routesApi.get(routeId);
        setForm({
          client_id: route.client_id,
          name: route.name,
          comment: route.comment,
          status: route.status as 'active' | 'draft',
          route_type: route.route_type as 'sms' | 'hlr' | 'max',
          operator_id: route.operator_id,
          provider_id: route.provider_id,
          priority: route.priority,
          share: route.share,
          condition_groups: route.condition_groups?.length
            ? route.condition_groups
            : [{ logic_op: 'IF', conditions: [{ type: 'operator', value: '' }] }],
          schedules: route.schedules ?? [],
        });
      } else {
        setForm({ ...EMPTY_FORM });
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось загрузить данные');
    } finally {
      setLoading(false);
    }
  }, [routeId]);

  useEffect(() => {
    if (open) {
      loadData();
    }
  }, [open, loadData]);

  function updateForm(partial: Partial<RouteFormData>) {
    setForm((prev) => ({ ...prev, ...partial }));
  }

  async function handleSave(status: 'active' | 'draft') {
    setError('');
    if (!form.name.trim()) {
      setError('Введите название маршрута');
      return;
    }
    if (!form.provider_id) {
      setError('Выберите провайдера');
      return;
    }

    setSaving(true);
    const data: RouteFormData = { ...form, status };
    try {
      if (isEdit && routeId) {
        await routesApi.update(routeId, data);
      } else {
        await routesApi.create(data);
      }
      onSaved();
      onClose();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось сохранить маршрут');
    } finally {
      setSaving(false);
    }
  }

  const activeProviders = providers.filter((p) => p.active);
  const providerName = providers.find((p) => p.id === form.provider_id)?.name || '';

  const scheduleValue: ScheduleJSON | null = form.schedules.length > 0 ? form.schedules[0] : null;

  function handleScheduleChange(sch: ScheduleJSON | null) {
    updateForm({ schedules: sch ? [sch] : [] });
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={isEdit ? 'Редактировать маршрут' : 'Новый маршрут'}
      description={isEdit ? 'Измените параметры маршрута' : 'Настройте новый маршрут доставки'}
      wide
    >
      {loading ? (
        <div className="py-8 text-center text-gray-500">Загрузка...</div>
      ) : (
        <div className="flex gap-6">
          {/* Left: Form */}
          <div className="flex-1 flex flex-col gap-6 min-w-0">
            {/* Section: Основное */}
            <section>
              <h3 className="text-sm font-semibold text-gray-700 mb-3">Основное</h3>
              <div className="flex flex-col gap-3">
                <div className="flex flex-col gap-1">
                  <label className="text-sm font-medium text-gray-700">Название</label>
                  <input
                    type="text"
                    value={form.name}
                    onChange={(e) => updateForm({ name: e.target.value })}
                    placeholder="Например: МТС через iDigital"
                    className="rounded border border-gray-300 px-3 py-2 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
                  />
                </div>

                <div className="flex flex-col gap-1">
                  <label className="text-sm font-medium text-gray-700">Тип маршрута</label>
                  <div className="flex gap-1">
                    {ROUTE_TYPES.map((rt) => (
                      <button
                        key={rt.value}
                        type="button"
                        onClick={() => updateForm({ route_type: rt.value })}
                        className={`px-4 py-2 text-sm font-medium rounded transition-colors ${
                          form.route_type === rt.value
                            ? 'bg-primary text-white'
                            : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
                        }`}
                      >
                        {rt.label}
                      </button>
                    ))}
                  </div>
                </div>

                <div className="grid grid-cols-2 gap-3">
                  <div className="flex flex-col gap-1">
                    <label className="text-sm font-medium text-gray-700">Приоритет</label>
                    <input
                      type="number"
                      value={form.priority}
                      onChange={(e) => updateForm({ priority: parseInt(e.target.value) || 0 })}
                      min="0"
                      className="rounded border border-gray-300 px-3 py-2 text-sm"
                    />
                  </div>
                  <div className="flex flex-col gap-1">
                    <label className="text-sm font-medium text-gray-700">Доля (%)</label>
                    <input
                      type="number"
                      value={form.share}
                      onChange={(e) => updateForm({ share: parseInt(e.target.value) || 0 })}
                      min="0"
                      max="100"
                      className="rounded border border-gray-300 px-3 py-2 text-sm"
                    />
                  </div>
                </div>

                <div className="flex flex-col gap-1">
                  <label className="text-sm font-medium text-gray-700">Комментарий</label>
                  <textarea
                    value={form.comment}
                    onChange={(e) => updateForm({ comment: e.target.value })}
                    rows={2}
                    className="rounded border border-gray-300 px-3 py-2 text-sm resize-none focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
                  />
                </div>
              </div>
            </section>

            {/* Section: Условия */}
            <section>
              <h3 className="text-sm font-semibold text-gray-700 mb-3">Условия срабатывания</h3>
              <ConditionEditor
                groups={form.condition_groups}
                onChange={(groups) => updateForm({ condition_groups: groups })}
              />
            </section>

            {/* Section: Канал доставки */}
            <section>
              <h3 className="text-sm font-semibold text-gray-700 mb-3">Канал доставки</h3>
              <Select
                label="Провайдер"
                value={form.provider_id}
                onChange={(val) => updateForm({ provider_id: val })}
                placeholder="Выберите провайдера"
                options={activeProviders.map((p) => ({ value: p.id, label: p.name }))}
              />
            </section>

            {/* Section: Расписание */}
            <section>
              <h3 className="text-sm font-semibold text-gray-700 mb-3">Расписание</h3>
              <ScheduleEditor schedule={scheduleValue} onChange={handleScheduleChange} />
            </section>

            {error && <p className="text-red-600 text-sm">{error}</p>}

            {/* Footer */}
            <div className="flex justify-end gap-2 pt-2 border-t">
              <Button variant="ghost" onClick={onClose} disabled={saving}>
                Отмена
              </Button>
              <Button variant="secondary" onClick={() => handleSave('draft')} disabled={saving}>
                {saving ? 'Сохранение...' : 'Сохранить как черновик'}
              </Button>
              <Button onClick={() => handleSave('active')} disabled={saving}>
                {saving ? 'Сохранение...' : 'Сохранить маршрут'}
              </Button>
            </div>
          </div>

          {/* Right: Sidebar preview */}
          <div className="w-56 shrink-0 hidden md:block">
            <div className="sticky top-0 bg-gray-50 rounded-lg p-4 border">
              <RouteSummary data={form} providerName={providerName} />
            </div>
          </div>
        </div>
      )}
    </Modal>
  );
}
