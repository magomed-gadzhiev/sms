import { ChevronsUpDown } from 'lucide-react';
import type { StatRow, SharedFilter, Pagination } from '../../api/networkStats';

interface StatisticsTableProps {
  rows: StatRow[];
  pagination?: Pagination;
  filters: SharedFilter;
  onFiltersChange: (partial: Partial<SharedFilter>) => void;
  onApply: () => void;
  onRowClick: (row: StatRow) => void;
  loading: boolean;
}

function fmt(n: number): string { return (n ?? 0).toLocaleString('ru-RU'); }
function fmtMoney(n: number): string { return `${(n ?? 0).toLocaleString('ru-RU')} ₽`; }
function fmtPct(n: number): string { return `${((n ?? 0) * 100).toFixed(1)}%`; }

function dlrColor(rate: number): string {
  if (rate < 0.80) return 'text-red-600 font-medium';
  if (rate < 0.90) return 'text-amber-600';
  return 'text-emerald-600';
}

function profitColor(v: number): string {
  return v < 0 ? 'text-red-600 font-medium' : 'text-emerald-600';
}

function healthDot(h: string): string {
  if (h === 'danger') return 'bg-red-600';
  if (h === 'warning') return 'bg-amber-500';
  return 'bg-emerald-600';
}

function rowBg(row: StatRow): string {
  return row.health === 'danger' ? 'bg-red-50 hover:bg-red-100' : 'hover:bg-gray-50';
}

const COLUMNS: { key: string; header: string; align: string; render: (r: StatRow) => React.ReactNode }[] = [
  { key: 'slice', header: 'Срез', align: 'left', render: r => <span className="font-medium text-blue-600">{r.slice}</span> },
  { key: 'total', header: 'Всего', align: 'right', render: r => fmt(r.total) },
  { key: 'delivered', header: 'Достав.', align: 'right', render: r => <span className="text-emerald-600">{fmt(r.delivered)}</span> },
  { key: 'failed', header: 'Не достав.', align: 'right', render: r => fmt(r.failed) },
  { key: 'pending', header: 'Ожидание', align: 'right', render: r => r.pending > 0 ? <span className={r.pending > 100 ? 'text-amber-600' : ''}>{fmt(r.pending)}</span> : '0' },
  { key: 'timeout', header: 'Таймаут', align: 'right', render: r => fmt(r.timeout) },
  { key: 'error', header: 'Ошибки', align: 'right', render: r => r.error > 0 ? <span className={r.error > 500 ? 'text-red-600' : 'text-amber-600'}>{fmt(r.error)}</span> : '0' },
  { key: 'dlr_rate', header: 'DLR%', align: 'right', render: r => <span className={dlrColor(r.dlr_rate)}>{fmtPct(r.dlr_rate)}</span> },
  { key: 'revenue', header: 'Выручка', align: 'right', render: r => fmtMoney(r.revenue) },
  { key: 'profit', header: 'Прибыль', align: 'right', render: r => <span className={profitColor(r.profit)}>{fmtMoney(r.profit)}</span> },
  { key: 'margin', header: 'Маржа', align: 'right', render: r => <span className={r.margin < 0 ? 'text-red-600' : ''}>{fmtPct(r.margin)}</span> },
  { key: 'health', header: '', align: 'center', render: r => <span className={`inline-block w-2 h-2 rounded-full ${healthDot(r.health)}`} /> },
];

export function StatisticsTable({ rows, pagination, filters, onFiltersChange, onApply, onRowClick, loading }: StatisticsTableProps) {
  function handleSort(key: string) {
    const newDir = filters.sort_by === key && filters.sort_dir === 'desc' ? 'asc' : 'desc';
    onFiltersChange({ sort_by: key, sort_dir: newDir });
    onApply();
  }

  function handlePageChange(page: number) {
    onFiltersChange({ page });
    onApply();
  }

  // Compute totals
  const totals = rows.reduce(
    (acc, r) => ({
      total: acc.total + (r.total ?? 0),
      delivered: acc.delivered + (r.delivered ?? 0),
      failed: acc.failed + (r.failed ?? 0),
      pending: acc.pending + (r.pending ?? 0),
      timeout: acc.timeout + (r.timeout ?? 0),
      error: acc.error + (r.error ?? 0),
      revenue: acc.revenue + (r.revenue ?? 0),
      profit: acc.profit + (r.profit ?? 0),
    }),
    { total: 0, delivered: 0, failed: 0, pending: 0, timeout: 0, error: 0, revenue: 0, profit: 0 },
  );

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
                  onClick={() => col.key !== 'health' && handleSort(col.key)}
                >
                  <span className="inline-flex items-center gap-0.5">
                    {col.header}
                    {col.key !== 'health' && <ChevronsUpDown size={10} className="text-gray-400" />}
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
          {rows.length > 0 && (
            <tfoot>
              <tr className="bg-gray-50 border-t-2 border-gray-200 font-bold">
                <td className="px-3 py-2.5">Итого</td>
                <td className="px-3 py-2.5 text-right">{fmt(totals.total)}</td>
                <td className="px-3 py-2.5 text-right">{fmt(totals.delivered)}</td>
                <td className="px-3 py-2.5 text-right">{fmt(totals.failed)}</td>
                <td className="px-3 py-2.5 text-right">{fmt(totals.pending)}</td>
                <td className="px-3 py-2.5 text-right">{fmt(totals.timeout)}</td>
                <td className="px-3 py-2.5 text-right">{fmt(totals.error)}</td>
                <td className="px-3 py-2.5 text-right">{totals.total > 0 ? fmtPct(totals.delivered / totals.total) : '—'}</td>
                <td className="px-3 py-2.5 text-right">{fmtMoney(totals.revenue)}</td>
                <td className="px-3 py-2.5 text-right">{fmtMoney(totals.profit)}</td>
                <td className="px-3 py-2.5 text-right">{totals.revenue > 0 ? fmtPct(totals.profit / totals.revenue) : '—'}</td>
                <td></td>
              </tr>
            </tfoot>
          )}
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
