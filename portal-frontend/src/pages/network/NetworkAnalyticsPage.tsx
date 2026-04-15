import { useState, useEffect } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { useToast } from '../../components/ui/Toast';
import { resellerApi, subAccountsApi, ApiError } from '../../api/client';

type Period = '7d' | '30d' | '90d';

interface AnalyticsData {
  summary: { total_sent: number; total_delivered: number; total_failed: number; delivery_rate: number };
  timeline: { period: string; sent: number; delivered: number; failed: number; delivery_rate: number }[];
  by_sub_account: { id: string; name: string; total_sent: number; total_delivered: number; total_failed: number; delivery_rate: number }[];
  period: string;
}

interface SubAccountOption {
  id: string;
  name: string;
}

function formatNumber(n: number): string {
  return n.toLocaleString('ru-RU');
}

export function NetworkAnalyticsPage() {
  const toast = useToast();
  const [period, setPeriod] = useState<Period>('7d');
  const [subFilter, setSubFilter] = useState('');
  const [subAccounts, setSubAccounts] = useState<SubAccountOption[]>([]);
  const [data, setData] = useState<AnalyticsData | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    subAccountsApi.list().then((r: any) => {
      setSubAccounts((r.sub_accounts || []).map((sa: any) => ({ id: sa.id, name: sa.name || sa.email })));
    }).catch(() => {});
  }, []);

  useEffect(() => {
    setLoading(true);
    resellerApi.getNetworkAnalytics({
      period,
      sub_account_id: subFilter || undefined,
      group_by: 'day',
    })
      .then((r) => setData(r as AnalyticsData))
      .catch((e) => toast.error(e instanceof ApiError ? e.message : 'Ошибка загрузки'))
      .finally(() => setLoading(false));
  }, [period, subFilter]);

  return (
    <div className="max-w-6xl">
      <PageHeader
        title="Аналитика сети"
        actions={
          <div className="flex gap-2 items-center">
            <select
              value={subFilter}
              onChange={(e) => setSubFilter(e.target.value)}
              className="border border-gray-300 rounded px-2 py-1.5 text-sm"
            >
              <option value="">Все субаккаунты</option>
              {subAccounts.map((sa) => (
                <option key={sa.id} value={sa.id}>{sa.name}</option>
              ))}
            </select>
            <div className="flex gap-1 bg-gray-100 rounded-lg p-0.5">
              {(['7d', '30d', '90d'] as Period[]).map((p) => (
                <button
                  key={p}
                  onClick={() => setPeriod(p)}
                  className={`px-3 py-1.5 text-sm rounded-md transition-colors ${
                    period === p ? 'bg-white text-gray-900 shadow-sm font-medium' : 'text-gray-500 hover:text-gray-700'
                  }`}
                >
                  {p === '7d' ? '7 дней' : p === '30d' ? '30 дней' : '90 дней'}
                </button>
              ))}
            </div>
          </div>
        }
      />

      {loading ? (
        <div className="py-8 text-center text-gray-400">Загрузка аналитики...</div>
      ) : !data ? null : (
        <>
          {/* Summary cards */}
          <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
            <div className="bg-white rounded-lg border border-gray-200 p-4">
              <div className="text-xs text-gray-500 uppercase mb-1">Отправлено</div>
              <div className="text-2xl font-bold">{formatNumber(data.summary.total_sent)}</div>
            </div>
            <div className="bg-white rounded-lg border border-gray-200 p-4">
              <div className="text-xs text-gray-500 uppercase mb-1">Доставлено</div>
              <div className="text-2xl font-bold text-green-600">{formatNumber(data.summary.total_delivered)}</div>
            </div>
            <div className="bg-white rounded-lg border border-gray-200 p-4">
              <div className="text-xs text-gray-500 uppercase mb-1">Ошибки</div>
              <div className="text-2xl font-bold text-red-500">{formatNumber(data.summary.total_failed)}</div>
            </div>
            <div className="bg-white rounded-lg border border-gray-200 p-4">
              <div className="text-xs text-gray-500 uppercase mb-1">Delivery Rate</div>
              <div className={`text-2xl font-bold ${data.summary.delivery_rate >= 90 ? 'text-green-600' : data.summary.delivery_rate >= 70 ? 'text-yellow-600' : 'text-red-600'}`}>
                {data.summary.delivery_rate.toFixed(1)}%
              </div>
            </div>
          </div>

          {/* Timeline table */}
          {data.timeline.length > 0 && (
            <div className="bg-white rounded-lg border border-gray-200 mb-6">
              <h3 className="text-sm font-semibold text-gray-700 p-4 border-b border-gray-200">Трафик по дням</h3>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="bg-gray-50 text-gray-500 text-xs uppercase">
                      <th className="text-left p-3">Период</th>
                      <th className="text-right p-3">Отправлено</th>
                      <th className="text-right p-3">Доставлено</th>
                      <th className="text-right p-3">Ошибки</th>
                      <th className="text-right p-3">DR</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.timeline.map((row) => (
                      <tr key={row.period} className="border-t border-gray-100">
                        <td className="p-3">{row.period}</td>
                        <td className="p-3 text-right">{formatNumber(row.sent)}</td>
                        <td className="p-3 text-right text-green-600">{formatNumber(row.delivered)}</td>
                        <td className="p-3 text-right text-red-500">{formatNumber(row.failed)}</td>
                        <td className="p-3 text-right">{row.delivery_rate.toFixed(1)}%</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {/* Per sub-account breakdown */}
          {!subFilter && data.by_sub_account.length > 0 && (
            <div className="bg-white rounded-lg border border-gray-200">
              <h3 className="text-sm font-semibold text-gray-700 p-4 border-b border-gray-200">По субаккаунтам</h3>
              <table className="w-full text-sm">
                <thead>
                  <tr className="bg-gray-50 text-gray-500 text-xs uppercase">
                    <th className="text-left p-3">Субаккаунт</th>
                    <th className="text-right p-3">Отправлено</th>
                    <th className="text-right p-3">Доставлено</th>
                    <th className="text-right p-3">Ошибки</th>
                    <th className="text-right p-3">DR</th>
                  </tr>
                </thead>
                <tbody>
                  {data.by_sub_account.map((sa) => (
                    <tr key={sa.id} className="border-t border-gray-100">
                      <td className="p-3 font-medium">{sa.name}</td>
                      <td className="p-3 text-right">{formatNumber(sa.total_sent)}</td>
                      <td className="p-3 text-right text-green-600">{formatNumber(sa.total_delivered)}</td>
                      <td className="p-3 text-right text-red-500">{formatNumber(sa.total_failed)}</td>
                      <td className={`p-3 text-right ${sa.delivery_rate >= 90 ? 'text-green-600' : sa.delivery_rate >= 70 ? 'text-yellow-600' : 'text-red-600'}`}>
                        {sa.delivery_rate.toFixed(1)}%
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}
    </div>
  );
}
