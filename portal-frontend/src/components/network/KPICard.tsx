import type { KPI } from '../../api/networkStats';
import { formatKPIValue } from './formatKPIValue';

/**
 * Shared KPI card. Renders a label, formatted value, and optional delta.
 *
 * Contract:
 *  - When `kpi.value` is missing, the card forces a neutral grey background
 *    and grey value text. A "—" must NEVER look "good" (audit F3).
 *  - Otherwise colour follows `kpi.status`.
 *  - `label` overrides the displayed name (callers translate the raw key).
 */

type Status = 'ok' | 'warning' | 'danger' | 'unknown';

function effectiveStatus(kpi: KPI): Status {
  if (kpi.value === undefined || kpi.value === null || Number.isNaN(kpi.value)) {
    return 'unknown';
  }
  return kpi.status ?? 'ok';
}

function valueColorClass(status: Status): string {
  switch (status) {
    case 'danger':
      return 'text-red-600';
    case 'warning':
      return 'text-amber-600';
    case 'unknown':
      return 'text-slate-400';
    default:
      return 'text-slate-900';
  }
}

function cardBgClass(status: Status): string {
  return status === 'unknown' ? 'bg-slate-50' : 'bg-white';
}

interface Props {
  kpi: KPI;
  label?: string;
  className?: string;
}

export function KPICard({ kpi, label, className }: Props) {
  const status = effectiveStatus(kpi);
  return (
    <div
      className={`rounded-lg border border-gray-200 p-3 ${cardBgClass(status)} ${className ?? ''}`}
    >
      <div className="text-[11px] uppercase tracking-wide text-gray-400">
        {label ?? kpi.name}
      </div>
      <div className={`mt-1 text-xl font-bold ${valueColorClass(status)}`}>
        {formatKPIValue(kpi)}
      </div>
      {kpi.delta != null && kpi.delta !== 0 && isFinite(kpi.delta) && (
        <div
          className={`mt-1 text-[11px] flex items-center gap-0.5 ${
            kpi.delta > 0 ? 'text-emerald-600' : 'text-red-600'
          }`}
        >
          {kpi.delta > 0 ? '↑' : '↓'} {Math.abs(kpi.delta).toFixed(1)}%
        </div>
      )}
    </div>
  );
}
