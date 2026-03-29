import { useEffect, useState, useCallback } from 'react';
import { auditApi } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { FilterBar, type FilterDef } from '../../components/data/FilterBar';
import { DataTable, type Column } from '../../components/data/DataTable';

interface AuditEntry {
  id: string;
  tenant_id: string;
  user_id: string;
  action: string;
  resource_type: string;
  resource_id: string;
  details: string;
  ip_address: string;
  created_at?: string;
}

interface AuditLogResponse {
  entries: AuditEntry[];
  total: number;
  page: number;
  total_pages: number;
}

const ACTION_OPTIONS = [
  { value: '', label: 'Все действия' },
  { value: 'auth.login', label: 'Вход' },
  { value: 'auth.logout', label: 'Выход' },
  { value: 'auth.password_reset', label: 'Сброс пароля' },
  { value: 'auth.totp_enabled', label: '2FA включена' },
  { value: 'auth.totp_disabled', label: '2FA отключена' },
  { value: 'api_key.created', label: 'API ключ создан' },
  { value: 'api_key.revoked', label: 'API ключ отозван' },
  { value: 'webhook.created', label: 'Вебхук создан' },
  { value: 'webhook.updated', label: 'Вебхук обновлён' },
  { value: 'webhook.deleted', label: 'Вебхук удалён' },
  { value: 'webhook.test_sent', label: 'Тест вебхука' },
  { value: 'sub_account.created', label: 'Суб-аккаунт создан' },
  { value: 'sub_account.deleted', label: 'Суб-аккаунт удалён' },
  { value: 'sub_account.limit_updated', label: 'Лимит суб-аккаунта изменён' },
  { value: 'balance.transfer_out', label: 'Перевод средств (исходящий)' },
  { value: 'balance.transfer_in', label: 'Перевод средств (входящий)' },
  { value: 'profile.updated', label: 'Профиль обновлён' },
];

const AUDIT_FILTERS: FilterDef[] = [
  { key: 'action', label: 'Действие', type: 'select', options: ACTION_OPTIONS.map(o => ({ value: o.value, label: o.label })) },
  { key: 'date_from', label: 'С даты', type: 'date' },
  { key: 'date_to', label: 'По дату', type: 'date' },
  { key: 'user_id', label: 'ID пользователя', type: 'text', placeholder: 'Фильтр по пользователю...' },
];

const INITIAL_FILTERS: Record<string, string> = { action: '', date_from: '', date_to: '', user_id: '' };

const formatDetails = (details: string): string => {
  if (!details || details === '{}' || details === 'null') return '-';
  try {
    const parsed = JSON.parse(details);
    return Object.entries(parsed)
      .map(([k, v]) => `${k}: ${v}`)
      .join(', ');
  } catch {
    return details;
  }
};

const columns: Column<AuditEntry>[] = [
  { key: 'created_at', header: 'Дата и время', render: (entry) => <span className="text-xs whitespace-nowrap">{entry.created_at ? new Date(entry.created_at).toLocaleString() : '-'}</span> },
  { key: 'action', header: 'Действие' },
  { key: 'resource_type', header: 'Тип ресурса' },
  { key: 'resource_id', header: 'ID ресурса', render: (entry) => <span className="text-xs font-mono">{entry.resource_id ? `${entry.resource_id.substring(0, 8)}...` : '-'}</span> },
  { key: 'user_id', header: 'ID пользователя', render: (entry) => <span className="text-xs font-mono">{entry.user_id ? `${entry.user_id.substring(0, 8)}...` : '-'}</span> },
  { key: 'ip_address', header: 'IP адрес', render: (entry) => <span className="text-xs">{entry.ip_address || '-'}</span> },
  { key: 'details', header: 'Детали', render: (entry) => <span className="text-xs block max-w-[250px] truncate">{formatDetails(entry.details)}</span> },
];

export function AuditLogPage() {
  const [data, setData] = useState<AuditLogResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [filterValues, setFilterValues] = useState<Record<string, string>>(INITIAL_FILTERS);

  const fetchAuditLog = useCallback(() => {
    setLoading(true);
    setError(null);

    const params: Record<string, string> = {
      page: String(page),
      per_page: '20',
    };
    if (filterValues.action) params.action = filterValues.action;
    if (filterValues.user_id) params.user_id = filterValues.user_id;
    if (filterValues.date_from) params.date_from = filterValues.date_from;
    if (filterValues.date_to) params.date_to = filterValues.date_to;

    auditApi
      .list(params)
      .then((resp) => setData(resp as AuditLogResponse))
      .catch((err) => setError(err.message || 'Не удалось загрузить журнал аудита'))
      .finally(() => setLoading(false));
  }, [page, filterValues]);

  useEffect(() => {
    fetchAuditLog();
  }, [fetchAuditLog]);

  const handleFilterChange = (values: Record<string, string>) => {
    setFilterValues(values);
    setPage(1);
  };

  const handleFilterReset = () => {
    setFilterValues(INITIAL_FILTERS);
    setPage(1);
  };

  return (
    <div>
      <PageHeader title="Журнал аудита" />

      <FilterBar
        filters={AUDIT_FILTERS}
        values={filterValues}
        onChange={handleFilterChange}
        onReset={handleFilterReset}
      />

      {error && <div className="text-red-600 mb-3">Ошибка: {error}</div>}

      {data && (
        <DataTable<AuditEntry>
          columns={columns}
          data={data.entries}
          total={data.total}
          page={page}
          pageSize={20}
          onPageChange={setPage}
          keyField="id"
          loading={loading}
        />
      )}

      {!data && loading && (
        <DataTable<AuditEntry>
          columns={columns}
          data={[]}
          total={0}
          page={1}
          pageSize={20}
          onPageChange={setPage}
          keyField="id"
          loading={true}
        />
      )}
    </div>
  );
}
