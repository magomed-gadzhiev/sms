import { useState, useEffect, useCallback } from 'react';
import { DataTable, type Column } from '../../../components/data/DataTable';
import { FilterBar, type FilterDef } from '../../../components/data/FilterBar';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { Select } from '../../../components/ui/Select';
import { Modal } from '../../../components/ui/Modal';
import { Badge } from '../../../components/ui/Badge';
import { useToast } from '../../../components/ui/Toast';
import { billingApi, type BalanceInfoItem } from '../../../api/admin';

const PAGE_SIZE = 20;

const balanceFilters: FilterDef[] = [
  { key: 'search', label: 'Поиск', type: 'text', placeholder: 'Имя клиента...' },
  {
    key: 'status',
    label: 'Статус',
    type: 'select',
    options: [
      { value: '', label: 'Все' },
      { value: 'active', label: 'Активен' },
      { value: 'frozen', label: 'Заморожен' },
    ],
  },
  {
    key: 'below_threshold',
    label: 'Ниже порога',
    type: 'select',
    options: [
      { value: '', label: 'Все' },
      { value: 'yes', label: 'Да' },
    ],
  },
];

interface CreditDebitForm {
  type: 'credit' | 'debit' | 'adjustment' | 'refund';
  amount: string;
  reason: string;
}

const typeOptions = [
  { value: 'credit', label: 'Credit' },
  { value: 'debit', label: 'Debit' },
  { value: 'adjustment', label: 'Adjustment' },
  { value: 'refund', label: 'Refund' },
];

export function BalancesTab() {
  const toast = useToast();
  const [data, setData] = useState<BalanceInfoItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [filter, setFilter] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);

  // Credit/Debit modal
  const [modalOpen, setModalOpen] = useState(false);
  const [modalMode, setModalMode] = useState<'credit' | 'debit'>('credit');
  const [modalClientId, setModalClientId] = useState('');
  const [modalCurrency, setModalCurrency] = useState('');
  const [creditForm, setCreditForm] = useState<CreditDebitForm>({
    type: 'credit',
    amount: '',
    reason: '',
  });
  const [saving, setSaving] = useState(false);

  const fetchBalances = useCallback(async () => {
    setLoading(true);
    try {
      const params: Record<string, string | boolean | number | undefined> = {
        ...filter,
        limit: PAGE_SIZE,
        offset: (page - 1) * PAGE_SIZE,
      };
      if (filter.below_threshold === 'yes') {
        params.below_threshold = true;
      } else {
        delete params.below_threshold;
      }
      const res = await billingApi.listBalances(params as Parameters<typeof billingApi.listBalances>[0]);
      setData(res.balances || []);
      setTotal(res.total);
    } catch {
      toast.error('Не удалось загрузить балансы');
    } finally {
      setLoading(false);
    }
  }, [page, filter, toast]);

  useEffect(() => {
    fetchBalances();
  }, [fetchBalances]);

  const openCreditModal = (clientId: string, currency: string) => {
    setModalClientId(clientId);
    setModalCurrency(currency);
    setModalMode('credit');
    setCreditForm({ type: 'credit', amount: '', reason: '' });
    setModalOpen(true);
  };

  const openDebitModal = (clientId: string, currency: string) => {
    setModalClientId(clientId);
    setModalCurrency(currency);
    setModalMode('debit');
    setCreditForm({ type: 'debit', amount: '', reason: '' });
    setModalOpen(true);
  };

  const handleSubmitCreditDebit = async () => {
    if (!creditForm.reason.trim()) {
      toast.error('Причина обязательна');
      return;
    }
    setSaving(true);
    try {
      const isNegative = creditForm.type === 'debit';
      const amount = isNegative
        ? `-${creditForm.amount}`
        : creditForm.amount;
      const res = await billingApi.addCredits(modalClientId, {
        amount,
        currency: modalCurrency,
        description: `[${creditForm.type}] ${creditForm.reason}`,
      });
      toast.success(`Операция выполнена. Новый баланс: ${res.new_balance}`);
      setModalOpen(false);
      fetchBalances();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка операции');
    } finally {
      setSaving(false);
    }
  };

  const handleFreeze = async (clientId: string) => {
    try {
      await billingApi.freeze(clientId);
      toast.success('Счёт заморожен');
      fetchBalances();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Не удалось заморозить');
    }
  };

  const handleUnfreeze = async (clientId: string) => {
    try {
      await billingApi.unfreeze(clientId);
      toast.success('Счёт разморожен');
      fetchBalances();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Не удалось разморозить');
    }
  };

  const columns: Column<BalanceInfoItem>[] = [
    { key: 'client_name', header: 'Клиент' },
    {
      key: 'balance',
      header: 'Баланс',
      render: (b) => `${b.balance} ${b.currency}`,
      sortable: true,
    },
    {
      key: 'frozen',
      header: 'Статус',
      render: (b) =>
        b.frozen ? (
          <Badge variant="danger">Заморожен</Badge>
        ) : (
          <Badge variant="success">Активен</Badge>
        ),
    },
    {
      key: 'credit_limit',
      header: 'Кредитный лимит',
      render: (b) => b.credit_limit || '—',
    },
    {
      key: 'low_balance_threshold',
      header: 'Порог уведомления',
      render: (b) => b.low_balance_threshold || '—',
    },
  ];

  return (
    <>
      <FilterBar
        filters={balanceFilters}
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
        columns={columns}
        data={data}
        total={total}
        page={page}
        pageSize={PAGE_SIZE}
        onPageChange={setPage}
        loading={loading}
        keyField="client_id"
        rowActions={(item) => (
          <div className="flex gap-1">
            <Button size="sm" variant="secondary" onClick={() => openCreditModal(item.client_id, item.currency)}>
              Начислить
            </Button>
            <Button size="sm" variant="secondary" onClick={() => openDebitModal(item.client_id, item.currency)}>
              Списать
            </Button>
            {item.frozen ? (
              <Button size="sm" variant="ghost" onClick={() => handleUnfreeze(item.client_id)}>
                Разморозить
              </Button>
            ) : (
              <Button size="sm" variant="danger" onClick={() => handleFreeze(item.client_id)}>
                Заморозить
              </Button>
            )}
          </div>
        )}
      />

      <Modal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        title={modalMode === 'credit' ? 'Начисление средств' : 'Списание средств'}
      >
        <div className="space-y-4">
          <Select
            label="Тип операции"
            options={typeOptions}
            value={creditForm.type}
            onChange={(v) =>
              setCreditForm({ ...creditForm, type: v as CreditDebitForm['type'] })
            }
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
            label="Причина"
            value={creditForm.reason}
            onChange={(e) => setCreditForm({ ...creditForm, reason: e.target.value })}
            required
            placeholder="Укажите причину операции"
          />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setModalOpen(false)}>
              Отмена
            </Button>
            <Button
              onClick={handleSubmitCreditDebit}
              disabled={saving || !creditForm.amount || !creditForm.reason.trim()}
            >
              {saving ? 'Сохранение...' : 'Подтвердить'}
            </Button>
          </div>
        </div>
      </Modal>
    </>
  );
}
