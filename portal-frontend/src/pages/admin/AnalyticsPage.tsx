import { useState, useEffect, useCallback } from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, BarChart, Bar } from 'recharts';
import { PageHeader } from '../../components/layout/PageHeader';
import { StatCard } from '../../components/data/StatCard';
import { FilterBar, type FilterDef } from '../../components/data/FilterBar';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { useToast } from '../../components/ui/Toast';
import { analyticsAdminApi, type GroupedStatsResponse } from '../../api/admin';

type GroupBy = 'day' | 'operator' | 'country';

const GROUP_TABS: { value: GroupBy; label: string }[] = [
  { value: 'day', label: 'По дням' },
  { value: 'operator', label: 'По операторам' },
  { value: 'country', label: 'По странам' },
];

const filters: FilterDef[] = [
  {
    key: 'period',
    label: 'Период',
    type: 'select',
    options: [
      { value: '7d', label: 'Последние 7 дней' },
      { value: '30d', label: 'Последние 30 дней' },
      { value: '90d', label: 'Последние 90 дней' },
    ],
  },
  { key: 'client_id', label: 'ID клиента', type: 'text', placeholder: 'Все клиенты' },
];

function periodToRange(period: string): { from: string; to: string } {
  const to = new Date();
  const from = new Date();
  from.setDate(from.getDate() - (period === '90d' ? 90 : period === '30d' ? 30 : 7));
  return { from: from.toISOString().slice(0, 10), to: to.toISOString().slice(0, 10) };
}

function fmtNum(n: number) {
  return n.toLocaleString('ru-RU');
}

function fmtRate(n: number) {
  return `${n.toFixed(1)}%`;
}

function fmtCost(n: number) {
  return `${n.toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ₽`;
}

interface StatsResponse {
  summary?: {
    total_sent?: number;
    total_delivered?: number;
    total_failed?: number;
    delivery_rate?: number;
    total_cost?: number;
  };
  timeline?: { date: string; sent: number; delivered: number; failed: number }[];
  by_provider?: { provider: string; sent: number; delivered: number; success_rate: number }[];
}

