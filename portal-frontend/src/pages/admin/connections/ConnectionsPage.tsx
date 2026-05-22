import { useState, useCallback } from 'react';
import { PageHeader } from '../../../components/layout/PageHeader';
import { Button } from '../../../components/ui/Button';
import { StatusBadge } from '../../../components/ui/Badge';
import { Modal } from '../../../components/ui/Modal';
import { useToast } from '../../../components/ui/Toast';
import { usePolling } from '../../../hooks/usePolling';
import { connectionsApi, type ConnectionInfo } from '../../../api/admin';
import { ConnectionLog } from './ConnectionLog';

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
  if (rate >= 95) return 'text-green-600 dark:text-green-400';
  if (rate >= 80) return 'text-yellow-600 dark:text-yellow-400';
  return 'text-red-600 dark:text-red-400';
}

export function ConnectionsPage() {
  const toast = useToast();
  const [connections, setConnections] = useState<ConnectionInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [actioning, setActioning] = useState<string | null>(null);

  // Detail modal state
  const [selectedConnectionId, setSelectedConnectionId] = useState<string | null>(null);
  const [selectedDetail, setSelectedDetail] = useState<ConnectionInfo | null>(null);
  const [loadingDetail, setLoadingDetail] = useState(false);

  const fetchConnections = useCallback(async () => {
    try {
      const res = await connectionsApi.list();
      setConnections(res.connections ?? []);
      setTotal(res.total ?? 0);
      setLoading(false);
    } catch {
      // suppress auto-refresh errors after initial load
    }
  }, []);

  const { pause, resume, isPaused } = usePolling(fetchConnections, REFRESH_INTERVAL);

  const handleRowClick = async (conn: ConnectionInfo) => {
    setSelectedConnectionId(conn.provider_id);
    setLoadingDetail(true);
    setSelectedDetail(null);
    try {
      const detail = await connectionsApi.get(conn.provider_id);
      setSelectedDetail(detail);
    } catch {
      toast.error(`Не удалось загрузить детали подключения «${conn.name}»`);
      setSelectedConnectionId(null);
    } finally {
      setLoadingDetail(false);
    }
  };

  const handleCloseDetail = () => {
    setSelectedConnectionId(null);
    setSelectedDetail(null);
  };

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

      {/* Refresh controls */}
      <div className="flex items-center gap-2 mb-4">
        <Button
          variant="secondary"
          size="sm"
          onClick={() => (isPaused ? resume() : pause())}
        >
          {isPaused ? 'Автообновление: возобновить' : 'Пауза автообновления'}
        </Button>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => fetchConnections()}
        >
          Обновить
        </Button>
      </div>

      {loading ? (
        <div className="text-sm text-gray-500 dark:text-slate-400 py-8 text-center">Загрузка...</div>
      ) : connections.length === 0 ? (
        <div className="text-sm text-gray-500 dark:text-slate-400 py-8 text-center">Подключения не найдены</div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr className="border-b border-gray-200 dark:border-slate-700 bg-gray-50 dark:bg-slate-950 text-left">
                <th className="px-4 py-3 font-medium text-gray-700 dark:text-slate-300">Провайдер</th>
                <th className="px-4 py-3 font-medium text-gray-700 dark:text-slate-300">Host:Port</th>
                <th className="px-4 py-3 font-medium text-gray-700 dark:text-slate-300">System ID</th>
                <th className="px-4 py-3 font-medium text-gray-700 dark:text-slate-300">Bind</th>
                <th className="px-4 py-3 font-medium text-gray-700 dark:text-slate-300">Статус</th>
                <th className="px-4 py-3 font-medium text-gray-700 dark:text-slate-300">Сессий</th>
                <th className="px-4 py-3 font-medium text-gray-700 dark:text-slate-300">Успех %</th>
                <th className="px-4 py-3 font-medium text-gray-700 dark:text-slate-300">Отправлено 24ч</th>
                <th className="px-4 py-3 font-medium text-gray-700 dark:text-slate-300">Действия</th>
              </tr>
            </thead>
            <tbody>
              {connections.map((conn) => (
                <tr
                  key={conn.provider_id}
                  className="border-b border-gray-100 dark:border-slate-800 hover:bg-gray-50 dark:hover:bg-slate-800 cursor-pointer"
                  onClick={() => handleRowClick(conn)}
                >
                  <td className="px-4 py-3 font-semibold text-gray-900 dark:text-slate-100">{conn.name}</td>
                  <td className="px-4 py-3">
                    <span className="font-mono text-xs text-gray-600 dark:text-slate-400">{conn.host}:{conn.port}</span>
                  </td>
                  <td className="px-4 py-3">
                    <span className="font-mono text-xs text-gray-600 dark:text-slate-400">{conn.system_id}</span>
                  </td>
                  <td className="px-4 py-3 text-gray-700 dark:text-slate-300">{bindTypeLabel(conn.bind_type)}</td>
                  <td className="px-4 py-3">{statusBadge(conn.status)}</td>
                  <td className="px-4 py-3 text-gray-700 dark:text-slate-300">
                    {conn.active_connections}/{conn.max_connections}
                  </td>
                  <td className="px-4 py-3">
                    <span className={`font-medium ${successRateColor(conn.success_rate)}`}>
                      {conn.success_rate}%
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <span className="text-green-600 dark:text-green-400 font-medium">
                      {conn.messages_sent_24h.toLocaleString()}
                    </span>
                    {conn.messages_failed_24h > 0 && (
                      <span className="ml-2 text-red-600 dark:text-red-400 text-xs">
                        / {conn.messages_failed_24h.toLocaleString()} ош.
                      </span>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2" onClick={(e) => e.stopPropagation()}>
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

      {/* Detail modal */}
      <Modal
        open={selectedConnectionId !== null}
        onClose={handleCloseDetail}
        title="Детали подключения"
        wide
      >
        {loadingDetail ? (
          <div className="text-sm text-gray-500 dark:text-slate-400 py-8 text-center">Загрузка...</div>
        ) : selectedDetail ? (
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-x-6 gap-y-3 text-sm">
              <div>
                <span className="text-gray-500 dark:text-slate-400">Провайдер</span>
                <div className="font-semibold text-gray-900 dark:text-slate-100 mt-0.5">{selectedDetail.name}</div>
              </div>
              <div>
                <span className="text-gray-500 dark:text-slate-400">Host:Port</span>
                <div className="font-mono text-xs text-gray-700 dark:text-slate-300 mt-0.5">
                  {selectedDetail.host}:{selectedDetail.port}
                </div>
              </div>
              <div>
                <span className="text-gray-500 dark:text-slate-400">System ID</span>
                <div className="font-mono text-xs text-gray-700 dark:text-slate-300 mt-0.5">{selectedDetail.system_id}</div>
              </div>
              <div>
                <span className="text-gray-500 dark:text-slate-400">Bind Type</span>
                <div className="mt-0.5 text-gray-700 dark:text-slate-300">{bindTypeLabel(selectedDetail.bind_type)}</div>
              </div>
              <div>
                <span className="text-gray-500 dark:text-slate-400">Статус</span>
                <div className="mt-0.5">{statusBadge(selectedDetail.status)}</div>
              </div>
              <div>
                <span className="text-gray-500 dark:text-slate-400">Сессий</span>
                <div className="mt-0.5 text-gray-700 dark:text-slate-300">
                  {selectedDetail.active_connections}/{selectedDetail.max_connections}
                </div>
              </div>
              <div>
                <span className="text-gray-500 dark:text-slate-400">Успех %</span>
                <div className={`mt-0.5 font-medium ${successRateColor(selectedDetail.success_rate)}`}>
                  {selectedDetail.success_rate}%
                </div>
              </div>
              <div>
                <span className="text-gray-500 dark:text-slate-400">Отправлено 24ч</span>
                <div className="mt-0.5 text-green-600 dark:text-green-400 font-medium">
                  {selectedDetail.messages_sent_24h.toLocaleString()}
                  {selectedDetail.messages_failed_24h > 0 && (
                    <span className="ml-2 text-red-600 dark:text-red-400 text-xs font-normal">
                      / {selectedDetail.messages_failed_24h.toLocaleString()} ош.
                    </span>
                  )}
                </div>
              </div>
            </div>

            {selectedDetail.last_error && (
              <div className="rounded-md bg-red-50 dark:bg-red-950/40 border border-red-200 dark:border-red-800 p-3 text-sm text-red-700 dark:text-red-300">
                <span className="font-medium">Последняя ошибка: </span>
                {selectedDetail.last_error}
              </div>
            )}

            <div className="flex items-center gap-2 pt-2 border-t border-gray-100 dark:border-slate-800">
              <Button
                variant="secondary"
                size="sm"
                onClick={() => handleReconnect(selectedDetail)}
                disabled={actioning !== null}
              >
                {actioning === selectedDetail.provider_id + ':reconnect' ? 'Реконнект...' : 'Переподключить'}
              </Button>
              <Button
                variant="danger"
                size="sm"
                onClick={() => handleStop(selectedDetail)}
                disabled={!selectedDetail.active || actioning !== null}
              >
                {actioning === selectedDetail.provider_id + ':stop' ? 'Стоп...' : 'Остановить'}
              </Button>
            </div>

            <div className="mt-4">
              <p className="text-sm font-semibold text-gray-700 dark:text-slate-300 mb-2">Лог подключения</p>
              <ConnectionLog connectionId={selectedConnectionId!} />
            </div>
          </div>
        ) : null}
      </Modal>
    </>
  );
}
