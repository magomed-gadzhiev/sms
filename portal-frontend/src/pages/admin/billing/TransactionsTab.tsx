import { useState, useEffect, useCallback } from 'react';
import { DataTable, type Column } from '../../../components/data/DataTable';
import { FilterBar, type FilterDef } from '../../../components/data/FilterBar';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { Modal } from '../../../components/ui/Modal';
import { Badge } from '../../../components/ui/Badge';
import { useToast } from '../../../components/ui/Toast';
import { billingApi, type Transaction } from '../../../api/admin';

const PAGE_SIZE = 20;

const txFilters: FilterDef[] = [
  { key: 'client_id', label: 'ID клиента', type: 'text', placeholder: 'UUID...' },
  {
    key: 'transaction_type',
    label: 'Тип',
    type: 'select',
    options: [
      { value: 'credit', label: 'Пополнение' },
      { value: 'charge', label: 'Списание (charge)' },
      { value: 'refund', label: 'Возврат' },
      { value: 'adjustment', label: 'Корректировка' },
      { value: 'transfer_in', label: 'Трансфер: входящий' },
      { value: 'transfer_out', label: 'Трансфер: исходящий' },
    ],
  },
  { key: 'from', label: 'От', type: 'date' },
  { key: 'to', label: 'До', type: 'date' },
];

const txColumns: Column<Transaction>[] = [
  {
    key: 'created_at',
    header: 'Дата',
    render: (t) => new Date(t.created_at).toLocaleString(),
    sortable: true,
  },
  {
    key: 'client_id',
    header: 'Клиент',
    render: (t) => t.client_id.slice(0, 8) + '...',
  },
  {
    key: 'type',
    header: 'Тип',
    render: (t) => (
      <Badge variant={t.type === 'credit' ? 'success' : 'danger'}>{t.type === 'credit' ? 'Пополнение' : t.type === 'charge' || t.type === 'debit' ? 'Списание' : t.type}</Badge>
    ),
  },
  {
    key: 'amount',
    header: 'Сумма',
    render: (t) => `${parseFloat(t.amount).toFixed(2)} ${t.currency}`,
  },
  { key: 'balance_after', header: 'Баланс после', render: (t) => parseFloat(t.balance_after).toFixed(2) },
  { key: 'description', header: 'Описание' },
];

export function TransactionsTab() {
  const toast = useToast();
  const [data, setData] = useState<Transaction[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [filter, setFilter] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);

  // Add credits modal
  const [showAddCredits, setShowAddCredits] = useState(false);
  const [creditForm, setCreditForm] = useState({ client_id: '', amount: '', description: '' });
  const [saving, setSaving] = useState(false);

  const fetchTransactions = useCallback(async () => {
    setLoading(true);
    try {
      const res = await billingApi.getTransactions({
        ...filter,
        limit: PAGE_SIZE,
        offset: (page - 1) * PAGE_SIZE,
      });
      setData(res.transactions || []);
      setTotal(res.total);
    } catch {
      toast.error('Не удалось загрузить транзакции');
    } finally {
      setLoading(false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, filter]);

  useEffect(() => {
    fetchTransactions();
  }, [fetchTransactions]);

  const handleAddCredits = async () => {
    setSaving(true);
    try {
      const res = await billingApi.addCredits(creditForm.client_id, {
        amount: creditForm.amount,
        description: creditForm.description,
      });
      toast.success(`Средства начислены. Новый баланс: ${res.new_balance}`);
      setShowAddCredits(false);
      setCreditForm({ client_id: '', amount: '', description: '' });
      fetchTransactions();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка');
    } finally {
      setSaving(false);
    }
  };

  return (
    <>
      <div className="flex justify-end mb-4">
        <Button
          onClick={() => {
            setCreditForm({ client_id: '', amount: '', description: '' });
            setShowAddCredits(true);
          }}
        >
          Начислить средства
        </Button>
      </div>
      <FilterBar
        filters={txFilters}
        values={filter}
        onChange={(v) => {
          setFilter(v);
          setPage(1);
        }}
        onReset={() => {
          setFilter({});
          setPage(1);
        }}
      />
      <DataTable
        columns={txColumns}
        data={data}
        total={total}
        page={page}
        pageSize={PAGE_SIZE}
        onPageChange={setPage}
        loading={loading}
        keyField="transaction_id"
      />

      <Modal open={showAddCredits} onClose={() => setShowAddCredits(false)} title="Начисление средств">
        <div className="space-y-4">
          <Input
            label="ID клиента"
            value={creditForm.client_id}
            onChange={(e) => setCreditForm({ ...creditForm, client_id: e.target.value })}
            required
            placeholder="UUID"
          />
          <Input
            label="Сумма"
            type="number"
            value={creditForm.amount}
            onChange={(e) => setCreditForm({ ...creditForm, amount: e.target.value })}
            required
            placeholder="0.00"
          />
          <Input
            label="Описание"
            value={creditForm.description}
            onChange={(e) => setCreditForm({ ...creditForm, description: e.target.value })}
          />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setShowAddCredits(false)}>
              Отмена
            </Button>
            <Button
              onClick={handleAddCredits}
              disabled={saving || !creditForm.client_id || !creditForm.amount}
            >
              {saving ? 'Начисление...' : 'Начислить'}
            </Button>
          </div>
        </div>
      </Modal>
    </>
  );
}
