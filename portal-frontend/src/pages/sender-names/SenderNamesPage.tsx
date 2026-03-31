import { useState, useEffect, useCallback, type FormEvent } from 'react';
import { senderNamesApi, ApiError, type SenderNameInfo, type SenderNameHistoryEntry } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Modal } from '../../components/ui/Modal';
import { Input } from '../../components/ui/Input';
import { DataTable, type Column } from '../../components/data/DataTable';
import { Badge } from '../../components/ui/Badge';

const PAGE_SIZE = 20;

const STATUS_BADGE: Record<string, { variant: 'warning' | 'success' | 'danger' | 'default'; label: string }> = {
  pending: { variant: 'warning', label: 'На модерации' },
  approved: { variant: 'success', label: 'Одобрено' },
  rejected: { variant: 'danger', label: 'Отклонено' },
  deactivated: { variant: 'default', label: 'Деактивировано' },
};

const ALPHANUMERIC_RE = /^[A-Za-z0-9 ]{1,11}$/;
const NUMERIC_RE = /^\d{1,15}$/;

function validateName(name: string): string {
  if (!name.trim()) return 'Имя обязательно';
  if (NUMERIC_RE.test(name)) return '';
  if (ALPHANUMERIC_RE.test(name) && name.trim() !== '') return '';
  return 'Имя должно быть 1–11 латинских букв/цифр или 1–15 цифр';
}

function formatDate(dt: string) {
  return new Date(dt).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });
}

