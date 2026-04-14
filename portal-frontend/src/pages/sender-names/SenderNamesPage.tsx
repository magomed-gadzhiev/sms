import { useState, useEffect, useCallback, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { senderNamesApi, companiesApi, ApiError, type SenderNameInfo, type CompanyInfo } from '../../api/client';
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
  const navigate = useNavigate();

  const [items, setItems] = useState<SenderNameInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Companies
  const [companies, setCompanies] = useState<CompanyInfo[]>([]);
  const [selectedCompanyId, setSelectedCompanyId] = useState('');

  // Create modal
  const [showCreate, setShowCreate] = useState(false);
  const [createName, setCreateName] = useState('');
  const [createError, setCreateError] = useState('');
  const [creating, setCreating] = useState(false);

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

  useEffect(() => {
    companiesApi.list().then((res) => {
      const list = res.companies ?? [];
      setCompanies(list);
      const def = list.find((c) => c.is_default);
      if (def) setSelectedCompanyId(def.id);
    }).catch(() => {});
  }, []);

  const handleCreate = async (e: FormEvent) => {
    e.preventDefault();
    const nameErr = validateName(createName);
    if (nameErr) { setCreateError(nameErr); return; }
    setCreating(true);
    setCreateError('');
    try {
      await senderNamesApi.create(createName.trim(), selectedCompanyId || undefined);
      setShowCreate(false);
      setCreateName('');
      load();
    } catch (e) {
      setCreateError(e instanceof ApiError ? e.message : 'Ошибка создания');
    } finally {
      setCreating(false);
    }
  };

  function nextBillingDate() {
    const now = new Date();
    return new Date(now.getFullYear(), now.getMonth() + 1, 1).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });
  }

  const columns: Column<SenderNameInfo>[] = [
    { key: 'name', header: 'Имя отправителя', render: (sn) => <span className="font-mono font-medium">{sn.name}</span> },
    {
      key: 'status', header: 'Статус', render: (sn) => {
        const s = STATUS_BADGE[sn.status] ?? { variant: 'default' as const, label: sn.status };
        return <Badge variant={s.variant}>{s.label}</Badge>;
      },
    },
    { key: 'next_billing', header: 'Следующее начисление', render: () => <span className="text-xs text-gray-400">{nextBillingDate()}</span> },
    { key: 'created_at', header: 'Создано', render: (sn) => formatDate(sn.created_at) },
    {
      key: 'actions', header: '', render: (sn) => (
        <Button variant="ghost" size="sm" onClick={(e) => { e.stopPropagation(); navigate(`/sender-names/${sn.id}`); }}>
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
        onRowClick={(sn) => navigate(`/sender-names/${sn.id}`)}
        total={total}
        page={page}
        pageSize={PAGE_SIZE}
        onPageChange={setPage}
        bulkActions={[]}
      />

      {/* Create modal */}
      <Modal open={showCreate} onClose={() => setShowCreate(false)} title="Зарегистрировать имя отправителя">
        <form onSubmit={handleCreate} className="space-y-4">
          {companies.length > 0 && (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Компания</label>
              <select
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                value={selectedCompanyId}
                onChange={(e) => setSelectedCompanyId(e.target.value)}
              >
                {companies.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}{c.is_offer ? ' (Оферта)' : ''}
                  </option>
                ))}
              </select>
            </div>
          )}
          <div>
            <Input
              label="Имя отправителя"
              value={createName}
              onChange={(e) => { setCreateName(e.target.value); setCreateError(''); }}
              placeholder="Например: MyBrand или 79001234567"
              maxLength={15}
            />
            <p id="sender-name-hint" className="mt-1 text-xs text-gray-500">
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
    </div>
  );
}
