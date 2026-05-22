import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { usePageTitle } from '../../contexts/PageTitleContext';
import { Button } from '../../components/ui/Button';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import {
  routesApi,
  providersApi,
  ApiError,
  type Provider,
} from '../../api/client';
import { useAuth } from '../../contexts/AuthContext';
import type { RouteListItem } from './types';
import { RouteFilters } from './components/RouteFilters';
import { RouteTable } from './components/RouteTable';
import { RouteModal } from './RouteModal';

export function RoutingPage() {
  usePageTitle('Маршрутизация');
  const { user } = useAuth();
  const isSubAccount = !!user?.parent_client_id;
  const [routes, setRoutes] = useState<RouteListItem[]>([]);
  const [providers, setProviders] = useState<Map<string, Provider>>(new Map());
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Filters
  const [search, setSearch] = useState('');
  const [routeType, setRouteType] = useState('');
  const [status, setStatus] = useState('');

  // Modal state
  const [modalOpen, setModalOpen] = useState(false);
  const [editRouteId, setEditRouteId] = useState<string | null>(null);

  // Delete state
  const [deleteRoute, setDeleteRoute] = useState<RouteListItem | null>(null);
  const [deleting, setDeleting] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const params: Record<string, string> = {};
      if (search) params.search = search;
      if (routeType) params.route_type = routeType;
      if (status) params.status = status;

      const [routesResp, provsResp] = await Promise.all([
        routesApi.list(params),
        providersApi.list(),
      ]);
      setRoutes(routesResp.routes ?? []);
      const provMap = new Map<string, Provider>();
      for (const p of provsResp.providers ?? []) {
        provMap.set(p.id, p);
      }
      setProviders(provMap);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось загрузить маршруты');
    } finally {
      setLoading(false);
    }
  }, [search, routeType, status]);

  useEffect(() => {
    load();
  }, [load]);

  function handleFilterChange(key: 'route_type' | 'status', value: string) {
    if (key === 'route_type') setRouteType(value);
    if (key === 'status') setStatus(value);
  }

  function handleEdit(route: RouteListItem) {
    setEditRouteId(route.id);
    setModalOpen(true);
  }

  function handleAdd() {
    setEditRouteId(null);
    setModalOpen(true);
  }

  async function handleDelete() {
    if (!deleteRoute) return;
    setDeleting(true);
    try {
      await routesApi.remove(deleteRoute.id);
      setDeleteRoute(null);
      load();
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'Не удалось удалить маршрут');
    } finally {
      setDeleting(false);
    }
  }

  if (isSubAccount) {
    return (
      <div>
        <div className="bg-blue-50 border border-blue-200 rounded-lg p-6 max-w-2xl">
          <p className="text-sm text-blue-900">
            В режиме суб-аккаунта маршрутизацию настраивает агрегатор. Вы не управляете маршрутами самостоятельно — трафик направляется по правилам агрегатора.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div>
      <PageHeader
        subtitle="Управление маршрутами доставки сообщений"
        actions={
          <Button onClick={handleAdd}>+ Добавить маршрут</Button>
        }
      />

      {error && <p className="text-red-600 mb-4">{error}</p>}

      <RouteFilters
        search={search}
        onSearch={setSearch}
        routeType={routeType}
        status={status}
        onFilterChange={handleFilterChange}
      />

      <RouteTable
        routes={routes}
        providers={providers}
        loading={loading}
        onEdit={handleEdit}
        onDelete={setDeleteRoute}
      />

      <RouteModal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        onSaved={load}
        routeId={editRouteId}
      />

      <ConfirmDialog
        open={deleteRoute !== null}
        onConfirm={handleDelete}
        onCancel={() => setDeleteRoute(null)}
        title="Удалить маршрут"
        description={`Вы уверены, что хотите удалить маршрут "${deleteRoute?.name}"?`}
        confirmLabel="Удалить"
        variant="danger"
        loading={deleting}
      />
    </div>
  );
}
