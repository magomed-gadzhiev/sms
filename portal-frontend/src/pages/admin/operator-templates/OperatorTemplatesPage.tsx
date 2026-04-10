import { useState, useEffect, useCallback } from 'react';
import { useSearchParams } from 'react-router-dom';
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
  operatorTemplatesApi,
  operatorsApi,
  senderNamesApi,
  type OperatorTemplate,
  type OperatorInfo,
  type SenderNameInfo,
} from '../../../api/admin';

const STATUS_LABEL: Record<string, string> = {
  active: 'Активен',
  inactive: 'Неактивен',
  pending: 'На проверке',
};

const STATUS_VARIANT: Record<string, 'success' | 'danger' | 'warning' | 'default'> = {
  active: 'success',
  inactive: 'default',
  pending: 'warning',
};

const STATUS_OPTIONS = [
  { value: 'active', label: 'Активен' },
  { value: 'inactive', label: 'Неактивен' },
  { value: 'pending', label: 'На проверке' },
];

const VARIABLE_HINTS = ['{name}', '{code}', '{amount}', '{date}', '{link}', '{order_id}'];

interface FormState {
  name: string;
  operator_id: string;
  sender_name_id: string;
  body: string;
  variables: string;
  status: string;
}

const EMPTY_FORM: FormState = {
  name: '',
  operator_id: '',
  sender_name_id: '',
  body: '',
  variables: '',
  status: 'active',
};

