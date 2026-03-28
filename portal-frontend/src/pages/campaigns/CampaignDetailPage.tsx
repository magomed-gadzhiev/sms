import { useState, useEffect, useCallback } from 'react';
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

const STATUS_CONFIG: Record<
  string,
  { variant: 'default' | 'info' | 'warning' | 'success' | 'danger'; label: string }
> = {
  draft: { variant: 'default', label: 'Черновик' },
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

  const [campaign, setCampaign] = useState<Campaign | null>(null);
  const [stats, setStats] = useState<CampaignStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [actionLoading, setActionLoading] = useState('');

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

  // Poll for running campaigns
  useEffect(() => {
    if (!campaign || campaign.status !== 'running') return;
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
      alert(err instanceof ApiError ? err.message : 'Ошибка');
    } finally {
      setActionLoading('');
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
      alert(err instanceof ApiError ? err.message : 'Ошибка при отмене');
    } finally {
      setCancelling(false);
    }
  }

  if (loading) {
    return (
      <div className="animate-pulse p-8 text-center text-gray-500">
        Загрузка...
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-8">
        <p className="text-red-600 mb-4">{error}</p>
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

  function renderActions() {
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
  }

  return (
    <div>
      <PageHeader
        title={campaign.name}
        breadcrumbs={[
          { label: 'Рассылки', href: '/campaigns' },
          { label: campaign.name },
        ]}
        actions={renderActions()}
      />

      {/* Status badge */}
      <div className="mb-6">
        <Badge variant={statusCfg.variant}>{statusCfg.label}</Badge>
        {campaign.started_at && (
          <span className="text-sm text-gray-400 ml-3">
            Начата: {new Date(campaign.started_at).toLocaleString('ru-RU')}
          </span>
        )}
        {campaign.completed_at && (
          <span className="text-sm text-gray-400 ml-3">
            Завершена:{' '}
            {new Date(campaign.completed_at).toLocaleString('ru-RU')}
          </span>
        )}
      </div>

      {/* Stats cards */}
      <div className="grid grid-cols-2 md:grid-cols-5 gap-4 mb-6">
        <StatCard
          label="Получатели"
          value={stats?.total_recipients ?? campaign.total_recipients}
        />
        <StatCard
          label="Отправлено"
          value={stats?.sent ?? campaign.sent_count}
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
          value={
            stats
              ? `${(stats.delivery_rate * 100).toFixed(1)}%`
              : '—'
          }
          color="text-indigo-600"
          subtext={
            stats?.total_cost
              ? `Стоимость: ${stats.total_cost.toFixed(2)}`
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
              {campaign.sent_count} / {campaign.total_recipients}
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

      {/* Variants table (A/B test) */}
      {campaign.variants && campaign.variants.length > 0 && (
        <div className="bg-white border border-gray-200 rounded-lg overflow-hidden mb-6">
          <div className="px-4 py-3 border-b border-gray-200 bg-gray-50">
            <h3 className="font-medium text-gray-900">Варианты (A/B тест)</h3>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-gray-50 border-b border-gray-200">
                <tr>
                  <th className="px-4 py-3 text-left font-medium text-gray-700">
                    Вариант
                  </th>
                  <th className="px-4 py-3 text-left font-medium text-gray-700">
                    Шаблон
                  </th>
                  <th className="px-4 py-3 text-left font-medium text-gray-700">
                    Доля %
                  </th>
                  <th className="px-4 py-3 text-left font-medium text-gray-700">
                    Отправлено
                  </th>
                  <th className="px-4 py-3 text-left font-medium text-gray-700">
                    Доставлено
                  </th>
                  <th className="px-4 py-3 text-left font-medium text-gray-700">
                    Ошибки
                  </th>
                  <th className="px-4 py-3 text-left font-medium text-gray-700">
                    Статус
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {campaign.variants.map((v) => (
                  <tr key={v.id} className="hover:bg-gray-50">
                    <td className="px-4 py-3 font-medium text-gray-900">
                      {v.name}
                      {v.is_control && (
                        <Badge variant="default">Контроль</Badge>
                      )}
                    </td>
                    <td className="px-4 py-3 text-gray-600 font-mono text-xs">
                      {v.template_id}
                    </td>
                    <td className="px-4 py-3 text-gray-600">
                      {v.percentage}%
                    </td>
                    <td className="px-4 py-3 text-gray-800">
                      {v.sent_count.toLocaleString()}
                    </td>
                    <td className="px-4 py-3 text-green-600">
                      {v.delivered_count.toLocaleString()}
                    </td>
                    <td className="px-4 py-3 text-red-600">
                      {v.failed_count.toLocaleString()}
                    </td>
                    <td className="px-4 py-3">
                      {v.is_winner ? (
                        <Badge variant="success">Победитель</Badge>
                      ) : (
                        <span className="text-gray-400">—</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Per-variant stats from stats endpoint */}
      {stats?.per_variant && stats.per_variant.length > 0 && !campaign.variants?.length && (
        <div className="bg-white border border-gray-200 rounded-lg overflow-hidden mb-6">
          <div className="px-4 py-3 border-b border-gray-200 bg-gray-50">
            <h3 className="font-medium text-gray-900">
              Статистика по вариантам
            </h3>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-gray-50 border-b border-gray-200">
                <tr>
                  <th className="px-4 py-3 text-left font-medium text-gray-700">
                    Вариант
                  </th>
                  <th className="px-4 py-3 text-left font-medium text-gray-700">
                    Отправлено
                  </th>
                  <th className="px-4 py-3 text-left font-medium text-gray-700">
                    Доставлено
                  </th>
                  <th className="px-4 py-3 text-left font-medium text-gray-700">
                    Доставляемость
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {stats.per_variant.map((pv) => (
                  <tr key={pv.variant_id} className="hover:bg-gray-50">
                    <td className="px-4 py-3 font-medium text-gray-900">
                      {pv.variant_name}
                    </td>
                    <td className="px-4 py-3 text-gray-800">
                      {pv.sent.toLocaleString()}
                    </td>
                    <td className="px-4 py-3 text-green-600">
                      {pv.delivered.toLocaleString()}
                    </td>
                    <td className="px-4 py-3 text-gray-800">
                      {(pv.delivery_rate * 100).toFixed(1)}%
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
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
