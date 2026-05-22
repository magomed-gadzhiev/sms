import { useState, useEffect, useCallback } from 'react';
import { sraStuckApi, type SRAStuckRow } from '../../api/admin';
import { useToast } from '../../components/ui/Toast';

function formatDate(iso: string | null): string {
  if (!iso) return '—';
  return new Date(iso).toLocaleString('ru-RU');
}

export function SRAStuckPage() {
  const toast = useToast();
  const [rows, setRows] = useState<SRAStuckRow[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [resetting, setResetting] = useState<string | null>(null);

  const fetchRows = useCallback(async () => {
    try {
      const res = await sraStuckApi.list();
      setRows(res.rows);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка загрузки');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void fetchRows();
  }, [fetchRows]);

  const handleReset = useCallback(
    async (row: SRAStuckRow) => {
      const confirmed = window.confirm(
        `Сбросить retry-state для ${row.client_email}? Убедись что root-cause устранён.`,
      );
      if (!confirmed) return;

      setResetting(row.client_id);
      try {
        await sraStuckApi.reset(row.client_id);
        toast.success(`Retry-state сброшен для ${row.client_email}`);
        setLoading(true);
        await fetchRows();
      } catch (err) {
        toast.error(err instanceof Error ? err.message : 'Не удалось сбросить retry-state');
      } finally {
        setResetting(null);
      }
    },
    [fetchRows, toast],
  );

  return (
    <div className="p-6">
      <h1 className="text-xl font-semibold text-gray-900 dark:text-slate-100 mb-1">Залипшие SRA-rows (cap=100)</h1>
      <p className="text-sm text-gray-500 dark:text-slate-400 mb-6">
        Subaccount-routing-assignment, у которых retry_count достиг cap. Ops должен исследовать
        root-cause и нажать Reset.
      </p>

      {loading && <p className="text-gray-500 dark:text-slate-400">Загрузка...</p>}

      {!loading && error && (
        <div className="rounded-md bg-red-50 dark:bg-red-950/40 border border-red-200 dark:border-red-800 p-4 text-red-700 dark:text-red-300 text-sm">
          {error}
        </div>
      )}

      {!loading && !error && rows !== null && rows.length === 0 && (
        <p className="text-gray-500 dark:text-slate-400 text-sm">Нет залипших row.</p>
      )}

      {!loading && !error && rows !== null && rows.length > 0 && (
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200 dark:divide-slate-700 text-sm">
            <thead className="bg-gray-50 dark:bg-slate-950">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-slate-400 uppercase tracking-wider">
                  Client
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-slate-400 uppercase tracking-wider">
                  Provider Set
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-slate-400 uppercase tracking-wider">
                  Route Set
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-slate-400 uppercase tracking-wider">
                  Retry Count
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-slate-400 uppercase tracking-wider">
                  Last Error At
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-slate-400 uppercase tracking-wider">
                  Last Error Text
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-slate-400 uppercase tracking-wider">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="bg-white dark:bg-slate-900 divide-y divide-gray-200 dark:divide-slate-700">
              {rows.map((row) => (
                <tr key={row.client_id} className="hover:bg-gray-50 dark:hover:bg-slate-800">
                  <td className="px-4 py-3">
                    <div className="font-medium text-gray-900 dark:text-slate-100">{row.client_email}</div>
                    <div className="text-xs text-gray-400 dark:text-slate-500">{row.client_id}</div>
                  </td>
                  <td className="px-4 py-3 text-gray-700 dark:text-slate-300">{row.provider_set_id ?? '—'}</td>
                  <td className="px-4 py-3 text-gray-700 dark:text-slate-300">{row.route_set_id ?? '—'}</td>
                  <td className="px-4 py-3 font-mono text-gray-900 dark:text-slate-100">
                    {row.materialize_retry_count}
                  </td>
                  <td className="px-4 py-3 text-gray-700 dark:text-slate-300 whitespace-nowrap">
                    {formatDate(row.last_materialize_error_at)}
                  </td>
                  <td className="px-4 py-3 text-gray-700 dark:text-slate-300 max-w-xs">
                    <span
                      className="block truncate"
                      title={row.last_materialize_error_text ?? undefined}
                    >
                      {row.last_materialize_error_text ?? '—'}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <button
                      onClick={() => void handleReset(row)}
                      disabled={resetting === row.client_id}
                      className="inline-flex items-center px-3 py-1 border border-red-300 text-xs font-medium rounded text-red-700 dark:text-red-300 bg-white dark:bg-slate-900 hover:bg-red-50 disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                      {resetting === row.client_id ? '...' : 'Reset'}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
