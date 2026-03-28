import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { campaignsApi, type Campaign } from '../../api/campaigns';
import { ApiError } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Badge } from '../../components/ui/Badge';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { DataTable, type Column } from '../../components/data/DataTable';

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

export function CampaignsPage() {
  const navigate = useNavigate();
  const [campaigns, setCampaigns] = useState<Campaign[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Delete state
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

  const perPage = 20;

  async function load() {
    setLoading(true);
    setError('');
    try {
      const resp = await campaignsApi.list(page, perPage);
      setCampaigns(resp.campaigns ?? []);
      setTotal(resp.total ?? 0);
    } catch (err) {
      setError(
        err instanceof ApiError ? err.message : 'Не удалось загрузить рассылки',
      );
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, [page]);

  async function confirmDelete() {
    if (!deleteId) return;
    setDeleting(true);
    try {
      await campaignsApi.remove(deleteId);
      setCampaigns((prev) => prev.filter((c) => c.id !== deleteId));
      setTotal((prev) => prev - 1);
      setDeleteId(null);
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'Ошибка при удалении');
    } finally {
      setDeleting(false);
    }
  }

  function renderProgress(campaign: Campaign) {
    const total = campaign.total_recipients || 1;
    const sent = campaign.sent_count || 0;
    const pct = Math.round((sent / total) * 100);
    return (
      <div className="flex items-center gap-2 min-w-[120px]">
        <div className="flex-1 bg-gray-200 rounded-full h-2">
          <div
            className="bg-blue-600 h-2 rounded-full transition-all"
            style={{ width: `${pct}%` }}
          />
        </div>
        <span className="text-xs text-gray-500 whitespace-nowrap">
          {sent}/{campaign.total_recipients}
        </span>
      </div>
    );
  }

  const columns: Column<Campaign>[] = [
    {
      key: 'name',
      header: 'Название',
      render: (c) => (
        <span className="font-medium text-gray-900">{c.name}</span>
      ),
    },
    {
      key: 'status',
      header: 'Статус',
      render: (c) => {
        const cfg = STATUS_CONFIG[c.status] ?? {
          variant: 'default' as const,
          label: c.status,
        };
        return <Badge variant={cfg.variant}>{cfg.label}</Badge>;
      },
    },
    {
      key: 'progress',
      header: 'Прогресс',
      render: renderProgress,
    },
    {
      key: 'delivered_count',
      header: 'Доставлено',
      responsive: true,
      render: (c) => (
        <span className="font-mono text-sm">
          {c.delivered_count.toLocaleString()}
        </span>
      ),
    },
    {
      key: 'created_at',
      header: 'Дата создания',
      responsive: true,
      render: (c) => (
        <span className="text-gray-500 text-sm">
          {new Date(c.created_at).toLocaleDateString('ru-RU')}
        </span>
      ),
    },
  ];

  return (
    <div>
      <PageHeader
        title="Рассылки"
        subtitle="Управление массовыми рассылками SMS"
        actions={
          <Button onClick={() => navigate('/campaigns/new')}>
            + Создать рассылку
          </Button>
        }
      />

      {error && <p className="text-red-600 mb-4">{error}</p>}

      {!loading && !error && campaigns.length === 0 && (
        <div className="text-center py-12 text-gray-500">
          <p className="mb-2">Рассылки не созданы</p>
          <p className="text-sm mb-4">
            Создайте первую рассылку для массовой отправки SMS
          </p>
          <Button variant="ghost" onClick={() => navigate('/campaigns/new')}>
            Создать рассылку
          </Button>
        </div>
      )}

      {(loading || campaigns.length > 0) && (
        <DataTable
          columns={columns}
          data={campaigns}
          total={total}
          page={page}
          pageSize={perPage}
          onPageChange={setPage}
          loading={loading}
          keyField="id"
          onRowClick={(c) => navigate(`/campaigns/${c.id}`)}
          rowActions={(c) => (
            <div className="flex gap-2">
              <Button
                variant="secondary"
                size="sm"
                onClick={() => navigate(`/campaigns/${c.id}`)}
              >
                Открыть
              </Button>
              {c.status === 'draft' && (
                <Button
                  variant="danger"
                  size="sm"
                  onClick={() => setDeleteId(c.id)}
                >
                  Удалить
                </Button>
              )}
            </div>
          )}
        />
      )}

      <ConfirmDialog
        open={deleteId !== null}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteId(null)}
        title="Удалить рассылку"
        description="Вы уверены, что хотите удалить эту рассылку? Это действие нельзя отменить."
        confirmLabel="Удалить"
        variant="danger"
        loading={deleting}
      />
    </div>
  );
}
