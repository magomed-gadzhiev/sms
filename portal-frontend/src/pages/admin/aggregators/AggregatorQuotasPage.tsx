import { useState, useEffect, useCallback } from 'react';
import { useParams } from 'react-router-dom';
import { PageHeader } from '../../../components/layout/PageHeader';
import { DataTable, type Column } from '../../../components/data/DataTable';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { Modal } from '../../../components/ui/Modal';
import { Badge } from '../../../components/ui/Badge';
import { useToast } from '../../../components/ui/Toast';
import { aggregatorQuotasApi, type AggregatorQuota } from '../../../api/admin';

interface CreateFormState {
  period_start: string;
  period_end: string;
  segment_limit: string;
  overage_rate: string;
  currency: string;
  auto_renew: boolean;
}

interface EditFormState {
  segment_limit: string;
  overage_rate: string;
  auto_renew: boolean;
}

const EMPTY_CREATE_FORM: CreateFormState = {
  period_start: '',
  period_end: '',
  segment_limit: '',
  overage_rate: '',
  currency: 'RUB',
  auto_renew: false,
};

function formatDate(iso: string) {
  if (!iso) return '—';
  return new Date(iso).toLocaleDateString('ru-RU');
}

function utilizationColor(pct: number): string {
  if (pct >= 100) return 'bg-red-500';
  if (pct >= 80) return 'bg-yellow-500';
  return 'bg-green-500';
}

