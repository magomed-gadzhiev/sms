import { useState, useEffect, useCallback } from 'react';
import { DataTable, type Column } from '../../../components/data/DataTable';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { Modal } from '../../../components/ui/Modal';
import { Badge } from '../../../components/ui/Badge';
import { useToast } from '../../../components/ui/Toast';
import { tarificationApi, type TariffPlan } from '../../../api/admin';

export function PlansTab() {
  const toast = useToast();
  const [plans, setPlans] = useState<TariffPlan[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({ name: '', description: '' });

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
  }, [toast]);

  useEffect(() => {
    fetchPlans();
  }, [fetchPlans]);

  const handleCreate = async () => {
    if (!form.name.trim()) {
      toast.error('Название обязательно');
      return;
    }
    setSaving(true);
    try {
      await tarificationApi.createTariffPlan({
        name: form.name,
        description: form.description,
        active: true,
      });
      toast.success('Тарифный план создан');
      setShowCreate(false);
      setForm({ name: '', description: '' });
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
    { key: 'name', header: 'Название', sortable: true },
    {
      key: 'description',
      header: 'Описание',
      responsive: true,
      render: (p) => p.description || '—',
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
      render: (p) => new Date(p.created_at).toLocaleDateString('ru-RU'),
    },
  ];

  return (
    <>
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold">Тарифные планы</h2>
        <Button
          size="sm"
          onClick={() => {
            setForm({ name: '', description: '' });
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
          <Input
            label="Название"
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.target.value })}
            required
            placeholder="Базовый тариф"
          />
          <Input
            label="Описание"
            value={form.description}
            onChange={(e) => setForm({ ...form, description: e.target.value })}
            placeholder="Описание тарифного плана"
          />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setShowCreate(false)}>
              Отмена
            </Button>
            <Button
              onClick={handleCreate}
              disabled={saving || !form.name.trim()}
            >
              {saving ? 'Создание...' : 'Создать'}
            </Button>
          </div>
        </div>
      </Modal>
    </>
  );
}
