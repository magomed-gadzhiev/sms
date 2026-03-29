import { useState, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { StatCard } from '../../components/data/StatCard';
import { DataTable, type Column } from '../../components/data/DataTable';
import { StatusBadge } from '../../components/ui/Badge';
import { Button } from '../../components/ui/Button';
import { useToast } from '../../components/ui/Toast';
import { usePolling } from '../../hooks/usePolling';
import { analyticsAdminApi, providersApi, type ProviderInfo, type ProviderHealth, type RealTimeMetrics } from '../../api/admin';

const REFRESH_INTERVAL = 10_000;

export function MonitoringPage() {
  const toast = useToast();
  const [metrics, setMetrics] = useState<RealTimeMetrics | null>(null);
  const [providers, setProviders] = useState<(ProviderInfo & { health?: ProviderHealth })[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  const fetchData = useCallback(async () => {
    try {
      const [metricsRes, providersRes] = await Promise.all([analyticsAdminApi.getRealTimeMetrics(), providersApi.list({ active_only: true, limit: 100, offset: 0 })]);
      setMetrics(metricsRes);
      const withHealth = await Promise.all((providersRes.providers || []).map(async (p) => {
        try { const h = await providersApi.health(p.provider_id); return { ...p, health: h }; }
        catch { return { ...p, health: undefined }; }
      }));
      setProviders(withHealth);
      setError(false);
      setLoading(false);
    } catch {
      setError(true);
      if (loading) { toast.error('Failed to load monitoring data'); setLoading(false); }
    }
  }, [loading, toast]);

  const { isPaused, lastUpdated, pause, resume } = usePolling(fetchData, REFRESH_INTERVAL);

  const providerColumns: Column<(typeof providers)[0]>[] = [
    { key: 'name', header: 'Провайдер' },
    { key: 'status', header: 'Статус', render: (p) => <StatusBadge status={p.health?.status || 'unknown'} /> },
    { key: 'connections', header: 'Подключения', render: (p) => p.health ? `${p.health.active_connections}/${p.health.total_connections}` : '-' },
    { key: 'success_rate', header: 'Успех %', render: (p) => p.health ? `${p.health.success_rate}%` : '-' },
    { key: 'sent_24h', header: 'Отправлено (24ч)', render: (p) => p.health?.messages_sent_24h?.toLocaleString() || '-' },
    { key: 'failed_24h', header: 'Ошибки (24ч)', render: (p) => p.health?.messages_failed_24h?.toLocaleString() || '-' },
    { key: 'last_error', header: 'Последняя ошибка', render: (p) => p.health?.last_error ? <span className="text-xs text-danger">{p.health.last_error}</span> : '-' },
  ];

  const secondsAgo = lastUpdated ? Math.floor((Date.now() - lastUpdated.getTime()) / 1000) : null;

  return (
    <>
      <PageHeader title="Мониторинг" subtitle={error ? 'Соединение потеряно' : lastUpdated ? `Обновлено ${secondsAgo}с назад` : undefined} breadcrumbs={[{ label: 'Админ', href: '/admin/dashboard' }, { label: 'Мониторинг' }]} actions={<Button variant={isPaused ? 'primary' : 'secondary'} onClick={() => isPaused ? resume() : pause()}>{isPaused ? 'Продолжить' : 'Пауза'}</Button>} />
      {error && <div className="mb-4 p-3 bg-yellow-50 border border-yellow-200 rounded text-sm text-yellow-800">Проблема с подключением. Автоматическая переподключение...</div>}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        <StatCard title="Сообщений/сек" value={metrics?.messages_per_second ?? '-'} />
        <StatCard title="Доставлено" value={metrics?.messages_delivered?.toLocaleString() ?? '-'} />
        <StatCard title="Ошибки" value={metrics?.messages_failed?.toLocaleString() ?? '-'} />
        <StatCard title="Очередь" value={metrics?.queue_depth?.toLocaleString() ?? '-'} />
      </div>
      <h2 className="text-lg font-semibold mb-3">Статус провайдеров</h2>
      <DataTable columns={providerColumns} data={providers} total={providers.length} page={1} pageSize={100} onPageChange={() => {}} loading={loading} keyField="provider_id" />
    </>
  );
}
