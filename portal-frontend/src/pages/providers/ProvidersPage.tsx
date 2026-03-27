import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { providersApi, ApiError, type Provider } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { DataTable, type Column } from '../../components/data/DataTable';
import { StatusBadge } from '../../components/ui/Badge';

const BIND_LABELS: Record<number, string> = { 0: 'TRX', 1: 'TX', 2: 'RX' };

export function ProvidersPage() {
  const navigate = useNavigate();
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
      setError(err instanceof ApiError ? err.message : 'Failed to load providers');
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
      alert(err instanceof ApiError ? err.message : 'Failed to delete provider');
    } finally {
      setDeleting(false);
    }
  }

  const columns: Column<Provider>[] = [
    { key: 'name', header: 'Name', render: (p) => (
      <div>
        <div>{p.name}</div>
        {p.description && <div className="text-xs text-gray-500">{p.description}</div>}
      </div>
    )},
    { key: 'host', header: 'Host', render: (p) => <span className="font-mono">{p.host}:{p.port}</span> },
    { key: 'bind_type', header: 'Bind', render: (p) => <>{BIND_LABELS[p.bind_type] ?? '-'}</> },
    { key: 'max_connections', header: 'Conns' },
    { key: 'tps_limit', header: 'TPS' },
    { key: 'active', header: 'Status', render: (p) => <StatusBadge status={p.active ? 'active' : 'inactive'} /> },
  ];

  return (
    <div>
      <PageHeader
        title="SMPP Providers"
        actions={<Button onClick={() => navigate('/providers/new')}>+ Add Provider</Button>}
      />

      {error && <p className="text-red-600">{error}</p>}

      {!loading && !error && providers.length === 0 && (
        <div className="text-center py-12 text-gray-500">
          <p className="mb-2">Провайдеры не настроены</p>
          <p className="text-sm mb-4">Подключите SMPP-провайдера для начала отправки SMS</p>
          <Button variant="ghost" onClick={() => navigate('/providers/new')}>
            Добавить провайдера
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
            <Button variant="danger" size="sm" onClick={() => setDeleteId(p.id)}>
              Delete
            </Button>
          )}
        />
      )}

      <ConfirmDialog
        open={deleteId !== null}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteId(null)}
        title="Delete Provider"
        description="Are you sure you want to delete this provider? This action cannot be undone."
        confirmLabel="Delete"
        variant="danger"
        loading={deleting}
      />
    </div>
  );
}
