import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { PageHeader } from '../../components/layout/PageHeader';
import { usePageTitle } from '../../contexts/PageTitleContext';
import { apiFetch, ApiError } from '../../api/client';

interface NetworkBalance {
  own_balance: string;
  sub_balance: string;
  total: string;
  currency: string;
}

interface TrafficData {
  total_sent: number;
  total_delivered: number;
  total_failed: number;
  delivery_rate: number;
}

interface ModerationCounts {
  sender_names: number;
  templates: number;
  registrations: number;
}

interface TopSubAccount {
  id: string;
  name: string;
  total_sent: number;
  delivery_rate: number;
}

interface ProblemAlert {
  sub_account_id: string;
  name: string;
  type: string;
  detail: string;
}

interface DashboardData {
  network_balance: NetworkBalance;
  traffic: TrafficData;
  moderation_counts: ModerationCounts;
  top_sub_accounts: TopSubAccount[];
  problem_sub_accounts: ProblemAlert[];
  period: string;
  revenue: { available: boolean; message: string };
}

type Period = 'today' | '7d' | '30d';

function MetricCard({ title, children, className }: { title: string; children: React.ReactNode; className?: string }) {
  return (
    <div className={`bg-white dark:bg-slate-900 rounded-lg border border-gray-200 dark:border-slate-700 p-5 ${className || ''}`}>
      <h3 className="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-slate-400 mb-3">{title}</h3>
      {children}
    </div>
  );
}

function formatNumber(n: number): string {
  return n.toLocaleString('ru-RU');
}