export function OperatorTemplatesPage() {
  const toast = useToast();
  const [searchParams] = useSearchParams();
  const [items, setItems] = useState<OperatorTemplate[]>([]);
  const [loading, setLoading] = useState(false);
  const [operators, setOperators] = useState<OperatorInfo[]>([]);
  const [senderNames, setSenderNames] = useState<SenderNameInfo[]>([]);
  const [filterOperatorId, setFilterOperatorId] = useState('');
  const [filterStatus, setFilterStatus] = useState('');
  const [showForm, setShowForm] = useState(false);
  const [editItem, setEditItem] = useState<OperatorTemplate | null>(null);
  const [deleteItem, setDeleteItem] = useState<OperatorTemplate | null>(null);
  const [form, setForm] = useState<FormState>(EMPTY_FORM);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    const senderNameId = searchParams.get('sender_name_id');
    if (senderNameId) {
      setForm(f => ({ ...f, sender_name_id: senderNameId }));
      setShowForm(true);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const fetchItems = useCallback(async () => {
    setLoading(true);
    try {
      const res = await operatorTemplatesApi.list({
        operator_id: filterOperatorId,
        status: filterStatus,
        limit: 200,
      });
      setItems(res.operator_templates || []);
    } catch {
      toast.error('Не удалось загрузить шаблоны операторов');
    } finally {
      setLoading(false);
    }
  }, [filterOperatorId, filterStatus, toast]);

  useEffect(() => {
    operatorsApi.list({ limit: 500 }).then((r) => setOperators(r.operators || [])).catch(() => {});
    senderNamesApi.list({ limit: 500 }).then((r) => setSenderNames(r.sender_names || [])).catch(() => {});
  }, []);

  useEffect(() => { fetchItems(); }, [fetchItems]);

  const operatorFilterOptions = [
    { value: '', label: 'Все операторы' },
    ...operators.map((o) => ({ value: o.operator_id, label: o.name })),
  ];

  const operatorFormOptions = [
    { value: '', label: 'Выберите оператора' },
    ...operators.map((o) => ({ value: o.operator_id, label: o.name })),
  ];

  const senderNameOptions = [
    { value: '', label: 'Без имени отправителя' },
    ...senderNames.map((sn) => ({ value: sn.sender_name_id, label: sn.name })),
  ];

  const filterStatusOptions = [
    { value: '', label: 'Все статусы' },
    ...STATUS_OPTIONS,
  ];

  function openCreate() {
    setEditItem(null);
    setForm(EMPTY_FORM);
    setShowForm(true);
  }

  function openEdit(item: OperatorTemplate) {
    setEditItem(item);
    setForm({
      name: item.name,
      operator_id: item.operator_id,
      sender_name_id: item.sender_name_id || '',
      body: item.body,
      variables: (item.variables || []).join(', '),
      status: item.status,
    });
    setShowForm(true);
  }

  function insertVariable(variable: string) {
    setForm((f) => ({ ...f, body: f.body + variable }));
  }

  async function handleSave() {
    if (!form.name.trim() || !form.operator_id || !form.body.trim()) {
      toast.error('Название, оператор и текст шаблона обязательны');
      return;
    }
    const variables = form.variables
      .split(',')
      .map((v) => v.trim())
      .filter(Boolean);

    setSaving(true);
    try {
      const payload = {
        name: form.name,
        operator_id: form.operator_id,
        sender_name_id: form.sender_name_id,
        body: form.body,
        variables,
        status: form.status,
      };
      if (editItem) {
        await operatorTemplatesApi.update(editItem.id, payload);
        toast.success('Шаблон обновлён');
      } else {
        await operatorTemplatesApi.create(payload);
        toast.success('Шаблон создан');
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
      await operatorTemplatesApi.delete(deleteItem.id);
      toast.success('Шаблон удалён');
      setDeleteItem(null);
      fetchItems();
    } catch {
      toast.error('Не удалось удалить');
    }
  }

  const columns: Column<OperatorTemplate>[] = [
    {
      key: 'name',
      header: 'Название',
      render: (row) => <span className="font-medium text-sm">{row.name}</span>,
    },
    {
      key: 'operator_name',
      header: 'Оператор',
      render: (row) => <span className="text-sm">{row.operator_name || '—'}</span>,
    },
    {
      key: 'sender_name',
      header: 'Имя отправителя',
      render: (row) => <span className="text-sm font-mono">{row.sender_name || '—'}</span>,
    },
    {
      key: 'body',
      header: 'Текст',
      render: (row) => (
        <span className="text-xs text-gray-600 truncate max-w-[250px] block" title={row.body}>
          {row.body}
        </span>
      ),
    },
    {
      key: 'variables',
      header: 'Переменные',
      render: (row) =>
        row.variables && row.variables.length > 0 ? (
          <div className="flex flex-wrap gap-1">
            {row.variables.map((v) => (
              <span key={v} className="px-1.5 py-0.5 bg-gray-100 rounded text-xs font-mono">{v}</span>
            ))}
          </div>
        ) : (
          <span className="text-gray-400 text-xs">—</span>
        ),
    },
    {
      key: 'status',
      header: 'Статус',
      render: (row) => (
        <Badge variant={STATUS_VARIANT[row.status] ?? 'default'}>
          {STATUS_LABEL[row.status] ?? row.status}
        </Badge>
      ),
    },
  ];

  return (
    <>
      <PageHeader
        title="Шаблоны операторов"
        breadcrumbs={[{ label: 'Админ', href: '/admin/dashboard' }, { label: 'Шаблоны операторов' }]}
        actions={<Button onClick={openCreate}>+ Добавить</Button>}
      />

      <div className="bg-white border border-gray-200 rounded-lg p-4 mb-4">
        <div className="flex gap-3 flex-wrap">
          <div className="w-48">
            <Select
              label="Оператор"
              value={filterOperatorId}
              onChange={setFilterOperatorId}
              options={operatorFilterOptions}
            />
          </div>
          <div className="w-40">
            <Select
              label="Статус"
              value={filterStatus}
              onChange={setFilterStatus}
              options={filterStatusOptions}
            />
          </div>
        </div>
      </div>

      <div className="bg-white border border-gray-200 rounded-lg overflow-hidden">
        <DataTable
          columns={columns}
          data={items}
          total={items.length}
          page={1}
          pageSize={items.length || 1}
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
        title={editItem ? 'Изменить шаблон' : 'Добавить шаблон оператора'}
      >
        <div className="space-y-4">
          <Input
            label="Название *"
            value={form.name}
            onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            placeholder="Шаблон верификации МТС"
          />
          <Select
            label="Оператор *"
            value={form.operator_id}
            onChange={(v) => setForm((f) => ({ ...f, operator_id: v }))}
            options={operatorFormOptions}
          />
          <Select
            label="Имя отправителя"
            value={form.sender_name_id}
            onChange={(v) => setForm((f) => ({ ...f, sender_name_id: v }))}
            options={senderNameOptions}
          />
          <Select
            label="Статус"
            value={form.status}
            onChange={(v) => setForm((f) => ({ ...f, status: v }))}
            options={STATUS_OPTIONS}
          />
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Текст шаблона *</label>
            <div className="mb-2 flex flex-wrap gap-1">
              <span className="text-xs text-gray-500 mr-1">Вставить переменную:</span>
              {VARIABLE_HINTS.map((v) => (
                <button
                  key={v}
                  type="button"
                  onClick={() => insertVariable(v)}
                  className="px-2 py-0.5 text-xs bg-blue-50 hover:bg-blue-100 text-blue-700 rounded border border-blue-200 font-mono transition-colors"
                >
                  {v}
                </button>
              ))}
            </div>
            <textarea
              className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary/50 font-mono"
              rows={5}
              value={form.body}
              onChange={(e) => setForm((f) => ({ ...f, body: e.target.value }))}
              placeholder="Ваш код: {code}. Не сообщайте его никому."
            />
            {form.body && (
              <div className="mt-2 p-3 bg-gray-50 rounded-lg border text-sm">
                <p className="text-xs text-gray-500 mb-1 font-medium">Предпросмотр с выделенными переменными:</p>
                <p className="leading-relaxed break-words">
                  {form.body.split(/(\{[^}]+\})/g).map((part, i) =>
                    /^\{[^}]+\}$/.test(part) ? (
                      <span key={i} className="text-indigo-600 bg-indigo-50 px-1 rounded font-medium">{part}</span>
                    ) : (
                      <span key={i}>{part}</span>
                    )
                  )}
                </p>
              </div>
            )}
            {form.body && (
              <div className="mt-2">
                <p className="text-xs text-gray-500 mb-1 font-medium">Предпросмотр с примером значений:</p>
                <div className="bg-white border border-gray-200 rounded-xl p-4 shadow-sm max-w-xs">
                  <p className="text-xs text-gray-400 font-medium mb-1">
                    {senderNames.find(sn => sn.sender_name_id === form.sender_name_id)?.name || 'SENDER'}
                  </p>
                  <p className="text-sm text-gray-800 leading-relaxed">
                    {form.body
                      .replace(/\{code\}/g, '1234')
                      .replace(/\{name\}/g, 'Иван')
                      .replace(/\{date\}/g, '11.04.2026')
                      .replace(/\{sum\}/g, '1 500 ₽')
                      .replace(/\{amount\}/g, '1 500 ₽')
                      .replace(/\{link\}/g, 'https://example.com/abc')
                      .replace(/\{order_id\}/g, 'ORD-9821')}
                  </p>
                </div>
              </div>
            )}
          </div>
          <Input
            label="Переменные (через запятую)"
            value={form.variables}
            onChange={(e) => setForm((f) => ({ ...f, variables: e.target.value }))}
            placeholder="{code}, {name}, {amount}"
          />
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
        title="Удалить шаблон?"
        description={`Шаблон «${deleteItem?.name}» будет удалён. Это действие нельзя отменить.`}
        confirmLabel="Удалить"
        variant="danger"
      />
    </>
  );
}
