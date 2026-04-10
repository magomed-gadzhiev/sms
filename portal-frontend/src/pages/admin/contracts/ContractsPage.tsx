import { useState, useEffect, useCallback, useMemo } from 'react';
import { PageHeader } from '../../../components/layout/PageHeader';
import { DataTable, type Column } from '../../../components/data/DataTable';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { Select } from '../../../components/ui/Select';
import { Modal } from '../../../components/ui/Modal';
import { ConfirmDialog } from '../../../components/ui/ConfirmDialog';
import { Badge } from '../../../components/ui/Badge';
import { useToast } from '../../../components/ui/Toast';
import {
  contractsApi,
  clientsApi,
  legalEntitiesApi,
  type Contract,
  type ClientInfo,
  type LegalEntity,
} from '../../../api/admin';

// Expiry computation helpers
type ExpiryStatus = 'active' | 'expiring' | 'expired';

function getExpiryStatus(endDate: string | null | undefined): ExpiryStatus {
  if (!endDate) return 'active';
  const end = new Date(endDate);
  const now = new Date();
  const diffDays = Math.ceil((end.getTime() - now.getTime()) / (1000 * 60 * 60 * 24));
  if (diffDays < 0) return 'expired';
  if (diffDays <= 30) return 'expiring';
  return 'active';
}

const STATUS_LABEL: Record<string, string> = {
  active: 'Активный',
  expired: 'Истёк',
  terminated: 'Расторгнут',
};

const STATUS_VARIANT: Record<string, 'success' | 'danger' | 'warning' | 'default'> = {
  active: 'success',
  expired: 'warning',
  terminated: 'danger',
};

const STATUS_OPTIONS = [
  { value: 'active', label: 'Активный' },
  { value: 'expired', label: 'Истёк' },
  { value: 'terminated', label: 'Расторгнут' },
];

interface FormState {
  contract_number: string;
  client_id: string;
  legal_entity_id: string;
  status: string;
  start_date: string;
  end_date: string;
  description: string;
}

const EMPTY_FORM: FormState = {
  contract_number: '',
  client_id: '',
  legal_entity_id: '',
  status: 'active',
  start_date: '',
  end_date: '',
  description: '',
};

type StatusFilterValue = 'all' | 'active' | 'expiring' | 'expired' | 'terminated';

