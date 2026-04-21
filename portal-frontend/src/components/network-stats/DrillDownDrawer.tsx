import { X, ChevronRight, Info } from 'lucide-react';
import * as Tabs from '@radix-ui/react-tabs';
import type { DrillDownResponse } from '../../api/networkStats';

interface DrillDownLevel {
  sliceType: string;
  sliceValue: string;
  label: string;
}

interface DrillDownDrawerProps {
  open: boolean;
  onClose: () => void;
  data: DrillDownResponse | null;
  stack: DrillDownLevel[];
  activeView: string;
  onViewChange: (view: string) => void;
  onNavigate: (level: number) => void;
  onDrillDeeper: (sliceType: string, sliceValue: string, label: string) => void;
  loading: boolean;
}

function fmt(n: number): string { return (n ?? 0).toLocaleString('ru-RU'); }
function fmtMoney(n: number): string { return `${(n ?? 0).toLocaleString('ru-RU')} ₽`; }
function fmtPct(n: number): string { return `${((n ?? 0) * 100).toFixed(1)}%`; }

function fmtKPI(kpi: { name: string; value: number }): string {
  const name = (kpi.name ?? '').toLowerCase();
  if (name.includes('rate') || name.includes('маржа') || name.includes('margin') || name.includes('доставляемость')) return fmtPct(kpi.value);
  if (name.includes('revenue') || name.includes('profit') || name.includes('cost') || name.includes('выручка') || name.includes('прибыль') || name.includes('себестоимость')) return fmtMoney(kpi.value);
  return fmt(kpi.value);
}

function dlrColor(rate: number): string {
  if (rate < 0.80) return 'text-red-600 font-medium';
  if (rate < 0.90) return 'text-amber-600';
  return 'text-emerald-600';
}

function shortLabel(label: string): string {
  if (!label) return '—';
  // "2026-04-15 00:00:00+00" / "2026-04-15T00:00:00Z" → "2026-04-15"
  const midnight = label.match(/^(\d{4}-\d{2}-\d{2})[T ]00:00(:00)?(\+\d{2}(:?\d{2})?|Z)?$/);
  if (midnight) return midnight[1];
  // "2026-04-15 09:00:00+00" → "2026-04-15 09:00"
  const m = label.match(/^(\d{4}-\d{2}-\d{2})[T ](\d{2}:\d{2})/);
  if (m) return `${m[1]} ${m[2]}`;
  return label;
}

// Value of the matching money field per synthetic row slice. Backend sets
// Revenue/Cost/Profit in currency units and Margin as a [0..1] fraction; see
// drillDownMoney in internal/services/network_analytics/.../stats_repository.go.
function moneyRowValue(row: { slice: string; revenue: number; cost: number; profit: number; margin: number }): string {
  switch (row.slice) {
    case 'Выручка': return fmtMoney(row.revenue);
    case 'Себестоимость': return fmtMoney(row.cost);
    case 'Прибыль': return fmtMoney(row.profit);
    case 'Маржа': return fmtPct(row.margin);
    default: return '—';
  }
}

function healthBadge(h: string): { bg: string; text: string; label: string } {
  if (h === 'danger') return { bg: 'bg-red-50 border-red-200', text: 'text-red-600', label: 'Проблемный' };
  if (h === 'warning') return { bg: 'bg-amber-50 border-amber-200', text: 'text-amber-600', label: 'Внимание' };
  return { bg: 'bg-emerald-50 border-emerald-200', text: 'text-emerald-600', label: 'Норма' };
}

