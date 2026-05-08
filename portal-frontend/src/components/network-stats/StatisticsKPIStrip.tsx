import type { KPI } from '../../api/networkStats';
import { KPICard } from '../network/KPICard';

const KPI_LABELS: Record<string, string> = {
  total: 'Всего',
  delivered: 'Доставлено',
  failed: 'Ошибки',
  pending: 'Ожидание',
  timeout: 'Таймаут',
  timeouts: 'Таймауты',
  errors: 'Ошибки',
  error: 'Ошибки',
  dlr_rate: 'Доставляемость',
  'delivery rate': 'Доставляемость',
  delivery_rate: 'Доставляемость',
  revenue: 'Выручка',
  cost: 'Себестоимость',
  profit: 'Прибыль',
  margin: 'Маржа',
};

function kpiLabel(kpi: KPI): string {
  const key = (kpi.name ?? '').toLowerCase().trim();
  return KPI_LABELS[key] ?? kpi.name ?? '';
}

export function StatisticsKPIStrip({ kpis }: { kpis: KPI[] }) {
  if (!kpis || !kpis.length) return null;
  return (
    <div className="flex gap-3 px-4 py-3">
      {kpis.map((kpi) => (
        <KPICard key={kpi.name} kpi={kpi} label={kpiLabel(kpi)} className="flex-1" />
      ))}
    </div>
  );
}
