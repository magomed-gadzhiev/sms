import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { FilterBar, type FilterDef } from '../../components/data/FilterBar';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Select } from '../../components/ui/Select';
import { Modal } from '../../components/ui/Modal';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { StatusBadge } from '../../components/ui/Badge';
import { useToast } from '../../components/ui/Toast';
import { providersApi, type ProviderInfo, type ProviderHealth } from '../../api/admin';

const PAGE_SIZE = 20;

const filters: FilterDef[] = [
  { key: 'active_only', label: 'Статус', type: 'select', options: [
    { value: 'true', label: 'Только активные' },
  ]},
];

export function ProvidersPage() {
  const toast = useToast();
  const [data, setData] = useState<ProviderInfo[]>([]);
  const [healthMap, setHealthMap] = useState<Record<string, ProviderHealth>>({});
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [filterValues, setFilterValues] = useState<Record<string, string>>({});
  const [showCreate, setShowCreate] = useState(false);
  const [editProvider, setEditProvider] = useState<ProviderInfo | null>(null);
  const [deleteProvider, setDeleteProvider] = useState<ProviderInfo | null>(null);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({ name: '', host: '', port: 2775, system_id: '', password: '', system_type: '', bind_type: 1, max_connections: 1, window_size: 10, active: true });

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await providersApi.list({ active_only: filterValues.active_only === 'true' ? true : undefined, limit: PAGE_SIZE, offset: (page - 1) * PAGE_SIZE });
      setData(res.providers || []);
      setTotal(res.total);
      const healthEntries = await Promise.allSettled((res.providers || []).map((p) => providersApi.health(p.provider_id).then((h) => [p.provider_id, h] as const)));
      const map: Record<string, ProviderHealth> = {};
      healthEntries.forEach((r) => { if (r.status === 'fulfilled') map[r.value[0]] = r.value[1]; });
      setHealthMap(map);
    } catch { toast.error('Не удалось загрузить провайдеров'); }
    finally { setLoading(false); }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, filterValues]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const columns: Column<ProviderInfo>[] = [
    { key: 'name', header: 'Название', sortable: true },
    { key: 'host', header: 'Хост', render: (p) => `${p.host}:${p.port}` },
    { key: 'system_id', header: 'System ID' },
    { key: 'max_connections', header: 'Макс. подкл.' },
    { key: 'active', header: 'Статус', render: (p) => <StatusBadge status={p.active ? 'active' : 'inactive'} /> },
    { key: 'health', header: 'Здоровье', render: (p) => { const h = healthMap[p.provider_id]; return h ? <StatusBadge status={h.status} /> : '-'; }},
    { key: 'success_rate', header: 'Успех %', render: (p) => { const h = healthMap[p.provider_id]; return h ? `${h.success_rate}%` : '-'; }},
  ];

  const openCreate = () => { setForm({ name: '', host: '', port: 2775, system_id: '', password: '', system_type: '', bind_type: 1, max_connections: 1, window_size: 10, active: true }); setShowCreate(true); };
  const openEdit = (provider: ProviderInfo) => { setForm({ name: provider.name, host: provider.host, port: provider.port, system_id: provider.system_id, password: '', system_type: provider.system_type, bind_type: provider.bind_type, max_connections: provider.max_connections, window_size: provider.window_size, active: provider.active }); setEditProvider(provider); };

  const handleSave = async () => {
    setSaving(true);
    try {
      if (editProvider) { const { password: _p, ...rest } = form; await providersApi.update(editProvider.provider_id, rest); toast.success('Провайдер обновлён'); setEditProvider(null); }
      else { await providersApi.create(form); toast.success('Провайдер создан'); setShowCreate(false); }
      fetchData();
    } catch (e) { toast.error(e instanceof Error ? e.message : 'Save failed'); }
    finally { setSaving(false); }
  };

  const handleDelete = async () => {
    if (!deleteProvider) return;
    setSaving(true);
    try { await providersApi.delete(deleteProvider.provider_id); toast.success('Провайдер удалён'); setDeleteProvider(null); fetchData(); }
    catch (e) { toast.error(e instanceof Error ? e.message : 'Delete failed'); }
    finally { setSaving(false); }
  };

  return (
    <>
      <PageHeader title="Провайдеры" subtitle={`${total} провайдеров`} breadcrumbs={[{ label: 'Админ', href: '/admin/dashboard' }, { label: 'Провайдеры' }]} actions={<Button onClick={openCreate}>Добавить провайдера</Button>} />
      <FilterBar filters={filters} values={filterValues} onChange={(v) => { setFilterValues(v); setPage(1); }} onReset={() => { setFilterValues({}); setPage(1); }} />
      <DataTable columns={columns} data={data} total={total} page={page} pageSize={PAGE_SIZE} onPageChange={setPage} loading={loading} keyField="provider_id"
        rowActions={(p) => (<div className="flex gap-1"><Button size="sm" variant="ghost" onClick={() => openEdit(p)}>Изменить</Button><Button size="sm" variant="ghost" onClick={() => setDeleteProvider(p)}>Удалить</Button></div>)}
      />
      <Modal open={showCreate || !!editProvider} onClose={() => { setShowCreate(false); setEditProvider(null); }} title={editProvider ? 'Редактирование провайдера' : 'Добавление провайдера'} wide>
        <div className="grid grid-cols-2 gap-4">
          <Input label="Название" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
          <Input label="Хост" value={form.host} onChange={(e) => setForm({ ...form, host: e.target.value })} required />
          <Input label="Порт" type="number" value={String(form.port)} onChange={(e) => setForm({ ...form, port: Number(e.target.value) })} required />
          <Input label="System ID" value={form.system_id} onChange={(e) => setForm({ ...form, system_id: e.target.value })} required />
          {!editProvider && <Input label="Пароль" type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} required />}
          <Input label="Тип системы" value={form.system_type} onChange={(e) => setForm({ ...form, system_type: e.target.value })} />
          <Input label="Макс. подключений" type="number" value={String(form.max_connections)} onChange={(e) => setForm({ ...form, max_connections: Number(e.target.value) })} />
          <Input label="Размер окна" type="number" value={String(form.window_size)} onChange={(e) => setForm({ ...form, window_size: Number(e.target.value) })} />
          <Select label="Статус" options={[{ value: 'true', label: 'Активен' }, { value: 'false', label: 'Неактивен' }]} value={String(form.active)} onChange={(v) => setForm({ ...form, active: v === 'true' })} />
        </div>
        <div className="flex justify-end gap-3 pt-4">
          <Button variant="secondary" onClick={() => { setShowCreate(false); setEditProvider(null); }}>Отмена</Button>
          <Button onClick={handleSave} disabled={saving || !form.name || !form.host}>{saving ? 'Сохранение...' : 'Сохранить'}</Button>
        </div>
      </Modal>
      <ConfirmDialog open={!!deleteProvider} onConfirm={handleDelete} onCancel={() => setDeleteProvider(null)} title="Удаление провайдера" description={`Удалить "${deleteProvider?.name}"? Это действие необратимо.`} confirmLabel="Удалить" variant="danger" loading={saving} />
    </>
  );
}
