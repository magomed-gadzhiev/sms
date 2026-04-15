import { useEffect, useState, useCallback } from 'react';
import { Link } from 'react-router-dom';
import { billingApi, companiesApi, ApiError, type CompanyInfo } from '../../api/client';
import { useAuth } from '../../contexts/AuthContext';
import { PageHeader } from '../../components/layout/PageHeader';
import { FilterBar, type FilterDef } from '../../components/data/FilterBar';
import { DataTable, type Column } from '../../components/data/DataTable';
import { Button } from '../../components/ui/Button';
import { Modal } from '../../components/ui/Modal';
import { Input } from '../../components/ui/Input';
import { Badge } from '../../components/ui/Badge';

/* ---------- Types ---------- */

interface BalanceInfo {
  client_id: string;
  balance: string;
  currency: string;
  updated_at?: string;
  low_balance_threshold?: string;
}

interface TransactionItem {
  transaction_id: string;
  type: string;
  amount: string;
  balance_after: string;
  currency: string;
  description: string;
  message_id?: string;
  created_at: string;
}

interface TransactionsResponse {
  transactions: TransactionItem[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
}

/* ---------- Helpers ---------- */

function fmtMoney(val: string, currency: string): string {
  const num = parseFloat(val);
  if (isNaN(num)) return `${val} ${currency}`;
  return `${num.toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ${currency}`;
}

/* ---------- Constants ---------- */

const PAGE_SIZE = 20;

const TYPE_OPTIONS = [
  { value: 'charge', label: 'Списание' },
  { value: 'credit', label: 'Пополнение' },
  { value: 'refund', label: 'Возврат' },
  { value: 'transfer', label: 'Перевод' },
];

const TRANSACTION_FILTERS: FilterDef[] = [
  { key: 'date_from', label: 'С даты', type: 'date' },
  { key: 'date_to', label: 'По дату', type: 'date' },
  { key: 'type', label: 'Тип', type: 'select', options: TYPE_OPTIONS },
];

const INITIAL_FILTERS: Record<string, string> = {
  date_from: '',
  date_to: '',
  type: '',
};

const typeBadgeVariant: Record<string, 'danger' | 'success' | 'warning' | 'default'> = {
  charge: 'danger',
  credit: 'success',
  refund: 'warning',
  transfer: 'default',
};

const typeLabel: Record<string, string> = {
  charge: 'Списание',
  credit: 'Пополнение',
  refund: 'Возврат',
  transfer: 'Перевод',
};

const columns: Column<TransactionItem>[] = [
  {
    key: 'created_at',
    header: 'Дата',
    render: (tx) => (
      <span className="text-xs whitespace-nowrap">
        {tx.created_at ? new Date(tx.created_at).toLocaleString('ru-RU') : '-'}
      </span>
    ),
  },
  {
    key: 'type',
    header: 'Тип',
    render: (tx) => (
      <Badge variant={typeBadgeVariant[tx.type] ?? 'default'}>
        {typeLabel[tx.type] ?? tx.type}
      </Badge>
    ),
  },
  {
    key: 'amount',
    header: 'Сумма',
    render: (tx) => (
      <span className={tx.type === 'charge' ? 'text-red-600 font-medium' : 'text-green-600 font-medium'}>
        {tx.type === 'charge' ? '-' : '+'}{fmtMoney(tx.amount, tx.currency)}
      </span>
    ),
  },
  {
    key: 'balance_after',
    header: 'Баланс после',
    render: (tx) => <span className="tabular-nums">{fmtMoney(tx.balance_after, tx.currency)}</span>,
  },
  { key: 'description', header: 'Описание' },
  {
    key: 'message_id',
    header: 'ID сообщения',
    render: (tx) =>
      tx.message_id ? (
        <span className="font-mono text-xs">{tx.message_id.substring(0, 8)}...</span>
      ) : (
        <span className="text-gray-400">-</span>
      ),
  },
];

/* ---------- Component ---------- */

export function BillingPage() {
  const { user } = useAuth();
  const isReseller = !!user?.is_reseller;

  /* Companies state */
  const [companies, setCompanies] = useState<CompanyInfo[]>([]);

  /* Balance state */
  const [balance, setBalance] = useState<BalanceInfo | null>(null);
  const [balanceLoading, setBalanceLoading] = useState(false);
  const [balanceError, setBalanceError] = useState<string | null>(null);

  /* Transactions state */
  const [data, setData] = useState<TransactionsResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [filterValues, setFilterValues] = useState<Record<string, string>>(INITIAL_FILTERS);

  /* Top-up modal state */
  const [showTopUp, setShowTopUp] = useState(false);
  const [topUpAmount, setTopUpAmount] = useState('');
  const [topUpLoading, setTopUpLoading] = useState(false);
  const [topUpError, setTopUpError] = useState('');

  /* Alert settings state */
  const [thresholdInput, setThresholdInput] = useState('');
  const [thresholdSaving, setThresholdSaving] = useState(false);
  const [thresholdSaved, setThresholdSaved] = useState(false);
  const [thresholdError, setThresholdError] = useState('');

  /* ---------- Fetchers ---------- */

  const fetchBalance = useCallback(() => {
    setBalanceLoading(true);
    setBalanceError(null);
    billingApi
      .getBalance()
      .then((resp) => {
        setBalance(resp as BalanceInfo);
        if (resp.low_balance_threshold) {
          setThresholdInput(resp.low_balance_threshold);
        }
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 404) {
          setBalance({ client_id: '', balance: '0.00', currency: 'RUB' });
        } else {
          setBalanceError('Не удалось загрузить данные биллинга');
        }
      })
      .finally(() => setBalanceLoading(false));
  }, []);

