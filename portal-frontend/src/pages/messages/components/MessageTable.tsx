import { useId, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import type { DetalizationMessage } from '../../../api/client';

const STATUS_LABELS: Record<string, string> = {
  pending: 'Ожидание', queued: 'В очереди', sent: 'Отправлено',
  delivered: 'Доставлено', failed: 'Ошибка', expired: 'Истекло',
  rejected: 'Отклонено', scheduled: 'Запланировано',
};
const STATUS_COLORS: Record<string, string> = {
  delivered: 'bg-green-100 text-green-800', sent: 'bg-blue-100 text-blue-800',
  failed: 'bg-red-100 text-red-800', rejected: 'bg-red-100 text-red-800',
  expired: 'bg-gray-100 text-gray-600', queued: 'bg-yellow-100 text-yellow-800',
  pending: 'bg-yellow-100 text-yellow-800', scheduled: 'bg-purple-100 text-purple-800',
};

function fmt(s?: string | null) {
  if (!s) return '—';
  return new Date(s).toLocaleString('ru-RU');
}

export type SortField = 'submitted_at' | 'created_at' | 'status_at' | 'total_amount' | 'segment_count' | 'status';
export interface SortState { field: SortField; order: 'asc' | 'desc' }

interface Props {
  data: DetalizationMessage[];
  total: number;
  page: number;
  pageSize: number;
  onPageChange: (page: number) => void;
  onPageSizeChange: (size: number) => void;
  visibleColumns: Set<string>;
  sort: SortState;
  onSort: (field: SortField) => void;
  onRowClick: (msg: DetalizationMessage) => void;
  loading: boolean;
  /** Selection: when these are provided, a leading checkbox column is rendered. */
  selectedIds?: Set<string>;
  onToggleOne?: (id: string) => void;
  onToggleAll?: () => void;
  allSelected?: boolean;
  someSelected?: boolean;
}

interface ColSpec {
  key: string;
  header: string;
  sortField?: SortField;
  render: (msg: DetalizationMessage) => ReactNode;
}

export const ALL_COLUMNS: ColSpec[] = [
  { key: 'login', header: 'Логин', render: (m) => m.login || '—' },
  { key: 'destination', header: 'Номер телефона', render: (m) => m.destination },
  { key: 'operator_name', header: 'Оператор', render: (m) => m.operator_name || '—' },
  { key: 'channel', header: 'Канал', render: (m) => m.channel || '—' },
  {
    key: 'text_preview', header: 'Текст сообщения',
    render: (m) => <span className="block max-w-[200px] truncate text-sm" title={m.text_preview}>{m.text_preview}</span>,
  },
  {
    key: 'submitted_at', header: 'Дата отправки', sortField: 'submitted_at',
    render: (m) => <span className="text-xs whitespace-nowrap">{fmt(m.submitted_at)}</span>,
  },
  {
    key: 'status', header: 'Статус', sortField: 'status',
    render: (m) => (
      <span className={`inline-block rounded px-2 py-0.5 text-xs font-medium ${STATUS_COLORS[m.status] ?? 'bg-gray-100 text-gray-700'}`}>
        {STATUS_LABELS[m.status] ?? m.status}
      </span>
    ),
  },
  {
    key: 'total_amount', header: 'Стоимость', sortField: 'total_amount',
    render: (m) => {
      if (!m.total_amount) return '—';
      const n = parseFloat(m.total_amount);
      if (!Number.isFinite(n)) return '—';
      return `${n.toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ₽`;
    },
  },
  // optional columns
  { key: 'id', header: 'ID', render: (m) => <span className="font-mono text-xs">{m.id.substring(0, 8)}...</span> },
  { key: 'source', header: 'Имя отправителя', render: (m) => m.source || '—' },
  {
    key: 'created_at', header: 'Дата создания', sortField: 'created_at',
    render: (m) => <span className="text-xs whitespace-nowrap">{fmt(m.created_at)}</span>,
  },
  {
    key: 'status_at', header: 'Дата статуса', sortField: 'status_at',
    render: (m) => <span className="text-xs whitespace-nowrap">{fmt(m.delivered_at ?? m.failed_at)}</span>,
  },
  {
    key: 'segment_count', header: 'Сегменты', sortField: 'segment_count',
    render: (m) => String(m.segment_count),
  },
  { key: 'provider_name', header: 'Провайдер', render: (m) => m.provider_name || '—' },
  { key: 'country_name', header: 'Страна', render: (m) => m.country_name || '—' },
  { key: 'send_method', header: 'Способ отправки', render: (m) => m.send_method || '—' },
];

function buildPageWindow(current: number, total: number): (number | 'ellipsis-left' | 'ellipsis-right')[] {
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1);
  const delta = 2;
  const start = Math.max(2, current - delta);
  const end = Math.min(total - 1, current + delta);
  const pages: (number | 'ellipsis-left' | 'ellipsis-right')[] = [1];
  if (start > 2) pages.push('ellipsis-left');
  for (let p = start; p <= end; p++) pages.push(p);
  if (end < total - 1) pages.push('ellipsis-right');
  pages.push(total);
  return pages;
}

