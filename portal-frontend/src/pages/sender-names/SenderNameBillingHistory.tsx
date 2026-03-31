import { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { senderTariffApi, ApiError } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { Button } from '../../components/ui/Button';

interface BillingRecord {
  id: string;
  billing_month: string;
  amount: string;
  created_at: string;
}

function formatMonth(dateStr: string) {
  const d = new Date(dateStr);
  return d.toLocaleDateString('ru-RU', { month: 'long', year: 'numeric' });
}

function formatRub(amount: string) {
  return parseFloat(amount).toLocaleString('ru-RU', { style: 'currency', currency: 'RUB' });
}

function formatDate(dt: string) {
  return new Date(dt).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' });
}

export function SenderNameBillingHistory() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [records, setRecords] = useState<BillingRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError('');
    try {
      const res = await senderTariffApi.getBillingHistory(id);
      setRecords(res.records ?? []);
      setTotal(res.total ?? 0);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Ошибка загрузки');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => { load(); }, [load]);

  const columns: Column<BillingRecord>[] = [
    {
      key: 'billing_month',
      header: 'Расчётный месяц',
      render: (r) => <span className="font-medium">{formatMonth(r.billing_month)}</span>,
    },
    {
      key: 'amount',
      header: 'Сумма',
      render: (r) => <span className="font-semibold text-blue-700">{formatRub(r.amount)}</span>,
    },
    {
      key: 'created_at',
      header: 'Дата начисления',
      render: (r) => <span className="text-sm text-gray-500">{formatDate(r.created_at)}</span>,
    },
  ];

  return (
    <div>
      <PageHeader
        title="История начислений"
        subtitle={`Регистрация: ${id}`}
        breadcrumbs={[
          { label: 'Имена отправителей', href: '/sender-names' },
          { label: 'История начислений' },
        ]}
        actions={<Button variant="secondary" onClick={() => navigate(-1)}>Назад</Button>}
      />

      {error && <div className="mb-4 p-3 bg-red-50 text-red-700 rounded-md text-sm">{error}</div>}

      <div className="mb-3 text-sm text-gray-500">Всего начислений: {total}</div>

      <DataTable
        columns={columns}
        data={records}
        loading={loading}
        total={total}
        page={1}
        pageSize={50}
        onPageChange={() => {}}
        keyField="id"
      />
    </div>
  );
}