  const fetchTransactions = useCallback(() => {
    setLoading(true);
    setError(null);

    const params: Record<string, string> = {
      page: String(page),
      per_page: String(PAGE_SIZE),
    };
    if (filterValues.date_from) params.date_from = filterValues.date_from;
    if (filterValues.date_to) params.date_to = filterValues.date_to;
    if (filterValues.type) params.type = filterValues.type;

    billingApi
      .getTransactions(params)
      .then((resp) => setData(resp as TransactionsResponse))
      .catch(() => setError('Не удалось загрузить транзакции'))
      .finally(() => setLoading(false));
  }, [page, filterValues]);

  useEffect(() => {
    fetchBalance();
  }, [fetchBalance]);

  useEffect(() => {
    companiesApi.list()
      .then((res) => setCompanies(res.companies ?? []))
      .catch(() => {});
  }, []);

  useEffect(() => {
    fetchTransactions();
  }, [fetchTransactions]);

  /* ---------- Handlers ---------- */

  const handleFilterChange = (values: Record<string, string>) => {
    setFilterValues(values);
    setPage(1);
  };

  const handleFilterReset = () => {
    setFilterValues(INITIAL_FILTERS);
    setPage(1);
  };

  const MAX_TOP_UP = 1_000_000;

  const handleTopUp = async () => {
    const amount = Number(topUpAmount);
    if (!topUpAmount || amount <= 0) {
      setTopUpError('Введите сумму больше нуля');
      return;
    }
    if (amount > MAX_TOP_UP) {
      setTopUpError(`Максимальная сумма разового пополнения — ${MAX_TOP_UP.toLocaleString()} ₽`);
      return;
    }
    setTopUpLoading(true);
    setTopUpError('');
    try {
      const resp = await billingApi.topUp(topUpAmount);
      if (resp.payment_url) {
        window.location.href = resp.payment_url;
      }
    } catch (err) {
      setTopUpError(err instanceof Error ? err.message : 'Ошибка при создании платежа');
    } finally {
      setTopUpLoading(false);
    }
  };

  const openTopUpModal = () => {
    setTopUpAmount('');
    setTopUpError('');
    setShowTopUp(true);
  };

  const handleSaveThreshold = async () => {
    const value = thresholdInput.trim();
    if (!value || Number(value) < 0) {
      setThresholdError('Введите корректное значение порога (≥ 0)');
      return;
    }
    setThresholdSaving(true);
    setThresholdError('');
    setThresholdSaved(false);
    try {
      await billingApi.setLowBalanceThreshold(value);
      setThresholdSaved(true);
      setTimeout(() => setThresholdSaved(false), 3000);
    } catch (err) {
      setThresholdError(err instanceof Error ? err.message : 'Ошибка сохранения');
    } finally {
      setThresholdSaving(false);
    }
  };

  /* ---------- Render ---------- */

