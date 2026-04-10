import { useState, useEffect, useCallback } from 'react';
import { campaignSchedulesApi, type CampaignSchedule, ApiError } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Badge } from '../../components/ui/Badge';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { Modal } from '../../components/ui/Modal';
import { DataTable, type Column } from '../../components/data/DataTable';

const FREQUENCY_LABELS: Record<string, string> = {
  daily: 'Ежедневно',
  weekly: 'Еженедельно',
  monthly: 'Ежемесячно',
  custom: 'По расписанию',
};

function formatDate(s?: string): string {
  if (!s) return '—';
  const d = new Date(s);
  return isNaN(d.getTime()) ? '—' : d.toLocaleString('ru-RU');
}

interface CreateForm {
  name: string;
  template_campaign_id: string;
  frequency: string;
  cron_expression: string;
  max_runs: string;
}

const EMPTY_FORM: CreateForm = {
  name: '',
  template_campaign_id: '',
  frequency: 'daily',
  cron_expression: '',
  max_runs: '',
};

export function CampaignSchedulesPage() {
  const [schedules, setSchedules] = useState<CampaignSchedule[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Create modal state
  const [showCreate, setShowCreate] = useState(false);
  const [form, setForm] = useState<CreateForm>(EMPTY_FORM);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState('');

  // Delete state
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const resp = await campaignSchedulesApi.list();
      setSchedules(resp.schedules ?? []);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось загрузить расписания');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  async function handleToggle(s: CampaignSchedule) {
    const newActive = !s.is_active;
    // Optimistic update
    setSchedules((prev) =>
      prev.map((item) => (item.id === s.id ? { ...item, is_active: newActive } : item)),
    );
    try {
      await campaignSchedulesApi.toggle(s.id, newActive);
    } catch {
      // Revert on error
      setSchedules((prev) =>
        prev.map((item) => (item.id === s.id ? { ...item, is_active: s.is_active } : item)),
      );
    }
  }

  async function handleCreate() {
    setCreateError('');
    if (!form.name.trim() || !form.template_campaign_id.trim() || !form.frequency) {
      setCreateError('Название, ID кампании-шаблона и частота обязательны');
      return;
    }
    setCreating(true);
    try {
      await campaignSchedulesApi.create({
        name: form.name.trim(),
        template_campaign_id: form.template_campaign_id.trim(),
        frequency: form.frequency,
        cron_expression: form.frequency === 'custom' && form.cron_expression.trim()
          ? form.cron_expression.trim()
          : undefined,
        max_runs: form.max_runs ? parseInt(form.max_runs, 10) : undefined,
      });
      setShowCreate(false);
      setForm(EMPTY_FORM);
      await load();
    } catch (err) {
      setCreateError(err instanceof ApiError ? err.message : 'Ошибка создания расписания');
    } finally {
      setCreating(false);
    }
  }

  async function confirmDelete() {
    if (!deleteId) return;
    setDeleting(true);
    try {
      await campaignSchedulesApi.remove(deleteId);
      setSchedules((prev) => prev.filter((s) => s.id !== deleteId));
      setDeleteId(null);
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'Ошибка при удалении');
    } finally {
      setDeleting(false);
    }
  }

  const columns: Column<CampaignSchedule>[] = [
    {
      key: 'name',
      header: 'Название',
      render: (s) => <span className="font-medium text-gray-900">{s.name}</span>,
    },
    {
      key: 'frequency',
      header: 'Частота',
      render: (s) => <span>{FREQUENCY_LABELS[s.frequency] ?? s.frequency}</span>,
    },
    {
      key: 'next_run_at',
      header: 'Следующий запуск',
      responsive: true,
      render: (s) => <span className="text-gray-500 text-sm">{formatDate(s.next_run_at)}</span>,
    },
    {
      key: 'run_count',
      header: 'Запусков',
      responsive: true,
      render: (s) => (
        <span className="font-mono text-sm">
          {s.run_count}{s.max_runs != null ? ` / ${s.max_runs}` : ''}
        </span>
      ),
    },
    {
      key: 'is_active',
      header: 'Активна',
      render: (s) => (
        <Badge variant={s.is_active ? 'success' : 'default'}>
          {s.is_active ? 'Да' : 'Нет'}
        </Badge>
      ),
    },
  ];

  return (
    <div>
      <PageHeader
        title="Повторяющиеся рассылки"
        subtitle="Автоматические рассылки по расписанию"
        actions={
          <Button onClick={() => { setForm(EMPTY_FORM); setCreateError(''); setShowCreate(true); }}>
            + Новое расписание
          </Button>
        }
      />

      {error && <p className="text-red-600 mb-4">{error}</p>}

      {!loading && !error && schedules.length === 0 && (
        <div className="text-center py-12 text-gray-500">
          <p className="mb-2">Расписания не созданы</p>
          <p className="text-sm mb-4">Создайте первое расписание для автоматических рассылок</p>
          <Button variant="ghost" onClick={() => { setForm(EMPTY_FORM); setCreateError(''); setShowCreate(true); }}>
            Создать расписание
          </Button>
        </div>
      )}

      {(loading || schedules.length > 0) && (
        <DataTable
          columns={columns}
          data={schedules}
          total={schedules.length}
          page={1}
          pageSize={schedules.length || 1}
          onPageChange={() => {}}
          loading={loading}
          keyField="id"
          tableLabel="Список расписаний"
          rowActions={(s) => (
            <div className="flex gap-2">
              <Button
                variant="secondary"
                size="sm"
                aria-label={`${s.is_active ? 'Деактивировать' : 'Активировать'} расписание ${s.name}`}
                onClick={() => handleToggle(s)}
              >
                {s.is_active ? 'Деактивировать' : 'Активировать'}
              </Button>
              <Button
                variant="danger"
                size="sm"
                aria-label={`Удалить расписание ${s.name}`}
                onClick={() => setDeleteId(s.id)}
              >
                Удалить
              </Button>
            </div>
          )}
        />
      )}

      {/* Create modal */}
      <Modal
        open={showCreate}
        onClose={() => setShowCreate(false)}
        title="Новое расписание"
      >
        <div className="flex flex-col gap-4">
          {createError && <p className="text-red-600 text-sm">{createError}</p>}

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Название</label>
            <input
              type="text"
              className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              value={form.name}
              onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
              placeholder="Название расписания"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              ID кампании-шаблона
            </label>
            <input
              type="text"
              className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              value={form.template_campaign_id}
              onChange={(e) => setForm((f) => ({ ...f, template_campaign_id: e.target.value }))}
              placeholder="UUID кампании"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Частота</label>
            <select
              className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              value={form.frequency}
              onChange={(e) => setForm((f) => ({ ...f, frequency: e.target.value }))}
            >
              <option value="daily">Ежедневно</option>
              <option value="weekly">Еженедельно</option>
              <option value="monthly">Ежемесячно</option>
              <option value="custom">По расписанию (cron)</option>
            </select>
          </div>

          {form.frequency === 'custom' && (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Cron-выражение
              </label>
              <input
                type="text"
                className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                value={form.cron_expression}
                onChange={(e) => setForm((f) => ({ ...f, cron_expression: e.target.value }))}
                placeholder="0 9 * * 1"
              />
            </div>
          )}

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Макс. запусков (необязательно)
            </label>
            <input
              type="number"
              min="1"
              className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              value={form.max_runs}
              onChange={(e) => setForm((f) => ({ ...f, max_runs: e.target.value }))}
              placeholder="Без ограничений"
            />
          </div>

          <div className="flex justify-end gap-2 pt-2">
            <Button variant="secondary" onClick={() => setShowCreate(false)} disabled={creating}>
              Отмена
            </Button>
            <Button onClick={handleCreate} disabled={creating}>
              {creating ? 'Создание...' : 'Создать'}
            </Button>
          </div>
        </div>
      </Modal>

      <ConfirmDialog
        open={deleteId !== null}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteId(null)}
        title="Удалить расписание"
        description="Вы уверены, что хотите удалить это расписание? Это действие нельзя отменить."
        confirmLabel="Удалить"
        variant="danger"
        loading={deleting}
      />
    </div>
  );
}
