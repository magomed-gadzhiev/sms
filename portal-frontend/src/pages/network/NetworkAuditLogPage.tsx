import { Fragment, useEffect, useState } from 'react';
import {
  auditApi,
  ApiError,
  type NetworkAuditLogEntry,
} from '../../api/client';
import { useToast } from '../../components/ui/Toast';
import { PageHeader } from '../../components/layout/PageHeader';

const RESOURCE_TYPES: Array<{ value: string; label: string }> = [
  { value: '', label: 'Все типы' },
  { value: 'route_set', label: 'Route-set' },
  { value: 'route_set_item', label: 'Route-set item' },
  { value: 'provider_set', label: 'Provider-set' },
  { value: 'assignment', label: 'Назначение' },
  { value: 'route_override', label: 'Route override' },
  { value: 'provider_override', label: 'Provider override' },
];

const ACTION_LABELS: Record<string, string> = {
  create: 'Создание',
  update: 'Обновление',
  delete: 'Удаление',
  reorder: 'Переупорядочивание',
  bulk: 'Bulk-операция',
};

const fmtDate = (iso: string): string =>
  iso ? new Date(iso).toLocaleString('ru-RU') : '—';

const prettifyDetails = (details: string): string => {
  if (!details || details === 'null') return '{}';
  try {
    return JSON.stringify(JSON.parse(details), null, 2);
  } catch {
    return details;
  }
};

export function NetworkAuditLogPage() {
  const toast = useToast();
  const [entries, setEntries] = useState<NetworkAuditLogEntry[]>([]);
  const [resourceType, setResourceType] = useState<string>('');
  const [dateFrom, setDateFrom] = useState<string>('');
  const [dateTo, setDateTo] = useState<string>('');
  const [page, setPage] = useState<number>(1);
  const [totalPages, setTotalPages] = useState<number>(0);
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [loading, setLoading] = useState<boolean>(false);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    auditApi
      .getNetworkLog({
        resource_type: resourceType || undefined,
        date_from: dateFrom || undefined,
        date_to: dateTo || undefined,
        page,
        per_page: 20,
      })
      .then((r) => {
        if (cancelled) return;
        setEntries(r.entries || []);
        setTotalPages(r.total_pages || 0);
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        const msg = e instanceof ApiError ? e.message : 'Ошибка загрузки журнала';
        toast.error(msg);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [resourceType, dateFrom, dateTo, page, toast]);

  return (
    <div className="max-w-6xl">
      <PageHeader
        title="История изменений сети"
        subtitle="Аудит изменений route-set, provider-set, назначений и переопределений"
      />

      <div className="flex flex-wrap items-end gap-3 mb-4">
        <div>
          <label
            htmlFor="resource-type-filter"
            className="block text-xs text-gray-600 mb-1"
          >
            Тип ресурса
          </label>
          <select
            id="resource-type-filter"
            value={resourceType}
            onChange={(e) => {
              setResourceType(e.target.value);
              setPage(1);
            }}
            className="border rounded px-3 py-2"
          >
            {RESOURCE_TYPES.map((rt) => (
              <option key={rt.value} value={rt.value}>
                {rt.label}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label
            htmlFor="date-from-filter"
            className="block text-xs text-gray-600 mb-1"
          >
            С даты
          </label>
          <input
            id="date-from-filter"
            type="date"
            value={dateFrom}
            onChange={(e) => {
              setDateFrom(e.target.value);
              setPage(1);
            }}
            className="border rounded px-3 py-2"
          />
        </div>
        <div>
          <label
            htmlFor="date-to-filter"
            className="block text-xs text-gray-600 mb-1"
          >
            По дату
          </label>
          <input
            id="date-to-filter"
            type="date"
            value={dateTo}
            onChange={(e) => {
              setDateTo(e.target.value);
              setPage(1);
            }}
            className="border rounded px-3 py-2"
          />
        </div>
      </div>

      <div className="bg-white border rounded-lg overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-gray-50 text-gray-700">
            <tr>
              <th className="text-left p-3">Когда</th>
              <th className="text-left p-3">Тип</th>
              <th className="text-left p-3">Действие</th>
              <th className="text-left p-3">ID ресурса</th>
              <th className="text-left p-3">Пользователь</th>
              <th className="text-left p-3">IP</th>
              <th className="text-left p-3 w-12" aria-label="Раскрыть"></th>
            </tr>
          </thead>
          <tbody>
            {loading && (
              <tr>
                <td colSpan={7} className="p-3 text-center text-gray-500">
                  Загрузка...
                </td>
              </tr>
            )}
            {!loading && entries.length === 0 && (
              <tr>
                <td colSpan={7} className="p-3 text-center text-gray-500">
                  Нет записей
                </td>
              </tr>
            )}
            {!loading &&
              entries.map((e) => (
                <Fragment key={e.id}>
                  <tr className="border-t">
                    <td className="p-3 whitespace-nowrap">
                      {fmtDate(e.created_at)}
                    </td>
                    <td className="p-3">{e.resource_type}</td>
                    <td className="p-3">
                      {ACTION_LABELS[e.action] || e.action}
                    </td>
                    <td className="p-3 font-mono text-xs">
                      {e.resource_id || '—'}
                    </td>
                    <td className="p-3 font-mono text-xs">
                      {e.user_id ? e.user_id.slice(0, 8) : '—'}
                    </td>
                    <td className="p-3 font-mono text-xs">
                      {e.ip_address || '—'}
                    </td>
                    <td className="p-3">
                      <button
                        type="button"
                        onClick={() =>
                          setExpanded((prev) => ({
                            ...prev,
                            [e.id]: !prev[e.id],
                          }))
                        }
                        className="text-blue-600 hover:underline"
                        aria-label={
                          expanded[e.id]
                            ? 'Скрыть детали'
                            : 'Раскрыть детали'
                        }
                        aria-expanded={!!expanded[e.id]}
                      >
                        {expanded[e.id] ? '▲' : '▼'}
                      </button>
                    </td>
                  </tr>
                  {expanded[e.id] && (
                    <tr className="bg-gray-50">
                      <td colSpan={7} className="p-3">
                        <pre className="text-xs whitespace-pre-wrap break-all">
                          {prettifyDetails(e.details)}
                        </pre>
                      </td>
                    </tr>
                  )}
                </Fragment>
              ))}
          </tbody>
        </table>
      </div>

      {totalPages > 1 && (
        <div className="flex justify-center items-center gap-2 mt-4">
          <button
            type="button"
            disabled={page <= 1}
            onClick={() => setPage((p) => p - 1)}
            className="px-3 py-1 border rounded disabled:opacity-50"
            aria-label="Предыдущая страница"
          >
            ←
          </button>
          <span className="px-3 py-1 text-sm">
            {page} / {totalPages}
          </span>
          <button
            type="button"
            disabled={page >= totalPages}
            onClick={() => setPage((p) => p + 1)}
            className="px-3 py-1 border rounded disabled:opacity-50"
            aria-label="Следующая страница"
          >
            →
          </button>
        </div>
      )}
    </div>
  );
}
