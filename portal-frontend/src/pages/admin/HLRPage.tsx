import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Modal } from '../../components/ui/Modal';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { StatusBadge } from '../../components/ui/Badge';
import { useToast } from '../../components/ui/Toast';
import { hlrApi, type HLRProvider } from '../../api/admin';

export function HLRPage() {
  const toast = useToast();
  const [providers, setProviders] = useState<HLRProvider[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreateProvider, setShowCreateProvider] = useState(false);
  const [deleteProvider, setDeleteProvider] = useState<HLRProvider | null>(null);
  const [saving, setSaving] = useState(false);
  const [providerForm, setProviderForm] = useState({ name: '', active: true });

  const fetchData = useCallback(async () => {
    setLoading(true);
    try { const pRes = await hlrApi.listProviders(); setProviders(pRes.providers || []); }
    catch { toast.error('Failed to load HLR data'); }
    finally { setLoading(false); }
  }, [toast]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const providerColumns: Column<HLRProvider>[] = [
    { key: 'name', header: 'Name' },
    { key: 'active', header: 'Status', render: (p) => <StatusBadge status={p.active ? 'active' : 'inactive'} /> },
    { key: 'created_at', header: 'Created', render: (p) => new Date(p.created_at).toLocaleDateString() },
  ];

  const handleCreateProvider = async () => { setSaving(true); try { await hlrApi.createProvider(providerForm); toast.success('HLR provider created'); setShowCreateProvider(false); fetchData(); } catch (e) { toast.error(e instanceof Error ? e.message : 'Failed'); } finally { setSaving(false); } };
  const handleDeleteProvider = async () => { if (!deleteProvider) return; setSaving(true); try { await hlrApi.deleteProvider(deleteProvider.provider_id); toast.success('Deleted'); setDeleteProvider(null); fetchData(); } catch (e) { toast.error(e instanceof Error ? e.message : 'Failed'); } finally { setSaving(false); } };

  return (
    <>
      <PageHeader title="HLR Providers" breadcrumbs={[{ label: 'Админ', href: '/admin/dashboard' }, { label: 'HLR' }]} />
      <div className="mb-8">
        <div className="flex items-center justify-between mb-3"><h2 className="text-lg font-semibold">HLR Providers</h2><Button size="sm" onClick={() => { setProviderForm({ name: '', active: true }); setShowCreateProvider(true); }}>Add Provider</Button></div>
        <DataTable columns={providerColumns} data={providers} total={providers.length} page={1} pageSize={100} onPageChange={() => {}} loading={loading} keyField="provider_id" rowActions={(p) => <Button size="sm" variant="ghost" onClick={() => setDeleteProvider(p)}>Delete</Button>} />
      </div>
      <Modal open={showCreateProvider} onClose={() => setShowCreateProvider(false)} title="Add HLR Provider">
        <div className="space-y-4">
          <Input label="Name" value={providerForm.name} onChange={(e) => setProviderForm({ ...providerForm, name: e.target.value })} required />
          <div className="flex justify-end gap-3 pt-2"><Button variant="secondary" onClick={() => setShowCreateProvider(false)}>Cancel</Button><Button onClick={handleCreateProvider} disabled={saving || !providerForm.name}>{saving ? 'Creating...' : 'Create'}</Button></div>
        </div>
      </Modal>
      <ConfirmDialog open={!!deleteProvider} onConfirm={handleDeleteProvider} onCancel={() => setDeleteProvider(null)} title="Delete HLR Provider" description={`Delete "${deleteProvider?.name}"?`} confirmLabel="Delete" variant="danger" loading={saving} />
    </>
  );
}
