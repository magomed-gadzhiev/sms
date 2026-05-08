import type { KPI } from '../../api/networkStats';

const MONITORING_KPI_LABELS: Record<string, string> = {
  throughput: 'Пропускная способность',
  dlr_rate: 'Доставляемость',
  pending: 'Ожидание',
  errors: 'Ошибки',
  timeouts: 'Таймауты',
  unhealthy_providers: 'Проблемные провайдеры',
};

function monitorKPILabel(kpi: KPI): string {
  return MONITORING_KPI_LABELS[(kpi.name ?? '').toLowerCase()] ?? kpi.name ?? '';
}

function formatMonitorValue(kpi: KPI): string {
  if (kpi.value === undefined || kpi.value === null || Number.isNaN(kpi.value)) {
    return '—';
  }
  const v = kpi.value;
  const name = (kpi.name ?? '').toLowerCase();
  if (name.includes('latency') || name.includes('p50') || name.includes('p95')) {
    return `${(v / 1000).toFixed(1)} сек`;
  }
  if (name.includes('throughput')) {
    return `${v.toFixed(0)} msg/s`;
  }
  if (name.includes('health')) {
    return '';
  }
  if (name.includes('dlr_rate') || name.includes('доставляемость')) {
    return `${(v * 100).toFixed(1)}%`;
  }
  return v.toLocaleString('ru-RU');
}

function kpiColor(kpi: KPI): string {
  if (kpi.value === undefined || kpi.value === null || Number.isNaN(kpi.value)) {
    return 'text-slate-400';
  }
  if (kpi.status === 'danger') return 'text-red-600';
  if (kpi.status === 'warning') return 'text-amber-600';
  return 'text-slate-900';
}

export function MonitoringKPIGrid({ kpis }: { kpis: KPI[] }) {
  if (!kpis || !kpis.length) return null;
  return (
    <div className="grid grid-cols-4 gap-2.5 px-4 py-3">
      {kpis.map((kpi) => (
        <div key={kpi.name} className="rounded-lg border border-gray-200 bg-white px-3 py-2.5">
          <div className="text-[10px] uppercase tracking-wide text-gray-400">{monitorKPILabel(kpi)}</div>
          <div className={`mt-1 text-xl font-bold ${kpiColor(kpi)}`}>
            {formatMonitorValue(kpi)}
          </div>
        </div>
      ))}
    </div>
  );
}