export function ContractsPage() {
  const toast = useToast();
  const [items, setItems] = useState<Contract[]>([]);
  const [loading, setLoading] = useState(false);
  const [clients, setClients] = useState<ClientInfo[]>([]);
  const [legalEntities, setLegalEntities] = useState<LegalEntity[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [editItem, setEditItem] = useState<Contract | null>(null);
  const [deleteItem, setDeleteItem] = useState<Contract | null>(null);
  const [form, setForm] = useState<FormState>(EMPTY_FORM);
  const [saving, setSaving] = useState(false);
  const [statusFilter, setStatusFilter] = useState<StatusFilterValue>('all');

  const filteredContracts = useMemo(() => {
    if (statusFilter === 'all') return items;
    if (statusFilter === 'expiring') return items.filter(c => getExpiryStatus(c.end_date) === 'expiring');
    if (statusFilter === 'expired') return items.filter(c => getExpiryStatus(c.end_date) === 'expired');
    if (statusFilter === 'terminated') return items.filter(c => c.status === 'terminated');
    return items.filter(c => c.status === statusFilter);
  }, [items, statusFilter]);

  const fetchItems = useCallback(async () => {
    setLoading(true);
    try {
      const res = await contractsApi.list({ limit: 200 });
      setItems(res.contracts || []);
    } catch {
      toast.error('Не удалось загрузить договоры');
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    fetchItems();
    clientsApi.list({ limit: 500 }).then((r) => setClients(r.clients || [])).catch(() => {});
    legalEntitiesApi.list({ limit: 200 }).then((r) => setLegalEntities(r.legal_entities || [])).catch(() => {});
  }, [fetchItems]);

  const clientOptions = [
    { value: '', label: 'Выберите клиента' },
    ...clients.map((c) => ({ value: c.client_id, label: c.name })),
  ];

  const legalEntityOptions = [
    { value: '', label: 'Без юр. лица' },
    ...legalEntities.map((le) => ({ value: le.id, label: `${le.inn} — ${le.name}` })),
  ];

  function openCreate() {
    setEditItem(null);
    setForm(EMPTY_FORM);
    setShowForm(true);
  }

  function openEdit(item: Contract) {
    setEditItem(item);
    setForm({
      contract_number: item.contract_number,
      client_id: item.client_id,
      legal_entity_id: item.legal_entity_id || '',
      status: item.status,
      start_date: item.start_date || '',
      end_date: item.end_date || '',
      description: item.description || '',
    });
    setShowForm(true);
  }

  async function handleSave() {
    if (!form.contract_number.trim() || !form.client_id || !form.start_date) {
      toast.error('Номер договора, клиент и дата начала обязательны');
      return;
    }
    setSaving(true);
    try {
      if (editItem) {
        await contractsApi.update(editItem.id, form);
        toast.success('Договор обновлён');
      } else {
        await contractsApi.create(form);
        toast.success('Договор создан');
      }
      setShowForm(false);
      fetchItems();
    } catch {
      toast.error('Не удалось сохранить');
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete() {
    if (!deleteItem) return;
    try {
      await contractsApi.delete(deleteItem.id);
      toast.success('Договор удалён');
      setDeleteItem(null);
      fetchItems();
    } catch {
      toast.error('Не удалось удалить');
    }
  }

  const columns: Column<Contract>[] = [
    {
      key: 'contract_number',
      header: 'Номер договора',
      render: (row) => <span className="font-mono text-sm font-medium">{row.contract_number}</span>,
    },
    {
      key: 'client_name',
      header: 'Клиент',
      render: (row) => <span className="text-sm">{row.client_name || '—'}</span>,
    },
    {
      key: 'legal_entity_name',
      header: 'Юридическое лицо',
      render: (row) =>
        row.legal_entity_inn ? (
          <div>
            <span className="font-mono text-xs text-gray-500">{row.legal_entity_inn}</span>
            <span className="ml-2 text-sm">{row.legal_entity_name}</span>
          </div>
        ) : (
          <span className="text-gray-400 text-sm">—</span>
        ),
    },
    {
      key: 'status',
      header: 'Статус',
      render: (row) => {
        const expiry = getExpiryStatus(row.end_date);
        return (
          <div className="flex flex-col gap-1">
            <Badge variant={STATUS_VARIANT[row.status] ?? 'default'}>
              {STATUS_LABEL[row.status] ?? row.status}
            </Badge>
            {expiry === 'expiring' && <Badge variant="warning">Истекает</Badge>}
            {expiry === 'expired' && row.status === 'active' && <Badge variant="danger">Срок истёк</Badge>}
          </div>
        );
      },
    },
    {
      key: 'start_date',
      header: 'С',
      render: (row) => <span className="text-sm text-gray-600">{row.start_date || '—'}</span>,
    },
    {
      key: 'end_date',
      header: 'По',
      render: (row) => <span className="text-sm text-gray-600">{row.end_date || '—'}</span>,
    },
  ];

  return (
    <>
      <PageHeader
        title="Договоры"
        breadcrumbs={[{ label: 'Админ', href: '/admin/dashboard' }, { label: 'Договоры' }]}
        actions={<Button onClick={openCreate}>+ Добавить</Button>}
      />

      <div className="bg-white border border-gray-200 rounded-lg overflow-hidden">
        <div className="p-4 border-b border-gray-200">
          <Select
            label="Статус"
            value={statusFilter}
            onChange={(v) => setStatusFilter(v as StatusFilterValue)}
            options={[
              { value: 'all', label: 'Все' },
              { value: 'active', label: 'Активные' },
              { value: 'expiring', label: 'Истекающие (≤30 дней)' },
              { value: 'expired', label: 'Истёкшие' },
              { value: 'terminated', label: 'Расторгнутые' },
            ]}
          />
        </div>
        <DataTable
          columns={columns}
          data={filteredContracts}
          total={filteredContracts.length}
          page={1}
          pageSize={filteredContracts.length || 1}
          onPageChange={() => {}}
          loading={loading}
          rowActions={(row) => (
            <div className="flex gap-2">
              <Button size="sm" variant="ghost" onClick={() => openEdit(row)}>Изменить</Button>
              <Button size="sm" variant="ghost" onClick={() => setDeleteItem(row)}>Удалить</Button>
            </div>
          )}
        />
      </div>

      <Modal
        open={showForm}
        onClose={() => setShowForm(false)}
        title={editItem ? 'Изменить договор' : 'Добавить договор'}
      >
        <div className="space-y-4">
          <Input
            label="Номер договора *"
            value={form.contract_number}
            onChange={(e) => setForm((f) => ({ ...f, contract_number: e.target.value }))}
            placeholder="№ 123/2026"
          />
          <Select
            label="Клиент *"
            value={form.client_id}
            onChange={(v) => setForm((f) => ({ ...f, client_id: v }))}
            options={clientOptions}
          />
          <Select
            label="Юридическое лицо"
            value={form.legal_entity_id}
            onChange={(v) => setForm((f) => ({ ...f, legal_entity_id: v }))}
            options={legalEntityOptions}
          />
          <Select
            label="Статус"
            value={form.status}
            onChange={(v) => setForm((f) => ({ ...f, status: v }))}
            options={STATUS_OPTIONS}
          />
          <div className="grid grid-cols-2 gap-4">
            <Input
              label="Дата начала *"
              type="date"
              value={form.start_date}
              onChange={(e) => setForm((f) => ({ ...f, start_date: e.target.value }))}
            />
            <Input
              label="Дата окончания"
              type="date"
              value={form.end_date}
              onChange={(e) => setForm((f) => ({ ...f, end_date: e.target.value }))}
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Описание</label>
            <textarea
              className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary/50"
              rows={3}
              value={form.description}
              onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
              placeholder="Дополнительная информация о договоре"
            />
          </div>
          <div className="flex gap-2 justify-end pt-2">
            <Button variant="ghost" onClick={() => setShowForm(false)}>Отмена</Button>
            <Button onClick={handleSave} disabled={saving}>
              {saving ? 'Сохранение...' : 'Сохранить'}
            </Button>
          </div>
        </div>
      </Modal>

      <ConfirmDialog
        open={!!deleteItem}
        onConfirm={handleDelete}
        onCancel={() => setDeleteItem(null)}
        title="Удалить договор?"
        description={`Договор ${deleteItem?.contract_number} будет удалён. Это действие нельзя отменить.`}
        confirmLabel="Удалить"
        variant="danger"
      />
    </>
  );
}
