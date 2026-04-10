import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../../components/layout/PageHeader';
import { DataTable, type Column } from '../../../components/data/DataTable';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { Modal } from '../../../components/ui/Modal';
import { ConfirmDialog } from '../../../components/ui/ConfirmDialog';
import { Badge } from '../../../components/ui/Badge';
import { useToast } from '../../../components/ui/Toast';
import { legalEntitiesApi, type LegalEntity } from '../../../api/admin';

interface FormState {
  inn: string;
  name: string;
  full_name: string;
  address: string;
}

const EMPTY_FORM: FormState = { inn: '', name: '', full_name: '', address: '' };

// ── INN Validation ──

function validateINN(inn: string): boolean {
  if (!/^\d{10}$|^\d{12}$/.test(inn)) return false;
  const d = inn.split('').map(Number);
  if (d.length === 10) {
    const check = ([2, 4, 10, 3, 5, 9, 4, 6, 8].reduce((s, w, i) => s + w * d[i], 0) % 11) % 10;
    return check === d[9];
  }
  // 12-digit
  const check1 = ([7, 2, 4, 10, 3, 5, 9, 4, 6, 8].reduce((s, w, i) => s + w * d[i], 0) % 11) % 10;
  const check2 = ([3, 7, 2, 4, 10, 3, 5, 9, 4, 6, 8].reduce((s, w, i) => s + w * d[i], 0) % 11) % 10;
  return check1 === d[10] && check2 === d[11];
}

export function LegalEntitiesPage() {
  const toast = useToast();
  const [items, setItems] = useState<LegalEntity[]>([]);
  const [loading, setLoading] = useState(false);
  const [showForm, setShowForm] = useState(false);
  const [editItem, setEditItem] = useState<LegalEntity | null>(null);
  const [deleteItem, setDeleteItem] = useState<LegalEntity | null>(null);
  const [form, setForm] = useState<FormState>(EMPTY_FORM);
  const [saving, setSaving] = useState(false);
  const [innError, setInnError] = useState('');

  const fetchItems = useCallback(async () => {
    setLoading(true);
    try {
      const res = await legalEntitiesApi.list({ limit: 200 });
      setItems(res.legal_entities || []);
    } catch {
      toast.error('Не удалось загрузить юридические лица');
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => { fetchItems(); }, [fetchItems]);

  function openCreate() {
    setEditItem(null);
    setForm(EMPTY_FORM);
    setInnError('');
    setShowForm(true);
  }

  function openEdit(item: LegalEntity) {
    setEditItem(item);
    setForm({
      inn: item.inn,
      name: item.name,
      full_name: item.full_name || '',
      address: item.address || '',
    });
    setInnError('');
    setShowForm(true);
  }

  async function handleSave() {
    if (!form.inn.trim() || !form.name.trim()) {
      toast.error('ИНН и название обязательны');
      return;
    }
    if (form.inn && !validateINN(form.inn)) {
      setInnError('Некорректный ИНН');
      return;
    }
    setSaving(true);
    try {
      if (editItem) {
        await legalEntitiesApi.update(editItem.id, form);
        toast.success('Юридическое лицо обновлено');
      } else {
        await legalEntitiesApi.create(form);
        toast.success('Юридическое лицо создано');
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
      await legalEntitiesApi.delete(deleteItem.id);
      toast.success('Удалено');
      setDeleteItem(null);
      fetchItems();
    } catch {
      toast.error('Не удалось удалить');
    }
  }

  const columns: Column<LegalEntity>[] = [
    {
      key: 'inn',
      header: 'ИНН',
      render: (row) => <span className="font-mono text-sm">{row.inn}</span>,
    },
    { key: 'name', header: 'Краткое название' },
    {
      key: 'full_name',
      header: 'Полное название',
      render: (row) => <span className="text-sm text-gray-600">{row.full_name || '—'}</span>,
    },
    {
      key: 'address',
      header: 'Адрес',
      render: (row) => <span className="text-sm text-gray-600">{row.address || '—'}</span>,
    },
    {
      key: 'operators',
      header: 'Операторы',
      render: (row) => {
        const operators = (row as any).operators;
        return <span className="text-sm text-gray-600">{operators?.join(', ') ?? '—'}</span>;
      },
    },
    {
      key: 'active',
      header: 'Статус',
      render: (row) => (
        <Badge variant={row.active ? 'success' : 'default'}>
          {row.active ? 'Активно' : 'Неактивно'}
        </Badge>
      ),
    },
  ];

  return (
    <>
      <PageHeader
        title="Юридические лица"
        breadcrumbs={[{ label: 'Админ', href: '/admin/dashboard' }, { label: 'Юридические лица' }]}
        actions={<Button onClick={openCreate}>+ Добавить</Button>}
      />

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
        title={editItem ? 'Изменить юридическое лицо' : 'Добавить юридическое лицо'}
      >
        <div className="space-y-4">
          <Input
            label="ИНН *"
            value={form.inn}
            onChange={(e) => {
              setForm((f) => ({ ...f, inn: e.target.value }));
              if (e.target.value && !validateINN(e.target.value)) {
                setInnError('Некорректный ИНН');
              } else {
                setInnError('');
              }
            }}
            placeholder="1234567890"
            maxLength={12}
          />
          {innError && <p className="text-xs text-red-500 mt-1">{innError}</p>}
          <Input
            label="Краткое название *"
            value={form.name}
            onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            placeholder="ООО Ромашка"
          />
          <Input
            label="Полное название"
            value={form.full_name}
            onChange={(e) => setForm((f) => ({ ...f, full_name: e.target.value }))}
            placeholder="Общество с ограниченной ответственностью «Ромашка»"
          />
          <Input
            label="Адрес"
            value={form.address}
            onChange={(e) => setForm((f) => ({ ...f, address: e.target.value }))}
            placeholder="г. Москва, ул. Ленина, д. 1"
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
        title="Удалить юридическое лицо?"
        description={`Юридическое лицо «${deleteItem?.name}» будет удалено. Это действие нельзя отменить.`}
        confirmLabel="Удалить"
        variant="danger"
      />
    </>
  );
}
