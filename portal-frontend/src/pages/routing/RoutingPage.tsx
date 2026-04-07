import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Select } from '../../components/ui/Select';
import { Modal } from '../../components/ui/Modal';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { StatusBadge } from '../../components/ui/Badge';
import { DataTable, type Column } from '../../components/data/DataTable';
import {
  routingApi,
  providersApi,
  ApiError,
  type ClientRoute,
  type OperatorInfo,
  type Provider,
} from '../../api/client';

export function RoutingPage() {
  const [routingMode, setRoutingMode] = useState('legacy');
  const [routes, setRoutes] = useState<ClientRoute[]>([]);
  const [operators, setOperators] = useState<OperatorInfo[]>([]);
  const [providers, setProviders] = useState<Provider[]>([]);
  const [strategy, setStrategy] = useState('priority');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showAddModal, setShowAddModal] = useState(false);
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [saving, setSaving] = useState(false);

  // Form state
  const [formOperator, setFormOperator] = useState('');
  const [formProvider, setFormProvider] = useState('');
  const [formPriority, setFormPriority] = useState('100');
  const [formWeight, setFormWeight] = useState('100');
  const [formError, setFormError] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const [modeResp, routesResp, opsResp, provsResp] = await Promise.all([
        routingApi.getMode(),
        routingApi.listRoutes(),
        routingApi.listOperators(),
        providersApi.list(),
      ]);
      setRoutingMode(modeResp.routing_mode);
      setRoutes(routesResp.routes ?? []);
      setOperators(opsResp.operators ?? []);
      setProviders(provsResp.providers ?? []);

      // Try to get strategy, ignore error (may not be set yet)
      try {
        const stratResp = await routingApi.getStrategy();
        if (stratResp.strategy) setStrategy(stratResp.strategy);
      } catch {
        // Strategy not configured yet, use default
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось загрузить данные маршрутизации');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const operatorMap = Object.fromEntries(operators.map(o => [o.id, o]));
  const providerMap = Object.fromEntries(providers.map(p => [p.id, p]));

  async function handleModeChange(mode: string) {
    setSaving(true);
    try {
      await routingApi.setMode(mode);
      setRoutingMode(mode);
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'Не удалось изменить режим');
    } finally {
      setSaving(false);
    }
  }

  async function handleStrategyChange(strat: string) {
    setSaving(true);
    try {
      await routingApi.setStrategy({ strategy: strat });
      setStrategy(strat);
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'Не удалось изменить стратегию');
    } finally {
      setSaving(false);
    }
  }

  async function handleAddRoute() {
    setFormError('');
    if (!formOperator || !formProvider) {
      setFormError('Выберите оператора и провайдера');
      return;
    }
    setSaving(true);
    try {
      const route = await routingApi.createRoute({
        operator_id: formOperator,
        provider_id: formProvider,
        priority: parseInt(formPriority) || 100,
        weight: parseInt(formWeight) || 100,
      });
      setRoutes(prev => [...prev, route]);
      setShowAddModal(false);
      setFormOperator('');
      setFormProvider('');
      setFormPriority('100');
      setFormWeight('100');
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : 'Не удалось создать маршрут');
    } finally {
      setSaving(false);
    }
  }

  async function handleToggleRoute(route: ClientRoute) {
    try {
      await routingApi.updateRoute(route.id, { active: !route.active });
      setRoutes(prev => prev.map(r => r.id === route.id ? { ...r, active: !r.active } : r));
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'Не удалось обновить маршрут');
    }
  }

  async function handleDeleteRoute() {
    if (!deleteId) return;
    setDeleting(true);
    try {
      await routingApi.deleteRoute(deleteId);
      setRoutes(prev => prev.filter(r => r.id !== deleteId));
      setDeleteId(null);
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'Не удалось удалить маршрут');
    } finally {
      setDeleting(false);
    }
  }

  const isCustomRouting = routingMode === 'new' || routingMode === 'hybrid';

  const columns: Column<ClientRoute>[] = [
    {
      key: 'operator_id',
      header: 'Оператор',
      render: (r) => {
        const op = operatorMap[r.operator_id];
        return op ? `${op.name} (${op.code})` : r.operator_id;
      },
    },
    {
      key: 'provider_id',
      header: 'Провайдер',
      render: (r) => {
        const prov = providerMap[r.provider_id];
        return prov ? prov.name : r.provider_id;
      },
    },
    { key: 'priority', header: 'Приоритет' },
    { key: 'weight', header: 'Вес' },
    {
      key: 'active',
      header: 'Статус',
      render: (r) => <StatusBadge status={r.active ? 'active' : 'inactive'} />,
    },
  ];

  return (
    <div>
      <PageHeader
        title="Маршрутизация"
        subtitle="Настройте, через какие провайдеры отправлять сообщения"
      />

      {error && <p className="text-red-600 mb-4">{error}</p>}

      {/* Routing Mode Toggle */}
      <div className="bg-white rounded-lg border p-6 mb-6">
        <h2 className="text-lg font-medium mb-4">Режим маршрутизации</h2>
        <div className="flex items-start gap-6">
          <label className="flex items-start gap-3 cursor-pointer">
            <input
              type="radio"
              name="routing_mode"
              value="legacy"
              checked={routingMode === 'legacy'}
              onChange={() => handleModeChange('legacy')}
              disabled={saving}
              className="mt-1"
            />
            <div>
              <div className="font-medium">Глобальная</div>
              <div className="text-sm text-gray-500">Сообщения идут через глобальные маршруты платформы</div>
            </div>
          </label>
          <label className="flex items-start gap-3 cursor-pointer">
            <input
              type="radio"
              name="routing_mode"
              value="new"
              checked={routingMode === 'new'}
              onChange={() => handleModeChange('new')}
              disabled={saving}
              className="mt-1"
            />
            <div>
              <div className="font-medium">Своя маршрутизация</div>
              <div className="text-sm text-gray-500">Сообщения идут только через ваши маршруты</div>
            </div>
          </label>
          <label className="flex items-start gap-3 cursor-pointer">
            <input
              type="radio"
              name="routing_mode"
              value="hybrid"
              checked={routingMode === 'hybrid'}
              onChange={() => handleModeChange('hybrid')}
              disabled={saving}
              className="mt-1"
            />
            <div>
              <div className="font-medium">Гибридная</div>
              <div className="text-sm text-gray-500">Сначала ваши маршруты, если нет — глобальные</div>
            </div>
          </label>
        </div>
      </div>

      {/* Routes Table — only show when custom routing is enabled */}
      {isCustomRouting && (
        <>
          {/* Strategy selection */}
          <div className="bg-white rounded-lg border p-6 mb-6">
            <h2 className="text-lg font-medium mb-4">Стратегия выбора провайдера</h2>
            <div className="max-w-xs">
              <Select
                label=""
                value={strategy}
                onChange={handleStrategyChange}
                options={[
                  { value: 'priority', label: 'По приоритету — выбирается провайдер с наибольшим приоритетом' },
                  { value: 'weighted', label: 'По весу — случайный выбор с учётом веса' },
                ]}
              />
            </div>
          </div>

          {/* Routes */}
          <div className="bg-white rounded-lg border p-6">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-medium">Маршруты</h2>
              <Button onClick={() => setShowAddModal(true)} disabled={providers.length === 0}>
                + Добавить маршрут
              </Button>
            </div>

            {providers.length === 0 && (
              <p className="text-gray-500 text-sm mb-4">
                Сначала добавьте провайдера на странице «SMPP Провайдеры», затем создайте маршрут.
              </p>
            )}

            {routes.length === 0 && providers.length > 0 && !loading && (
              <p className="text-gray-500 text-sm mb-4">
                Нет настроенных маршрутов. Добавьте маршрут, чтобы сообщения шли через вашего провайдера.
              </p>
            )}

            {(loading || routes.length > 0) && (
              <DataTable
                columns={columns}
                data={routes}
                total={routes.length}
                page={1}
                pageSize={routes.length || 10}
                onPageChange={() => {}}
                loading={loading}
                keyField="id"
                rowActions={(r) => (
                  <div className="flex gap-2">
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => handleToggleRoute(r)}
                    >
                      {r.active ? 'Выкл' : 'Вкл'}
                    </Button>
                    <Button variant="danger" size="sm" onClick={() => setDeleteId(r.id)}>
                      Удалить
                    </Button>
                  </div>
                )}
              />
            )}
          </div>
        </>
      )}

      {/* Add Route Modal */}
      <Modal
        open={showAddModal}
        onClose={() => setShowAddModal(false)}
        title="Добавить маршрут"
        description="Настройте маршрут сообщений через выбранного провайдера"
      >
        <div className="flex flex-col gap-4">
          <Select
            label="Оператор"
            value={formOperator}
            onChange={setFormOperator}
            placeholder="Выберите оператора"
            options={operators.map(o => ({ value: o.id, label: `${o.name} (${o.code})` }))}
          />
          <Select
            label="Провайдер"
            value={formProvider}
            onChange={setFormProvider}
            placeholder="Выберите провайдера"
            options={providers.filter(p => p.active).map(p => ({ value: p.id, label: p.name }))}
          />
          <div className="grid grid-cols-2 gap-4">
            <div className="flex flex-col gap-1">
              <label className="text-sm font-medium text-gray-700">Приоритет</label>
              <input
                type="number"
                value={formPriority}
                onChange={(e) => setFormPriority(e.target.value)}
                className="rounded border border-gray-300 px-3 py-2 text-sm"
                min="1"
              />
              <span className="text-xs text-gray-500">Чем выше число, тем выше приоритет</span>
            </div>
            <div className="flex flex-col gap-1">
              <label className="text-sm font-medium text-gray-700">Вес</label>
              <input
                type="number"
                value={formWeight}
                onChange={(e) => setFormWeight(e.target.value)}
                className="rounded border border-gray-300 px-3 py-2 text-sm"
                min="1"
              />
              <span className="text-xs text-gray-500">Используется при стратегии «По весу»</span>
            </div>
          </div>

          {formError && <p className="text-red-600 text-sm">{formError}</p>}

          <div className="flex justify-end gap-2 mt-2">
            <Button variant="ghost" onClick={() => setShowAddModal(false)}>Отмена</Button>
            <Button onClick={handleAddRoute} disabled={saving}>
              {saving ? 'Сохранение...' : 'Добавить'}
            </Button>
          </div>
        </div>
      </Modal>

      {/* Delete Confirmation */}
      <ConfirmDialog
        open={deleteId !== null}
        onConfirm={handleDeleteRoute}
        onCancel={() => setDeleteId(null)}
        title="Удалить маршрут"
        description="Вы уверены, что хотите удалить этот маршрут?"
        confirmLabel="Удалить"
        variant="danger"
        loading={deleting}
      />
    </div>
  );
}
