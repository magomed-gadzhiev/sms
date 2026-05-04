import { useState, useEffect, useCallback, useMemo } from 'react';
import { Link } from 'react-router-dom';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { useToast } from '../../components/ui/Toast';
import { networkApi, ApiError } from '../../api/client';
import type { NetworkAssignment, NetworkProviderSet } from '../../api/client';

export function AssignmentsPage() {
  const toast = useToast();
  const [list, setList] = useState<NetworkAssignment[]>([]);
  const [providerSets, setProviderSets] = useState<NetworkProviderSet[]>([]);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [search, setSearch] = useState('');
  const [bulkSetID, setBulkSetID] = useState('');
  const [bulkRunning, setBulkRunning] = useState(false);

  const reload = useCallback(() => {
    networkApi
      .listAssignments()
      .then((r) => setList(r.assignments))
      .catch(() => toast.error('Ошибка загрузки назначений'));
  }, [toast]);

  useEffect(() => {
    reload();
    networkApi
      .listProviderSets()
      .then((r) => setProviderSets(r.provider_sets))
      .catch(() => toast.error('Ошибка загрузки provider-sets'));
  }, [reload, toast]);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return list;
    return list.filter((a) => a.sub_account_name.toLowerCase().includes(q));
  }, [list, search]);

  const setOne = async (clientID: string, providerSetID: string | null) => {
    try {
      await networkApi.putAssignment(clientID, {
        provider_set_id: providerSetID,
        route_set_id: null,
      });
      toast.success('Назначение сохранено');
      reload();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
    }
  };

  const toggleOne = (clientID: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(clientID)) {
        next.delete(clientID);
      } else {
        next.add(clientID);
      }
      return next;
    });
  };

  const toggleAll = () => {
    setSelected((prev) => {
      const allFilteredSelected = filtered.length > 0 && filtered.every((a) => prev.has(a.client_id));
      if (allFilteredSelected) {
        const next = new Set(prev);
        filtered.forEach((a) => next.delete(a.client_id));
        return next;
      }
      const next = new Set(prev);
      filtered.forEach((a) => next.add(a.client_id));
      return next;
    });
  };

  const bulkApply = async () => {
    if (selected.size === 0) return;
    setBulkRunning(true);
    try {
      const r = await networkApi.bulkAssign({
        client_ids: Array.from(selected),
        provider_set_id: bulkSetID || null,
        route_set_id: null,
      });
      const ok = r.results.filter((x) => x.status === 'ok').length;
      const total = r.results.length;
      if (ok === total) {
        toast.success(`Назначено: ${ok} из ${total}`);
      } else {
        const firstErr = r.results.find((x) => x.status === 'error')?.error;
        toast.error(`Назначено: ${ok} из ${total}${firstErr ? ` (${firstErr})` : ''}`);
      }
      setSelected(new Set());
      reload();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
    } finally {
      setBulkRunning(false);
    }
  };

  const cancelBulk = () => {
    setSelected(new Set());
    setBulkSetID('');
  };

  const allFilteredChecked = filtered.length > 0 && filtered.every((a) => selected.has(a.client_id));

  const statusIcon = (s: NetworkAssignment['validation_status']) => {
    if (s === 'ok') return <span className="text-green-600">✓</span>;
    if (s === 'unassigned') return <span className="text-gray-400">—</span>;
    return <span className="text-amber-600" title="conflict">⚠</span>;
  };

  return (
    <div className="max-w-6xl">
      <PageHeader title="Назначения суб-аккаунтам" />

      {/* Sticky bulk-panel */}
      {selected.size > 0 && (
        <div className="sticky top-0 z-10 bg-white border border-blue-200 rounded-lg shadow-sm p-3 mb-4 flex flex-wrap items-center gap-3">
          <span className="text-sm font-medium text-gray-700">Выбрано: {selected.size}</span>
          <select
            value={bulkSetID}
            onChange={(e) => setBulkSetID(e.target.value)}
            className="rounded border border-gray-300 px-3 py-1.5 text-sm"
          >
            <option value="">— Снять назначение —</option>
            {providerSets.map((ps) => (
              <option key={ps.id} value={ps.id}>
                {ps.name}
                {ps.is_default ? ' (default)' : ''}
              </option>
            ))}
          </select>
          <Button onClick={bulkApply} disabled={bulkRunning}>
            {bulkRunning ? 'Применение...' : 'Применить'}
          </Button>
          <button
            type="button"
            onClick={cancelBulk}
            className="text-sm text-gray-500 hover:text-gray-700"
          >
            Отменить
          </button>
        </div>
      )}

      {/* Search */}
      <div className="mb-4 max-w-sm">
        <Input
          placeholder="Поиск по имени суб-аккаунта..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>

      <div className="bg-white border border-gray-200 rounded-lg overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-gray-50 text-gray-500 text-xs uppercase">
              <th className="p-3 w-10">
                <input
                  type="checkbox"
                  checked={allFilteredChecked}
                  onChange={toggleAll}
                  aria-label="Выбрать все"
                />
              </th>
              <th className="text-left p-3">Суб-аккаунт</th>
              <th className="text-left p-3">Provider-set</th>
              <th className="text-left p-3 w-28">Override</th>
              <th className="text-center p-3 w-20">Статус</th>
            </tr>
          </thead>
          <tbody>
            {filtered.length === 0 ? (
              <tr>
                <td colSpan={5} className="p-8 text-center text-gray-400">
                  {list.length === 0 ? 'Нет суб-аккаунтов' : 'Нет результатов по фильтру'}
                </td>
              </tr>
            ) : (
              filtered.map((a) => (
                <tr key={a.client_id} className="border-t border-gray-100">
                  <td className="p-3">
                    <input
                      type="checkbox"
                      checked={selected.has(a.client_id)}
                      onChange={() => toggleOne(a.client_id)}
                      aria-label={`Выбрать ${a.sub_account_name}`}
                    />
                  </td>
                  <td className="p-3 font-medium">
                    <Link
                      to={`/network/sub-accounts/${a.client_id}`}
                      className="text-blue-600 hover:underline"
                    >
                      {a.sub_account_name}
                    </Link>
                  </td>
                  <td className="p-3">
                    <select
                      value={a.provider_set_id ?? ''}
                      onChange={(e) => setOne(a.client_id, e.target.value || null)}
                      className="rounded border border-gray-300 px-2 py-1 text-sm min-w-[180px]"
                    >
                      <option value="">— Не назначен —</option>
                      {providerSets.map((ps) => (
                        <option key={ps.id} value={ps.id}>
                          {ps.name}
                          {ps.is_default ? ' (default)' : ''}
                        </option>
                      ))}
                    </select>
                  </td>
                  <td className="p-3">
                    {a.has_overrides ? (
                      <span className="text-amber-700 text-xs font-medium">override</span>
                    ) : (
                      <span className="text-gray-400">—</span>
                    )}
                  </td>
                  <td className="p-3 text-center">{statusIcon(a.validation_status)}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
