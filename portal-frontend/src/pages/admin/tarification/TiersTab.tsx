import { useState, useEffect, useCallback, useMemo } from 'react';
import {
  ResponsiveContainer,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
} from 'recharts';
import { DataTable, type Column } from '../../../components/data/DataTable';
import { FilterBar, type FilterDef } from '../../../components/data/FilterBar';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { Select } from '../../../components/ui/Select';
import { Modal } from '../../../components/ui/Modal';
import { useToast } from '../../../components/ui/Toast';
import {
  tarificationApi,
  type TariffPlan,
  type TariffPeriod,
  type TariffTier,
} from '../../../api/admin';

export function TiersTab() {
  const toast = useToast();
  const [tiers, setTiers] = useState<TariffTier[]>([]);
  const [plans, setPlans] = useState<TariffPlan[]>([]);
  const [periods, setPeriods] = useState<TariffPeriod[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<Record<string, string>>({});
  const [showCreate, setShowCreate] = useState(false);
  const [showEdit, setShowEdit] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({
    tariff_period_id: '',
    from_count: '',
    price_per_segment: '',
  });
  const [editId, setEditId] = useState('');

  const fetchPlans = useCallback(async () => {
    try {
      const res = await tarificationApi.listTariffPlans();
      setPlans(res.tariff_plans || []);
    } catch {
      toast.error('Не удалось загрузить планы');
    }
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const fetchPeriods = useCallback(async () => {
    try {
      const params: { tariff_plan_id?: string } = {};
      if (filter.tariff_plan_id) params.tariff_plan_id = filter.tariff_plan_id;
      const res = await tarificationApi.listTariffPeriods(params);
      setPeriods(res.periods || []);
    } catch {
      toast.error('Не удалось загрузить периоды');
    }
  }, [filter.tariff_plan_id]); // eslint-disable-line react-hooks/exhaustive-deps

  const fetchTiers = useCallback(async () => {
    setLoading(true);
    try {
      const params: { tariff_period_id?: string } = {};
      if (filter.tariff_period_id)
        params.tariff_period_id = filter.tariff_period_id;
      const res = await tarificationApi.listTariffTiers(params);
      setTiers(res.tiers || []);
    } catch {
      toast.error('Не удалось загрузить тиры');
    } finally {
      setLoading(false);
    }
  }, [filter.tariff_period_id]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    fetchPlans();
  }, [fetchPlans]);

  useEffect(() => {
    fetchPeriods();
  }, [fetchPeriods]);

  useEffect(() => {
    fetchTiers();
  }, [fetchTiers]);

  const planOptions = useMemo(
    () => plans.map((p) => ({ value: p.tariff_plan_id, label: p.name })),
    [plans],
  );

  const periodOptions = useMemo(
    () =>
      periods.map((p) => ({
        value: p.id,
        label: `${new Date(p.start_date).toLocaleDateString('ru-RU')} - ${new Date(p.end_date).toLocaleDateString('ru-RU')}`,
      })),
    [periods],
  );

  const filters: FilterDef[] = useMemo(
    () => [
      {
        key: 'tariff_plan_id',
        label: 'Тарифный план',
        type: 'select' as const,
        options: planOptions,
        placeholder: 'Все планы',
      },
      {
        key: 'tariff_period_id',
        label: 'Период',
        type: 'select' as const,
        options: periodOptions,
        placeholder: 'Все периоды',
      },
    ],
    [planOptions, periodOptions],
  );

  const handleCreate = async () => {
    if (!form.tariff_period_id || !form.from_count || !form.price_per_segment) {
      toast.error('Заполните все поля');
      return;
    }
    setSaving(true);
    try {
      await tarificationApi.createTariffTier({
        tariff_period_id: form.tariff_period_id,
        from_count: Number(form.from_count),
        price_per_segment: form.price_per_segment,
      });
      toast.success('Тир создан');
      setShowCreate(false);
      setForm({ tariff_period_id: '', from_count: '', price_per_segment: '' });
      fetchTiers();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка создания');
    } finally {
      setSaving(false);
    }
  };

  const handleEdit = async () => {
    if (!form.from_count || !form.price_per_segment) {
      toast.error('Заполните все поля');
      return;
    }
    setSaving(true);
    try {
      await tarificationApi.updateTariffTier(editId, {
        from_count: Number(form.from_count),
        price_per_segment: form.price_per_segment,
      });
      toast.success('Тир обновлён');
      setShowEdit(false);
      fetchTiers();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка обновления');
    } finally {
      setSaving(false);
    }
  };

  const openEditModal = (tier: TariffTier) => {
    setEditId(tier.id);
    setForm({
      tariff_period_id: tier.tariff_period_id,
      from_count: String(tier.from_count),
      price_per_segment: tier.price_per_segment,
    });
    setShowEdit(true);
  };

  const columns: Column<TariffTier>[] = [
    {
      key: 'tariff_period_id',
      header: 'Период',
      render: (t) => {
        const period = periods.find((p) => p.id === t.tariff_period_id);
        return period
          ? `${new Date(period.start_date).toLocaleDateString('ru-RU')} - ${new Date(period.end_date).toLocaleDateString('ru-RU')}`
          : t.tariff_period_id.slice(0, 8) + '...';
      },
    },
    {
      key: 'from_count',
      header: 'От (кол-во)',
      sortable: true,
      render: (t) => (t.from_count ?? 0).toLocaleString('ru-RU'),
    },
    {
      key: 'price_per_segment',
      header: 'Цена за сегмент',
      render: (t) => t.price_per_segment,
    },
  ];

  // Chart data: sorted tiers for selected period
  const chartData = useMemo(() => {
    if (!filter.tariff_period_id) return [];
    return [...tiers]
      .sort((a, b) => a.from_count - b.from_count)
      .map((t) => ({
        from: t.from_count,
        price: Number(t.price_per_segment),
      }));
  }, [tiers, filter.tariff_period_id]);

  return (
    <>
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold">Тиры тарификации</h2>
        <Button
          size="sm"
          onClick={() => {
            setForm({
              tariff_period_id: filter.tariff_period_id || '',
              from_count: '',
              price_per_segment: '',
            });
            setShowCreate(true);
          }}
        >
          Создать тир
        </Button>
      </div>

      <FilterBar
        filters={filters}
        values={filter}
        onChange={(v) => {
          if (v.tariff_plan_id !== filter.tariff_plan_id) {
            setFilter({ ...v, tariff_period_id: '' });
          } else {
            setFilter(v);
          }
        }}
        onReset={() => setFilter({})}
      />

      {chartData.length > 1 && (
        <div className="mb-6 border border-gray-200 rounded-lg p-4">
          <h3 className="text-sm font-medium text-gray-700 mb-3">
            Ступенчатая шкала цен
          </h3>
          <ResponsiveContainer width="100%" height={200}>
            <BarChart data={chartData}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis
                dataKey="from"
                label={{
                  value: 'От (кол-во)',
                  position: 'insideBottom',
                  offset: -5,
                }}
              />
              <YAxis
                label={{
                  value: 'Цена',
                  angle: -90,
                  position: 'insideLeft',
                }}
              />
              <Tooltip
                formatter={(value) => [`${value}`, 'Цена за сегмент']}
                labelFormatter={(label) => `От ${label} сегментов`}
              />
              <Bar dataKey="price" fill="#3b82f6" radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}

      <DataTable
        columns={columns}
        data={tiers}
        total={tiers.length}
        page={1}
        pageSize={100}
        onPageChange={() => {}}
        loading={loading}
        keyField="id"
        rowActions={(item) => (
          <Button
            size="sm"
            variant="secondary"
            onClick={() => openEditModal(item)}
          >
            Изменить
          </Button>
        )}
      />

      <Modal
        open={showCreate}
        onClose={() => setShowCreate(false)}
        title="Создать тир"
      >
        <div className="space-y-4">
          <Select
            label="Период"
            options={periodOptions}
            value={form.tariff_period_id}
            onChange={(v) => setForm({ ...form, tariff_period_id: v })}
            placeholder="Выберите период"
          />
          <Input
            label="От (кол-во сегментов)"
            type="number"
            value={form.from_count}
            onChange={(e) => setForm({ ...form, from_count: e.target.value })}
            required
            placeholder="0"
          />
          <Input
            label="Цена за сегмент"
            type="number"
            step="0.0001"
            value={form.price_per_segment}
            onChange={(e) =>
              setForm({ ...form, price_per_segment: e.target.value })
            }
            required
            placeholder="0.00"
          />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setShowCreate(false)}>
              Отмена
            </Button>
            <Button
              onClick={handleCreate}
              disabled={
                saving ||
                !form.tariff_period_id ||
                !form.from_count ||
                !form.price_per_segment
              }
            >
              {saving ? 'Создание...' : 'Создать'}
            </Button>
          </div>
        </div>
      </Modal>

      <Modal
        open={showEdit}
        onClose={() => setShowEdit(false)}
        title="Редактировать тир"
      >
        <div className="space-y-4">
          <Input
            label="От (кол-во сегментов)"
            type="number"
            value={form.from_count}
            onChange={(e) => setForm({ ...form, from_count: e.target.value })}
            required
          />
          <Input
            label="Цена за сегмент"
            type="number"
            step="0.0001"
            value={form.price_per_segment}
            onChange={(e) =>
              setForm({ ...form, price_per_segment: e.target.value })
            }
            required
          />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setShowEdit(false)}>
              Отмена
            </Button>
            <Button
              onClick={handleEdit}
              disabled={
                saving || !form.from_count || !form.price_per_segment
              }
            >
              {saving ? 'Сохранение...' : 'Сохранить'}
            </Button>
          </div>
        </div>
      </Modal>
    </>
  );
}
