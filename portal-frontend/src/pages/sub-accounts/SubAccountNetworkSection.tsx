import { useState, useEffect, useCallback, type FormEvent } from 'react';
import {
  networkApi,
  ApiError,
  type NetworkSubAccountOverview,
  type NetworkProvider,
  type NetworkProviderSet,
} from '../../api/client';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Select } from '../../components/ui/Select';
import { Modal } from '../../components/ui/Modal';
import { Badge } from '../../components/ui/Badge';
import { useToast } from '../../components/ui/Toast';

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

export function SubAccountNetworkSection({ subAccountID, subAccountName }: Props) {
  const toast = useToast();
  const [overview, setOverview] = useState<NetworkSubAccountOverview | null>(null);
  const [providers, setProviders] = useState<NetworkProvider[]>([]);
  const [providerSets, setProviderSets] = useState<NetworkProviderSet[]>([]);
  const [loadError, setLoadError] = useState('');

  // Modal: change provider-set
  const [changeSetOpen, setChangeSetOpen] = useState(false);
  const [newSetID, setNewSetID] = useState<string>('');
  const [savingSet, setSavingSet] = useState(false);

  // Modal: add override
  const [addOpen, setAddOpen] = useState(false);
  const [addForm, setAddForm] = useState<AddOverrideForm>(EMPTY_ADD_FORM);
  const [savingOverride, setSavingOverride] = useState(false);
  const [removingProviderID, setRemovingProviderID] = useState<string | null>(null);

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
  }, []);

  async function changeSet() {
    setSavingSet(true);
    try {
      await networkApi.putAssignment(subAccountID, {
        provider_set_id: newSetID || null,
        route_set_id: null,
      });
      toast.success('Provider-set обновлён');
      setChangeSetOpen(false);
      await loadOverview();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка');
    } finally {
      setSavingSet(false);
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

      {/* Modal: add override */}
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
    </div>
  );
}
