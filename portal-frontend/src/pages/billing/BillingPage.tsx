import { useEffect, useState, useCallback } from 'react';
import { billingApi } from '../../api/client';
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
  { value: '', label: 'Все' },
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

  /* ---------- Fetchers ---------- */

  const fetchBalance = useCallback(() => {
    setBalanceLoading(true);
    setBalanceError(null);
    billingApi
      .getBalance()
      .then((resp) => setBalance(resp as BalanceInfo))
      .catch((err) => setBalanceError(err.message || 'Не удалось загрузить баланс'))
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
      .catch((err) => setError(err.message || 'Не удалось загрузить транзакции'))
      .finally(() => setLoading(false));
  }, [page, filterValues]);

  useEffect(() => {
    fetchBalance();
  }, [fetchBalance]);

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

  /* ---------- Render ---------- */

  return (
    <div>
      <PageHeader
        title="Биллинг"
        actions={<Button onClick={openTopUpModal}>Пополнить</Button>}
      />

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
                {balance.balance}{' '}
                <span className="text-lg font-normal text-gray-500">{balance.currency}</span>
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
    </div>
  );
}