function SortIcon({ active, order }: { active: boolean; order: 'asc' | 'desc' }) {
  return (
    <svg className={`w-3 h-3 inline ml-1 ${active ? 'text-primary' : 'text-gray-300'}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      {(order === 'asc' && active)
        ? <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" />
        : <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
      }
    </svg>
  );
}

export function MessageTable({
  data, total, page, pageSize, onPageChange, onPageSizeChange,
  visibleColumns, sort, onSort, onRowClick, loading,
  selectedIds, onToggleOne, onToggleAll, allSelected, someSelected,
}: Props) {
  const pageSizeId = useId();
  const cols = ALL_COLUMNS.filter((c) => visibleColumns.has(c.key));
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const selectable = !!onToggleOne;

  return (
    <div>
      <div className="overflow-x-auto rounded-lg border border-gray-200 dark:border-slate-700">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-gray-50 dark:bg-slate-800 border-b border-gray-200 dark:border-slate-700">
              {selectable && (
                <th className="px-3 py-2 w-10">
                  <input
                    type="checkbox"
                    checked={!!allSelected}
                    ref={(el) => { if (el) el.indeterminate = !!someSelected && !allSelected; }}
                    onChange={() => onToggleAll?.()}
                    aria-label="Выбрать все на странице"
                    className="rounded"
                  />
                </th>
              )}
              {cols.map((col) => {
                const handleSortActivate = col.sortField ? () => onSort(col.sortField!) : undefined;
                return (
                  <th
                    key={col.key}
                    className={`text-left px-3 py-2 text-xs font-medium text-gray-500 whitespace-nowrap ${col.sortField ? 'cursor-pointer hover:text-gray-900 select-none' : ''}`}
                    onClick={handleSortActivate}
                    tabIndex={col.sortField ? 0 : undefined}
                    onKeyDown={col.sortField ? (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); handleSortActivate!(); } } : undefined}
                  >
                    {col.header}
                    {col.sortField && (
                      <SortIcon active={sort.field === col.sortField} order={sort.order} />
                    )}
                  </th>
                );
              })}
              <th className="px-3 py-2 text-xs font-medium text-gray-500 text-right">
                <span className="sr-only">Действия</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {loading && (
              <tr>
                <td colSpan={cols.length + 1 + (selectable ? 1 : 0)} className="text-center py-8 text-gray-400 text-sm">
                  <span role="status" className="inline-flex items-center gap-2">
                    <svg className="animate-spin w-4 h-4" fill="none" viewBox="0 0 24 24" aria-hidden="true">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
                    </svg>
                    Загрузка...
                  </span>
                </td>
              </tr>
            )}
            {!loading && data.length === 0 && (
              <tr>
                <td colSpan={cols.length + 1 + (selectable ? 1 : 0)} className="py-12">
                  <div className="text-center text-gray-400 text-sm">
                    <svg className="mx-auto mb-2 w-8 h-8 text-gray-300" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5}
                        d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-3 3v-3z" />
                    </svg>
                    <p>Сообщений не найдено</p>
                    <p className="text-xs mt-1">Попробуйте изменить фильтры</p>
                  </div>
                </td>
              </tr>
            )}
            {!loading && data.map((msg) => {
              const isSelected = !!selectedIds?.has(msg.id);
              return (
              <tr
                key={msg.id}
                onClick={() => onRowClick(msg)}
                tabIndex={0}
                onKeyDown={(e) => { if (e.key === 'Enter') onRowClick(msg); }}
                className={`border-b border-gray-100 dark:border-slate-800 hover:bg-gray-50 dark:hover:bg-slate-800 cursor-pointer ${isSelected ? 'bg-blue-50/40 dark:bg-blue-950/30' : ''}`}
              >
                {selectable && (
                  <td className="px-3 py-2" onClick={(e) => e.stopPropagation()}>
                    <input
                      type="checkbox"
                      checked={isSelected}
                      onChange={() => onToggleOne?.(msg.id)}
                      aria-label={`Выбрать сообщение ${msg.destination}`}
                      className="rounded"
                    />
                  </td>
                )}
                {cols.map((col) => (
                  <td key={col.key} className="px-3 py-2 text-gray-800">
                    {col.render(msg)}
                  </td>
                ))}
                <td className="px-3 py-2 text-right">
                  <Link
                    to={`/messages/${msg.id}`}
                    onClick={(e) => e.stopPropagation()}
                    className="text-xs text-primary hover:underline whitespace-nowrap"
                    title="Открыть детальную страницу"
                  >
                    Детали →
                  </Link>
                </td>
              </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      <div className="flex items-center justify-between mt-3">
        <div className="flex items-center gap-2 text-sm text-gray-500">
          <label htmlFor={pageSizeId}>Строк на странице:</label>
          <select
            id={pageSizeId}
            value={pageSize}
            onChange={(e) => onPageSizeChange(Number(e.target.value))}
            className="border border-gray-300 rounded px-1 py-0.5 text-sm"
          >
            <option value={20}>20</option>
            <option value={50}>50</option>
            <option value={100}>100</option>
          </select>
        </div>
        <div className="flex items-center gap-1">
          <button
            disabled={page <= 1}
            onClick={() => onPageChange(page - 1)}
            className="px-2 py-1 rounded border border-gray-200 text-sm disabled:opacity-40 hover:bg-gray-50"
          >← Предыдущая</button>
          {buildPageWindow(page, totalPages).map((p) =>
            typeof p === 'string' ? (
              <span key={p} className="px-2 text-gray-400 text-sm">…</span>
            ) : (
              <button
                key={p}
                onClick={() => onPageChange(p)}
                className={`px-3 py-1 rounded text-sm ${
                  p === page ? 'bg-primary text-white font-medium' : 'text-gray-600 hover:bg-gray-100'
                }`}
              >
                {p}
              </button>
            )
          )}
          <button
            disabled={page >= totalPages}
            onClick={() => onPageChange(page + 1)}
            className="px-2 py-1 rounded border border-gray-200 text-sm disabled:opacity-40 hover:bg-gray-50"
          >Следующая →</button>
        </div>
      </div>
    </div>
  );
}
