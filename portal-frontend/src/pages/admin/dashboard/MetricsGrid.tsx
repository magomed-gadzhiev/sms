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
          title="Messages/sec"
          value={metrics?.messages_per_second ?? '-'}
        />
        <StatCard
          title="Delivered Today"
          value={metrics?.messages_delivered?.toLocaleString() ?? '-'}
        />
        <StatCard
          title="Errors"
          value={metrics?.messages_failed?.toLocaleString() ?? '-'}
        />
        <StatCard
          title="Queue Depth"
          value={metrics?.queue_depth?.toLocaleString() ?? '-'}
        />
      </div>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <StatCard title="Active Clients" value={clientCount} />
        <StatCard title="Revenue Today" value={`${revenue} \u20BD`} />
        <StatCard
          title="Providers"
          value={`${healthyProviders}/${totalProviders} healthy`}
        />
        <StatCard title="Templates on Review" value={pendingTemplates} />
      </div>
    </div>
  );
}
