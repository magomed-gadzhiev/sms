import { useState, useEffect, useCallback } from 'react';
import { DataTable, type Column } from '../../../components/data/DataTable';
import { Button } from '../../../components/ui/Button';
import { Modal } from '../../../components/ui/Modal';
import { Badge } from '../../../components/ui/Badge';
import { Select } from '../../../components/ui/Select';
import { useToast } from '../../../components/ui/Toast';
import { tarificationApi, operatorsApi, type TariffPlan, type OperatorInfo } from '../../../api/admin';

const SENDER_CATEGORY_OPTIONS = [
  { value: 'shared', label: 'Общий (shared)' },
  { value: 'paid_registered', label: 'Платная регистрация (paid_registered)' },
  { value: 'free_registered', label: 'Бесплатная регистрация (free_registered)' },
];

const STRATEGY_OPTIONS = [
  { value: 'fixed', label: 'Фиксированная (fixed)' },
  { value: 'threshold', label: 'Пороговая (threshold)' },
  { value: 'threshold_recalc', label: 'Пороговая с пересчётом (threshold_recalc)' },
  { value: 'prepaid_threshold', label: 'Предоплата с порогом (prepaid_threshold)' },
];

export function PlansTab() {
  const toast = useToast();
  const [plans, setPlans] = useState<TariffPlan[]>([]);
  const [operators, setOperators] = useState<{ value: string; label: string }[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({
    operator_id: '',
    sender_category: '',
    strategy: '',
  });

  const fetchPlans = useCallback(async () => {
    setLoading(true);
    try {
      const res = await tarificationApi.listTariffPlans();
      setPlans(res.tariff_plans || []);
    } catch {
      toast.error('Не удалось загрузить тарифные планы');
    } finally {
      setLoading(false);
    }
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const fetchOperators = useCallback(async () => {
    try {
      const res = await operatorsApi.list({ limit: 200 });
      const ops = (res.operators || []).map((o: OperatorInfo) => ({
        value: o.operator_id || o.id || '',
        label: o.name,
      }));
      setOperators(ops);
    } catch {
      // operators not critical
    }
  }, []);

  useEffect(() => {
    fetchPlans();
    fetchOperators();
  }, [fetchPlans, fetchOperators]);

  const handleCreate = async () => {
    if (!form.operator_id || !form.sender_category || !form.strategy) {
      toast.error('Заполните все поля');
      return;
    }
    setSaving(true);
    try {
      await tarificationApi.createTariffPlan({
        operator_id: form.operator_id,
        sender_category: form.sender_category,
        strategy: form.strategy,
      });
      toast.success('Тарифный план создан');
      setShowCreate(false);
      setForm({ operator_id: '', sender_category: '', strategy: '' });
      fetchPlans();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка создания');
    } finally {
      setSaving(false);
    }
  };

  const handleToggleActive = async (plan: TariffPlan) => {
    try {
      await tarificationApi.updateTariffPlan(plan.tariff_plan_id, {
        active: !plan.active,
      });
      toast.success(plan.active ? 'План деактивирован' : 'План активирован');
      fetchPlans();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка обновления');
    }
  };

  const columns: Column<TariffPlan>[] = [
    {
      key: 'tariff_plan_id',
      header: 'ID',
      render: (p) => p.tariff_plan_id.slice(0, 8) + '...',
    },
    {
      key: 'name',
      header: 'Категория / Стратегия',
      sortable: true,
      render: (p) => p.name,
    },
    {
      key: 'operator_id',
      header: 'Оператор',
      responsive: true,
      render: (p) => {
        const op = operators.find((o) => o.value === p.operator_id);
        return op ? op.label : (p.operator_id ? p.operator_id.slice(0, 8) + '...' : '—');
      },
    },
    {
      key: 'active',
      header: 'Активен',
      render: (p) =>
        p.active ? (
          <Badge variant="success">Да</Badge>
        ) : (
          <Badge variant="default">Нет</Badge>
        ),
    },
    {
      key: 'created_at',
      header: 'Создан',
      responsive: true,
      render: (p) =>
        p.created_at ? new Date(p.created_at).toLocaleDateString('ru-RU') : '—',
    },
  ];

  return (
    <>
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold">Тарифные планы</h2>
        <Button
          size="sm"
          onClick={() => {
            setForm({ operator_id: '', sender_category: '', strategy: '' });
            setShowCreate(true);
          }}
        >
          Создать план
        </Button>
      </div>

      <DataTable
        columns={columns}
        data={plans}
        total={plans.length}
        page={1}
        pageSize={100}
        onPageChange={() => {}}
        loading={loading}
        keyField="tariff_plan_id"
        rowActions={(item) => (
          <Button
            size="sm"
            variant={item.active ? 'danger' : 'secondary'}
            onClick={() => handleToggleActive(item)}
          >
            {item.active ? 'Деактивировать' : 'Активировать'}
          </Button>
        )}
      />

      <Modal
        open={showCreate}
        onClose={() => setShowCreate(false)}
        title="Создать тарифный план"
      >
        <div className="space-y-4">
          <Select
            label="Оператор"
            options={operators}
            value={form.operator_id}
            onChange={(v) => setForm({ ...form, operator_id: v })}
            placeholder="Выберите оператора"
          />
          <Select
            label="Категория отправителя"
            options={SENDER_CATEGORY_OPTIONS}
            value={form.sender_category}
            onChange={(v) => setForm({ ...form, sender_category: v })}
            placeholder="Выберите категорию"
          />
          <Select
            label="Стратегия тарификации"
            options={STRATEGY_OPTIONS}
            value={form.strategy}
            onChange={(v) => setForm({ ...form, strategy: v })}
            placeholder="Выберите стратегию"
          />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setShowCreate(false)}>
              Отмена
            </Button>
            <Button
              onClick={handleCreate}
              disabled={
                saving ||
                !form.operator_id ||
                !form.sender_category ||
                !form.strategy
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
