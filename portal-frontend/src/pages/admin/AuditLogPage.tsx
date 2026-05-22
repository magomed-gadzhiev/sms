import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { FilterBar, type FilterDef } from '../../components/data/FilterBar';
import { useToast } from '../../components/ui/Toast';
import { auditAdminApi, clientsApi, type AuditEntry, type ClientInfo } from '../../api/admin';

const PAGE_SIZE = 30;

const columns: Column<AuditEntry>[] = [
  { key: 'created_at', header: 'Timestamp', render: (e) => new Date(e.created_at).toLocaleString(), sortable: true },
  { key: 'action', header: 'Action' },
  { key: 'resource_type', header: 'Resource' },
  { key: 'resource_id', header: 'Resource ID', render: (e) => e.resource_id ? e.resource_id.slice(0, 8) + '...' : '-' },
  { key: 'user_id', header: 'User', render: (e) => e.user_id.slice(0, 8) + '...' },
  { key: 'ip_address', header: 'IP' },
  { key: 'details', header: 'Details', render: (e) => {
    if (!e.details || Object.keys(e.details).length === 0) return '-';
    return <span className="text-xs text-gray-500 dark:text-slate-400 truncate max-w-[150px] inline-block">{JSON.stringify(e.details)}</span>;
  }},
];

export function AuditLogPage() {
  const toast = useToast();
  const [data, setData] = useState<AuditEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [filterValues, setFilterValues] = useState<Record<string, string>>({});
  const [clients, setClients] = useState<ClientInfo[]>([]);

  // BUG-55/57: backend gRPC AuditService.QueryAuditLog требует tenant_id —
  // admin global view не поддерживается. Грузим список клиентов в селект и
  // запускаем fetch только когда client_id выбран.
  useEffect(() => {
    clientsApi.list({ limit: 500 }).then((res) => setClients(res.clients || [])).catch(() => {});
  }, []);

  const filters: FilterDef[] = [
    {
      key: 'client_id',
      label: 'Клиент',
      type: 'select',
      options: clients.map((c) => ({ value: c.client_id, label: c.name })),
      placeholder: 'Выберите клиента',
    },
    { key: 'user_id', label: 'User ID', type: 'text', placeholder: 'UUID...' },
    { key: 'action', label: 'Action', type: 'select', options: [
      { value: 'login', label: 'Login' }, { value: 'logout', label: 'Logout' },
      { value: 'create', label: 'Create' }, { value: 'update', label: 'Update' },
      { value: 'delete', label: 'Delete' },
    ] },
    { key: 'resource_type', label: 'Resource Type', type: 'select', options: [
      { value: 'client', label: 'Client' }, { value: 'provider', label: 'Provider' },
      { value: 'template', label: 'Template' }, { value: 'user', label: 'User' },
      { value: 'billing', label: 'Billing' }, { value: 'route', label: 'Route' },
    ] },
    { key: 'date_from', label: 'From', type: 'date' },
    { key: 'date_to', label: 'To', type: 'date' },
  ];

  const fetchData = useCallback(async () => {
    if (!filterValues.client_id) {
      setData([]);
      setTotal(0);
      return;
    }
    setLoading(true);
    try {
      const res = await auditAdminApi.list({
        client_id: filterValues.client_id,
        user_id: filterValues.user_id || undefined,
        action: filterValues.action || undefined,
        resource_type: filterValues.resource_type || undefined,
        date_from: filterValues.date_from || undefined,
        date_to: filterValues.date_to || undefined,
        page,
        per_page: PAGE_SIZE,
      });
      setData(res.entries || []);
      setTotal(res.total);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Не удалось загрузить аудит-лог');
    } finally {
      setLoading(false);
    }
  }, [page, filterValues, toast]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const noClient = !filterValues.client_id;

  return (
    <>
      <PageHeader title="Audit Log" subtitle={noClient ? 'Выберите клиента' : `${total} entries`} breadcrumbs={[{ label: 'Админ', href: '/admin/dashboard' }, { label: 'Аудит' }]} />
      <FilterBar filters={filters} values={filterValues} onChange={(v) => { setFilterValues(v); setPage(1); }} onReset={() => { setFilterValues({}); setPage(1); }} />
      <DataTable columns={columns} data={data} total={total} page={page} pageSize={PAGE_SIZE} onPageChange={setPage} loading={loading} keyField="id"
        emptyMessage={
          <div className="text-center py-12 text-gray-500 dark:text-slate-400">
            {noClient ? (
              <>
                <p className="text-lg font-medium">Выберите клиента</p>
                <p className="text-sm mt-1">Аудит-лог фильтруется по клиенту — без выбранного client_id просмотр не поддерживается.</p>
              </>
            ) : (
              <>
                <p className="text-lg font-medium">Нет записей</p>
                <p className="text-sm mt-1">Попробуйте изменить фильтры или выбрать другой период.</p>
              </>
            )}
          </div>
        }
      />
    </>
  );
}
