import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Select } from '../../components/ui/Select';
import { Modal } from '../../components/ui/Modal';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { StatusBadge } from '../../components/ui/Badge';
import { useToast } from '../../components/ui/Toast';
import { routesApi, type RouteInfo } from '../../api/admin';

const PAGE_SIZE = 20;

const columns: Column<RouteInfo>[] = [
  { key: 'name', header: 'Name', sortable: true },
  { key: 'pattern', header: 'Pattern' },
  { key: 'priority', header: 'Priority', sortable: true },
  { key: 'load_balance_strategy', header: 'Strategy' },
  { key: 'failover_enabled', header: 'Failover', render: (r) => r.failover_enabled ? 'Yes' : 'No' },
  { key: 'provider_ids', header: 'Providers', render: (r) => String(r.provider_ids?.length ?? 0) },
  { key: 'active', header: 'Status', render: (r) => <StatusBadge status={r.active ? 'active' : 'inactive'} /> },
];

export function RoutesPage() {
  const toast = useToast();
  const [data, setData] = useState<RouteInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [editRoute, setEditRoute] = useState<RouteInfo | null>(null);
  const [deleteRoute, setDeleteRoute] = useState<RouteInfo | null>(null);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({ name: '', pattern: '', priority: 0, provider_ids: '', load_balance_strategy: 'round_robin', failover_enabled: true });

  const fetchData = useCallback(async () => {
    setLoading(true);
    try { const res = await routesApi.list({ limit: PAGE_SIZE, offset: (page - 1) * PAGE_SIZE }); setData(res.routes || []); setTotal(res.total); }
    catch { toast.error('Failed to load routes'); }
    finally { setLoading(false); }
  }, [page, toast]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const openCreate = () => { setForm({ name: '', pattern: '', priority: 0, provider_ids: '', load_balance_strategy: 'round_robin', failover_enabled: true }); setShowForm(true); };
  const openEdit = (route: RouteInfo) => { setForm({ name: route.name, pattern: route.pattern, priority: route.priority, provider_ids: (route.provider_ids || []).join(', '), load_balance_strategy: route.load_balance_strategy, failover_enabled: route.failover_enabled }); setEditRoute(route); setShowForm(true); };

  const handleSave = async () => {
    setSaving(true);
    try {
      const payload = { ...form, priority: Number(form.priority), provider_ids: form.provider_ids.split(',').map((s) => s.trim()).filter(Boolean) };
      if (editRoute) { await routesApi.update(editRoute.route_id, payload); toast.success('Route updated'); }
      else { await routesApi.create(payload); toast.success('Route created'); }
      setShowForm(false); setEditRoute(null); fetchData();
    } catch (e) { toast.error(e instanceof Error ? e.message : 'Save failed'); }
    finally { setSaving(false); }
  };

  const handleDelete = async () => {
    if (!deleteRoute) return;
    setSaving(true);
    try { await routesApi.delete(deleteRoute.route_id); toast.success('Route deleted'); setDeleteRoute(null); fetchData(); }
    catch (e) { toast.error(e instanceof Error ? e.message : 'Delete failed'); }
    finally { setSaving(false); }
  };

  return (
    <>
      <PageHeader title="Routes" subtitle={`${total} routes`} actions={<Button onClick={openCreate}>Create Route</Button>} />
      <DataTable columns={columns} data={data} total={total} page={page} pageSize={PAGE_SIZE} onPageChange={setPage} loading={loading} keyField="route_id"
        rowActions={(r) => (<div className="flex gap-1"><Button size="sm" variant="ghost" onClick={() => openEdit(r)}>Edit</Button><Button size="sm" variant="ghost" onClick={() => setDeleteRoute(r)}>Delete</Button></div>)}
      />
      <Modal open={showForm} onClose={() => { setShowForm(false); setEditRoute(null); }} title={editRoute ? 'Edit Route' : 'Create Route'}>
        <div className="space-y-4">
          <Input label="Name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
          <Input label="Pattern (regex)" value={form.pattern} onChange={(e) => setForm({ ...form, pattern: e.target.value })} required />
          <Input label="Priority" type="number" value={String(form.priority)} onChange={(e) => setForm({ ...form, priority: Number(e.target.value) })} />
          <Input label="Provider IDs (comma-separated)" value={form.provider_ids} onChange={(e) => setForm({ ...form, provider_ids: e.target.value })} required />
          <Select label="Strategy" options={[{ value: 'round_robin', label: 'Round Robin' }, { value: 'weighted', label: 'Weighted' }, { value: 'priority', label: 'Priority' }]} value={form.load_balance_strategy} onChange={(v) => setForm({ ...form, load_balance_strategy: v })} />
          <Select label="Failover" options={[{ value: 'true', label: 'Enabled' }, { value: 'false', label: 'Disabled' }]} value={String(form.failover_enabled)} onChange={(v) => setForm({ ...form, failover_enabled: v === 'true' })} />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => { setShowForm(false); setEditRoute(null); }}>Cancel</Button>
            <Button onClick={handleSave} disabled={saving || !form.name || !form.pattern}>{saving ? 'Saving...' : 'Save'}</Button>
          </div>
        </div>
      </Modal>
      <ConfirmDialog open={!!deleteRoute} onConfirm={handleDelete} onCancel={() => setDeleteRoute(null)} title="Delete Route" description={`Delete "${deleteRoute?.name}"?`} confirmLabel="Delete" variant="danger" loading={saving} />
    </>
  );
}
