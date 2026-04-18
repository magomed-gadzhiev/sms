import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import {
  LineChart, Line, XAxis, Tooltip, ResponsiveContainer, Legend,
  YAxis, CartesianGrid,
} from 'recharts';
import * as Tabs from '@radix-ui/react-tabs';
import { analyticsApi, ApiError, type AnalyticsDataExtended } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { StatCard } from '../../components/data/StatCard';
import { DataTable, type Column } from '../../components/data/DataTable';

type CountryEntry = { country: string; sent: number; delivered: number; failed: number; delivery_rate: number };
type TimelineEntry = { period: string; sent: number; delivered: number; failed: number; delivery_rate: number };

const PERIODS = ['7d', '30d', '90d'] as const;
const GROUP_BY_OPTIONS = ['day', 'week', 'country'] as const;
type Period = (typeof PERIODS)[number];
type GroupBy = (typeof GROUP_BY_OPTIONS)[number];
const MAX_CUSTOM_RANGE_DAYS = 366;

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
  { key: 'sent', header: 'Отправлено', sortable: true },
  { key: 'delivered', header: 'Доставлено', sortable: true },
  { key: 'failed', header: 'Ошибки', sortable: true },
  { key: 'delivery_rate', header: 'Доставляемость', render: (row) => <>{row.delivery_rate}%</>, sortable: true },
];

