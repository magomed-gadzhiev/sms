import { useState, useEffect, useCallback, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { adminSenderNamesApi, AdminApiError, type AdminSenderNameInfo } from '../../api/admin';
import { Button } from '../../components/ui/Button';
import { Badge } from '../../components/ui/Badge';
import { Modal } from '../../components/ui/Modal';
import { Input } from '../../components/ui/Input';
import { DataTable, type Column } from '../../components/data/DataTable';

const PAGE_SIZE = 20;

const SENDER_NAME_REGEX = /^[A-Za-z0-9._-]{1,11}$/;

function validateSenderName(value: string): string {
  if (!value) return 'Поле обязательно';
  if (/\s/.test(value)) return 'Пробелы запрещены';
  if (!SENDER_NAME_REGEX.test(value)) return 'Латиница, не более 11 символов. Можно использовать цифры и знаки . _ —';
  return '';
}

const STATUS_BADGE: Record<string, { variant: 'warning' | 'success' | 'danger' | 'default'; label: string }> = {
  pending: { variant: 'warning', label: 'На модерации' },
  approved: { variant: 'success', label: 'Одобрено' },
  rejected: { variant: 'danger', label: 'Отклонено' },
  deactivated: { variant: 'default', label: 'Деактивировано' },
};

const STATUS_FILTERS = [
  { value: '', label: 'Все' },
  { value: 'pending', label: 'На модерации' },
  { value: 'approved', label: 'Одобрено' },
  { value: 'rejected', label: 'Отклонено' },
  { value: 'deactivated', label: 'Деактивировано' },
];

function formatDate(dt: string) {
  return new Date(dt).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });
}

