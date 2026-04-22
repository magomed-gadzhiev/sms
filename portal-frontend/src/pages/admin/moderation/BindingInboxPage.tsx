import { useState, useEffect, useCallback, useMemo, type FormEvent } from 'react';
import { moderationApi, type BindingInboxItem, AdminApiError } from '../../../api/admin';
import { PageHeader } from '../../../components/layout/PageHeader';
import { Button } from '../../../components/ui/Button';
import { Badge } from '../../../components/ui/Badge';
import { Modal } from '../../../components/ui/Modal';
import { Input } from '../../../components/ui/Input';

const CHANNEL_CHIP: Record<string, string> = { sms: 'SMS', voice: 'Voice', viber: 'Viber' };

function formatDate(dt: string) {
  return new Date(dt).toLocaleString('ru-RU', { dateStyle: 'short', timeStyle: 'short' });
}

function truncate(s: string, n: number) {
  return s.length > n ? s.slice(0, n) + '...' : s;
}

export function BindingInboxPage() {
  const [items, setItems] = useState<BindingInboxItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [operatorFilter, setOperatorFilter] = useState('');
  const [rejectTarget, setRejectTarget] = useState<BindingInboxItem | null>(null);
  const [rejectReason, setRejectReason] = useState('');
  const [rejectError, setRejectError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const res = await moderationApi.listBindings({
        operator_id: operatorFilter || undefined,
        limit: 200,
      });
      setItems(res.bindings ?? []);
    } catch (e) {
      setError(e instanceof AdminApiError ? e.message : 'Ошибка');
    } finally {
      setLoading(false);
    }
  }, [operatorFilter]);

  useEffect(() => { load(); }, [load]);

  const emitChanged = () => window.dispatchEvent(new CustomEvent('moderation:changed'));

  const operators = useMemo(() => {
    const map = new Map<string, string>();
    items.forEach((b) => map.set(b.operator_id, b.operator_name));
    return Array.from(map.entries()).map(([id, name]) => ({ id, name }));
  }, [items]);

  const grouped = useMemo(() => {
    const groups = new Map<string, { name: string; bindings: BindingInboxItem[] }>();
    items.forEach((b) => {
      const g = groups.get(b.operator_id) ?? { name: b.operator_name, bindings: [] };
      g.bindings.push(b);
      groups.set(b.operator_id, g);
    });
    return Array.from(groups.entries()).map(([id, g]) => ({ id, ...g }));
  }, [items]);

  const handleApprove = async (b: BindingInboxItem) => {
    try {
      await moderationApi.approveBinding(b.id);
      emitChanged();
      load();
    } catch (e) {
      setError(e instanceof AdminApiError ? e.message : 'Ошибка одобрения');
    }
  };

  const handleReject = async (e: FormEvent) => {
    e.preventDefault();
    if (!rejectTarget) return;
    if (!rejectReason.trim()) { setRejectError('Укажите причину'); return; }
    setSubmitting(true);
    try {
      await moderationApi.rejectBinding(rejectTarget.id, rejectReason.trim());
      setRejectTarget(null);
      setRejectReason('');
      emitChanged();
      load();
    } catch (e) {
      setRejectError(e instanceof AdminApiError ? e.message : 'Ошибка');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div>
      <PageHeader
        title="Модерация биндингов"
        subtitle={`${items.length} ожидают модерации`}
        breadcrumbs={[
          { label: 'Админ', href: '/admin/dashboard' },
          { label: 'Модерация биндингов' },
        ]}
      />

      {error && <div className="mb-4 p-3 bg-red-50 text-red-700 rounded-md text-sm">{error}</div>}

      <div className="mb-4 flex gap-2 items-center">
        <label htmlFor="operator-filter" className="text-sm text-gray-700">Оператор:</label>
        <select
          id="operator-filter"
          className="border border-gray-300 rounded-md px-3 py-1.5 text-sm"
          value={operatorFilter}
          onChange={(e) => setOperatorFilter(e.target.value)}
        >
          <option value="">Все операторы</option>
          {operators.map((op) => (
            <option key={op.id} value={op.id}>{op.name}</option>
          ))}
        </select>
      </div>

      {loading && items.length === 0 && <div role="status">Загрузка...</div>}

      {!loading && !error && items.length === 0 && (
        <div className="text-center py-16 text-gray-500">
          <p className="mb-1">Нет биндингов на модерации</p>
          <p className="text-sm text-gray-400">Новые будут появляться здесь автоматически</p>
        </div>
      )}

      {grouped.map((group) => (
        <div key={group.id} className="mb-6 bg-white border border-gray-200 rounded-lg overflow-hidden">
          <div className="px-4 py-3 bg-gray-50 border-b border-gray-200 flex items-center justify-between">
            <h3 className="text-sm font-semibold text-gray-900">{group.name}</h3>
            <span className="text-xs text-gray-500">{group.bindings.length} ожидает</span>
          </div>
          <table className="w-full text-sm">
            <thead className="text-xs uppercase text-gray-500 bg-gray-50">
              <tr>
                <th className="px-4 py-2 text-left">Шаблон</th>
                <th className="px-4 py-2 text-left">Имя отправителя</th>
                <th className="px-4 py-2 text-left">Канал</th>
                <th className="px-4 py-2 text-left">Клиент</th>
                <th className="px-4 py-2 text-left">Подано</th>
                <th className="px-4 py-2 text-right">Действия</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {group.bindings.map((b) => (
                <tr key={b.id}>
                  <td className="px-4 py-3">
                    <div className="font-medium text-gray-900">{b.template_name}</div>
                    <div className="text-xs text-gray-500 truncate max-w-[300px]" title={b.template_body}>
                      {truncate(b.template_body, 80)}
                    </div>
                  </td>
                  <td className="px-4 py-3 font-mono">{b.sender_name}</td>
                  <td className="px-4 py-3">
                    <Badge variant="default">{CHANNEL_CHIP[b.channel] ?? b.channel}</Badge>
                  </td>
                  <td className="px-4 py-3">{b.client_email}</td>
                  <td className="px-4 py-3 text-sm text-gray-600">{formatDate(b.created_at)}</td>
                  <td className="px-4 py-3 text-right">
                    <Button size="sm" onClick={() => handleApprove(b)}>Одобрить</Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      className="ml-1"
                      onClick={() => { setRejectTarget(b); setRejectReason(''); setRejectError(''); }}
                    >
                      Отклонить
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ))}

      <Modal open={!!rejectTarget} onClose={() => setRejectTarget(null)} title="Отклонить биндинг">
        <form onSubmit={handleReject} className="space-y-4">
          <p className="text-sm text-gray-600">
            Шаблон: <strong>{rejectTarget?.template_name}</strong><br />
            Оператор: {rejectTarget?.operator_name}
          </p>
          <Input
            label="Причина отклонения"
            value={rejectReason}
            onChange={(e) => { setRejectReason(e.target.value); setRejectError(''); }}
            placeholder="Укажите причину..."
          />
          {rejectError && <p className="text-sm text-red-600">{rejectError}</p>}
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={() => setRejectTarget(null)}>Отмена</Button>
            <Button type="submit" disabled={submitting}>Отклонить</Button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
