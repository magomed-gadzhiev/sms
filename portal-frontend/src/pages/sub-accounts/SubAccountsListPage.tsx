import { useState, useEffect, useMemo, type FormEvent } from 'react';
import { useNavigate, useSearchParams, Link } from 'react-router-dom';
import { subAccountsApi, billingApi, ApiError } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { usePageTitle } from '../../contexts/PageTitleContext';
import { Button } from '../../components/ui/Button';
import { Modal } from '../../components/ui/Modal';
import { Input } from '../../components/ui/Input';
import { DataTable, type Column } from '../../components/data/DataTable';
import { StatusBadge } from '../../components/ui/Badge';
import { useToast } from '../../components/ui/Toast';
import { CapacityIndicator } from './components/CapacityIndicator';
import { SubAccountKebabMenu } from './components/SubAccountKebabMenu';
import { SubAccountCard } from './components/SubAccountCard';
import { formatNumber, formatRub } from './utils/formatNumber';

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

const LOW_BALANCE_THRESHOLD = 100;

type SortKey = keyof SubAccount;
type SortDir = 'asc' | 'desc';

function compareValues(a: unknown, b: unknown): number {
  if (typeof a === 'number' && typeof b === 'number') return a - b;
  if (typeof a === 'string' && typeof b === 'string') {
    const an = parseFloat(a);
    const bn = parseFloat(b);
    if (Number.isFinite(an) && Number.isFinite(bn)) return an - bn;
    return a.localeCompare(b, 'ru');
  }
  return 0;
}

