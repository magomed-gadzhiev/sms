import { useState, useEffect, useCallback, type FormEvent } from 'react';
import { adminSenderNamesApi, AdminApiError, type AdminSenderNameInfo } from '../../../api/admin';
import { PageHeader } from '../../../components/layout/PageHeader';
import { FilterBar, type FilterDef } from '../../../components/data/FilterBar';
import { Button } from '../../../components/ui/Button';
import { Badge } from '../../../components/ui/Badge';
import { Modal } from '../../../components/ui/Modal';
import { Input } from '../../../components/ui/Input';
import { DataTable, type Column } from '../../../components/data/DataTable';

const PAGE_SIZE = 20;

const STATUS_BADGE: Record<string, { variant: 'success' | 'default'; label: string }> = {
  approved:    { variant: 'success', label: 'Одобрено' },
  deactivated: { variant: 'default', label: 'Деактивировано' },
};

const CHANNEL_CHIP: Record<string, string> = { sms: 'SMS', voice: 'Voice', viber: 'Viber' };

const NAME_REGEX = /^[A-Za-z0-9._-]{1,11}$/;
const NUMERIC_REGEX = /^\d{1,15}$/;

function validateName(name: string): string {
  if (!name) return 'Имя обязательно';
  if (/\s/.test(name)) return 'Пробелы запрещены';
  if (NUMERIC_REGEX.test(name) || NAME_REGEX.test(name)) return '';
  return 'Латиница, не более 11 символов. Можно использовать цифры и знаки . _ -';
}

function formatDate(dt: string) {
  return new Date(dt).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });
}

