import React, { type ReactNode } from 'react';
import { Button } from '../ui/Button';

export interface Column<T> {
  key: string;
  header: string;
  render?: (item: T) => ReactNode;
  sortable?: boolean;
  responsive?: boolean; // hidden on small screens
}

interface DataTableProps<T> {
  columns: Column<T>[];
  data: T[];
  total: number;
  page: number;
  pageSize: number;
  onPageChange: (page: number) => void;
  sortBy?: string;
  sortDir?: 'asc' | 'desc';
  onSort?: (key: string) => void;
  loading?: boolean;
  onRowClick?: (item: T) => void;
  rowActions?: (item: T) => ReactNode;
  keyField?: string;
  tableLabel?: string;
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function DataTable<T extends Record<string, any>>({
  columns, data, total, page, pageSize, onPageChange,
  sortBy, sortDir, onSort, loading, onRowClick, rowActions, keyField = 'id', tableLabel = 'Таблица данных',
}: DataTableProps<T>) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  if (loading) {
    return (
      <div className="border border-gray-200 rounded-lg overflow-hidden">
        <div className="animate-pulse p-8 text-center text-gray-500" role="status" aria-live="polite">Загрузка...</div>
      </div>
    );
  }

  return (
    <div className="border border-gray-200 rounded-lg overflow-hidden" role="region" aria-label="Таблица данных">
      <div className="overflow-x-auto">
        <table className="w-full text-sm" role="table">
          <caption className="sr-only">{tableLabel}</caption>
          <thead className="bg-gray-50 border-b border-gray-200">
            <tr>
              {columns.map((col) => (
                <th
                  key={col.key}
                  className={`px-4 py-3 text-left font-medium text-gray-700
                    ${col.sortable ? 'cursor-pointer hover:text-gray-900 select-none' : ''}
                    ${col.responsive ? 'hidden sm:table-cell' : ''}`}
                  onClick={() => col.sortable && onSort?.(col.key)}
                  {...(col.sortable ? {
                    role: 'button',
                    tabIndex: 0,
                    onKeyDown: (e: React.KeyboardEvent) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        onSort?.(col.key);
                      }
                    },
                    'aria-sort': sortBy === col.key ? (sortDir === 'asc' ? 'ascending' as const : 'descending' as const) : undefined,
                  } : {})}
                >
                  <span className="inline-flex items-center gap-1">
                    {col.header}
                    {col.sortable && sortBy === col.key && (
                      <span className="text-primary">{sortDir === 'asc' ? '↑' : '↓'}</span>
                    )}
                  </span>
                </th>
              ))}
              {rowActions && <th className="px-4 py-3 w-12"></th>}
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {data.length === 0 ? (
              <tr>
                <td colSpan={columns.length + (rowActions ? 1 : 0)} className="px-4 py-8 text-center text-gray-500">
                  Данные не найдены
                </td>
              </tr>
            ) : (
              data.map((item, idx) => (
                <tr
                  key={String(item[keyField] ?? idx)}
                  className={`hover:bg-gray-50 ${onRowClick ? 'cursor-pointer' : ''}`}
                  onClick={() => onRowClick?.(item)}
                  {...(onRowClick ? {
                    tabIndex: 0,
                    role: 'row',
                    onKeyDown: (e: React.KeyboardEvent) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        onRowClick(item);
                      }
                    },
                  } : {})}
                >
                  {columns.map((col) => (
                    <td key={col.key} className={`px-4 py-3 text-gray-800 ${col.responsive ? 'hidden sm:table-cell' : ''}`}>
                      {col.render ? col.render(item) : String(item[col.key] ?? '')}
                    </td>
                  ))}
                  {rowActions && (
                    <td className="px-4 py-3" onClick={(e) => e.stopPropagation()}>
                      {rowActions(item)}
                    </td>
                  )}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
      {total > 0 && total > pageSize && (
        <nav aria-label="Постраничная навигация">
          <div className="flex items-center justify-between px-4 py-3 border-t border-gray-200 bg-gray-50">
            <span className="text-sm text-gray-700">
              {(page - 1) * pageSize + 1}–{Math.min(page * pageSize, total)} из {total}
            </span>
            <div className="flex gap-2">
              <Button
                size="sm"
                variant="secondary"
                disabled={page <= 1}
                onClick={() => onPageChange(page - 1)}
                aria-label="Предыдущая страница"
                aria-disabled={page <= 1}
              >
                Назад
              </Button>
              <Button
                size="sm"
                variant="secondary"
                disabled={page >= totalPages}
                onClick={() => onPageChange(page + 1)}
                aria-label="Следующая страница"
                aria-disabled={page >= totalPages}
              >
                Вперёд
              </Button>
            </div>
          </div>
        </nav>
      )}
    </div>
  );
}
