import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { cascadeDeliveriesApi, type Delivery, type DeliveryStrategy } from '../../api/cascade';
import { Badge } from '../../components/ui/Badge';
import { DataTable, type Column } from '../../components/data/DataTable';

const STATUS_BADGE: Record<string, { variant: 'warning' | 'success' | 'danger' | 'default'; label: string }> = {
  pending:     { variant: 'warning', label: 'Ожидание' },
  in_progress: { variant: 'warning', label: 'В процессе' },
  delivered:   { variant: 'success', label: 'Доставлено' },
  failed:      { variant: 'danger',  label: 'Не доставлено' },
  cancelled:   { variant: 'default', label: 'Отменено' },
};

function formatDate(dt: string) {
  return new Date(dt).toLocaleString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' });
}

const PAGE_SIZE = 20;

export function CascadeHistoryPage() {
  const navigate = useNavigate();
  const [deliveries, setDeliveries] = useState<Delivery[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const [strategies, setStrategies] = useState<DeliveryStrategy[]>([]);

  // Локальное (несохранённое) состояние фильтров — обновляется при изменении select'ов
  const [filterStrategy, setFilterStrategy] = useState('');
  const [filterStatus, setFilterStatus] = useState('');

  // Применённые фильтры — обновляются только при клике «Применить»
  const [appliedStrategy, setAppliedStrategy] = useState('');
  const [appliedStatus, setAppliedStatus] = useState('');

  useEffect(() => {
    cascadeDeliveriesApi.listStrategies().then((r) => setStrategies(r.strategies ?? [])).catch(() => {});
  }, []);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const res = await cascadeDeliveriesApi.list({
        strategy_id: appliedStrategy || undefined,
        status: appliedStatus || undefined,
        page,
        page_size: PAGE_SIZE,
      });
      setDeliveries(res.deliveries ?? []);
      setTotal(res.total ?? 0);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Ошибка загрузки');
    } finally {
      setLoading(false);
    }
  }, [page, appliedStrategy, appliedStatus]);

  useEffect(() => { load(); }, [load]);

  const applyFilters = () => {
    setAppliedStrategy(filterStrategy);
    setAppliedStatus(filterStatus);
    setPage(1);
  };

  const columns: Column<Delivery>[] = [
    {
      key: 'id',
      header: 'ID',
      render: (d) => <span className="font-mono text-xs">{d.id.slice(0, 8)}…</span>,
    },
    { key: 'recipient', header: 'Получатель' },
    {
      key: 'strategy_id',
      header: 'Стратегия',
      render: (d) => {
        const s = strategies.find((st) => st.strategy_id === d.strategy_id);
        return s?.name ?? d.strategy_id.slice(0, 8);
      },
    },
    {
      key: 'status',
      header: 'Статус',
      render: (d) => {
        const b = STATUS_BADGE[d.status] ?? { variant: 'default' as const, label: d.status };
        return <Badge variant={b.variant}>{b.label}</Badge>;
      },
    },
    { key: 'delivered_via', header: 'Канал', render: (d) => d.delivered_via || '—' },
    {
      key: 'total_cost',
      header: 'Стоимость',
      render: (d) => `${d.total_cost} ${d.currency}`,
    },
    { key: 'created_at', header: 'Дата', render: (d) => formatDate(d.created_at) },
  ];

  return (
    <div className="p-6">
      <h1 className="text-2xl font-semibold mb-6">История каскадных доставок</h1>

      {/* Filters */}
      <div className="flex flex-wrap gap-3 mb-4">
        <select
          value={filterStrategy}
          onChange={(e) => setFilterStrategy(e.target.value)}
          className="border border-gray-300 rounded-md px-3 py-2 text-sm"
        >
          <option value="">Все стратегии</option>
          {strategies.map((s) => (
            <option key={s.strategy_id} value={s.strategy_id}>{s.name}</option>
          ))}
        </select>

        <select
          value={filterStatus}
          onChange={(e) => setFilterStatus(e.target.value)}
          className="border border-gray-300 rounded-md px-3 py-2 text-sm"
        >
          <option value="">Все статусы</option>
          {Object.entries(STATUS_BADGE).map(([val, { label }]) => (
            <option key={val} value={val}>{label}</option>
          ))}
        </select>

        <button
          onClick={applyFilters}
          className="px-4 py-2 text-sm bg-primary text-white rounded-md hover:bg-primary/90"
        >
          Применить
        </button>
      </div>

      {error && (
        <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded text-red-700 text-sm">{error}</div>
      )}

      <DataTable
        columns={columns}
        data={deliveries}
        loading={loading}
        total={total}
        page={page}
        pageSize={PAGE_SIZE}
        onPageChange={setPage}
        keyField="id"
        onRowClick={(d) => navigate(`/cascade/history/${d.id}`)}
      />

    </div>
  );
}
