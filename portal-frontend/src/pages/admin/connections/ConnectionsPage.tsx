import { useState, useEffect, useCallback, useRef } from 'react';
import { PageHeader } from '../../../components/layout/PageHeader';
import { Button } from '../../../components/ui/Button';
import { StatusBadge } from '../../../components/ui/Badge';
import { useToast } from '../../../components/ui/Toast';
import { connectionsApi, type ConnectionInfo } from '../../../api/admin';

const REFRESH_INTERVAL = 10_000;

function bindTypeLabel(bindType: number): string {
  if (bindType === 1) return 'TX';
  if (bindType === 2) return 'RX';
  if (bindType === 3) return 'TRX';
  return String(bindType);
}

function statusBadge(status: string) {
  if (status === 'healthy') return <StatusBadge status="healthy" />;
  if (status === 'degraded') return <StatusBadge status="degraded" />;
  if (status === 'unhealthy') return <StatusBadge status="unhealthy" />;
  return <StatusBadge status="inactive" />;
}

function successRateColor(rate: number): string {
  if (rate >= 95) return 'text-green-600';
  if (rate >= 80) return 'text-yellow-600';
  return 'text-red-600';
}

export function ConnectionsPage() {
  const toast = useToast();
  const [connections, setConnections] = useState<ConnectionInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const firstLoadDoneRef = useRef(false);
  const [actioning, setActioning] = useState<string | null>(null);

  const fetchConnections = useCallback(async (isAuto = false) => {
    try {
      const res = await connectionsApi.list();
      setConnections(res.connections ?? []);
      setTotal(res.total ?? 0);
      setLoading(false);
      firstLoadDoneRef.current = true;
    } catch {
      if (!isAuto && !firstLoadDoneRef.current) {
        toast.error('Не удалось загрузить список подключений');
        setLoading(false);
      }
    }
  }, [toast]);

  useEffect(() => {
    fetchConnections(false);
    const interval = setInterval(() => fetchConnections(true), REFRESH_INTERVAL);
    return () => clearInterval(interval);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleReconnect = async (conn: ConnectionInfo) => {
    setActioning(conn.provider_id + ':reconnect');
    try {
      await connectionsApi.reconnect(conn.provider_id);
      toast.success(`Реконнект для «${conn.name}» поставлен в очередь`);
    } catch {
      toast.error(`Не удалось выполнить реконнект для «${conn.name}»`);
    } finally {
      setActioning(null);
    }
  };

  const handleStop = async (conn: ConnectionInfo) => {
    setActioning(conn.provider_id + ':stop');
    try {
      await connectionsApi.stop(conn.provider_id);
      toast.success(`Остановка «${conn.name}» поставлена в очередь`);
    } catch {
      toast.error(`Не удалось остановить «${conn.name}»`);
    } finally {
      setActioning(null);
    }
  };

  return (
    <>
      <PageHeader
        title="Список подключений"
        subtitle={`${total} SMPP-подключений · автообновление 10 с`}
        breadcrumbs={[
          { label: 'Админ', href: '/admin/dashboard' },
          { label: 'Подключения' },
        ]}
      />

      {loading ? (
        <div className="text-sm text-gray-500 py-8 text-center">Загрузка...</div>
      ) : connections.length === 0 ? (
        <div className="text-sm text-gray-500 py-8 text-center">Подключения не найдены</div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr className="border-b border-gray-200 bg-gray-50 text-left">
                <th className="px-4 py-3 font-medium text-gray-700">Провайдер</th>
                <th className="px-4 py-3 font-medium text-gray-700">Host:Port</th>
                <th className="px-4 py-3 font-medium text-gray-700">System ID</th>
                <th className="px-4 py-3 font-medium text-gray-700">Bind</th>
                <th className="px-4 py-3 font-medium text-gray-700">Статус</th>
                <th className="px-4 py-3 font-medium text-gray-700">Сессий</th>
                <th className="px-4 py-3 font-medium text-gray-700">Успех %</th>
                <th className="px-4 py-3 font-medium text-gray-700">Отправлено 24ч</th>
                <th className="px-4 py-3 font-medium text-gray-700">Действия</th>
              </tr>
            </thead>
            <tbody>
              {connections.map((conn) => (
                <tr key={conn.provider_id} className="border-b border-gray-100 hover:bg-gray-50">
                  <td className="px-4 py-3 font-semibold text-gray-900">{conn.name}</td>
                  <td className="px-4 py-3">
                    <span className="font-mono text-xs text-gray-600">{conn.host}:{conn.port}</span>
                  </td>
                  <td className="px-4 py-3">
                    <span className="font-mono text-xs text-gray-600">{conn.system_id}</span>
                  </td>
                  <td className="px-4 py-3 text-gray-700">{bindTypeLabel(conn.bind_type)}</td>
                  <td className="px-4 py-3">{statusBadge(conn.status)}</td>
                  <td className="px-4 py-3 text-gray-700">
                    {conn.active_connections}/{conn.max_connections}
                  </td>
                  <td className="px-4 py-3">
                    <span className={`font-medium ${successRateColor(conn.success_rate)}`}>
                      {conn.success_rate}%
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <span className="text-green-600 font-medium">
                      {conn.messages_sent_24h.toLocaleString()}
                    </span>
                    {conn.messages_failed_24h > 0 && (
                      <span className="ml-2 text-red-600 text-xs">
                        / {conn.messages_failed_24h.toLocaleString()} ош.
                      </span>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => handleReconnect(conn)}
                        disabled={actioning !== null}
                      >
                        {actioning === conn.provider_id + ':reconnect' ? 'Реконнект...' : 'Реконнект'}
                      </Button>
                      <Button
                        variant="danger"
                        size="sm"
                        onClick={() => handleStop(conn)}
                        disabled={!conn.active || actioning !== null}
                      >
                        {actioning === conn.provider_id + ':stop' ? 'Стоп...' : 'Стоп'}
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  );
}