export function SenderNamesAdminPage() {
  const navigate = useNavigate();
  const [items, setItems] = useState<AdminSenderNameInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [statusFilter, setStatusFilter] = useState('pending');
  const [nameQuery, setNameQuery] = useState('');

  // Reject modal
  const [rejectTarget, setRejectTarget] = useState<AdminSenderNameInfo | null>(null);
  const [rejectReason, setRejectReason] = useState('');
  const [rejectError, setRejectError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  // Deactivate modal
  const [deactivateTarget, setDeactivateTarget] = useState<AdminSenderNameInfo | null>(null);
  const [deactivateReason, setDeactivateReason] = useState('');
  const [deactivateError, setDeactivateError] = useState('');

  // Add modal
  const [showAddModal, setShowAddModal] = useState(false);
  const [addForm, setAddForm] = useState({ name: '', clientId: '', channel: 'sms' as 'sms' | 'voice' | 'viber' });
  const [nameError, setNameError] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const res = await adminSenderNamesApi.list({
        status: statusFilter || undefined,
        name_query: nameQuery || undefined,
        limit: PAGE_SIZE,
        offset,
      });
      setItems(res.sender_names ?? []);
      setTotal(res.total ?? 0);
    } catch (e) {
      setError(e instanceof AdminApiError ? e.message : 'Ошибка загрузки');
    } finally {
      setLoading(false);
    }
  }, [statusFilter, nameQuery, offset]);

  useEffect(() => { load(); }, [load]);

  const handleApprove = async (sn: AdminSenderNameInfo) => {
    try {
      await adminSenderNamesApi.approve(sn.id);
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
      await adminSenderNamesApi.reject(rejectTarget.id, rejectReason.trim());
      setRejectTarget(null);
      setRejectReason('');
      load();
    } catch (e) {
      setRejectError(e instanceof AdminApiError ? e.message : 'Ошибка');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDeactivate = async (e: FormEvent) => {
    e.preventDefault();
    if (!deactivateTarget) return;
    setSubmitting(true);
    try {
      await adminSenderNamesApi.deactivate(deactivateTarget.id, deactivateReason.trim());
      setDeactivateTarget(null);
      setDeactivateReason('');
      load();
    } catch (e) {
      setDeactivateError(e instanceof AdminApiError ? e.message : 'Ошибка');
    } finally {
      setSubmitting(false);
    }
  };

  const handleAddSubmit = async (e: FormEvent) => {
    e.preventDefault();
    const error = validateSenderName(addForm.name ?? '');
    if (error) { setNameError(error); return; }
    if (!addForm.clientId.trim()) { setNameError('Client ID обязателен'); return; }
    setSubmitting(true);
    try {
      await adminSenderNamesApi.create({
        client_id: addForm.clientId.trim(),
        name: addForm.name.trim(),
        channel: addForm.channel,
      });
      setShowAddModal(false);
      setAddForm({ name: '', clientId: '', channel: 'sms' });
      setNameError('');
      load();
    } catch (e) {
      setNameError(e instanceof AdminApiError ? e.message : 'Ошибка создания');
    } finally {
      setSubmitting(false);
    }
  };

  const page = Math.floor(offset / PAGE_SIZE) + 1;

  const columns: Column<AdminSenderNameInfo>[] = [
    { key: 'name', header: 'Имя', render: (sn) => <span className="font-mono font-medium">{sn.name}</span> },
    { key: 'client_email', header: 'Клиент', render: (sn) => (
      <span className="text-sm text-gray-700">{sn.client_email || sn.client_id}</span>
    ) },
    {
      key: 'status', header: 'Статус', render: (sn) => {
        const s = STATUS_BADGE[sn.status] ?? { variant: 'default' as const, label: sn.status };
        return <Badge variant={s.variant}>{s.label}</Badge>;
      },
    },
    { key: 'created_at', header: 'Создано', render: (sn) => formatDate(sn.created_at) },
    {
      key: 'actions', header: 'Действия', render: (sn) => (
        <div className="flex gap-1">
          {sn.status === 'pending' && (
            <>
              <Button size="sm" variant="secondary" onClick={(e) => { e.stopPropagation(); handleApprove(sn); }}>Одобрить</Button>
              <Button size="sm" variant="ghost" onClick={(e) => { e.stopPropagation(); setRejectTarget(sn); setRejectReason(''); setRejectError(''); }}>Отклонить</Button>
            </>
          )}
          {sn.status === 'approved' && (
            <Button size="sm" variant="ghost" onClick={(e) => { e.stopPropagation(); setDeactivateTarget(sn); setDeactivateReason(''); setDeactivateError(''); }}>Деактивировать</Button>
          )}
        </div>
      ),
    },
  ];

  return (
    <div>
      <h1 className="text-2xl font-semibold mb-6">Имена отправителей</h1>

      {error && <div className="mb-4 p-3 bg-red-50 text-red-700 rounded-md text-sm">{error}</div>}

      <div className="flex gap-3 mb-4">
        <Button onClick={() => { setShowAddModal(true); setAddForm({ name: '', clientId: '', channel: 'sms' }); setNameError(''); }}>Добавить имя</Button>
      </div>

      <div className="flex gap-3 mb-4">
        <select
          value={statusFilter}
          onChange={(e) => { setStatusFilter(e.target.value); setOffset(0); }}
          className="border border-gray-300 rounded-md px-3 py-1.5 text-sm"
        >
          {STATUS_FILTERS.map((f) => (
            <option key={f.value} value={f.value}>{f.label}</option>
          ))}
        </select>
        <input
          type="text"
          placeholder="Поиск по имени..."
          value={nameQuery}
          onChange={(e) => { setNameQuery(e.target.value); setOffset(0); }}
          className="border border-gray-300 rounded-md px-3 py-1.5 text-sm w-48"
        />
      </div>

      <DataTable
        columns={columns}
        data={items}
        loading={loading}
        total={total}
        page={page}
        pageSize={PAGE_SIZE}
        onPageChange={(p) => setOffset((p - 1) * PAGE_SIZE)}
        onRowClick={(sn) => navigate(`/admin/sender-names/${sn.id}`)}
      />

      {/* Add modal */}
      <Modal open={showAddModal} onClose={() => { setShowAddModal(false); setNameError(''); }} title="Добавить имя отправителя">
        <form onSubmit={handleAddSubmit} className="space-y-4">
          <div>
            <label htmlFor="add-client-id" className="block text-sm font-medium text-gray-700 mb-1">Client ID</label>
            <input
              id="add-client-id"
              type="text"
              value={addForm.clientId}
              onChange={(e) => setAddForm(f => ({ ...f, clientId: e.target.value }))}
              placeholder="UUID клиента..."
              className="w-full border border-gray-300 rounded-md px-3 py-1.5 text-sm"
            />
          </div>
          <Input
            label="Имя"
            value={addForm.name}
            onChange={(e) => {
              setAddForm(f => ({ ...f, name: e.target.value }));
              setNameError(validateSenderName(e.target.value));
            }}
            placeholder="Укажите имя..."
          />
          <div>
            <label htmlFor="add-channel" className="block text-sm font-medium text-gray-700 mb-1">Канал</label>
            <select
              id="add-channel"
              value={addForm.channel}
              onChange={(e) => setAddForm(f => ({ ...f, channel: e.target.value as 'sms' | 'voice' | 'viber' }))}
              className="w-full border border-gray-300 rounded-md px-3 py-1.5 text-sm"
            >
              <option value="sms">SMS</option>
              <option value="voice">Voice</option>
              <option value="viber">Viber</option>
            </select>
          </div>
          <div className="mt-1 space-y-0.5">
            <p className="text-xs text-gray-500">Имя должно совпадать с названием организации, ИП, товарным знаком или доменом</p>
            <p className="text-xs text-gray-500">Латиница, не более 11 символов. Можно использовать цифры и знаки . _ —</p>
            <p className="text-xs text-gray-500">Пробелы запрещены</p>
            {nameError && <p className="text-xs text-red-500">{nameError}</p>}
          </div>
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={() => { setShowAddModal(false); setNameError(''); }}>Отмена</Button>
            <Button type="submit" disabled={submitting || !!nameError}>Добавить</Button>
          </div>
        </form>
      </Modal>

      {/* Reject modal */}
      <Modal open={!!rejectTarget} onClose={() => setRejectTarget(null)} title="Отклонить имя отправителя">
        <form onSubmit={handleReject} className="space-y-4">
          <p className="text-sm text-gray-600">Имя: <strong className="font-mono">{rejectTarget?.name}</strong></p>
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

      {/* Deactivate modal */}
      <Modal open={!!deactivateTarget} onClose={() => setDeactivateTarget(null)} title="Деактивировать имя отправителя">
        <form onSubmit={handleDeactivate} className="space-y-4">
          <p className="text-sm text-gray-600">Имя: <strong className="font-mono">{deactivateTarget?.name}</strong></p>
          <Input
            label="Причина (опционально)"
            value={deactivateReason}
            onChange={(e) => { setDeactivateReason(e.target.value); setDeactivateError(''); }}
            placeholder="Укажите причину..."
          />
          {deactivateError && <p className="text-sm text-red-600">{deactivateError}</p>}
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={() => setDeactivateTarget(null)}>Отмена</Button>
            <Button type="submit" disabled={submitting}>Деактивировать</Button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
