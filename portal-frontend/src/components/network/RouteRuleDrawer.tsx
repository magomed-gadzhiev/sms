import { useState, useEffect } from 'react';
import { Drawer } from '../ui/Drawer';
import { Button } from '../ui/Button';
import { Input } from '../ui/Input';
import { useToast } from '../ui/Toast';
import { ApiError, type NetworkRouteSetItem, type NetworkProvider } from '../../api/client';
import { RouteConditionsEditor } from './RouteConditionsEditor';
import { RouteSchedulesEditor } from './RouteSchedulesEditor';

type RuleData = Omit<NetworkRouteSetItem, 'id' | 'provider_name'>;

const EMPTY: RuleData = {
  name: '',
  comment: '',
  provider_id: '',
  priority: 0,
  share: 100,
  route_type: 'sms',
  status: 'active',
  condition_groups: [],
  schedules: [],
};

interface Props {
  open: boolean;
  onClose: () => void;
  initial?: NetworkRouteSetItem | null;
  providers: NetworkProvider[];
  onSubmit: (data: RuleData) => Promise<void>;
  title: string;
}

export function RouteRuleDrawer({ open, onClose, initial, providers, onSubmit, title }: Props) {
  const toast = useToast();
  const [form, setForm] = useState<RuleData>(EMPTY);
  const [submitting, setSubmitting] = useState<boolean>(false);

  useEffect(() => {
    if (open) {
      if (initial) {
        setForm({
          name: initial.name,
          comment: initial.comment,
          provider_id: initial.provider_id,
          priority: initial.priority,
          share: initial.share,
          route_type: initial.route_type,
          status: initial.status,
          condition_groups: initial.condition_groups,
          schedules: initial.schedules,
        });
      } else {
        setForm(EMPTY);
      }
    }
  }, [open, initial]);

  const submit = async (): Promise<void> => {
    if (!form.provider_id) {
      toast.error('Выберите провайдера');
      return;
    }
    setSubmitting(true);
    try {
      await onSubmit(form);
      onClose();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка сохранения');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Drawer open={open} onClose={onClose} title={title} width="lg">
      <div className="space-y-4">
        <Input
          label="Имя"
          value={form.name}
          onChange={(e) => setForm({ ...form, name: e.target.value })}
        />
        <Input
          label="Комментарий"
          value={form.comment}
          onChange={(e) => setForm({ ...form, comment: e.target.value })}
        />

        <div>
          <label className="block text-sm text-gray-600 mb-1">Провайдер</label>
          <select
            value={form.provider_id}
            onChange={(e) => setForm({ ...form, provider_id: e.target.value })}
            className="w-full border border-gray-300 rounded px-3 py-2 text-sm"
          >
            <option value="">— выбрать —</option>
            {providers.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name} ({p.ownership === 'private' ? 'свой' : 'платформа'})
              </option>
            ))}
          </select>
        </div>

        <div className="grid grid-cols-3 gap-3">
          <Input
            label="Приоритет"
            type="number"
            value={String(form.priority)}
            onChange={(e) => setForm({ ...form, priority: parseInt(e.target.value, 10) || 0 })}
          />
          <Input
            label="Доля %"
            type="number"
            value={String(form.share)}
            onChange={(e) => setForm({ ...form, share: parseInt(e.target.value, 10) || 100 })}
          />
          <div>
            <label className="block text-sm text-gray-600 mb-1">Статус</label>
            <select
              value={form.status}
              onChange={(e) => setForm({ ...form, status: e.target.value as 'active' | 'inactive' })}
              className="w-full border rounded px-3 py-2 text-sm"
            >
              <option value="active">Активен</option>
              <option value="inactive">Выключен</option>
            </select>
          </div>
        </div>

        <RouteConditionsEditor
          groups={form.condition_groups}
          onChange={(groups) => setForm({ ...form, condition_groups: groups })}
        />

        <RouteSchedulesEditor
          schedules={form.schedules}
          onChange={(schedules) => setForm({ ...form, schedules })}
        />

        <div className="flex gap-2 pt-2">
          <Button onClick={submit} disabled={submitting}>
            {submitting ? 'Сохранение...' : 'Сохранить'}
          </Button>
          <Button variant="secondary" onClick={onClose}>Отмена</Button>
        </div>
      </div>
    </Drawer>
  );
}