export function AggregatorQuotasPage() {
  const { aggregatorId } = useParams<{ aggregatorId: string }>();
  const toast = useToast();

  const [activeQuota, setActiveQuota] = useState<AggregatorQuota | null>(null);
  const [quotas, setQuotas] = useState<AggregatorQuota[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);

  // Create dialog
  const [showCreate, setShowCreate] = useState(false);
  const [createForm, setCreateForm] = useState<CreateFormState>(EMPTY_CREATE_FORM);
  const [creating, setCreating] = useState(false);

  // Edit dialog
  const [editTarget, setEditTarget] = useState<AggregatorQuota | null>(null);
  const [editForm, setEditForm] = useState<EditFormState>({ segment_limit: '', overage_rate: '', auto_renew: false });
  const [saving, setSaving] = useState(false);

  const id = aggregatorId ?? '';

  const fetchActiveQuota = useCallback(async () => {
    if (!id) return;
    try {
      const res = await aggregatorQuotasApi.getActive(id);
      setActiveQuota(res.quota ?? null);
    } catch {
      setActiveQuota(null);
    }
  }, [id]);

  const fetchQuotas = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    try {
      const res = await aggregatorQuotasApi.list(id, { limit: 50, offset: 0 });
      setQuotas(res.quotas ?? []);
      setTotal(res.total ?? 0);
    } catch {
      toast.error('Не удалось загрузить квоты');
    } finally {
      setLoading(false);
    }
  }, [id, toast]);

  useEffect(() => {
    fetchActiveQuota();
    fetchQuotas();
  }, [fetchActiveQuota, fetchQuotas]);

  // ── Create ──

  function openCreate() {
    setCreateForm(EMPTY_CREATE_FORM);
    setShowCreate(true);
  }

  async function handleCreate() {
    if (!createForm.period_start || !createForm.period_end || !createForm.segment_limit || !createForm.overage_rate) {
      toast.error('Заполните обязательные поля');
      return;
    }
    setCreating(true);
    try {
      await aggregatorQuotasApi.create(id, {
        period_start: createForm.period_start,
        period_end: createForm.period_end,
        segment_limit: parseInt(createForm.segment_limit, 10),
        overage_rate: createForm.overage_rate,
        currency: createForm.currency || undefined,
        auto_renew: createForm.auto_renew,
      });
      toast.success('Квота создана');
      setShowCreate(false);
      fetchActiveQuota();
      fetchQuotas();
    } catch {
      toast.error('Не удалось создать квоту');
    } finally {
      setCreating(false);
    }
  }

  // ── Edit ──

  function openEdit(quota: AggregatorQuota) {
    setEditTarget(quota);
    setEditForm({
      segment_limit: String(quota.segment_limit),
      overage_rate: quota.overage_rate,
      auto_renew: quota.auto_renew,
    });
  }

  async function handleEdit() {
    if (!editTarget) return;
    if (!editForm.segment_limit || !editForm.overage_rate) {
      toast.error('Заполните обязательные поля');
      return;
    }
    setSaving(true);
    try {
      await aggregatorQuotasApi.update(id, editTarget.quota_id, {
        segment_limit: parseInt(editForm.segment_limit, 10),
        overage_rate: editForm.overage_rate,
        auto_renew: editForm.auto_renew,
      });
      toast.success('Квота обновлена');
      setEditTarget(null);
      fetchActiveQuota();
      fetchQuotas();
    } catch {
      toast.error('Не удалось обновить квоту');
    } finally {
      setSaving(false);
    }
  }

  // ── Computed ──

  const utilizationPct = activeQuota
    ? Math.min(100, Math.round((activeQuota.segments_used / Math.max(activeQuota.segment_limit, 1)) * 100))
    : 0;

  // ── Table columns ──

  const columns: Column<AggregatorQuota>[] = [
    {
      key: 'period_start',
      header: 'Начало периода',
      render: (row) => <span className="text-sm">{formatDate(row.period_start)}</span>,
    },
    {
      key: 'period_end',
      header: 'Конец периода',
      render: (row) => <span className="text-sm">{formatDate(row.period_end)}</span>,
    },
    {
      key: 'segment_limit',
      header: 'Лимит сегментов',
      render: (row) => <span className="text-sm font-mono">{row.segment_limit.toLocaleString('ru-RU')}</span>,
    },
    {
      key: 'segments_used',
      header: 'Использовано',
      render: (row) => <span className="text-sm font-mono">{row.segments_used.toLocaleString('ru-RU')}</span>,
    },
    {
      key: 'overage_rate',
      header: 'Ставка за превышение',
      render: (row) => (
        <span className="text-sm font-mono">
          {row.overage_rate} {row.currency}
        </span>
      ),
    },
    {
      key: 'auto_renew',
      header: 'Авто-продление',
      render: (row) => (
        <Badge variant={row.auto_renew ? 'success' : 'default'}>
          {row.auto_renew ? 'Да' : 'Нет'}
        </Badge>
      ),
    },
    {
      key: 'active',
      header: 'Статус',
      render: (row) => (
        <Badge variant={row.active ? 'success' : 'default'}>
          {row.active ? 'Активна' : 'Архив'}
        </Badge>
      ),
    },
  ];

  return (
    <>
      <PageHeader
        title="Квоты агрегатора"
        breadcrumbs={[
          { label: 'Админ', href: '/admin/dashboard' },
          { label: 'Агрегаторы', href: '/admin/aggregators' },
          { label: 'Квоты' },
        ]}
        actions={<Button onClick={openCreate}>+ Создать квоту</Button>}
      />

      {/* Active Quota Card */}
      <div className="mb-6">
        <h2 className="text-base font-semibold text-gray-700 dark:text-slate-300 mb-3">Текущая активная квота</h2>
        {activeQuota ? (
          <div className="bg-white dark:bg-slate-900 border border-gray-200 dark:border-slate-700 rounded-lg p-5 space-y-4">
            <div className="flex items-start justify-between">
              <div className="grid grid-cols-2 gap-x-10 gap-y-2 text-sm">
                <div>
                  <span className="text-gray-500 dark:text-slate-400">Период:</span>{' '}
                  <span className="font-medium">
                    {formatDate(activeQuota.period_start)} — {formatDate(activeQuota.period_end)}
                  </span>
                </div>
                <div>
                  <span className="text-gray-500 dark:text-slate-400">Лимит сегментов:</span>{' '}
                  <span className="font-mono font-medium">{activeQuota.segment_limit.toLocaleString('ru-RU')}</span>
                </div>
                <div>
                  <span className="text-gray-500 dark:text-slate-400">Использовано:</span>{' '}
                  <span className="font-mono font-medium">{activeQuota.segments_used.toLocaleString('ru-RU')}</span>
                </div>
                <div>
                  <span className="text-gray-500 dark:text-slate-400">Ставка превышения:</span>{' '}
                  <span className="font-mono font-medium">
                    {activeQuota.overage_rate} {activeQuota.currency}
                  </span>
                </div>
                <div>
                  <span className="text-gray-500 dark:text-slate-400">Авто-продление:</span>{' '}
                  <Badge variant={activeQuota.auto_renew ? 'success' : 'default'}>
                    {activeQuota.auto_renew ? 'Да' : 'Нет'}
                  </Badge>
                </div>
              </div>
              <Button size="sm" variant="ghost" onClick={() => openEdit(activeQuota)}>
                Изменить
              </Button>
            </div>

            {/* Utilization progress bar */}
            <div>
              <div className="flex justify-between text-xs text-gray-500 dark:text-slate-400 mb-1">
                <span>Использование</span>
                <span>{utilizationPct}%</span>
              </div>
              <div className="w-full bg-gray-100 dark:bg-slate-800 rounded-full h-3 overflow-hidden">
                <div
                  className={`h-3 rounded-full transition-all ${utilizationColor(utilizationPct)}`}
                  style={{ width: `${utilizationPct}%` }}
                />
              </div>
            </div>
          </div>
        ) : (
          <div className="bg-white dark:bg-slate-900 border border-gray-200 dark:border-slate-700 rounded-lg p-5 text-sm text-gray-500 dark:text-slate-400">
            Нет активной квоты. Создайте новую квоту с помощью кнопки выше.
          </div>
        )}
      </div>

      {/* History Table */}
      <h2 className="text-base font-semibold text-gray-700 dark:text-slate-300 mb-3">История квот</h2>
      <div className="bg-white dark:bg-slate-900 border border-gray-200 dark:border-slate-700 rounded-lg overflow-hidden">
        <DataTable
          columns={columns}
          data={quotas}
          total={total}
          page={1}
          pageSize={quotas.length || 1}
          onPageChange={() => {}}
          loading={loading}
          rowActions={(row) => (
            <Button size="sm" variant="ghost" onClick={() => openEdit(row)}>
              Изменить
            </Button>
          )}
        />
      </div>

      {/* Create Dialog */}
      <Modal open={showCreate} onClose={() => setShowCreate(false)} title="Создать квоту">
        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <Input
              label="Начало периода *"
              type="date"
              value={createForm.period_start}
              onChange={(e) => setCreateForm((f) => ({ ...f, period_start: e.target.value }))}
            />
            <Input
              label="Конец периода *"
              type="date"
              value={createForm.period_end}
              onChange={(e) => setCreateForm((f) => ({ ...f, period_end: e.target.value }))}
            />
          </div>
          <Input
            label="Лимит сегментов *"
            type="number"
            value={createForm.segment_limit}
            onChange={(e) => setCreateForm((f) => ({ ...f, segment_limit: e.target.value }))}
            placeholder="1000000"
          />
          <div className="grid grid-cols-2 gap-4">
            <Input
              label="Ставка за превышение *"
              value={createForm.overage_rate}
              onChange={(e) => setCreateForm((f) => ({ ...f, overage_rate: e.target.value }))}
              placeholder="0.05"
            />
            <Input
              label="Валюта"
              value={createForm.currency}
              onChange={(e) => setCreateForm((f) => ({ ...f, currency: e.target.value }))}
              placeholder="RUB"
            />
          </div>
          <label className="flex items-center gap-2 text-sm cursor-pointer">
            <input
              type="checkbox"
              className="rounded border-gray-300 dark:border-slate-600"
              checked={createForm.auto_renew}
              onChange={(e) => setCreateForm((f) => ({ ...f, auto_renew: e.target.checked }))}
            />
            <span>Авто-продление</span>
          </label>
          <div className="flex gap-2 justify-end pt-2">
            <Button variant="ghost" onClick={() => setShowCreate(false)}>
              Отмена
            </Button>
            <Button onClick={handleCreate} disabled={creating}>
              {creating ? 'Создание...' : 'Создать'}
            </Button>
          </div>
        </div>
      </Modal>

      {/* Edit Dialog */}
      <Modal open={!!editTarget} onClose={() => setEditTarget(null)} title="Изменить квоту">
        <div className="space-y-4">
          <Input
            label="Лимит сегментов *"
            type="number"
            value={editForm.segment_limit}
            onChange={(e) => setEditForm((f) => ({ ...f, segment_limit: e.target.value }))}
          />
          <Input
            label="Ставка за превышение *"
            value={editForm.overage_rate}
            onChange={(e) => setEditForm((f) => ({ ...f, overage_rate: e.target.value }))}
          />
          <label className="flex items-center gap-2 text-sm cursor-pointer">
            <input
              type="checkbox"
              className="rounded border-gray-300 dark:border-slate-600"
              checked={editForm.auto_renew}
              onChange={(e) => setEditForm((f) => ({ ...f, auto_renew: e.target.checked }))}
            />
            <span>Авто-продление</span>
          </label>
          <div className="flex gap-2 justify-end pt-2">
            <Button variant="ghost" onClick={() => setEditTarget(null)}>
              Отмена
            </Button>
            <Button onClick={handleEdit} disabled={saving}>
              {saving ? 'Сохранение...' : 'Сохранить'}
            </Button>
          </div>
        </div>
      </Modal>
    </>
  );
}
