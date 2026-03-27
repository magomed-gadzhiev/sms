import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { FilterBar, type FilterDef } from '../../components/data/FilterBar';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Modal } from '../../components/ui/Modal';
import { StatusBadge } from '../../components/ui/Badge';
import { useToast } from '../../components/ui/Toast';
import { templatesApi, type TemplateInfo } from '../../api/admin';

const PAGE_SIZE = 20;

const filters: FilterDef[] = [
  { key: 'client_id', label: 'Client ID', type: 'text', placeholder: 'UUID...' },
  { key: 'status', label: 'Status', type: 'select', options: [{ value: 'pending', label: 'Pending' }, { value: 'approved', label: 'Approved' }, { value: 'rejected', label: 'Rejected' }] },
];

const columns: Column<TemplateInfo>[] = [
  { key: 'name', header: 'Name', sortable: true },
  { key: 'client_id', header: 'Client', render: (t) => t.client_id.slice(0, 8) + '...' },
  { key: 'body', header: 'Body', render: (t) => <span className="truncate max-w-[200px] inline-block">{t.body}</span> },
  { key: 'status', header: 'Status', render: (t) => <StatusBadge status={t.status} /> },
  { key: 'created_at', header: 'Created', render: (t) => new Date(t.created_at).toLocaleDateString() },
];

export function TemplatesPage() {
  const toast = useToast();
  const [data, setData] = useState<TemplateInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [filterValues, setFilterValues] = useState<Record<string, string>>({});
  const [rejectModal, setRejectModal] = useState<TemplateInfo | null>(null);
  const [rejectReason, setRejectReason] = useState('');
  const [saving, setSaving] = useState(false);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try { const res = await templatesApi.list({ ...filterValues, limit: PAGE_SIZE, offset: (page - 1) * PAGE_SIZE }); setData(res.templates || []); setTotal(res.total); }
    catch { toast.error('Failed to load templates'); }
    finally { setLoading(false); }
  }, [page, filterValues, toast]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const handleApprove = async (template: TemplateInfo) => {
    try { await templatesApi.approve(template.template_id); toast.success('Template approved'); fetchData(); }
    catch (e) { toast.error(e instanceof Error ? e.message : 'Failed'); }
  };

  const handleReject = async () => {
    if (!rejectModal) return;
    setSaving(true);
    try { await templatesApi.reject(rejectModal.template_id, { reason: rejectReason }); toast.success('Template rejected'); setRejectModal(null); setRejectReason(''); fetchData(); }
    catch (e) { toast.error(e instanceof Error ? e.message : 'Failed'); }
    finally { setSaving(false); }
  };

  return (
    <>
      <PageHeader title="Templates" subtitle={`${total} templates`} />
      <FilterBar filters={filters} values={filterValues} onChange={(v) => { setFilterValues(v); setPage(1); }} onReset={() => { setFilterValues({}); setPage(1); }} />
      <DataTable columns={columns} data={data} total={total} page={page} pageSize={PAGE_SIZE} onPageChange={setPage} loading={loading} keyField="template_id"
        rowActions={(t) => t.status === 'pending' ? (<div className="flex gap-1"><Button size="sm" variant="primary" onClick={() => handleApprove(t)}>Approve</Button><Button size="sm" variant="danger" onClick={() => setRejectModal(t)}>Reject</Button></div>) : null}
      />
      <Modal open={!!rejectModal} onClose={() => setRejectModal(null)} title="Reject Template">
        <div className="space-y-4">
          <p className="text-sm text-gray-600">Template: {rejectModal?.name}</p>
          <Input label="Reason" value={rejectReason} onChange={(e) => setRejectReason(e.target.value)} placeholder="Reason for rejection..." />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setRejectModal(null)}>Cancel</Button>
            <Button variant="danger" onClick={handleReject} disabled={saving}>{saving ? 'Rejecting...' : 'Reject'}</Button>
          </div>
        </div>
      </Modal>
    </>
  );
}
