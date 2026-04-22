import { useState, useEffect, useMemo, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { campaignsApi, type Campaign } from '../../api/campaigns';
import { ApiError } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Badge } from '../../components/ui/Badge';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { DataTable, type Column } from '../../components/data/DataTable';
import { useAuth } from '../../contexts/AuthContext';

const STATUS_CONFIG: Record<
  string,
  { variant: 'default' | 'info' | 'warning' | 'success' | 'danger'; label: string }
> = {
  draft: { variant: 'default', label: 'Черновик' },
  scheduled: { variant: 'default', label: 'Запланирована' },
  materializing: { variant: 'warning', label: 'Подготовка' },
  running: { variant: 'info', label: 'Запущена' },
  paused: { variant: 'warning', label: 'На паузе' },
  completed: { variant: 'success', label: 'Завершена' },
  cancelled: { variant: 'danger', label: 'Отменена' },
};

export function CampaignsPage() {
  const navigate = useNavigate();
  const { user } = useAuth();
  const isSubAccount = !!user?.parent_client_id;
  const isReseller = !!user?.is_reseller;
  const [campaigns, setCampaigns] = useState<Campaign[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Delete state
  const [statusFilter, setStatusFilter] = useState('');

  // Delete state
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState('');

  const perPage = 20;

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const resp = await campaignsApi.list(page, perPage, statusFilter);
      setCampaigns(resp.campaigns ?? []);
      setTotal(resp.total ?? 0);
    } catch (err) {
      setError(
        err instanceof ApiError ? err.message : 'Не удалось загрузить рассылки',
      );
    } finally {
      setLoading(false);
    }
  }, [page, statusFilter]);

  useEffect(() => {
    setPage(1);
  }, [statusFilter]);

  useEffect(() => {
    load();
  }, [load]);

  async function confirmDelete() {
    if (!deleteId) return;
    setDeleting(true);
    setDeleteError('');
    try {
      await campaignsApi.remove(deleteId);
      setCampaigns((prev) => prev.filter((c) => c.id !== deleteId));
      setTotal((prev) => prev - 1);
      setDeleteId(null);
    } catch (err) {
      setDeleteError(err instanceof ApiError ? err.message : 'Ошибка при удалении');
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

  const handleOpen = useCallback((c: Campaign) => navigate(`/campaigns/${c.id}`), [navigate]);
  const handleDelete = useCallback((c: Campaign) => setDeleteId(c.id), []);

  const columns = useMemo<Column<Campaign>[]>(() => [
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
          {(c.delivered_count ?? 0).toLocaleString()}
        </span>
      ),
    },
    {
      key: 'created_at',
      header: 'Дата создания',
      responsive: true,
      render: (c) => (
        <span className="text-gray-500 text-sm">
          {(() => {
            const raw = c.created_at ?? (c as any).createdAt;
            if (!raw) return '-';
            const d = typeof raw === 'object' && raw !== null && 'seconds' in raw
              ? new Date((raw as {seconds: number}).seconds * 1000)
              : new Date(raw);
            return isNaN(d.getTime()) ? '-' : d.toLocaleDateString('ru-RU');
          })()}
        </span>
      ),
    },
  // eslint-disable-next-line react-hooks/exhaustive-deps
  ], []);

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

      {isSubAccount && (
        <div className="mb-4 bg-blue-50 border border-blue-200 rounded-lg px-4 py-3 text-sm text-blue-800">
          Вы работаете в режиме суб-аккаунта. Рассылки доступны только в рамках вашего аккаунта.
        </div>
      )}

      {isReseller && (
        <div className="mb-4 bg-amber-50 border border-amber-200 rounded-lg px-4 py-3 text-sm text-amber-800">
          Отображаются только ваши рассылки. Рассылки суб-аккаунтов доступны в разделе{' '}
          <a href="/network/sub-accounts" className="underline font-medium hover:text-amber-900">Суб-аккаунты</a>.
        </div>
      )}

      <div className="mb-4 flex items-center gap-3">
        <label htmlFor="campaign-status-filter" className="text-sm font-medium text-gray-700 whitespace-nowrap">
          Статус:
        </label>
        <select
          id="campaign-status-filter"
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className="border border-gray-300 rounded-md px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
        >
          <option value="">Все</option>
          <option value="draft">Черновик</option>
          <option value="scheduled">Запланирована</option>
          <option value="materializing">Подготовка</option>
          <option value="running">Запущена</option>
          <option value="paused">На паузе</option>
          <option value="completed">Завершена</option>
          <option value="cancelled">Отменена</option>
        </select>
      </div>

      {error && <p role="alert" className="text-red-600 mb-4">{error}</p>}
      {deleteError && <p role="alert" className="text-red-600 mb-4">{deleteError}</p>}

      {!loading && !error && campaigns.length === 0 && (
        <div className="text-center py-12 text-gray-500">
          <svg className="mx-auto mb-3 w-12 h-12 text-gray-300" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5}
              d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
          </svg>
          {statusFilter ? (
            <>
              <p className="mb-2 font-medium">Рассылок с этим статусом нет</p>
              <p className="text-sm mb-4">Попробуйте выбрать другой статус или сбросить фильтр</p>
              <Button variant="ghost" onClick={() => setStatusFilter('')}>
                Сбросить фильтр
              </Button>
            </>
          ) : (
            <>
              <p className="mb-2 font-medium">Рассылки не созданы</p>
              <p className="text-sm mb-4">
                Создайте первую рассылку для массовой отправки SMS
              </p>
              <Button variant="ghost" onClick={() => navigate('/campaigns/new')}>
                Создать рассылку
              </Button>
            </>
          )}
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
          tableLabel="Список рассылок"
          onRowClick={handleOpen}
          rowActions={(c) => (
            <div className="flex gap-2">
              <Button
                variant="secondary"
                size="sm"
                aria-label={`Открыть рассылку ${c.name}`}
                onClick={() => handleOpen(c)}
              >
                Открыть
              </Button>
              {c.status === 'draft' && (
                <Button
                  variant="danger"
                  size="sm"
                  aria-label={`Удалить рассылку ${c.name}`}
                  onClick={() => handleDelete(c)}
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
        onCancel={() => { setDeleteId(null); setDeleteError(''); }}
        title="Удалить рассылку"
        description="Вы уверены, что хотите удалить эту рассылку? Это действие нельзя отменить."
        confirmLabel="Удалить"
        variant="danger"
        loading={deleting}
      />
    </div>
  );
}
