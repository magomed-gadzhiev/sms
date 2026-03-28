import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { FilterBar, type FilterDef } from '../../components/data/FilterBar';
import { useToast } from '../../components/ui/Toast';
import { auditAdminApi, type AuditEntry } from '../../api/admin';

const PAGE_SIZE = 30;

const filters: FilterDef[] = [
  { key: 'user_id', label: 'User ID', type: 'text', placeholder: 'UUID...' },
  { key: 'action', label: 'Action', type: 'select', options: [{ value: 'login', label: 'Login' }, { value: 'logout', label: 'Logout' }, { value: 'create', label: 'Create' }, { value: 'update', label: 'Update' }, { value: 'delete', label: 'Delete' }] },
  { key: 'resource_type', label: 'Resource Type', type: 'select', options: [
    { value: 'client', label: 'Client' },
    { value: 'provider', label: 'Provider' },
    { value: 'template', label: 'Template' },
    { value: 'user', label: 'User' },
    { value: 'billing', label: 'Billing' },
    { value: 'route', label: 'Route' },
  ] },
  { key: 'from', label: 'From', type: 'date' },
  { key: 'to', label: 'To', type: 'date' },
];

const columns: Column<AuditEntry>[] = [
  { key: 'created_at', header: 'Timestamp', render: (e) => new Date(e.created_at).toLocaleString(), sortable: true },
  { key: 'action', header: 'Action' },
  { key: 'resource_type', header: 'Resource' },
  { key: 'resource_id', header: 'Resource ID', render: (e) => e.resource_id ? e.resource_id.slice(0, 8) + '...' : '-' },
  { key: 'user_id', header: 'User', render: (e) => e.user_id.slice(0, 8) + '...' },
  { key: 'ip_address', header: 'IP' },
  { key: 'details', header: 'Details', render: (e) => {
    if (!e.details || Object.keys(e.details).length === 0) return '-';
    return <span className="text-xs text-gray-500 truncate max-w-[150px] inline-block">{JSON.stringify(e.details)}</span>;
  }},
];

export function AuditLogPage() {
  const toast = useToast();
  const [data, setData] = useState<AuditEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [filterValues, setFilterValues] = useState<Record<string, string>>({});

  const fetchData = useCallback(async () => {
    setLoading(true);
    try { const res = await auditAdminApi.list({ ...filterValues, limit: PAGE_SIZE, offset: (page - 1) * PAGE_SIZE }); setData(res.entries || []); setTotal(res.total); }
    catch { toast.error('Failed to load audit log'); }
    finally { setLoading(false); }
  }, [page, filterValues, toast]);

  useEffect(() => { fetchData(); }, [fetchData]);

  return (
    <>
      <PageHeader title="Audit Log" subtitle={`${total} entries`} breadcrumbs={[{ label: 'Админ', href: '/admin/dashboard' }, { label: 'Аудит' }]} />
      <FilterBar filters={filters} values={filterValues} onChange={(v) => { setFilterValues(v); setPage(1); }} onReset={() => { setFilterValues({}); setPage(1); }} />
      <DataTable columns={columns} data={data} total={total} page={page} pageSize={PAGE_SIZE} onPageChange={setPage} loading={loading} keyField="id" />
    </>
  );
}
