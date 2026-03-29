import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Select } from '../../components/ui/Select';
import { Modal } from '../../components/ui/Modal';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { StatusBadge } from '../../components/ui/Badge';
import { useToast } from '../../components/ui/Toast';
import { routesApi, type RouteInfo } from '../../api/admin';

const PAGE_SIZE = 20;

const columns: Column<RouteInfo>[] = [
  { key: 'name', header: 'Название', sortable: true },
  { key: 'pattern', header: 'Шаблон' },
  { key: 'priority', header: 'Приоритет', sortable: true },
  { key: 'load_balance_strategy', header: 'Стратегия', render: (r) => ({ round_robin: 'По кругу', weighted: 'Взвешенная', priority: 'Приоритет' }[r.load_balance_strategy] ?? r.load_balance_strategy) },
  { key: 'failover_enabled', header: 'Отказоуст.', render: (r) => r.failover_enabled ? 'Да' : 'Нет' },
  { key: 'provider_ids', header: 'Провайдеры', render: (r) => String(r.provider_ids?.length ?? 0) },
  { key: 'active', header: 'Статус', render: (r) => <StatusBadge status={r.active ? 'active' : 'inactive'} /> },
];

export function RoutesPage() {
  const toast = useToast();
  const [data, setData] = useState<RouteInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [editRoute, setEditRoute] = useState<RouteInfo | null>(null);
  const [deleteRoute, setDeleteRoute] = useState<RouteInfo | null>(null);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({ name: '', pattern: '', priority: 0, provider_ids: '', load_balance_strategy: 'round_robin', failover_enabled: true });

  const fetchData = useCallback(async () => {
    setLoading(true);
    try { const res = await routesApi.list({ limit: PAGE_SIZE, offset: (page - 1) * PAGE_SIZE }); setData(res.routes || []); setTotal(res.total); }
    catch { toast.error('Не удалось загрузить маршруты'); }
    finally { setLoading(false); }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const openCreate = () => { setForm({ name: '', pattern: '', priority: 0, provider_ids: '', load_balance_strategy: 'round_robin', failover_enabled: true }); setShowForm(true); };
  const openEdit = (route: RouteInfo) => { setForm({ name: route.name, pattern: route.pattern, priority: route.priority, provider_ids: (route.provider_ids || []).join(', '), load_balance_strategy: route.load_balance_strategy, failover_enabled: route.failover_enabled }); setEditRoute(route); setShowForm(true); };

  const handleSave = async () => {
    setSaving(true);
    try {
      const payload = { ...form, priority: Number(form.priority), provider_ids: form.provider_ids.split(',').map((s) => s.trim()).filter(Boolean) };
      if (editRoute) { await routesApi.update(editRoute.route_id, payload); toast.success('Маршрут обновлён'); }
      else { await routesApi.create(payload); toast.success('Маршрут создан'); }
      setShowForm(false); setEditRoute(null); fetchData();
    } catch (e) { toast.error(e instanceof Error ? e.message : 'Ошибка сохранения'); }
    finally { setSaving(false); }
  };

  const handleDelete = async () => {
    if (!deleteRoute) return;
    setSaving(true);
    try { await routesApi.delete(deleteRoute.route_id); toast.success('Маршрут удалён'); setDeleteRoute(null); fetchData(); }
    catch (e) { toast.error(e instanceof Error ? e.message : 'Ошибка удаления'); }
    finally { setSaving(false); }
  };

  return (
    <>
      <PageHeader title="Маршруты" subtitle={`${total} маршрутов`} breadcrumbs={[{ label: 'Админ', href: '/admin/dashboard' }, { label: 'Маршруты' }]} actions={<Button onClick={openCreate}>Создать маршрут</Button>} />
      <DataTable columns={columns} data={data} total={total} page={page} pageSize={PAGE_SIZE} onPageChange={setPage} loading={loading} keyField="route_id"
        rowActions={(r) => (<div className="flex gap-1"><Button size="sm" variant="ghost" onClick={() => openEdit(r)}>Изменить</Button><Button size="sm" variant="ghost" onClick={() => setDeleteRoute(r)}>Удалить</Button></div>)}
      />
      <Modal open={showForm} onClose={() => { setShowForm(false); setEditRoute(null); }} title={editRoute ? 'Редактирование маршрута' : 'Создание маршрута'}>
        <div className="space-y-4">
          <Input label="Название" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
          <Input label="Шаблон (regex)" value={form.pattern} onChange={(e) => setForm({ ...form, pattern: e.target.value })} required />
          <Input label="Приоритет" type="number" value={String(form.priority)} onChange={(e) => setForm({ ...form, priority: Number(e.target.value) })} />
          <Input label="ID провайдеров (через запятую)" value={form.provider_ids} onChange={(e) => setForm({ ...form, provider_ids: e.target.value })} required />
          <Select label="Стратегия" options={[{ value: 'round_robin', label: 'По кругу' }, { value: 'weighted', label: 'Взвешенная' }, { value: 'priority', label: 'Приоритет' }]} value={form.load_balance_strategy} onChange={(v) => setForm({ ...form, load_balance_strategy: v })} />
          <Select label="Отказоустойчивость" options={[{ value: 'true', label: 'Включена' }, { value: 'false', label: 'Выключена' }]} value={String(form.failover_enabled)} onChange={(v) => setForm({ ...form, failover_enabled: v === 'true' })} />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => { setShowForm(false); setEditRoute(null); }}>Отмена</Button>
            <Button onClick={handleSave} disabled={saving || !form.name || !form.pattern}>{saving ? 'Сохранение...' : 'Сохранить'}</Button>
          </div>
        </div>
      </Modal>
      <ConfirmDialog open={!!deleteRoute} onConfirm={handleDelete} onCancel={() => setDeleteRoute(null)} title="Удаление маршрута" description={`Удалить "${deleteRoute?.name}"? Это действие необратимо.`} confirmLabel="Удалить" variant="danger" loading={saving} />
    </>
  );
}
