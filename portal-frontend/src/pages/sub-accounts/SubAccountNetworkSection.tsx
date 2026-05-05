import { useState, useEffect, useCallback, type FormEvent } from 'react';
import {
  networkApi,
  ApiError,
  type NetworkSubAccountOverview,
  type NetworkProvider,
  type NetworkProviderSet,
  type NetworkRouteSet,
  type NetworkRouteSetItem,
} from '../../api/client';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Select } from '../../components/ui/Select';
import { Modal } from '../../components/ui/Modal';
import { Badge } from '../../components/ui/Badge';
import { useToast } from '../../components/ui/Toast';
import { RouteRuleDrawer } from '../../components/network/RouteRuleDrawer';

interface Props {
  subAccountID: string;
  subAccountName: string;
}

interface AddOverrideForm {
  provider_id: string;
  priority: number;
  expose_cost: boolean;
  expose_provider_name: boolean;
}

const EMPTY_ADD_FORM: AddOverrideForm = {
  provider_id: '',
  priority: 100,
  expose_cost: false,
  expose_provider_name: false,
};

type RouteOverrideData = Omit<NetworkRouteSetItem, 'id' | 'provider_name'>;

export function SubAccountNetworkSection({ subAccountID, subAccountName }: Props) {
  const toast = useToast();
  const [overview, setOverview] = useState<NetworkSubAccountOverview | null>(null);
  const [providers, setProviders] = useState<NetworkProvider[]>([]);
  const [providerSets, setProviderSets] = useState<NetworkProviderSet[]>([]);
  const [routeSets, setRouteSets] = useState<NetworkRouteSet[]>([]);
  const [loadError, setLoadError] = useState('');

  // Modal: change provider-set
  const [changeSetOpen, setChangeSetOpen] = useState(false);
  const [newSetID, setNewSetID] = useState<string>('');
  const [savingSet, setSavingSet] = useState(false);

  // Modal: change route-set
  const [changeRouteSetOpen, setChangeRouteSetOpen] = useState(false);
  const [newRouteSetID, setNewRouteSetID] = useState<string>('');
  const [savingRouteSet, setSavingRouteSet] = useState(false);

  // Modal: add provider override
  const [addOpen, setAddOpen] = useState(false);
  const [addForm, setAddForm] = useState<AddOverrideForm>(EMPTY_ADD_FORM);
  const [savingOverride, setSavingOverride] = useState(false);
  const [removingProviderID, setRemovingProviderID] = useState<string | null>(null);

  // Drawer: route override
  const [routeDrawerOpen, setRouteDrawerOpen] = useState(false);
  const [editingRouteOverride, setEditingRouteOverride] = useState<NetworkRouteSetItem | null>(null);
  const [removingRouteID, setRemovingRouteID] = useState<string | null>(null);

  const loadOverview = useCallback(async () => {
    setLoadError('');
    try {
      const resp = await networkApi.getSubAccountNetworkOverview(subAccountID);
      setOverview(resp);
    } catch (err) {
      setLoadError(err instanceof ApiError ? err.message : 'Не удалось загрузить сетевые настройки');
    }
  }, [subAccountID]);

  useEffect(() => {
    loadOverview();
  }, [loadOverview]);

  useEffect(() => {
    networkApi
      .listProviders()
      .then((r) => setProviders(r.providers || []))
      .catch(() => {
        // Soft-fail: providers list нужны только в модалках; ошибка будет видна там
      });
    networkApi
      .listProviderSets()
      .then((r) => setProviderSets(r.provider_sets || []))
      .catch(() => {
        // Soft-fail: provider-sets нужны только в модалке смены; см. выше
      });
    networkApi
      .listRouteSets()
      .then((r) => setRouteSets(r.route_sets || []))
      .catch(() => {
        // Soft-fail: route-sets нужны только в модалке смены route-set
      });
  }, []);

  async function changeSet() {
    setSavingSet(true);
    try {
      const result = await networkApi.putAssignment(subAccountID, {
        provider_set_id: newSetID || null,
        route_set_id: overview?.route_set?.id ?? null,
      });
      if (result.warnings && result.warnings.length > 0) {
        const summary = result.warnings.map((w) => `${w.step}: ${w.error}`).join('; ');
        toast.info(`Provider-set сохранён, но материализация частично не удалась: ${summary}. Повторим автоматически.`);
      } else {
        toast.success('Provider-set обновлён');
      }
      setChangeSetOpen(false);
      await loadOverview();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка');
    } finally {
      setSavingSet(false);
    }
  }

  async function changeRouteSet() {
    setSavingRouteSet(true);
    try {
      const result = await networkApi.putAssignment(subAccountID, {
        provider_set_id: overview?.provider_set?.id ?? null,
        route_set_id: newRouteSetID || null,
      });
      if (result.warnings && result.warnings.length > 0) {
        const summary = result.warnings.map((w) => `${w.step}: ${w.error}`).join('; ');
        toast.info(`Route-set сохранён, но материализация частично не удалась: ${summary}. Повторим автоматически.`);
      } else {
        toast.success('Route-set обновлён');
      }
      setChangeRouteSetOpen(false);
      await loadOverview();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка');
    } finally {
      setSavingRouteSet(false);
    }
  }

  async function addOverride(e: FormEvent) {
    e.preventDefault();
    if (!addForm.provider_id) {
      toast.error('Выберите провайдера');
      return;
    }
    setSavingOverride(true);
    try {
      await networkApi.addProviderOverride(subAccountID, addForm);
      toast.success('Провайдер добавлен в override');
      setAddOpen(false);
      setAddForm(EMPTY_ADD_FORM);
      await loadOverview();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка');
    } finally {
      setSavingOverride(false);
    }
  }

  async function removeOverride(providerID: string) {
    setRemovingProviderID(providerID);
    try {
      await networkApi.deleteProviderOverride(subAccountID, providerID);
      toast.success('Провайдер удалён из override');
      await loadOverview();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка');
    } finally {
      setRemovingProviderID(null);
    }
  }

  async function submitRouteOverride(data: RouteOverrideData) {
    if (editingRouteOverride) {
      await networkApi.updateRouteOverride(subAccountID, editingRouteOverride.id, data);
      toast.success('Override-маршрут обновлён');
    } else {
      await networkApi.addRouteOverride(subAccountID, data);
      toast.success('Override-маршрут создан');
    }
    await loadOverview();
  }

  async function removeRouteOverride(routeID: string) {
    setRemovingRouteID(routeID);
    try {
      await networkApi.deleteRouteOverride(subAccountID, routeID);
      toast.success('Override-маршрут удалён');
      await loadOverview();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка');
    } finally {
      setRemovingRouteID(null);
    }
  }

  if (loadError) {
    return (
      <div role="alert" className="bg-red-50 border border-red-200 text-red-700 rounded-lg p-4">
        <p className="font-medium mb-1">Ошибка загрузки</p>
        <p className="text-sm">{loadError}</p>
        <button onClick={loadOverview} className="mt-2 text-sm underline text-red-700 hover:text-red-900">
          Повторить
        </button>
      </div>
    );
  }

  if (!overview) return <div className="text-sm text-gray-500 py-6">Загрузка...</div>;

  const overrideProviderIDs = new Set(overview.provider_overrides.map((o) => o.provider_id));
  const availableProviders = providers.filter((p) => p.active && !overrideProviderIDs.has(p.id));
  const providerSetOptions = [
    { value: '', label: '— Без provider-set —' },
    ...providerSets.map((s) => ({
      value: s.id,
      label: s.is_default ? `${s.name} (по умолчанию)` : s.name,
    })),
  ];
  const routeSetOptions = [
    { value: '', label: '— Без route-set —' },
    ...routeSets.map((s) => ({
      value: s.id,
      label: s.is_default ? `${s.name} (по умолчанию)` : s.name,
    })),
  ];
  const availableProviderOptions = [
    { value: '', label: '— Выберите провайдера —' },
    ...availableProviders.map((p) => ({
      value: p.id,
      label: p.ownership === 'private' ? `${p.name} (private)` : p.name,
    })),
  ];

  return (
    <div className="space-y-6">
      {/* Provider-set */}
      <div className="bg-white border border-gray-200 rounded-lg p-4">
        <div className="flex items-center justify-between mb-2">
          <h3 className="text-base font-semibold text-gray-900 m-0">Provider-set</h3>
          <Button
            variant="secondary"
            onClick={() => {
              setNewSetID(overview.provider_set?.id ?? '');
              setChangeSetOpen(true);
            }}
          >
            Изменить
          </Button>
        </div>
        {overview.provider_set ? (
          <p className="text-sm text-gray-700 m-0">
            Текущий: <span className="font-medium">{overview.provider_set.name}</span>
          </p>
        ) : (
          <p className="text-sm text-gray-500 m-0">Provider-set не назначен — суб-аккаунт работает только на override-провайдерах.</p>
        )}
      </div>

      {/* Route-set */}
      <div className="bg-white border border-gray-200 rounded-lg p-4">
        <div className="flex items-center justify-between mb-2">
          <h3 className="text-base font-semibold text-gray-900 m-0">Route-set</h3>
          <Button
            variant="secondary"
            onClick={() => {
              setNewRouteSetID(overview.route_set?.id ?? '');
              setChangeRouteSetOpen(true);
            }}
          >
            Изменить
          </Button>
        </div>
        {overview.route_set ? (
          <p className="text-sm text-gray-700 m-0">
            Текущий: <span className="font-medium">{overview.route_set.name}</span>
          </p>
        ) : (
          <p className="text-sm text-gray-500 m-0">Route-set не назначен.</p>
        )}
      </div>

      {/* Provider overrides */}
      <div className="bg-white border border-gray-200 rounded-lg p-4">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-base font-semibold text-gray-900 m-0">Кастомные провайдеры (override)</h3>
          <Button onClick={() => { setAddForm(EMPTY_ADD_FORM); setAddOpen(true); }}>
            + Добавить
          </Button>
        </div>

        <div className="bg-amber-50 border border-amber-200 rounded px-3 py-2 mb-3 text-xs text-amber-900">
          Эти настройки переопределяют шаблон. Изменения шаблона не затронут эти записи.
        </div>

        {overview.provider_overrides.length === 0 ? (
          <p className="text-sm text-gray-500 m-0">Override-провайдеров нет.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full text-sm">
              <thead>
                <tr className="text-left text-xs uppercase text-gray-500 border-b border-gray-200">
                  <th className="py-2 pr-3">Провайдер</th>
                  <th className="py-2 pr-3">Приоритет</th>
                  <th className="py-2 pr-3">Тип</th>
                  <th className="py-2 pr-3 text-right">Действия</th>
                </tr>
              </thead>
              <tbody>
                {overview.provider_overrides.map((o) => (
                  <tr key={o.provider_id} className="border-b border-gray-100 last:border-b-0">
                    <td className="py-2 pr-3 font-medium text-gray-900">{o.name}</td>
                    <td className="py-2 pr-3 tabular-nums">{o.priority}</td>
                    <td className="py-2 pr-3">
                      <Badge variant={o.ownership === 'private' ? 'info' : 'default'}>
                        {o.ownership}
                      </Badge>
                    </td>
                    <td className="py-2 pr-3 text-right">
                      <button
                        type="button"
                        onClick={() => removeOverride(o.provider_id)}
                        disabled={removingProviderID === o.provider_id}
                        className="text-sm text-red-600 hover:text-red-800 disabled:opacity-50"
                      >
                        {removingProviderID === o.provider_id ? 'Удаление...' : 'Удалить'}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Route overrides */}
      <div className="bg-white border border-gray-200 rounded-lg p-4">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-base font-semibold text-gray-900 m-0">Кастомные маршруты (overrides)</h3>
          <Button
            onClick={() => {
              setEditingRouteOverride(null);
              setRouteDrawerOpen(true);
            }}
          >
            + Добавить
          </Button>
        </div>

        <div className="bg-amber-50 border border-amber-200 rounded px-3 py-2 mb-3 text-xs text-amber-900">
          Эти маршруты переопределяют шаблон. Изменения шаблона их не затрагивают.
        </div>

        {overview.route_overrides.length === 0 ? (
          <p className="text-sm text-gray-500 m-0">Override-маршрутов нет.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full text-sm">
              <thead>
                <tr className="text-left text-xs uppercase text-gray-500 border-b border-gray-200">
                  <th className="py-2 pr-3">Имя</th>
                  <th className="py-2 pr-3">Провайдер</th>
                  <th className="py-2 pr-3 text-center">Приоритет</th>
                  <th className="py-2 pr-3 text-center">Статус</th>
                  <th className="py-2 pr-3 text-right">Действия</th>
                </tr>
              </thead>
              <tbody>
                {overview.route_overrides.map((o) => (
                  <tr key={o.id} className="border-b border-gray-100 last:border-b-0">
                    <td className="py-2 pr-3 font-medium text-gray-900">{o.name || '—'}</td>
                    <td className="py-2 pr-3">{o.provider_name}</td>
                    <td className="py-2 pr-3 text-center tabular-nums">{o.priority}</td>
                    <td className="py-2 pr-3 text-center">{o.status}</td>
                    <td className="py-2 pr-3 text-right">
                      <button
                        type="button"
                        onClick={() => removeRouteOverride(o.id)}
                        disabled={removingRouteID === o.id}
                        className="text-sm text-red-600 hover:text-red-800 disabled:opacity-50"
                      >
                        {removingRouteID === o.id ? 'Удаление...' : 'Удалить'}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Modal: change provider-set */}
      <Modal
        open={changeSetOpen}
        onClose={() => setChangeSetOpen(false)}
        title="Изменить provider-set"
        description={`Сменить provider-set для ${subAccountName}`}
      >
        <div className="space-y-4">
          <Select
            label="Provider-set"
            value={newSetID}
            options={providerSetOptions}
            onChange={setNewSetID}
          />
          <div className="flex justify-end gap-2">
            <Button variant="secondary" onClick={() => setChangeSetOpen(false)} disabled={savingSet}>
              Отмена
            </Button>
            <Button onClick={changeSet} disabled={savingSet}>
              {savingSet ? 'Сохранение...' : 'Применить'}
            </Button>
          </div>
        </div>
      </Modal>

      {/* Modal: change route-set */}
      <Modal
        open={changeRouteSetOpen}
        onClose={() => setChangeRouteSetOpen(false)}
        title="Изменить route-set"
        description={`Сменить route-set для ${subAccountName}`}
      >
        <div className="space-y-4">
          <Select
            label="Route-set"
            value={newRouteSetID}
            options={routeSetOptions}
            onChange={setNewRouteSetID}
          />
          <div className="flex justify-end gap-2">
            <Button variant="secondary" onClick={() => setChangeRouteSetOpen(false)} disabled={savingRouteSet}>
              Отмена
            </Button>
            <Button onClick={changeRouteSet} disabled={savingRouteSet}>
              {savingRouteSet ? 'Сохранение...' : 'Применить'}
            </Button>
          </div>
        </div>
      </Modal>

      {/* Modal: add provider override */}
      <Modal
        open={addOpen}
        onClose={() => setAddOpen(false)}
        title="Добавить провайдера в override"
        description="Добавление провайдера поверх шаблона provider-set"
      >
        <form onSubmit={addOverride} className="space-y-4">
          <Select
            label="Провайдер"
            value={addForm.provider_id}
            options={availableProviderOptions}
            onChange={(v) => setAddForm((f) => ({ ...f, provider_id: v }))}
          />
          <Input
            label="Приоритет"
            type="number"
            min="1"
            value={String(addForm.priority)}
            onChange={(e) => setAddForm((f) => ({ ...f, priority: Number(e.target.value) || 0 }))}
          />
          <div className="flex flex-col gap-2">
            <label className="flex items-center gap-2 text-sm text-gray-700">
              <input
                type="checkbox"
                checked={addForm.expose_cost}
                onChange={(e) => setAddForm((f) => ({ ...f, expose_cost: e.target.checked }))}
              />
              Раскрывать стоимость
            </label>
            <label className="flex items-center gap-2 text-sm text-gray-700">
              <input
                type="checkbox"
                checked={addForm.expose_provider_name}
                onChange={(e) => setAddForm((f) => ({ ...f, expose_provider_name: e.target.checked }))}
              />
              Раскрывать имя провайдера
            </label>
          </div>
          <div className="flex justify-end gap-2">
            <Button type="button" variant="secondary" onClick={() => setAddOpen(false)} disabled={savingOverride}>
              Отмена
            </Button>
            <Button type="submit" disabled={savingOverride}>
              {savingOverride ? 'Сохранение...' : 'Добавить'}
            </Button>
          </div>
        </form>
      </Modal>

      {/* Drawer: route override */}
      <RouteRuleDrawer
        open={routeDrawerOpen}
        onClose={() => setRouteDrawerOpen(false)}
        initial={editingRouteOverride}
        providers={providers}
        onSubmit={submitRouteOverride}
        title={editingRouteOverride ? 'Редактировать override-маршрут' : 'Новый override-маршрут'}
      />
    </div>
  );
}
