import { useState, useEffect, useCallback } from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, BarChart, Bar } from 'recharts';
import { PageHeader } from '../../components/layout/PageHeader';
import { StatCard } from '../../components/data/StatCard';
import { FilterBar, type FilterDef } from '../../components/data/FilterBar';
import { Button } from '../../components/ui/Button';
import { useToast } from '../../components/ui/Toast';
import { analyticsAdminApi } from '../../api/admin';

const filters: FilterDef[] = [
  { key: 'period', label: 'Period', type: 'select', options: [{ value: '7d', label: 'Last 7 days' }, { value: '30d', label: 'Last 30 days' }, { value: '90d', label: 'Last 90 days' }] },
  { key: 'client_id', label: 'Client ID', type: 'text', placeholder: 'All clients' },
  { key: 'group_by', label: 'Group by', type: 'select', options: [{ value: 'day', label: 'Day' }, { value: 'week', label: 'Week' }, { value: 'country', label: 'Country' }] },
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
  }, [filterValues, toast]);

  useEffect(() => { fetchStats(); }, [fetchStats]);

  const summary = stats?.summary;

  return (
    <>
      <PageHeader title="Analytics" actions={<Button variant="secondary" onClick={fetchStats}>Refresh</Button>} />
      <FilterBar filters={filters} values={filterValues} onChange={setFilterValues} />
      <div className="grid grid-cols-2 md:grid-cols-5 gap-4 mb-6">
        <StatCard title="Total Sent" value={summary?.total_sent?.toLocaleString() ?? '-'} />
        <StatCard title="Delivered" value={summary?.total_delivered?.toLocaleString() ?? '-'} />
        <StatCard title="Failed" value={summary?.total_failed?.toLocaleString() ?? '-'} />
        <StatCard title="Delivery Rate" value={summary?.delivery_rate != null ? `${summary.delivery_rate}%` : '-'} />
        <StatCard title="Total Cost" value={summary?.total_cost != null ? `$${summary.total_cost}` : '-'} />
      </div>
      {!loading && stats?.timeline && stats.timeline.length > 0 && (
        <div className="bg-white border border-gray-200 rounded-lg p-4 mb-6">
          <h3 className="text-sm font-medium text-gray-600 mb-3">Message Volume</h3>
          <ResponsiveContainer width="100%" height={300}>
            <LineChart data={stats.timeline}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="date" tick={{ fontSize: 12 }} />
              <YAxis tick={{ fontSize: 12 }} />
              <Tooltip />
              <Line type="monotone" dataKey="delivered" stroke="#4caf50" strokeWidth={2} name="Delivered" />
              <Line type="monotone" dataKey="failed" stroke="#d32f2f" strokeWidth={2} name="Failed" />
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}
      {!loading && stats?.by_provider && stats.by_provider.length > 0 && (
        <div className="bg-white border border-gray-200 rounded-lg p-4">
          <h3 className="text-sm font-medium text-gray-600 mb-3">By Provider</h3>
          <ResponsiveContainer width="100%" height={250}>
            <BarChart data={stats.by_provider}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="provider" tick={{ fontSize: 12 }} />
              <YAxis tick={{ fontSize: 12 }} />
              <Tooltip />
              <Bar dataKey="delivered" fill="#4caf50" name="Delivered" />
              <Bar dataKey="sent" fill="#1976d2" name="Sent" />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}
    </>
  );
}
