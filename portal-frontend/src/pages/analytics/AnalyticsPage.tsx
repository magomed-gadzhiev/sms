import { useState, useEffect, useCallback } from 'react';
import { analyticsApi, ApiError } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { StatCard } from '../../components/data/StatCard';
import { DataTable, type Column } from '../../components/data/DataTable';

interface AnalyticsSummary {
  total_sent: number;
  total_delivered: number;
  total_failed: number;
  total_expired: number;
  delivery_rate: number;
  total_cost: string;
  currency: string;
}

interface TimelineEntry {
  period: string;
  sent: number;
  delivered: number;
  failed: number;
  delivery_rate: number;
}

interface CountryEntry {
  country: string;
  sent: number;
  delivered: number;
  failed: number;
  delivery_rate: number;
}

interface AnalyticsData {
  summary: AnalyticsSummary;
  timeline: TimelineEntry[];
  by_country: CountryEntry[];
}

const PERIODS = ['7d', '30d', '90d'] as const;

function formatPeriod(iso: string): string {
  const parts = iso.split('-');
  if (parts.length === 3) return `${parts[2]}.${parts[1]}.${parts[0]}`;
  return iso;
}

const timelineColumns: Column<TimelineEntry>[] = [
  { key: 'period', header: 'Период', render: (row) => <>{formatPeriod(row.period)}</> },
  { key: 'sent', header: 'Отправлено' },
  { key: 'delivered', header: 'Доставлено' },
  { key: 'failed', header: 'Ошибки' },
  { key: 'delivery_rate', header: 'Доставляемость', render: (row) => <>{row.delivery_rate}%</> },
];

const countryColumns: Column<CountryEntry>[] = [
  { key: 'country', header: 'Страна' },
  { key: 'sent', header: 'Отправлено' },
  { key: 'delivered', header: 'Доставлено' },
  { key: 'failed', header: 'Ошибки' },
  { key: 'delivery_rate', header: 'Доставляемость', render: (row) => <>{row.delivery_rate}%</> },
];

export function AnalyticsPage() {
  const [data, setData] = useState<AnalyticsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const [period, setPeriod] = useState<string>('7d');
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');
  const [groupBy, setGroupBy] = useState('day');
  const [useCustomDates, setUseCustomDates] = useState(false);

  const loadAnalytics = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const params: Record<string, string> = { group_by: groupBy };
      if (useCustomDates && dateFrom && dateTo) {
        params.date_from = dateFrom;
        params.date_to = dateTo;
      } else {
        params.period = period;
      }
      const resp = await analyticsApi.get(params);
      setData(resp as AnalyticsData);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось загрузить аналитику');
    } finally {
      setLoading(false);
    }
  }, [period, dateFrom, dateTo, groupBy, useCustomDates]);

  useEffect(() => {
    loadAnalytics();
  }, [loadAnalytics]);

  return (
    <div className="max-w-5xl">
      <PageHeader title="Аналитика" />

      {/* Period selector + date filters */}
      <fieldset className="border-none p-0 mb-4">
        <legend className="font-bold mb-2">Фильтры</legend>
        <div className="flex gap-2 items-center flex-wrap">
          {PERIODS.map((p) => (
            <button
              key={p}
              type="button"
              onClick={() => {
                setPeriod(p);
                setUseCustomDates(false);
              }}
              className={`px-4 py-1.5 text-sm rounded border ${
                !useCustomDates && period === p
                  ? 'bg-primary text-white border-primary'
                  : 'bg-white text-gray-700 border-gray-300 hover:bg-gray-50'
              }`}
            >
              {p}
            </button>
          ))}

          <span className="mx-2 text-gray-600">или</span>

          <label className="flex items-center gap-1">
            С:
            <input
              type="date"
              value={dateFrom}
              onChange={(e) => {
                setDateFrom(e.target.value);
                setUseCustomDates(true);
              }}
              className="rounded border border-gray-300 px-2 py-1 text-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary"
            />
          </label>
          <label className="flex items-center gap-1">
            По:
            <input
              type="date"
              value={dateTo}
              onChange={(e) => {
                setDateTo(e.target.value);
                setUseCustomDates(true);
              }}
              className="rounded border border-gray-300 px-2 py-1 text-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary"
            />
          </label>

          <span className="mx-2 text-gray-600">|</span>

          <label className="flex items-center gap-1">
            Группировка:
            <select
              value={groupBy}
              onChange={(e) => setGroupBy(e.target.value)}
              className="rounded border border-gray-300 px-2 py-1 text-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary"
            >
              <option value="day">День</option>
              <option value="week">Неделя</option>
              <option value="country">Страна</option>
            </select>
          </label>
        </div>
      </fieldset>

      {error && <p className="text-red-600">{error}</p>}
      {loading && <div role="status">Загрузка аналитики...</div>}

      {data && !loading && (
        <>
          {/* Summary cards */}
          <div className="grid grid-cols-2 md:grid-cols-5 gap-4 mb-6">
            <StatCard title="Отправлено" value={data.summary.total_sent} />
            <StatCard title="Доставлено" value={data.summary.total_delivered} />
            <StatCard title="Ошибки" value={data.summary.total_failed} />
            <StatCard title="Доставляемость" value={`${data.summary.delivery_rate}%`} />
            <StatCard title="Стоимость" value={data.summary.total_cost ? new Intl.NumberFormat('ru-RU', { style: 'currency', currency: 'RUB', minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(parseFloat(data.summary.total_cost) || 0) : '—'} />
          </div>

          {/* Timeline table */}
          {data.timeline.length > 0 && (
            <div className="mb-6">
              <h3 className="text-lg font-semibold text-gray-900 mb-3">Хронология</h3>
              <DataTable<TimelineEntry>
                columns={timelineColumns}
                data={data.timeline}
                total={data.timeline.length}
                page={1}
                pageSize={data.timeline.length}
                onPageChange={() => {}}
                keyField="period"
                tableLabel="Хронология отправок по периодам"
              />
            </div>
          )}

          {/* Country breakdown table */}
          {data.by_country.length > 0 && (
            <div>
              <h3 className="text-lg font-semibold text-gray-900 mb-3">По странам</h3>
              <DataTable<CountryEntry>
                columns={countryColumns}
                data={data.by_country}
                total={data.by_country.length}
                page={1}
                pageSize={data.by_country.length}
                onPageChange={() => {}}
                keyField="country"
                tableLabel="Разбивка отправок по странам"
              />
            </div>
          )}
        </>
      )}
    </div>
  );
}