export function NetworkDashboardPage() {
  usePageTitle('Дашборд сети');
  const [data, setData] = useState<DashboardData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [period, setPeriod] = useState<Period>('today');

  useEffect(() => {
    setLoading(true);
    setError('');
    apiFetch<DashboardData>(`/reseller/dashboard?period=${period}`)
      .then(setData)
      .catch((e) => setError(e instanceof ApiError ? e.message : 'Ошибка загрузки'))
      .finally(() => setLoading(false));
  }, [period]);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16">
        <div className="w-8 h-8 border-2 border-primary border-t-transparent rounded-full animate-spin" />
        <span className="ml-3 text-sm text-gray-500 dark:text-slate-400">Загрузка дашборда...</span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="max-w-5xl">
        <div className="bg-red-50 dark:bg-red-950/40 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 rounded p-4">{error}</div>
      </div>
    );
  }

  if (!data) return null;

  const moderationTotal = data.moderation_counts.sender_names + data.moderation_counts.templates + data.moderation_counts.registrations;

  return (
    <div className="max-w-6xl">
      <PageHeader
        actions={
          <div className="flex gap-1 bg-gray-100 dark:bg-slate-800 rounded-lg p-0.5">
            {(['today', '7d', '30d'] as Period[]).map((p) => (
              <button
                key={p}
                onClick={() => setPeriod(p)}
                className={`px-3 py-1.5 text-sm rounded-md transition-colors ${
                  period === p ? 'bg-white dark:bg-slate-900 text-gray-900 dark:text-slate-100 shadow-sm font-medium' : 'text-gray-500 dark:text-slate-400 hover:text-gray-700 dark:hover:text-slate-200'
                }`}
              >
                {p === 'today' ? 'Сегодня' : p === '7d' ? '7 дней' : '30 дней'}
              </button>
            ))}
          </div>
        }
      />

      {/* Row 1: Balance + Traffic + Delivery Rate */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
        <MetricCard title="Баланс сети">
          <div className="text-2xl font-bold text-gray-900 dark:text-slate-100">
            {parseFloat(data.network_balance.total).toLocaleString('ru-RU', { minimumFractionDigits: 2 })} {data.network_balance.currency || '₽'}
          </div>
          <div className="mt-2 flex gap-4 text-sm text-gray-500 dark:text-slate-400">
            <span>Свой: {parseFloat(data.network_balance.own_balance).toLocaleString('ru-RU', { minimumFractionDigits: 2 })}</span>
            <span>Сеть: {parseFloat(data.network_balance.sub_balance).toLocaleString('ru-RU', { minimumFractionDigits: 2 })}</span>
          </div>
        </MetricCard>

        <MetricCard title="Трафик">
          <div className="text-2xl font-bold text-gray-900 dark:text-slate-100">{formatNumber(data.traffic.total_sent)}</div>
          <div className="mt-2 flex gap-3 text-sm">
            <span className="text-green-600 dark:text-green-400">Доставлено: {formatNumber(data.traffic.total_delivered)}</span>
            <span className="text-red-500 dark:text-red-400">Ошибки: {formatNumber(data.traffic.total_failed)}</span>
          </div>
        </MetricCard>

        <MetricCard title="Delivery Rate">
          <div className={`text-2xl font-bold ${data.traffic.delivery_rate >= 90 ? 'text-green-600 dark:text-green-400' : data.traffic.delivery_rate >= 70 ? 'text-yellow-600 dark:text-yellow-400' : 'text-red-600 dark:text-red-400'}`}>
            {data.traffic.delivery_rate.toFixed(1)}%
          </div>
          <div className="mt-2 text-sm text-gray-500 dark:text-slate-400">
            {data.traffic.delivery_rate >= 90 ? 'Отличный показатель' : data.traffic.delivery_rate >= 70 ? 'Нормальный показатель' : 'Требует внимания'}
          </div>
        </MetricCard>
      </div>

      {/* Row 2: Moderation + Revenue */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
        <MetricCard title="Модерация">
          {moderationTotal === 0 ? (
            <div className="text-sm text-gray-400 dark:text-slate-500">Нет ожидающих заявок</div>
          ) : (
            <Link to="/network/moderation" className="block">
              <div className="text-2xl font-bold text-amber-600 mb-2">{moderationTotal} {moderationTotal % 10 === 1 && moderationTotal % 100 !== 11 ? 'заявка' : moderationTotal % 10 >= 2 && moderationTotal % 10 <= 4 && (moderationTotal % 100 < 10 || moderationTotal % 100 >= 20) ? 'заявки' : 'заявок'}</div>
              <div className="flex gap-4 text-sm text-gray-600 dark:text-slate-400">
                {data.moderation_counts.sender_names > 0 && <span>Имена: {data.moderation_counts.sender_names}</span>}
                {data.moderation_counts.templates > 0 && <span>Шаблоны: {data.moderation_counts.templates}</span>}
                {data.moderation_counts.registrations > 0 && <span>Регистрации: {data.moderation_counts.registrations}</span>}
              </div>
            </Link>
          )}
        </MetricCard>

        <MetricCard title="Выручка сети">
          <div className="text-sm text-gray-400 dark:text-slate-500">{data.revenue.message}</div>
        </MetricCard>
      </div>

      {/* Row 3: Top 5 + Problems */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <MetricCard title="Топ-5 субаккаунтов по трафику" className="min-h-[200px]">
          {data.top_sub_accounts.length === 0 ? (
            <div className="text-sm text-gray-400 dark:text-slate-500">Нет данных за текущий месяц</div>
          ) : (
            <table className="w-full text-sm">
              <thead>
                <tr className="text-gray-500 dark:text-slate-400 text-xs uppercase">
                  <th className="text-left pb-2">Субаккаунт</th>
                  <th className="text-right pb-2">SMS</th>
                  <th className="text-right pb-2">DR</th>
                </tr>
              </thead>
              <tbody>
                {data.top_sub_accounts.map((sa) => (
                  <tr key={sa.id} className="border-t border-gray-100 dark:border-slate-800">
                    <td className="py-1.5">
                      <Link to={`/network/sub-accounts/${sa.id}`} className="text-primary hover:underline">
                        {sa.name}
                      </Link>
                    </td>
                    <td className="text-right py-1.5 text-gray-700 dark:text-slate-300">{formatNumber(sa.total_sent)}</td>
                    <td className={`text-right py-1.5 ${sa.delivery_rate >= 90 ? 'text-green-600 dark:text-green-400' : sa.delivery_rate >= 70 ? 'text-yellow-600 dark:text-yellow-400' : 'text-red-600 dark:text-red-400'}`}>
                      {sa.delivery_rate.toFixed(1)}%
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </MetricCard>

        <MetricCard title="Проблемные субаккаунты" className="min-h-[200px]">
          {data.problem_sub_accounts.length === 0 ? (
            <div className="text-sm text-green-600 dark:text-green-400">Все субаккаунты в норме</div>
          ) : (
            <ul className="space-y-2">
              {data.problem_sub_accounts.map((p, i) => (
                <li key={`${p.sub_account_id}-${i}`} className="flex items-start gap-2 text-sm">
                  <span className={`mt-0.5 flex-shrink-0 w-2 h-2 rounded-full ${p.type === 'low_balance' ? 'bg-amber-400' : 'bg-red-400'}`} />
                  <div>
                    <Link to={`/network/sub-accounts/${p.sub_account_id}`} className="font-medium text-gray-900 dark:text-slate-100 hover:text-primary">
                      {p.name}
                    </Link>
                    <span className="text-gray-500 dark:text-slate-400 ml-1">— {p.detail}</span>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </MetricCard>
      </div>
    </div>
  );
}
