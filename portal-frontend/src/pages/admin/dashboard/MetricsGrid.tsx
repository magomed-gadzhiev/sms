import { StatCard } from '../../../components/data/StatCard';
import type { RealTimeMetrics } from '../../../api/admin';

interface MetricsGridProps {
  metrics: RealTimeMetrics | null;
  clientCount: number;
  revenue: string;
  healthyProviders: number;
  totalProviders: number;
  pendingTemplates: number;
}

export function MetricsGrid({
  metrics,
  clientCount,
  revenue,
  healthyProviders,
  totalProviders,
  pendingTemplates,
}: MetricsGridProps) {
  return (
    <div className="space-y-4 mb-6">
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <StatCard
          title="Сообщений/сек"
          value={metrics?.messages_per_second ?? '-'}
        />
        <StatCard
          title="Доставлено сегодня"
          value={metrics?.messages_delivered?.toLocaleString() ?? '-'}
        />
        <StatCard
          title="Ошибки"
          value={metrics?.messages_failed?.toLocaleString() ?? '-'}
        />
        <StatCard
          title="Очередь"
          value={metrics?.queue_depth?.toLocaleString() ?? '-'}
        />
      </div>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <StatCard title="Активные клиенты" value={clientCount} />
        <StatCard title="Выручка сегодня" value={`${revenue} \u20BD`} />
        <StatCard
          title="Провайдеры"
          value={`${healthyProviders}/${totalProviders} исправны`}
        />
        <StatCard title="Шаблоны на проверке" value={pendingTemplates} />
      </div>
    </div>
  );
}