export function DrillDownDrawer({ open, onClose, data, stack, activeView, onViewChange, onNavigate, onDrillDeeper, loading }: DrillDownDrawerProps) {
  if (!open) return null;

  const badge = healthBadge(data?.health || 'ok');

  return (
    <>
      {/* Overlay */}
      <div className="fixed inset-0 bg-black/20 z-40" onClick={onClose} />

      {/* Drawer */}
      <div className="fixed top-0 right-0 h-full w-[480px] bg-white border-l border-gray-200 shadow-xl z-50 flex flex-col overflow-y-auto">
        {/* Header */}
        <div className="sticky top-0 bg-white z-10 px-4 py-4 border-b border-gray-200">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-1 text-xs">
              <button onClick={() => onNavigate(0)} className="text-blue-600 hover:underline">Статистика</button>
              {stack.map((level, i) => (
                <span key={i} className="flex items-center gap-1">
                  <ChevronRight size={12} className="text-gray-400" />
                  {i < stack.length - 1 ? (
                    <button onClick={() => onNavigate(i + 1)} className="text-blue-600 hover:underline">{shortLabel(level.label)}</button>
                  ) : (
                    <span className="font-semibold text-slate-900">{shortLabel(level.label)}</span>
                  )}
                </span>
              ))}
            </div>
            <button onClick={onClose} className="p-1 text-gray-400 hover:text-gray-600">
              <X size={18} />
            </button>
          </div>
          {data && (
            <div className="mt-2 flex items-center gap-2">
              <span className={`px-2 py-0.5 rounded text-[11px] font-medium border ${badge.bg} ${badge.text}`}>
                {badge.label}
              </span>
            </div>
          )}
        </div>

        {/* Summary KPIs */}
        {data?.summary && data.summary.length > 0 && (
          <div className="grid grid-cols-3 gap-2 px-4 py-3 border-b border-gray-100">
            {data.summary.map(kpi => (
              <div key={kpi.name} className="text-center">
                <div className="text-[10px] uppercase text-gray-400">{kpi.name}</div>
                <div className="text-lg font-bold text-slate-900">{fmtKPI(kpi)}</div>
              </div>
            ))}
          </div>
        )}

        {/* Tabs */}
        <Tabs.Root value={activeView} onValueChange={onViewChange}>
          <Tabs.List className="flex border-b border-gray-200 px-4">
            {['operators', 'statuses', 'errors', 'timeline', 'money'].map(view => (
              <Tabs.Trigger
                key={view}
                value={view}
                className="px-3 py-2.5 text-xs font-medium border-b-2 data-[state=active]:border-blue-600 data-[state=active]:text-blue-600 text-gray-500 border-transparent"
              >
                {{ operators: 'По операторам', statuses: 'По статусам', errors: 'По ошибкам', timeline: 'Динамика', money: 'Деньги' }[view]}
              </Tabs.Trigger>
            ))}
          </Tabs.List>

          <Tabs.Content value={activeView} className="flex-1 px-4 py-3">
            {loading ? (
              <div className="text-center py-8 text-gray-400">Загрузка...</div>
            ) : data?.rows && data.rows.length > 0 ? (
              activeView === 'money' ? (
                <table className="w-full text-xs">
                  <thead>
                    <tr className="border-b border-gray-200">
                      <th className="text-left py-1.5 px-2 text-gray-500 font-semibold">Показатель</th>
                      <th className="text-right py-1.5 px-2 text-gray-500 font-semibold">Значение</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.rows.map((row, i) => (
                      <tr key={row.slice + i} className="border-b border-gray-100">
                        <td className="py-2 px-2 text-slate-900">{row.slice || '—'}</td>
                        <td className="py-2 px-2 text-right font-medium text-slate-900">{moneyRowValue(row)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              ) : (
                <table className="w-full text-xs">
                  <thead>
                    <tr className="border-b border-gray-200">
                      <th className="text-left py-1.5 px-2 text-gray-500 font-semibold">Срез</th>
                      <th className="text-right py-1.5 px-2 text-gray-500 font-semibold">Всего</th>
                      <th className="text-right py-1.5 px-2 text-gray-500 font-semibold">Достав.</th>
                      <th className="text-right py-1.5 px-2 text-gray-500 font-semibold">Ошибки</th>
                      <th className="text-right py-1.5 px-2 text-gray-500 font-semibold">DLR%</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.rows.map((row, i) => (
                      <tr
                        key={row.slice + i}
                        className={`border-b border-gray-100 cursor-pointer transition-colors ${row.health === 'danger' ? 'bg-red-50 hover:bg-red-100' : 'hover:bg-gray-50'}`}
                        onClick={() => onDrillDeeper('operator', row.slice, row.slice)}
                      >
                        <td className="py-2 px-2 text-blue-600 flex items-center gap-1">
                          <ChevronRight size={10} className="text-blue-400" />
                          {shortLabel(row.slice)}
                        </td>
                        <td className="py-2 px-2 text-right">{fmt(row.total)}</td>
                        <td className="py-2 px-2 text-right text-emerald-600">{fmt(row.delivered)}</td>
                        <td className="py-2 px-2 text-right">{row.error > 0 ? <span className="text-red-600 font-medium">{fmt(row.error)}</span> : '0'}</td>
                        <td className="py-2 px-2 text-right"><span className={dlrColor(row.dlr_rate)}>{fmtPct(row.dlr_rate)}</span></td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )
            ) : (
              <div className="text-center py-8 text-gray-400">Нет данных</div>
            )}
          </Tabs.Content>
        </Tabs.Root>

        {/* Hint */}
        {data?.rows && data.rows.length > 0 && activeView !== 'money' && (
          <div className="mx-4 mb-4 px-3 py-2 bg-gray-50 border border-gray-200 rounded-md text-[11px] text-gray-500 flex items-center gap-1.5">
            <Info size={14} className="text-gray-400 shrink-0" />
            Кликните по строке для перехода на следующий уровень детализации
          </div>
        )}
      </div>
    </>
  );
}
