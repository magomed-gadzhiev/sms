import { useState, useEffect } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { usePageTitle } from '../../contexts/PageTitleContext';
import { useToast } from '../../components/ui/Toast';
import { quotaApi, ApiError } from '../../api/client';
import type { QuotaData, QuotaHistoryEntry } from '../../api/client';

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });
}

function formatNumber(n: number): string {
  return n.toLocaleString('ru-RU');
}

function getProgressColor(pct: number): string {
  if (pct >= 100) return 'bg-red-500';
  if (pct >= 80) return 'bg-yellow-400';
  return 'bg-green-500';
}

function getProgressTextColor(pct: number): string {
  if (pct >= 100) return 'text-red-600 dark:text-red-400';
  if (pct >= 80) return 'text-yellow-600 dark:text-yellow-400';
  return 'text-green-600 dark:text-green-400';
}

interface QuotaCardProps {
  quota: QuotaData;
}

function QuotaCard({ quota }: QuotaCardProps) {
  const usedPct = quota.segment_limit > 0
    ? Math.min((quota.segments_used / quota.segment_limit) * 100, 100)
    : 0;
  const displayPct = quota.segment_limit > 0
    ? (quota.segments_used / quota.segment_limit) * 100
    : 0;

  return (
    <div className="bg-white dark:bg-slate-900 rounded-lg border border-gray-200 dark:border-slate-700 p-6">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-base font-semibold text-gray-800 dark:text-slate-200">Текущая квота</h2>
        <span className="text-xs text-gray-500 dark:text-slate-400">
          {formatDate(quota.period_start)} — {formatDate(quota.period_end)}
        </span>
      </div>

      {/* Main stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        <div>
          <div className="text-xs text-gray-500 dark:text-slate-400 uppercase mb-1">Лимит сегментов</div>
          <div className="text-2xl font-bold text-gray-900 dark:text-slate-100">{formatNumber(quota.segment_limit)}</div>
        </div>
        <div>
          <div className="text-xs text-gray-500 dark:text-slate-400 uppercase mb-1">Использовано</div>
          <div className={`text-2xl font-bold ${getProgressTextColor(displayPct)}`}>
            {formatNumber(quota.segments_used)}
          </div>
        </div>
        <div>
          <div className="text-xs text-gray-500 dark:text-slate-400 uppercase mb-1">Утилизация</div>
          <div className={`text-2xl font-bold ${getProgressTextColor(displayPct)}`}>
            {displayPct.toFixed(1)}%
          </div>
        </div>
        <div>
          <div className="text-xs text-gray-500 dark:text-slate-400 uppercase mb-1">Осталось</div>
          <div className="text-2xl font-bold text-gray-700 dark:text-slate-300">
            {quota.segments_used < quota.segment_limit
              ? formatNumber(quota.segment_limit - quota.segments_used)
              : '0'}
          </div>
        </div>
      </div>

      {/* Progress bar */}
      <div className="mb-4">
        <div className="flex justify-between text-xs text-gray-500 dark:text-slate-400 mb-1">
          <span>0</span>
          <span>{formatNumber(quota.segment_limit)}</span>
        </div>
        <div className="h-3 bg-gray-100 dark:bg-slate-800 rounded-full overflow-hidden">
          <div
            className={`h-full rounded-full transition-all ${getProgressColor(displayPct)}`}
            style={{ width: `${usedPct}%` }}
          />
        </div>
      </div>

      {/* Overage section */}
      {quota.overage_segments > 0 && (
        <div className="mt-4 p-3 bg-red-50 dark:bg-red-950/40 border border-red-200 dark:border-red-800 rounded-md flex items-start gap-2">
          <svg className="w-4 h-4 text-red-500 dark:text-red-400 mt-0.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" />
          </svg>
          <div>
            <p className="text-sm font-medium text-red-700 dark:text-red-300">
              Превышение квоты: {formatNumber(quota.overage_segments)} сегментов
            </p>
            <p className="text-xs text-red-600 dark:text-red-400 mt-0.5">
              Тариф за превышение: {quota.overage_rate} {quota.currency} / сегмент
            </p>
          </div>
        </div>
      )}

      {/* Overage rate info */}
      {quota.overage_segments === 0 && quota.overage_rate && Number(quota.overage_rate) > 0 && (
        <div className="mt-4 text-xs text-gray-500 dark:text-slate-400">
          Тариф за превышение квоты: {quota.overage_rate} {quota.currency} / сегмент
        </div>
      )}
    </div>
  );
}

