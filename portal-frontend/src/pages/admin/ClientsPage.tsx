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
import { clientsApi, type ClientInfo } from '../../api/admin';

const PAGE_SIZE = 20;

const filters: FilterDef[] = [
  { key: 'search', label: 'Поиск', type: 'text', placeholder: 'Имя или email...' },
  { key: 'active_only', label: 'Статус', type: 'select', options: [
    { value: 'true', label: 'Только активные' },
  ]},
];

const columns: Column<ClientInfo>[] = [
  { key: 'name', header: 'Название', sortable: true },
  { key: 'email', header: 'Email' },
  { key: 'contact_person', header: 'Контакт' },
  { key: 'active', header: 'Статус', render: (c) => <StatusBadge status={c.active ? 'active' : 'inactive'} /> },
  { key: 'rate_limits', header: 'Лимит (сообщ/с)', render: (c) => String(c.rate_limits?.messages_per_second ?? '-') },
  { key: 'created_at', header: 'Создан', render: (c) => new Date(c.created_at).toLocaleDateString() },
];

export function ClientsPage() {
  const toast = useToast();
  const [data, setData] = useState<ClientInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [filterValues, setFilterValues] = useState<Record<string, string>>({});
  const [showCreate, setShowCreate] = useState(false);
  const [editClient, setEditClient] = useState<ClientInfo | null>(null);
  const [deleteClient, setDeleteClient] = useState<ClientInfo | null>(null);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({ name: '', email: '', contact_person: '', phone: '', active: true });

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await clientsApi.list({
        search: filterValues.search || undefined,
        active_only: filterValues.active_only === 'true' ? true : undefined,
        limit: PAGE_SIZE, offset: (page - 1) * PAGE_SIZE,
      });
      setData(res.clients || []);
      setTotal(res.total);
    } catch { toast.error('Не удалось загрузить клиентов'); }
    finally { setLoading(false); }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, filterValues]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const openCreate = () => {
    setForm({ name: '', email: '', contact_person: '', phone: '', active: true });
    setShowCreate(true);
  };

  const openEdit = (client: ClientInfo) => {
    setForm({ name: client.name, email: client.email, contact_person: client.contact_person, phone: client.phone, active: client.active });
    setEditClient(client);
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      if (editClient) {
        await clientsApi.update(editClient.client_id, form);
        toast.success('Клиент обновлён');
        setEditClient(null);
      } else {
        await clientsApi.create(form);
        toast.success('Клиент создан');
        setShowCreate(false);
      }
      fetchData();
    } catch (e) { toast.error(e instanceof Error ? e.message : 'Ошибка сохранения'); }
    finally { setSaving(false); }
  };

  const handleDelete = async () => {
    if (!deleteClient) return;
    setSaving(true);
    try {
      await clientsApi.delete(deleteClient.client_id);
      toast.success('Клиент деактивирован');
      setDeleteClient(null);
      fetchData();
    } catch (e) { toast.error(e instanceof Error ? e.message : 'Ошибка удаления'); }
    finally { setSaving(false); }
  };

  return (
    <>
      <PageHeader title="Клиенты" subtitle={`${total} клиентов`} breadcrumbs={[{ label: 'Админ', href: '/admin/dashboard' }, { label: 'Клиенты' }]} actions={<Button onClick={openCreate}>Создать клиента</Button>} />
      <FilterBar filters={filters} values={filterValues} onChange={(v) => { setFilterValues(v); setPage(1); }} onReset={() => { setFilterValues({}); setPage(1); }} />
      <DataTable columns={columns} data={data} total={total} page={page} pageSize={PAGE_SIZE} onPageChange={setPage} loading={loading} keyField="client_id"
        rowActions={(client) => (
          <div className="flex gap-1">
            <Button size="sm" variant="ghost" onClick={() => openEdit(client)}>Изменить</Button>
            <Button size="sm" variant="ghost" onClick={() => setDeleteClient(client)}>Удалить</Button>
          </div>
        )}
      />
      <Modal open={showCreate || !!editClient} onClose={() => { setShowCreate(false); setEditClient(null); }} title={editClient ? 'Редактирование клиента' : 'Создание клиента'}>
        <div className="space-y-4">
          <Input label="Название" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
          <Input label="Email" type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} required />
          <Input label="Контактное лицо" value={form.contact_person} onChange={(e) => setForm({ ...form, contact_person: e.target.value })} />
          <Input label="Телефон" value={form.phone} onChange={(e) => setForm({ ...form, phone: e.target.value })} />
          <Select label="Статус" options={[{ value: 'true', label: 'Активен' }, { value: 'false', label: 'Неактивен' }]} value={String(form.active)} onChange={(v) => setForm({ ...form, active: v === 'true' })} />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => { setShowCreate(false); setEditClient(null); }}>Отмена</Button>
            <Button onClick={handleSave} disabled={saving || !form.name || !form.email}>{saving ? 'Сохранение...' : 'Сохранить'}</Button>
          </div>
        </div>
      </Modal>
      <ConfirmDialog open={!!deleteClient} onConfirm={handleDelete} onCancel={() => setDeleteClient(null)} title="Деактивация клиента" description={`Деактивировать "${deleteClient?.name}"? Клиент станет неактивным.`} confirmLabel="Деактивировать" variant="danger" loading={saving} />
    </>
  );
}