export function AnalyticsPage() {
  const toast = useToast();
  const [filterValues, setFilterValues] = useState<Record<string, string>>({ period: '7d' });
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');
  const [groupBy, setGroupBy] = useState<GroupBy>('day');
  const [stats, setStats] = useState<StatsResponse | null>(null);
  const [grouped, setGrouped] = useState<GroupedStatsResponse | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchStats = useCallback(async () => {
    setLoading(true);
    setGrouped(null);
    try {
      const { from: periodFrom, to: periodTo } = periodToRange(filterValues.period || '7d');
      const from = dateFrom || periodFrom;
      const to = dateTo || periodTo;
      const clientId = filterValues.client_id || undefined;

      const [res, groupedRes] = await Promise.all([
        analyticsAdminApi.getStats({ from, to, client_id: clientId, group_by: 'day' }),
        analyticsAdminApi.getGroupedStats({ from, to, group_by: groupBy, client_id: clientId }),
      ]);
      setStats(res as StatsResponse);
      setGrouped(groupedRes);
    } catch {
      toast.error('Ошибка загрузки аналитики');
    } finally {
      setLoading(false);
    }
  }, [filterValues, dateFrom, dateTo, groupBy, toast]);

  useEffect(() => { fetchStats(); }, [fetchStats]);

  const summary = stats?.summary;
  const rows = grouped?.rows ?? [];
  const totals = grouped?.totals;

  const labelHeader = groupBy === 'day' ? 'Дата' : groupBy === 'operator' ? 'Оператор' : 'Страна';

  return (
    <>
      <PageHeader
        title="Статистика"
        breadcrumbs={[{ label: 'Админ', href: '/admin/dashboard' }, { label: 'Статистика' }]}
        actions={<Button variant="secondary" onClick={fetchStats}>Обновить</Button>}
      />

      <FilterBar filters={filters} values={filterValues} onChange={setFilterValues} onReset={() => setFilterValues({ period: '7d' })} />

      <div className="flex items-end gap-3 mb-4">
        <Input
          type="date"
          label="С"
          value={dateFrom}
          onChange={e => setDateFrom(e.target.value)}
        />
        <Input
          type="date"
          label="По"
          value={dateTo}
          onChange={e => setDateTo(e.target.value)}
        />
        {(dateFrom || dateTo) && (
          <Button variant="ghost" size="sm" onClick={() => { setDateFrom(''); setDateTo(''); }}>
            Сбросить даты
          </Button>
        )}
      </div>

      <div className="grid grid-cols-2 md:grid-cols-5 gap-4 mb-6">
        <StatCard title="Всего отправлено" value={summary?.total_sent?.toLocaleString() ?? '-'} />
        <StatCard title="Доставлено" value={summary?.total_delivered?.toLocaleString() ?? '-'} />
        <StatCard title="Ошибки" value={summary?.total_failed?.toLocaleString() ?? '-'} />
        <StatCard title="Доставляемость" value={summary?.delivery_rate != null ? `${summary.delivery_rate}%` : '-'} />
        <StatCard title="Общая стоимость" value={summary?.total_cost != null ? `${summary.total_cost} ₽` : '-'} />
      </div>

      <div className="bg-white border border-gray-200 rounded-lg mb-6">
        <div className="flex items-center gap-1 px-4 pt-4 pb-0 border-b border-gray-200">
          {GROUP_TABS.map((tab) => (
            <button
              key={tab.value}
              onClick={() => setGroupBy(tab.value)}
              className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                groupBy === tab.value
                  ? 'border-primary text-primary'
                  : 'border-transparent text-gray-500 hover:text-gray-700'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>

        <div className="h-[520px] overflow-y-auto relative border-t border-gray-200">
          <table className="w-full text-sm">
            <thead className="sticky top-0 bg-gray-50 z-10 shadow-sm">
              <tr className="border-b border-gray-200">
                <th className="text-left px-4 py-2 font-medium text-gray-600">{labelHeader}</th>
                <th className="text-right px-4 py-2 font-medium text-gray-600">Отправлено</th>
                <th className="text-right px-4 py-2 font-medium text-gray-600">Доставлено</th>
                <th className="text-right px-4 py-2 font-medium text-gray-600">Ошибки</th>
                <th className="text-right px-4 py-2 font-medium text-gray-600">Доставляемость</th>
                <th className="text-right px-4 py-2 font-medium text-gray-600">Стоимость</th>
              </tr>
            </thead>
            <tbody>
              {loading && (
                <tr>
                  <td colSpan={6} className="text-center py-12 text-gray-400">Загрузка...</td>
                </tr>
              )}
              {!loading && rows.length === 0 && (
                <tr>
                  <td colSpan={6} className="text-center py-12 text-gray-400">Нет данных за выбранный период</td>
                </tr>
              )}
              {!loading && rows.map((row) => (
                <tr key={row.label} className="border-b border-gray-100 hover:bg-gray-50">
                  <td className="px-4 py-2 text-gray-900">{row.label}</td>
                  <td className="px-4 py-2 text-right text-gray-700">{fmtNum(row.sent)}</td>
                  <td className="px-4 py-2 text-right text-green-700">{fmtNum(row.delivered)}</td>
                  <td className="px-4 py-2 text-right text-red-600">{fmtNum(row.failed)}</td>
                  <td className="px-4 py-2 text-right text-gray-700">{fmtRate(row.delivery_rate)}</td>
                  <td className="px-4 py-2 text-right text-gray-700">{fmtCost(row.cost)}</td>
                </tr>
              ))}
            </tbody>
            {!loading && totals && rows.length > 0 && (
              <tfoot className="sticky bottom-0 bg-gray-50 font-semibold border-t-2 border-gray-200">
                <tr className="text-gray-900">
                  <td className="px-4 py-2">Итого</td>
                  <td className="px-4 py-2 text-right">{fmtNum(totals.sent)}</td>
                  <td className="px-4 py-2 text-right text-green-700">{fmtNum(totals.delivered)}</td>
                  <td className="px-4 py-2 text-right text-red-600">{fmtNum(totals.failed)}</td>
                  <td className="px-4 py-2 text-right">{fmtRate(totals.delivery_rate)}</td>
                  <td className="px-4 py-2 text-right">{fmtCost(totals.cost)}</td>
                </tr>
              </tfoot>
            )}
          </table>
        </div>
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
