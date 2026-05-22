import { useState, useEffect, useCallback } from 'react';
import { DataTable, type Column } from '../../../components/data/DataTable';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { Select } from '../../../components/ui/Select';
import { SearchableSelect } from '../../../components/ui/SearchableSelect';
import { Modal } from '../../../components/ui/Modal';
import { StatusBadge } from '../../../components/ui/Badge';
import { useToast } from '../../../components/ui/Toast';
import { tarificationApi, clientsApi, operatorsApi, type SenderRegistration, type ClientInfo, type OperatorInfo } from '../../../api/admin';

const PAGE_SIZE = 20;

const typeOptions = [
  { value: 'alphanumeric', label: 'Alphanumeric' },
  { value: 'numeric', label: 'Numeric' },
  { value: 'shortcode', label: 'Shortcode' },
];

const statusOptions = [
  { value: 'pending', label: 'Ожидает' },
  { value: 'approved', label: 'Одобрено' },
  { value: 'rejected', label: 'Отклонено' },
  { value: 'blocked', label: 'Заблокировано' },
];

export function SenderRegistrationsTab() {
  const toast = useToast();
  const [registrations, setRegistrations] = useState<SenderRegistration[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [showStatusEdit, setShowStatusEdit] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({
    client_id: '',
    operator_id: '',
    sender_name: '',
    type: 'alphanumeric',
  });
  const [statusForm, setStatusForm] = useState({ id: '', status: '', type: '' });
  const [clients, setClients] = useState<ClientInfo[]>([]);
  const [operators, setOperators] = useState<OperatorInfo[]>([]);
  const [refLoading, setRefLoading] = useState(false);

  const clientOptions = clients.map((c) => ({ value: c.client_id, label: c.name }));
  const operatorOptions = operators.map((o) => ({ value: o.operator_id, label: `${o.name} (${o.mcc}/${o.mnc})` }));

  useEffect(() => {
    setRefLoading(true);
    Promise.all([
      clientsApi.list({ limit: 500 }),
      operatorsApi.list({ limit: 500 }),
    ]).then(([c, o]) => {
      setClients(c.clients || []);
      setOperators(o.operators || []);
    }).catch(() => {}).finally(() => setRefLoading(false));
  }, []);

  const fetchRegistrations = useCallback(async () => {
    setLoading(true);
    try {
      const res = await tarificationApi.listSenderRegistrations({
        limit: PAGE_SIZE,
        offset: (page - 1) * PAGE_SIZE,
      });
      setRegistrations(res.registrations || []);
      setTotal(res.total);
    } catch {
      toast.error('Не удалось загрузить регистрации');
    } finally {
      setLoading(false);
    }
  }, [page, toast]);

  useEffect(() => {
    fetchRegistrations();
  }, [fetchRegistrations]);

  const handleCreate = async () => {
    if (
      !form.client_id.trim() ||
      !form.operator_id.trim() ||
      !form.sender_name.trim()
    ) {
      toast.error('Заполните все обязательные поля');
      return;
    }
    setSaving(true);
    try {
      await tarificationApi.createSenderRegistration({
        client_id: form.client_id,
        operator_id: form.operator_id,
        sender_name: form.sender_name,
        type: form.type,
      });
      toast.success('Регистрация создана');
      setShowCreate(false);
      setForm({
        client_id: '',
        operator_id: '',
        sender_name: '',
        type: 'alphanumeric',
      });
      fetchRegistrations();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка создания');
    } finally {
      setSaving(false);
    }
  };

  const handleStatusUpdate = async () => {
    if (!statusForm.status) {
      toast.error('Выберите статус');
      return;
    }
    setSaving(true);
    try {
      await tarificationApi.updateSenderRegistration(statusForm.id, {
        status: statusForm.status,
        type: statusForm.type,
      });
      toast.success('Статус обновлён');
      setShowStatusEdit(false);
      fetchRegistrations();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка обновления');
    } finally {
      setSaving(false);
    }
  };

  const openStatusModal = (reg: SenderRegistration) => {
    setStatusForm({ id: reg.id, status: reg.status, type: reg.type });
    setShowStatusEdit(true);
  };

  const columns: Column<SenderRegistration>[] = [
    { key: 'sender_name', header: 'Имя отправителя', sortable: true },
    {
      key: 'operator_id',
      header: 'Оператор',
      render: (r) => r.operator_id.slice(0, 8) + '...',
    },
    {
      key: 'client_id',
      header: 'Клиент',
      render: (r) => r.client_id.slice(0, 8) + '...',
      responsive: true,
    },
    { key: 'type', header: 'Тип' },
    {
      key: 'status',
      header: 'Статус',
      render: (r) => <StatusBadge status={r.status} />,
    },
    {
      key: 'created_at',
      header: 'Создано',
      responsive: true,
      render: (r) => new Date(r.created_at).toLocaleDateString('ru-RU'),
    },
  ];

  return (
    <>
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold">Регистрация отправителей</h2>
        <Button
          size="sm"
          onClick={() => {
            setForm({
              client_id: '',
              operator_id: '',
              sender_name: '',
              type: 'alphanumeric',
            });
            setShowCreate(true);
          }}
        >
          Создать регистрацию
        </Button>
      </div>

      <DataTable
        columns={columns}
        data={registrations}
        total={total}
        page={page}
        pageSize={PAGE_SIZE}
        onPageChange={setPage}
        loading={loading}
        keyField="id"
        rowActions={(item) => (
          <Button
            size="sm"
            variant="secondary"
            onClick={() => openStatusModal(item)}
          >
            Изменить статус
          </Button>
        )}
      />

      <Modal
        open={showCreate}
        onClose={() => setShowCreate(false)}
        title="Регистрация отправителя"
      >
        <div className="space-y-4">
          <SearchableSelect
            label="Клиент"
            options={clientOptions}
            value={form.client_id}
            onChange={(v) => setForm({ ...form, client_id: v })}
            placeholder="Выберите клиента..."
            loading={refLoading}
            required
          />
          <SearchableSelect
            label="Оператор"
            options={operatorOptions}
            value={form.operator_id}
            onChange={(v) => setForm({ ...form, operator_id: v })}
            placeholder="Выберите оператора..."
            loading={refLoading}
            required
          />
          <Input
            label="Имя отправителя"
            value={form.sender_name}
            onChange={(e) => setForm({ ...form, sender_name: e.target.value })}
            required
            placeholder="MySender"
          />
          <Select
            label="Тип"
            options={typeOptions}
            value={form.type}
            onChange={(v) => setForm({ ...form, type: v })}
          />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setShowCreate(false)}>
              Отмена
            </Button>
            <Button
              onClick={handleCreate}
              disabled={
                saving ||
                !form.client_id.trim() ||
                !form.operator_id.trim() ||
                !form.sender_name.trim()
              }
            >
              {saving ? 'Создание...' : 'Создать'}
            </Button>
          </div>
        </div>
      </Modal>

      <Modal
        open={showStatusEdit}
        onClose={() => setShowStatusEdit(false)}
        title="Обновить статус"
      >
        <div className="space-y-4">
          <Select
            label="Статус"
            options={statusOptions}
            value={statusForm.status}
            onChange={(v) => setStatusForm({ ...statusForm, status: v })}
          />
          <Select
            label="Тип"
            options={typeOptions}
            value={statusForm.type}
            onChange={(v) => setStatusForm({ ...statusForm, type: v })}
          />
          <div className="flex justify-end gap-3 pt-2">
            <Button
              variant="secondary"
              onClick={() => setShowStatusEdit(false)}
            >
              Отмена
            </Button>
            <Button
              onClick={handleStatusUpdate}
              disabled={saving || !statusForm.status}
            >
              {saving ? 'Сохранение...' : 'Сохранить'}
            </Button>
          </div>
        </div>
      </Modal>
    </>
  );
}