export function SubAccountsListPage() {
  usePageTitle('Суб-аккаунты');
  const toast = useToast();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();

  const [data, setData] = useState<SubAccountsListResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [canCreate, setCanCreate] = useState(true);

  const [search, setSearch] = useState('');
  const [lowOnly, setLowOnly] = useState(searchParams.get('filter') === 'low');
  const [sortBy, setSortBy] = useState<SortKey>('name');
  const [sortDir, setSortDir] = useState<SortDir>('asc');

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

  useEffect(() => { loadSubAccounts(); }, []);

  function toggleLowOnly() {
    const next = !lowOnly;
    setLowOnly(next);
    if (next) setSearchParams({ filter: 'low' });
    else setSearchParams({});
  }

  function clearFilters() {
    setSearch('');
    setLowOnly(false);
    setSearchParams({});
  }

  const filtered = useMemo(() => {
    const all = data?.sub_accounts ?? [];
    const q = search.trim().toLowerCase();
    return all.filter((sa) => {
      if (q && !sa.name.toLowerCase().includes(q) && !sa.email.toLowerCase().includes(q)) return false;
      if (lowOnly && parseFloat(sa.balance) >= LOW_BALANCE_THRESHOLD) return false;
      return true;
    });
  }, [data, search, lowOnly]);

  const sorted = useMemo(() => {
    const arr = [...filtered];
    arr.sort((a, b) => {
      const cmp = compareValues(a[sortBy], b[sortBy]);
      return sortDir === 'asc' ? cmp : -cmp;
    });
    return arr;
  }, [filtered, sortBy, sortDir]);

  function handleSort(key: string) {
    if (sortBy === key) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
    } else {
      setSortBy(key as SortKey);
      setSortDir('asc');
    }
  }

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
      // не критично
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
      setFormName(''); setFormEmail(''); setFormEmailError('');
      setFormContactPerson(''); setFormInitialBalance('');
      setFormDailyLimit(''); setFormMonthlyLimit(''); setFormLimitError('');
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

  const columns: Column<SubAccount>[] = [
    {
      key: 'name',
      header: 'Название',
      sortable: true,
      render: (sa) => {
        const isLow = parseFloat(sa.balance) < LOW_BALANCE_THRESHOLD;
        return (
          <span className="inline-flex items-center gap-2">
            {isLow && (
              <span className="w-2 h-2 rounded-full bg-red-500 shrink-0"
                    aria-label="Низкий баланс" title="Баланс ниже 100 ₽" />
            )}
            <Link
              to={`/network/sub-accounts/${sa.id}`}
              className="text-primary hover:underline"
              onClick={(e) => e.stopPropagation()}
            >
              {sa.name}
            </Link>
          </span>
        );
      },
    },
    {
      key: 'email',
      header: 'Email',
      sortable: true,
      responsive: true,
      render: (sa) => sa.email || <span className="text-gray-400">—</span>,
    },
    {
      key: 'active',
      header: 'Статус',
      render: (sa) => <StatusBadge status={sa.active ? 'active' : 'inactive'} />,
    },
    {
      key: 'balance',
      header: 'Баланс',
      sortable: true,
      render: (sa) => {
        const v = parseFloat(sa.balance);
        const isLow = v < LOW_BALANCE_THRESHOLD;
        return (
          <span className={isLow ? 'text-red-600 dark:text-red-400 font-medium' : ''}>
            {formatRub(sa.balance)}
          </span>
        );
      },
    },
    {
      key: 'daily_limit',
      header: 'Дневной лимит',
      sortable: true,
      responsive: true,
      render: (sa) => sa.daily_limit === 0
        ? <span aria-label="без лимита" title="Без лимита">∞</span>
        : formatNumber(sa.daily_limit),
    },
    {
      key: 'monthly_limit',
      header: 'Месячный лимит',
      sortable: true,
      responsive: true,
      render: (sa) => sa.monthly_limit === 0
        ? <span aria-label="без лимита" title="Без лимита">∞</span>
        : formatNumber(sa.monthly_limit),
    },
    {
      key: 'messages_today',
      header: 'Сегодня',
      sortable: true,
      responsive: true,
      render: (sa) => sa.daily_limit > 0
        ? `${formatNumber(sa.messages_today)} / ${formatNumber(sa.daily_limit)}`
        : `${formatNumber(sa.messages_today)} msg`,
    },
    {
      key: 'messages_this_month',
      header: 'За месяц',
      sortable: true,
      responsive: true,
      render: (sa) => sa.monthly_limit > 0
        ? `${formatNumber(sa.messages_this_month)} / ${formatNumber(sa.monthly_limit)}`
        : `${formatNumber(sa.messages_this_month)} msg`,
    },
  ];

  if (loading) return (
    <div className="flex items-center justify-center py-16" role="status" aria-live="polite">
      <div className="w-8 h-8 border-2 border-primary border-t-transparent rounded-full animate-spin" aria-hidden="true" />
      <span className="ml-3 text-sm text-gray-500 dark:text-slate-400">Загрузка суб-аккаунтов...</span>
    </div>
  );

  const subAccounts = data?.sub_accounts ?? [];
  const atLimit = data ? data.current_count >= data.max_sub_accounts : false;
  const filtersActive = search.trim() !== '' || lowOnly;

  return (
    <div className="max-w-6xl">
      <PageHeader
        actions={canCreate ? (
          <Button onClick={openCreateForm} disabled={atLimit}
                  title={atLimit ? 'Достигнут лимит суб-аккаунтов в вашем плане' : undefined}>
            Создать суб-аккаунт
          </Button>
        ) : undefined}
      />

      {data && (
        <div className="mb-4">
          <CapacityIndicator current={data.current_count} max={data.max_sub_accounts} />
        </div>
      )}

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
          <a href="https://t.me/sms_support" target="_blank" rel="noopener noreferrer"
             className="inline-flex items-center gap-1 text-sm text-primary hover:underline">
            Связаться с поддержкой →
          </a>
        </div>
      )}

      {error && canCreate && (
        <div role="alert" className="bg-red-50 dark:bg-red-950/40 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 rounded p-3 mb-4 text-sm flex items-center justify-between">
          <span>{error}</span>
          <Button size="sm" variant="secondary" onClick={loadSubAccounts}>Повторить</Button>
        </div>
      )}

      <Modal open={showCreateForm} onClose={() => setShowCreateForm(false)} title="Создать суб-аккаунт">
        <form onSubmit={handleCreate} className="flex flex-col gap-4">
          <Input label="Название" value={formName} onChange={(e) => setFormName(e.target.value)}
                 required placeholder="Название суб-аккаунта" />
          <div>
            <Input label="Email" type="email" value={formEmail}
                   onChange={(e) => { setFormEmail(e.target.value); setFormEmailError(''); }}
                   required placeholder="email@example.com"
                   aria-describedby={formEmailError ? 'email-error' : undefined}
                   aria-invalid={!!formEmailError} />
            {formEmailError && (
              <p id="email-error" role="alert" className="mt-1 text-xs text-red-600 dark:text-red-400">{formEmailError}</p>
            )}
          </div>
          <Input label="Контактное лицо" value={formContactPerson}
                 onChange={(e) => setFormContactPerson(e.target.value)} placeholder="Имя контактного лица" />
          <div>
            <Input label="Начальный баланс" value={formInitialBalance}
                   onChange={(e) => setFormInitialBalance(e.target.value)} placeholder="0.00" />
            {loadingBalance && (
              <p className="mt-1 text-xs text-gray-400 dark:text-slate-500">Загрузка баланса...</p>
            )}
            {parentBalance !== null && !loadingBalance && (
              <p className="mt-1 text-xs text-gray-500 dark:text-slate-400">
                Доступный баланс: {formatRub(parentBalance)}
              </p>
            )}
            {parentBalance !== null && formInitialBalance !== '' &&
              parseFloat(formInitialBalance) > parseFloat(parentBalance) && (
                <p className="mt-1 text-xs text-amber-600">
                  Сумма превышает доступный баланс. Суб-аккаунт будет создан, но перевод средств не выполнится.
                </p>
              )}
          </div>
          <div>
            <div className="flex gap-4">
              <Input label="Дневной лимит" type="number" min="0" value={formDailyLimit}
                     onChange={(e) => { setFormDailyLimit(e.target.value); setFormLimitError(''); }}
                     placeholder="напр. 1000" />
              <Input label="Месячный лимит" type="number" min="0" value={formMonthlyLimit}
                     onChange={(e) => { setFormMonthlyLimit(e.target.value); setFormLimitError(''); }}
                     placeholder="напр. 30000" />
            </div>
            {formLimitError && (
              <p role="alert" className="mt-1 text-xs text-red-600 dark:text-red-400">{formLimitError}</p>
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

      {canCreate && subAccounts.length === 0 && !error && (
        <div className="border border-dashed border-gray-300 dark:border-slate-600 rounded-lg p-12 text-center">
          <p className="text-gray-500 dark:text-slate-400 font-medium mb-1">Суб-аккаунтов пока нет</p>
          <p className="text-sm text-gray-400 dark:text-slate-500">Нажмите «Создать суб-аккаунт», чтобы добавить первый</p>
        </div>
      )}

      {canCreate && subAccounts.length > 0 && (
        <>
          <div className="mb-3 flex flex-col sm:flex-row sm:items-center gap-3">
            <div className="flex-1 max-w-sm">
              <input
                type="text"
                aria-label="Поиск по имени или email"
                placeholder="Поиск по имени или email"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 placeholder-gray-400 dark:placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary"
              />
            </div>
            <button
              type="button"
              onClick={toggleLowOnly}
              className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-sm border cursor-pointer
                          transition-colors
                          ${lowOnly
                            ? 'bg-primary text-white border-primary'
                            : 'bg-white dark:bg-slate-800 text-gray-700 dark:text-slate-200 border-gray-300 dark:border-slate-600 hover:bg-gray-50 dark:hover:bg-slate-700'}`}
              aria-pressed={lowOnly}
            >
              <span className="w-2 h-2 rounded-full bg-red-500" aria-hidden="true" />
              Только с низким балансом
            </button>
            <span className="text-sm text-gray-500 dark:text-slate-400 sm:ml-auto">
              Показано {sorted.length} из {subAccounts.length}
            </span>
          </div>

          {sorted.length === 0 && filtersActive && (
            <div className="border border-dashed border-gray-300 dark:border-slate-600 rounded-lg p-8 text-center">
              <p className="text-gray-500 dark:text-slate-400 mb-3">По запросу ничего не найдено</p>
              <Button size="sm" variant="secondary" onClick={clearFilters}>Сбросить фильтры</Button>
            </div>
          )}

          {sorted.length > 0 && (
            <>
              <div className="hidden md:block">
                <DataTable
                  columns={columns}
                  data={sorted}
                  total={sorted.length}
                  page={1}
                  pageSize={sorted.length}
                  onPageChange={() => {}}
                  keyField="id"
                  sortBy={sortBy}
                  sortDir={sortDir}
                  onSort={handleSort}
                  onRowClick={(sa) => navigate(`/network/sub-accounts/${sa.id}`)}
                  rowActions={(sa) => <SubAccountKebabMenu subAccount={sa} onChanged={loadSubAccounts} />}
                />
              </div>

              <div className="md:hidden space-y-3">
                {sorted.map((sa) => (
                  <SubAccountCard key={sa.id} subAccount={sa} onChanged={loadSubAccounts} />
                ))}
              </div>
            </>
          )}
        </>
      )}
    </div>
  );
}
