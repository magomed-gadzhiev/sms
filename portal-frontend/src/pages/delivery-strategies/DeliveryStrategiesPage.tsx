import { useState, useEffect, useCallback } from 'react';
import { cascadeStrategiesApi, type DeliveryStrategy } from '../../api/cascade';
import { Button } from '../../components/ui/Button';
import { Badge } from '../../components/ui/Badge';
import { DataTable, type Column } from '../../components/data/DataTable';
import { StrategyFormModal } from './StrategyFormModal';
import { OperatorSupportMatrix } from './OperatorSupportMatrix';

export function DeliveryStrategiesPage() {
  const [strategies, setStrategies] = useState<DeliveryStrategy[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [modalOpen, setModalOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<DeliveryStrategy | null>(null);
  const [ocsOpen, setOcsOpen] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const res = await cascadeStrategiesApi.list();
      setStrategies(res.strategies ?? []);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Ошибка загрузки');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleDelete = async (s: DeliveryStrategy) => {
    if (!confirm(`Удалить стратегию "${s.name}"?`)) return;
    try {
      await cascadeStrategiesApi.delete(s.strategy_id);
      load();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Ошибка удаления');
    }
  };

  const columns: Column<DeliveryStrategy>[] = [
    { key: 'name', header: 'Название' },
    {
      key: 'mode',
      header: 'Режим',
      render: (s) => (
        <Badge variant={s.mode === 'sequential' ? 'default' : 'warning'}>
          {s.mode === 'sequential' ? 'Последовательный' : 'Параллельный'}
        </Badge>
      ),
    },
    {
      key: 'steps',
      header: 'Шаги',
      render: (s) => `${s.steps?.length ?? 0} канал(а)`,
    },
    {
      key: 'active',
      header: 'Статус',
      render: (s) => (
        <Badge variant={s.active ? 'success' : 'default'}>
          {s.active ? 'Активна' : 'Отключена'}
        </Badge>
      ),
    },
    {
      key: 'actions' as keyof DeliveryStrategy,
      header: 'Действия',
      render: (s) => (
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={() => { setEditTarget(s); setModalOpen(true); }}>
            Редактировать
          </Button>
          <Button variant="secondary" size="sm" onClick={() => handleDelete(s)}>
            Удалить
          </Button>
        </div>
      ),
    },
  ];

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-semibold">Стратегии доставки</h1>
        <div className="flex gap-3">
          <Button variant="secondary" onClick={() => setOcsOpen(true)}>
            OCS матрица
          </Button>
          <Button onClick={() => { setEditTarget(null); setModalOpen(true); }}>
            Добавить стратегию
          </Button>
        </div>
      </div>

      {error && (
        <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded text-red-700 text-sm">
          {error}
        </div>
      )}

      <DataTable
        columns={columns}
        data={strategies}
        loading={loading}
        total={strategies.length}
        page={1}
        pageSize={strategies.length || 1}
        onPageChange={() => {}}
        keyField="strategy_id"
      />

      {modalOpen && (
        <StrategyFormModal
          strategy={editTarget}
          onClose={() => setModalOpen(false)}
          onSaved={() => { setModalOpen(false); load(); }}
        />
      )}

      {ocsOpen && (
        <OperatorSupportMatrix onClose={() => setOcsOpen(false)} />
      )}
    </div>
  );
}
