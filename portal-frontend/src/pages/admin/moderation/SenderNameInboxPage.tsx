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

const CHANNEL_CHIP: Record<string, string> = {
  sms: 'SMS',
  voice: 'Voice',
  viber: 'Viber',
};

function formatDate(dt: string) {
  return new Date(dt).toLocaleString('ru-RU', { dateStyle: 'short', timeStyle: 'short' });
}

export function SenderNameInboxPage() {
  const [items, setItems] = useState<AdminSenderNameInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const [channel, setChannel] = useState('');
  const [nameQuery, setNameQuery] = useState('');

  // Reject modal state
  const [rejectTarget, setRejectTarget] = useState<AdminSenderNameInfo | null>(null);
  const [rejectReason, setRejectReason] = useState('');
  const [rejectError, setRejectError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const res = await adminSenderNamesApi.list({
        status: 'pending',
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
  }, [channel, nameQuery, offset]);

  useEffect(() => { load(); }, [load]);

  const emitChanged = () => window.dispatchEvent(new CustomEvent('moderation:changed'));

  const handleApprove = async (sn: AdminSenderNameInfo) => {
    try {
      await adminSenderNamesApi.approve(sn.id);
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
      await adminSenderNamesApi.reject(rejectTarget.id, rejectReason.trim());
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

  const page = Math.floor(offset / PAGE_SIZE) + 1;

  const filters: FilterDef[] = [
    {
      key: 'channel',
      label: 'Канал',
      type: 'select',
      options: [
        { value: 'sms', label: 'SMS' },
        { value: 'voice', label: 'Voice' },
        { value: 'viber', label: 'Viber' },
      ],
      placeholder: 'Все каналы',
    },
    {
      key: 'name_query',
      label: 'Поиск по имени',
      type: 'text',
      placeholder: 'Поиск...',
    },
  ];

  const columns: Column<AdminSenderNameInfo>[] = [
    { key: 'name', header: 'Имя', render: (sn) => <span className="font-mono font-medium">{sn.name}</span> },
    {
      key: 'client_email',
      header: 'Клиент',
      render: (sn) => <span className="text-sm text-gray-700">{sn.client_email ?? sn.client_id}</span>,
    },
    {
      key: 'channel',
      header: 'Канал',
      render: (sn) => (
        <Badge variant="default">{CHANNEL_CHIP[sn.channel] ?? sn.channel}</Badge>
      ),
    },
    {
      key: 'created_at',
      header: 'Подано',
      render: (sn) => <span className="text-sm text-gray-600">{formatDate(sn.created_at)}</span>,
    },
    {
      key: 'actions',
      header: 'Действия',
      render: (sn) => (
        <div className="flex gap-1">
          <Button size="sm" onClick={(e) => { e.stopPropagation(); handleApprove(sn); }}>Одобрить</Button>
          <Button
            size="sm"
            variant="ghost"
            onClick={(e) => { e.stopPropagation(); setRejectTarget(sn); setRejectReason(''); setRejectError(''); }}
          >
            Отклонить
          </Button>
        </div>
      ),
    },
  ];

  return (
    <div>
      <PageHeader
        title="Модерация имён отправителей"
        subtitle={total > 0 ? `${total} заявок ожидают модерации` : 'Очередь пуста'}
        breadcrumbs={[
          { label: 'Админ', href: '/admin/dashboard' },
          { label: 'Модерация имён' },
        ]}
      />

      {error && <div className="mb-4 p-3 bg-red-50 text-red-700 rounded-md text-sm">{error}</div>}

      <FilterBar
        filters={filters}
        values={{ channel, name_query: nameQuery }}
        onChange={(v) => {
          setChannel(v.channel ?? '');
          setNameQuery(v.name_query ?? '');
          setOffset(0);
        }}
        onReset={() => { setChannel(''); setNameQuery(''); setOffset(0); }}
      />

      {!loading && !error && items.length === 0 ? (
        <div className="text-center py-16 text-gray-500">
          <p className="mb-1">Очередь модерации пуста</p>
          <p className="text-sm text-gray-400">Новые заявки появятся здесь автоматически</p>
        </div>
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
    </div>
  );
}
