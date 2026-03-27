import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Modal } from '../../components/ui/Modal';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { StatusBadge } from '../../components/ui/Badge';
import { useToast } from '../../components/ui/Toast';
import { hlrApi, type HLRProvider, type SmartRouteWeight } from '../../api/admin';

export function HLRPage() {
  const toast = useToast();
  const [providers, setProviders] = useState<HLRProvider[]>([]);
  const [weights, setWeights] = useState<SmartRouteWeight[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreateProvider, setShowCreateProvider] = useState(false);
  const [showCreateWeight, setShowCreateWeight] = useState(false);
  const [deleteProvider, setDeleteProvider] = useState<HLRProvider | null>(null);
  const [deleteWeight, setDeleteWeight] = useState<SmartRouteWeight | null>(null);
  const [saving, setSaving] = useState(false);
  const [providerForm, setProviderForm] = useState({ name: '', active: true });
  const [weightForm, setWeightForm] = useState({ country_code: '', provider_id: '', weight: 100 });

  const fetchData = useCallback(async () => {
    setLoading(true);
    try { const [pRes, wRes] = await Promise.all([hlrApi.listProviders(), hlrApi.listWeights()]); setProviders(pRes.providers || []); setWeights(wRes.weights || []); }
    catch { toast.error('Failed to load HLR data'); }
    finally { setLoading(false); }
  }, [toast]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const providerColumns: Column<HLRProvider>[] = [
    { key: 'name', header: 'Name' },
    { key: 'active', header: 'Status', render: (p) => <StatusBadge status={p.active ? 'active' : 'inactive'} /> },
    { key: 'created_at', header: 'Created', render: (p) => new Date(p.created_at).toLocaleDateString() },
  ];
  const weightColumns: Column<SmartRouteWeight>[] = [
    { key: 'country_code', header: 'Country' },
    { key: 'provider_id', header: 'Provider', render: (w) => w.provider_id.slice(0, 8) + '...' },
    { key: 'weight', header: 'Weight' },
  ];

  const handleCreateProvider = async () => { setSaving(true); try { await hlrApi.createProvider(providerForm); toast.success('HLR provider created'); setShowCreateProvider(false); fetchData(); } catch (e) { toast.error(e instanceof Error ? e.message : 'Failed'); } finally { setSaving(false); } };
  const handleDeleteProvider = async () => { if (!deleteProvider) return; setSaving(true); try { await hlrApi.deleteProvider(deleteProvider.provider_id); toast.success('Deleted'); setDeleteProvider(null); fetchData(); } catch (e) { toast.error(e instanceof Error ? e.message : 'Failed'); } finally { setSaving(false); } };
  const handleCreateWeight = async () => { setSaving(true); try { await hlrApi.setWeights({ ...weightForm, weight: Number(weightForm.weight) }); toast.success('Weight set'); setShowCreateWeight(false); fetchData(); } catch (e) { toast.error(e instanceof Error ? e.message : 'Failed'); } finally { setSaving(false); } };
  const handleDeleteWeight = async () => { if (!deleteWeight) return; setSaving(true); try { await hlrApi.deleteWeight(deleteWeight.weight_id); toast.success('Removed'); setDeleteWeight(null); fetchData(); } catch (e) { toast.error(e instanceof Error ? e.message : 'Failed'); } finally { setSaving(false); } };

  return (
    <>
      <PageHeader title="HLR Providers & Routing Weights" />
      <div className="mb-8">
        <div className="flex items-center justify-between mb-3"><h2 className="text-lg font-semibold">HLR Providers</h2><Button size="sm" onClick={() => { setProviderForm({ name: '', active: true }); setShowCreateProvider(true); }}>Add Provider</Button></div>
        <DataTable columns={providerColumns} data={providers} total={providers.length} page={1} pageSize={100} onPageChange={() => {}} loading={loading} keyField="provider_id" rowActions={(p) => <Button size="sm" variant="ghost" onClick={() => setDeleteProvider(p)}>Delete</Button>} />
      </div>
      <div>
        <div className="flex items-center justify-between mb-3"><h2 className="text-lg font-semibold">Smart Route Weights</h2><Button size="sm" onClick={() => { setWeightForm({ country_code: '', provider_id: '', weight: 100 }); setShowCreateWeight(true); }}>Set Weight</Button></div>
        <DataTable columns={weightColumns} data={weights} total={weights.length} page={1} pageSize={100} onPageChange={() => {}} loading={loading} keyField="weight_id" rowActions={(w) => <Button size="sm" variant="ghost" onClick={() => setDeleteWeight(w)}>Delete</Button>} />
      </div>
      <Modal open={showCreateProvider} onClose={() => setShowCreateProvider(false)} title="Add HLR Provider">
        <div className="space-y-4">
          <Input label="Name" value={providerForm.name} onChange={(e) => setProviderForm({ ...providerForm, name: e.target.value })} required />
          <div className="flex justify-end gap-3 pt-2"><Button variant="secondary" onClick={() => setShowCreateProvider(false)}>Cancel</Button><Button onClick={handleCreateProvider} disabled={saving || !providerForm.name}>{saving ? 'Creating...' : 'Create'}</Button></div>
        </div>
      </Modal>
      <Modal open={showCreateWeight} onClose={() => setShowCreateWeight(false)} title="Set Route Weight">
        <div className="space-y-4">
          <Input label="Country Code" value={weightForm.country_code} onChange={(e) => setWeightForm({ ...weightForm, country_code: e.target.value })} required placeholder="US" />
          <Input label="Provider ID" value={weightForm.provider_id} onChange={(e) => setWeightForm({ ...weightForm, provider_id: e.target.value })} required placeholder="UUID" />
          <Input label="Weight" type="number" value={String(weightForm.weight)} onChange={(e) => setWeightForm({ ...weightForm, weight: Number(e.target.value) })} required />
          <div className="flex justify-end gap-3 pt-2"><Button variant="secondary" onClick={() => setShowCreateWeight(false)}>Cancel</Button><Button onClick={handleCreateWeight} disabled={saving || !weightForm.country_code || !weightForm.provider_id}>{saving ? 'Saving...' : 'Save'}</Button></div>
        </div>
      </Modal>
      <ConfirmDialog open={!!deleteProvider} onConfirm={handleDeleteProvider} onCancel={() => setDeleteProvider(null)} title="Delete HLR Provider" description={`Delete "${deleteProvider?.name}"?`} confirmLabel="Delete" variant="danger" loading={saving} />
      <ConfirmDialog open={!!deleteWeight} onConfirm={handleDeleteWeight} onCancel={() => setDeleteWeight(null)} title="Delete Weight" description="Delete this routing weight?" confirmLabel="Delete" variant="danger" loading={saving} />
    </>
  );
}