export function AnalyticsPage() {
  const [data, setData] = useState<AnalyticsDataExtended | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const [period, setPeriod] = useState<Period>('7d');
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');
  const [groupBy, setGroupBy] = useState<GroupBy>('day');
  const [useCustomDates, setUseCustomDates] = useState(false);
  const [compare, setCompare] = useState(false);
  const abortRef = useRef<AbortController | null>(null);
  const requestIdRef = useRef(0);

  const loadAnalytics = useCallback(async () => {
    const currentRequestId = ++requestIdRef.current;
    setLoading(true);
    setError('');

    const safeGroupBy: GroupBy = GROUP_BY_OPTIONS.includes(groupBy) ? groupBy : 'day';
    const safePeriod: Period = PERIODS.includes(period) ? period : '7d';

    if (useCustomDates) {
      if (!dateFrom || !dateTo) {
        setError('Для пользовательского периода укажите обе даты: «С» и «По».');
        setLoading(false);
        return;
      }
      if (dateFrom > dateTo) {
        setError('Дата «По» не может быть раньше даты «С».');
        setLoading(false);
        return;
      }
      const fromTs = Date.parse(`${dateFrom}T00:00:00Z`);
      const toTs = Date.parse(`${dateTo}T23:59:59Z`);
      if (Number.isNaN(fromTs) || Number.isNaN(toTs)) {
        setError('Неверный формат даты.');
        setLoading(false);
        return;
      }
      const rangeDays = Math.ceil((toTs - fromTs) / (24 * 60 * 60 * 1000));
      if (rangeDays > MAX_CUSTOM_RANGE_DAYS) {
        setError('Максимальный пользовательский диапазон — 366 дней.');
        setLoading(false);
        return;
      }
    }

    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    try {
      const params: Record<string, string> = { group_by: safeGroupBy, include_cost: 'true' };
      if (useCustomDates && dateFrom && dateTo) {
        params.date_from = dateFrom;
        params.date_to = dateTo;
      } else {
        params.period = safePeriod;
      }
      if (compare) params.compare = 'true';
      const resp = await analyticsApi.get(params, { signal: controller.signal });
      if (currentRequestId !== requestIdRef.current) return;
      setData(resp as AnalyticsDataExtended);
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') return;
      if (currentRequestId !== requestIdRef.current) return;
      setError(err instanceof ApiError ? err.message : 'Не удалось загрузить аналитику');
    } finally {
      if (currentRequestId === requestIdRef.current) {
        setLoading(false);
      }
    }
  }, [period, dateFrom, dateTo, groupBy, useCustomDates, compare]);

  useEffect(() => {
    loadAnalytics();
    return () => {
      abortRef.current?.abort();
    };
  }, [loadAnalytics]);

  const timeline = (data?.timeline ?? []) as TimelineEntry[];
  const prevTimeline = data?.previous_timeline ?? [];
  const byCountry = (data?.by_country ?? []) as CountryEntry[];

  const mergedTimeline = useMemo(() => {
    const prevByPeriod = new Map(prevTimeline.map((entry) => [entry.period, entry]));
    return timeline.map((entry) => {
      const prev = prevByPeriod.get(entry.period);
      return {
        ...entry,
        prev_sent: prev?.sent,
        prev_delivered: prev?.delivered,
      };
    });
  }, [timeline, prevTimeline]);

  return (
    <div className="max-w-5xl">
      <PageHeader title="Аналитика" />

      {/* Filters */}
      <fieldset className="border-none p-0 mb-4">
        <legend className="font-bold mb-2">Фильтры</legend>
        <div className="flex gap-2 items-center flex-wrap">
          {PERIODS.map((p) => (
            <button
              key={p}
              type="button"
              onClick={() => { setPeriod(p); setUseCustomDates(false); }}
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
            <input type="date" value={dateFrom} onChange={(e) => { setDateFrom(e.target.value); setUseCustomDates(true); }}
              className="rounded border border-gray-300 px-2 py-1 text-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary" />
          </label>
          <label className="flex items-center gap-1">
            По:
            <input type="date" value={dateTo} onChange={(e) => { setDateTo(e.target.value); setUseCustomDates(true); }}
              className="rounded border border-gray-300 px-2 py-1 text-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary" />
          </label>
          <span className="mx-2 text-gray-600">|</span>
          <label className="flex items-center gap-1">
            Группировка:
            <select value={groupBy} onChange={(e) => setGroupBy((e.target.value as GroupBy))}
              className="rounded border border-gray-300 px-2 py-1 text-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary">
              <option value="day">День</option>
              <option value="week">Неделя</option>
              <option value="country">Страна</option>
            </select>
          </label>
          <label className="flex items-center gap-2 ml-2 text-sm cursor-pointer">
            <input type="checkbox" checked={compare} onChange={(e) => setCompare(e.target.checked)} className="rounded" />
            Сравнить с предыдущим периодом
          </label>
        </div>
      </fieldset>

      {error && <p role="alert" className="text-red-600">{error}</p>}
      {loading && <div role="status" aria-live="polite">Загрузка аналитики...</div>}

      {data && !loading && (
        <>
          {/* Summary cards */}
          <div className="grid grid-cols-2 md:grid-cols-5 gap-4 mb-6">
            <StatCard title="Отправлено" value={data.summary.total_sent} />
            <StatCard title="Доставлено" value={data.summary.total_delivered} />
            <StatCard title="Ошибки" value={data.summary.total_failed} />
            <StatCard title="Доставляемость" value={`${data.summary.delivery_rate}%`} />
            <StatCard title="Стоимость" value={
              data.summary.total_cost
                ? new Intl.NumberFormat('ru-RU', { style: 'currency', currency: 'RUB', minimumFractionDigits: 2 }).format(parseFloat(data.summary.total_cost) || 0)
                : '—'
            } />
          </div>

          {/* Tabs */}
          <Tabs.Root defaultValue="timeline">
            <Tabs.List className="flex gap-1 mb-4 border-b border-gray-200">
              {['timeline', 'countries'].map((tab) => (
                <Tabs.Trigger
                  key={tab}
                  value={tab}
                  className="px-4 py-2 text-sm text-gray-600 hover:text-gray-900 data-[state=active]:border-b-2 data-[state=active]:border-primary data-[state=active]:text-primary -mb-px"
                >
                  {tab === 'timeline' ? 'Хронология' : 'По странам'}
                </Tabs.Trigger>
              ))}
            </Tabs.List>

            {/* Timeline tab */}
            <Tabs.Content value="timeline">
              {mergedTimeline.length > 0 ? (
                <div className="mb-6">
                  <div className="border border-gray-200 rounded-lg p-4 mb-4">
                    <ResponsiveContainer width="100%" height={220}>
                      <LineChart data={mergedTimeline}>
                        <XAxis dataKey="period" tick={{ fontSize: 11 }} tickFormatter={(v: string) => (typeof v === 'string' && v.length > 5 ? v.slice(5) : v)} />
                        <YAxis tick={{ fontSize: 11 }} />
                        <CartesianGrid strokeDasharray="3 3" />
                        <Tooltip />
                        <Legend />
                        <Line type="monotone" dataKey="sent" stroke="#3B82F6" strokeWidth={2} dot={false} name="Отправлено" />
                        <Line type="monotone" dataKey="delivered" stroke="#10B981" strokeWidth={2} dot={false} name="Доставлено" />
                        {compare && prevTimeline.length > 0 && (
                          <>
                            <Line type="monotone" dataKey="prev_sent" stroke="#93C5FD" strokeWidth={1.5} strokeDasharray="4 2" dot={false} name="Отправлено (пред.)" />
                            <Line type="monotone" dataKey="prev_delivered" stroke="#6EE7B7" strokeWidth={1.5} strokeDasharray="4 2" dot={false} name="Доставлено (пред.)" />
                          </>
                        )}
                      </LineChart>
                    </ResponsiveContainer>
                  </div>
                  <DataTable<TimelineEntry>
                    columns={timelineColumns}
                    data={timeline}
                    total={timeline.length}
                    page={1}
                    pageSize={timeline.length}
                    onPageChange={() => {}}
                    keyField="period"
                    tableLabel="Хронология отправок"
                  />
                </div>
              ) : (
                <div className="py-8 text-center text-gray-500 text-sm">
                  Нет данных за выбранный период
                </div>
              )}
            </Tabs.Content>

            {/* Countries tab */}
            <Tabs.Content value="countries">
              {byCountry.length > 0 ? (
                <DataTable<CountryEntry>
                  columns={countryColumns}
                  data={byCountry}
                  total={byCountry.length}
                  page={1}
                  pageSize={byCountry.length}
                  onPageChange={() => {}}
                  keyField="country"
                  tableLabel="Разбивка по странам"
                />
              ) : (
                <div className="py-8 text-center text-gray-500 text-sm">
                  Выберите группировку «Страна» для просмотра разбивки
                </div>
              )}
            </Tabs.Content>
          </Tabs.Root>
        </>
      )}
    </div>
  );
}
