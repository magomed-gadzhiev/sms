import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { FilterBar, type FilterDef } from '../../components/data/FilterBar';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Modal } from '../../components/ui/Modal';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { StatusBadge } from '../../components/ui/Badge';
import { useToast } from '../../components/ui/Toast';
import { webhooksAdminApi, type WebhookInfo } from '../../api/admin';

const filters: FilterDef[] = [{ key: 'client_id', label: 'Client ID', type: 'text', placeholder: 'UUID...' }];

const columns: Column<WebhookInfo>[] = [
  { key: 'url', header: 'URL' },
  { key: 'client_id', header: 'Client', render: (w) => w.client_id.slice(0, 8) + '...' },
  { key: 'events', header: 'Events', render: (w) => (w.events || []).join(', ') },
  { key: 'active', header: 'Status', render: (w) => <StatusBadge status={w.active ? 'active' : 'inactive'} /> },
  { key: 'created_at', header: 'Created', render: (w) => new Date(w.created_at).toLocaleDateString() },
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

  const fetchData = useCallback(async () => {
    setLoading(true);
    try { const res = await webhooksAdminApi.list(filterValues); setData(res.webhooks || []); }
    catch { toast.error('Failed to load webhooks'); }
    finally { setLoading(false); }
  }, [filterValues, toast]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const handleCreate = async () => {
    setSaving(true);
    try { await webhooksAdminApi.create({ client_id: form.client_id, url: form.url, events: form.events.split(',').map((s) => s.trim()).filter(Boolean), active: true }); toast.success('Webhook created'); setShowCreate(false); fetchData(); }
    catch (e) { toast.error(e instanceof Error ? e.message : 'Failed'); }
    finally { setSaving(false); }
  };

  const handleDelete = async () => {
    if (!deleteWebhook) return;
    setSaving(true);
    try { await webhooksAdminApi.delete(deleteWebhook.webhook_id, deleteWebhook.client_id); toast.success('Webhook deleted'); setDeleteWebhook(null); fetchData(); }
    catch (e) { toast.error(e instanceof Error ? e.message : 'Failed'); }
    finally { setSaving(false); }
  };

  return (
    <>
      <PageHeader title="Webhooks" subtitle={`${data.length} webhooks`} actions={<Button onClick={() => { setForm({ client_id: '', url: '', events: '' }); setShowCreate(true); }}>Create Webhook</Button>} />
      <FilterBar filters={filters} values={filterValues} onChange={setFilterValues} onReset={() => setFilterValues({})} />
      <DataTable columns={columns} data={data} total={data.length} page={1} pageSize={100} onPageChange={() => {}} loading={loading} keyField="webhook_id"
        rowActions={(w) => <Button size="sm" variant="ghost" onClick={() => setDeleteWebhook(w)}>Delete</Button>}
      />
      <Modal open={showCreate} onClose={() => setShowCreate(false)} title="Create Webhook">
        <div className="space-y-4">
          <Input label="Client ID" value={form.client_id} onChange={(e) => setForm({ ...form, client_id: e.target.value })} required placeholder="UUID" />
          <Input label="URL" value={form.url} onChange={(e) => setForm({ ...form, url: e.target.value })} required placeholder="https://..." />
          <Input label="Events (comma-separated)" value={form.events} onChange={(e) => setForm({ ...form, events: e.target.value })} placeholder="message.delivered, message.failed" />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setShowCreate(false)}>Cancel</Button>
            <Button onClick={handleCreate} disabled={saving || !form.client_id || !form.url}>{saving ? 'Creating...' : 'Create'}</Button>
          </div>
        </div>
      </Modal>
      <ConfirmDialog open={!!deleteWebhook} onConfirm={handleDelete} onCancel={() => setDeleteWebhook(null)} title="Delete Webhook" description={`Delete webhook for "${deleteWebhook?.url}"?`} confirmLabel="Delete" variant="danger" loading={saving} />
    </>
  );
}
