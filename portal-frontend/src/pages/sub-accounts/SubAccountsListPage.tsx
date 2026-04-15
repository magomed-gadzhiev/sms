import { useState, useEffect, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { subAccountsApi, billingApi, ApiError } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Modal } from '../../components/ui/Modal';
import { Input } from '../../components/ui/Input';
import { DataTable, type Column } from '../../components/data/DataTable';
import { StatusBadge } from '../../components/ui/Badge';
import { useToast } from '../../components/ui/Toast';

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
  {
    key: 'balance',
    header: 'Баланс',
    render: (sa) => `${parseFloat(sa.balance).toFixed(2)} ₽`,
  },
  { key: 'daily_limit', header: 'Дневной лимит' },
  { key: 'monthly_limit', header: 'Месячный лимит' },
  { key: 'messages_today', header: 'Сегодня' },
  { key: 'messages_this_month', header: 'За месяц' },
];

export function SubAccountsListPage() {
  const toast = useToast();
  const [data, setData] = useState<SubAccountsListResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [canCreate, setCanCreate] = useState(true);

  // Create form state
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [creating, setCreating] = useState(false);
  const [formName, setFormName] = useState('');
  const [formEmail, setFormEmail] = useState('');
  const [formEmailError, setFormEmailError] = useState('');
  const [formContactPerson, setFormContactPerson] = useState('');
  const [formInitialBalance, setFormInitialBalance] = useState('');
  const [formDailyLimit, setFormDailyLimit] = useState('');
  const [formMonthlyLimit, setFormMonthlyLimit] = useState('');
  const [formLimitError, setFormLimitError] = useState('');
  const [parentBalance, setParentBalance] = useState<string | null>(null);
  const [loadingBalance, setLoadingBalance] = useState(false);

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

  const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

  function validateForm(): boolean {
    let valid = true;
    if (!EMAIL_RE.test(formEmail)) {
      setFormEmailError('Введите корректный email адрес');
      valid = false;
    } else {
      setFormEmailError('');
    }
    const daily = formDailyLimit ? Number(formDailyLimit) : 0;
    const monthly = formMonthlyLimit ? Number(formMonthlyLimit) : 0;
    if ((formDailyLimit && daily < 0) || (formMonthlyLimit && monthly < 0)) {
      setFormLimitError('Лимиты не могут быть отрицательными');
      valid = false;
    } else {
      setFormLimitError('');
    }
    return valid;
  }

  async function openCreateForm() {
    setShowCreateForm(true);
    setParentBalance(null);
    setLoadingBalance(true);
    try {
      const res = await billingApi.getBalance() as { balance: string };
      setParentBalance(res.balance);
    } catch {
      // не критично — просто не показываем баланс
    } finally {
      setLoadingBalance(false);
    }
  }

  async function handleCreate(e: FormEvent) {
    e.preventDefault();
    if (!validateForm()) return;
    setCreating(true);
    setError('');
    try {
      const resp = await subAccountsApi.create({
        name: formName,
        email: formEmail,
        contact_person: formContactPerson || undefined,
        initial_balance: formInitialBalance || undefined,
        daily_limit: formDailyLimit ? Number(formDailyLimit) : undefined,
        monthly_limit: formMonthlyLimit ? Number(formMonthlyLimit) : undefined,
      }) as Record<string, unknown>;
      setShowCreateForm(false);
      setFormName('');
      setFormEmail('');
      setFormEmailError('');
      setFormContactPerson('');
      setFormInitialBalance('');
      setFormDailyLimit('');
      setFormMonthlyLimit('');
      setFormLimitError('');
      if (resp?.balance_transfer_error) {
        toast.info('Суб-аккаунт создан. Перевод начального баланса не выполнен — недостаточно средств');
      } else {
        toast.success('Суб-аккаунт успешно создан');
      }
      await loadSubAccounts();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось создать суб-аккаунт');
    } finally {
      setCreating(false);
    }
  }

  if (loading) return (
    <div className="flex items-center justify-center py-16" role="status" aria-live="polite">
      <div className="w-8 h-8 border-2 border-primary border-t-transparent rounded-full animate-spin" aria-hidden="true" />
      <span className="ml-3 text-sm text-gray-500">Загрузка суб-аккаунтов...</span>
    </div>
  );

  const subAccounts = data?.sub_accounts ?? [];

  return (
    <div className="max-w-5xl">
      <PageHeader
        title="Суб-аккаунты"
        subtitle={data ? `${data.current_count} / ${data.max_sub_accounts}` : undefined}
        actions={canCreate ? <Button onClick={openCreateForm}>Создать суб-аккаунт</Button> : undefined}
      />

      {error && !canCreate && (
        <div className="border border-dashed border-amber-300 rounded-lg p-12 text-center">
          <svg className="mx-auto mb-3 w-12 h-12 text-amber-400" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5}
              d="M16.5 10.5V6.75a4.5 4.5 0 10-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 002.25-2.25v-6.75a2.25 2.25 0 00-2.25-2.25H6.75a2.25 2.25 0 00-2.25 2.25v6.75a2.25 2.25 0 002.25 2.25z" />
          </svg>
          <p className="text-amber-800 font-medium text-lg mb-2">Суб-аккаунты недоступны</p>
          <p className="text-amber-600 text-sm mb-4">
            Функция доступна на тарифах с поддержкой реселлерских возможностей.
          </p>
          <a
            href="https://t.me/sms_support"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1 text-sm text-primary hover:underline"
          >
            Связаться с поддержкой →
          </a>
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
          <div>
            <Input
              label="Email"
              type="email"
              value={formEmail}
              onChange={(e) => { setFormEmail(e.target.value); setFormEmailError(''); }}
              required
              placeholder="email@example.com"
              aria-describedby={formEmailError ? 'email-error' : undefined}
              aria-invalid={!!formEmailError}
            />
            {formEmailError && (
              <p id="email-error" role="alert" className="mt-1 text-xs text-red-600">{formEmailError}</p>
            )}
          </div>
          <Input
            label="Контактное лицо"
            value={formContactPerson}
            onChange={(e) => setFormContactPerson(e.target.value)}
            placeholder="Имя контактного лица"
          />
          <div>
            <Input
              label="Начальный баланс"
              value={formInitialBalance}
              onChange={(e) => setFormInitialBalance(e.target.value)}
              placeholder="0.00"
            />
            {loadingBalance && (
              <p className="mt-1 text-xs text-gray-400">Загрузка баланса...</p>
            )}
            {parentBalance !== null && !loadingBalance && (
              <p className="mt-1 text-xs text-gray-500">
                Доступный баланс: {parseFloat(parentBalance).toFixed(2)} ₽
              </p>
            )}
            {parentBalance !== null &&
              formInitialBalance !== '' &&
              parseFloat(formInitialBalance) > parseFloat(parentBalance) && (
                <p className="mt-1 text-xs text-amber-600">
                  Сумма превышает доступный баланс. Суб-аккаунт будет создан, но перевод средств не выполнится.
                </p>
              )}
          </div>
          <div>
            <div className="flex gap-4">
              <Input
                label="Дневной лимит"
                type="number"
                min="0"
                value={formDailyLimit}
                onChange={(e) => { setFormDailyLimit(e.target.value); setFormLimitError(''); }}
                placeholder="напр. 1000"
              />
              <Input
                label="Месячный лимит"
                type="number"
                min="0"
                value={formMonthlyLimit}
                onChange={(e) => { setFormMonthlyLimit(e.target.value); setFormLimitError(''); }}
                placeholder="напр. 30000"
              />
            </div>
            {formLimitError && (
              <p role="alert" className="mt-1 text-xs text-red-600">{formLimitError}</p>
            )}
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
      {canCreate && subAccounts.length === 0 && !error && (
        <div className="border border-dashed border-gray-300 rounded-lg p-12 text-center">
          <p className="text-gray-500 font-medium mb-1">Суб-аккаунтов пока нет</p>
          <p className="text-sm text-gray-400">Нажмите «Создать суб-аккаунт», чтобы добавить первый</p>
        </div>
      )}

      {canCreate && subAccounts.length > 0 && (
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
