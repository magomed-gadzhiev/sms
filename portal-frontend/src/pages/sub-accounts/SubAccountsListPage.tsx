import { useState, useEffect, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { subAccountsApi, ApiError } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Modal } from '../../components/ui/Modal';
import { Input } from '../../components/ui/Input';
import { DataTable, type Column } from '../../components/data/DataTable';
import { StatusBadge } from '../../components/ui/Badge';

interface SubAccount {
  id: string;
  name: string;
  email: string;
  active: boolean;
  balance: string;
  daily_limit: number;
  monthly_limit: number;
  messages_today: number;
  messages_this_month: number;
}

interface SubAccountsListResponse {
  sub_accounts: SubAccount[];
  current_count: number;
  max_sub_accounts: number;
}

const columns: Column<SubAccount>[] = [
  {
    key: 'name',
    header: 'Название',
    render: (sa) => (
      <Link to={`/sub-accounts/${sa.id}`} className="text-primary hover:underline">
        {sa.name}
      </Link>
    ),
  },
  { key: 'email', header: 'Email' },
  {
    key: 'active',
    header: 'Активен',
    render: (sa) => <StatusBadge status={sa.active ? 'active' : 'inactive'} />,
  },
  { key: 'balance', header: 'Баланс' },
  { key: 'daily_limit', header: 'Дневной лимит' },
  { key: 'monthly_limit', header: 'Месячный лимит' },
  { key: 'messages_today', header: 'Сегодня' },
  { key: 'messages_this_month', header: 'За месяц' },
];

export function SubAccountsListPage() {
  const [data, setData] = useState<SubAccountsListResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [canCreate, setCanCreate] = useState(true);

  // Create form state
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [creating, setCreating] = useState(false);
  const [formName, setFormName] = useState('');
  const [formEmail, setFormEmail] = useState('');
  const [formContactPerson, setFormContactPerson] = useState('');
  const [formInitialBalance, setFormInitialBalance] = useState('');
  const [formDailyLimit, setFormDailyLimit] = useState('');
  const [formMonthlyLimit, setFormMonthlyLimit] = useState('');

  async function loadSubAccounts() {
    setLoading(true);
    setError('');
    try {
      const resp = await subAccountsApi.list();
      setData(resp as SubAccountsListResponse);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setCanCreate(false);
      }
      setError(err instanceof ApiError ? err.message : 'Не удалось загрузить суб-аккаунты');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    loadSubAccounts();
  }, []);

  async function handleCreate(e: FormEvent) {
    e.preventDefault();
    setCreating(true);
    setError('');
    try {
      await subAccountsApi.create({
        name: formName,
        email: formEmail,
        contact_person: formContactPerson || undefined,
        initial_balance: formInitialBalance || undefined,
        daily_limit: formDailyLimit ? Number(formDailyLimit) : undefined,
        monthly_limit: formMonthlyLimit ? Number(formMonthlyLimit) : undefined,
      });
      setShowCreateForm(false);
      setFormName('');
      setFormEmail('');
      setFormContactPerson('');
      setFormInitialBalance('');
      setFormDailyLimit('');
      setFormMonthlyLimit('');
      await loadSubAccounts();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось создать суб-аккаунт');
    } finally {
      setCreating(false);
    }
  }

  if (loading) return <div role="status">Загрузка суб-аккаунтов...</div>;

  const subAccounts = data?.sub_accounts ?? [];

  return (
    <div className="max-w-5xl">
      <PageHeader
        title="Суб-аккаунты"
        subtitle={data ? `${data.current_count} / ${data.max_sub_accounts}` : undefined}
        actions={canCreate ? <Button onClick={() => setShowCreateForm(true)}>Создать суб-аккаунт</Button> : undefined}
      />

      {error && !canCreate && (
        <div className="bg-amber-50 border border-amber-200 rounded-lg p-6 text-center">
          <p className="text-amber-800 font-medium text-lg mb-2">Суб-аккаунты недоступны</p>
          <p className="text-amber-600 text-sm">
            Для управления суб-аккаунтами необходим тарифный план с поддержкой реселлерских функций.
            Обратитесь к администратору для обновления тарифа.
          </p>
        </div>
      )}

      {error && canCreate && (
        <div role="alert" className="bg-red-50 border border-red-200 text-red-700 rounded p-3 mb-4 text-sm">
          {error}
        </div>
      )}

      {/* Create form modal */}
      <Modal open={showCreateForm} onClose={() => setShowCreateForm(false)} title="Создать суб-аккаунт">
        <form onSubmit={handleCreate} className="flex flex-col gap-4">
          <Input
            label="Название"
            value={formName}
            onChange={(e) => setFormName(e.target.value)}
            required
            placeholder="Название суб-аккаунта"
          />
          <Input
            label="Email"
            type="email"
            value={formEmail}
            onChange={(e) => setFormEmail(e.target.value)}
            required
            placeholder="email@example.com"
          />
          <Input
            label="Контактное лицо"
            value={formContactPerson}
            onChange={(e) => setFormContactPerson(e.target.value)}
            placeholder="Имя контактного лица"
          />
          <Input
            label="Начальный баланс"
            value={formInitialBalance}
            onChange={(e) => setFormInitialBalance(e.target.value)}
            placeholder="0.00"
          />
          <div className="flex gap-4">
            <Input
              label="Дневной лимит"
              type="number"
              value={formDailyLimit}
              onChange={(e) => setFormDailyLimit(e.target.value)}
              placeholder="напр. 1000"
            />
            <Input
              label="Месячный лимит"
              type="number"
              value={formMonthlyLimit}
              onChange={(e) => setFormMonthlyLimit(e.target.value)}
              placeholder="напр. 30000"
            />
          </div>
          <div className="flex gap-2 pt-2">
            <Button type="submit" disabled={creating}>
              {creating ? 'Создание...' : 'Создать'}
            </Button>
            <Button type="button" variant="secondary" onClick={() => setShowCreateForm(false)}>
              Отмена
            </Button>
          </div>
        </form>
      </Modal>

      {/* Sub-accounts table */}
      {canCreate && (
        <DataTable
          columns={columns}
          data={subAccounts}
          total={subAccounts.length}
          page={1}
          pageSize={subAccounts.length}
          onPageChange={() => {}}
          keyField="id"
        />
      )}
    </div>
  );
}
