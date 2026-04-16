import type { KPI } from '../../api/networkStats';

function formatValue(kpi: KPI): string {
  const v = kpi.value;
  const name = kpi.name.toLowerCase();
  if (name.includes('rate') || name.includes('маржа') || name.includes('доставляемость')) {
    return `${(v * 100).toFixed(1)}%`;
  }
  if (name.includes('выручка') || name.includes('прибыль') || name.includes('себестоимость') || name.includes('стоимость')) {
    return `${v.toLocaleString('ru-RU')} ₽`;
  }
  return v.toLocaleString('ru-RU');
}

function valueColor(kpi: KPI): string {
  if (kpi.status === 'danger') return 'text-red-600';
  if (kpi.status === 'warning') return 'text-amber-600';
  const name = kpi.name.toLowerCase();
  if (name.includes('ошибк') || name.includes('error')) return kpi.value > 0 ? 'text-red-600' : 'text-slate-900';
  if (name.includes('доставлен') || name.includes('прибыль')) return 'text-emerald-600';
  return 'text-slate-900';
}

export function StatisticsKPIStrip({ kpis }: { kpis: KPI[] }) {
  if (!kpis.length) return null;
  return (
    <div className="flex gap-3 px-4 py-3">
      {kpis.map((kpi) => (
        <div key={kpi.name} className="flex-1 rounded-lg border border-gray-200 bg-white p-3">
          <div className="text-[11px] uppercase tracking-wide text-gray-400">{kpi.name}</div>
          <div className={`mt-1 text-xl font-bold ${valueColor(kpi)}`}>
            {formatValue(kpi)}
          </div>
          {kpi.delta !== 0 && (
            <div className={`mt-1 text-[11px] flex items-center gap-0.5 ${kpi.delta > 0 ? 'text-emerald-600' : 'text-red-600'}`}>
              {kpi.delta > 0 ? '↑' : '↓'} {Math.abs(kpi.delta * 100).toFixed(1)}%
            </div>
          )}
        </div>
      ))}
    </div>
  );
}
