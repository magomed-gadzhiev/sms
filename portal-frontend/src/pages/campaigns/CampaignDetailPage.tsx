import { useState, useEffect, useCallback, useMemo } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  campaignsApi,
  type Campaign,
  type CampaignStats,
} from '../../api/campaigns';
import { ApiError } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Badge } from '../../components/ui/Badge';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { ABStatusPanel } from '../../components/campaigns/ABStatusPanel';
import { useToast } from '../../components/ui/Toast';

const STATUS_CONFIG: Record<
  string,
  { variant: 'default' | 'info' | 'warning' | 'success' | 'danger'; label: string }
> = {
  draft: { variant: 'default', label: 'Черновик' },
  scheduled: { variant: 'default', label: 'Запланирована' },
  materializing: { variant: 'warning', label: 'Подготовка' },
  running: { variant: 'info', label: 'Запущена' },
  paused: { variant: 'warning', label: 'На паузе' },
  completed: { variant: 'success', label: 'Завершена' },
  cancelled: { variant: 'danger', label: 'Отменена' },
};

interface StatCardProps {
  label: string;
  value: string | number;
  subtext?: string;
  color?: string;
}

function StatCard({ label, value, subtext, color = 'text-gray-900' }: StatCardProps) {
  return (
    <div className="bg-white border border-gray-200 rounded-lg p-4">
      <div className="text-sm text-gray-500 mb-1">{label}</div>
      <div className={`text-2xl font-semibold ${color}`}>
        {typeof value === 'number' ? value.toLocaleString() : value}
      </div>
      {subtext && <div className="text-xs text-gray-400 mt-1">{subtext}</div>}
    </div>
  );
}

