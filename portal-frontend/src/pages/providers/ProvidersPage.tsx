import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { providersApi, ApiError, type Provider } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { DataTable, type Column } from '../../components/data/DataTable';
import { StatusBadge } from '../../components/ui/Badge';
import { useAuth } from '../../contexts/AuthContext';

const BIND_LABELS: Record<number, string> = { 0: 'TRX', 1: 'TX', 2: 'RX' };

export function ProvidersPage() {
  const navigate = useNavigate();
  const { user } = useAuth();
  const isSubAccount = !!user?.parent_client_id;
  const [providers, setProviders] = useState<Provider[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

  async function load() {
    setLoading(true);
    setError('');
    try {
      const resp = await providersApi.list();
      setProviders(resp.providers ?? []);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось загрузить провайдеров');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { load(); }, []);

  async function confirmDelete() {
    if (!deleteId) return;
    setDeleting(true);
    try {
      await providersApi.remove(deleteId);
      setProviders(prev => prev.filter(p => p.id !== deleteId));
      setDeleteId(null);
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'Не удалось удалить провайдера');
    } finally {
      setDeleting(false);
    }
  }

  const columns: Column<Provider>[] = [
    { key: 'name', header: 'Название', render: (p) => (
      <div>
        <div>{p.name}</div>
        {p.description && <div className="text-xs text-gray-500">{p.description}</div>}
      </div>
    )},
    { key: 'host', header: 'Хост', render: (p) => <span className="font-mono">{p.host}:{p.port}</span> },
    { key: 'bind_type', header: 'Привязка', render: (p) => <>{BIND_LABELS[p.bind_type] ?? '-'}</> },
    { key: 'max_connections', header: 'Подкл.' },
    { key: 'tps_limit', header: 'TPS' },
    { key: 'active', header: 'Статус', render: (p) => <StatusBadge status={p.active ? 'active' : 'inactive'} /> },
  ];

  return (
    <div>
      <PageHeader
        title="Подключения"
        actions={!isSubAccount ? <Button onClick={() => navigate('/providers/new')}>+ Добавить подключение</Button> : null}
      />

      {error && <p className="text-red-600">{error}</p>}

      {isSubAccount && (
        <div className="bg-blue-50 border border-blue-200 rounded-lg p-6 max-w-2xl mb-4">
          <p className="text-sm text-blue-900">
            В режиме суб-аккаунта SMPP-провайдеры настраивает агрегатор. Вы не управляете подключениями самостоятельно — трафик уходит через инфраструктуру агрегатора.
          </p>
        </div>
      )}

      {!loading && !error && !isSubAccount && providers.length === 0 && (
        <div className="text-center py-12 text-gray-500">
          <p className="mb-2">Подключения не настроены</p>
          <p className="text-sm mb-4">Подключите SMPP-провайдера для начала отправки SMS</p>
          <Button variant="ghost" onClick={() => navigate('/providers/new')}>
            Добавить подключение
          </Button>
        </div>
      )}

      {(loading || providers.length > 0) && (
        <DataTable
          columns={columns}
          data={providers}
          total={providers.length}
          page={1}
          pageSize={providers.length}
          onPageChange={() => {}}
          loading={loading}
          keyField="id"
          rowActions={(p) => (
            <div className="flex gap-1.5">
              <Button variant="ghost" size="sm" onClick={() => navigate(`/providers/${p.id}/edit`)}>
                Редактировать
              </Button>
              <Button variant="danger" size="sm" onClick={() => setDeleteId(p.id)}>
                Удалить
              </Button>
            </div>
          )}
        />
      )}

      <ConfirmDialog
        open={deleteId !== null}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteId(null)}
        title="Удалить подключение"
        description="Вы уверены, что хотите удалить это подключение? Это действие нельзя отменить."
        confirmLabel="Удалить"
        variant="danger"
        loading={deleting}
      />
    </div>
  );
}
