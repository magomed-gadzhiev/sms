import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../../components/layout/PageHeader';
import { DataTable, type Column } from '../../../components/data/DataTable';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { Select } from '../../../components/ui/Select';
import { Modal } from '../../../components/ui/Modal';
import { ConfirmDialog } from '../../../components/ui/ConfirmDialog';
import { Badge } from '../../../components/ui/Badge';
import { useToast } from '../../../components/ui/Toast';
import {
  clientsApi,
  operatorsApi,
  providersApi,
  clientRoutesApi,
  type ClientInfo,
  type OperatorInfo,
  type ProviderInfo,
  type ClientRoute,
} from '../../../api/admin';

interface FormState {
  operator_id: string;
  provider_id: string;
  priority: number;
  weight: number;
  active: boolean;
}

const EMPTY_FORM: FormState = {
  operator_id: '',
  provider_id: '',
  priority: 1,
  weight: 1,
  active: true,
};

export function ClientRoutesPage() {
  const toast = useToast();

  const [clients, setClients] = useState<ClientInfo[]>([]);
  const [selectedClientId, setSelectedClientId] = useState('');
  const [routes, setRoutes] = useState<ClientRoute[]>([]);
  const [operators, setOperators] = useState<OperatorInfo[]>([]);
  const [providers, setProviders] = useState<ProviderInfo[]>([]);
  const [loading, setLoading] = useState(false);

  const [showForm, setShowForm] = useState(false);
  const [editRoute, setEditRoute] = useState<ClientRoute | null>(null);
  const [deleteRoute, setDeleteRoute] = useState<ClientRoute | null>(null);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState<FormState>(EMPTY_FORM);

  const operatorMap = Object.fromEntries(operators.map((o) => [o.operator_id, o.name]));
  const providerMap = Object.fromEntries(providers.map((p) => [p.provider_id, p.name]));

  const clientOptions = clients.map((c) => ({ value: c.client_id, label: c.name }));
  const operatorOptions = operators.map((o) => ({ value: o.operator_id, label: o.name }));
  const providerOptions = providers.map((p) => ({ value: p.provider_id, label: p.name }));

  useEffect(() => {
    clientsApi.list({ limit: 500 }).then((r) => setClients(r.clients || [])).catch(() => {});
    operatorsApi.list({ limit: 500 }).then((r) => setOperators(r.operators || [])).catch(() => {});
    providersApi.list({ limit: 500 }).then((r) => setProviders(r.providers || [])).catch(() => {});
  }, []);

  const fetchRoutes = useCallback(async () => {
    if (!selectedClientId) return;
    setLoading(true);
    try {
      const res = await clientRoutesApi.list(selectedClientId);
      setRoutes(res.routes || []);
    } catch {
      toast.error('Не удалось загрузить маршруты');
    } finally {
      setLoading(false);
    }
  }, [selectedClientId, toast]);

  useEffect(() => { fetchRoutes(); }, [fetchRoutes]);

  function openCreate() {
    setEditRoute(null);
    setForm(EMPTY_FORM);
    setShowForm(true);
  }

  function openEdit(r: ClientRoute) {
    setEditRoute(r);
    setForm({ operator_id: r.operator_id, provider_id: r.provider_id, priority: r.priority, weight: r.weight, active: r.active });
    setShowForm(true);
  }

  async function handleSave() {
    if (!selectedClientId) return;
    if (!form.operator_id || !form.provider_id) {
      toast.error('Выберите оператора и провайдера');
      return;
    }
    setSaving(true);
    try {
      if (editRoute) {
        await clientRoutesApi.update(selectedClientId, editRoute.id, { priority: form.priority, weight: form.weight, active: form.active });
        toast.success('Маршрут обновлён');
      } else {
        await clientRoutesApi.create(selectedClientId, { operator_id: form.operator_id, provider_id: form.provider_id, priority: form.priority, weight: form.weight });
        toast.success('Маршрут создан');
      }
      setShowForm(false);
      fetchRoutes();
    } catch {
      toast.error('Не удалось сохранить маршрут');
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete() {
    if (!deleteRoute || !selectedClientId) return;
    try {
      await clientRoutesApi.delete(selectedClientId, deleteRoute.id);
      toast.success('Маршрут удалён');
      setDeleteRoute(null);
      fetchRoutes();
    } catch {
      toast.error('Не удалось удалить маршрут');
    }
  }

  const selectedClient = clients.find((c) => c.client_id === selectedClientId);

  const columns: Column<ClientRoute>[] = [
    {
      key: 'operator_id',
      header: 'Оператор',
      render: (r) => operatorMap[r.operator_id] || r.operator_id.slice(0, 8),
    },
    {
      key: 'provider_id',
      header: 'Провайдер',
      render: (r) => providerMap[r.provider_id] || r.provider_id.slice(0, 8),
    },
    { key: 'priority', header: 'Приоритет', sortable: true },
    { key: 'weight', header: 'Вес', sortable: true },
    {
      key: 'active',
      header: 'Статус',
      render: (r) => (
        <Badge variant={r.active ? 'success' : 'default'}>{r.active ? 'Активен' : 'Неактивен'}</Badge>
      ),
    },
  ];

  return (
    <>
      <PageHeader
        title="Индивидуальные маршруты"
        breadcrumbs={[
          { label: 'Админ', href: '/admin/dashboard' },
          { label: 'Индивидуальные маршруты' },
        ]}
      />

      <div className="mb-4 flex items-end gap-4">
        <div className="w-80">
          <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1">Клиент</label>
          <Select
            value={selectedClientId}
            onChange={setSelectedClientId}
            options={clientOptions}
            placeholder="Выберите клиента..."
          />
        </div>
        {selectedClientId && (
          <Button onClick={openCreate}>+ Добавить маршрут</Button>
        )}
      </div>

      {selectedClientId && (
        <>
          {selectedClient && (
            <div className="mb-4 p-3 bg-blue-50 dark:bg-blue-950/40 border border-blue-200 dark:border-blue-800 rounded-md text-sm text-blue-800 dark:text-blue-300">
              Маршруты клиента: <strong>{selectedClient.name}</strong>
            </div>
          )}
          <DataTable
            columns={columns}
            data={routes}
            total={routes.length}
            page={1}
            pageSize={routes.length || 1}
            onPageChange={() => {}}
            loading={loading}
            rowActions={(r) => (
              <div className="flex gap-2">
                <Button size="sm" variant="ghost" onClick={() => openEdit(r)}>Изменить</Button>
                <Button size="sm" variant="ghost" onClick={() => setDeleteRoute(r)}>Удалить</Button>
              </div>
            )}
          />
        </>
      )}

      {!selectedClientId && (
        <div className="text-center py-16 text-gray-400 dark:text-slate-500">
          Выберите клиента для просмотра его маршрутов
        </div>
      )}

      <Modal
        open={showForm}
        onClose={() => setShowForm(false)}
        title={editRoute ? 'Изменить маршрут' : 'Добавить маршрут'}
      >
        <div className="space-y-4">
          {!editRoute && (
            <>
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1">Оператор *</label>
                <Select
                  value={form.operator_id}
                  onChange={(v) => setForm((f) => ({ ...f, operator_id: v }))}
                  options={operatorOptions}
                  placeholder="Выберите оператора..."
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1">Провайдер *</label>
                <Select
                  value={form.provider_id}
                  onChange={(v) => setForm((f) => ({ ...f, provider_id: v }))}
                  options={providerOptions}
                  placeholder="Выберите провайдера..."
                />
              </div>
            </>
          )}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1">Приоритет</label>
              <Input
                type="number"
                min={1}
                value={String(form.priority)}
                onChange={(e) => setForm((f) => ({ ...f, priority: Number(e.target.value) }))}
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1">Вес</label>
              <Input
                type="number"
                min={1}
                value={String(form.weight)}
                onChange={(e) => setForm((f) => ({ ...f, weight: Number(e.target.value) }))}
              />
            </div>
          </div>
          {editRoute && (
            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                id="route-active"
                checked={form.active}
                onChange={(e) => setForm((f) => ({ ...f, active: e.target.checked }))}
                className="rounded"
              />
              <label htmlFor="route-active" className="text-sm text-gray-700 dark:text-slate-300">Активен</label>
            </div>
          )}
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="ghost" onClick={() => setShowForm(false)}>Отмена</Button>
            <Button onClick={handleSave} disabled={saving}>
              {saving ? 'Сохранение...' : 'Сохранить'}
            </Button>
          </div>
        </div>
      </Modal>

      <ConfirmDialog
        open={!!deleteRoute}
        title="Удалить маршрут?"
        description={`Маршрут ${deleteRoute ? `(${operatorMap[deleteRoute.operator_id] || ''} → ${providerMap[deleteRoute.provider_id] || ''})` : ''} будет удалён.`}
        onConfirm={handleDelete}
        onCancel={() => setDeleteRoute(null)}
      />
    </>
  );
}