export function CampaignDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const toast = useToast();

  const [campaign, setCampaign] = useState<Campaign | null>(null);
  const [stats, setStats] = useState<CampaignStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [actionLoading, setActionLoading] = useState('');

  // Tab navigation
  const [activeTab, setActiveTab] = useState<'overview' | 'ab_results'>('overview');

  // A/B comparison data
  const [variantComparison, setVariantComparison] = useState<{
    rows: { variant_id: string; variant_name: string; sent: number; delivered: number; delivery_rate: number }[];
    winner_variant_id: string;
  } | null>(null);
  const [selectingWinner, setSelectingWinner] = useState('');

  // Cancel dialog
  const [showCancel, setShowCancel] = useState(false);
  const [cancelling, setCancelling] = useState(false);

  const loadCampaign = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError('');
    try {
      const [campaignData, statsData] = await Promise.all([
        campaignsApi.get(id),
        campaignsApi.getStats(id).catch(() => null),
      ]);
      setCampaign(campaignData);
      setStats(statsData);
      if (campaignData.variants && campaignData.variants.length > 0) {
        campaignsApi.getVariantComparison(id).then(setVariantComparison).catch(() => {});
      }
    } catch (err) {
      setError(
        err instanceof ApiError ? err.message : 'Не удалось загрузить рассылку',
      );
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    loadCampaign();
  }, [loadCampaign]);

  // Poll for running/materializing campaigns
  useEffect(() => {
    if (!campaign || (campaign.status !== 'running' && campaign.status !== 'materializing')) return;
    const interval = setInterval(() => {
      if (!id) return;
      campaignsApi.get(id).then(setCampaign).catch(() => {});
      campaignsApi
        .getStats(id)
        .then(setStats)
        .catch(() => {});
    }, 5000);
    return () => clearInterval(interval);
  }, [campaign?.status, id]);

  async function handleAction(
    action: 'launch' | 'pause' | 'resume' | 'retry',
  ) {
    if (!id) return;
    setActionLoading(action);
    try {
      let result: Campaign;
      switch (action) {
        case 'launch':
          result = await campaignsApi.launch(id);
          break;
        case 'pause':
          result = await campaignsApi.pause(id);
          break;
        case 'resume':
          result = await campaignsApi.resume(id);
          break;
        case 'retry':
          result = await campaignsApi.retryFailed(id);
          break;
      }
      setCampaign(result);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка при выполнении действия');
    } finally {
      setActionLoading('');
    }
  }

  async function handleSelectWinner(variantId: string) {
    if (!id) return;
    setSelectingWinner(variantId);
    try {
      const result = await campaignsApi.selectWinner(id, variantId);
      setCampaign(result);
      campaignsApi.getVariantComparison(id).then(setVariantComparison).catch(() => {});
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка при выборе победителя');
    } finally {
      setSelectingWinner('');
    }
  }

  async function handleCancel() {
    if (!id) return;
    setCancelling(true);
    try {
      const result = await campaignsApi.cancel(id);
      setCampaign(result);
      setShowCancel(false);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка при отмене рассылки');
    } finally {
      setCancelling(false);
    }
  }

  const renderedActions = useMemo(() => {
    if (!campaign) return null;
    const buttons: React.ReactNode[] = [];

    if (campaign.status === 'draft') {
      buttons.push(
        <Button
          key="launch"
          onClick={() => handleAction('launch')}
          disabled={!!actionLoading}
        >
          {actionLoading === 'launch' ? 'Запуск...' : 'Запустить'}
        </Button>,
      );
    }
    if (campaign.status === 'running') {
      buttons.push(
        <Button
          key="pause"
          variant="secondary"
          onClick={() => handleAction('pause')}
          disabled={!!actionLoading}
        >
          {actionLoading === 'pause' ? '...' : 'Пауза'}
        </Button>,
      );
    }
    if (campaign.status === 'paused') {
      buttons.push(
        <Button
          key="resume"
          onClick={() => handleAction('resume')}
          disabled={!!actionLoading}
        >
          {actionLoading === 'resume' ? '...' : 'Возобновить'}
        </Button>,
      );
    }
    if (
      campaign.status === 'completed' &&
      campaign.failed_count > 0
    ) {
      buttons.push(
        <Button
          key="retry"
          variant="secondary"
          onClick={() => handleAction('retry')}
          disabled={!!actionLoading}
        >
          {actionLoading === 'retry' ? '...' : 'Повторить неудачные'}
        </Button>,
      );
    }
    if (
      campaign.status === 'running' ||
      campaign.status === 'paused'
    ) {
      buttons.push(
        <Button
          key="cancel"
          variant="danger"
          onClick={() => setShowCancel(true)}
        >
          Отменить
        </Button>,
      );
    }

    return <div className="flex gap-2">{buttons}</div>;
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [campaign, actionLoading]);

  if (loading) {
    return (
      <div role="status" aria-label="Загрузка рассылки" className="p-8">
        <div className="animate-pulse space-y-4 max-w-2xl">
          <div className="h-6 bg-gray-200 rounded w-1/3" />
          <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
            {[...Array(5)].map((_, i) => <div key={i} className="h-20 bg-gray-100 rounded" />)}
          </div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-8">
        <p role="alert" className="text-red-600 mb-4">{error}</p>
        <Button variant="secondary" onClick={() => navigate('/campaigns')}>
          Назад к рассылкам
        </Button>
      </div>
    );
  }

  if (!campaign) return null;

  const statusCfg = STATUS_CONFIG[campaign.status] ?? {
    variant: 'default' as const,
    label: campaign.status,
  };

  const hasABTest = !!(campaign.variants && campaign.variants.length > 0);

  return (
    <div>
      <PageHeader
        title={campaign.name}
        breadcrumbs={[
          { label: 'Рассылки', href: '/campaigns' },
          { label: campaign.name },
        ]}
        actions={renderedActions}
      />

      {/* Status badge */}
      <div className="mb-6">
        <Badge variant={statusCfg.variant}>{statusCfg.label}</Badge>
        {hasABTest && (
          <span className="ml-2"><Badge variant="info">A/B Тест</Badge></span>
        )}
        {campaign.started_at && (
          <span className="text-sm text-gray-400 ml-3">
            Начата: {(() => {
              const d = typeof campaign.started_at === 'object' && campaign.started_at !== null && 'seconds' in campaign.started_at
                ? new Date((campaign.started_at as {seconds: number}).seconds * 1000)
                : new Date(campaign.started_at);
              return isNaN(d.getTime()) ? '—' : d.toLocaleString('ru-RU');
            })()}
          </span>
        )}
        {campaign.completed_at && (
          <span className="text-sm text-gray-400 ml-3">
            Завершена:{' '}
            {(() => {
              const d = typeof campaign.completed_at === 'object' && campaign.completed_at !== null && 'seconds' in campaign.completed_at
                ? new Date((campaign.completed_at as {seconds: number}).seconds * 1000)
                : new Date(campaign.completed_at);
              return isNaN(d.getTime()) ? '—' : d.toLocaleString('ru-RU');
            })()}
          </span>
        )}
      </div>

      {/* Tab navigation (only shown when A/B test is present) */}
      {hasABTest && (
        <div className="flex gap-1 border-b border-gray-200 mb-6">
          <button
            type="button"
            onClick={() => setActiveTab('overview')}
            className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
              activeTab === 'overview'
                ? 'border-blue-600 text-blue-600'
                : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
            }`}
          >
            Обзор
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('ab_results')}
            className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
              activeTab === 'ab_results'
                ? 'border-blue-600 text-blue-600'
                : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
            }`}
          >
            A/B Результаты
          </button>
        </div>
      )}

      {/* ── Overview tab (or default when no A/B test) ── */}
      {(!hasABTest || activeTab === 'overview') && (
        <>
          {/* Stats cards */}
          <div className="grid grid-cols-2 md:grid-cols-5 gap-4 mb-6">
            <StatCard
              label="Получатели"
              value={stats?.total_recipients ?? campaign.total_recipients}
            />
            <StatCard
              label="Отправлено"
              value={stats?.sent || campaign.sent_count}
              color="text-blue-600"
            />
            <StatCard
              label="Доставлено"
              value={stats?.delivered ?? campaign.delivered_count}
              color="text-green-600"
            />
            <StatCard
              label="Ошибки"
              value={stats?.failed ?? campaign.failed_count}
              color="text-red-600"
            />
            <StatCard
              label="Доставляемость"
              value={(() => {
                const delivered = stats?.delivered ?? campaign.delivered_count ?? 0;
                const total = stats?.total_recipients ?? campaign.total_recipients ?? 0;
                if (stats && isFinite(stats.delivery_rate) && stats.delivery_rate > 0) {
                  return `${(stats.delivery_rate * 100).toFixed(1)}%`;
                }
                if (delivered > 0 && total > 0) {
                  return `${((delivered / total) * 100).toFixed(1)}%`;
                }
                return '0%';
              })()}
              color="text-indigo-600"
              subtext={
                stats?.total_cost
                  ? `Стоимость: ${stats.total_cost.toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} RUB`
                  : undefined
              }
            />
          </div>

          {/* Progress bar for running campaigns */}
          {(campaign.status === 'running' || campaign.status === 'paused') && (
            <div className="bg-white border border-gray-200 rounded-lg p-4 mb-6">
              <div className="flex justify-between text-sm text-gray-600 mb-2">
                <span>Прогресс отправки</span>
                <span>
                  {campaign.sent_count ?? 0} / {campaign.total_recipients}
                </span>
              </div>
              <div className="w-full bg-gray-200 rounded-full h-3">
                <div
                  className="bg-blue-600 h-3 rounded-full transition-all duration-500"
                  style={{
                    width: `${campaign.total_recipients > 0 ? (campaign.sent_count / campaign.total_recipients) * 100 : 0}%`,
                  }}
                />
              </div>
            </div>
          )}

          {/* Per-variant stats from stats endpoint (legacy, no variants array) */}
          {stats?.per_variant && stats.per_variant.length > 0 && !hasABTest && (
            <div className="bg-white border border-gray-200 rounded-lg overflow-hidden mb-6">
              <div className="px-4 py-3 border-b border-gray-200 bg-gray-50">
                <h3 className="font-medium text-gray-900">
                  Статистика по вариантам
                </h3>
              </div>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <caption className="sr-only">Таблица статистики по вариантам</caption>
                  <thead className="bg-gray-50 border-b border-gray-200">
                    <tr>
                      <th className="px-4 py-3 text-left font-medium text-gray-700">Вариант</th>
                      <th className="px-4 py-3 text-left font-medium text-gray-700">Отправлено</th>
                      <th className="px-4 py-3 text-left font-medium text-gray-700">Доставлено</th>
                      <th className="px-4 py-3 text-left font-medium text-gray-700">Доставляемость</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100">
                    {stats.per_variant.map((pv) => (
                      <tr key={pv.variant_id} className="hover:bg-gray-50">
                        <td className="px-4 py-3 font-medium text-gray-900">{pv.variant_name}</td>
                        <td className="px-4 py-3 text-gray-800">{(pv.sent ?? 0).toLocaleString()}</td>
                        <td className="px-4 py-3 text-green-600">{(pv.delivered ?? 0).toLocaleString()}</td>
                        <td className="px-4 py-3 text-gray-800">{((pv.delivery_rate ?? 0) * 100).toFixed(1)}%</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </>
      )}

      {/* ── A/B Results tab ── */}
      {hasABTest && activeTab === 'ab_results' && (
        <div className="space-y-6">
          {/* Live A/B status panel */}
          <ABStatusPanel campaignId={campaign.id} />

          {/* A/B configuration summary */}
          {campaign.ab_config && (
            <div className="bg-blue-50 border border-blue-200 rounded-lg p-4 text-sm">
              <h4 className="font-semibold text-blue-800 mb-2">Конфигурация теста</h4>
              <div className="flex flex-wrap gap-4 text-blue-700">
                <span>
                  Метрика: <strong>
                    {campaign.ab_config.metric === 'delivery_rate' ? 'Доставляемость' : 'Кликабельность'}
                  </strong>
                </span>
                <span>
                  Длительность: <strong>{campaign.ab_config.test_duration_hours}ч</strong>
                </span>
                <span>
                  Авто-выбор: <strong>{campaign.ab_config.auto_select_winner ? 'Да' : 'Нет'}</strong>
                </span>
              </div>
            </div>
          )}

          {/* Variant comparison table */}
          <div className="bg-white border border-gray-200 rounded-lg overflow-hidden">
            <div className="px-4 py-3 border-b border-gray-200 bg-gray-50">
              <h3 className="font-medium text-gray-900">Сравнение вариантов</h3>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <caption className="sr-only">Таблица сравнения вариантов A/B теста</caption>
                <thead className="bg-gray-50 border-b border-gray-200">
                  <tr>
                    <th className="px-4 py-3 text-left font-medium text-gray-700">Вариант</th>
                    <th className="px-4 py-3 text-left font-medium text-gray-700">Доля</th>
                    <th className="px-4 py-3 text-left font-medium text-gray-700">Отправлено</th>
                    <th className="px-4 py-3 text-left font-medium text-gray-700">Доставлено</th>
                    <th className="px-4 py-3 text-left font-medium text-gray-700">Ошибки</th>
                    <th className="px-4 py-3 text-left font-medium text-gray-700">Доставляемость</th>
                    <th className="px-4 py-3 text-left font-medium text-gray-700">Статус</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                  {campaign.variants!.map((v) => {
                    const compRow = variantComparison?.rows.find(
                      (r) => r.variant_id === v.id,
                    );
                    const isWinner =
                      v.is_winner ||
                      variantComparison?.winner_variant_id === v.id;
                    const deliveryRate = compRow?.delivery_rate ?? (
                      v.sent_count > 0 ? v.delivered_count / v.sent_count : 0
                    );

                    return (
                      <tr
                        key={v.id}
                        className={`hover:bg-gray-50 ${isWinner ? 'bg-green-50' : ''}`}
                      >
                        <td className="px-4 py-3">
                          <div className="flex items-center gap-2">
                            <span className="font-medium text-gray-900">{v.name}</span>
                            {v.is_control && (
                              <Badge variant="default">Контроль</Badge>
                            )}
                          </div>
                        </td>
                        <td className="px-4 py-3 text-gray-600">{v.percentage}%</td>
                        <td className="px-4 py-3 text-gray-800">
                          {(compRow?.sent ?? v.sent_count ?? 0).toLocaleString()}
                        </td>
                        <td className="px-4 py-3 text-green-600">
                          {(compRow?.delivered ?? v.delivered_count ?? 0).toLocaleString()}
                        </td>
                        <td className="px-4 py-3 text-red-600">
                          {(v.failed_count ?? 0).toLocaleString()}
                        </td>
                        <td className="px-4 py-3">
                          <div className="flex items-center gap-2">
                            <div className="flex-1 bg-gray-200 rounded-full h-2 min-w-[60px]">
                              <div
                                className="bg-blue-500 h-2 rounded-full"
                                style={{ width: `${Math.min(deliveryRate * 100, 100)}%` }}
                              />
                            </div>
                            <span className="text-gray-700 text-xs font-medium w-12 text-right">
                              {(deliveryRate * 100).toFixed(1)}%
                            </span>
                          </div>
                        </td>
                        <td className="px-4 py-3">
                          {isWinner ? (
                            <Badge variant="success">ПОБЕДИТЕЛЬ</Badge>
                          ) : variantComparison?.winner_variant_id || campaign.status === 'completed' ? (
                            <span className="text-gray-400">—</span>
                          ) : (
                            <Button
                              variant="secondary"
                              onClick={() => handleSelectWinner(v.id)}
                              disabled={!!selectingWinner}
                            >
                              {selectingWinner === v.id ? '...' : 'Выбрать'}
                            </Button>
                          )}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </div>

          {/* Stats summary for A/B tab */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <StatCard
              label="Всего получателей"
              value={stats?.total_recipients ?? campaign.total_recipients}
            />
            <StatCard
              label="Отправлено"
              value={stats?.sent || campaign.sent_count}
              color="text-blue-600"
            />
            <StatCard
              label="Доставлено"
              value={stats?.delivered ?? campaign.delivered_count}
              color="text-green-600"
            />
            <StatCard
              label="Доставляемость"
              value={(() => {
                const delivered = stats?.delivered ?? campaign.delivered_count ?? 0;
                const total = stats?.total_recipients ?? campaign.total_recipients ?? 0;
                if (stats && isFinite(stats.delivery_rate) && stats.delivery_rate > 0) {
                  return `${(stats.delivery_rate * 100).toFixed(1)}%`;
                }
                if (delivered > 0 && total > 0) {
                  return `${((delivered / total) * 100).toFixed(1)}%`;
                }
                return '0%';
              })()}
              color="text-indigo-600"
            />
          </div>
        </div>
      )}

      {/* Cancel dialog */}
      <ConfirmDialog
        open={showCancel}
        onConfirm={handleCancel}
        onCancel={() => setShowCancel(false)}
        title="Отменить рассылку"
        description="Вы уверены, что хотите отменить эту рассылку? Неотправленные сообщения не будут доставлены."
        confirmLabel="Отменить рассылку"
        variant="danger"
        loading={cancelling}
      />
    </div>
  );
}
