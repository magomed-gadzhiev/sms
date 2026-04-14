import { useEffect, useState, useCallback } from 'react';
import { billingApi, companiesApi, ApiError, type CompanyInfo } from '../../api/client';
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

/* ---------- Constants ---------- */

const PAGE_SIZE = 20;

const TYPE_OPTIONS = [
  { value: 'charge', label: 'Списание' },
  { value: 'credit', label: 'Пополнение' },
  { value: 'refund', label: 'Возврат' },
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
};

const typeLabel: Record<string, string> = {
  charge: 'Списание',
  credit: 'Пополнение',
  refund: 'Возврат',
};

const columns: Column<TransactionItem>[] = [
  {
    key: 'created_at',
    header: 'Дата',
    render: (tx) => (
      <span className="text-xs whitespace-nowrap">
        {tx.created_at ? new Date(tx.created_at).toLocaleString() : '-'}
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
        {tx.type === 'charge' ? '-' : '+'}{tx.amount} {tx.currency}
      </span>
    ),
  },
  {
    key: 'balance_after',
    header: 'Баланс после',
    render: (tx) => <span className="tabular-nums">{tx.balance_after} {tx.currency}</span>,
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

  const handleTopUp = async () => {
    if (!topUpAmount || Number(topUpAmount) <= 0) return;
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
          <p className="text-red-600 text-sm mb-2">{balanceError}</p>
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
                  Обновлено: {new Date(balance.updated_at).toLocaleString()}
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
          <p className="text-green-600 text-sm mt-2">Порог сохранён</p>
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
            label="Сумма"
            type="number"
            value={topUpAmount}
            onChange={(e) => setTopUpAmount(e.target.value)}
            placeholder="0.00"
            required
          />
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="secondary" onClick={() => setShowTopUp(false)}>
              Отмена
            </Button>
            <Button
              onClick={handleTopUp}
              disabled={topUpLoading || !topUpAmount || Number(topUpAmount) <= 0}
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

      {error && <div className="text-red-600 mb-3">Ошибка: {error}</div>}

      {!loading && !error && (data?.transactions?.length ?? 0) === 0 && (
        <div className="text-center py-8 text-gray-500 text-sm">
          История транзакций пуста
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