export function SenderNameDirectoryPage() {
  const [items, setItems] = useState<AdminSenderNameInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const [status, setStatus] = useState('approved');
  const [channel, setChannel] = useState('');
  const [nameQuery, setNameQuery] = useState('');

  // Deactivate modal
  const [deactivateTarget, setDeactivateTarget] = useState<AdminSenderNameInfo | null>(null);
  const [deactivateReason, setDeactivateReason] = useState('');
  const [deactivateError, setDeactivateError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  // Create modal
  const [showCreate, setShowCreate] = useState(false);
  const [createForm, setCreateForm] = useState({ clientId: '', channel: 'sms' as 'sms' | 'voice' | 'viber', name: '' });
  const [createError, setCreateError] = useState('');
  const [nameError, setNameError] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const res = await adminSenderNamesApi.list({
        status: status || undefined,
        channel: channel || undefined,
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
  }, [status, channel, nameQuery, offset]);

  useEffect(() => { load(); }, [load]);

  const emitChanged = () => window.dispatchEvent(new CustomEvent('moderation:changed'));

  const handleDeactivate = async (e: FormEvent) => {
    e.preventDefault();
    if (!deactivateTarget) return;
    setSubmitting(true);
    setDeactivateError('');
    try {
      await adminSenderNamesApi.deactivate(deactivateTarget.id, deactivateReason.trim());
      setDeactivateTarget(null);
      setDeactivateReason('');
      emitChanged();
      load();
    } catch (e) {
      setDeactivateError(e instanceof AdminApiError ? e.message : 'Ошибка');
    } finally {
      setSubmitting(false);
    }
  };

  const handleCreate = async (e: FormEvent) => {
    e.preventDefault();
    const nErr = validateName(createForm.name);
    if (nErr) { setNameError(nErr); return; }
    if (!createForm.clientId.trim()) { setCreateError('Client ID обязателен'); return; }
    setSubmitting(true);
    setCreateError('');
    try {
      await adminSenderNamesApi.create({
        client_id: createForm.clientId.trim(),
        channel: createForm.channel,
        name: createForm.name.trim(),
      });
      setShowCreate(false);
      setCreateForm({ clientId: '', channel: 'sms', name: '' });
      setNameError('');
      emitChanged();
      load();
    } catch (e) {
      setCreateError(e instanceof AdminApiError ? e.message : 'Ошибка создания');
    } finally {
      setSubmitting(false);
    }
  };

  const page = Math.floor(offset / PAGE_SIZE) + 1;

  const filters: FilterDef[] = [
    {
      key: 'status',
      label: 'Статус',
      type: 'select',
      options: [
        { value: 'approved',    label: 'Одобрено' },
        { value: 'deactivated', label: 'Деактивировано' },
      ],
    },
    {
      key: 'channel',
      label: 'Канал',
      type: 'select',
      options: [
        { value: 'sms',   label: 'SMS' },
        { value: 'voice', label: 'Voice' },
        { value: 'viber', label: 'Viber' },
      ],
      placeholder: 'Все каналы',
    },
    {
      key: 'name_query',
      label: 'Имя',
      type: 'text',
      placeholder: 'Поиск...',
    },
  ];

  const columns: Column<AdminSenderNameInfo>[] = [
    { key: 'name', header: 'Имя', render: (sn) => <span className="font-mono font-medium">{sn.name}</span> },
    { key: 'client_email', header: 'Клиент', render: (sn) => <span className="text-sm text-gray-700">{sn.client_email ?? sn.client_id}</span> },
    { key: 'channel', header: 'Канал', render: (sn) => <Badge variant="default">{CHANNEL_CHIP[sn.channel] ?? sn.channel}</Badge> },
    {
      key: 'status',
      header: 'Статус',
      render: (sn) => {
        const c = STATUS_BADGE[sn.status] ?? { variant: 'default' as const, label: sn.status };
        return <Badge variant={c.variant}>{c.label}</Badge>;
      },
    },
    { key: 'created_at', header: 'Создано', render: (sn) => formatDate(sn.created_at) },
    {
      key: 'actions',
      header: 'Действия',
      render: (sn) => (
        sn.status === 'approved' ? (
          <Button
            size="sm"
            variant="ghost"
            onClick={(e) => {
              e.stopPropagation();
              setDeactivateTarget(sn);
              setDeactivateReason('');
              setDeactivateError('');
            }}
          >
            Деактивировать
          </Button>
        ) : null
      ),
    },
  ];

  return (
    <div>
      <PageHeader
        title="Справочник имён отправителей"
        subtitle={`${total} записей`}
        breadcrumbs={[{ label: 'Админ', href: '/admin/dashboard' }, { label: 'Справочник имён' }]}
        actions={<Button onClick={() => { setShowCreate(true); setCreateForm({ clientId: '', channel: 'sms', name: '' }); setNameError(''); setCreateError(''); }}>Создать имя</Button>}
      />

      {error && <div className="mb-4 p-3 bg-red-50 text-red-700 rounded-md text-sm">{error}</div>}

      <FilterBar
        filters={filters}
        values={{ status, channel, name_query: nameQuery }}
        onChange={(v) => {
          setStatus(v.status ?? 'approved');
          setChannel(v.channel ?? '');
          setNameQuery(v.name_query ?? '');
          setOffset(0);
        }}
        onReset={() => { setStatus('approved'); setChannel(''); setNameQuery(''); setOffset(0); }}
      />

      {!loading && !error && items.length === 0 ? (
        <div className="text-center py-12 text-gray-500">Ничего не найдено по выбранным фильтрам</div>
      ) : (
        <DataTable
          columns={columns}
          data={items}
          loading={loading}
          total={total}
          page={page}
          pageSize={PAGE_SIZE}
          onPageChange={(p) => setOffset((p - 1) * PAGE_SIZE)}
        />
      )}

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

      {/* Create modal */}
      <Modal open={showCreate} onClose={() => setShowCreate(false)} title="Создать имя отправителя">
        <form onSubmit={handleCreate} className="space-y-4">
          <div>
            <label htmlFor="dir-create-client-id" className="block text-sm font-medium text-gray-700 mb-1">Client ID</label>
            <input
              id="dir-create-client-id"
              type="text"
              value={createForm.clientId}
              onChange={(e) => setCreateForm(f => ({ ...f, clientId: e.target.value }))}
              placeholder="UUID клиента"
              className="w-full border border-gray-300 rounded-md px-3 py-1.5 text-sm"
            />
          </div>
          <div>
            <label htmlFor="dir-create-channel" className="block text-sm font-medium text-gray-700 mb-1">Канал</label>
            <select
              id="dir-create-channel"
              value={createForm.channel}
              onChange={(e) => setCreateForm(f => ({ ...f, channel: e.target.value as 'sms' | 'voice' | 'viber' }))}
              className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
            >
              <option value="sms">SMS</option>
              <option value="voice">Voice</option>
              <option value="viber">Viber</option>
            </select>
          </div>
          <div>
            <Input
              label="Имя"
              value={createForm.name}
              onChange={(e) => {
                const v = e.target.value;
                setCreateForm(f => ({ ...f, name: v }));
                setNameError(validateName(v));
              }}
              placeholder="Например: MyBrand"
            />
            <p className="mt-1 text-xs text-gray-500">
              Латиница, не более 11 символов. Можно использовать цифры и знаки . _ -. Пробелы запрещены.
            </p>
            {nameError && <p className="mt-1 text-sm text-red-600">{nameError}</p>}
          </div>
          {createError && <p className="text-sm text-red-600">{createError}</p>}
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={() => setShowCreate(false)}>Отмена</Button>
            <Button type="submit" disabled={submitting || !!nameError}>Создать</Button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