  return (
    <div>
      <PageHeader
        title="Биллинг"
        actions={<Button onClick={openTopUpModal}>Пополнить</Button>}
      />

      {isReseller && (
        <div className="mb-4 bg-blue-50 border border-blue-200 rounded-lg px-4 py-3 text-sm text-blue-800">
          Отображается баланс и транзакции вашего аккаунта. Балансы суб-аккаунтов и переводы доступны
          в разделе <Link to="/sub-accounts" className="font-medium underline hover:text-blue-900">Суб-аккаунты</Link>.
        </div>
      )}

      {/* Company balances */}
      {companies.length > 0 && (
        <div className="mb-6">
          <h2 className="text-sm font-semibold text-gray-600 uppercase tracking-wide mb-3">
            Балансы по компаниям
          </h2>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {companies.map((c) => (
              <div key={c.id} className="bg-white rounded-lg border border-gray-200 p-4">
                <div className="flex items-start justify-between">
                  <div>
                    <p className="text-sm font-medium text-gray-900">{c.name}</p>
                    {c.inn && <p className="text-xs text-gray-500 mt-0.5">ИНН: {c.inn}</p>}
                  </div>
                  <div className="flex flex-col items-end gap-1">
                    {c.is_default && (
                      <span className="text-xs bg-green-100 text-green-700 px-1.5 py-0.5 rounded">
                        Основная
                      </span>
                    )}
                    {c.is_offer && (
                      <span className="text-xs bg-gray-100 text-gray-600 px-1.5 py-0.5 rounded">
                        Оферта
                      </span>
                    )}
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Balance card */}
      <div className="mb-6 rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
        {balanceError && (
          <p role="alert" className="text-red-600 text-sm mb-2">{balanceError}</p>
        )}
        {balanceLoading && !balance ? (
          <div className="animate-pulse space-y-2">
            <div className="h-8 w-40 bg-gray-200 rounded" />
            <div className="h-4 w-28 bg-gray-100 rounded" />
          </div>
        ) : balance ? (
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-gray-500 mb-1">Текущий баланс</p>
              <p className="text-3xl font-bold tabular-nums">
                {new Intl.NumberFormat('ru-RU', { style: 'currency', currency: 'RUB', minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(parseFloat(balance.balance) || 0)}
              </p>
              {balance.updated_at && (
                <p className="text-xs text-gray-500 mt-1">
                  Обновлено: {new Date(balance.updated_at).toLocaleString('ru-RU')}
                </p>
              )}
            </div>
            <Button onClick={openTopUpModal}>Пополнить</Button>
          </div>
        ) : null}
      </div>

      {/* Alert Settings card */}
      <div className="mb-6 rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
        <h2 className="text-base font-semibold mb-3">Настройки уведомлений</h2>
        <p className="text-sm text-gray-500 mb-4">
          Получайте уведомление через webhook, когда баланс опускается ниже порогового значения.
        </p>
        <div className="flex items-end gap-3">
          <div className="flex-1 max-w-xs">
            <Input
              label="Порог низкого баланса (₽)"
              type="number"
              value={thresholdInput}
              onChange={(e) => {
                setThresholdInput(e.target.value);
                setThresholdError('');
                setThresholdSaved(false);
              }}
              placeholder="0.00"
              min="0"
              step="0.01"
            />
          </div>
          <Button
            onClick={handleSaveThreshold}
            disabled={thresholdSaving}
            variant="secondary"
          >
            {thresholdSaving ? 'Сохранение...' : 'Сохранить'}
          </Button>
        </div>
        {thresholdError && (
          <p role="alert" className="text-red-600 text-sm mt-2">{thresholdError}</p>
        )}
        {thresholdSaved && (
          <p role="status" className="text-green-600 text-sm mt-2">Порог сохранён</p>
        )}
      </div>

      {/* Top-up modal */}
      <Modal
        open={showTopUp}
        onClose={() => setShowTopUp(false)}
        title="Пополнение баланса"
        description="Введите сумму для пополнения"
      >
        <div className="space-y-4">
          {topUpError && (
            <p role="alert" className="text-red-600 text-sm">{topUpError}</p>
          )}
          <Input
            label="Сумма (₽)"
            type="number"
            value={topUpAmount}
            onChange={(e) => { setTopUpAmount(e.target.value); setTopUpError(''); }}
            placeholder="1000.00"
            required
            min="1"
            max="1000000"
            step="0.01"
          />
          <p className="text-xs text-gray-400">Минимум: 1 ₽ · Максимум: 1 000 000 ₽</p>
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="secondary" onClick={() => setShowTopUp(false)}>
              Отмена
            </Button>
            <Button
              onClick={handleTopUp}
              disabled={topUpLoading || !topUpAmount || Number(topUpAmount) <= 0 || Number(topUpAmount) > 1_000_000}
            >
              {topUpLoading ? 'Создание платежа...' : 'Оплатить'}
            </Button>
          </div>
        </div>
      </Modal>

      {/* Transactions section */}
      <h2 className="text-lg font-semibold mb-3">История транзакций</h2>

      <FilterBar
        filters={TRANSACTION_FILTERS}
        values={filterValues}
        onChange={handleFilterChange}
        onReset={handleFilterReset}
      />

      {error && <div role="alert" className="text-red-600 mb-3">Ошибка: {error}</div>}

      {!loading && !error && (data?.transactions?.length ?? 0) === 0 && (
        <div className="text-center py-10 text-gray-500">
          <svg className="mx-auto mb-3 w-10 h-10 text-gray-300" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5}
              d="M9 14l2 2 4-4M7 21h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v12a2 2 0 002 2z" />
          </svg>
          <p className="text-sm">История транзакций пуста</p>
          <p className="text-xs text-gray-400 mt-1">Здесь будут отображаться пополнения, списания и возвраты</p>
        </div>
      )}

      {(loading || (data?.transactions?.length ?? 0) > 0) && (
        <DataTable<TransactionItem>
          columns={columns}
          data={data?.transactions ?? []}
          total={data?.total ?? 0}
          page={page}
          pageSize={PAGE_SIZE}
          onPageChange={setPage}
          keyField="transaction_id"
          loading={loading}
        />
      )}
    </div>
  );
}
