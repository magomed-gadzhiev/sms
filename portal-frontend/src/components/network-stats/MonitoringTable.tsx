import { ChevronsUpDown, Activity } from 'lucide-react';
import type { MonitorRow, SharedFilter, Pagination } from '../../api/networkStats';

interface MonitoringTableProps {
  rows: MonitorRow[];
  pagination?: Pagination;
  filters: SharedFilter;
  onFiltersChange: (partial: Partial<SharedFilter>) => void;
  onApply: () => void;
  onToggleSort: (key: string) => void;
  onRowClick: (row: MonitorRow) => void;
  loading: boolean;
}

function fmt(n: number): string { return (n ?? 0).toLocaleString('ru-RU'); }
function fmtLatency(ms: number): string { return `${((ms ?? 0) / 1000).toFixed(1)}с`; }
function fmtPct(n: number): string { return `${((n ?? 0) * 100).toFixed(1)}%`; }

function latencyColor(ms: number): string {
  if (ms > 30000) return 'text-red-600 font-medium';
  if (ms > 10000) return 'text-amber-600';
  return '';
}

function dlrColor(rate: number): string {
  if (rate < 0.80) return 'text-red-600 font-medium';
  if (rate < 0.90) return 'text-amber-600';
  return 'text-emerald-600';
}

function healthDot(h: string): string {
  if (h === 'danger') return 'bg-red-600';
  if (h === 'warning') return 'bg-amber-500';
  return 'bg-emerald-600';
}

function rowBg(row: MonitorRow): string {
  return row.health === 'danger' ? 'bg-red-50 hover:bg-red-100' : 'hover:bg-gray-50';
}

const COLUMNS: { key: string; header: string; align: string; render: (r: MonitorRow) => React.ReactNode }[] = [
  { key: 'slice', header: 'Провайдер', align: 'left', render: r => { const s = r.slice || ''; const display = /^\d{4}-\d{2}-\d{2}/.test(s) ? s.slice(0, 10) : s; return <span className="font-medium text-blue-600">{display}</span>; } },
  { key: 'throughput', header: 'msg/s', align: 'right', render: r => (r.throughput ?? 0).toFixed(0) },
  { key: 'sent', header: 'Отпр.', align: 'right', render: r => fmt(r.sent) },
  { key: 'delivered', header: 'Достав.', align: 'right', render: r => fmt(r.delivered) },
  { key: 'pending', header: 'Ожидание', align: 'right', render: r => r.pending > 0 ? <span className={r.pending > 100 ? 'text-amber-600 font-medium' : ''}>{fmt(r.pending)}</span> : '0' },
  { key: 'timeout', header: 'Таймаут', align: 'right', render: r => r.timeout > 0 ? <span className="text-amber-600">{fmt(r.timeout)}</span> : '0' },
  { key: 'error', header: 'Ошибки', align: 'right', render: r => r.error > 0 ? <span className="text-red-600 font-medium">{fmt(r.error)}</span> : '0' },
  { key: 'dlr_latency_p50', header: 'p50', align: 'right', render: r => fmtLatency(r.dlr_latency_p50) },
  { key: 'dlr_latency_p95', header: 'p95', align: 'right', render: r => <span className={latencyColor(r.dlr_latency_p95)}>{fmtLatency(r.dlr_latency_p95)}</span> },
  { key: 'dlr_rate', header: 'DLR%', align: 'right', render: r => <span className={dlrColor(r.dlr_rate)}>{fmtPct(r.dlr_rate)}</span> },
  { key: 'top_error', header: 'Осн. ошибка', align: 'left', render: r => r.top_error ? <span className="font-mono text-[11px] text-red-600">{r.top_error}</span> : <span className="text-gray-300">—</span> },
  { key: 'health', header: '', align: 'center', render: r => <span className={`inline-block w-2 h-2 rounded-full ${healthDot(r.health)}`} /> },
];

export function MonitoringTable({ rows, pagination, filters, onFiltersChange, onApply, onToggleSort, onRowClick, loading }: MonitoringTableProps) {
  // D-20 fix: toggle logic delegated to useNetworkStats.toggleSort.
  void filters; void onFiltersChange; void onApply;

  function handlePageChange(page: number) {
    onFiltersChange({ page });
    onApply();
  }

  return (
    <div className="px-4 pb-4">
      <div className="overflow-x-auto rounded-lg border border-gray-200 bg-white">
        <table className="w-full text-[13px]">
          <thead>
            <tr className="bg-gray-50 border-b-2 border-gray-200">
              {COLUMNS.map(col => (
                <th
                  key={col.key}
                  className={`px-3 py-2.5 font-semibold text-gray-500 text-xs whitespace-nowrap cursor-pointer select-none ${col.align === 'right' ? 'text-right' : col.align === 'center' ? 'text-center' : 'text-left'}`}
                  onClick={() => col.key !== 'health' && onToggleSort(col.key)}
                >
                  <span className="inline-flex items-center gap-0.5">
                    {col.header}
                    {col.key === 'health' ? <Activity size={12} className="text-gray-400" /> : <ChevronsUpDown size={10} className="text-gray-400" />}
                  </span>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {loading && !rows.length ? (
              <tr><td colSpan={COLUMNS.length} className="text-center py-12 text-gray-400">Загрузка...</td></tr>
            ) : !rows.length ? (
              <tr><td colSpan={COLUMNS.length} className="text-center py-12 text-gray-400">Нет данных</td></tr>
            ) : (
              rows.map((row, i) => (
                <tr
                  key={row.slice + i}
                  className={`border-b border-gray-100 cursor-pointer transition-colors ${rowBg(row)}`}
                  onClick={() => onRowClick(row)}
                >
                  {COLUMNS.map(col => (
                    <td key={col.key} className={`px-3 py-2.5 ${col.align === 'right' ? 'text-right' : col.align === 'center' ? 'text-center' : ''}`}>
                      {col.render(row)}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {pagination && pagination.total_pages > 1 && (
        <div className="flex items-center justify-center gap-2 mt-3">
          <button disabled={pagination.page <= 1} onClick={() => handlePageChange(pagination.page - 1)} className="px-3 py-1 rounded border border-gray-200 text-xs text-gray-500 disabled:opacity-40">←</button>
          <span className="text-xs text-gray-500">Страница {pagination.page} из {pagination.total_pages}</span>
          <button disabled={pagination.page >= pagination.total_pages} onClick={() => handlePageChange(pagination.page + 1)} className="px-3 py-1 rounded border border-gray-200 text-xs text-gray-500 disabled:opacity-40">→</button>
        </div>
      )}
    </div>
  );
}
