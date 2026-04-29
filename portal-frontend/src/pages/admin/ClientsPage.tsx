import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { FilterBar, type FilterDef } from '../../components/data/FilterBar';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Modal } from '../../components/ui/Modal';
import { Badge } from '../../components/ui/Badge';
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
  { key: 'active', header: 'Статус', render: (c) => c.active
    ? <Badge variant="success">Активен</Badge>
    : <Badge variant="danger">Заблокирован</Badge>
  },
  { key: 'rate_limits', header: 'Лимит (сообщ/с)', render: (c) => { const v = c.rate_limits?.messages_per_second; return (!v || v <= 0) ? 'Без лимита' : String(v); } },
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
  const [blockClient, setBlockClient] = useState<ClientInfo | null>(null);
  const [blockReason, setBlockReason] = useState('');
  const [unblockClient, setUnblockClient] = useState<ClientInfo | null>(null);
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
        // Не отправляем active — статус управляется через Block/Unblock,
        // редактирование профиля не должно разблокировать заблокированного.
        const { active: _ignored, ...editPayload } = form;
        void _ignored;
        await clientsApi.update(editClient.client_id, editPayload);
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

  const handleBlock = async () => {
    if (!blockClient) return;
    if (!blockReason.trim()) {
      toast.error('Укажите причину блокировки');
      return;
    }
    setSaving(true);
    try {
      // Backend пока хранит только active flag; reason отображается в UI/toast
      // и должен попасть в audit log на стороне сервера через UpdateClient.
      // Полный каскад (refresh-tokens, SMPP-disconnect, pipeline cancel) —
      // отдельный план M-уровня.
      await clientsApi.update(blockClient.client_id, { active: false });
      toast.success(`Клиент "${blockClient.name}" заблокирован`);
      setBlockClient(null);
      setBlockReason('');
      fetchData();
    } catch (e) { toast.error(e instanceof Error ? e.message : 'Ошибка блокировки'); }
    finally { setSaving(false); }
  };

  const handleUnblock = async () => {
    if (!unblockClient) return;
    setSaving(true);
    try {
      await clientsApi.update(unblockClient.client_id, { active: true });
      toast.success(`Клиент "${unblockClient.name}" разблокирован`);
      setUnblockClient(null);
      fetchData();
    } catch (e) { toast.error(e instanceof Error ? e.message : 'Ошибка разблокировки'); }
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
            {client.active ? (
              <Button size="sm" variant="danger" onClick={() => { setBlockClient(client); setBlockReason(''); }}>Заблокировать</Button>
            ) : (
              <Button size="sm" variant="ghost" onClick={() => setUnblockClient(client)}>Разблокировать</Button>
            )}
          </div>
        )}
      />
      <Modal open={showCreate || !!editClient} onClose={() => { setShowCreate(false); setEditClient(null); }} title={editClient ? 'Редактирование клиента' : 'Создание клиента'}>
        <div className="space-y-4">
          <Input label="Название" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
          <Input label="Email" type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} required />
          <Input label="Контактное лицо" value={form.contact_person} onChange={(e) => setForm({ ...form, contact_person: e.target.value })} />
          <Input label="Телефон" value={form.phone} onChange={(e) => setForm({ ...form, phone: e.target.value })} />
          {editClient && (
            <p className="text-xs text-gray-500">
              Статус блокировки управляется кнопками «Заблокировать»/«Разблокировать» в списке.
            </p>
          )}
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => { setShowCreate(false); setEditClient(null); }}>Отмена</Button>
            <Button onClick={handleSave} disabled={saving || !form.name || !form.email}>{saving ? 'Сохранение...' : 'Сохранить'}</Button>
          </div>
        </div>
      </Modal>

      <Modal open={!!blockClient} onClose={() => { setBlockClient(null); setBlockReason(''); }} title="Заблокировать клиента">
        <div className="space-y-4">
          <p className="text-sm text-gray-700">
            Клиент <strong>{blockClient?.name}</strong> будет заблокирован. Доступ к аккаунту через
            авторизацию будет ограничен.
          </p>
          <p className="text-xs text-amber-700 bg-amber-50 border border-amber-200 rounded px-3 py-2">
            ⚠ Текущая реализация: устанавливается флаг неактивности. Каскад (отзыв refresh-токенов,
            закрытие SMPP-сессий, отмена сообщений в pipeline) — отдельная задача,
            запланирована.
          </p>
          <div>
            <label htmlFor="block-reason" className="block text-sm font-medium text-gray-700 mb-1">
              Причина блокировки <span className="text-red-600">*</span>
            </label>
            <textarea
              id="block-reason"
              value={blockReason}
              onChange={(e) => setBlockReason(e.target.value)}
              maxLength={500}
              rows={3}
              required
              placeholder="Например: нарушение условий использования"
              className="w-full rounded border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary"
            />
            <p className="mt-1 text-xs text-gray-500">{blockReason.length}/500</p>
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => { setBlockClient(null); setBlockReason(''); }}>Отмена</Button>
            <Button variant="danger" onClick={handleBlock} disabled={saving || !blockReason.trim()}>
              {saving ? 'Блокировка...' : 'Заблокировать'}
            </Button>
          </div>
        </div>
      </Modal>

      <Modal open={!!unblockClient} onClose={() => setUnblockClient(null)} title="Разблокировать клиента">
        <div className="space-y-4">
          <p className="text-sm text-gray-700">
            Восстановить доступ для клиента <strong>{unblockClient?.name}</strong>?
          </p>
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setUnblockClient(null)}>Отмена</Button>
            <Button onClick={handleUnblock} disabled={saving}>
              {saving ? 'Разблокировка...' : 'Разблокировать'}
            </Button>
          </div>
        </div>
      </Modal>
    </>
  );
}
