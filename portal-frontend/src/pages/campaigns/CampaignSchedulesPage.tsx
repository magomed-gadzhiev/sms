import { useState, useEffect, useCallback } from 'react';
import { campaignSchedulesApi, type CampaignSchedule, ApiError } from '../../api/client';
import { campaignsApi, type Campaign } from '../../api/campaigns';
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

interface EditForm {
  name: string;
  frequency: string;
  cron_expression: string;
  max_runs: string;
}

export function CampaignSchedulesPage() {
  const [schedules, setSchedules] = useState<CampaignSchedule[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Create modal state
  const [showCreate, setShowCreate] = useState(false);
  const [form, setForm] = useState<CreateForm>(EMPTY_FORM);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState('');

  // Campaigns for template picker
  const [campaigns, setCampaigns] = useState<Campaign[]>([]);
  const [campaignsLoading, setCampaignsLoading] = useState(false);
  const [campaignsError, setCampaignsError] = useState(false);

  // Delete state
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState('');

  // Toggle error state
  const [toggleError, setToggleError] = useState('');

  // Edit modal state
  const [editId, setEditId] = useState<string | null>(null);
  const [editForm, setEditForm] = useState<EditForm>({ name: '', frequency: 'daily', cron_expression: '', max_runs: '' });
  const [editing, setEditing] = useState(false);
  const [editError, setEditError] = useState('');

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

  async function loadCampaigns() {
    setCampaignsError(false);
    setCampaignsLoading(true);
    try {
      const resp = await campaignsApi.list(1, 200);
      setCampaigns(resp.campaigns ?? []);
    } catch {
      setCampaigns([]);
      setCampaignsError(true);
    } finally {
      setCampaignsLoading(false);
    }
  }

  async function openCreateModal() {
    setForm(EMPTY_FORM);
    setCreateError('');
    setShowCreate(true);
    await loadCampaigns();
  }

  async function handleToggle(s: CampaignSchedule) {
    const newActive = !s.is_active;
    setToggleError('');
    // Optimistic update
    setSchedules((prev) =>
      prev.map((item) => (item.id === s.id ? { ...item, is_active: newActive } : item)),
    );
    try {
      await campaignSchedulesApi.toggle(s.id, newActive);
    } catch (err) {
      // Revert on error and show message
      setSchedules((prev) =>
        prev.map((item) => (item.id === s.id ? { ...item, is_active: s.is_active } : item)),
      );
      setToggleError(err instanceof ApiError ? err.message : 'Не удалось изменить статус расписания');
    }
  }

  async function handleCreate() {
    setCreateError('');
    if (!form.name.trim()) {
      setCreateError('Укажите название расписания');
      return;
    }
    if (!form.template_campaign_id) {
      setCreateError('Выберите кампанию-шаблон');
      return;
    }
    if (!form.frequency) {
      setCreateError('Выберите частоту');
      return;
    }
    if (form.frequency === 'custom' && !form.cron_expression.trim()) {
      setCreateError('Укажите cron-выражение для частоты "По расписанию"');
      return;
    }
    if (form.max_runs && parseInt(form.max_runs, 10) <= 0) {
      setCreateError('Макс. запусков должно быть положительным числом');
      return;
    }
    setCreating(true);
    try {
      await campaignSchedulesApi.create({
        name: form.name.trim(),
        template_campaign_id: form.template_campaign_id,
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

  function openEditModal(s: CampaignSchedule) {
    setEditId(s.id);
    setEditForm({
      name: s.name,
      frequency: s.frequency,
      cron_expression: s.cron_expression ?? '',
      max_runs: s.max_runs != null ? String(s.max_runs) : '',
    });
    setEditError('');
  }

  async function handleEdit() {
    if (!editId) return;
    setEditError('');
    if (!editForm.name.trim()) {
      setEditError('Укажите название расписания');
      return;
    }
    if (!editForm.frequency) {
      setEditError('Выберите частоту');
      return;
    }
    if (editForm.frequency === 'custom' && !editForm.cron_expression.trim()) {
      setEditError('Укажите cron-выражение для частоты "По расписанию"');
      return;
    }
    if (editForm.max_runs && parseInt(editForm.max_runs, 10) <= 0) {
      setEditError('Макс. запусков должно быть положительным числом');
      return;
    }
    setEditing(true);
    try {
      await campaignSchedulesApi.update(editId, {
        name: editForm.name.trim(),
        frequency: editForm.frequency,
        cron_expression: editForm.frequency === 'custom' && editForm.cron_expression.trim()
          ? editForm.cron_expression.trim()
          : undefined,
        max_runs: editForm.max_runs ? parseInt(editForm.max_runs, 10) : undefined,
        clear_max_runs: !editForm.max_runs,
      });
      setEditId(null);
      await load();
    } catch (err) {
      setEditError(err instanceof ApiError ? err.message : 'Ошибка сохранения расписания');
    } finally {
      setEditing(false);
    }
  }

  async function confirmDelete() {
    if (!deleteId) return;
    setDeleting(true);
    setDeleteError('');
    try {
      await campaignSchedulesApi.remove(deleteId);
      setSchedules((prev) => prev.filter((s) => s.id !== deleteId));
      setDeleteId(null);
    } catch (err) {
      setDeleteError(err instanceof ApiError ? err.message : 'Ошибка при удалении');
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
      key: 'template_campaign_name',
      header: 'Кампания-шаблон',
      responsive: true,
      render: (s) => (
        <span className="text-gray-600 text-sm">
          {s.template_campaign_name || <span className="text-gray-400 italic">удалена</span>}
        </span>
      ),
    },
    {
      key: 'frequency',
      header: 'Частота',
      render: (s) => (
        <span>
          {FREQUENCY_LABELS[s.frequency] ?? s.frequency}
          {s.frequency === 'custom' && s.cron_expression && (
            <code className="ml-1 text-xs text-gray-400 font-mono">({s.cron_expression})</code>
          )}
        </span>
      ),
    },
    {
      key: 'last_run_at',
      header: 'Последний запуск',
      responsive: true,
      render: (s) => <span className="text-gray-500 text-sm">{formatDate(s.last_run_at)}</span>,
    },
    {
      key: 'next_run_at',
      header: 'Следующий запуск',
      responsive: true,
      render: (s) => (
        <span className="text-gray-500 text-sm">
          {s.is_active ? formatDate(s.next_run_at) : '—'}
        </span>
      ),
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
          <Button onClick={openCreateModal}>
            + Новое расписание
          </Button>
        }
      />

      {error && <p className="text-red-600 mb-4">{error}</p>}
      {toggleError && (
        <p className="text-red-600 mb-4 text-sm">{toggleError}</p>
      )}

      {!loading && !error && schedules.length === 0 && (
        <div className="text-center py-12 text-gray-500">
          <p className="mb-2">Расписания не созданы</p>
          <p className="text-sm mb-4">Создайте первое расписание для автоматических рассылок</p>
          <Button variant="ghost" onClick={openCreateModal}>
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
                aria-label={`Редактировать расписание ${s.name}`}
                onClick={() => openEditModal(s)}
              >
                Изменить
              </Button>
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
              Кампания-шаблон
            </label>
            {campaignsLoading ? (
              <select disabled className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm bg-gray-50 text-gray-400">
                <option>Загрузка кампаний...</option>
              </select>
            ) : campaignsError ? (
              <p className="text-sm text-red-600">
                Не удалось загрузить кампании.{' '}
                <button type="button" className="underline" onClick={loadCampaigns}>
                  Повторить
                </button>
              </p>
            ) : campaigns.length === 0 ? (
              <p className="text-sm text-amber-600">
                Нет доступных кампаний. Создайте кампанию, чтобы использовать её как шаблон.
              </p>
            ) : (
              <select
                className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                value={form.template_campaign_id}
                onChange={(e) => setForm((f) => ({ ...f, template_campaign_id: e.target.value }))}
              >
                <option value="">— выберите кампанию —</option>
                {campaigns.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </select>
            )}
            <p className="text-xs text-gray-400 mt-1">
              Настройки (список контактов, шаблон сообщения) будут скопированы из выбранной кампании.
            </p>
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
              <p className="text-xs text-gray-400 mt-1">
                Формат: минута час день-месяца месяц день-недели. Пример: <code>0 9 * * 1</code> — каждый понедельник в 9:00.
              </p>
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
            <Button onClick={handleCreate} disabled={creating || campaignsLoading || campaigns.length === 0}>
              {creating ? 'Создание...' : 'Создать'}
            </Button>
          </div>
        </div>
      </Modal>

      {/* Edit modal */}
      <Modal
        open={editId !== null}
        onClose={() => setEditId(null)}
        title="Редактировать расписание"
      >
        <div className="flex flex-col gap-4">
          {editError && <p className="text-red-600 text-sm">{editError}</p>}

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Название</label>
            <input
              type="text"
              className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              value={editForm.name}
              onChange={(e) => setEditForm((f) => ({ ...f, name: e.target.value }))}
              placeholder="Название расписания"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Частота</label>
            <select
              className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              value={editForm.frequency}
              onChange={(e) => setEditForm((f) => ({ ...f, frequency: e.target.value }))}
            >
              <option value="daily">Ежедневно</option>
              <option value="weekly">Еженедельно</option>
              <option value="monthly">Ежемесячно</option>
              <option value="custom">По расписанию (cron)</option>
            </select>
          </div>

          {editForm.frequency === 'custom' && (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Cron-выражение
              </label>
              <input
                type="text"
                className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                value={editForm.cron_expression}
                onChange={(e) => setEditForm((f) => ({ ...f, cron_expression: e.target.value }))}
                placeholder="0 9 * * 1"
              />
              <p className="text-xs text-gray-400 mt-1">
                Формат: минута час день-месяца месяц день-недели. Пример: <code>0 9 * * 1</code> — каждый понедельник в 9:00.
              </p>
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
              value={editForm.max_runs}
              onChange={(e) => setEditForm((f) => ({ ...f, max_runs: e.target.value }))}
              placeholder="Без ограничений"
            />
          </div>

          <div className="flex justify-end gap-2 pt-2">
            <Button variant="secondary" onClick={() => setEditId(null)} disabled={editing}>
              Отмена
            </Button>
            <Button onClick={handleEdit} disabled={editing}>
              {editing ? 'Сохранение...' : 'Сохранить'}
            </Button>
          </div>
        </div>
      </Modal>

      <ConfirmDialog
        open={deleteId !== null}
        onConfirm={confirmDelete}
        onCancel={() => { setDeleteId(null); setDeleteError(''); }}
        title="Удалить расписание"
        description={
          deleteError
            ? deleteError
            : 'Вы уверены, что хотите удалить это расписание? Это действие нельзя отменить.'
        }
        confirmLabel="Удалить"
        variant="danger"
        loading={deleting}
      />
    </div>
  );
}
