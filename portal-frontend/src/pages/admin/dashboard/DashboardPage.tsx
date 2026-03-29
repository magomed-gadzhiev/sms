import { useState, useCallback } from 'react';
import { usePolling } from '../../../hooks/usePolling';
import { PageHeader } from '../../../components/layout/PageHeader';
import { Button } from '../../../components/ui/Button';
import {
  analyticsAdminApi,
  clientsApi,
  providersApi,
  templatesApi,
  billingApi,
} from '../../../api/admin';
import type { RealTimeMetrics, ProviderHealth } from '../../../api/admin';
import { MetricsGrid } from './MetricsGrid';
import { TrafficChart } from './TrafficChart';
import { AlertsFeed } from './AlertsFeed';

interface ChartPoint {
  time: string;
  delivered: number;
  failed: number;
}

interface Alert {
  id: string;
  type: 'critical' | 'warning' | 'info';
  title: string;
  description: string;
  time: string;
}

function formatTime(date: Date): string {
  return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
}

export function DashboardPage() {
  const [metrics, setMetrics] = useState<RealTimeMetrics | null>(null);
  const [clientCount, setClientCount] = useState(0);
  const [revenue, setRevenue] = useState('0');
  const [healthyProviders, setHealthyProviders] = useState(0);
  const [totalProviders, setTotalProviders] = useState(0);
  const [pendingTemplates, setPendingTemplates] = useState(0);
  const [chartData, setChartData] = useState<ChartPoint[]>([]);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchAllData = useCallback(async () => {
    try {
      const now = new Date();
      const oneHourAgo = new Date(now.getTime() - 60 * 60 * 1000);

      const [
        metricsRes,
        clientsRes,
        providersRes,
        templatesRes,
        balancesRes,
        statsRes,
      ] = await Promise.allSettled([
        analyticsAdminApi.getRealTimeMetrics(),
        clientsApi.list({ limit: 1 }),
        providersApi.list({ active_only: true, limit: 100 }),
        templatesApi.list({ status: 'pending', limit: 1 }),
        billingApi.listBalances({ below_threshold: true, limit: 100 }),
        analyticsAdminApi.getStats({
          from: oneHourAgo.toISOString(),
          to: now.toISOString(),
          group_by: '5min',
        }),
      ]);

      // Metrics
      const metricsData =
        metricsRes.status === 'fulfilled' ? metricsRes.value : null;
      setMetrics(metricsData);

      // Client count
      if (clientsRes.status === 'fulfilled') {
        setClientCount(clientsRes.value.total);
      }

      // Templates pending
      if (templatesRes.status === 'fulfilled') {
        setPendingTemplates(templatesRes.value.total);
      }

      // Providers health
      if (providersRes.status === 'fulfilled') {
        const providers = providersRes.value.providers || [];
        setTotalProviders(providers.length);

        const healthResults = await Promise.allSettled(
          providers.map((p) => providersApi.health(p.provider_id)),
        );

        let healthy = 0;
        const newAlerts: Alert[] = [];

        healthResults.forEach((result, idx) => {
          if (result.status === 'fulfilled') {
            const health: ProviderHealth = result.value;
            if (health.success_rate >= 95) {
              healthy++;
            } else {
              newAlerts.push({
                id: `provider-${providers[idx].provider_id}`,
                type: 'critical',
                title: `Провайдер «${providers[idx].name}» деградирован`,
                description: `Успешность: ${health.success_rate.toFixed(1)}%`,
                time: formatTime(now),
              });
            }
          }
        });

        setHealthyProviders(healthy);

        // Queue alert
        if (metricsData && metricsData.queue_depth > 1000) {
          newAlerts.push({
            id: 'queue-depth',
            type: 'warning',
            title: 'Большая очередь',
            description: `Глубина очереди: ${metricsData.queue_depth.toLocaleString()} сообщений`,
            time: formatTime(now),
          });
        }

        // Low balance alerts
        if (balancesRes.status === 'fulfilled') {
          const balances = balancesRes.value.balances || [];
          for (const b of balances) {
            const balance = parseFloat(b.balance);
            const threshold = parseFloat(b.low_balance_threshold);
            if (balance < threshold && threshold > 0) {
              newAlerts.push({
                id: `balance-${b.client_id}`,
                type: 'warning',
                title: `Низкий баланс: ${b.client_name}`,
                description: `Баланс: ${b.balance} ${b.currency} (порог: ${b.low_balance_threshold})`,
                time: formatTime(now),
              });
            }
          }
        }

        setAlerts(newAlerts);
      }

      // Revenue placeholder (not available as single endpoint yet)
      setRevenue('0');

      // Chart data
      if (statsRes.status === 'fulfilled') {
        const raw = statsRes.value;
        // Transform API response to chart format
        // The API may return various formats — handle array or object with points
        const points: ChartPoint[] = [];

        if (Array.isArray(raw)) {
          for (const item of raw) {
            points.push({
              time: item.time
                ? formatTime(new Date(item.time))
                : item.period
                  ? formatTime(new Date(item.period))
                  : '',
              delivered: Number(item.delivered ?? item.messages_delivered ?? 0),
              failed: Number(item.failed ?? item.messages_failed ?? 0),
            });
          }
        } else if (raw && typeof raw === 'object' && 'points' in raw) {
          const rawPoints = (raw as { points: Record<string, unknown>[] })
            .points;
          if (Array.isArray(rawPoints)) {
            for (const item of rawPoints) {
              points.push({
                time: item.time
                  ? formatTime(new Date(item.time as string))
                  : item.period
                    ? formatTime(new Date(item.period as string))
                    : '',
                delivered: Number(
                  (item as Record<string, unknown>).delivered ??
                    (item as Record<string, unknown>).messages_delivered ??
                    0,
                ),
                failed: Number(
                  (item as Record<string, unknown>).failed ??
                    (item as Record<string, unknown>).messages_failed ??
                    0,
                ),
              });
            }
          }
        }

        setChartData(points);
      }

      setError(null);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Ошибка загрузки данных панели',
      );
    } finally {
      setLoading(false);
    }
  }, []);

  const { pause, resume, isPaused, lastUpdated } = usePolling(
    fetchAllData,
    30_000,
  );

  const lastUpdatedText = lastUpdated
    ? `Обновлено: ${formatTime(lastUpdated)}`
    : undefined;

  if (loading && !metrics) {
    return (
      <div className="flex items-center justify-center h-64 text-gray-400">
        Загрузка...
      </div>
    );
  }

  return (
    <div>
      <PageHeader
        title="Панель управления"
        subtitle={lastUpdatedText}
        breadcrumbs={[{ label: 'Админ' }, { label: 'Панель управления' }]}
        actions={
          <Button
            variant={isPaused ? 'primary' : 'secondary'}
            size="sm"
            onClick={isPaused ? resume : pause}
          >
            {isPaused ? 'Продолжить' : 'Пауза'}
          </Button>
        }
      />

      {error && (
        <div className="mb-4 rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          {error}
        </div>
      )}

      <MetricsGrid
        metrics={metrics}
        clientCount={clientCount}
        revenue={revenue}
        healthyProviders={healthyProviders}
        totalProviders={totalProviders}
        pendingTemplates={pendingTemplates}
      />

      <TrafficChart data={chartData} />

      <AlertsFeed alerts={alerts} />
    </div>
  );
}
