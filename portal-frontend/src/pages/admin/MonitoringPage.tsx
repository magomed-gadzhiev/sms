import { useState, useEffect, useRef, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { StatCard } from '../../components/data/StatCard';
import { DataTable, type Column } from '../../components/data/DataTable';
import { StatusBadge } from '../../components/ui/Badge';
import { Button } from '../../components/ui/Button';
import { useToast } from '../../components/ui/Toast';
import { analyticsAdminApi, providersApi, type ProviderInfo, type ProviderHealth, type RealTimeMetrics } from '../../api/admin';

const REFRESH_INTERVAL = 10_000;

export function MonitoringPage() {
  const toast = useToast();
  const [metrics, setMetrics] = useState<RealTimeMetrics | null>(null);
  const [providers, setProviders] = useState<(ProviderInfo & { health?: ProviderHealth })[]>([]);
  const [loading, setLoading] = useState(true);
  const [paused, setPaused] = useState(false);
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);
  const [error, setError] = useState(false);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchData = useCallback(async () => {
    try {
      const [metricsRes, providersRes] = await Promise.all([analyticsAdminApi.getRealTimeMetrics(), providersApi.list({ active_only: true, limit: 100, offset: 0 })]);
      setMetrics(metricsRes);
      const withHealth = await Promise.all((providersRes.providers || []).map(async (p) => {
        try { const h = await providersApi.health(p.provider_id); return { ...p, health: h }; }
        catch { return { ...p, health: undefined }; }
      }));
      setProviders(withHealth);
      setLastUpdate(new Date());
      setError(false);
      setLoading(false);
    } catch {
      setError(true);
      if (loading) { toast.error('Failed to load monitoring data'); setLoading(false); }
    }
  }, [loading, toast]);

  useEffect(() => { fetchData(); }, [fetchData]);

  useEffect(() => {
    if (paused) { if (intervalRef.current) clearInterval(intervalRef.current); return; }
    intervalRef.current = setInterval(fetchData, REFRESH_INTERVAL);
    return () => { if (intervalRef.current) clearInterval(intervalRef.current); };
  }, [paused, fetchData]);

  const providerColumns: Column<(typeof providers)[0]>[] = [
    { key: 'name', header: 'Provider' },
    { key: 'status', header: 'Status', render: (p) => <StatusBadge status={p.health?.status || 'unknown'} /> },
    { key: 'connections', header: 'Connections', render: (p) => p.health ? `${p.health.active_connections}/${p.health.total_connections}` : '-' },
    { key: 'success_rate', header: 'Success %', render: (p) => p.health ? `${p.health.success_rate}%` : '-' },
    { key: 'sent_24h', header: 'Sent (24h)', render: (p) => p.health?.messages_sent_24h?.toLocaleString() || '-' },
    { key: 'failed_24h', header: 'Failed (24h)', render: (p) => p.health?.messages_failed_24h?.toLocaleString() || '-' },
    { key: 'last_error', header: 'Last Error', render: (p) => p.health?.last_error ? <span className="text-xs text-danger">{p.health.last_error}</span> : '-' },
  ];

  const secondsAgo = lastUpdate ? Math.floor((Date.now() - lastUpdate.getTime()) / 1000) : null;

  return (
    <>
      <PageHeader title="Monitoring" subtitle={error ? 'Connection lost' : lastUpdate ? `Updated ${secondsAgo}s ago` : undefined} actions={<Button variant={paused ? 'primary' : 'secondary'} onClick={() => setPaused(!paused)}>{paused ? 'Resume' : 'Pause'}</Button>} />
      {error && <div className="mb-4 p-3 bg-yellow-50 border border-yellow-200 rounded text-sm text-yellow-800">Connection issue. Retrying automatically...</div>}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        <StatCard title="Messages/sec" value={metrics?.messages_per_second ?? '-'} />
        <StatCard title="Delivered" value={metrics?.messages_delivered?.toLocaleString() ?? '-'} />
        <StatCard title="Failed" value={metrics?.messages_failed?.toLocaleString() ?? '-'} />
        <StatCard title="Queue Depth" value={metrics?.queue_depth?.toLocaleString() ?? '-'} />
      </div>
      <h2 className="text-lg font-semibold mb-3">Provider Status</h2>
      <DataTable columns={providerColumns} data={providers} total={providers.length} page={1} pageSize={100} onPageChange={() => {}} loading={loading} keyField="provider_id" />
    </>
  );
}
