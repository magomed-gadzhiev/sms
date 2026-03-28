import { useState, useEffect, useCallback, useMemo } from 'react';
import { DataTable, type Column } from '../../../components/data/DataTable';
import { FilterBar, type FilterDef } from '../../../components/data/FilterBar';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { Select } from '../../../components/ui/Select';
import { Modal } from '../../../components/ui/Modal';
import { Badge } from '../../../components/ui/Badge';
import { useToast } from '../../../components/ui/Toast';
import {
  tarificationApi,
  type TariffPlan,
  type TariffPeriod,
} from '../../../api/admin';

function computeStatus(period: TariffPeriod): {
  label: string;
  variant: 'success' | 'warning' | 'default';
} {
  const now = new Date();
  const start = new Date(period.start_date);
  const end = new Date(period.end_date);
  if (end < now) return { label: 'Истёк', variant: 'default' };
  if (start > now) return { label: 'Запланирован', variant: 'warning' };
  return { label: 'Активен', variant: 'success' };
}

export function PeriodsTab() {
  const toast = useToast();
  const [periods, setPeriods] = useState<TariffPeriod[]>([]);
  const [plans, setPlans] = useState<TariffPlan[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<Record<string, string>>({});
  const [showCreate, setShowCreate] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({
    tariff_plan_id: '',
    start_date: '',
    end_date: '',
  });

  const fetchPlans = useCallback(async () => {
    try {
      const res = await tarificationApi.listTariffPlans();
      setPlans(res.tariff_plans || []);
    } catch {
      toast.error('Не удалось загрузить планы');
    }
  }, [toast]);

  const fetchPeriods = useCallback(async () => {
    setLoading(true);
    try {
      const params: { tariff_plan_id?: string } = {};
      if (filter.tariff_plan_id) params.tariff_plan_id = filter.tariff_plan_id;
      const res = await tarificationApi.listTariffPeriods(params);
      setPeriods(res.periods || []);
    } catch {
      toast.error('Не удалось загрузить периоды');
    } finally {
      setLoading(false);
    }
  }, [filter, toast]);

  useEffect(() => {
    fetchPlans();
  }, [fetchPlans]);

  useEffect(() => {
    fetchPeriods();
  }, [fetchPeriods]);

  const planOptions = useMemo(
    () => plans.map((p) => ({ value: p.tariff_plan_id, label: p.name })),
    [plans],
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
    ],
    [planOptions],
  );

  const handleCreate = async () => {
    if (!form.tariff_plan_id || !form.start_date || !form.end_date) {
      toast.error('Заполните все поля');
      return;
    }
    setSaving(true);
    try {
      await tarificationApi.createTariffPeriod({
        tariff_plan_id: form.tariff_plan_id,
        start_date: form.start_date,
        end_date: form.end_date,
      });
      toast.success('Период создан');
      setShowCreate(false);
      setForm({ tariff_plan_id: '', start_date: '', end_date: '' });
      fetchPeriods();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка создания');
    } finally {
      setSaving(false);
    }
  };

  const planNameMap = useMemo(() => {
    const map: Record<string, string> = {};
    for (const p of plans) map[p.tariff_plan_id] = p.name;
    return map;
  }, [plans]);

  const columns: Column<TariffPeriod>[] = [
    {
      key: 'tariff_plan_id',
      header: 'План',
      render: (p) =>
        planNameMap[p.tariff_plan_id] || p.tariff_plan_id.slice(0, 8) + '...',
    },
    {
      key: 'start_date',
      header: 'Дата начала',
      render: (p) => new Date(p.start_date).toLocaleDateString('ru-RU'),
      sortable: true,
    },
    {
      key: 'end_date',
      header: 'Дата окончания',
      render: (p) => new Date(p.end_date).toLocaleDateString('ru-RU'),
    },
    {
      key: 'status',
      header: 'Статус',
      render: (p) => {
        const s = computeStatus(p);
        return <Badge variant={s.variant}>{s.label}</Badge>;
      },
    },
  ];

  return (
    <>
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold">Периоды тарификации</h2>
        <Button
          size="sm"
          onClick={() => {
            setForm({ tariff_plan_id: '', start_date: '', end_date: '' });
            setShowCreate(true);
          }}
        >
          Создать период
        </Button>
      </div>

      <FilterBar
        filters={filters}
        values={filter}
        onChange={(v) => setFilter(v)}
        onReset={() => setFilter({})}
      />

      <DataTable
        columns={columns}
        data={periods}
        total={periods.length}
        page={1}
        pageSize={100}
        onPageChange={() => {}}
        loading={loading}
        keyField="id"
      />

      <Modal
        open={showCreate}
        onClose={() => setShowCreate(false)}
        title="Создать период"
      >
        <div className="space-y-4">
          <Select
            label="Тарифный план"
            options={planOptions}
            value={form.tariff_plan_id}
            onChange={(v) => setForm({ ...form, tariff_plan_id: v })}
            placeholder="Выберите план"
          />
          <Input
            label="Дата начала"
            type="date"
            value={form.start_date}
            onChange={(e) => setForm({ ...form, start_date: e.target.value })}
            required
          />
          <Input
            label="Дата окончания"
            type="date"
            value={form.end_date}
            onChange={(e) => setForm({ ...form, end_date: e.target.value })}
            required
          />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setShowCreate(false)}>
              Отмена
            </Button>
            <Button
              onClick={handleCreate}
              disabled={
                saving ||
                !form.tariff_plan_id ||
                !form.start_date ||
                !form.end_date
              }
            >
              {saving ? 'Создание...' : 'Создать'}
            </Button>
          </div>
        </div>
      </Modal>
    </>
  );
}
