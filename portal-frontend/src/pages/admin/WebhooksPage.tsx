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

const columns: Column<WebhookInfo>[] = [
  { key: 'url', header: 'URL' },
  { key: 'client_id', header: 'Клиент', render: (w) => w.client_id.slice(0, 8) + '...' },
  { key: 'events', header: 'События', render: (w) => (w.events || []).join(', ') },
  { key: 'active', header: 'Статус', render: (w) => <StatusBadge status={w.active ? 'active' : 'inactive'} /> },
  { key: 'created_at', header: 'Создан', render: (w) => new Date(w.created_at).toLocaleDateString() },
];

export function WebhooksPage() {
  const toast = useToast();
  const [data, setData] = useState<WebhookInfo[]>([]);
  const [loading, setLoading] = useState(true);
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
      placeholder: 'Все клиенты',
    },
  ];

  const fetchData = useCallback(async () => {
    setLoading(true);
    try { const res = await webhooksAdminApi.list(filterValues); setData(res.webhooks || []); }
    catch { toast.error('Не удалось загрузить вебхуки'); }
    finally { setLoading(false); }
  }, [filterValues, toast]);

  const fetchClients = useCallback(async () => {
    setClientsLoading(true);
    try { const res = await clientsApi.list({ limit: 500 }); setClients(res.clients || []); }
    catch { /* non-critical */ }
    finally { setClientsLoading(false); }
  }, []);

  useEffect(() => { fetchData(); }, [fetchData]);
  useEffect(() => { fetchClients(); }, [fetchClients]);

  const handleCreate = async () => {
    setSaving(true);
    try { await webhooksAdminApi.create({ client_id: form.client_id, url: form.url, events: form.events.split(',').map((s) => s.trim()).filter(Boolean), active: true }); toast.success('Вебхук создан'); setShowCreate(false); fetchData(); }
    catch (e) { toast.error(e instanceof Error ? e.message : 'Ошибка'); }
    finally { setSaving(false); }
  };

  const handleDelete = async () => {
    if (!deleteWebhook) return;
    setSaving(true);
    try { await webhooksAdminApi.delete(deleteWebhook.webhook_id, deleteWebhook.client_id); toast.success('Вебхук удалён'); setDeleteWebhook(null); fetchData(); }
    catch (e) { toast.error(e instanceof Error ? e.message : 'Ошибка'); }
    finally { setSaving(false); }
  };

  return (
    <>
      <PageHeader title="Вебхуки" subtitle={`${data.length} вебхуков`} breadcrumbs={[{ label: 'Админ', href: '/admin/dashboard' }, { label: 'Вебхуки' }]} actions={<Button onClick={() => { setForm({ client_id: '', url: '', events: '' }); setShowCreate(true); }}>Создать вебхук</Button>} />
      <FilterBar filters={filters} values={filterValues} onChange={setFilterValues} onReset={() => setFilterValues({})} />
      <DataTable columns={columns} data={data} total={data.length} page={1} pageSize={100} onPageChange={() => {}} loading={loading} keyField="webhook_id"
        rowActions={(w) => <Button size="sm" variant="ghost" onClick={() => setDeleteWebhook(w)}>Удалить</Button>}
        emptyMessage={
          <div className="text-center py-12 text-gray-500">
            <p className="text-lg font-medium">Нет данных</p>
            <p className="text-sm mt-1">Нажмите «Создать вебхук», чтобы добавить первый вебхук</p>
          </div>
        }
      />
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
          <Input label="URL" value={form.url} onChange={(e) => setForm({ ...form, url: e.target.value })} required placeholder="https://..." />
          <Input label="События (через запятую)" value={form.events} onChange={(e) => setForm({ ...form, events: e.target.value })} placeholder="message.delivered, message.failed" />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setShowCreate(false)}>Отмена</Button>
            <Button onClick={handleCreate} disabled={saving || !form.client_id || !form.url}>{saving ? 'Создание...' : 'Создать'}</Button>
          </div>
        </div>
      </Modal>
      <ConfirmDialog open={!!deleteWebhook} onConfirm={handleDelete} onCancel={() => setDeleteWebhook(null)} title="Удаление вебхука" description={`Удалить вебхук "${deleteWebhook?.url}"?`} confirmLabel="Удалить" variant="danger" loading={saving} />
    </>
  );
}