export function SenderNamesPage() {
  const [items, setItems] = useState<SenderNameInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Create modal
  const [showCreate, setShowCreate] = useState(false);
  const [createName, setCreateName] = useState('');
  const [createError, setCreateError] = useState('');
  const [creating, setCreating] = useState(false);

  // Detail / edit panel
  const [selected, setSelected] = useState<SenderNameInfo | null>(null);
  const [editName, setEditName] = useState('');
  const [editError, setEditError] = useState('');
  const [editing, setEditing] = useState(false);
  const [showEdit, setShowEdit] = useState(false);

  // History
  const [history, setHistory] = useState<SenderNameHistoryEntry[]>([]);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [showHistory, setShowHistory] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const res = await senderNamesApi.list({ page: String(page), per_page: String(PAGE_SIZE) });
      setItems(res.sender_names ?? []);
      setTotal(res.total ?? 0);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Ошибка загрузки');
    } finally {
      setLoading(false);
    }
  }, [page]);

  useEffect(() => { load(); }, [load]);

  const handleCreate = async (e: FormEvent) => {
    e.preventDefault();
    const err = validateName(createName);
    if (err) { setCreateError(err); return; }
    setCreating(true);
    setCreateError('');
    try {
      await senderNamesApi.create(createName.trim());
      setShowCreate(false);
      setCreateName('');
      load();
    } catch (e) {
      setCreateError(e instanceof ApiError ? e.message : 'Ошибка создания');
    } finally {
      setCreating(false);
    }
  };

  const openDetail = async (sn: SenderNameInfo) => {
    setSelected(sn);
    setEditName(sn.name);
    setEditError('');
    setShowEdit(false);
    setHistory([]);
    setShowHistory(false);
  };

  const handleUpdate = async (e: FormEvent) => {
    e.preventDefault();
    if (!selected) return;
    const err = validateName(editName);
    if (err) { setEditError(err); return; }
    setEditing(true);
    setEditError('');
    try {
      await senderNamesApi.update(selected.id, editName.trim());
      setShowEdit(false);
      load();
      setSelected(null);
    } catch (e) {
      setEditError(e instanceof ApiError ? e.message : 'Ошибка обновления');
    } finally {
      setEditing(false);
    }
  };

  const handleResubmit = async () => {
    if (!selected) return;
    setEditing(true);
    try {
      await senderNamesApi.resubmit(selected.id);
      setShowEdit(false);
      setSelected(null);
      load();
    } catch (e) {
      setEditError(e instanceof ApiError ? e.message : 'Ошибка повторной отправки');
    } finally {
      setEditing(false);
    }
  };

  const loadHistory = async (id: string) => {
    setHistoryLoading(true);
    try {
      const res = await senderNamesApi.getHistory(id);
      setHistory(res.entries ?? []);
      setShowHistory(true);
    } catch {
      // ignore
    } finally {
      setHistoryLoading(false);
    }
  };

  const columns: Column<SenderNameInfo>[] = [
    { key: 'name', header: 'Имя отправителя', render: (sn) => <span className="font-mono font-medium">{sn.name}</span> },
    {
      key: 'status', header: 'Статус', render: (sn) => {
        const s = STATUS_BADGE[sn.status] ?? { variant: 'default' as const, label: sn.status };
        return <Badge variant={s.variant}>{s.label}</Badge>;
      },
    },
    { key: 'created_at', header: 'Создано', render: (sn) => formatDate(sn.created_at) },
    {
      key: 'actions', header: '', render: (sn) => (
        <Button variant="ghost" size="sm" onClick={(e) => { e.stopPropagation(); openDetail(sn); }}>
          Подробнее
        </Button>
      ),
    },
  ];

  return (
    <div>
      <PageHeader
        title="Имена отправителей"
        subtitle="Управление зарегистрированными именами отправителей SMS"
        actions={<Button onClick={() => { setShowCreate(true); setCreateName(''); setCreateError(''); }}>Зарегистрировать имя</Button>}
      />

      {error && <div className="mb-4 p-3 bg-red-50 text-red-700 rounded-md text-sm">{error}</div>}

      <DataTable
        columns={columns}
        data={items}
        loading={loading}
        onRowClick={openDetail}
        total={total}
        page={page}
        pageSize={PAGE_SIZE}
        onPageChange={setPage}
      />

      {/* Create modal */}
      <Modal open={showCreate} onClose={() => setShowCreate(false)} title="Зарегистрировать имя отправителя">
        <form onSubmit={handleCreate} className="space-y-4">
          <div>
            <Input
              label="Имя отправителя"
              value={createName}
              onChange={(e) => { setCreateName(e.target.value); setCreateError(''); }}
              placeholder="Например: MyBrand или 79001234567"
              maxLength={15}
            />
            <p className="mt-1 text-xs text-gray-500">
              1–11 латинских букв/цифр/пробелов или 1–15 цифр
            </p>
            {createError && <p className="mt-1 text-sm text-red-600">{createError}</p>}
          </div>
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={() => setShowCreate(false)}>Отмена</Button>
            <Button type="submit" disabled={creating}>Зарегистрировать</Button>
          </div>
        </form>
      </Modal>

      {/* Detail modal */}
      {selected && (
        <Modal open={!!selected} onClose={() => setSelected(null)} title={`Имя: ${selected.name}`}>
          <div className="space-y-4">
            <div className="flex items-center gap-2">
              <span className="text-sm text-gray-500">Статус:</span>
              {(() => {
                const s = STATUS_BADGE[selected.status] ?? { variant: 'default' as const, label: selected.status };
                return <Badge variant={s.variant}>{s.label}</Badge>;
              })()}
            </div>

            {selected.status === 'rejected' && selected.rejection_reason && (
              <div className="p-3 bg-red-50 border border-red-200 rounded-md">
                <p className="text-sm font-medium text-red-800">Причина отклонения</p>
                <p className="text-sm text-red-700 mt-1">{selected.rejection_reason}</p>
              </div>
            )}

            {selected.status === 'rejected' && (
              <div className="border-t pt-4">
                {!showEdit ? (
                  <Button variant="secondary" onClick={() => setShowEdit(true)}>Редактировать и повторить</Button>
                ) : (
                  <form onSubmit={handleUpdate} className="space-y-3">
                    <Input
                      label="Новое имя"
                      value={editName}
                      onChange={(e) => { setEditName(e.target.value); setEditError(''); }}
                      maxLength={15}
                    />
                    {editError && <p className="text-sm text-red-600">{editError}</p>}
                    <div className="flex gap-2">
                      <Button type="submit" disabled={editing}>Сохранить</Button>
                      <Button type="button" variant="secondary" onClick={handleResubmit} disabled={editing}>
                        Отправить на рассмотрение
                      </Button>
                      <Button type="button" variant="ghost" onClick={() => setShowEdit(false)}>Отмена</Button>
                    </div>
                  </form>
                )}
              </div>
            )}

            <div className="border-t pt-4">
              {!showHistory ? (
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => loadHistory(selected.id)}
                  disabled={historyLoading}
                >
                  Показать историю статусов
                </Button>
              ) : (
                <div>
                  <p className="text-sm font-medium text-gray-700 mb-2">История статусов</p>
                  <div className="space-y-2">
                    {history.map((e) => (
                      <div key={e.id} className="flex items-start gap-2 text-sm">
                        <span className="text-gray-400 shrink-0">{formatDate(e.created_at)}</span>
                        <span>
                          {e.old_status ? `${e.old_status} → ` : ''}<strong>{e.new_status}</strong>
                          {e.comment && <span className="text-gray-500 ml-1">({e.comment})</span>}
                          <span className="text-xs text-gray-400 ml-1">({e.actor_type})</span>
                        </span>
                      </div>
                    ))}
                    {history.length === 0 && <p className="text-sm text-gray-400">История пуста</p>}
                  </div>
                </div>
              )}
            </div>

            <div className="flex justify-end">
              <Button variant="ghost" onClick={() => setSelected(null)}>Закрыть</Button>
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
}
