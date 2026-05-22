import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  companiesApi,
  ApiError,
  type CompanyInfo,
  type CompanyUpsertRequest,
} from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { usePageTitle } from '../../contexts/PageTitleContext';
import { DataTable, type Column } from '../../components/data/DataTable';
import { Button } from '../../components/ui/Button';
import { Modal } from '../../components/ui/Modal';
import { Input } from '../../components/ui/Input';
import { Badge } from '../../components/ui/Badge';

function validateINN(inn: string): string {
  if (inn === '') return '';
  if (!/^\d+$/.test(inn)) return 'ИНН должен содержать только цифры';
  if (inn.length !== 10 && inn.length !== 12) return 'ИНН должен быть 10 или 12 цифр';
  return '';
}

export function CompaniesPage() {
  usePageTitle('Мои компании');
  const navigate = useNavigate();
  const [companies, setCompanies] = useState<CompanyInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [form, setForm] = useState<CompanyUpsertRequest>({ name: '' });
  const [formError, setFormError] = useState('');
  const [creating, setCreating] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const res = await companiesApi.list();
      setCompanies(res.companies ?? []);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Ошибка загрузки');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    const innErr = validateINN(form.inn ?? '');
    if (innErr) { setFormError(innErr); return; }
    if (!form.name.trim()) { setFormError('Наименование обязательно'); return; }
    setCreating(true);
    setFormError('');
    try {
      await companiesApi.create(form);
      setShowCreate(false);
      setForm({ name: '' });
      load();
    } catch (e) {
      setFormError(e instanceof ApiError ? e.message : 'Ошибка создания');
    } finally {
      setCreating(false);
    }
  };

  const handleSetDefault = async (e: React.MouseEvent, id: string) => {
    e.stopPropagation();
    try {
      await companiesApi.setDefault(id);
      load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Ошибка');
    }
  };

  const columns: Column<CompanyInfo>[] = [
    {
      key: 'name',
      header: 'Наименование',
      render: (c) => (
        <div className="flex items-center gap-2">
          <span className="font-medium">{c.name}</span>
          {c.is_offer && <Badge variant="default">Оферта</Badge>}
        </div>
      ),
    },
    {
      key: 'inn',
      header: 'ИНН',
      render: (c) => <span className="font-mono text-sm">{c.inn || '—'}</span>,
    },
    {
      key: 'is_default',
      header: 'Статус',
      render: (c) =>
        c.is_default ? (
          <Badge variant="success">Основная</Badge>
        ) : (
          <Button variant="ghost" size="sm" onClick={(e) => handleSetDefault(e, c.id)}>
            Сделать основной
          </Button>
        ),
    },
    {
      key: 'actions',
      header: '',
      render: (c) => (
        <Button
          variant="ghost"
          size="sm"
          onClick={(e) => { e.stopPropagation(); navigate(`/companies/${c.id}`); }}
        >
          Реквизиты
        </Button>
      ),
    },
  ];

  return (
    <div>
      <PageHeader
        subtitle="Управление юридическими лицами для регистрации имён отправителей"
        actions={
          <Button onClick={() => { setShowCreate(true); setForm({ name: '' }); setFormError(''); }}>
            Добавить компанию
          </Button>
        }
      />

      {error && <div className="mb-4 p-3 bg-red-50 dark:bg-red-950/40 text-red-700 dark:text-red-300 rounded-md text-sm">{error}</div>}

      {!loading && companies.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-center">
          <div className="text-4xl mb-4">🏢</div>
          <h3 className="text-lg font-medium text-gray-900 dark:text-slate-100 mb-2">Нет компаний</h3>
          <p className="text-sm text-gray-500 dark:text-slate-400 mb-6 max-w-sm">
            Добавьте юридическое лицо, чтобы регистрировать имена отправителей от имени компании.
          </p>
          <Button onClick={() => { setShowCreate(true); setForm({ name: '' }); setFormError(''); }}>
            Добавить первую компанию
          </Button>
        </div>
      ) : (
        <DataTable
          columns={columns}
          data={companies}
          loading={loading}
          onRowClick={(c) => navigate(`/companies/${c.id}`)}
          total={companies.length}
          page={1}
          pageSize={companies.length || 20}
          onPageChange={() => {}}
          bulkActions={[]}
        />
      )}

      <Modal open={showCreate} onClose={() => setShowCreate(false)} title="Добавить компанию">
        <form onSubmit={handleCreate} className="space-y-4">
          <Input
            label="ИНН"
            value={form.inn ?? ''}
            onChange={(e) => setForm((f) => ({ ...f, inn: e.target.value }))}
            placeholder="10 или 12 цифр (необязательно)"
            maxLength={12}
          />
          <Input
            label="Краткое наименование *"
            value={form.name}
            onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            placeholder="ООО Ромашка"
          />
          {formError && <p className="text-sm text-red-600 dark:text-red-400">{formError}</p>}
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={() => setShowCreate(false)}>
              Отмена
            </Button>
            <Button type="submit" disabled={creating}>
              {creating ? 'Создание...' : 'Создать'}
            </Button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
