import { useState, useEffect, useCallback } from 'react';
import type { DragEvent } from 'react';
import { Info } from 'lucide-react';
import { PageHeader } from '../../components/layout/PageHeader';
import { usePageTitle } from '../../contexts/PageTitleContext';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Modal } from '../../components/ui/Modal';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { useToast } from '../../components/ui/Toast';
import { RouteRuleDrawer } from '../../components/network/RouteRuleDrawer';
import {
  networkApi,
  ApiError,
  type NetworkRouteSet,
  type NetworkRouteSetItem,
  type NetworkProvider,
  type RoutePreviewMatch,
} from '../../api/client';

type RuleData = Omit<NetworkRouteSetItem, 'id' | 'provider_name'>;

interface PreviewForm {
  phone: string;
  sender_id: string;
  traffic_type: string;
}

export function RouteSetsPage() {
  usePageTitle('Route-sets');
  const toast = useToast();
  const [sets, setSets] = useState<NetworkRouteSet[]>([]);
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [items, setItems] = useState<NetworkRouteSetItem[]>([]);
  const [providers, setProviders] = useState<NetworkProvider[]>([]);

  const [createOpen, setCreateOpen] = useState(false);
  const [createName, setCreateName] = useState('');
  const [createDefault, setCreateDefault] = useState(false);
  const [creating, setCreating] = useState(false);

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editingItem, setEditingItem] = useState<NetworkRouteSetItem | null>(null);

  const [confirmDelete, setConfirmDelete] = useState<NetworkRouteSet | null>(null);
  const [deleting, setDeleting] = useState(false);

  const [previewForm, setPreviewForm] = useState<PreviewForm>({
    phone: '',
    sender_id: '',
    traffic_type: 'transactional',
  });
  const [previewMatches, setPreviewMatches] = useState<RoutePreviewMatch[]>([]);

  const [dragIdx, setDragIdx] = useState<number | null>(null);

  const reloadSets = useCallback(() => {
    networkApi
      .listRouteSets()
      .then((r) => setSets(r.route_sets))
      .catch(() => toast.error('Ошибка загрузки шаблонов'));
  }, [toast]);

  const reloadItems = useCallback(
    (id: string) => {
      networkApi
        .listRouteSetItems(id)
        .then((r) => setItems(r.items))
        .catch(() => toast.error('Ошибка загрузки правил'));
    },
    [toast],
  );

  useEffect(() => {
    reloadSets();
    networkApi
      .listProviders()
      .then((r) => setProviders(r.providers || []))
      .catch(() => toast.error('Ошибка загрузки провайдеров'));
  }, [reloadSets, toast]);

  useEffect(() => {
    if (selectedID) {
      reloadItems(selectedID);
    } else {
      setItems([]);
      setPreviewMatches([]);
    }
  }, [selectedID, reloadItems]);

  const selected = sets.find((s) => s.id === selectedID) || null;

  const create = async () => {
    if (!createName.trim()) {
      toast.error('Введите имя');
      return;
    }
    setCreating(true);
    try {
      const r = await networkApi.createRouteSet({
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

  const renameSelected = async (newName: string) => {
    if (!selected) return;
    if (!newName.trim() || newName === selected.name) return;
    try {
      await networkApi.updateRouteSet(selected.id, {
        name: newName.trim(),
        is_default: selected.is_default,
      });
      reloadSets();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
    }
  };

  const toggleDefault = async () => {
    if (!selected) return;
    try {
      await networkApi.updateRouteSet(selected.id, {
        name: selected.name,
        is_default: !selected.is_default,
      });
      reloadSets();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
    }
  };

  const deleteSet = async () => {
    if (!confirmDelete) return;
    setDeleting(true);
    try {
      await networkApi.deleteRouteSet(confirmDelete.id);
      toast.success('Удалён');
      if (selectedID === confirmDelete.id) setSelectedID(null);
      setConfirmDelete(null);
      reloadSets();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
      setConfirmDelete(null);
    } finally {
      setDeleting(false);
    }
  };

  const submitItem = async (data: RuleData): Promise<void> => {
    if (!selected) return;
    try {
      if (editingItem) {
        await networkApi.updateRouteSetItem(selected.id, editingItem.id, data);
        toast.success('Обновлено');
      } else {
        await networkApi.createRouteSetItem(selected.id, data);
        toast.success('Создано');
      }
      reloadItems(selected.id);
      reloadSets();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
      throw e;
    }
  };

  const deleteItem = async (itemId: string) => {
    if (!selected) return;
    try {
      await networkApi.deleteRouteSetItem(selected.id, itemId);
      toast.success('Удалено');
      reloadItems(selected.id);
      reloadSets();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
    }
  };

  const duplicateItem = async (itemId: string) => {
    if (!selected) return;
    try {
      await networkApi.duplicateRouteSetItem(selected.id, itemId);
      toast.success('Дублировано');
      reloadItems(selected.id);
      reloadSets();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
    }
  };

  // Native HTML5 drag-and-drop reorder
  const onDragStart = (idx: number) => () => setDragIdx(idx);
  const onDragOver = () => (e: DragEvent<HTMLTableRowElement>) => {
    e.preventDefault();
  };
  const onDrop = (idx: number) => async (e: DragEvent<HTMLTableRowElement>) => {
    e.preventDefault();
    if (dragIdx === null || dragIdx === idx || !selected) {
      setDragIdx(null);
      return;
    }
    const next = [...items];
    const [moved] = next.splice(dragIdx, 1);
    next.splice(idx, 0, moved);
    setItems(next);
    setDragIdx(null);
    // Top of list = highest priority. Step = 10 для удобства.
    const reorderPayload = next.map((it, i) => ({
      item_id: it.id,
      priority: (next.length - i) * 10,
    }));
    try {
      await networkApi.reorderRouteSetItems(selected.id, reorderPayload);
      reloadItems(selected.id);
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
      reloadItems(selected.id);
    }
  };

  const runPreview = async () => {
    if (!selected) return;
    try {
      const r = await networkApi.previewRouteSet(selected.id, previewForm);
      setPreviewMatches(r.matches);
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
    }
  };

  return (
    <div className="max-w-7xl">
      <PageHeader
        actions={<Button onClick={() => setCreateOpen(true)}>+ Создать</Button>}
      />

      <div className="grid grid-cols-12 gap-4">
        {/* Left: list */}
        <aside className="col-span-12 md:col-span-4 space-y-2">
          {sets.length === 0 ? (
            <div className="bg-white dark:bg-slate-900 border border-gray-200 dark:border-slate-700 rounded-lg p-6 text-center text-gray-400 dark:text-slate-500 text-sm">
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
                      ? 'bg-blue-50 dark:bg-blue-950/40 border-blue-300'
                      : 'bg-white dark:bg-slate-900 border-gray-200 dark:border-slate-700 hover:bg-gray-50 dark:hover:bg-slate-800'
                  }`}
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-medium truncate">{s.name}</span>
                    {s.is_default && (
                      <span className="px-2 py-0.5 text-xs rounded bg-amber-100 text-amber-700 shrink-0">
                        default
                      </span>
                    )}
                  </div>
                  <div className="text-xs text-gray-500 dark:text-slate-400 mt-1">
                    {s.item_count} правил · {s.assigned_count} назначений
                  </div>
                </button>
              );
            })
          )}
        </aside>

        {/* Right: details */}
        <section className="col-span-12 md:col-span-8">
          {!selected ? (
            <div className="bg-white dark:bg-slate-900 border border-gray-200 dark:border-slate-700 rounded-lg p-12 text-center text-gray-400 dark:text-slate-500">
              Выбери шаблон или создай новый
            </div>
          ) : (
            <div className="bg-white dark:bg-slate-900 border border-gray-200 dark:border-slate-700 rounded-lg p-5 space-y-5">
              {/* Header */}
              <div className="flex flex-wrap items-center gap-4">
                <div className="flex-1 min-w-[200px]">
                  <Input
                    key={selected.id}
                    defaultValue={selected.name}
                    onBlur={(e) => {
                      if (e.target.value !== selected.name) {
                        void renameSelected(e.target.value);
                      }
                    }}
                    className="text-lg font-semibold"
                  />
                </div>
                <label className="flex items-center gap-2 text-sm text-gray-700 dark:text-slate-300">
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
                  className="text-red-600 dark:text-red-400 hover:underline text-sm"
                >
                  Удалить
                </button>
              </div>

              {/* Preview */}
              <details className="border border-gray-200 dark:border-slate-700 rounded">
                <summary className="cursor-pointer p-3 text-sm text-gray-600 dark:text-slate-400 hover:bg-gray-50 dark:hover:bg-slate-800 flex items-center gap-2">
                  <span>Превью маршрутизации</span>
                  <span
                    className="inline-flex items-center text-amber-600"
                    title="Preview не проверяет соответствие условий по оператору и country-prefix в полном объёме. В реальном пайплайне маршрутизация может отличаться. Используйте preview как ориентир, не как enforcement."
                    aria-label="Preview не проверяет соответствие условий по оператору и country-prefix в полном объёме. В реальном пайплайне маршрутизация может отличаться. Используйте preview как ориентир, не как enforcement."
                  >
                    <Info size={14} />
                  </span>
                </summary>
                <div className="p-3 space-y-3 border-t border-gray-100 dark:border-slate-800">
                  <div className="text-xs text-amber-700 bg-amber-50 border border-amber-200 rounded p-2">
                    Preview — ориентир, не enforcement. Условия по оператору и country-prefix
                    проверяются упрощённо (без JOIN на таблицу operators); реальная маршрутизация
                    в пайплайне может отличаться.
                  </div>
                  <div className="grid grid-cols-1 md:grid-cols-3 gap-2">
                    <Input
                      label="Телефон"
                      value={previewForm.phone}
                      onChange={(e) =>
                        setPreviewForm({ ...previewForm, phone: e.target.value })
                      }
                      placeholder="79991234567"
                    />
                    <Input
                      label="Sender ID"
                      value={previewForm.sender_id}
                      onChange={(e) =>
                        setPreviewForm({ ...previewForm, sender_id: e.target.value })
                      }
                    />
                    <div className="flex flex-col gap-1">
                      <label className="text-sm font-medium text-gray-700 dark:text-slate-300">Traffic type</label>
                      <select
                        value={previewForm.traffic_type}
                        onChange={(e) =>
                          setPreviewForm({ ...previewForm, traffic_type: e.target.value })
                        }
                        className="rounded border border-gray-300 dark:border-slate-600 px-3 py-2 text-sm"
                      >
                        <option value="transactional">transactional</option>
                        <option value="promo">promo</option>
                        <option value="service">service</option>
                      </select>
                    </div>
                  </div>
                  <Button onClick={runPreview}>Симулировать</Button>
                  {previewMatches.length === 0 ? (
                    <div className="text-xs text-gray-400 dark:text-slate-500">Нет совпадений</div>
                  ) : (
                    <ul className="text-sm">
                      {previewMatches.map((m) => (
                        <li
                          key={m.matched_item_id}
                          className="border-l-2 border-blue-500 pl-2 mt-1"
                        >
                          <strong>{m.item_name}</strong> → {m.provider_name} (приоритет{' '}
                          {m.priority})
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
              </details>

              {/* Items table */}
              <table className="w-full text-sm">
                <thead>
                  <tr className="bg-gray-50 dark:bg-slate-950 text-gray-500 dark:text-slate-400 text-xs uppercase">
                    <th className="text-left p-2 w-16">№</th>
                    <th className="text-left p-2">Имя</th>
                    <th className="text-left p-2">Условия</th>
                    <th className="text-left p-2">Провайдер</th>
                    <th className="text-center p-2 w-16">Доля</th>
                    <th className="text-center p-2 w-20">Статус</th>
                    <th className="text-right p-2 w-32"></th>
                  </tr>
                </thead>
                <tbody>
                  {items.length === 0 ? (
                    <tr>
                      <td colSpan={7} className="p-6 text-center text-gray-400 dark:text-slate-500">
                        Нет правил
                      </td>
                    </tr>
                  ) : (
                    items.map((it, idx) => (
                      <tr
                        key={it.id}
                        draggable
                        onDragStart={onDragStart(idx)}
                        onDragOver={onDragOver()}
                        onDrop={onDrop(idx)}
                        className="border-t border-gray-100 dark:border-slate-800 hover:bg-gray-50 dark:hover:bg-slate-800"
                      >
                        <td className="p-2 text-gray-400 dark:text-slate-500 cursor-move select-none">
                          ⋮⋮ {idx + 1}
                        </td>
                        <td
                          className="p-2 cursor-pointer"
                          onClick={() => {
                            setEditingItem(it);
                            setDrawerOpen(true);
                          }}
                        >
                          {it.name || '—'}
                        </td>
                        <td className="p-2 text-xs text-gray-500 dark:text-slate-400">
                          {it.condition_groups.length === 0
                            ? 'все'
                            : it.condition_groups
                                .flatMap((g) =>
                                  g.conditions.map((c) => `${c.type}=${c.value}`),
                                )
                                .join(', ')}
                        </td>
                        <td className="p-2">{it.provider_name}</td>
                        <td className="p-2 text-center">{it.share}%</td>
                        <td className="p-2 text-center">
                          <span
                            className={`text-xs px-2 py-0.5 rounded ${
                              it.status === 'active'
                                ? 'bg-green-100 dark:bg-green-900/40 text-green-700 dark:text-green-300'
                                : 'bg-gray-100 dark:bg-slate-800 text-gray-500 dark:text-slate-400'
                            }`}
                          >
                            {it.status === 'active' ? 'активен' : 'выкл'}
                          </span>
                        </td>
                        <td className="p-2 text-right whitespace-nowrap">
                          <button
                            type="button"
                            onClick={() => duplicateItem(it.id)}
                            className="text-blue-600 dark:text-blue-400 hover:underline text-xs mr-2"
                          >
                            Дубль
                          </button>
                          <button
                            type="button"
                            onClick={() => deleteItem(it.id)}
                            className="text-red-600 dark:text-red-400 hover:underline text-xs"
                          >
                            Удалить
                          </button>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>

              <Button
                variant="secondary"
                onClick={() => {
                  setEditingItem(null);
                  setDrawerOpen(true);
                }}
              >
                + Правило
              </Button>
            </div>
          )}
        </section>
      </div>

      <Modal open={createOpen} onClose={() => setCreateOpen(false)} title="Создать route-set">
        <div className="space-y-4">
          <Input
            label="Имя"
            value={createName}
            onChange={(e) => setCreateName(e.target.value)}
          />
          <label className="flex items-center gap-2 text-sm text-gray-700 dark:text-slate-300">
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

      <RouteRuleDrawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        initial={editingItem}
        providers={providers}
        onSubmit={submitItem}
        title={editingItem ? 'Редактировать правило' : 'Новое правило'}
      />

      <ConfirmDialog
        open={!!confirmDelete}
        onCancel={() => setConfirmDelete(null)}
        onConfirm={deleteSet}
        title="Удалить route-set?"
        description={`«${confirmDelete?.name ?? ''}» будет удалён. Если назначен суб-аккаунтам — операция вернёт 409.`}
        variant="danger"
        confirmLabel="Удалить"
        loading={deleting}
      />
    </div>
  );
}
