import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Modal } from '../../components/ui/Modal';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { useToast } from '../../components/ui/Toast';
import { networkApi, ApiError } from '../../api/client';
import type {
  NetworkProviderSet,
  NetworkProviderSetItem,
  NetworkProvider,
} from '../../api/client';

export function ProviderSetsPage() {
  const toast = useToast();
  const [sets, setSets] = useState<NetworkProviderSet[]>([]);
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [items, setItems] = useState<NetworkProviderSetItem[]>([]);
  const [allProviders, setAllProviders] = useState<NetworkProvider[]>([]);
  const [nameDraft, setNameDraft] = useState('');

  const [createOpen, setCreateOpen] = useState(false);
  const [createName, setCreateName] = useState('');
  const [createDefault, setCreateDefault] = useState(false);
  const [creating, setCreating] = useState(false);

  const [addItemOpen, setAddItemOpen] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState<NetworkProviderSet | null>(null);
  const [deleting, setDeleting] = useState(false);

  const reloadSets = useCallback(() => {
    networkApi
      .listProviderSets()
      .then((r) => setSets(r.provider_sets))
      .catch(() => toast.error('Ошибка загрузки шаблонов'));
  }, [toast]);

  const reloadItems = useCallback(
    (id: string) => {
      networkApi
        .listProviderSetItems(id)
        .then((r) => setItems(r.items))
        .catch(() => toast.error('Ошибка загрузки items'));
    },
    [toast],
  );

  useEffect(() => {
    reloadSets();
    networkApi
      .listProviders()
      .then((r) => setAllProviders(r.providers))
      .catch(() => toast.error('Ошибка загрузки провайдеров'));
  }, [reloadSets, toast]);

  useEffect(() => {
    if (selectedID) {
      reloadItems(selectedID);
    } else {
      setItems([]);
    }
  }, [selectedID, reloadItems]);

  const selected = sets.find((s) => s.id === selectedID) || null;

  useEffect(() => {
    setNameDraft(selected?.name ?? '');
  }, [selected?.id, selected?.name]);

  const create = async () => {
    if (!createName.trim()) {
      toast.error('Введите имя');
      return;
    }
    setCreating(true);
    try {
      const r = await networkApi.createProviderSet({
        name: createName.trim(),
        is_default: createDefault,
      });
      toast.success('Создан');
      setCreateOpen(false);
      setCreateName('');
      setCreateDefault(false);
      reloadSets();
      setSelectedID(r.id);
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
    } finally {
      setCreating(false);
    }
  };

  const renameSelected = async (name: string) => {
    if (!selected || name === selected.name) return;
    if (!name.trim()) {
      toast.error('Имя не может быть пустым');
      setNameDraft(selected.name);
      return;
    }
    try {
      await networkApi.updateProviderSet(selected.id, {
        name: name.trim(),
        is_default: selected.is_default,
      });
      reloadSets();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
      setNameDraft(selected.name);
    }
  };

  const toggleDefault = async () => {
    if (!selected) return;
    try {
      await networkApi.updateProviderSet(selected.id, {
        name: selected.name,
        is_default: !selected.is_default,
      });
      reloadSets();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
    }
  };

  const putItems = async (next: NetworkProviderSetItem[]) => {
    if (!selectedID) return;
    try {
      await networkApi.putProviderSetItems(
        selectedID,
        next.map((it) => ({
          provider_id: it.provider_id,
          priority: it.priority,
          expose_cost: it.expose_cost,
          expose_provider_name: it.expose_provider_name,
        })),
      );
      reloadItems(selectedID);
      reloadSets();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
      // refresh items to reset UI to server state
      reloadItems(selectedID);
    }
  };

  const addItem = (providerID: string) => {
    const provider = allProviders.find((p) => p.id === providerID);
    if (!provider) return;
    const nextPriority = items.length > 0 ? Math.max(...items.map((i) => i.priority)) + 10 : 10;
    const next: NetworkProviderSetItem[] = [
      ...items,
      {
        id: '',
        provider_id: providerID,
        provider_name: provider.name,
        priority: nextPriority,
        expose_cost: false,
        expose_provider_name: false,
      },
    ];
    setAddItemOpen(false);
    void putItems(next);
  };

  const removeItem = (providerID: string) => {
    void putItems(items.filter((it) => it.provider_id !== providerID));
  };

  const updateItemField = <K extends 'priority' | 'expose_cost' | 'expose_provider_name'>(
    providerID: string,
    field: K,
    value: NetworkProviderSetItem[K],
  ) => {
    void putItems(
      items.map((it) =>
        it.provider_id === providerID ? { ...it, [field]: value } : it,
      ),
    );
  };

  const deleteSet = async () => {
    if (!confirmDelete) return;
    setDeleting(true);
    try {
      await networkApi.deleteProviderSet(confirmDelete.id);
      toast.success('Удалён');
      if (selectedID === confirmDelete.id) {
        setSelectedID(null);
      }
      setConfirmDelete(null);
      reloadSets();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
      setConfirmDelete(null);
    } finally {
      setDeleting(false);
    }
  };

  const usedProviderIDs = new Set(items.map((it) => it.provider_id));
  const availableProviders = allProviders.filter((p) => p.active && !usedProviderIDs.has(p.id));

  return (
    <div className="max-w-7xl">
      <PageHeader
        title="Provider-sets"
        actions={<Button onClick={() => setCreateOpen(true)}>+ Создать</Button>}
      />

      <div className="grid grid-cols-12 gap-4">
        {/* Left: list */}
        <aside className="col-span-12 md:col-span-4 space-y-2">
          {sets.length === 0 ? (
            <div className="bg-white border border-gray-200 rounded-lg p-6 text-center text-gray-400 text-sm">
              Нет шаблонов
            </div>
          ) : (
            sets.map((s) => {
              const isSelected = s.id === selectedID;
              return (
                <button
                  key={s.id}
                  type="button"
                  onClick={() => setSelectedID(s.id)}
                  className={`w-full text-left p-3 rounded-lg border transition-colors ${
                    isSelected
                      ? 'bg-blue-50 border-blue-300'
                      : 'bg-white border-gray-200 hover:bg-gray-50'
                  }`}
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-medium truncate">{s.name}</span>
                    {s.is_default && (
                      <span className="px-2 py-0.5 text-xs rounded bg-green-100 text-green-700 shrink-0">
                        default
                      </span>
                    )}
                  </div>
                  <div className="text-xs text-gray-500 mt-1">
                    {s.item_count} провайдеров · {s.assigned_count} назначений
                  </div>
                </button>
              );
            })
          )}
        </aside>

        {/* Right: details */}
        <section className="col-span-12 md:col-span-8">
          {!selected ? (
            <div className="bg-white border border-gray-200 rounded-lg p-12 text-center text-gray-400">
              Выбери шаблон или создай новый
            </div>
          ) : (
            <div className="bg-white border border-gray-200 rounded-lg p-5 space-y-5">
              {/* Header */}
              <div className="flex flex-wrap items-center gap-4">
                <div className="flex-1 min-w-[200px]">
                  <Input
                    value={nameDraft}
                    onChange={(e) => setNameDraft(e.target.value)}
                    onBlur={() => renameSelected(nameDraft)}
                  />
                </div>
                <label className="flex items-center gap-2 text-sm text-gray-700">
                  <input
                    type="checkbox"
                    checked={selected.is_default}
                    onChange={toggleDefault}
                  />
                  По умолчанию для новых суб-аккаунтов
                </label>
                <button
                  type="button"
                  onClick={() => setConfirmDelete(selected)}
                  className="text-red-600 hover:underline text-sm"
                >
                  Удалить
                </button>
              </div>

              {/* Items table */}
              <table className="w-full text-sm">
                <thead>
                  <tr className="bg-gray-50 text-gray-500 text-xs uppercase">
                    <th className="text-left p-2">Провайдер</th>
                    <th className="text-left p-2 w-24">Приоритет</th>
                    <th className="text-center p-2 w-28">Expose cost</th>
                    <th className="text-center p-2 w-28">Expose name</th>
                    <th className="text-right p-2 w-24">Действия</th>
                  </tr>
                </thead>
                <tbody>
                  {items.length === 0 ? (
                    <tr>
                      <td colSpan={5} className="p-6 text-center text-gray-400">
                        Нет провайдеров
                      </td>
                    </tr>
                  ) : (
                    items
                      .slice()
                      .sort((a, b) => a.priority - b.priority)
                      .map((it) => (
                        <tr key={it.provider_id} className="border-t border-gray-100">
                          <td className="p-2 font-medium">{it.provider_name}</td>
                          <td className="p-2">
                            <input
                              type="number"
                              defaultValue={it.priority}
                              key={`${it.provider_id}-${it.priority}`}
                              onBlur={(e) => {
                                const v = parseInt(e.target.value, 10);
                                if (!isNaN(v) && v !== it.priority) {
                                  updateItemField(it.provider_id, 'priority', v);
                                }
                              }}
                              className="w-20 rounded border border-gray-300 px-2 py-1 text-sm"
                            />
                          </td>
                          <td className="p-2 text-center">
                            <input
                              type="checkbox"
                              checked={it.expose_cost}
                              onChange={(e) =>
                                updateItemField(it.provider_id, 'expose_cost', e.target.checked)
                              }
                            />
                          </td>
                          <td className="p-2 text-center">
                            <input
                              type="checkbox"
                              checked={it.expose_provider_name}
                              onChange={(e) =>
                                updateItemField(
                                  it.provider_id,
                                  'expose_provider_name',
                                  e.target.checked,
                                )
                              }
                            />
                          </td>
                          <td className="p-2 text-right">
                            <button
                              type="button"
                              onClick={() => removeItem(it.provider_id)}
                              className="text-red-600 hover:underline"
                            >
                              Удалить
                            </button>
                          </td>
                        </tr>
                      ))
                  )}
                </tbody>
              </table>

              <Button variant="secondary" onClick={() => setAddItemOpen(true)}>
                + Добавить провайдера
              </Button>
            </div>
          )}
        </section>
      </div>

      {/* Create modal */}
      <Modal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        title="Создать provider-set"
      >
        <div className="space-y-4">
          <Input
            label="Имя"
            value={createName}
            onChange={(e) => setCreateName(e.target.value)}
          />
          <label className="flex items-center gap-2 text-sm text-gray-700">
            <input
              type="checkbox"
              checked={createDefault}
              onChange={(e) => setCreateDefault(e.target.checked)}
            />
            По умолчанию для новых суб-аккаунтов
          </label>
          <Button onClick={create} disabled={creating}>
            {creating ? 'Создание...' : 'Создать'}
          </Button>
        </div>
      </Modal>

      {/* Add item modal */}
      <Modal
        open={addItemOpen}
        onClose={() => setAddItemOpen(false)}
        title="Добавить провайдера"
      >
        {availableProviders.length === 0 ? (
          <div className="text-sm text-gray-400 py-4 text-center">
            Все доступные провайдеры уже добавлены
          </div>
        ) : (
          <div className="space-y-1 max-h-96 overflow-y-auto">
            {availableProviders.map((p) => (
              <button
                key={p.id}
                type="button"
                onClick={() => addItem(p.id)}
                className="w-full text-left p-3 rounded hover:bg-gray-50 border border-gray-100"
              >
                <div className="font-medium">{p.name}</div>
                <div className="text-xs text-gray-500">
                  {p.ownership === 'private' ? 'Свой' : 'Платформенный'}
                  {p.smpp_host ? ` · ${p.smpp_host}` : ''}
                </div>
              </button>
            ))}
          </div>
        )}
      </Modal>

      <ConfirmDialog
        open={!!confirmDelete}
        onCancel={() => setConfirmDelete(null)}
        onConfirm={deleteSet}
        title="Удалить provider-set?"
        description={`«${confirmDelete?.name ?? ''}» будет удалён. Если он назначен суб-аккаунтам — операция вернёт 409.`}
        variant="danger"
        confirmLabel="Удалить"
        loading={deleting}
      />
    </div>
  );
}