interface HistoryTableProps {
  history: QuotaHistoryEntry[];
}

function HistoryTable({ history }: HistoryTableProps) {
  if (history.length === 0) return null;

  return (
    <div className="bg-white dark:bg-slate-900 rounded-lg border border-gray-200 dark:border-slate-700 mt-6">
      <h3 className="text-sm font-semibold text-gray-700 dark:text-slate-300 p-4 border-b border-gray-200 dark:border-slate-700">История квот</h3>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-gray-50 dark:bg-slate-950 text-gray-500 dark:text-slate-400 text-xs uppercase">
              <th className="text-left p-3">Период</th>
              <th className="text-right p-3">Лимит</th>
              <th className="text-right p-3">Использовано</th>
              <th className="text-right p-3">Превышение</th>
              <th className="text-right p-3">Тариф превышения</th>
            </tr>
          </thead>
          <tbody>
            {history.map((row) => {
              const pct = row.segment_limit > 0
                ? (row.segments_used / row.segment_limit) * 100
                : 0;
              return (
                <tr key={row.id} className="border-t border-gray-100 dark:border-slate-800 hover:bg-gray-50 dark:hover:bg-slate-800">
                  <td className="p-3 text-gray-700 dark:text-slate-300">
                    {formatDate(row.period_start)} — {formatDate(row.period_end)}
                  </td>
                  <td className="p-3 text-right">{formatNumber(row.segment_limit)}</td>
                  <td className={`p-3 text-right font-medium ${getProgressTextColor(pct)}`}>
                    {formatNumber(row.segments_used)}
                    <span className="text-xs text-gray-400 dark:text-slate-500 ml-1">({pct.toFixed(0)}%)</span>
                  </td>
                  <td className="p-3 text-right">
                    {row.overage_segments > 0 ? (
                      <span className="text-red-600 dark:text-red-400 font-medium">{formatNumber(row.overage_segments)}</span>
                    ) : (
                      <span className="text-gray-400 dark:text-slate-500">—</span>
                    )}
                  </td>
                  <td className="p-3 text-right text-gray-600 dark:text-slate-400">
                    {row.overage_rate} {row.currency}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

export function NetworkQuotaPage() {
  usePageTitle('Квота сегментов');
  const toast = useToast();
  const [quota, setQuota] = useState<QuotaData | null>(null);
  const [history, setHistory] = useState<QuotaHistoryEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [showHistory, setShowHistory] = useState(false);
  const [historyLoading, setHistoryLoading] = useState(false);

  useEffect(() => {
    setLoading(true);
    quotaApi.getMyQuota()
      .then((r) => setQuota(r.quota))
      .catch((e) => toast.error(e instanceof ApiError ? e.message : 'Ошибка загрузки квоты'))
      .finally(() => setLoading(false));
  }, []);

  function handleLoadHistory() {
    if (showHistory) {
      setShowHistory(false);
      return;
    }
    setHistoryLoading(true);
    quotaApi.getQuotaHistory()
      .then((r) => {
        setHistory(r.history || []);
        setShowHistory(true);
      })
      .catch((e) => toast.error(e instanceof ApiError ? e.message : 'Ошибка загрузки истории'))
      .finally(() => setHistoryLoading(false));
  }

  return (
    <div className="max-w-4xl">
      <PageHeader
        actions={
          <button
            onClick={handleLoadHistory}
            disabled={historyLoading}
            className="px-3 py-1.5 text-sm border border-gray-300 dark:border-slate-600 rounded-md hover:bg-gray-50 dark:hover:bg-slate-800 text-gray-700 dark:text-slate-300 disabled:opacity-50"
          >
            {historyLoading ? 'Загрузка...' : showHistory ? 'Скрыть историю' : 'История квот'}
          </button>
        }
      />

      {loading ? (
        <div className="py-12 text-center text-gray-400 dark:text-slate-500">Загрузка квоты...</div>
      ) : quota === null ? (
        <div className="bg-white dark:bg-slate-900 rounded-lg border border-gray-200 dark:border-slate-700 p-12 text-center">
          <div className="text-gray-400 dark:text-slate-500 text-4xl mb-3">📊</div>
          <p className="text-gray-600 dark:text-slate-400 font-medium">Активная квота не назначена</p>
          <p className="text-sm text-gray-400 dark:text-slate-500 mt-1">Обратитесь к менеджеру для подключения тарифа с квотой сегментов</p>
        </div>
      ) : (
        <QuotaCard quota={quota} />
      )}

      {showHistory && <HistoryTable history={history} />}
    </div>
  );
}
