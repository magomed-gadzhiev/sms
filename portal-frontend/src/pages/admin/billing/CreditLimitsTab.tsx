import { useState, useEffect, useCallback } from 'react';
import { DataTable, type Column } from '../../../components/data/DataTable';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { Modal } from '../../../components/ui/Modal';
import { useToast } from '../../../components/ui/Toast';
import { billingApi, type BalanceInfoItem } from '../../../api/admin';

const PAGE_SIZE = 20;

export function CreditLimitsTab() {
  const toast = useToast();
  const [data, setData] = useState<BalanceInfoItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);

  // Edit modal
  const [modalOpen, setModalOpen] = useState(false);
  const [editClientId, setEditClientId] = useState('');
  const [editClientName, setEditClientName] = useState('');
  const [editLimit, setEditLimit] = useState('');
  const [saving, setSaving] = useState(false);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await billingApi.listBalances({
        limit: PAGE_SIZE,
        offset: (page - 1) * PAGE_SIZE,
      });
      setData(res.balances || []);
      setTotal(res.total);
    } catch {
      toast.error('Не удалось загрузить данные');
    } finally {
      setLoading(false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const openEditModal = (item: BalanceInfoItem) => {
    setEditClientId(item.client_id);
    setEditClientName(item.client_name);
    setEditLimit(item.credit_limit || '0');
    setModalOpen(true);
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      await billingApi.setCreditLimit(editClientId, { credit_limit: editLimit });
      toast.success('Кредитный лимит обновлён');
      setModalOpen(false);
      fetchData();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка обновления');
    } finally {
      setSaving(false);
    }
  };

  const computeAvailable = (item: BalanceInfoItem): string => {
    const balance = parseFloat(item.balance) || 0;
    const limit = parseFloat(item.credit_limit) || 0;
    return (balance + limit).toFixed(2);
  };

  const computeOverdraft = (item: BalanceInfoItem): string => {
    const balance = parseFloat(item.balance) || 0;
    if (balance < 0) {
      return Math.abs(balance).toFixed(2);
    }
    return '—';
  };

  const columns: Column<BalanceInfoItem>[] = [
    { key: 'client_name', header: 'Клиент' },
    {
      key: 'balance',
      header: 'Баланс',
      render: (b) => `${parseFloat(b.balance).toFixed(2)} ${b.currency}`,
      sortable: true,
    },
    {
      key: 'credit_limit',
      header: 'Кредитный лимит',
      render: (b) => b.credit_limit || '0',
    },
    {
      key: 'available',
      header: 'Доступно',
      render: (b) => `${computeAvailable(b)} ${b.currency}`,
    },
    {
      key: 'overdraft',
      header: 'Овердрафт',
      render: (b) => {
        const overdraft = computeOverdraft(b);
        return overdraft === '—' ? '—' : `${overdraft} ${b.currency}`;
      },
    },
  ];

  return (
    <>
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
          <Button size="sm" variant="secondary" onClick={() => openEditModal(item)}>
            Изменить лимит
          </Button>
        )}
      />

      <Modal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        title={`Кредитный лимит — ${editClientName}`}
      >
        <div className="space-y-4">
          <Input
            label="Кредитный лимит"
            type="number"
            value={editLimit}
            onChange={(e) => setEditLimit(e.target.value)}
            required
            placeholder="0.00"
          />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setModalOpen(false)}>
              Отмена
            </Button>
            <Button onClick={handleSave} disabled={saving || !editLimit}>
              {saving ? 'Сохранение...' : 'Сохранить'}
            </Button>
          </div>
        </div>
      </Modal>
    </>
  );
}
