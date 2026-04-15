import { useState, useEffect } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Modal } from '../../components/ui/Modal';
import { Input } from '../../components/ui/Input';
import { useToast } from '../../components/ui/Toast';
import { resellerApi, subAccountsApi, ApiError } from '../../api/client';

type Tab = 'providers' | 'routes';

interface ProviderAssignment {
  id: string;
  client_id: string;
  sub_account_name: string;
  provider_id: string;
  provider_name: string;
  active: boolean;
  priority: number;
}

interface RouteEntry {
  id: string;
  client_id: string;
  sub_account_name: string;
  provider_id: string;
  provider_name: string;
  country_id: string | null;
  operator_id: string | null;
  priority: number;
  active: boolean;
}

interface SubAccountOption {
  id: string;
  name: string;
}

export function NetworkRoutingPage() {
  const toast = useToast();
  const [tab, setTab] = useState<Tab>('providers');
  const [subFilter, setSubFilter] = useState('');
  const [subAccounts, setSubAccounts] = useState<SubAccountOption[]>([]);

  const [providers, setProviders] = useState<ProviderAssignment[]>([]);
  const [routes, setRoutes] = useState<RouteEntry[]>([]);
  const [loading, setLoading] = useState(false);

  // Bulk assign modal
  const [showBulk, setShowBulk] = useState(false);
  const [bulkProviderID, setBulkProviderID] = useState('');
  const [bulkPriority, setBulkPriority] = useState('10');
  const [bulkSelected, setBulkSelected] = useState<Set<string>>(new Set());
  const [bulkSubmitting, setBulkSubmitting] = useState(false);

  useEffect(() => {
    subAccountsApi.list().then((r: any) => {
      setSubAccounts((r.sub_accounts || []).map((sa: any) => ({ id: sa.id, name: sa.name || sa.email })));
    }).catch(() => {});
  }, []);

  useEffect(() => {
    setLoading(true);
    const params = subFilter ? { sub_account_id: subFilter } : undefined;
    if (tab === 'providers') {
      resellerApi.listNetworkProviders(params)
        .then((r) => setProviders(r.providers as ProviderAssignment[]))
        .catch(() => toast.error('Ошибка загрузки'))
        .finally(() => setLoading(false));
    } else {
      resellerApi.listNetworkRoutes(params)
        .then((r) => setRoutes(r.routes as RouteEntry[]))
        .catch(() => toast.error('Ошибка загрузки'))
        .finally(() => setLoading(false));
    }
  }, [tab, subFilter]);

  async function handleBulkAssign() {
    if (bulkSelected.size === 0 || !bulkProviderID) return;
    setBulkSubmitting(true);
    try {
      const result = await resellerApi.bulkAssignProvider({
        sub_account_ids: Array.from(bulkSelected),
        provider_id: bulkProviderID,
        priority: parseInt(bulkPriority) || 10,
      });
      const ok = (result.results as any[]).filter((r: any) => r.status === 'assigned').length;
      toast.success(`Назначено: ${ok} из ${bulkSelected.size}`);
      setShowBulk(false);
      setBulkSelected(new Set());
      // Reload
      const params = subFilter ? { sub_account_id: subFilter } : undefined;
      resellerApi.listNetworkProviders(params).then((r) => setProviders(r.providers as ProviderAssignment[]));
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
    } finally {
      setBulkSubmitting(false);
    }
  }

  return (
    <div className="max-w-6xl">
      <PageHeader
        title="Маршрутизация сети"
        actions={
          tab === 'providers' ? (
            <Button onClick={() => setShowBulk(true)}>Массовое назначение</Button>
          ) : undefined
        }
      />

      <div className="flex items-center gap-4 mb-4">
        <div className="flex gap-1">
          <button
            onClick={() => setTab('providers')}
            className={`px-3 py-1.5 text-sm rounded ${tab === 'providers' ? 'bg-primary text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200'}`}
          >
            Провайдеры
          </button>
          <button
            onClick={() => setTab('routes')}
            className={`px-3 py-1.5 text-sm rounded ${tab === 'routes' ? 'bg-primary text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200'}`}
          >
            Маршруты
          </button>
        </div>
        <select
          value={subFilter}
          onChange={(e) => setSubFilter(e.target.value)}
          className="border border-gray-300 rounded px-2 py-1.5 text-sm"
        >
          <option value="">Все субаккаунты</option>
          {subAccounts.map((sa) => (
            <option key={sa.id} value={sa.id}>{sa.name}</option>
          ))}
        </select>
      </div>

      {loading ? (
        <div className="py-8 text-center text-gray-400">Загрузка...</div>
      ) : tab === 'providers' ? (
        providers.length === 0 ? (
          <div className="py-8 text-center text-gray-400">Нет назначенных провайдеров</div>
        ) : (
          <table className="w-full text-sm bg-white rounded-lg border border-gray-200">
            <thead>
              <tr className="bg-gray-50 text-gray-500 text-xs uppercase">
                <th className="text-left p-3">Субаккаунт</th>
                <th className="text-left p-3">Провайдер</th>
                <th className="text-center p-3">Приоритет</th>
                <th className="text-center p-3">Активен</th>
              </tr>
            </thead>
            <tbody>
              {providers.map((p) => (
                <tr key={p.id} className="border-t border-gray-100">
                  <td className="p-3">{p.sub_account_name}</td>
                  <td className="p-3">{p.provider_name}</td>
                  <td className="p-3 text-center">{p.priority}</td>
                  <td className="p-3 text-center">
                    <span className={`inline-block w-2 h-2 rounded-full ${p.active ? 'bg-green-500' : 'bg-gray-300'}`} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )
      ) : (
        routes.length === 0 ? (
          <div className="py-8 text-center text-gray-400">Нет маршрутов</div>
        ) : (
          <table className="w-full text-sm bg-white rounded-lg border border-gray-200">
            <thead>
              <tr className="bg-gray-50 text-gray-500 text-xs uppercase">
                <th className="text-left p-3">Субаккаунт</th>
                <th className="text-left p-3">Провайдер</th>
                <th className="text-left p-3">Страна</th>
                <th className="text-left p-3">Оператор</th>
                <th className="text-center p-3">Приоритет</th>
                <th className="text-center p-3">Активен</th>
              </tr>
            </thead>
            <tbody>
              {routes.map((rt) => (
                <tr key={rt.id} className="border-t border-gray-100">
                  <td className="p-3">{rt.sub_account_name}</td>
                  <td className="p-3">{rt.provider_name}</td>
                  <td className="p-3">{rt.country_id || '—'}</td>
                  <td className="p-3">{rt.operator_id || '—'}</td>
                  <td className="p-3 text-center">{rt.priority}</td>
                  <td className="p-3 text-center">
                    <span className={`inline-block w-2 h-2 rounded-full ${rt.active ? 'bg-green-500' : 'bg-gray-300'}`} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )
      )}

      {/* Bulk assign modal */}
      <Modal open={showBulk} onClose={() => setShowBulk(false)} title="Массовое назначение провайдера">
        <div className="flex flex-col gap-4">
          <Input
            label="ID провайдера"
            value={bulkProviderID}
            onChange={(e) => setBulkProviderID(e.target.value)}
            required
            placeholder="UUID провайдера"
          />
          <Input
            label="Приоритет"
            type="number"
            value={bulkPriority}
            onChange={(e) => setBulkPriority(e.target.value)}
          />
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">Субаккаунты</label>
            <div className="max-h-48 overflow-y-auto border border-gray-200 rounded p-2 space-y-1">
              {subAccounts.map((sa) => (
                <label key={sa.id} className="flex items-center gap-2 text-sm cursor-pointer">
                  <input
                    type="checkbox"
                    checked={bulkSelected.has(sa.id)}
                    onChange={(e) => {
                      const next = new Set(bulkSelected);
                      e.target.checked ? next.add(sa.id) : next.delete(sa.id);
                      setBulkSelected(next);
                    }}
                  />
                  {sa.name}
                </label>
              ))}
            </div>
          </div>
          <div className="flex gap-2">
            <Button onClick={handleBulkAssign} disabled={bulkSubmitting || bulkSelected.size === 0 || !bulkProviderID}>
              {bulkSubmitting ? 'Назначение...' : `Назначить (${bulkSelected.size})`}
            </Button>
            <Button variant="secondary" onClick={() => setShowBulk(false)}>Отмена</Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
