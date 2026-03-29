import { useState, useEffect, useCallback } from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, BarChart, Bar } from 'recharts';
import { PageHeader } from '../../components/layout/PageHeader';
import { StatCard } from '../../components/data/StatCard';
import { FilterBar, type FilterDef } from '../../components/data/FilterBar';
import { Button } from '../../components/ui/Button';
import { useToast } from '../../components/ui/Toast';
import { analyticsAdminApi } from '../../api/admin';

const filters: FilterDef[] = [
  { key: 'period', label: 'Период', type: 'select', options: [{ value: '7d', label: 'Последние 7 дней' }, { value: '30d', label: 'Последние 30 дней' }, { value: '90d', label: 'Последние 90 дней' }] },
  { key: 'client_id', label: 'Client ID', type: 'text', placeholder: 'Все клиенты' },
  { key: 'group_by', label: 'Группировка', type: 'select', options: [{ value: 'day', label: 'По дням' }, { value: 'week', label: 'По неделям' }, { value: 'country', label: 'По странам' }] },
];

function periodToRange(period: string): { from: string; to: string } {
  const to = new Date(); const from = new Date();
  from.setDate(from.getDate() - (period === '90d' ? 90 : period === '30d' ? 30 : 7));
  return { from: from.toISOString().slice(0, 10), to: to.toISOString().slice(0, 10) };
}

interface StatsResponse {
  summary?: { total_sent?: number; total_delivered?: number; total_failed?: number; delivery_rate?: number; total_cost?: number };
  timeline?: { date: string; sent: number; delivered: number; failed: number }[];
  by_provider?: { provider: string; sent: number; delivered: number; success_rate: number }[];
}

export function AnalyticsPage() {
  const toast = useToast();
  const [filterValues, setFilterValues] = useState<Record<string, string>>({ period: '7d', group_by: 'day' });
  const [stats, setStats] = useState<StatsResponse | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchStats = useCallback(async () => {
    setLoading(true);
    try {
      const { from, to } = periodToRange(filterValues.period || '7d');
      const res = await analyticsAdminApi.getStats({ from, to, client_id: filterValues.client_id || undefined, group_by: filterValues.group_by || 'day' });
      setStats(res as StatsResponse);
    } catch { toast.error('Failed to load analytics'); }
    finally { setLoading(false); }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filterValues]);

  useEffect(() => { fetchStats(); }, [fetchStats]);

  const summary = stats?.summary;

  return (
    <>
      <PageHeader title="Аналитика" breadcrumbs={[{ label: 'Админ', href: '/admin/dashboard' }, { label: 'Аналитика' }]} actions={<Button variant="secondary" onClick={fetchStats}>Обновить</Button>} />
      <FilterBar filters={filters} values={filterValues} onChange={setFilterValues} />
      <div className="grid grid-cols-2 md:grid-cols-5 gap-4 mb-6">
        <StatCard title="Всего отправлено" value={summary?.total_sent?.toLocaleString() ?? '-'} />
        <StatCard title="Доставлено" value={summary?.total_delivered?.toLocaleString() ?? '-'} />
        <StatCard title="Ошибки" value={summary?.total_failed?.toLocaleString() ?? '-'} />
        <StatCard title="Доставляемость" value={summary?.delivery_rate != null ? `${summary.delivery_rate}%` : '-'} />
        <StatCard title="Общая стоимость" value={summary?.total_cost != null ? `${summary.total_cost} ₽` : '-'} />
      </div>
      {!loading && stats?.timeline && stats.timeline.length > 0 && (
        <div className="bg-white border border-gray-200 rounded-lg p-4 mb-6">
          <h3 className="text-sm font-medium text-gray-600 mb-3">Объём сообщений</h3>
          <ResponsiveContainer width="100%" height={300}>
            <LineChart data={stats.timeline}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="date" tick={{ fontSize: 12 }} />
              <YAxis tick={{ fontSize: 12 }} />
              <Tooltip />
              <Line type="monotone" dataKey="delivered" stroke="#4caf50" strokeWidth={2} name="Доставлено" />
              <Line type="monotone" dataKey="failed" stroke="#d32f2f" strokeWidth={2} name="Ошибки" />
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}
      {!loading && stats?.by_provider && stats.by_provider.length > 0 && (
        <div className="bg-white border border-gray-200 rounded-lg p-4">
          <h3 className="text-sm font-medium text-gray-600 mb-3">По провайдерам</h3>
          <ResponsiveContainer width="100%" height={250}>
            <BarChart data={stats.by_provider}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="provider" tick={{ fontSize: 12 }} />
              <YAxis tick={{ fontSize: 12 }} />
              <Tooltip />
              <Bar dataKey="delivered" fill="#4caf50" name="Доставлено" />
              <Bar dataKey="sent" fill="#1976d2" name="Отправлено" />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}
    </>
  );
}
