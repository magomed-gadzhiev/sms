import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { FilterBar, type FilterDef } from '../../components/data/FilterBar';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { SearchableSelect } from '../../components/ui/SearchableSelect';
import { Modal } from '../../components/ui/Modal';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { StatusBadge } from '../../components/ui/Badge';
import { useToast } from '../../components/ui/Toast';
import { webhooksAdminApi, clientsApi, type WebhookInfo, type ClientInfo } from '../../api/admin';

const VALID_EVENT_TYPES = ['delivered', 'failed', 'expired', 'rejected'] as const;

const columns: Column<WebhookInfo>[] = [
  { key: 'url', header: 'URL' },
  { key: 'client_id', header: 'Клиент', render: (w) => w.client_id.slice(0, 8) + '...' },
  { key: 'event_types', header: 'События', render: (w) => (w.event_types || []).join(', ') },
  { key: 'active', header: 'Статус', render: (w) => <StatusBadge status={w.active ? 'active' : 'inactive'} /> },
  { key: 'created_at', header: 'Создан', render: (w) => new Date(w.created_at).toLocaleDateString() },
];

export function WebhooksPage() {
  const toast = useToast();
  const [data, setData] = useState<WebhookInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [filterValues, setFilterValues] = useState<Record<string, string>>({});
  const [showCreate, setShowCreate] = useState(false);
  const [deleteWebhook, setDeleteWebhook] = useState<WebhookInfo | null>(null);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({ client_id: '', url: '', events: '' });
  const [clients, setClients] = useState<ClientInfo[]>([]);
  const [clientsLoading, setClientsLoading] = useState(false);

  const clientOptions = clients.map((c) => ({ value: c.client_id, label: c.name }));

  const filters: FilterDef[] = [
    {
      key: 'client_id',
      label: 'Клиент',
      type: 'select',
      options: clientOptions,
      placeholder: 'Выберите клиента...',
    },
  ];

  const selectedClientID = filterValues.client_id || '';

  const fetchData = useCallback(async () => {
    if (!selectedClientID) {
      setData([]);
      return;
    }
    setLoading(true);
    try {
      const res = await webhooksAdminApi.list({ client_id: selectedClientID });
      setData(res.subscriptions || []);
    } catch {
      toast.error('Не удалось загрузить вебхуки');
    } finally {
      setLoading(false);
    }
  }, [selectedClientID, toast]);

  const fetchClients = useCallback(async () => {
    setClientsLoading(true);
    try { const res = await clientsApi.list({ limit: 500 }); setClients(res.clients || []); }
    catch { /* non-critical */ }
    finally { setClientsLoading(false); }
  }, []);

  useEffect(() => { fetchData(); }, [fetchData]);
  useEffect(() => { fetchClients(); }, [fetchClients]);

  const parsedEvents = form.events.split(',').map((s) => s.trim()).filter(Boolean);
  const invalidEvents = parsedEvents.filter((e) => !VALID_EVENT_TYPES.includes(e as typeof VALID_EVENT_TYPES[number]));

  const handleCreate = async () => {
    if (parsedEvents.length === 0) {
      toast.error('Укажите хотя бы один тип события');
      return;
    }
    if (invalidEvents.length > 0) {
      toast.error(`Недопустимые типы: ${invalidEvents.join(', ')}. Доступно: ${VALID_EVENT_TYPES.join(', ')}`);
      return;
    }
    setSaving(true);
    try {
      await webhooksAdminApi.create({ client_id: form.client_id, url: form.url, event_types: parsedEvents });
      toast.success('Вебхук создан');
      setShowCreate(false);
      if (form.client_id !== selectedClientID) {
        setFilterValues({ client_id: form.client_id });
      } else {
        fetchData();
      }
    } catch (e) { toast.error(e instanceof Error ? e.message : 'Ошибка'); }
    finally { setSaving(false); }
  };

  const handleDelete = async () => {
    if (!deleteWebhook) return;
    setSaving(true);
    try { await webhooksAdminApi.delete(deleteWebhook.id, deleteWebhook.client_id); toast.success('Вебхук удалён'); setDeleteWebhook(null); fetchData(); }
    catch (e) { toast.error(e instanceof Error ? e.message : 'Ошибка'); }
    finally { setSaving(false); }
  };

  return (
    <>
      <PageHeader title="Вебхуки" subtitle={selectedClientID ? `${data.length} вебхуков` : 'Выберите клиента, чтобы увидеть вебхуки'} breadcrumbs={[{ label: 'Админ', href: '/admin/dashboard' }, { label: 'Вебхуки' }]} actions={<Button onClick={() => { setForm({ client_id: selectedClientID, url: '', events: '' }); setShowCreate(true); }}>Создать вебхук</Button>} />
      <FilterBar filters={filters} values={filterValues} onChange={setFilterValues} onReset={() => setFilterValues({})} />
      {!selectedClientID ? (
        <div className="text-center py-12 text-gray-500 border border-dashed rounded">
          <p className="text-lg font-medium">Клиент не выбран</p>
          <p className="text-sm mt-1">Выберите клиента в фильтре выше, чтобы увидеть его вебхуки</p>
        </div>
      ) : (
        <DataTable columns={columns} data={data} total={data.length} page={1} pageSize={100} onPageChange={() => {}} loading={loading} keyField="id"
          rowActions={(w) => <Button size="sm" variant="ghost" onClick={() => setDeleteWebhook(w)}>Удалить</Button>}
          emptyMessage={
            <div className="text-center py-12 text-gray-500">
              <p className="text-lg font-medium">Нет данных</p>
              <p className="text-sm mt-1">Нажмите «Создать вебхук», чтобы добавить первый вебхук</p>
            </div>
          }
        />
      )}
      <Modal open={showCreate} onClose={() => setShowCreate(false)} title="Создание вебхука">
        <div className="space-y-4">
          <SearchableSelect
            label="Клиент"
            options={clientOptions}
            value={form.client_id}
            onChange={(v) => setForm({ ...form, client_id: v })}
            placeholder="Выберите клиента..."
            loading={clientsLoading}
            required
          />
          <Input label="URL (HTTPS)" value={form.url} onChange={(e) => setForm({ ...form, url: e.target.value })} required placeholder="https://..." />
          <div>
            <Input label="События (через запятую)" value={form.events} onChange={(e) => setForm({ ...form, events: e.target.value })} placeholder="delivered, failed, expired, rejected" />
            <p className="text-xs text-gray-500 mt-1">Доступные: {VALID_EVENT_TYPES.join(', ')}</p>
            {invalidEvents.length > 0 && (
              <p className="text-xs text-red-600 mt-1">Недопустимые: {invalidEvents.join(', ')}</p>
            )}
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setShowCreate(false)}>Отмена</Button>
            <Button onClick={handleCreate} disabled={saving || !form.client_id || !form.url || parsedEvents.length === 0 || invalidEvents.length > 0}>{saving ? 'Создание...' : 'Создать'}</Button>
          </div>
        </div>
      </Modal>
      <ConfirmDialog open={!!deleteWebhook} onConfirm={handleDelete} onCancel={() => setDeleteWebhook(null)} title="Удаление вебхука" description={`Удалить вебхук "${deleteWebhook?.url}"?`} confirmLabel="Удалить" variant="danger" loading={saving} />
    </>
  );
}
