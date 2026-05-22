import React, { useState, type ReactNode } from 'react';
import { Button } from '../ui/Button';
import { BulkActionBar, type BulkAction } from './BulkActionBar';

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
  bulkActions?: BulkAction<T>[];
  emptyMessage?: ReactNode;
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function DataTable<T extends Record<string, any>>({
  columns, data, total, page, pageSize, onPageChange,
  sortBy, sortDir, onSort, loading, onRowClick, rowActions,
  keyField = 'id', tableLabel = 'Таблица данных', bulkActions, emptyMessage,
}: DataTableProps<T>) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());

  const hasBulk = !!bulkActions && bulkActions.length > 0;
  const allIds = data.map((item) => String(item[keyField]));
  const allSelected = allIds.length > 0 && allIds.every((id) => selectedIds.has(id));
  const someSelected = !allSelected && allIds.some((id) => selectedIds.has(id));

  const toggleAll = () => {
    if (allSelected) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(allIds));
    }
  };

  const toggleOne = (id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const selectedItems = data.filter((item) => selectedIds.has(String(item[keyField])));

  if (loading) {
    return (
      <div className="border border-gray-200 dark:border-slate-700 rounded-lg overflow-hidden bg-white dark:bg-slate-900">
        <div className="animate-pulse p-8 text-center text-gray-500 dark:text-slate-400" role="status" aria-live="polite">Загрузка...</div>
      </div>
    );
  }

  return (
    <div>
      {hasBulk && (
        <BulkActionBar
          selectedIds={selectedIds}
          selectedItems={selectedItems}
          actions={bulkActions!}
          onClear={() => setSelectedIds(new Set())}
        />
      )}
      <div className="border border-gray-200 dark:border-slate-700 rounded-lg overflow-hidden bg-white dark:bg-slate-900" role="region" aria-label="Таблица данных">
        <div className="overflow-x-auto">
          <table className="w-full text-sm" role="table">
            <caption className="sr-only">{tableLabel}</caption>
            <thead className="bg-gray-50 dark:bg-slate-800 border-b border-gray-200 dark:border-slate-700">
              <tr>
                {hasBulk && (
                  <th className="px-3 py-3 w-10">
                    <input
                      type="checkbox"
                      checked={allSelected}
                      ref={(el) => { if (el) el.indeterminate = someSelected; }}
                      onChange={toggleAll}
                      aria-label="Выбрать все"
                      className="rounded"
                    />
                  </th>
                )}
                {columns.map((col) => (
                  <th
                    key={col.key}
                    className={`px-4 py-3 text-left font-medium text-gray-700 dark:text-slate-300
                      ${col.sortable ? 'cursor-pointer hover:text-gray-900 dark:hover:text-slate-100 select-none' : ''}
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
            <tbody className="divide-y divide-gray-100 dark:divide-slate-800">
              {data.length === 0 ? (
                <tr>
                  <td colSpan={columns.length + (rowActions ? 1 : 0) + (hasBulk ? 1 : 0)} className="px-4 py-8 text-center text-gray-500 dark:text-slate-400">
                    {emptyMessage ?? 'Данные не найдены'}
                  </td>
                </tr>
              ) : (
                data.map((item, idx) => {
                  const itemId = String(item[keyField] ?? idx);
                  const isSelected = selectedIds.has(itemId);
                  return (
                    <tr
                      key={itemId}
                      className={`hover:bg-gray-50 dark:hover:bg-slate-800 ${onRowClick ? 'cursor-pointer' : ''} ${isSelected ? 'bg-blue-50/40 dark:bg-blue-950/30' : ''}`}
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
                      {hasBulk && (
                        <td className="px-3 py-3" onClick={(e) => e.stopPropagation()}>
                          <input
                            type="checkbox"
                            checked={isSelected}
                            onChange={() => toggleOne(itemId)}
                            aria-label={`Выбрать элемент`}
                            className="rounded"
                          />
                        </td>
                      )}
                      {columns.map((col) => (
                        <td key={col.key} className={`px-4 py-3 text-gray-800 dark:text-slate-200 ${col.responsive ? 'hidden sm:table-cell' : ''}`}>
                          {col.render ? col.render(item) : String(item[col.key] ?? '')}
                        </td>
                      ))}
                      {rowActions && (
                        <td className="px-4 py-3" onClick={(e) => e.stopPropagation()}>
                          {rowActions(item)}
                        </td>
                      )}
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
        {total > 0 && total > pageSize && (
          <nav aria-label="Постраничная навигация">
            <div className="flex items-center justify-between px-4 py-3 border-t border-gray-200 dark:border-slate-700 bg-gray-50 dark:bg-slate-800">
              <span className="text-sm text-gray-700 dark:text-slate-300">
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
    </div>
  );
}
