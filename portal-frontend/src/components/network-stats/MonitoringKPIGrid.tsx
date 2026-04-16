import type { KPI } from '../../api/networkStats';

function formatMonitorValue(kpi: KPI): string {
  const name = kpi.name.toLowerCase();
  if (name.includes('latency') || name.includes('p50') || name.includes('p95')) {
    return `${(kpi.value / 1000).toFixed(1)} сек`;
  }
  if (name.includes('throughput')) {
    return `${kpi.value.toFixed(0)} msg/s`;
  }
  if (name.includes('health')) {
    return '';
  }
  return kpi.value.toLocaleString('ru-RU');
}

function kpiColor(kpi: KPI): string {
  if (kpi.status === 'danger') return 'text-red-600';
  if (kpi.status === 'warning') return 'text-amber-600';
  return 'text-slate-900';
}

export function MonitoringKPIGrid({ kpis }: { kpis: KPI[] }) {
  if (!kpis.length) return null;
  return (
    <div className="grid grid-cols-4 gap-2.5 px-4 py-3">
      {kpis.map((kpi) => (
        <div key={kpi.name} className="rounded-lg border border-gray-200 bg-white px-3 py-2.5">
          <div className="text-[10px] uppercase tracking-wide text-gray-400">{kpi.name}</div>
          <div className={`mt-1 text-xl font-bold ${kpiColor(kpi)}`}>
            {formatMonitorValue(kpi)}
          </div>
        </div>
      ))}
    </div>
  );
}
