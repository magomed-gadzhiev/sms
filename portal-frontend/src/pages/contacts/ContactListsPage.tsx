import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { contactListsApi, type ContactList } from '../../api/contacts';
import { ApiError } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Modal } from '../../components/ui/Modal';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { DataTable, type Column } from '../../components/data/DataTable';
import { useToast } from '../../components/ui/Toast';

export function ContactListsPage() {
  const navigate = useNavigate();
  const toast = useToast();
  const [lists, setLists] = useState<ContactList[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Create modal state
  const [showCreate, setShowCreate] = useState(false);
  const [createName, setCreateName] = useState('');
  const [createDesc, setCreateDesc] = useState('');
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState('');

  // Delete state
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

  const perPage = 20;

  async function load() {
    setLoading(true);
    setError('');
    try {
      const resp = await contactListsApi.list(page, perPage);
      setLists(resp.items ?? []);
      setTotal(resp.total ?? 0);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось загрузить списки');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, [page]);

  async function handleCreate() {
    if (!createName.trim()) return;
    setCreating(true);
    setCreateError('');
    try {
      await contactListsApi.create({
        name: createName.trim(),
        description: createDesc.trim() || undefined,
      });
      setShowCreate(false);
      setCreateName('');
      setCreateDesc('');
      toast.success('Контактная база создана');
      load();
    } catch (err) {
      setCreateError(err instanceof ApiError ? err.message : 'Ошибка при создании');
    } finally {
      setCreating(false);
    }
  }

  async function confirmDelete() {
    if (!deleteId) return;
    setDeleting(true);
    try {
      await contactListsApi.remove(deleteId);
      setLists((prev) => prev.filter((l) => l.id !== deleteId));
      setTotal((prev) => prev - 1);
      setDeleteId(null);
      toast.success('Контактная база удалена');
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка при удалении');
    } finally {
      setDeleting(false);
    }
  }

  const columns: Column<ContactList>[] = [
    {
      key: 'name',
      header: 'Название',
      render: (item) => (
        <span className="font-medium text-gray-900">{item.name}</span>
      ),
    },
    {
      key: 'description',
      header: 'Описание',
      responsive: true,
      render: (item) => (
        <span className="text-gray-500">{item.description || '—'}</span>
      ),
    },
    {
      key: 'contacts_count',
      header: 'Контактов',
      render: (item) => (
        <span className="font-mono text-sm">{(item.contacts_count ?? 0).toLocaleString()}</span>
      ),
    },
    {
      key: 'created_at',
      header: 'Дата создания',
      responsive: true,
      render: (item) => (
        <span className="text-gray-500 text-sm">
          {(() => {
            const d = item.created_at;
            if (!d) return '—';
            // Handle protobuf Timestamp {seconds, nanos}
            if (typeof d === 'object' && 'seconds' in (d as any)) {
              return new Date((d as any).seconds * 1000).toLocaleDateString('ru-RU');
            }
            return new Date(d).toLocaleDateString('ru-RU');
          })()}
        </span>
      ),
    },
  ];

  return (
    <div>
      <PageHeader
        title="Контактные базы"
        subtitle="Управление списками контактов для рассылок"
        actions={
          <Button onClick={() => setShowCreate(true)}>+ Создать базу</Button>
        }
      />

      {error && <p className="text-red-600 mb-4">{error}</p>}

      {!loading && !error && lists.length === 0 && (
        <div className="text-center py-12 text-gray-500">
          <p className="mb-2">Контактные базы не созданы</p>
          <p className="text-sm mb-4">
            Создайте первую базу контактов для отправки рассылок
          </p>
          <Button variant="ghost" onClick={() => setShowCreate(true)}>
            Создать базу
          </Button>
        </div>
      )}

      {(loading || lists.length > 0) && (
        <DataTable
          columns={columns}
          data={lists}
          total={total}
          page={page}
          pageSize={perPage}
          onPageChange={setPage}
          loading={loading}
          keyField="id"
          onRowClick={(item) => navigate(`/contact-lists/${item.id}`)}
          rowActions={(item) => (
            <div className="flex gap-2">
              <Button
                variant="secondary"
                size="sm"
                onClick={() => navigate(`/contact-lists/${item.id}`)}
              >
                Открыть
              </Button>
              <Button
                variant="danger"
                size="sm"
                onClick={() => setDeleteId(item.id)}
              >
                Удалить
              </Button>
            </div>
          )}
        />
      )}

      {/* Create modal */}
      <Modal
        open={showCreate}
        onClose={() => setShowCreate(false)}
        title="Создать контактную базу"
      >
        <div className="space-y-4">
          {createError && (
            <p role="alert" className="text-sm text-danger bg-red-50 border border-red-200 rounded p-3">
              {createError}
            </p>
          )}
          <Input
            label="Название *"
            type="text"
            value={createName}
            onChange={(e) => setCreateName(e.target.value)}
            placeholder="Например: Клиенты Москва"
            autoFocus
            required
          />
          <div className="flex flex-col gap-1">
            <label htmlFor="create-desc" className="text-sm font-medium text-gray-700">
              Описание
            </label>
            <textarea
              id="create-desc"
              value={createDesc}
              onChange={(e) => setCreateDesc(e.target.value)}
              className="rounded border border-gray-300 px-3 py-2 text-sm transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50 focus-visible:border-primary"
              rows={3}
              placeholder="Необязательное описание списка"
            />
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <Button
              variant="secondary"
              onClick={() => setShowCreate(false)}
              disabled={creating}
            >
              Отмена
            </Button>
            <Button
              onClick={handleCreate}
              disabled={creating || !createName.trim()}
            >
              {creating ? 'Создание...' : 'Создать'}
            </Button>
          </div>
        </div>
      </Modal>

      {/* Delete confirm */}
      <ConfirmDialog
        open={deleteId !== null}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteId(null)}
        title="Удалить контактную базу"
        description="Вы уверены, что хотите удалить эту базу? Все контакты будут потеряны. Это действие нельзя отменить."
        confirmLabel="Удалить"
        variant="danger"
        loading={deleting}
      />
    </div>
  );
}
